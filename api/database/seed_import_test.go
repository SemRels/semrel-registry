//go:build container

package database

import (
	"context"
	"strings"
	"testing"

	"github.com/SemRels/semrel-registry/api/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpsertSeedPluginRollsBackWholePluginOnChildFailure(t *testing.T) {
	dsn := testutil.DatabaseURL(t, "..")
	db, err := Connect(dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.RunMigrations("migrations"))

	ctx := context.Background()
	var pluginID int64
	require.NoError(t, db.Pool().QueryRow(ctx, `
		INSERT INTO plugins (namespace, name, description, category, repository)
		VALUES ('@community', 'atomic-seed', 'before', 'provider',
		        'https://github.com/community/atomic-seed')
		RETURNING id`).Scan(&pluginID))
	_, err = db.Pool().Exec(ctx,
		`INSERT INTO plugin_aliases (plugin_id, alias) VALUES ($1, 'old-alias')`, pluginID)
	require.NoError(t, err)
	var oldVersionID int64
	require.NoError(t, db.Pool().QueryRow(ctx, `
		INSERT INTO plugin_versions (plugin_id, version, changelog, download_url)
		VALUES ($1, '1.0.0', 'before version', 'https://example.test/before')
		RETURNING id`, pluginID).Scan(&oldVersionID))
	_, err = db.Pool().Exec(ctx, `
		INSERT INTO plugin_checksums (version_id, platform, algorithm, hash)
		VALUES ($1, 'linux-amd64', 'sha256', 'before-hash')`, oldVersionID)
	require.NoError(t, err)

	base := SeedPlugin{
		Namespace:   "@community",
		Name:        "atomic-seed",
		Description: "after",
		Category:    "provider",
		Repository:  "https://github.com/community/atomic-seed",
		Aliases:     []string{"new-alias"},
	}

	assertUnchanged := func(t *testing.T) {
		t.Helper()
		var description string
		var aliases []string
		require.NoError(t, db.Pool().QueryRow(ctx, `
			SELECT p.description, ARRAY_AGG(a.alias ORDER BY a.alias)
			FROM plugins p
			JOIN plugin_aliases a ON a.plugin_id = p.id
			WHERE p.id = $1
			GROUP BY p.id`, pluginID).Scan(&description, &aliases))
		assert.Equal(t, "before", description)
		assert.Equal(t, []string{"old-alias"}, aliases)

		var version, changelog, downloadURL, platform, hash string
		require.NoError(t, db.Pool().QueryRow(ctx, `
			SELECT v.version, v.changelog, v.download_url, c.platform, c.hash
			FROM plugin_versions v
			JOIN plugin_checksums c ON c.version_id = v.id
			WHERE v.plugin_id = $1`, pluginID).
			Scan(&version, &changelog, &downloadURL, &platform, &hash))
		assert.Equal(t, "1.0.0", version)
		assert.Equal(t, "before version", changelog)
		assert.Equal(t, "https://example.test/before", downloadURL)
		assert.Equal(t, "linux-amd64", platform)
		assert.Equal(t, "before-hash", hash)

		var versionCount int
		require.NoError(t, db.Pool().QueryRow(ctx,
			`SELECT COUNT(*) FROM plugin_versions WHERE plugin_id = $1`, pluginID).
			Scan(&versionCount))
		assert.Equal(t, 1, versionCount)
	}

	t.Run("version failure", func(t *testing.T) {
		seed := base
		seed.Versions = []SeedPluginVersion{{
			Version: strings.Repeat("v", 51), DownloadURL: "https://example.test/invalid",
		}}
		_, seedErr := UpsertSeedPlugin(ctx, db.Pool(), seed)
		require.ErrorContains(t, seedErr, "version")
		assertUnchanged(t)
	})

	t.Run("checksum failure after version updates", func(t *testing.T) {
		seed := base
		seed.Versions = []SeedPluginVersion{
			{
				Version: "1.0.0", Changelog: "after version",
				DownloadURL: "https://example.test/after",
				Checksums:   map[string]string{"linux-amd64": "after-hash"},
			},
			{
				Version: "2.0.0", DownloadURL: "https://example.test/new",
				Checksums: map[string]string{strings.Repeat("p", 51): "invalid"},
			},
		}
		_, seedErr := UpsertSeedPlugin(ctx, db.Pool(), seed)
		require.ErrorContains(t, seedErr, "checksum")
		assertUnchanged(t)
	})
}
