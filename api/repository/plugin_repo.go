package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/SemRels/semrel-registry/api/database"
	appErrors "github.com/SemRels/semrel-registry/api/internal"
	"github.com/SemRels/semrel-registry/api/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const sha256Algorithm = "sha256"

type PluginRepository interface {
	GetAll(ctx context.Context, limit, offset int, filters ...Filter) ([]models.Plugin, error)
	GetByID(ctx context.Context, id int64) (*models.Plugin, error)
	GetByName(ctx context.Context, name string) (*models.Plugin, error)
	GetVersions(ctx context.Context, pluginID int64) ([]models.PluginVersion, error)
	Create(ctx context.Context, plugin *models.Plugin) (int64, error)
	Update(ctx context.Context, plugin *models.Plugin) error
	UpdateStatus(ctx context.Context, id int64, status string) error
	UpdateValidationChecks(ctx context.Context, id int64, checksJSON []byte) error
	Delete(ctx context.Context, id int64) error
	AddVersion(ctx context.Context, version *models.PluginVersion) (int64, error)
}

type pgRepository struct {
	db *database.Database
}

func NewPluginRepository(db *database.Database) PluginRepository {
	return &pgRepository{db: db}
}

func (r *pgRepository) GetAll(ctx context.Context, limit, offset int, filters ...Filter) ([]models.Plugin, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}

	var query strings.Builder
	query.WriteString(`
SELECT id, name, COALESCE(description, ''), COALESCE(author, ''), category, COALESCE(repository, ''), COALESCE(license, ''), COALESCE(status, 'active'), COALESCE(tags, ARRAY[]::TEXT[]), validation_checks, validated_at, created_at, updated_at, deleted_at
FROM plugins
WHERE deleted_at IS NULL`)

	args := make([]interface{}, 0)
	hasSort := false
	for _, filter := range filters {
		if filter == nil {
			continue
		}
		if _, ok := filter.(SortFilter); ok {
			hasSort = true
		}
		filter.ApplyTo(&query, &args)
	}

	if !hasSort {
		query.WriteString(" ORDER BY name ASC")
	}
	if limit > 0 {
		args = append(args, limit)
		query.WriteString(fmt.Sprintf(" LIMIT $%d", len(args)))
	}
	if offset > 0 {
		args = append(args, offset)
		query.WriteString(fmt.Sprintf(" OFFSET $%d", len(args)))
	}

	rows, err := r.db.Pool().Query(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("query plugins: %w", err)
	}
	defer rows.Close()

	plugins := make([]models.Plugin, 0)
	for rows.Next() {
		plugin, err := scanPlugin(rows)
		if err != nil {
			return nil, err
		}

		plugin.Versions, err = r.GetVersions(ctx, plugin.ID)
		if err != nil {
			return nil, err
		}

		plugins = append(plugins, *plugin)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plugins: %w", err)
	}

	return plugins, nil
}

func (r *pgRepository) GetByID(ctx context.Context, id int64) (*models.Plugin, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}

	row := r.db.Pool().QueryRow(ctx, `
SELECT id, name, COALESCE(description, ''), COALESCE(author, ''), category, COALESCE(repository, ''), COALESCE(license, ''), COALESCE(status, 'active'), COALESCE(tags, ARRAY[]::TEXT[]), validation_checks, validated_at, created_at, updated_at, deleted_at
FROM plugins
WHERE id = $1 AND deleted_at IS NULL`, id)

	plugin, err := scanPlugin(row)
	if err != nil {
		return nil, err
	}

	plugin.Versions, err = r.GetVersions(ctx, plugin.ID)
	if err != nil {
		return nil, err
	}

	return plugin, nil
}

func (r *pgRepository) GetByName(ctx context.Context, name string) (*models.Plugin, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}

	row := r.db.Pool().QueryRow(ctx, `
SELECT id, name, COALESCE(description, ''), COALESCE(author, ''), category, COALESCE(repository, ''), COALESCE(license, ''), COALESCE(status, 'active'), COALESCE(tags, ARRAY[]::TEXT[]), validation_checks, validated_at, created_at, updated_at, deleted_at
FROM plugins
WHERE name = $1 AND deleted_at IS NULL`, name)

	plugin, err := scanPlugin(row)
	if err != nil {
		return nil, err
	}

	plugin.Versions, err = r.GetVersions(ctx, plugin.ID)
	if err != nil {
		return nil, err
	}

	return plugin, nil
}

func (r *pgRepository) GetVersions(ctx context.Context, pluginID int64) ([]models.PluginVersion, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}

	rows, err := r.db.Pool().Query(ctx, `
SELECT id, plugin_id, version, release_date, COALESCE(changelog, ''), download_url, prerelease, created_at
FROM plugin_versions
WHERE plugin_id = $1
ORDER BY release_date DESC NULLS LAST, created_at DESC`, pluginID)
	if err != nil {
		return nil, fmt.Errorf("query versions: %w", err)
	}
	defer rows.Close()

	versions := make([]models.PluginVersion, 0)
	for rows.Next() {
		version, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}

		version.Checksums, err = r.loadChecksums(ctx, version.ID)
		if err != nil {
			return nil, err
		}

		versions = append(versions, *version)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate versions: %w", err)
	}

	return versions, nil
}

func (r *pgRepository) Create(ctx context.Context, plugin *models.Plugin) (int64, error) {
	if err := r.validate(); err != nil {
		return 0, err
	}
	if plugin == nil {
		return 0, fmt.Errorf("plugin is required")
	}

	tx, err := r.db.BeginTx()
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	err = tx.QueryRow(ctx, `
INSERT INTO plugins (name, description, author, category, repository, license, status, tags)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, created_at, updated_at`,
		plugin.Name,
		plugin.Description,
		plugin.Author,
		plugin.Category,
		plugin.Repository,
		plugin.License,
		plugin.Status,
		plugin.Tags,
	).Scan(&plugin.ID, &plugin.CreatedAt, &plugin.UpdatedAt)
	if err != nil {
		return 0, wrapWriteError("create plugin", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit create plugin: %w", err)
	}

	return plugin.ID, nil
}

func (r *pgRepository) Update(ctx context.Context, plugin *models.Plugin) error {
	if err := r.validate(); err != nil {
		return err
	}
	if plugin == nil {
		return fmt.Errorf("plugin is required")
	}

	row := r.db.Pool().QueryRow(ctx, `
UPDATE plugins
SET name = $1,
    description = $2,
    author = $3,
    category = $4,
    repository = $5,
    license = $6,
    tags = $7,
    updated_at = NOW()
WHERE id = $8 AND deleted_at IS NULL
RETURNING updated_at`,
		plugin.Name,
		plugin.Description,
		plugin.Author,
		plugin.Category,
		plugin.Repository,
		plugin.License,
		plugin.Tags,
		plugin.ID,
	)

	if err := row.Scan(&plugin.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return appErrors.ErrPluginNotFound
		}
		return wrapWriteError("update plugin", err)
	}

	return nil
}

func (r *pgRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	if err := r.validate(); err != nil {
		return err
	}
	result, err := r.db.Pool().Exec(ctx, `
UPDATE plugins SET status = $1, updated_at = NOW()
WHERE id = $2 AND deleted_at IS NULL`, status, id)
	if err != nil {
		return fmt.Errorf("update plugin status: %w", err)
	}
	if result.RowsAffected() == 0 {
		return appErrors.ErrPluginNotFound
	}
	return nil
}

func (r *pgRepository) UpdateValidationChecks(ctx context.Context, id int64, checksJSON []byte) error {
	if err := r.validate(); err != nil {
		return err
	}
	result, err := r.db.Pool().Exec(ctx, `
UPDATE plugins SET validation_checks = $1, validated_at = NOW(), updated_at = NOW()
WHERE id = $2 AND deleted_at IS NULL`, checksJSON, id)
	if err != nil {
		return fmt.Errorf("update validation checks: %w", err)
	}
	if result.RowsAffected() == 0 {
		return appErrors.ErrPluginNotFound
	}
	return nil
}

func (r *pgRepository) Delete(ctx context.Context, id int64) error {
	if err := r.validate(); err != nil {
		return err
	}

	result, err := r.db.Pool().Exec(ctx, `
UPDATE plugins
SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("delete plugin: %w", err)
	}
	if result.RowsAffected() == 0 {
		return appErrors.ErrPluginNotFound
	}

	return nil
}

func (r *pgRepository) AddVersion(ctx context.Context, version *models.PluginVersion) (int64, error) {
	if err := r.validate(); err != nil {
		return 0, err
	}
	if version == nil {
		return 0, fmt.Errorf("plugin version is required")
	}

	tx, err := r.db.BeginTx()
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	err = tx.QueryRow(ctx, `
INSERT INTO plugin_versions (plugin_id, version, release_date, changelog, download_url, prerelease)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, created_at`,
		version.PluginID,
		version.Version,
		version.ReleaseDate,
		version.Changelog,
		version.DownloadURL,
		version.Prerelease,
	).Scan(&version.ID, &version.CreatedAt)
	if err != nil {
		return 0, wrapWriteError("create plugin version", err)
	}

	checksumPlatforms := make([]string, 0, len(version.Checksums))
	for platform := range version.Checksums {
		checksumPlatforms = append(checksumPlatforms, platform)
	}
	sort.Strings(checksumPlatforms)

	for _, platform := range checksumPlatforms {
		if _, err := tx.Exec(ctx, `
INSERT INTO plugin_checksums (version_id, platform, algorithm, hash)
VALUES ($1, $2, $3, $4)`, version.ID, platform, sha256Algorithm, version.Checksums[platform]); err != nil {
			return 0, wrapWriteError("create plugin checksum", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit create plugin version: %w", err)
	}

	return version.ID, nil
}

func (r *pgRepository) loadChecksums(ctx context.Context, versionID int64) (map[string]string, error) {
	rows, err := r.db.Pool().Query(ctx, `
SELECT platform, hash
FROM plugin_checksums
WHERE version_id = $1
ORDER BY platform ASC`, versionID)
	if err != nil {
		return nil, fmt.Errorf("query checksums: %w", err)
	}
	defer rows.Close()

	checksums := make(map[string]string)
	for rows.Next() {
		var platform string
		var hash string
		if err := rows.Scan(&platform, &hash); err != nil {
			return nil, fmt.Errorf("scan checksum: %w", err)
		}
		checksums[platform] = hash
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate checksums: %w", err)
	}

	return checksums, nil
}

func (r *pgRepository) validate() error {
	if r == nil || r.db == nil || r.db.Pool() == nil {
		return fmt.Errorf("database is not initialized")
	}
	return nil
}

func scanPlugin(scanner interface {
	Scan(dest ...interface{}) error
}) (*models.Plugin, error) {
	var plugin models.Plugin
	if err := scanner.Scan(
		&plugin.ID,
		&plugin.Name,
		&plugin.Description,
		&plugin.Author,
		&plugin.Category,
		&plugin.Repository,
		&plugin.License,
		&plugin.Status,
		&plugin.Tags,
		&plugin.ValidationChecks,
		&plugin.ValidatedAt,
		&plugin.CreatedAt,
		&plugin.UpdatedAt,
		&plugin.DeletedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appErrors.ErrPluginNotFound
		}
		return nil, fmt.Errorf("scan plugin: %w", err)
	}
	if plugin.Tags == nil {
		plugin.Tags = []string{}
	}
	if plugin.Versions == nil {
		plugin.Versions = []models.PluginVersion{}
	}
	return &plugin, nil
}

func scanVersion(scanner interface {
	Scan(dest ...interface{}) error
}) (*models.PluginVersion, error) {
	var version models.PluginVersion
	if err := scanner.Scan(
		&version.ID,
		&version.PluginID,
		&version.Version,
		&version.ReleaseDate,
		&version.Changelog,
		&version.DownloadURL,
		&version.Prerelease,
		&version.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan version: %w", err)
	}
	if version.Checksums == nil {
		version.Checksums = make(map[string]string)
	}
	return &version, nil
}

func wrapWriteError(operation string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return fmt.Errorf("%s: %w", operation, appErrors.ErrDuplicatePlugin)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return appErrors.ErrPluginNotFound
	}
	return fmt.Errorf("%s: %w", operation, err)
}
