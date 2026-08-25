//go:build container

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/SemRels/semrel-registry/api/database"
	appErrors "github.com/SemRels/semrel-registry/api/internal"
	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/repository"
	"github.com/SemRels/semrel-registry/api/service"
	"github.com/SemRels/semrel-registry/api/testutil"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresProductionBehavior(t *testing.T) {
	dsn := testutil.DatabaseURL(t, ".")

	t.Run("migrates empty database to latest idempotently", func(t *testing.T) {
		testDSN := createDatabase(t, dsn, "migrate_latest")
		db, err := database.Connect(testDSN)
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })

		var schemaExists bool
		require.NoError(t, db.Pool().QueryRow(context.Background(),
			`SELECT to_regclass('public.plugins') IS NOT NULL`).Scan(&schemaExists))
		require.False(t, schemaExists)

		require.NoError(t, db.RunMigrations("database/migrations"))
		require.NoError(t, db.RunMigrations("database/migrations"))
		require.NoError(t, db.Pool().QueryRow(context.Background(),
			`SELECT to_regclass('public.plugins') IS NOT NULL`).Scan(&schemaExists))
		require.True(t, schemaExists)

		var version int
		var dirty bool
		require.NoError(t, db.Pool().QueryRow(context.Background(),
			`SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty))
		assert.EqualValues(t, latestMigrationVersion(t), version)
		assert.False(t, dirty)
	})

	t.Run("version eight startup keeps canonical metadata while merging production data", func(t *testing.T) {
		testDSN := createDatabase(t, dsn, "migration_metadata_precedence")
		migrateVersion(t, testDSN, 8)
		db, err := database.Connect(testDSN)
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		ctx := context.Background()

		_, err = db.Pool().Exec(ctx, `
			CREATE TABLE plugin_aliases (
			    plugin_id INT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
			    alias VARCHAR(356) NOT NULL,
			    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			    PRIMARY KEY (plugin_id, alias)
			)`)
		require.NoError(t, err)

		var legacyID, canonicalID int64
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugins
			    (namespace, name, description, author, category, repository,
			     license, status, tags, views, downloads, validation_checks, validated_at)
			VALUES
			    ('@semrel', 'conventional', 'legacy description', 'legacy author',
			     'analyzer', 'https://github.com/SemRels/analyzer-conventional',
			     'MIT', 'rejected', ARRAY['legacy', 'shared'], 17, 19,
			     '{"source":"legacy"}'::jsonb, TIMESTAMPTZ '2026-07-22 00:00:00+00')
			RETURNING id`).Scan(&legacyID))
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugins
			    (namespace, name, description, author, category, repository,
			     license, status, tags, views, downloads, validation_checks, validated_at)
			VALUES
			    ('@semrel', 'analyzer-conventional', 'canonical description',
			     'canonical author', 'analyzer',
			     'https://github.com/SemRels/analyzer-conventional',
			     'Apache-2.0', 'pending', ARRAY['canonical', 'shared'], 11, 13,
			     '{"source":"canonical"}'::jsonb, TIMESTAMPTZ '2026-07-21 00:00:00+00')
			RETURNING id`).Scan(&canonicalID))
		require.Greater(t, canonicalID, legacyID,
			"the exact canonical row must win even when it is not the oldest")

		_, err = db.Pool().Exec(ctx, `
			INSERT INTO plugin_aliases (plugin_id, alias) VALUES
			    ($1, 'legacy-existing-alias'),
			    ($2, 'canonical-existing-alias')`, legacyID, canonicalID)
		require.NoError(t, err)

		var legacyVersionID, uniqueVersionID, canonicalVersionID int64
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugin_versions
			    (plugin_id, version, release_date, changelog, download_url,
			     prerelease, views, downloads)
			VALUES
			    ($1, '1.0.0', TIMESTAMP '2026-07-01 00:00:00',
			     'shared release', 'https://example.invalid/analyzer/1.0.0',
			     FALSE, 11, 13)
			RETURNING id`, legacyID).Scan(&legacyVersionID))
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugin_versions
			    (plugin_id, version, release_date, changelog, download_url,
			     prerelease, views, downloads)
			VALUES
			    ($1, '2.0.0', TIMESTAMP '2026-07-02 00:00:00',
			     'legacy-only release', 'https://example.invalid/analyzer/2.0.0',
			     FALSE, 17, 19)
			RETURNING id`, legacyID).Scan(&uniqueVersionID))
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugin_versions
			    (plugin_id, version, release_date, changelog, download_url,
			     prerelease, views, downloads)
			VALUES
			    ($1, '1.0.0', TIMESTAMP '2026-07-01 00:00:00',
			     'shared release', 'https://example.invalid/analyzer/1.0.0',
			     FALSE, 5, 7)
			RETURNING id`, canonicalID).Scan(&canonicalVersionID))
		_, err = db.Pool().Exec(ctx, `
			INSERT INTO plugin_checksums (version_id, platform, algorithm, hash) VALUES
			    ($1, 'LINUX-AMD64', 'sha256', 'shared-hash'),
			    ($2, 'darwin-arm64', 'sha256', 'unique-hash'),
			    ($3, 'linux-amd64', 'sha256', 'shared-hash')`,
			legacyVersionID, uniqueVersionID, canonicalVersionID)
		require.NoError(t, err)

		_, err = db.Pool().Exec(ctx, `
			INSERT INTO metric_events (plugin_id, version_id, metric_type) VALUES
			    ($1, $2, 'download'),
			    ($3, $4, 'view')`,
			legacyID, legacyVersionID, canonicalID, canonicalVersionID)
		require.NoError(t, err)
		_, err = db.Pool().Exec(ctx, `
			INSERT INTO metric_daily_plugin (day, plugin_id, metric_type, count) VALUES
			    (CURRENT_DATE, $1, 'view', 3),
			    (CURRENT_DATE, $2, 'view', 2)`,
			legacyID, canonicalID)
		require.NoError(t, err)
		_, err = db.Pool().Exec(ctx, `
			INSERT INTO metric_daily_version
			    (day, plugin_id, version_id, metric_type, count) VALUES
			    (CURRENT_DATE, $1, $2, 'view', 6),
			    (CURRENT_DATE, $3, $4, 'view', 4),
			    (CURRENT_DATE, $1, $5, 'download', 8)`,
			legacyID, legacyVersionID, canonicalID, canonicalVersionID, uniqueVersionID)
		require.NoError(t, err)

		require.NoError(t, db.RunMigrations("database/migrations"))
		require.NoError(t, db.RunMigrations("database/migrations"),
			"repeat production startup must be idempotent")

		var version int
		var dirty bool
		require.NoError(t, db.Pool().QueryRow(ctx,
			`SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty))
		assert.EqualValues(t, latestMigrationVersion(t), version)
		assert.False(t, dirty)

		var retainedID, views, downloads int64
		var namespace, name, category, description, author, license, status string
		var tags []string
		var validation []byte
		var keptNewestValidation bool
		require.NoError(t, db.Pool().QueryRow(ctx, `
			SELECT id, namespace, name, category, description, author, license, status,
			       tags, views, downloads, validation_checks,
			       validated_at = TIMESTAMPTZ '2026-07-22 00:00:00+00'
			FROM plugins
			WHERE repository = 'https://github.com/SemRels/analyzer-conventional'`).
			Scan(&retainedID, &namespace, &name, &category, &description, &author,
				&license, &status, &tags, &views, &downloads, &validation,
				&keptNewestValidation))
		assert.Equal(t, canonicalID, retainedID)
		assert.Equal(t, "@semrel", namespace)
		assert.Equal(t, "analyzer-conventional", name)
		assert.Equal(t, "analyzer", category)
		assert.Equal(t, "canonical description", description)
		assert.Equal(t, "canonical author", author)
		assert.Equal(t, "Apache-2.0", license)
		assert.Equal(t, "pending", status)
		assert.ElementsMatch(t, []string{"canonical", "legacy", "shared"}, tags)
		assert.EqualValues(t, 28, views)
		assert.EqualValues(t, 32, downloads)
		assert.JSONEq(t, `{"source":"legacy"}`, string(validation))
		assert.True(t, keptNewestValidation)

		var aliases []string
		require.NoError(t, db.Pool().QueryRow(ctx, `
			SELECT ARRAY_AGG(alias ORDER BY alias)
			FROM plugin_aliases
			WHERE plugin_id = $1`, canonicalID).Scan(&aliases))
		assert.ElementsMatch(t, []string{
			"@semrel/conventional",
			"analyzer-conventional",
			"canonical-existing-alias",
			"conventional",
			"legacy-existing-alias",
		}, aliases)

		var pluginCount, versionCount, checksumCount, eventCount, eventVersionOwnerCount int
		require.NoError(t, db.Pool().QueryRow(ctx, `
			SELECT
			    (SELECT COUNT(*) FROM plugins
			     WHERE repository = 'https://github.com/SemRels/analyzer-conventional'),
			    (SELECT COUNT(*) FROM plugin_versions WHERE plugin_id = $1),
			    (SELECT COUNT(*) FROM plugin_checksums c
			     JOIN plugin_versions v ON v.id = c.version_id
			     WHERE v.plugin_id = $1),
			    (SELECT COUNT(*) FROM metric_events WHERE plugin_id = $1),
			    (SELECT COUNT(*) FROM metric_events e
			     JOIN plugin_versions v ON v.id = e.version_id
			     WHERE e.plugin_id = $1 AND v.plugin_id = $1)`,
			canonicalID).Scan(&pluginCount, &versionCount, &checksumCount, &eventCount,
			&eventVersionOwnerCount))
		assert.Equal(t, 1, pluginCount)
		assert.Equal(t, 2, versionCount)
		assert.Equal(t, 2, checksumCount)
		assert.Equal(t, 2, eventCount)
		assert.Equal(t, 2, eventVersionOwnerCount)

		var checksums []string
		require.NoError(t, db.Pool().QueryRow(ctx, `
			SELECT ARRAY_AGG(LOWER(c.platform) || ':' || c.hash
			                 ORDER BY LOWER(c.platform), c.hash)
			FROM plugin_checksums c
			JOIN plugin_versions v ON v.id = c.version_id
			WHERE v.plugin_id = $1`, canonicalID).Scan(&checksums))
		assert.Equal(t, []string{
			"darwin-arm64:unique-hash",
			"linux-amd64:shared-hash",
		}, checksums)

		var mergedVersionID, versionViews, versionDownloads int64
		require.NoError(t, db.Pool().QueryRow(ctx, `
			SELECT id, views, downloads
			FROM plugin_versions
			WHERE plugin_id = $1 AND version = '1.0.0'`, canonicalID).
			Scan(&mergedVersionID, &versionViews, &versionDownloads))
		assert.Equal(t, canonicalVersionID, mergedVersionID)
		assert.EqualValues(t, 16, versionViews)
		assert.EqualValues(t, 20, versionDownloads)
		var uniqueOwner int64
		require.NoError(t, db.Pool().QueryRow(ctx,
			`SELECT plugin_id FROM plugin_versions WHERE id = $1`, uniqueVersionID).
			Scan(&uniqueOwner))
		assert.Equal(t, canonicalID, uniqueOwner)

		var pluginDaily, mergedVersionDaily, uniqueVersionDaily int64
		require.NoError(t, db.Pool().QueryRow(ctx, `
			SELECT count FROM metric_daily_plugin
			WHERE plugin_id = $1 AND day = CURRENT_DATE AND metric_type = 'view'`,
			canonicalID).Scan(&pluginDaily))
		require.NoError(t, db.Pool().QueryRow(ctx, `
			SELECT count FROM metric_daily_version
			WHERE version_id = $1 AND day = CURRENT_DATE AND metric_type = 'view'`,
			canonicalVersionID).Scan(&mergedVersionDaily))
		require.NoError(t, db.Pool().QueryRow(ctx, `
			SELECT count FROM metric_daily_version
			WHERE version_id = $1 AND day = CURRENT_DATE AND metric_type = 'download'`,
			uniqueVersionID).Scan(&uniqueVersionDaily))
		assert.EqualValues(t, 5, pluginDaily)
		assert.EqualValues(t, 10, mergedVersionDaily)
		assert.EqualValues(t, 8, uniqueVersionDaily)
	})

	t.Run("version eight startup fills only empty canonical metadata", func(t *testing.T) {
		testDSN := createDatabase(t, dsn, "migration_metadata_fallback")
		migrateVersion(t, testDSN, 8)
		db, err := database.Connect(testDSN)
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		ctx := context.Background()

		var legacyID, canonicalID int64
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugins
			    (namespace, name, description, author, category, repository, license, status)
			VALUES
			    ('@semrel', 'conventional', 'fallback description', 'fallback author',
			     'analyzer', 'https://github.com/SemRels/analyzer-conventional',
			     'MIT', 'active')
			RETURNING id`).Scan(&legacyID))
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugins
			    (namespace, name, description, author, category, repository, license, status)
			VALUES
			    ('@semrel', 'analyzer-conventional', '', '', 'analyzer',
			     'https://github.com/SemRels/analyzer-conventional', '', 'pending')
			RETURNING id`).Scan(&canonicalID))
		require.Greater(t, canonicalID, legacyID)

		require.NoError(t, db.RunMigrations("database/migrations"))

		var retainedID int64
		var description, author, license, status string
		require.NoError(t, db.Pool().QueryRow(ctx, `
			SELECT id, description, author, license, status
			FROM plugins
			WHERE repository = 'https://github.com/SemRels/analyzer-conventional'`).
			Scan(&retainedID, &description, &author, &license, &status))
		assert.Equal(t, canonicalID, retainedID)
		assert.Equal(t, "fallback description", description)
		assert.Equal(t, "fallback author", author)
		assert.Equal(t, "MIT", license)
		assert.Equal(t, "pending", status)
	})

	t.Run("startup seed repairs partial catalog and is idempotent", func(t *testing.T) {
		testDSN := createDatabase(t, dsn, "seed_aliases")
		db := migratedDatabase(t, testDSN)
		repo := repository.NewPluginRepository(db)
		svc := service.NewPluginService(repo)
		ctx := context.Background()

		seedFile := filepath.Join(t.TempDir(), "plugins.json")
		payload := map[string]any{"plugins": []map[string]any{
			{
				"namespace":   "@semrel",
				"name":        "condition-generic",
				"aliases":     []string{"generic", "@semrel/generic", "condition-generic"},
				"description": "generic condition",
				"author":      "SemRels",
				"category":    "condition",
				"repository":  "https://github.com/SemRels/condition-generic",
				"license":     "Apache-2.0",
				"tags":        []string{"condition"},
				"versions": []map[string]any{{
					"version":     "1.2.3",
					"releaseDate": "2026-07-20T12:00:00Z",
					"downloadUrl": "https://example.test/condition-generic/1.2.3",
					"changelog":   "bundled release",
					"checksums": map[string]string{
						"linux-amd64": "generic-linux-hash",
					},
				}},
			},
			{
				"namespace":   "@semrel",
				"name":        "provider-fresh",
				"aliases":     []string{"fresh"},
				"description": "fresh bundled plugin",
				"category":    "provider",
				"repository":  "https://github.com/SemRels/provider-fresh",
				"versions": []map[string]any{{
					"version":     "2.0.0",
					"downloadUrl": "https://example.test/provider-fresh/2.0.0",
					"checksums": map[string]string{
						"darwin-arm64": "fresh-darwin-hash",
					},
				}},
			},
		}}
		data, err := json.Marshal(payload)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(seedFile, data, 0o600))

		// Simulate an interrupted older startup: one bundled plugin exists with
		// stale metadata and no versions, alongside unrelated community data.
		var partialID, communityID int64
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugins (namespace, name, description, category, repository)
			VALUES ('@semrel', 'condition-generic', 'partial', 'condition',
			        'https://github.com/SemRels/condition-generic')
			RETURNING id`).Scan(&partialID))
		_, err = db.Pool().Exec(ctx,
			`INSERT INTO plugin_aliases (plugin_id, alias) VALUES ($1, 'stale-alias')`, partialID)
		require.NoError(t, err)
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugins (namespace, name, description, category, repository)
			VALUES ('@community', 'user-plugin', 'keep me', 'provider',
			        'https://github.com/community/user-plugin')
			RETURNING id`).Scan(&communityID))

		require.NoError(t, seedPlugins(ctx, db.Pool(), seedFile))
		require.NoError(t, seedPlugins(ctx, db.Pool(), seedFile))

		for _, ref := range []string{"@semrel/condition-generic", "generic", "@semrel/generic"} {
			plugin, err := svc.GetPlugin(ctx, ref)
			require.NoError(t, err, ref)
			assert.Equal(t, "@semrel/condition-generic", plugin.Ref())
		}

		var pluginID int64
		var description, version, checksum string
		require.NoError(t, db.Pool().QueryRow(ctx, `
			SELECT p.id, p.description, v.version, c.hash
			FROM plugins p
			JOIN plugin_versions v ON v.plugin_id = p.id
			JOIN plugin_checksums c ON c.version_id = v.id
			WHERE p.namespace = '@semrel' AND p.name = 'condition-generic'
			  AND c.platform = 'linux-amd64'`).
			Scan(&pluginID, &description, &version, &checksum))
		assert.Equal(t, partialID, pluginID)
		assert.Equal(t, "generic condition", description)
		assert.Equal(t, "1.2.3", version)
		assert.Equal(t, "generic-linux-hash", checksum)

		var pluginCount, versionCount, checksumCount int
		require.NoError(t, db.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM plugins`).
			Scan(&pluginCount))
		require.NoError(t, db.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM plugin_versions`).
			Scan(&versionCount))
		require.NoError(t, db.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM plugin_checksums`).
			Scan(&checksumCount))
		assert.Equal(t, 3, pluginCount)
		assert.Equal(t, 2, versionCount)
		assert.Equal(t, 2, checksumCount)

		var communityDescription string
		require.NoError(t, db.Pool().QueryRow(ctx,
			`SELECT description FROM plugins WHERE id = $1`, communityID).
			Scan(&communityDescription))
		assert.Equal(t, "keep me", communityDescription)

		require.NoError(t, db.Close())
		reopened, err := database.Connect(testDSN)
		require.NoError(t, err)
		t.Cleanup(func() { _ = reopened.Close() })
		persisted, err := service.NewPluginService(repository.NewPluginRepository(reopened)).
			GetPlugin(ctx, "generic")
		require.NoError(t, err)
		assert.Equal(t, "@semrel/condition-generic", persisted.Ref())
	})

	t.Run("startup seed surfaces child failures and rolls back plugin", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			versions []map[string]any
			wantErr  string
		}{
			{
				name: "version",
				versions: []map[string]any{{
					"version": strings.Repeat("v", 51), "downloadUrl": "https://example.test/invalid",
				}},
				wantErr: "version",
			},
			{
				name: "checksum",
				versions: []map[string]any{
					{
						"version": "1.0.0", "changelog": "after version",
						"downloadUrl": "https://example.test/after",
						"checksums":   map[string]string{"linux-amd64": "after-hash"},
					},
					{
						"version": "2.0.0", "downloadUrl": "https://example.test/new",
						"checksums": map[string]string{strings.Repeat("p", 51): "invalid"},
					},
				},
				wantErr: "checksum",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testDSN := createDatabase(t, dsn, "seed_rollback_"+tc.name)
				db := migratedDatabase(t, testDSN)
				ctx := context.Background()

				var pluginID, versionID int64
				require.NoError(t, db.Pool().QueryRow(ctx, `
					INSERT INTO plugins (namespace, name, description, category, repository)
					VALUES ('@semrel', 'atomic-startup', 'before', 'provider',
					        'https://github.com/SemRels/atomic-startup')
					RETURNING id`).Scan(&pluginID))
				_, err := db.Pool().Exec(ctx,
					`INSERT INTO plugin_aliases (plugin_id, alias) VALUES ($1, 'old-alias')`, pluginID)
				require.NoError(t, err)
				require.NoError(t, db.Pool().QueryRow(ctx, `
					INSERT INTO plugin_versions (plugin_id, version, changelog, download_url)
					VALUES ($1, '1.0.0', 'before version', 'https://example.test/before')
					RETURNING id`, pluginID).Scan(&versionID))
				_, err = db.Pool().Exec(ctx, `
					INSERT INTO plugin_checksums (version_id, platform, algorithm, hash)
					VALUES ($1, 'linux-amd64', 'sha256', 'before-hash')`, versionID)
				require.NoError(t, err)

				seedFile := filepath.Join(t.TempDir(), "plugins.json")
				data, err := json.Marshal(map[string]any{"plugins": []map[string]any{{
					"namespace":   "@semrel",
					"name":        "atomic-startup",
					"aliases":     []string{"new-alias"},
					"description": "after",
					"category":    "provider",
					"repository":  "https://github.com/SemRels/atomic-startup",
					"versions":    tc.versions,
				}}})
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(seedFile, data, 0o600))

				seedErr := seedPlugins(ctx, db.Pool(), seedFile)
				require.ErrorContains(t, seedErr, "import plugin")
				require.ErrorContains(t, seedErr, tc.wantErr)

				var description, alias, changelog, downloadURL, hash string
				require.NoError(t, db.Pool().QueryRow(ctx, `
					SELECT p.description, a.alias, v.changelog, v.download_url, c.hash
					FROM plugins p
					JOIN plugin_aliases a ON a.plugin_id = p.id
					JOIN plugin_versions v ON v.plugin_id = p.id
					JOIN plugin_checksums c ON c.version_id = v.id
					WHERE p.id = $1`, pluginID).
					Scan(&description, &alias, &changelog, &downloadURL, &hash))
				assert.Equal(t, "before", description)
				assert.Equal(t, "old-alias", alias)
				assert.Equal(t, "before version", changelog)
				assert.Equal(t, "https://example.test/before", downloadURL)
				assert.Equal(t, "before-hash", hash)

				var versionCount int
				require.NoError(t, db.Pool().QueryRow(ctx,
					`SELECT COUNT(*) FROM plugin_versions WHERE plugin_id = $1`, pluginID).
					Scan(&versionCount))
				assert.Equal(t, 1, versionCount)
			})
		}
	})

	t.Run("dedup is persistent and idempotent", func(t *testing.T) {
		testDSN := createDatabase(t, dsn, "dedup")
		db := migratedDatabase(t, testDSN)
		ctx := context.Background()

		var canonicalID, legacyID int64
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugins (namespace, name, category, repository, status,
			                     views, downloads, tags)
			VALUES ('@semrel', 'provider-git', 'provider',
			        'https://github.com/SemRels/provider-git', 'active',
			        2, 3, ARRAY['canonical'])
			RETURNING id`).Scan(&canonicalID))
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugins (name, description, author, category, repository,
			                     license, status, tags, views, downloads,
			                     validation_checks, validated_at)
			VALUES ('legacy-provider-git', 'legacy description', 'legacy author',
			        'provider', 'https://github.com/SemRels/provider-git',
			        'Apache-2.0', 'active', ARRAY['legacy'], 5, 7,
			        '{"valid":true}'::jsonb, NOW())
			RETURNING id`).Scan(&legacyID))
		_, err := db.Pool().Exec(ctx,
			`INSERT INTO plugin_aliases (plugin_id, alias) VALUES ($1, 'git'), ($1, '@semrel/git'), ($1, 'provider-git')`,
			canonicalID)
		require.NoError(t, err)
		_, err = db.Pool().Exec(ctx,
			`INSERT INTO plugin_aliases (plugin_id, alias) VALUES ($1, 'legacy-git')`,
			legacyID)
		require.NoError(t, err)
		var canonicalVersionID, legacyVersionID, uniqueVersionID int64
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugin_versions
			    (plugin_id, version, download_url, views, downloads)
			VALUES ($1, '1.0.0', 'https://example.invalid/provider-git/1', 11, 13)
			RETURNING id`, canonicalID).Scan(&canonicalVersionID))
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugin_versions
			    (plugin_id, version, download_url, views, downloads)
			VALUES ($1, '1.0.0', 'https://example.invalid/provider-git/1', 17, 19)
			RETURNING id`, legacyID).Scan(&legacyVersionID))
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugin_versions (plugin_id, version, download_url)
			VALUES ($1, '2.0.0', 'https://example.invalid/provider-git/2')
			RETURNING id`, legacyID).Scan(&uniqueVersionID))
		_, err = db.Pool().Exec(ctx, `
			INSERT INTO plugin_checksums (version_id, platform, algorithm, hash)
			VALUES ($1, 'linux-amd64', 'sha256', 'shared-hash'),
			       ($2, 'LINUX-AMD64', 'sha256', 'shared-hash')`,
			canonicalVersionID, legacyVersionID)
		require.NoError(t, err)
		_, err = db.Pool().Exec(ctx, `
			INSERT INTO metric_events (plugin_id, version_id, metric_type)
			VALUES ($1, $2, 'view')`, legacyID, legacyVersionID)
		require.NoError(t, err)
		_, err = db.Pool().Exec(ctx, `
			INSERT INTO metric_daily_plugin (day, plugin_id, metric_type, count)
			VALUES (CURRENT_DATE, $1, 'view', 2), (CURRENT_DATE, $2, 'view', 3)`,
			canonicalID, legacyID)
		require.NoError(t, err)
		_, err = db.Pool().Exec(ctx, `
			INSERT INTO metric_daily_version
			    (day, plugin_id, version_id, metric_type, count)
			VALUES (CURRENT_DATE, $1, $2, 'view', 4),
			       (CURRENT_DATE, $3, $4, 'view', 6)`,
			canonicalID, canonicalVersionID, legacyID, legacyVersionID)
		require.NoError(t, err)

		deleted, normalized, err := db.CleanupSemrelDuplicates(ctx)
		require.NoError(t, err)
		assert.EqualValues(t, 1, deleted)
		assert.Zero(t, normalized)
		deleted, normalized, err = db.CleanupSemrelDuplicates(ctx)
		require.NoError(t, err)
		assert.Zero(t, deleted)
		assert.Zero(t, normalized)

		var pluginCount, versionCount int
		require.NoError(t, db.Pool().QueryRow(ctx,
			`SELECT COUNT(*) FROM plugins WHERE repository = 'https://github.com/SemRels/provider-git'`).
			Scan(&pluginCount))
		require.NoError(t, db.Pool().QueryRow(ctx,
			`SELECT COUNT(*) FROM plugin_versions WHERE plugin_id = $1`, canonicalID).
			Scan(&versionCount))
		assert.Equal(t, 1, pluginCount)
		assert.Equal(t, 2, versionCount)

		var views, downloads int64
		var description, author, license string
		var tags []string
		var checks []byte
		require.NoError(t, db.Pool().QueryRow(ctx, `
			SELECT views, downloads, description, author, license, tags, validation_checks
			FROM plugins WHERE id = $1`, canonicalID).
			Scan(&views, &downloads, &description, &author, &license, &tags, &checks))
		assert.EqualValues(t, 7, views)
		assert.EqualValues(t, 10, downloads)
		assert.Equal(t, "legacy description", description)
		assert.Equal(t, "legacy author", author)
		assert.Equal(t, "Apache-2.0", license)
		assert.ElementsMatch(t, []string{"canonical", "legacy"}, tags)
		assert.JSONEq(t, `{"valid":true}`, string(checks))

		var versionViews, versionDownloads, checksumCount int64
		require.NoError(t, db.Pool().QueryRow(ctx, `
			SELECT views, downloads,
			       (SELECT COUNT(*) FROM plugin_checksums WHERE version_id = v.id)
			FROM plugin_versions v
			WHERE plugin_id = $1 AND version = '1.0.0'`, canonicalID).
			Scan(&versionViews, &versionDownloads, &checksumCount))
		assert.EqualValues(t, 28, versionViews)
		assert.EqualValues(t, 32, versionDownloads)
		assert.EqualValues(t, 1, checksumCount)

		var eventPluginID, eventVersionID, pluginDaily, versionDaily int64
		require.NoError(t, db.Pool().QueryRow(ctx,
			`SELECT plugin_id, version_id FROM metric_events LIMIT 1`).
			Scan(&eventPluginID, &eventVersionID))
		assert.Equal(t, canonicalID, eventPluginID)
		assert.Equal(t, canonicalVersionID, eventVersionID)
		require.NoError(t, db.Pool().QueryRow(ctx, `
			SELECT count FROM metric_daily_plugin
			WHERE plugin_id = $1 AND day = CURRENT_DATE AND metric_type = 'view'`,
			canonicalID).Scan(&pluginDaily))
		assert.EqualValues(t, 5, pluginDaily)
		require.NoError(t, db.Pool().QueryRow(ctx, `
			SELECT count FROM metric_daily_version
			WHERE version_id = $1 AND day = CURRENT_DATE AND metric_type = 'view'`,
			canonicalVersionID).Scan(&versionDaily))
		assert.EqualValues(t, 10, versionDaily)
		var aliasOwner, uniqueOwner int64
		require.NoError(t, db.Pool().QueryRow(ctx,
			`SELECT plugin_id FROM plugin_aliases WHERE alias = 'legacy-git'`).
			Scan(&aliasOwner))
		require.NoError(t, db.Pool().QueryRow(ctx,
			`SELECT plugin_id FROM plugin_versions WHERE id = $1`, uniqueVersionID).
			Scan(&uniqueOwner))
		assert.Equal(t, canonicalID, aliasOwner)
		assert.Equal(t, canonicalID, uniqueOwner)
	})

	t.Run("cleanup installs shared canonical aliases atomically", func(t *testing.T) {
		testDSN := createDatabase(t, dsn, "cleanup_aliases")
		db := migratedDatabase(t, testDSN)
		ctx := context.Background()

		_, err := db.Pool().Exec(ctx, `
			INSERT INTO plugins (namespace, name, category, repository) VALUES
			('@legacy', 'npm-updater', 'updater', 'https://github.com/SemRels/updater-npm'),
			('@legacy', 'npm-publisher', 'publisher', 'https://github.com/SemRels/publisher-npm'),
			('@legacy', 'changelog', 'generator', 'https://github.com/SemRels/generator-changelog-md'),
			('@community', 'bare-repository', 'updater', 'updater-npm'),
			('@community', 'unknown-semrels-repository', 'updater',
			 'https://github.com/SemRels/updater-community')`)
		require.NoError(t, err)

		deleted, normalized, err := db.CleanupSemrelDuplicates(ctx)
		require.NoError(t, err)
		assert.Zero(t, deleted)
		assert.EqualValues(t, 3, normalized)

		svc := service.NewPluginService(repository.NewPluginRepository(db))
		for ref, expected := range map[string]string{
			"npm":                            "@semrel/updater-npm",
			"@semrel/npm":                    "@semrel/updater-npm",
			"updater-npm":                    "@semrel/updater-npm",
			"publisher-npm":                  "@semrel/publisher-npm",
			"@semrel/publisher-npm":          "@semrel/publisher-npm",
			"changelog-md":                   "@semrel/generator-changelog-md",
			"@semrel/changelog-md":           "@semrel/generator-changelog-md",
			"generator-changelog-md":         "@semrel/generator-changelog-md",
			"@semrel/generator-changelog-md": "@semrel/generator-changelog-md",
		} {
			plugin, getErr := svc.GetPlugin(ctx, ref)
			require.NoError(t, getErr, ref)
			assert.Equal(t, expected, plugin.Ref(), ref)
		}
		_, err = svc.GetPlugin(ctx, "@semrel/npm-publisher")
		require.Error(t, err)
		for _, name := range []string{"bare-repository", "unknown-semrels-repository"} {
			var namespace string
			require.NoError(t, db.Pool().QueryRow(ctx,
				`SELECT namespace FROM plugins WHERE name = $1`, name).Scan(&namespace))
			assert.Equal(t, "@community", namespace)
		}
	})

	t.Run("cleanup alias collision rolls back normalization", func(t *testing.T) {
		testDSN := createDatabase(t, dsn, "cleanup_collision")
		db := migratedDatabase(t, testDSN)
		ctx := context.Background()

		var communityID int64
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugins (name, category, repository)
			VALUES ('community', 'updater', 'https://github.com/example/community')
			RETURNING id`).Scan(&communityID))
		_, err := db.Pool().Exec(ctx,
			`INSERT INTO plugin_aliases (plugin_id, alias) VALUES ($1, 'npm')`, communityID)
		require.NoError(t, err)
		var updaterID int64
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugins (namespace, name, category, repository)
			VALUES ('@legacy', 'legacy-updater', 'updater',
			        'https://github.com/SemRels/updater-npm')
			RETURNING id`).Scan(&updaterID))

		_, _, err = db.CleanupSemrelDuplicates(ctx)
		require.ErrorContains(t, err, "already owned by another plugin")

		var namespace, name string
		require.NoError(t, db.Pool().QueryRow(ctx,
			`SELECT namespace, name FROM plugins WHERE id = $1`, updaterID).
			Scan(&namespace, &name))
		assert.Equal(t, "@legacy", namespace)
		assert.Equal(t, "legacy-updater", name)
		var aliases int
		require.NoError(t, db.Pool().QueryRow(ctx,
			`SELECT COUNT(*) FROM plugin_aliases WHERE plugin_id = $1`, updaterID).
			Scan(&aliases))
		assert.Zero(t, aliases)
	})

	t.Run("soft delete releases all claims and restore reclaims them", func(t *testing.T) {
		testDSN := createDatabase(t, dsn, "soft_delete_claims")
		db := migratedDatabase(t, testDSN)
		ctx := context.Background()
		repo := repository.NewPluginRepository(db)

		var originalID int64
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugins (name, category, repository)
			VALUES ('original-plugin', 'hook', 'https://github.com/example/original')
			RETURNING id`).Scan(&originalID))
		_, err := db.Pool().Exec(ctx, `
			INSERT INTO plugin_aliases (plugin_id, alias)
			VALUES ($1, 'reusable-alias'), ($1, '@example/reusable-alias')`, originalID)
		require.NoError(t, err)
		require.NoError(t, repo.Delete(ctx, models.PluginDeletionSpec{PluginID: originalID, CascadeVersions: true}))

		var claims int
		require.NoError(t, db.Pool().QueryRow(ctx,
			`SELECT COUNT(*) FROM plugin_identity_claims WHERE plugin_id = $1`, originalID).
			Scan(&claims))
		assert.Zero(t, claims)

		var replacementID int64
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugins (name, category, repository)
			VALUES ('replacement-plugin', 'hook', 'https://github.com/example/replacement')
			RETURNING id`).Scan(&replacementID))
		_, err = db.Pool().Exec(ctx, `
			INSERT INTO plugin_aliases (plugin_id, alias)
			VALUES ($1, 'reusable-alias')`, replacementID)
		require.NoError(t, err)

		_, err = db.Pool().Exec(ctx,
			`UPDATE plugins SET deleted_at = NULL WHERE id = $1`, originalID)
		require.ErrorContains(t, err, "already owned")
		var deleted bool
		require.NoError(t, db.Pool().QueryRow(ctx,
			`SELECT deleted_at IS NOT NULL FROM plugins WHERE id = $1`, originalID).
			Scan(&deleted))
		assert.True(t, deleted)

		require.NoError(t, repo.Delete(ctx, models.PluginDeletionSpec{PluginID: replacementID, CascadeVersions: true}))
		_, err = db.Pool().Exec(ctx,
			`UPDATE plugins SET deleted_at = NULL WHERE id = $1`, originalID)
		require.NoError(t, err)
		require.NoError(t, db.Pool().QueryRow(ctx,
			`SELECT COUNT(*) FROM plugin_identity_claims WHERE plugin_id = $1`, originalID).
			Scan(&claims))
		assert.Equal(t, 3, claims)
	})

	t.Run("cleanup rejects differing duplicate version metadata atomically", func(t *testing.T) {
		testDSN := createDatabase(t, dsn, "cleanup_version_collision")
		db := migratedDatabase(t, testDSN)
		ctx := context.Background()

		var canonicalID, duplicateID int64
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugins (namespace, name, category, repository, views)
			VALUES ('@semrel', 'provider-git', 'provider',
			        'https://github.com/SemRels/provider-git', 2)
			RETURNING id`).Scan(&canonicalID))
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugins (namespace, name, category, repository, views)
			VALUES ('@legacy', 'legacy-git', 'provider',
			        'https://github.com/SemRels/provider-git', 5)
			RETURNING id`).Scan(&duplicateID))
		_, err := db.Pool().Exec(ctx, `
			INSERT INTO plugin_versions (plugin_id, version, download_url) VALUES
			($1, '1.0.0', 'https://example.invalid/canonical'),
			($2, '1.0.0', 'https://example.invalid/different')`,
			canonicalID, duplicateID)
		require.NoError(t, err)

		_, _, err = db.CleanupSemrelDuplicates(ctx)
		require.ErrorContains(t, err, "version metadata differs")

		var pluginCount int
		var totalViews int64
		require.NoError(t, db.Pool().QueryRow(ctx, `
			SELECT COUNT(*), SUM(views)
			FROM plugins
			WHERE repository = 'https://github.com/SemRels/provider-git'`).
			Scan(&pluginCount, &totalViews))
		assert.Equal(t, 2, pluginCount)
		assert.EqualValues(t, 7, totalViews)
	})

	t.Run("cleanup rejects differing duplicate checksums atomically", func(t *testing.T) {
		testDSN := createDatabase(t, dsn, "cleanup_checksum_collision")
		db := migratedDatabase(t, testDSN)
		ctx := context.Background()

		var canonicalID, duplicateID, canonicalVersionID, duplicateVersionID int64
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugins (namespace, name, category, repository)
			VALUES ('@semrel', 'provider-git', 'provider',
			        'https://github.com/SemRels/provider-git')
			RETURNING id`).Scan(&canonicalID))
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugins (namespace, name, category, repository)
			VALUES ('@legacy', 'legacy-git', 'provider',
			        'https://github.com/SemRels/provider-git')
			RETURNING id`).Scan(&duplicateID))
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugin_versions (plugin_id, version, download_url)
			VALUES ($1, '1.0.0', 'https://example.invalid/shared')
			RETURNING id`, canonicalID).Scan(&canonicalVersionID))
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugin_versions (plugin_id, version, download_url)
			VALUES ($1, '1.0.0', 'https://example.invalid/shared')
			RETURNING id`, duplicateID).Scan(&duplicateVersionID))
		_, err := db.Pool().Exec(ctx, `
			INSERT INTO plugin_checksums (version_id, platform, algorithm, hash) VALUES
			($1, 'linux-amd64', 'sha256', 'canonical-hash'),
			($2, 'LINUX-AMD64', 'sha256', 'different-hash')`,
			canonicalVersionID, duplicateVersionID)
		require.NoError(t, err)

		_, _, err = db.CleanupSemrelDuplicates(ctx)
		require.ErrorContains(t, err, "checksums differ")
		var pluginCount int
		require.NoError(t, db.Pool().QueryRow(ctx, `
			SELECT COUNT(*) FROM plugins
			WHERE repository = 'https://github.com/SemRels/provider-git'`).
			Scan(&pluginCount))
		assert.Equal(t, 2, pluginCount)
	})

	t.Run("cleanup uses canonical plugin metadata precedence", func(t *testing.T) {
		testDSN := createDatabase(t, dsn, "cleanup_metadata_precedence")
		db := migratedDatabase(t, testDSN)
		ctx := context.Background()

		var legacyID, canonicalID int64
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugins
			    (namespace, name, description, author, category, repository,
			     license, status, tags, views, downloads)
			VALUES
			    ('@semrel', 'git', 'legacy description', 'legacy author', 'provider',
			     'https://github.com/SemRels/provider-git', 'MIT', 'rejected',
			     ARRAY['legacy'], 5, 7)
			RETURNING id`).Scan(&legacyID))
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugins
			    (namespace, name, description, author, category, repository,
			     license, status, tags, views, downloads)
			VALUES
			    ('@semrel', 'provider-git', 'canonical description',
			     'canonical author', 'provider',
			     'https://github.com/SemRels/provider-git', 'Apache-2.0',
			     'pending', ARRAY['canonical'], 2, 3)
			RETURNING id`).Scan(&canonicalID))
		require.Greater(t, canonicalID, legacyID)

		deleted, normalized, err := db.CleanupSemrelDuplicates(ctx)
		require.NoError(t, err)
		assert.EqualValues(t, 1, deleted)
		assert.Zero(t, normalized)
		deleted, normalized, err = db.CleanupSemrelDuplicates(ctx)
		require.NoError(t, err)
		assert.Zero(t, deleted)
		assert.Zero(t, normalized)

		var retainedID, views, downloads int64
		var description, author, license, status string
		var tags []string
		require.NoError(t, db.Pool().QueryRow(ctx, `
			SELECT id, description, author, license, status, tags, views, downloads
			FROM plugins
			WHERE repository = 'https://github.com/SemRels/provider-git'`).
			Scan(&retainedID, &description, &author, &license, &status, &tags,
				&views, &downloads))
		assert.Equal(t, canonicalID, retainedID)
		assert.Equal(t, "canonical description", description)
		assert.Equal(t, "canonical author", author)
		assert.Equal(t, "Apache-2.0", license)
		assert.Equal(t, "pending", status)
		assert.ElementsMatch(t, []string{"canonical", "legacy"}, tags)
		assert.EqualValues(t, 7, views)
		assert.EqualValues(t, 10, downloads)
	})

	t.Run("concurrent canonical create has one durable winner", func(t *testing.T) {
		testDSN := createDatabase(t, dsn, "concurrency")
		db := migratedDatabase(t, testDSN)
		svc := service.NewPluginService(repository.NewPluginRepository(db))

		const workers = 8
		results := make(chan error, workers)
		var wg sync.WaitGroup
		wg.Add(workers)
		for range workers {
			go func() {
				defer wg.Done()
				_, err := svc.CreatePlugin(context.Background(), models.Plugin{
					Namespace:  "@semrel",
					Name:       "hook-teams",
					Category:   "hook",
					Repository: "https://github.com/SemRels/hook-teams",
					Status:     models.StatusActive,
				})
				results <- err
			}()
		}
		wg.Wait()
		close(results)

		successes, duplicates := 0, 0
		for err := range results {
			if err == nil {
				successes++
				continue
			}
			require.True(t, errors.Is(err, appErrors.ErrDuplicatePlugin),
				"concurrent create returned unexpected error: %v", err)
			duplicates++
		}
		assert.Equal(t, 1, successes)
		assert.Equal(t, workers-1, duplicates)

		var count int
		require.NoError(t, db.Pool().QueryRow(context.Background(),
			`SELECT COUNT(*) FROM plugins WHERE namespace = '@semrel' AND name = 'hook-teams'`).
			Scan(&count))
		assert.Equal(t, 1, count)
	})

	t.Run("concurrent alias and canonical identity have one claim winner", func(t *testing.T) {
		testDSN := createDatabase(t, dsn, "identity_claim_race")
		db := migratedDatabase(t, testDSN)
		ctx := context.Background()

		var aliasPluginID, canonicalPluginID int64
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugins (name, category, repository)
			VALUES ('alias-owner', 'hook', 'https://github.com/example/alias-owner')
			RETURNING id`).Scan(&aliasPluginID))
		require.NoError(t, db.Pool().QueryRow(ctx, `
			INSERT INTO plugins (name, category, repository)
			VALUES ('canonical-before-race', 'hook',
			        'https://github.com/example/canonical-owner')
			RETURNING id`).Scan(&canonicalPluginID))

		start := make(chan struct{})
		results := make(chan error, 2)
		var raceWG sync.WaitGroup
		raceWG.Add(2)
		go func() {
			defer raceWG.Done()
			<-start
			tx, beginErr := db.Pool().Begin(ctx)
			if beginErr != nil {
				results <- beginErr
				return
			}
			defer func() { _ = tx.Rollback(ctx) }()
			if _, execErr := tx.Exec(ctx,
				`INSERT INTO plugin_aliases (plugin_id, alias) VALUES ($1, 'shared-race-ref')`,
				aliasPluginID); execErr != nil {
				results <- execErr
				return
			}
			results <- tx.Commit(ctx)
		}()
		go func() {
			defer raceWG.Done()
			<-start
			tx, beginErr := db.Pool().Begin(ctx)
			if beginErr != nil {
				results <- beginErr
				return
			}
			defer func() { _ = tx.Rollback(ctx) }()
			if _, execErr := tx.Exec(ctx,
				`UPDATE plugins SET name = 'shared-race-ref' WHERE id = $1`,
				canonicalPluginID); execErr != nil {
				results <- execErr
				return
			}
			results <- tx.Commit(ctx)
		}()
		close(start)
		raceWG.Wait()
		close(results)

		successes, conflicts := 0, 0
		for result := range results {
			if result == nil {
				successes++
			} else {
				conflicts++
			}
		}
		assert.Equal(t, 1, successes)
		assert.Equal(t, 1, conflicts)

		var ownerID int64
		require.NoError(t, db.Pool().QueryRow(ctx, `
			SELECT plugin_id
			FROM plugin_identity_claims
			WHERE normalized_ref = 'shared-race-ref'`).Scan(&ownerID))
		assert.Contains(t, []int64{aliasPluginID, canonicalPluginID}, ownerID)
		var durableOwners int
		require.NoError(t, db.Pool().QueryRow(ctx, `
			SELECT
			    (SELECT COUNT(*) FROM plugin_aliases
			     WHERE LOWER(alias) = 'shared-race-ref')
			  + (SELECT COUNT(*) FROM plugins
			     WHERE deleted_at IS NULL
			       AND LOWER(CASE WHEN namespace IS NULL OR namespace = ''
			                      THEN name ELSE namespace || '/' || name END)
			           = 'shared-race-ref')`).Scan(&durableOwners))
		assert.Equal(t, 1, durableOwners)
	})

	t.Run("canonical migration collision fails without advancing", func(t *testing.T) {
		testDSN := createDatabase(t, dsn, "fail_fast")
		migrateVersion(t, testDSN, 8)
		pool, err := pgxpool.New(context.Background(), testDSN)
		require.NoError(t, err)
		_, err = pool.Exec(context.Background(), `
			INSERT INTO plugins (namespace, name, category, repository) VALUES
			('@semrel', 'npm', 'updater', 'https://github.com/SemRels/updater-npm'),
			('@semrel', 'updater-npm', 'updater', 'https://github.com/example/unrelated')`)
		require.NoError(t, err)
		pool.Close()

		db, err := database.Connect(testDSN)
		require.NoError(t, err)
		defer db.Close()
		err = db.RunMigrations("database/migrations")
		require.ErrorContains(t, err, "canonical identity is already occupied")
		source, err := filepath.Abs("database/migrations")
		require.NoError(t, err)
		migrator, err := migrate.New("file://"+filepath.ToSlash(source), testDSN)
		require.NoError(t, err)
		version, dirty, versionErr := migrator.Version()
		require.NoError(t, versionErr)
		assert.EqualValues(t, 8, version)
		assert.False(t, dirty)
		_, _ = migrator.Close()

		_, err = db.Pool().Exec(context.Background(),
			`DELETE FROM plugins WHERE repository = 'https://github.com/example/unrelated'`)
		require.NoError(t, err)
		require.NoError(t, db.RunMigrations("database/migrations"))
		migrator, err = migrate.New("file://"+filepath.ToSlash(source), testDSN)
		require.NoError(t, err)
		version, dirty, versionErr = migrator.Version()
		require.NoError(t, versionErr)
		assert.EqualValues(t, latestMigrationVersion(t), version)
		assert.False(t, dirty)
		_, _ = migrator.Close()
	})
}

func createDatabase(t *testing.T, adminDSN, name string) string {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), adminDSN)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), "CREATE DATABASE "+name)
	require.NoError(t, err)
	pool.Close()

	parts := strings.SplitN(adminDSN, "?", 2)
	base := parts[0]
	base = base[:strings.LastIndex(base, "/")+1] + name
	if len(parts) == 2 {
		base += "?" + parts[1]
	}
	return base
}

func migratedDatabase(t *testing.T, dsn string) *database.Database {
	t.Helper()
	db, err := database.Connect(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.RunMigrations("database/migrations"))
	return db
}

func migrateVersion(t *testing.T, dsn string, version uint) {
	t.Helper()
	source, err := filepath.Abs("database/migrations")
	require.NoError(t, err)
	migrator, err := migrate.New("file://"+filepath.ToSlash(source), dsn)
	require.NoError(t, err)
	defer migrator.Close()
	require.NoError(t, migrator.Migrate(version), fmt.Sprintf("migrate to %d", version))
}

func latestMigrationVersion(t *testing.T) uint {
	t.Helper()
	entries, err := os.ReadDir("database/migrations")
	require.NoError(t, err)

	var latest uint
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		prefix := strings.SplitN(entry.Name(), "_", 2)[0]
		version, err := strconv.ParseUint(prefix, 10, 32)
		require.NoError(t, err, entry.Name())
		if uint(version) > latest {
			latest = uint(version)
		}
	}
	require.NotZero(t, latest)
	return latest
}
