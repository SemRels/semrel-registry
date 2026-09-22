package service

import (
	"context"
	"testing"

	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// publishCompatible creates a plugin whose single stable release declares the
// given core-compatibility range.
func publishCompatible(t *testing.T, svc *PluginService, name, coreRange string) {
	t.Helper()
	ctx := context.Background()

	created, err := svc.CreatePlugin(ctx, models.Plugin{
		Name:       name,
		Category:   "analyzer",
		Author:     "alice",
		Repository: "https://github.com/alice/" + name,
	})
	require.NoError(t, err)

	_, err = svc.CreateVersion(ctx, created.Ref(), models.PluginVersion{
		Version:     "1.0.0",
		SemrelCore:  coreRange,
		DownloadURL: "https://github.com/alice/" + name + "/releases/download/v1.0.0/plugin-linux-amd64",
		Checksums:   map[string]string{"linux_amd64": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
	})
	require.NoError(t, err)
}

func compatibilityService(t *testing.T) *PluginService {
	t.Helper()
	repo, err := repository.NewFileRepository(t.TempDir())
	require.NoError(t, err)
	return NewPluginService(repo)
}

func namesOf(result PluginListResult) []string {
	names := make([]string, 0, len(result.Data))
	for _, p := range result.Data {
		names = append(names, p.Name)
	}
	return names
}

func TestCompatibleWithSelectsMatchingPlugins(t *testing.T) {
	svc := compatibilityService(t)
	publishCompatible(t, svc, "analyzer-current", ">=0.25.0 <1.0.0")
	publishCompatible(t, svc, "analyzer-old", "<0.20.0")
	publishCompatible(t, svc, "analyzer-future", ">=2.0.0")

	result, err := svc.ListPlugins(context.Background(), ListPluginsParams{CompatibleWith: "0.27.1"})
	require.NoError(t, err)

	assert.Equal(t, []string{"analyzer-current"}, namesOf(result))
	assert.Equal(t, int64(1), result.Pagination.Total, "the total must describe the filtered set, not the table")
}

// Compatibility metadata is optional and most of the catalogue predates it.
// Treating "no declared range" as "incompatible" would hide nearly everything.
func TestCompatibleWithKeepsPluginsThatDeclareNoRange(t *testing.T) {
	svc := compatibilityService(t)
	publishCompatible(t, svc, "analyzer-silent", "")
	publishCompatible(t, svc, "analyzer-incompatible", ">=9.0.0")

	result, err := svc.ListPlugins(context.Background(), ListPluginsParams{CompatibleWith: "0.27.1"})
	require.NoError(t, err)

	assert.Equal(t, []string{"analyzer-silent"}, namesOf(result))
}

// A publishing mistake in one plugin must not fail the whole listing, but the
// conservative answer to a compatibility question is to leave it out.
func TestCompatibleWithSkipsUnparseableRanges(t *testing.T) {
	svc := compatibilityService(t)
	publishCompatible(t, svc, "analyzer-broken", "not-a-range")
	publishCompatible(t, svc, "analyzer-good", ">=0.1.0")

	result, err := svc.ListPlugins(context.Background(), ListPluginsParams{CompatibleWith: "0.27.1"})
	require.NoError(t, err)

	assert.Equal(t, []string{"analyzer-good"}, namesOf(result))
}

func TestCompatibleWithRejectsANonVersion(t *testing.T) {
	svc := compatibilityService(t)

	_, err := svc.ListPlugins(context.Background(), ListPluginsParams{CompatibleWith: "latest"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compatibleWith")
}

func TestCompatibleWithPaginatesTheFilteredSet(t *testing.T) {
	svc := compatibilityService(t)
	for _, name := range []string{"analyzer-a", "analyzer-b", "analyzer-c"} {
		publishCompatible(t, svc, name, ">=0.1.0")
	}
	publishCompatible(t, svc, "analyzer-z", ">=9.0.0")

	first, err := svc.ListPlugins(context.Background(), ListPluginsParams{CompatibleWith: "0.27.1", Limit: 2, Page: 1})
	require.NoError(t, err)
	assert.Equal(t, []string{"analyzer-a", "analyzer-b"}, namesOf(first))
	assert.Equal(t, int64(3), first.Pagination.Total)
	assert.Equal(t, 2, first.Pagination.Pages)

	second, err := svc.ListPlugins(context.Background(), ListPluginsParams{CompatibleWith: "0.27.1", Limit: 2, Page: 2})
	require.NoError(t, err)
	assert.Equal(t, []string{"analyzer-c"}, namesOf(second))

	beyond, err := svc.ListPlugins(context.Background(), ListPluginsParams{CompatibleWith: "0.27.1", Limit: 2, Page: 9})
	require.NoError(t, err)
	assert.Empty(t, namesOf(beyond))
}

// The compatibility filter composes with the others rather than replacing them.
func TestCompatibleWithCombinesWithOtherFilters(t *testing.T) {
	svc := compatibilityService(t)
	publishCompatible(t, svc, "analyzer-match", ">=0.1.0")
	publishCompatible(t, svc, "analyzer-other", ">=0.1.0")

	result, err := svc.ListPlugins(context.Background(), ListPluginsParams{
		CompatibleWith: "0.27.1",
		Search:         "match",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"analyzer-match"}, namesOf(result))
}
