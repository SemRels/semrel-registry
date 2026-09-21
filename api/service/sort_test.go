package service

import (
	"context"
	"testing"

	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/repository"
	"github.com/stretchr/testify/require"
)

// The admin UI and the public registry both offer "most viewed" and "most
// downloaded". Neither was accepted by the service, so choosing them returned
// a validation error instead of a sorted list.
func TestListPluginsAcceptsPopularitySorts(t *testing.T) {
	repo, err := repository.NewFileRepository(t.TempDir())
	require.NoError(t, err)
	svc := NewPluginService(repo)

	for _, sortField := range []string{"views", "downloads", "name", "category", "created_at", "updated_at"} {
		t.Run(sortField, func(t *testing.T) {
			_, listErr := svc.ListPlugins(context.Background(), ListPluginsParams{Sort: sortField, SortDir: "desc"})
			require.NoError(t, listErr, "sort %q must be accepted", sortField)
		})
	}
}

func TestListPluginsRejectsUnknownSort(t *testing.T) {
	repo, err := repository.NewFileRepository(t.TempDir())
	require.NoError(t, err)
	svc := NewPluginService(repo)

	_, listErr := svc.ListPlugins(context.Background(), ListPluginsParams{Sort: "; DROP TABLE plugins"})
	require.Error(t, listErr)
}

// The file backend sorted by name whenever the field was one it did not
// recognise, which made "most downloaded" silently alphabetical.
func TestFileBackendSortsByPopularity(t *testing.T) {
	repo, err := repository.NewFileRepository(t.TempDir())
	require.NoError(t, err)
	svc := NewPluginService(repo)
	ctx := context.Background()

	for _, fixture := range []struct {
		name      string
		downloads int64
		views     int64
	}{
		{"analyzer-zulu", 100, 5},
		{"analyzer-alpha", 10, 500},
	} {
		created, createErr := svc.CreatePlugin(ctx, models.Plugin{
			Name:       fixture.name,
			Category:   "analyzer",
			Repository: "https://github.com/SemRels/" + fixture.name,
		})
		require.NoError(t, createErr)
		require.NoError(t, repo.IncrCounters(ctx, created.ID, 0, fixture.views, fixture.downloads))
	}

	byDownloads, err := svc.ListPlugins(ctx, ListPluginsParams{Sort: "downloads", SortDir: "desc"})
	require.NoError(t, err)
	require.Len(t, byDownloads.Data, 2)
	require.Equal(t, "analyzer-zulu", byDownloads.Data[0].Name, "most downloaded must come first, not the alphabetical winner")

	byViews, err := svc.ListPlugins(ctx, ListPluginsParams{Sort: "views", SortDir: "desc"})
	require.NoError(t, err)
	require.Len(t, byViews.Data, 2)
	require.Equal(t, "analyzer-alpha", byViews.Data[0].Name)
}
