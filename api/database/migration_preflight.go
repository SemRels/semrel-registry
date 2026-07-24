package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/SemRels/semrel-registry/api/naming"
)

type migrationPlugin struct {
	id         int64
	namespace  string
	name       string
	repository string
	deleted    bool
	canonical  naming.FirstPartyPlugin
	firstParty bool
}

func (d *Database) installMigrationWriteGuard() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var migrationsExist bool
	if err := d.pool.QueryRow(ctx,
		`SELECT to_regclass('public.schema_migrations') IS NOT NULL`).
		Scan(&migrationsExist); err != nil {
		return fmt.Errorf("inspect migration table for write guard: %w", err)
	}
	if !migrationsExist {
		return nil
	}
	var atVersionEight bool
	if err := d.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(version) = 8 AND NOT BOOL_OR(dirty), FALSE)
		FROM schema_migrations`).
		Scan(&atVersionEight); err != nil {
		return fmt.Errorf("read migration state for write guard: %w", err)
	}
	if !atVersionEight {
		return nil
	}
	_, err := d.pool.Exec(ctx, `
		CREATE OR REPLACE FUNCTION lock_semrel_plugin_write() RETURNS trigger AS $$
		BEGIN
			PERFORM pg_advisory_xact_lock(91557115086156);
			RETURN NULL;
		END;
		$$ LANGUAGE plpgsql;
		DROP TRIGGER IF EXISTS plugin_write_migration_guard ON plugins;
		CREATE TRIGGER plugin_write_migration_guard
		BEFORE INSERT OR UPDATE OR DELETE ON plugins
		FOR EACH STATEMENT EXECUTE FUNCTION lock_semrel_plugin_write()`)
	if err != nil {
		return fmt.Errorf("create version-8 plugin write guard: %w", err)
	}
	return nil
}

func (d *Database) preflightCanonicalNamesMigration() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var migrationsExist bool
	if err := d.pool.QueryRow(ctx,
		`SELECT to_regclass('public.schema_migrations') IS NOT NULL`).
		Scan(&migrationsExist); err != nil {
		return fmt.Errorf("inspect migration table: %w", err)
	}
	if !migrationsExist {
		return nil
	}

	var version int
	var dirty bool
	if err := d.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(version), 0), COALESCE(BOOL_OR(dirty), FALSE)
		FROM schema_migrations`).
		Scan(&version, &dirty); err != nil {
		return fmt.Errorf("read migration state: %w", err)
	}
	if version != 8 || dirty {
		return nil
	}

	plugins, err := d.migrationPlugins(ctx)
	if err != nil {
		return err
	}
	ownerFor := func(plugin migrationPlugin) string {
		if plugin.firstParty && !plugin.deleted {
			return "first-party:" + plugin.canonical.Name
		}
		return fmt.Sprintf("plugin:%d", plugin.id)
	}
	claims := make(map[string]string)
	claim := func(ref, owner string) error {
		normalized := strings.ToLower(ref)
		if existing, ok := claims[normalized]; ok && existing != owner {
			return fmt.Errorf("plugin identity %q is claimed by multiple plugins", ref)
		}
		claims[normalized] = owner
		return nil
	}

	for _, plugin := range plugins {
		if plugin.deleted || !plugin.firstParty {
			continue
		}
		for _, occupied := range plugins {
			if occupied.id == plugin.id ||
				!strings.EqualFold(occupied.namespace, naming.FirstPartyNamespace) ||
				!strings.EqualFold(occupied.name, plugin.canonical.Name) {
				continue
			}
			if !occupied.firstParty ||
				occupied.canonical.Name != plugin.canonical.Name {
				return fmt.Errorf("canonical identity is already occupied: %s",
					plugin.canonical.Name)
			}
		}
	}

	for _, plugin := range plugins {
		if plugin.deleted {
			continue
		}
		ref := plugin.name
		if plugin.namespace != "" {
			ref = plugin.namespace + "/" + plugin.name
		}
		if plugin.firstParty {
			ref = naming.FirstPartyNamespace + "/" + plugin.canonical.Name
		}
		if err := claim(ref, ownerFor(plugin)); err != nil {
			return err
		}
	}
	var aliasesExist bool
	if err := d.pool.QueryRow(ctx,
		`SELECT to_regclass('public.plugin_aliases') IS NOT NULL`).
		Scan(&aliasesExist); err != nil {
		return fmt.Errorf("inspect plugin aliases: %w", err)
	}
	if aliasesExist {
		rows, err := d.pool.Query(ctx, `
			SELECT a.alias, a.plugin_id
			FROM plugin_aliases a
			JOIN plugins p ON p.id = a.plugin_id
			WHERE p.deleted_at IS NULL
			ORDER BY LOWER(a.alias), a.plugin_id`)
		if err != nil {
			return fmt.Errorf("read plugin aliases: %w", err)
		}
		for rows.Next() {
			var alias string
			var pluginID int64
			if err := rows.Scan(&alias, &pluginID); err != nil {
				rows.Close()
				return fmt.Errorf("scan plugin alias: %w", err)
			}
			owner := fmt.Sprintf("plugin:%d", pluginID)
			for _, plugin := range plugins {
				if plugin.id == pluginID {
					owner = ownerFor(plugin)
					break
				}
			}
			if err := claim(alias, owner); err != nil {
				rows.Close()
				return err
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("iterate plugin aliases: %w", err)
		}
		rows.Close()
	}
	for _, plugin := range plugins {
		if plugin.deleted || !plugin.firstParty {
			continue
		}
		for _, alias := range plugin.canonical.Aliases {
			if err := claim(alias, ownerFor(plugin)); err != nil {
				return err
			}
		}
	}

	// Plugin description, author, license, and status are intentionally not
	// preflight conflicts. Migration 9 deterministically prefers the exact
	// canonical typed row, then a scoped legacy row, then the oldest ID. The
	// merge keeps nonempty target display metadata, fills only empty target
	// fields from sources, and leaves target status unchanged. These fields do
	// not identify artifacts, unlike version metadata and checksums below.

	return d.preflightDuplicateVersions(ctx, plugins, ownerFor)
}

func (d *Database) migrationPlugins(ctx context.Context) ([]migrationPlugin, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, COALESCE(namespace, ''), name, COALESCE(repository, ''),
		       deleted_at IS NOT NULL
		FROM plugins
		ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("read plugins: %w", err)
	}
	defer rows.Close()

	var plugins []migrationPlugin
	for rows.Next() {
		var plugin migrationPlugin
		if err := rows.Scan(&plugin.id, &plugin.namespace, &plugin.name,
			&plugin.repository, &plugin.deleted); err != nil {
			return nil, fmt.Errorf("scan plugin: %w", err)
		}
		plugin.canonical, plugin.firstParty =
			naming.FirstPartyByRepositoryURL(plugin.repository)
		plugins = append(plugins, plugin)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plugins: %w", err)
	}
	return plugins, nil
}

type versionMetadata struct {
	releaseDate *time.Time
	changelog   *string
	downloadURL string
	prerelease  bool
}

func (d *Database) preflightDuplicateVersions(
	ctx context.Context,
	plugins []migrationPlugin,
	ownerFor func(migrationPlugin) string,
) error {
	pluginOwners := make(map[int64]string)
	for _, plugin := range plugins {
		if plugin.firstParty && !plugin.deleted {
			pluginOwners[plugin.id] = ownerFor(plugin)
		}
	}
	rows, err := d.pool.Query(ctx, `
		SELECT plugin_id, version, release_date, changelog, download_url,
		       COALESCE(prerelease, FALSE)
		FROM plugin_versions
		ORDER BY id`)
	if err != nil {
		return fmt.Errorf("read plugin versions: %w", err)
	}
	defer rows.Close()

	versions := make(map[string]versionMetadata)
	for rows.Next() {
		var pluginID int64
		var number string
		var metadata versionMetadata
		if err := rows.Scan(&pluginID, &number, &metadata.releaseDate,
			&metadata.changelog, &metadata.downloadURL, &metadata.prerelease); err != nil {
			return fmt.Errorf("scan plugin version: %w", err)
		}
		owner, ok := pluginOwners[pluginID]
		if !ok {
			continue
		}
		key := owner + "\x00" + number
		if existing, ok := versions[key]; ok &&
			(!equalTime(existing.releaseDate, metadata.releaseDate) ||
				!equalString(existing.changelog, metadata.changelog) ||
				existing.downloadURL != metadata.downloadURL ||
				existing.prerelease != metadata.prerelease) {
			return fmt.Errorf("duplicate first-party version %s has differing metadata", number)
		}
		versions[key] = metadata
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate plugin versions: %w", err)
	}
	return d.preflightDuplicateChecksums(ctx, pluginOwners)
}

func (d *Database) preflightDuplicateChecksums(
	ctx context.Context,
	pluginOwners map[int64]string,
) error {
	rows, err := d.pool.Query(ctx, `
		SELECT v.plugin_id, v.version, c.platform, c.algorithm, c.hash
		FROM plugin_checksums c
		JOIN plugin_versions v ON v.id = c.version_id
		ORDER BY c.id`)
	if err != nil {
		return fmt.Errorf("read plugin checksums: %w", err)
	}
	defer rows.Close()

	checksums := make(map[string]string)
	for rows.Next() {
		var pluginID int64
		var version, platform, algorithm, hash string
		if err := rows.Scan(&pluginID, &version, &platform, &algorithm, &hash); err != nil {
			return fmt.Errorf("scan plugin checksum: %w", err)
		}
		owner, ok := pluginOwners[pluginID]
		if !ok {
			continue
		}
		key := owner + "\x00" + version + "\x00" + strings.ToLower(platform)
		value := algorithm + "\x00" + hash
		if existing, ok := checksums[key]; ok && existing != value {
			return fmt.Errorf("duplicate first-party version %s has differing checksums", version)
		}
		checksums[key] = value
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate plugin checksums: %w", err)
	}
	return nil
}

func equalTime(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

func equalString(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
