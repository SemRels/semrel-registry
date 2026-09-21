package service

import (
	"context"
	"testing"
	"time"

	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestService(t *testing.T) (*PluginService, repository.PluginRepository) {
	t.Helper()
	repo, err := repository.NewFileRepository(t.TempDir())
	require.NoError(t, err)
	return NewPluginService(repo), repo
}

func submitTestPlugin(t *testing.T, svc *PluginService) models.Plugin {
	t.Helper()
	plugin, err := svc.SubmitPlugin(context.Background(), models.Plugin{
		Name:       "analyzer-example",
		Category:   "analyzer",
		Author:     "alice",
		Repository: "https://github.com/alice/analyzer-example",
	})
	require.NoError(t, err)
	return plugin
}

// A "rejected" badge with no explanation leaves the author nothing to act on.
func TestRejectionRequiresAReason(t *testing.T) {
	svc, _ := newTestService(t)
	plugin := submitTestPlugin(t, svc)

	_, err := svc.ReviewPlugin(context.Background(), plugin.Ref(), models.StatusRejected, models.ReviewDecision{}, "maintainer")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reason")
}

func TestRejectionStoresReasonAndReviewer(t *testing.T) {
	svc, _ := newTestService(t)
	plugin := submitTestPlugin(t, svc)

	reviewed, err := svc.ReviewPlugin(context.Background(), plugin.Ref(), models.StatusRejected,
		models.ReviewDecision{Reason: "The repository is missing a plugin manifest."}, "maintainer")
	require.NoError(t, err)

	assert.Equal(t, models.StatusRejected, reviewed.Status)
	assert.Equal(t, "The repository is missing a plugin manifest.", reviewed.RejectionReason)
	assert.Equal(t, "maintainer", reviewed.ReviewedBy)
	assert.NotNil(t, reviewed.ReviewedAt)
}

// Approving needs no justification.
func TestApprovalWithoutReasonIsAllowed(t *testing.T) {
	svc, _ := newTestService(t)
	plugin := submitTestPlugin(t, svc)

	reviewed, err := svc.ReviewPlugin(context.Background(), plugin.Ref(), models.StatusActive, models.ReviewDecision{}, "maintainer")
	require.NoError(t, err)
	assert.Equal(t, models.StatusActive, reviewed.Status)
}

func addTestVersion(t *testing.T, svc *PluginService, ref, version string) models.PluginVersion {
	t.Helper()
	created, err := svc.CreateVersion(context.Background(), ref, models.PluginVersion{
		Version:     version,
		DownloadURL: "https://github.com/alice/analyzer-example/releases/download/v" + version + "/plugin-linux-amd64",
		Checksums:   map[string]string{"linux_amd64": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
	})
	require.NoError(t, err)
	return created
}

func TestYankRequiresAReason(t *testing.T) {
	svc, _ := newTestService(t)
	plugin := submitTestPlugin(t, svc)
	version := addTestVersion(t, svc, plugin.Ref(), "1.0.0")

	_, err := svc.YankVersion(context.Background(), plugin.Ref(), version.ID, true,
		models.VersionYankRequest{}, models.DeleteActor{Login: "alice"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reason")
}

// Yanking must not remove the version: builds that already pin it keep working.
func TestYankedVersionStaysResolvable(t *testing.T) {
	svc, _ := newTestService(t)
	plugin := submitTestPlugin(t, svc)
	version := addTestVersion(t, svc, plugin.Ref(), "1.0.0")

	yanked, err := svc.YankVersion(context.Background(), plugin.Ref(), version.ID, true,
		models.VersionYankRequest{Reason: "Ships a broken linux binary."}, models.DeleteActor{Login: "alice"})
	require.NoError(t, err)
	assert.True(t, yanked.Yanked())
	assert.Equal(t, "Ships a broken linux binary.", yanked.YankedReason)
	assert.Equal(t, "alice", yanked.YankedBy)

	versions, err := svc.ListVersions(context.Background(), plugin.Ref(), 10, 0)
	require.NoError(t, err)
	require.Len(t, versions, 1, "a yanked version must still be listed")
	assert.True(t, versions[0].Yanked())
}

func TestUnyankClearsTheRetraction(t *testing.T) {
	svc, _ := newTestService(t)
	plugin := submitTestPlugin(t, svc)
	version := addTestVersion(t, svc, plugin.Ref(), "1.0.0")

	_, err := svc.YankVersion(context.Background(), plugin.Ref(), version.ID, true,
		models.VersionYankRequest{Reason: "Mistake."}, models.DeleteActor{Login: "alice"})
	require.NoError(t, err)

	restored, err := svc.YankVersion(context.Background(), plugin.Ref(), version.ID, false,
		models.VersionYankRequest{}, models.DeleteActor{Login: "alice"})
	require.NoError(t, err)
	assert.False(t, restored.Yanked())
	assert.Empty(t, restored.YankedReason)
}

// Version resolution is the reason yanking exists.
func TestLatestInstallableVersionSkipsYankedAndPrereleases(t *testing.T) {
	now := time.Now()
	versions := []models.PluginVersion{
		{Version: "2.0.0-rc.1", Prerelease: true},
		{Version: "1.5.0", YankedAt: &now},
		{Version: "1.4.0"},
	}

	latest, ok := LatestInstallableVersion(versions)
	require.True(t, ok)
	assert.Equal(t, "1.4.0", latest.Version)
}

func TestLatestInstallableVersionReportsNoneAvailable(t *testing.T) {
	now := time.Now()
	_, ok := LatestInstallableVersion([]models.PluginVersion{{Version: "1.0.0", YankedAt: &now}})
	assert.False(t, ok)
}
