package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/repository"
	"github.com/SemRels/semrel-registry/api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTestCatalog(t *testing.T, plugins []map[string]any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plugins.json")
	data, err := json.Marshal(map[string]any{
		"schemaVersion": 2,
		"plugins":       plugins,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}

func bundledTestPlugin() map[string]any {
	return map[string]any{
		"namespace":   "@semrel",
		"name":        "condition-generic",
		"aliases":     []string{"generic", "@semrel/generic", "condition-generic"},
		"description": "generic condition",
		"author":      "SemRels",
		"category":    "condition",
		"repository":  "https://github.com/SemRels/condition-generic",
		"license":     "Apache-2.0",
		"tags":        []string{"condition", "release"},
		"versions": []map[string]any{{
			"version":     "1.2.3",
			"releaseDate": "2026-07-20T12:00:00Z",
			"downloadUrl": "https://example.test/condition-generic/1.2.3",
			"changelog":   "bundled release",
			"checksums": map[string]string{
				"linux_amd64": "generic-linux-hash",
			},
		}},
	}
}

func TestSeedStartupCatalogFileBackendFresh(t *testing.T) {
	repo, err := repository.NewFileRepository(t.TempDir())
	require.NoError(t, err)
	path := writeTestCatalog(t, []map[string]any{bundledTestPlugin()})

	require.NoError(t, seedStartupCatalog(context.Background(), repo, nil,
		catalogSource{path: path, required: true}))

	svc := service.NewPluginService(repo)
	for _, ref := range []string{"@semrel/condition-generic", "generic", "@semrel/generic"} {
		plugin, lookupErr := svc.GetPlugin(context.Background(), ref)
		require.NoError(t, lookupErr, ref)
		assert.Equal(t, "@semrel/condition-generic", plugin.Ref())
	}
	plugin, err := svc.GetPlugin(context.Background(), "@semrel/condition-generic")
	require.NoError(t, err)
	assert.Equal(t, "generic condition", plugin.Description)
	assert.Equal(t, []string{"condition", "release"}, plugin.Tags)
	assert.Equal(t, "1.2.3", plugin.LatestVersion)

	versions, err := repo.GetVersions(context.Background(), plugin.ID)
	require.NoError(t, err)
	require.Len(t, versions, 1)
	assert.Equal(t, "generic-linux-hash", versions[0].Checksums["linux_amd64"])
	assert.Equal(t, "bundled release", versions[0].Changelog)
}

func TestSeedStartupCatalogFileBackendRepairsPartialAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	repo, err := repository.NewFileRepository(t.TempDir())
	require.NoError(t, err)

	partial := models.Plugin{
		Namespace:   "@semrel",
		Name:        "condition-generic",
		Aliases:     []string{"stale-alias"},
		Description: "partial",
		Category:    "condition",
		Repository:  "https://github.com/SemRels/condition-generic",
		Status:      models.StatusActive,
	}
	partialID, err := repo.Create(ctx, &partial)
	require.NoError(t, err)
	community := models.Plugin{
		Namespace:   "@community",
		Name:        "user-plugin",
		Description: "keep me",
		Category:    "provider",
		Repository:  "https://github.com/community/user-plugin",
		Status:      models.StatusPending,
	}
	communityID, err := repo.Create(ctx, &community)
	require.NoError(t, err)

	path := writeTestCatalog(t, []map[string]any{
		bundledTestPlugin(),
		{
			"namespace":   "@semrel",
			"name":        "provider-fresh",
			"aliases":     []string{"fresh"},
			"description": "fresh bundled plugin",
			"category":    "provider",
			"repository":  "https://github.com/SemRels/provider-fresh",
			"versions": []map[string]any{{
				"version": "2.0.0",
				"checksums": map[string]string{
					"darwin_arm64": "fresh-darwin-hash",
				},
			}},
		},
	})
	source := catalogSource{path: path, required: true}
	require.NoError(t, seedStartupCatalog(ctx, repo, nil, source))

	afterFirst, err := repo.GetByID(ctx, partialID)
	require.NoError(t, err)
	require.NoError(t, seedStartupCatalog(ctx, repo, nil, source))
	afterSecond, err := repo.GetByID(ctx, partialID)
	require.NoError(t, err)
	assert.Equal(t, afterFirst, afterSecond)

	all, err := repo.GetAll(ctx, 0, 0)
	require.NoError(t, err)
	assert.Len(t, all, 3)
	assert.Equal(t, "generic condition", afterSecond.Description)
	assert.NotContains(t, afterSecond.Aliases, "stale-alias")
	require.Len(t, afterSecond.Versions, 1)
	assert.Equal(t, "generic-linux-hash", afterSecond.Versions[0].Checksums["linux_amd64"])

	preserved, err := repo.GetByID(ctx, communityID)
	require.NoError(t, err)
	assert.Equal(t, "keep me", preserved.Description)
	assert.Equal(t, models.StatusPending, preserved.Status)
}

func TestSeedStartupCatalogFileBackendSurfacesPluginFailure(t *testing.T) {
	repo, err := repository.NewFileRepository(t.TempDir())
	require.NoError(t, err)
	plugin := bundledTestPlugin()
	plugin["versions"] = []map[string]any{{
		"version":     "broken",
		"releaseDate": "not-a-date",
	}}
	path := writeTestCatalog(t, []map[string]any{plugin})

	err = seedStartupCatalog(context.Background(), repo, nil,
		catalogSource{path: path, required: true})
	require.ErrorContains(t, err, "parse release date")
	all, listErr := repo.GetAll(context.Background(), 0, 0)
	require.NoError(t, listErr)
	assert.Empty(t, all)
}

func TestSeedStartupCatalogFileBackendRollsBackWholeCatalog(t *testing.T) {
	ctx := context.Background()
	repo, err := repository.NewFileRepository(t.TempDir())
	require.NoError(t, err)
	existing := models.Plugin{
		Namespace:   "@semrel",
		Name:        "condition-generic",
		Description: "original",
		Category:    "condition",
		Repository:  "https://github.com/SemRels/condition-generic",
		Status:      models.StatusActive,
	}
	existingID, err := repo.Create(ctx, &existing)
	require.NoError(t, err)

	broken := map[string]any{
		"namespace":  "@semrel",
		"name":       "provider-broken",
		"category":   "provider",
		"repository": "https://github.com/SemRels/provider-broken",
		"versions": []map[string]any{{
			"version":     "1.0.0",
			"releaseDate": "not-a-date",
		}},
	}
	path := writeTestCatalog(t, []map[string]any{bundledTestPlugin(), broken})
	err = seedStartupCatalog(ctx, repo, nil, catalogSource{path: path, required: true})
	require.ErrorContains(t, err, "parse release date")

	unchanged, err := repo.GetByID(ctx, existingID)
	require.NoError(t, err)
	assert.Equal(t, "original", unchanged.Description)
	assert.Empty(t, unchanged.Versions)

	next := models.Plugin{Name: "community-next", Category: "provider", Status: models.StatusActive}
	nextID, err := repo.Create(ctx, &next)
	require.NoError(t, err)
	assert.Equal(t, int64(2), nextID)
}

func TestResolveCatalogSource(t *testing.T) {
	explicit := resolveCatalogSource("  custom/catalog.json  ", "dev")
	assert.Equal(t, "custom/catalog.json", explicit.path)
	assert.True(t, explicit.required)

	production := resolveCatalogSource("", "prod")
	assert.Equal(t, packagedCatalogPath, production.path)
	assert.True(t, production.required)

	development := resolveCatalogSource("", "dev")
	assert.Equal(t, "plugins.json", development.path)
	assert.False(t, development.required)
}

func TestSeedStartupCatalogMissingPathPolicy(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.json")
	require.NoError(t, seedStartupCatalog(context.Background(), nil, nil,
		catalogSource{path: missing, required: false}))

	err := seedStartupCatalog(context.Background(), nil, nil,
		catalogSource{path: missing, required: true})
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist)
	assert.Contains(t, err.Error(), missing)
}
