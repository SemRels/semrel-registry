//go:build container

package database

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SemRels/semrel-registry/api/naming"
	"github.com/SemRels/semrel-registry/api/testutil"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanonicalFirstPartyMigrationUpgradesExistingRows(t *testing.T) {
	dsn := testutil.DatabaseURL(t, "..")
	migrateToVersion(t, dsn, 8)

	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	var historicalID, duplicateID int64
	err = pool.QueryRow(context.Background(), `
		INSERT INTO plugins (namespace, name, category, repository, description, views)
		VALUES ('@semrel', 'npm', 'updater', 'https://github.com/SemRels/updater-npm',
		        'historical row', 3)
		RETURNING id`).Scan(&historicalID)
	require.NoError(t, err)
	err = pool.QueryRow(context.Background(), `
		INSERT INTO plugins (namespace, name, category, repository, downloads)
		VALUES (NULL, 'updater-npm', 'updater', 'https://github.com/SemRels/updater-npm', 5)
		RETURNING id`).Scan(&duplicateID)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `
		INSERT INTO plugin_versions (plugin_id, version, download_url, views) VALUES
		($1, '1.0.0', 'https://example.invalid/npm/1.0.0', 2),
		($2, '1.0.0', 'https://example.invalid/npm/1.0.0', 3),
		($2, '2.0.0', 'https://example.invalid/npm/2.0.0', 4)`,
		historicalID, duplicateID)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `
		INSERT INTO plugin_checksums (version_id, platform, algorithm, hash)
		SELECT id, 'linux-amd64', 'sha256', 'historical-checksum'
		FROM plugin_versions
		WHERE plugin_id = $1 AND version = '1.0.0'`, duplicateID)
	require.NoError(t, err)

	migrator := newMigrator(t, dsn)
	require.NoError(t, migrator.Up())
	version, dirty, err := migrator.Version()
	require.NoError(t, err)
	assert.EqualValues(t, 9, version)
	assert.False(t, dirty)
	_, _ = migrator.Close()

	db, err := Connect(dsn)
	require.NoError(t, err)
	defer db.Close()
	pool.Close()

	var namespace, name, category string
	var aliases []string
	err = db.Pool().QueryRow(context.Background(), `
		SELECT p.namespace, p.name, p.category, ARRAY_AGG(a.alias ORDER BY a.alias)
		FROM plugins p JOIN plugin_aliases a ON a.plugin_id = p.id
		WHERE p.repository = 'https://github.com/SemRels/updater-npm'
		GROUP BY p.id`).Scan(&namespace, &name, &category, &aliases)
	require.NoError(t, err)
	assert.Equal(t, "@semrel", namespace)
	assert.Equal(t, "updater-npm", name)
	assert.Equal(t, "updater", category)
	assert.ElementsMatch(t, []string{"@semrel/npm", "npm", "updater-npm"}, aliases)

	var pluginCount, versionCount, pluginViews, pluginDownloads, versionViews, checksumCount int
	err = db.Pool().QueryRow(context.Background(), `
		SELECT COUNT(*), SUM(views), SUM(downloads)
		FROM plugins
		WHERE repository = 'https://github.com/SemRels/updater-npm'`).
		Scan(&pluginCount, &pluginViews, &pluginDownloads)
	require.NoError(t, err)
	assert.Equal(t, 1, pluginCount)
	assert.Equal(t, 3, pluginViews)
	assert.Equal(t, 5, pluginDownloads)
	err = db.Pool().QueryRow(context.Background(), `
		SELECT COUNT(*), SUM(views)
		FROM plugin_versions
		WHERE plugin_id = (
			SELECT id FROM plugins
			WHERE repository = 'https://github.com/SemRels/updater-npm'
		)`).Scan(&versionCount, &versionViews)
	require.NoError(t, err)
	assert.Equal(t, 2, versionCount)
	assert.Equal(t, 9, versionViews)
	err = db.Pool().QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM plugin_checksums c
		JOIN plugin_versions v ON v.id = c.version_id
		JOIN plugins p ON p.id = v.plugin_id
		WHERE p.repository = 'https://github.com/SemRels/updater-npm'`).
		Scan(&checksumCount)
	require.NoError(t, err)
	assert.Equal(t, 1, checksumCount)

	for _, alias := range []string{"npm", "@semrel/npm", "updater-npm"} {
		var resolved string
		err = db.Pool().QueryRow(context.Background(), `
			SELECT p.namespace || '/' || p.name
			FROM plugins p
			JOIN plugin_aliases a ON a.plugin_id = p.id
			WHERE LOWER(a.alias) = LOWER($1)`, alias).Scan(&resolved)
		require.NoError(t, err, alias)
		assert.Equal(t, "@semrel/updater-npm", resolved)
	}

	var communityID int64
	err = db.Pool().QueryRow(context.Background(), `
		INSERT INTO plugins (name, category, repository)
		VALUES ('community-npm', 'updater', 'https://github.com/example/community-npm')
		RETURNING id`).Scan(&communityID)
	require.NoError(t, err)
	_, err = db.Pool().Exec(context.Background(),
		`INSERT INTO plugin_aliases (plugin_id, alias) VALUES ($1, 'NPM')`, communityID)
	require.Error(t, err)

	require.NoError(t, db.RunMigrations("migrations"))
}

func TestCanonicalMigrationRecoversDirtyVersionNineWithHistoricalURL(t *testing.T) {
	adminDSN := testutil.DatabaseURL(t, "..")
	admin, err := pgxpool.New(context.Background(), adminDSN)
	require.NoError(t, err)
	_, err = admin.Exec(context.Background(), `CREATE DATABASE semrel_dirty_v9_recovery`)
	require.NoError(t, err)
	admin.Close()

	dsn := strings.Replace(adminDSN, "/semrel_registry?", "/semrel_dirty_v9_recovery?", 1)
	migrateToVersion(t, dsn, 8)
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	defer pool.Close()

	_, err = pool.Exec(context.Background(), `
		INSERT INTO plugins (namespace, name, category, repository, description) VALUES
		('@semrel', 'gitea-actions', 'condition',
		 'https://github.com/SemRels/condition-gitea-actions', 'legacy'),
		('@semrel', 'condition-gitea-actions', 'condition',
		 'https://github.com/SemRels/condition-gitea-actions/', 'canonical')`)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(),
		`UPDATE schema_migrations SET version = 9, dirty = TRUE`)
	require.NoError(t, err)

	db, err := Connect(dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.RunMigrations("migrations"))
	require.NoError(t, db.RunMigrations("migrations"))

	var version int
	var dirty bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty))
	assert.Equal(t, 9, version)
	assert.False(t, dirty)

	var count int
	var name, description string
	require.NoError(t, pool.QueryRow(context.Background(), `
		SELECT COUNT(*), MIN(name), MIN(description)
		FROM plugins
		WHERE LOWER(RTRIM(repository, '/')) =
		      'https://github.com/semrels/condition-gitea-actions'`).
		Scan(&count, &name, &description))
	assert.Equal(t, 1, count)
	assert.Equal(t, "condition-gitea-actions", name)
	assert.Equal(t, "canonical", description)
}

func TestCanonicalFirstPartyMigrationDetectsIrreconcilableIdentityCollision(t *testing.T) {
	dsn := testutil.DatabaseURL(t, "..")
	admin, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	_, err = admin.Exec(context.Background(), `CREATE DATABASE semrel_collision`)
	require.NoError(t, err)
	admin.Close()

	collisionDSN := strings.Replace(dsn, "/semrel_registry?", "/semrel_collision?", 1)
	migrateToVersion(t, collisionDSN, 8)
	pool, err := pgxpool.New(context.Background(), collisionDSN)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `
		INSERT INTO plugins (namespace, name, category, repository) VALUES
		('@semrel', 'npm', 'updater', 'https://github.com/SemRels/updater-npm'),
		('@semrel', 'updater-npm', 'updater', 'https://github.com/example/unrelated')`)
	require.NoError(t, err)
	db, err := Connect(collisionDSN)
	require.NoError(t, err)
	defer db.Close()
	err = db.RunMigrations("migrations")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "canonical identity is already occupied")

	migrator := newMigrator(t, collisionDSN)
	version, dirty, err := migrator.Version()
	require.NoError(t, err)
	assert.EqualValues(t, 8, version)
	assert.False(t, dirty)
	_, _ = migrator.Close()

	_, err = pool.Exec(context.Background(),
		`DELETE FROM plugins WHERE repository = 'https://github.com/example/unrelated'`)
	require.NoError(t, err)
	require.NoError(t, db.RunMigrations("migrations"))
	migrator = newMigrator(t, collisionDSN)
	version, dirty, err = migrator.Version()
	require.NoError(t, err)
	assert.EqualValues(t, 9, version)
	assert.False(t, dirty)
	_, _ = migrator.Close()
	pool.Close()
}

func TestCanonicalMigrationSerializesConcurrentConflictingWrite(t *testing.T) {
	adminDSN := testutil.DatabaseURL(t, "..")
	admin, err := pgxpool.New(context.Background(), adminDSN)
	require.NoError(t, err)
	_, err = admin.Exec(context.Background(), `CREATE DATABASE semrel_migration_race`)
	require.NoError(t, err)
	admin.Close()

	dsn := strings.Replace(adminDSN, "/semrel_registry?", "/semrel_migration_race?", 1)
	migrateToVersion(t, dsn, 8)
	db, err := Connect(dsn)
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Pool().Exec(context.Background(), `
		INSERT INTO plugins (namespace, name, category, repository)
		VALUES ('@semrel', 'npm', 'updater',
		        'https://github.com/SemRels/updater-npm')`)
	require.NoError(t, err)

	writeTx, err := db.Pool().Begin(context.Background())
	require.NoError(t, err)
	require.NoError(t, LockPluginWrites(context.Background(), writeTx))
	migrationResult := make(chan error, 1)
	go func() {
		migrationResult <- db.RunMigrations("migrations")
	}()
	time.Sleep(100 * time.Millisecond)
	_, err = writeTx.Exec(context.Background(), `
		INSERT INTO plugins (namespace, name, category, repository)
		VALUES ('@semrel', 'updater-npm', 'updater',
		        'https://github.com/example/concurrent-conflict')`)
	require.NoError(t, err)
	require.NoError(t, writeTx.Commit(context.Background()))

	require.ErrorContains(t, <-migrationResult, "canonical identity is already occupied")
	migrator := newMigrator(t, dsn)
	version, dirty, err := migrator.Version()
	require.NoError(t, err)
	assert.EqualValues(t, 8, version)
	assert.False(t, dirty)
	_, _ = migrator.Close()
}

func TestCanonicalFirstPartyMigrationRoundTripPreservesMultiHyphenNames(t *testing.T) {
	adminDSN := testutil.DatabaseURL(t, "..")
	admin, err := pgxpool.New(context.Background(), adminDSN)
	require.NoError(t, err)
	_, err = admin.Exec(context.Background(), `CREATE DATABASE semrel_roundtrip`)
	require.NoError(t, err)
	admin.Close()

	dsn := strings.Replace(adminDSN, "/semrel_registry?", "/semrel_roundtrip?", 1)
	migrateToVersion(t, dsn, 8)
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	defer pool.Close()
	_, err = pool.Exec(context.Background(), `
		INSERT INTO plugins (namespace, name, category, repository) VALUES
		('@semrel', 'changelog-md', 'generator', 'https://github.com/SemRels/generator-changelog-md'),
		('@semrel', 'github-actions', 'condition', 'https://github.com/SemRels/condition-github-actions')`)
	require.NoError(t, err)

	migrator := newMigrator(t, dsn)
	require.NoError(t, migrator.Up())
	assertPluginNames(t, pool, map[string]string{
		"https://github.com/SemRels/generator-changelog-md":   "generator-changelog-md",
		"https://github.com/SemRels/condition-github-actions": "condition-github-actions",
	})
	var communityID int64
	require.NoError(t, pool.QueryRow(context.Background(), `
		INSERT INTO plugins (namespace, name, category, repository)
		VALUES ('@community', 'generator-community', 'generator',
		        'https://github.com/community/generator-community')
		RETURNING id`).Scan(&communityID))
	_, err = pool.Exec(context.Background(), `
		INSERT INTO plugin_aliases (plugin_id, alias)
		VALUES ($1, 'community-generator-alias')`, communityID)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `
		INSERT INTO plugins (namespace, name, category, repository)
		VALUES ('@semrel', 'updater-community', 'updater',
		        'https://github.com/SemRels/updater-community')`)
	require.NoError(t, err)

	require.NoError(t, migrator.Migrate(8))
	assertPluginNames(t, pool, map[string]string{
		"https://github.com/SemRels/generator-changelog-md":   "changelog-md",
		"https://github.com/SemRels/condition-github-actions": "github-actions",
	})
	assertCommunityAlias(t, pool, communityID)
	assertPluginNames(t, pool, map[string]string{
		"https://github.com/SemRels/updater-community": "updater-community",
	})

	require.NoError(t, migrator.Up())
	assertPluginNames(t, pool, map[string]string{
		"https://github.com/SemRels/generator-changelog-md":   "generator-changelog-md",
		"https://github.com/SemRels/condition-github-actions": "condition-github-actions",
	})
	assertCommunityAlias(t, pool, communityID)
	assertPluginNames(t, pool, map[string]string{
		"https://github.com/SemRels/updater-community": "updater-community",
	})
	_, _ = migrator.Close()
}

func TestCanonicalFirstPartyDowngradeSupportsEntireCatalog(t *testing.T) {
	dsn := testutil.DatabaseURL(t, "..")
	migrateToVersion(t, dsn, 8)
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	defer pool.Close()

	expectedLegacyNames := make(map[string]string)
	for _, plugin := range naming.FirstPartyPlugins() {
		legacyName := plugin.Name
		for _, alias := range plugin.Aliases {
			if alias != plugin.Name && !strings.Contains(alias, "/") {
				legacyName = alias
				break
			}
		}
		repository := "https://github.com/SemRels/" + plugin.Repository
		_, err := pool.Exec(context.Background(), `
			INSERT INTO plugins (namespace, name, category, repository)
			VALUES ('@semrel', $1, $2, $3)`,
			legacyName, plugin.Category, repository)
		require.NoError(t, err, plugin.Name)
		expectedLegacyNames[repository] = legacyName
	}

	migrator := newMigrator(t, dsn)
	require.NoError(t, migrator.Up())
	for _, plugin := range naming.FirstPartyPlugins() {
		assertPluginNames(t, pool, map[string]string{
			"https://github.com/SemRels/" + plugin.Repository: plugin.Name,
		})
	}
	require.NoError(t, migrator.Migrate(8))
	assertPluginNames(t, pool, expectedLegacyNames)
	assertPluginNames(t, pool, map[string]string{
		"https://github.com/SemRels/publisher-npm": "publisher-npm",
		"https://github.com/SemRels/updater-npm":   "npm",
	})
	_, _ = migrator.Close()
}

func TestMigrationsCanRollbackFromLatestToZero(t *testing.T) {
	dsn := testutil.DatabaseURL(t, "..")
	migrator := newMigrator(t, dsn)
	require.NoError(t, migrator.Up())
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	defer pool.Close()

	var communityID int64
	require.NoError(t, pool.QueryRow(context.Background(), `
		INSERT INTO plugins (namespace, name, category, repository)
		VALUES ('@community', 'rollback-alias', 'provider',
		        'https://github.com/community/rollback-alias')
		RETURNING id`).Scan(&communityID))
	_, err = pool.Exec(context.Background(),
		`INSERT INTO plugin_aliases (plugin_id, alias) VALUES ($1, 'retained-until-below-eight')`,
		communityID)
	require.NoError(t, err)

	require.NoError(t, migrator.Down())
	var pluginsTable, aliasesTable *string
	require.NoError(t, pool.QueryRow(context.Background(), `
		SELECT to_regclass('public.plugins')::TEXT,
		       to_regclass('public.plugin_aliases')::TEXT`).
		Scan(&pluginsTable, &aliasesTable))
	assert.Nil(t, pluginsTable)
	assert.Nil(t, aliasesTable)
	_, _, err = migrator.Version()
	assert.ErrorIs(t, err, migrate.ErrNilVersion)
	_, _ = migrator.Close()
}

func assertCommunityAlias(t *testing.T, pool *pgxpool.Pool, pluginID int64) {
	t.Helper()
	var alias string
	err := pool.QueryRow(context.Background(), `
		SELECT alias FROM plugin_aliases
		WHERE plugin_id = $1 AND alias = 'community-generator-alias'`, pluginID).Scan(&alias)
	require.NoError(t, err)
	assert.Equal(t, "community-generator-alias", alias)
}

func assertPluginNames(t *testing.T, pool *pgxpool.Pool, expected map[string]string) {
	t.Helper()
	for repository, expectedName := range expected {
		var name string
		err := pool.QueryRow(context.Background(),
			`SELECT name FROM plugins WHERE repository = $1`, repository).Scan(&name)
		require.NoError(t, err)
		assert.Equal(t, expectedName, name)
	}
}

func newMigrator(t *testing.T, dsn string) *migrate.Migrate {
	t.Helper()
	source, err := filepath.Abs("migrations")
	require.NoError(t, err)
	migrator, err := migrate.New(migrationSourceURL(source), dsn)
	require.NoError(t, err)
	return migrator
}

func migrateToVersion(t *testing.T, dsn string, version uint) {
	t.Helper()
	migrator := newMigrator(t, dsn)
	require.NoError(t, migrator.Migrate(version))
	_, _ = migrator.Close()
}

func migrateUp(t *testing.T, dsn string) {
	t.Helper()
	migrator := newMigrator(t, dsn)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()
}
