package database

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SeedPluginVersion struct {
	Version     string            `json:"version"`
	ReleaseDate string            `json:"releaseDate"`
	DownloadURL string            `json:"downloadUrl"`
	Changelog   string            `json:"changelog"`
	Prerelease  bool              `json:"prerelease"`
	Checksums   map[string]string `json:"checksums"`
}

type SeedPlugin struct {
	Namespace   string              `json:"namespace"`
	Name        string              `json:"name"`
	Aliases     []string            `json:"aliases"`
	Description string              `json:"description"`
	Author      string              `json:"author"`
	Category    string              `json:"category"`
	Repository  string              `json:"repository"`
	License     string              `json:"license"`
	Tags        []string            `json:"tags"`
	Versions    []SeedPluginVersion `json:"versions"`
}

type SeedRegistry struct {
	SchemaVersion int          `json:"schemaVersion"`
	GeneratedAt   string       `json:"generatedAt"`
	Plugins       []SeedPlugin `json:"plugins"`
}

type SeedImportResult struct {
	Plugins  int
	Versions int
}

// ImportSeedFile imports every bundled plugin using an independent transaction.
// A later startup safely retries the complete file after any plugin fails.
func ImportSeedFile(ctx context.Context, pool *pgxpool.Pool, filePath string) (SeedImportResult, error) {
	registry, err := ReadSeedFile(filePath)
	if err != nil {
		return SeedImportResult{}, err
	}

	result := SeedImportResult{}
	for _, plugin := range registry.Plugins {
		if _, err := UpsertSeedPlugin(ctx, pool, plugin); err != nil {
			return result, fmt.Errorf("import plugin %q: %w", plugin.Name, err)
		}
		result.Plugins++
		result.Versions += len(plugin.Versions)
	}
	return result, nil
}

// ReadSeedFile reads and parses a registry catalog without importing it.
func ReadSeedFile(filePath string) (SeedRegistry, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return SeedRegistry{}, fmt.Errorf("read seed file %q: %w", filePath, err)
	}

	var registry SeedRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		return SeedRegistry{}, fmt.Errorf("parse seed file %q: %w", filePath, err)
	}
	return registry, nil
}

// UpsertSeedPlugin atomically imports one plugin and all of its seeded child data.
func UpsertSeedPlugin(ctx context.Context, pool *pgxpool.Pool, plugin SeedPlugin) (int64, error) {
	if pool == nil {
		return 0, fmt.Errorf("database pool is required")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin plugin seed transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := LockPluginWrites(ctx, tx); err != nil {
		return 0, err
	}
	tags := plugin.Tags
	if tags == nil {
		tags = []string{}
	}

	var pluginID int64
	if plugin.Namespace != "" {
		err = tx.QueryRow(ctx, `
			INSERT INTO plugins (namespace, name, description, author, category, repository, license, tags, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
			ON CONFLICT (namespace, name) WHERE namespace IS NOT NULL DO UPDATE SET
				description = EXCLUDED.description,
				author      = EXCLUDED.author,
				category    = EXCLUDED.category,
				repository  = EXCLUDED.repository,
				license     = EXCLUDED.license,
				tags        = EXCLUDED.tags,
				updated_at  = NOW()
			RETURNING id`,
			plugin.Namespace, plugin.Name, plugin.Description, plugin.Author, plugin.Category,
			plugin.Repository, plugin.License, tags,
		).Scan(&pluginID)
	} else {
		err = tx.QueryRow(ctx, `
			INSERT INTO plugins (name, description, author, category, repository, license, tags, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
			ON CONFLICT (name) WHERE namespace IS NULL DO UPDATE SET
				description = EXCLUDED.description,
				author      = EXCLUDED.author,
				category    = EXCLUDED.category,
				repository  = EXCLUDED.repository,
				license     = EXCLUDED.license,
				tags        = EXCLUDED.tags,
				updated_at  = NOW()
			RETURNING id`,
			plugin.Name, plugin.Description, plugin.Author, plugin.Category,
			plugin.Repository, plugin.License, tags,
		).Scan(&pluginID)
	}
	if err != nil {
		return 0, fmt.Errorf("upsert plugin %q: %w", plugin.Name, err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM plugin_aliases WHERE plugin_id = $1`, pluginID); err != nil {
		return 0, fmt.Errorf("replace aliases for plugin %q: %w", plugin.Name, err)
	}
	for _, alias := range plugin.Aliases {
		if _, err := tx.Exec(ctx,
			`INSERT INTO plugin_aliases (plugin_id, alias) VALUES ($1, $2)`,
			pluginID, alias,
		); err != nil {
			return 0, fmt.Errorf("replace alias %q for plugin %q: %w", alias, plugin.Name, err)
		}
	}

	for _, version := range plugin.Versions {
		var releaseDate *time.Time
		if version.ReleaseDate != "" {
			parsed, parseErr := time.Parse(time.RFC3339, version.ReleaseDate)
			if parseErr != nil {
				return 0, fmt.Errorf("parse release date for plugin %q version %q: %w",
					plugin.Name, version.Version, parseErr)
			}
			releaseDate = &parsed
		}

		var versionID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO plugin_versions
				(plugin_id, version, release_date, changelog, download_url, prerelease, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, NOW())
			ON CONFLICT (plugin_id, version) DO UPDATE SET
				release_date = EXCLUDED.release_date,
				changelog    = EXCLUDED.changelog,
				download_url = EXCLUDED.download_url,
				prerelease   = EXCLUDED.prerelease
			RETURNING id`,
			pluginID, version.Version, releaseDate, version.Changelog,
			version.DownloadURL, version.Prerelease,
		).Scan(&versionID); err != nil {
			return 0, fmt.Errorf("upsert plugin %q version %q: %w",
				plugin.Name, version.Version, err)
		}

		if _, err := tx.Exec(ctx,
			`DELETE FROM plugin_checksums WHERE version_id = $1`, versionID); err != nil {
			return 0, fmt.Errorf("replace checksums for plugin %q version %q: %w",
				plugin.Name, version.Version, err)
		}
		platforms := make([]string, 0, len(version.Checksums))
		for platform := range version.Checksums {
			platforms = append(platforms, platform)
		}
		sort.Strings(platforms)
		for _, platform := range platforms {
			if _, err := tx.Exec(ctx, `
				INSERT INTO plugin_checksums (version_id, platform, algorithm, hash)
				VALUES ($1, $2, 'sha256', $3)`,
				versionID, platform, version.Checksums[platform],
			); err != nil {
				return 0, fmt.Errorf("replace checksum %q for plugin %q version %q: %w",
					platform, plugin.Name, version.Version, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit plugin %q seed: %w", plugin.Name, err)
	}
	return pluginID, nil
}
