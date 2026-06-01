package repository

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	appErrors "github.com/SemRels/semrel-registry/api/internal"
	"github.com/SemRels/semrel-registry/api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestFileRepo creates a temporary file-backed repository for a single test.
func newTestFileRepo(t *testing.T) PluginRepository {
	t.Helper()
	dir := t.TempDir()
	repo, err := NewFileRepository(dir)
	require.NoError(t, err)
	return repo
}

func basePlugin(name string) *models.Plugin {
	return &models.Plugin{
		Name:        name,
		Description: "test plugin",
		Author:      "alice",
		Category:    "provider",
		Repository:  "https://github.com/example/" + name,
		License:     "MIT",
		Status:      models.StatusActive,
		Tags:        []string{"test"},
	}
}

// -------------------------------------------------------------------------
// NewFileRepository
// -------------------------------------------------------------------------

func TestNewFileRepository_EmptyDir(t *testing.T) {
	_, err := NewFileRepository("")
	assert.Error(t, err)
}

func TestNewFileRepository_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	subdir := dir + "/nested/storage"
	_, err := NewFileRepository(subdir)
	require.NoError(t, err)
	_, statErr := os.Stat(subdir + "/plugins")
	assert.NoError(t, statErr)
}

// -------------------------------------------------------------------------
// Create
// -------------------------------------------------------------------------

func TestFileRepo_Create_AssignsID(t *testing.T) {
	repo := newTestFileRepo(t)
	p := basePlugin("provider-github")

	id, err := repo.Create(context.Background(), p)
	require.NoError(t, err)
	assert.Equal(t, int64(1), id)
	assert.Equal(t, int64(1), p.ID)
}

func TestFileRepo_Create_SetsTimestamps(t *testing.T) {
	repo := newTestFileRepo(t)
	p := basePlugin("provider-github")
	before := time.Now().Add(-time.Second)

	_, err := repo.Create(context.Background(), p)
	require.NoError(t, err)

	assert.True(t, p.CreatedAt.After(before))
	assert.True(t, p.UpdatedAt.After(before))
}

func TestFileRepo_Create_DuplicateReturnsError(t *testing.T) {
	repo := newTestFileRepo(t)
	_, err := repo.Create(context.Background(), basePlugin("provider-github"))
	require.NoError(t, err)

	_, err = repo.Create(context.Background(), basePlugin("provider-github"))
	assert.ErrorIs(t, err, appErrors.ErrDuplicatePlugin)
}

func TestFileRepo_Create_NilPluginReturnsError(t *testing.T) {
	repo := newTestFileRepo(t)
	_, err := repo.Create(context.Background(), nil)
	assert.Error(t, err)
}

// -------------------------------------------------------------------------
// GetByID
// -------------------------------------------------------------------------

func TestFileRepo_GetByID_Found(t *testing.T) {
	repo := newTestFileRepo(t)
	p := basePlugin("provider-github")
	id, err := repo.Create(context.Background(), p)
	require.NoError(t, err)

	got, err := repo.GetByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "provider-github", got.Name)
	assert.Equal(t, id, got.ID)
}

func TestFileRepo_GetByID_NotFound(t *testing.T) {
	repo := newTestFileRepo(t)
	_, err := repo.GetByID(context.Background(), 999)
	assert.ErrorIs(t, err, appErrors.ErrPluginNotFound)
}

func TestFileRepo_GetByID_DeletedReturnsNotFound(t *testing.T) {
	repo := newTestFileRepo(t)
	p := basePlugin("provider-github")
	id, err := repo.Create(context.Background(), p)
	require.NoError(t, err)

	require.NoError(t, repo.Delete(context.Background(), id))

	_, err = repo.GetByID(context.Background(), id)
	assert.ErrorIs(t, err, appErrors.ErrPluginNotFound)
}

// -------------------------------------------------------------------------
// GetByName / GetByNamespacedName
// -------------------------------------------------------------------------

func TestFileRepo_GetByName_Found(t *testing.T) {
	repo := newTestFileRepo(t)
	_, err := repo.Create(context.Background(), basePlugin("provider-github"))
	require.NoError(t, err)

	got, err := repo.GetByName(context.Background(), "provider-github")
	require.NoError(t, err)
	assert.Equal(t, "provider-github", got.Name)
}

func TestFileRepo_GetByName_NotFound(t *testing.T) {
	repo := newTestFileRepo(t)
	_, err := repo.GetByName(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, appErrors.ErrPluginNotFound)
}

func TestFileRepo_GetByNamespacedName_Found(t *testing.T) {
	repo := newTestFileRepo(t)
	p := basePlugin("provider-github")
	p.Namespace = "@semrel"
	_, err := repo.Create(context.Background(), p)
	require.NoError(t, err)

	got, err := repo.GetByNamespacedName(context.Background(), "@semrel", "provider-github")
	require.NoError(t, err)
	assert.Equal(t, "@semrel", got.Namespace)
	assert.Equal(t, "provider-github", got.Name)
}

func TestFileRepo_GetByNamespacedName_CaseInsensitive(t *testing.T) {
	repo := newTestFileRepo(t)
	p := basePlugin("provider-github")
	p.Namespace = "@SemRel"
	_, err := repo.Create(context.Background(), p)
	require.NoError(t, err)

	got, err := repo.GetByNamespacedName(context.Background(), "@semrel", "provider-github")
	require.NoError(t, err)
	assert.Equal(t, "provider-github", got.Name)
}

func TestFileRepo_GetByNamespacedName_NotFound(t *testing.T) {
	repo := newTestFileRepo(t)
	_, err := repo.GetByNamespacedName(context.Background(), "@other", "missing")
	assert.ErrorIs(t, err, appErrors.ErrPluginNotFound)
}

// -------------------------------------------------------------------------
// GetAll / Filters
// -------------------------------------------------------------------------

func TestFileRepo_GetAll_Empty(t *testing.T) {
	repo := newTestFileRepo(t)
	plugins, err := repo.GetAll(context.Background(), 0, 0)
	require.NoError(t, err)
	assert.Empty(t, plugins)
}

func TestFileRepo_GetAll_ExcludesDeleted(t *testing.T) {
	repo := newTestFileRepo(t)
	p1 := basePlugin("alpha")
	p2 := basePlugin("beta")
	id1, _ := repo.Create(context.Background(), p1)
	_, _ = repo.Create(context.Background(), p2)
	_ = repo.Delete(context.Background(), id1)

	plugins, err := repo.GetAll(context.Background(), 0, 0)
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	assert.Equal(t, "beta", plugins[0].Name)
}

func TestFileRepo_GetAll_DefaultSortByName(t *testing.T) {
	repo := newTestFileRepo(t)
	for _, name := range []string{"gamma", "alpha", "beta"} {
		_, _ = repo.Create(context.Background(), basePlugin(name))
	}

	plugins, err := repo.GetAll(context.Background(), 0, 0)
	require.NoError(t, err)
	require.Len(t, plugins, 3)
	assert.Equal(t, "alpha", plugins[0].Name)
	assert.Equal(t, "beta", plugins[1].Name)
	assert.Equal(t, "gamma", plugins[2].Name)
}

func TestFileRepo_GetAll_LimitAndOffset(t *testing.T) {
	repo := newTestFileRepo(t)
	for _, name := range []string{"alpha", "beta", "gamma", "delta"} {
		_, _ = repo.Create(context.Background(), basePlugin(name))
	}

	page, err := repo.GetAll(context.Background(), 2, 1) // skip first, take 2
	require.NoError(t, err)
	require.Len(t, page, 2)
	assert.Equal(t, "beta", page[0].Name)
	assert.Equal(t, "delta", page[1].Name)
}

func TestFileRepo_GetAll_CategoryFilter(t *testing.T) {
	repo := newTestFileRepo(t)
	p1 := basePlugin("provider-github")
	p1.Category = "provider"
	p2 := basePlugin("hook-slack")
	p2.Category = "hook"
	_, _ = repo.Create(context.Background(), p1)
	_, _ = repo.Create(context.Background(), p2)

	plugins, err := repo.GetAll(context.Background(), 0, 0, CategoryFilter{Category: "provider"})
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	assert.Equal(t, "provider-github", plugins[0].Name)
}

func TestFileRepo_GetAll_SearchFilter(t *testing.T) {
	repo := newTestFileRepo(t)
	p1 := basePlugin("provider-github")
	p1.Description = "GitHub provider for semrel"
	p1.Repository = "https://github.com/example/provider-github"
	p2 := basePlugin("hook-slack")
	p2.Description = "Slack notifications"
	p2.Repository = "https://example.com/hook-slack"
	_, _ = repo.Create(context.Background(), p1)
	_, _ = repo.Create(context.Background(), p2)

	plugins, err := repo.GetAll(context.Background(), 0, 0, SearchFilter{Query: "semrel"})
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	assert.Equal(t, "provider-github", plugins[0].Name)
}

func TestFileRepo_GetAll_StatusFilter(t *testing.T) {
	repo := newTestFileRepo(t)
	p1 := basePlugin("active-plugin")
	p1.Status = models.StatusActive
	p2 := basePlugin("pending-plugin")
	p2.Status = models.StatusPending
	id1, _ := repo.Create(context.Background(), p1)
	id2, _ := repo.Create(context.Background(), p2)
	_ = id1
	_ = id2

	plugins, err := repo.GetAll(context.Background(), 0, 0, StatusFilter{Statuses: []string{models.StatusPending}})
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	assert.Equal(t, "pending-plugin", plugins[0].Name)
}

func TestFileRepo_GetAll_NamespaceFilter(t *testing.T) {
	repo := newTestFileRepo(t)
	p1 := basePlugin("provider-github")
	p1.Namespace = "@semrel"
	p2 := basePlugin("hook-slack")
	p2.Namespace = "@other"
	_, _ = repo.Create(context.Background(), p1)
	_, _ = repo.Create(context.Background(), p2)

	plugins, err := repo.GetAll(context.Background(), 0, 0, NamespaceFilter{Namespace: "@semrel"})
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	assert.Equal(t, "provider-github", plugins[0].Name)
}

func TestFileRepo_GetAll_SortDescending(t *testing.T) {
	repo := newTestFileRepo(t)
	for _, name := range []string{"alpha", "gamma", "beta"} {
		_, _ = repo.Create(context.Background(), basePlugin(name))
	}

	plugins, err := repo.GetAll(context.Background(), 0, 0, SortFilter{Field: "name", Direction: "DESC"})
	require.NoError(t, err)
	require.Len(t, plugins, 3)
	assert.Equal(t, "gamma", plugins[0].Name)
	assert.Equal(t, "beta", plugins[1].Name)
	assert.Equal(t, "alpha", plugins[2].Name)
}

// -------------------------------------------------------------------------
// Update
// -------------------------------------------------------------------------

func TestFileRepo_Update_PersistsChanges(t *testing.T) {
	repo := newTestFileRepo(t)
	p := basePlugin("provider-github")
	id, err := repo.Create(context.Background(), p)
	require.NoError(t, err)

	p.ID = id
	p.Description = "updated description"
	require.NoError(t, repo.Update(context.Background(), p))

	got, err := repo.GetByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "updated description", got.Description)
}

func TestFileRepo_Update_NotFound(t *testing.T) {
	repo := newTestFileRepo(t)
	p := basePlugin("missing")
	p.ID = 999
	err := repo.Update(context.Background(), p)
	assert.ErrorIs(t, err, appErrors.ErrPluginNotFound)
}

func TestFileRepo_Update_NilReturnsError(t *testing.T) {
	repo := newTestFileRepo(t)
	assert.Error(t, repo.Update(context.Background(), nil))
}

// -------------------------------------------------------------------------
// UpdateStatus
// -------------------------------------------------------------------------

func TestFileRepo_UpdateStatus(t *testing.T) {
	repo := newTestFileRepo(t)
	id, err := repo.Create(context.Background(), basePlugin("plugin"))
	require.NoError(t, err)

	require.NoError(t, repo.UpdateStatus(context.Background(), id, models.StatusRejected))

	got, err := repo.GetByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, models.StatusRejected, got.Status)
}

func TestFileRepo_UpdateStatus_NotFound(t *testing.T) {
	repo := newTestFileRepo(t)
	err := repo.UpdateStatus(context.Background(), 999, models.StatusActive)
	assert.ErrorIs(t, err, appErrors.ErrPluginNotFound)
}

// -------------------------------------------------------------------------
// UpdateValidationChecks
// -------------------------------------------------------------------------

func TestFileRepo_UpdateValidationChecks(t *testing.T) {
	repo := newTestFileRepo(t)
	id, err := repo.Create(context.Background(), basePlugin("plugin"))
	require.NoError(t, err)

	checks := json.RawMessage(`{"readme":true}`)
	require.NoError(t, repo.UpdateValidationChecks(context.Background(), id, checks))

	got, err := repo.GetByID(context.Background(), id)
	require.NoError(t, err)
	assert.JSONEq(t, `{"readme":true}`, string(got.ValidationChecks))
	assert.NotNil(t, got.ValidatedAt)
}

// -------------------------------------------------------------------------
// Delete
// -------------------------------------------------------------------------

func TestFileRepo_Delete_SoftDelete(t *testing.T) {
	repo := newTestFileRepo(t)
	id, err := repo.Create(context.Background(), basePlugin("plugin"))
	require.NoError(t, err)

	require.NoError(t, repo.Delete(context.Background(), id))

	// Must not be returned by GetAll.
	plugins, err := repo.GetAll(context.Background(), 0, 0)
	require.NoError(t, err)
	assert.Empty(t, plugins)
}

func TestFileRepo_Delete_NotFound(t *testing.T) {
	repo := newTestFileRepo(t)
	err := repo.Delete(context.Background(), 999)
	assert.ErrorIs(t, err, appErrors.ErrPluginNotFound)
}

func TestFileRepo_Delete_AlreadyDeletedReturnsNotFound(t *testing.T) {
	repo := newTestFileRepo(t)
	id, _ := repo.Create(context.Background(), basePlugin("plugin"))
	require.NoError(t, repo.Delete(context.Background(), id))
	assert.ErrorIs(t, repo.Delete(context.Background(), id), appErrors.ErrPluginNotFound)
}

// -------------------------------------------------------------------------
// AddVersion / GetVersions
// -------------------------------------------------------------------------

func TestFileRepo_AddVersion_And_GetVersions(t *testing.T) {
	repo := newTestFileRepo(t)
	id, err := repo.Create(context.Background(), basePlugin("plugin"))
	require.NoError(t, err)

	v := &models.PluginVersion{
		PluginID:    id,
		Version:     "1.0.0",
		DownloadURL: "https://example.com/dl/1.0.0",
		Checksums:   map[string]string{"linux_amd64": "abc123"},
	}
	vid, err := repo.AddVersion(context.Background(), v)
	require.NoError(t, err)
	assert.Equal(t, int64(1), vid)

	versions, err := repo.GetVersions(context.Background(), id)
	require.NoError(t, err)
	require.Len(t, versions, 1)
	assert.Equal(t, "1.0.0", versions[0].Version)
	assert.Equal(t, "abc123", versions[0].Checksums["linux_amd64"])
}

func TestFileRepo_AddVersion_DuplicateVersionReturnsError(t *testing.T) {
	repo := newTestFileRepo(t)
	id, _ := repo.Create(context.Background(), basePlugin("plugin"))

	v := &models.PluginVersion{PluginID: id, Version: "1.0.0"}
	_, err := repo.AddVersion(context.Background(), v)
	require.NoError(t, err)

	_, err = repo.AddVersion(context.Background(), &models.PluginVersion{PluginID: id, Version: "1.0.0"})
	assert.ErrorIs(t, err, appErrors.ErrDuplicatePlugin)
}

func TestFileRepo_AddVersion_NilReturnsError(t *testing.T) {
	repo := newTestFileRepo(t)
	_, err := repo.AddVersion(context.Background(), nil)
	assert.Error(t, err)
}

func TestFileRepo_AddVersion_SortsByReleaseDateDesc(t *testing.T) {
	repo := newTestFileRepo(t)
	id, _ := repo.Create(context.Background(), basePlugin("plugin"))

	older := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	_, _ = repo.AddVersion(context.Background(), &models.PluginVersion{PluginID: id, Version: "1.0.0", ReleaseDate: &older})
	_, _ = repo.AddVersion(context.Background(), &models.PluginVersion{PluginID: id, Version: "2.0.0", ReleaseDate: &newer})

	versions, err := repo.GetVersions(context.Background(), id)
	require.NoError(t, err)
	require.Len(t, versions, 2)
	assert.Equal(t, "2.0.0", versions[0].Version)
}

func TestFileRepo_GetVersions_EmptyForNewPlugin(t *testing.T) {
	repo := newTestFileRepo(t)
	id, _ := repo.Create(context.Background(), basePlugin("plugin"))

	versions, err := repo.GetVersions(context.Background(), id)
	require.NoError(t, err)
	assert.Empty(t, versions)
}

func TestFileRepo_GetVersions_PluginNotFound(t *testing.T) {
	repo := newTestFileRepo(t)
	_, err := repo.GetVersions(context.Background(), 999)
	assert.ErrorIs(t, err, appErrors.ErrPluginNotFound)
}

// -------------------------------------------------------------------------
// ID autoincrement
// -------------------------------------------------------------------------

func TestFileRepo_IDsAutoIncrement(t *testing.T) {
	repo := newTestFileRepo(t)

	id1, _ := repo.Create(context.Background(), basePlugin("a"))
	id2, _ := repo.Create(context.Background(), basePlugin("b"))
	id3, _ := repo.Create(context.Background(), basePlugin("c"))

	assert.Equal(t, int64(1), id1)
	assert.Equal(t, int64(2), id2)
	assert.Equal(t, int64(3), id3)
}

// -------------------------------------------------------------------------
// Persistence across repository instances
// -------------------------------------------------------------------------

func TestFileRepo_PersistsAcrossInstances(t *testing.T) {
	dir := t.TempDir()

	repo1, err := NewFileRepository(dir)
	require.NoError(t, err)
	id, err := repo1.Create(context.Background(), basePlugin("provider-github"))
	require.NoError(t, err)

	// Create a second instance pointing at the same directory.
	repo2, err := NewFileRepository(dir)
	require.NoError(t, err)

	got, err := repo2.GetByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "provider-github", got.Name)
}
