package service

import (
	"context"
	"testing"

	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeFirstPartyPluginRequiresStrictRepositoryURL(t *testing.T) {
	for _, repository := range []string{
		"provider-github",
		"ftp://github.com/SemRels/provider-github",
		"http://github.com/SemRels/provider-github",
		"https://github.com.evil/SemRels/provider-github",
		"https://example.com/github.com/SemRels/provider-github",
	} {
		plugin := models.Plugin{
			Namespace:  "@community",
			Name:       "unchanged",
			Repository: repository,
		}
		assert.False(t, normalizeFirstPartyPlugin(&plugin), repository)
		assert.Equal(t, "@community", plugin.Namespace, repository)
		assert.Equal(t, "unchanged", plugin.Name, repository)
	}

	plugin := models.Plugin{Repository: "https://github.com/SemRels/provider-github"}
	assert.True(t, normalizeFirstPartyPlugin(&plugin))
	assert.Equal(t, "@semrel", plugin.Namespace)
	assert.Equal(t, "provider-github", plugin.Name)
}

func TestUpdateFirstPartyMetadataPreservesExtraAliases(t *testing.T) {
	store, err := repository.NewFileRepository(t.TempDir())
	require.NoError(t, err)
	svc := NewPluginService(store)
	existing := models.Plugin{
		Namespace:  "@semrel",
		Name:       "provider-github",
		Aliases:    []string{"provider-github", "github", "@semrel/github", "retained-extra", "RETAINED-EXTRA", " Provider-Legacy "},
		Category:   "provider",
		Repository: "https://github.com/SemRels/provider-github",
		Status:     models.StatusActive,
	}

	_, err = store.Create(context.Background(), &existing)
	require.NoError(t, err)

	description := "updated description"
	tags := []string{"provider", "updated"}
	updated, err := svc.UpdatePlugin(context.Background(), existing.Ref(), models.PluginPatch{
		Description: &description,
		Tags:        &tags,
	})
	require.NoError(t, err)
	assert.Equal(t, description, updated.Description)
	assert.Equal(t, tags, updated.Tags)
	assert.ElementsMatch(t, []string{
		"provider-github", "github", "@semrel/github", "retained-extra", "Provider-Legacy",
	}, updated.Aliases)
}

func TestLegacyNamespaceMigrationPreservesExtraAliases(t *testing.T) {
	store, err := repository.NewFileRepository(t.TempDir())
	require.NoError(t, err)
	svc := NewPluginService(store)
	legacy := models.Plugin{
		Name:       "github",
		Aliases:    []string{"retained-extra", " RETAINED-EXTRA ", "provider-legacy"},
		Category:   "provider",
		Repository: "https://github.com/SemRels/provider-github",
		Status:     models.StatusActive,
	}
	_, err = store.Create(context.Background(), &legacy)
	require.NoError(t, err)

	namespace, name := "@semrel", "provider-github"
	migrated, err := svc.UpdatePlugin(context.Background(), legacy.Ref(), models.PluginPatch{
		Namespace: &namespace,
		Name:      &name,
	})
	require.NoError(t, err)
	assert.Equal(t, "@semrel/provider-github", migrated.Ref())
	assert.ElementsMatch(t, []string{
		"provider-github", "github", "@semrel/github", "retained-extra", "provider-legacy",
	}, migrated.Aliases)
}
