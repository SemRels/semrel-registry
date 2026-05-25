package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	appErrors "github.com/SemRels/semrel-registry/api/internal"
	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/repository"
	"github.com/SemRels/semrel-registry/api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPluginRepository struct {
	listFunc          func(context.Context, service.ListPluginsParams) ([]models.Plugin, int64, error)
	getFunc           func(context.Context, string) (models.Plugin, error)
	listVersionsFunc  func(context.Context, string, int, int) ([]models.PluginVersion, error)
	createFunc        func(context.Context, models.Plugin) (models.Plugin, error)
	updateFunc        func(context.Context, string, models.PluginPatch) (models.Plugin, error)
	deleteFunc        func(context.Context, string) error
	createVersionFunc func(context.Context, string, models.PluginVersion) (models.PluginVersion, error)
}

func (m *mockPluginRepository) ListPlugins(ctx context.Context, params service.ListPluginsParams) (service.PluginListResult, error) {
	if m.listFunc != nil {
		plugins, total, err := m.listFunc(ctx, params)
		if err != nil {
			return service.PluginListResult{}, err
		}
		return service.PluginListResult{Data: plugins, Pagination: service.Pagination{Page: params.Page, Limit: params.Limit, Total: total}}, nil
	}
	return service.PluginListResult{Data: []models.Plugin{}, Pagination: service.Pagination{Page: params.Page, Limit: params.Limit}}, nil
}

func (m *mockPluginRepository) GetPlugin(ctx context.Context, ref string) (models.Plugin, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, ref)
	}
	return models.Plugin{}, nil
}

type mockRepositoryAdapter struct {
	mock         *mockPluginRepository
	listCache    []models.Plugin
	createdByPID map[int64][]models.PluginVersion
}

func (a *mockRepositoryAdapter) GetAll(ctx context.Context, limit, offset int, filters ...repository.Filter) ([]models.Plugin, error) {
	if limit == 0 && a.listCache != nil {
		return a.listCache, nil
	}

	params := service.ListPluginsParams{Page: 1, Limit: limit}
	if params.Limit == 0 {
		params.Limit = 20
	}
	if limit > 0 {
		params.Page = (offset / limit) + 1
	}
	for _, filter := range filters {
		switch f := filter.(type) {
		case repository.CategoryFilter:
			params.Category = f.Category
		case repository.SearchFilter:
			params.Search = f.Query
		case repository.SortFilter:
			params.Sort = f.Field
		}
	}
	if a.mock.listFunc == nil {
		return []models.Plugin{}, nil
	}
	plugins, _, err := a.mock.listFunc(ctx, params)
	if err == nil {
		a.listCache = plugins
	}
	return plugins, err
}

func (a *mockRepositoryAdapter) GetByID(ctx context.Context, id int64) (*models.Plugin, error) {
	plugin, err := a.mock.GetPlugin(ctx, strconv.FormatInt(id, 10))
	if err != nil {
		return nil, err
	}
	plugin.ID = id
	return &plugin, nil
}

func (a *mockRepositoryAdapter) GetByName(ctx context.Context, name string) (*models.Plugin, error) {
	plugin, err := a.mock.GetPlugin(ctx, name)
	if err != nil {
		return nil, err
	}
	return &plugin, nil
}

func (a *mockRepositoryAdapter) GetVersions(ctx context.Context, pluginID int64) ([]models.PluginVersion, error) {
	if a.mock.listVersionsFunc != nil {
		return a.mock.listVersionsFunc(ctx, strconv.FormatInt(pluginID, 10), 20, 0)
	}
	if a.createdByPID != nil {
		if versions, ok := a.createdByPID[pluginID]; ok {
			return versions, nil
		}
	}
	return []models.PluginVersion{}, nil
}

func (a *mockRepositoryAdapter) Create(ctx context.Context, plugin *models.Plugin) (int64, error) {
	if a.mock.createFunc == nil {
		plugin.ID = 1
		return plugin.ID, nil
	}
	created, err := a.mock.createFunc(ctx, *plugin)
	if err != nil {
		return 0, err
	}
	*plugin = created
	if plugin.ID == 0 {
		plugin.ID = 1
	}
	return plugin.ID, nil
}

func (a *mockRepositoryAdapter) Update(ctx context.Context, plugin *models.Plugin) error {
	if a.mock.updateFunc == nil {
		return nil
	}
	_, err := a.mock.updateFunc(ctx, strconv.FormatInt(plugin.ID, 10), models.PluginPatch{Description: &plugin.Description})
	return err
}

func (a *mockRepositoryAdapter) Delete(ctx context.Context, id int64) error {
	if a.mock.deleteFunc == nil {
		return nil
	}
	return a.mock.deleteFunc(ctx, strconv.FormatInt(id, 10))
}

func (a *mockRepositoryAdapter) AddVersion(ctx context.Context, version *models.PluginVersion) (int64, error) {
	if a.mock.createVersionFunc != nil {
		created, err := a.mock.createVersionFunc(ctx, strconv.FormatInt(version.PluginID, 10), *version)
		if err != nil {
			return 0, err
		}
		*version = created
	} else if version.ID == 0 {
		version.ID = 1
	}
	if a.createdByPID == nil {
		a.createdByPID = make(map[int64][]models.PluginVersion)
	}
	a.createdByPID[version.PluginID] = append(a.createdByPID[version.PluginID], *version)
	return version.ID, nil
}

func (m *mockPluginRepository) ListVersions(ctx context.Context, ref string, limit, offset int) ([]models.PluginVersion, error) {
	if m.listVersionsFunc != nil {
		return m.listVersionsFunc(ctx, ref, limit, offset)
	}
	return []models.PluginVersion{}, nil
}

func (m *mockPluginRepository) CreatePlugin(ctx context.Context, plugin models.Plugin) (models.Plugin, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, plugin)
	}
	return plugin, nil
}

func (m *mockPluginRepository) UpdatePlugin(ctx context.Context, ref string, patch models.PluginPatch) (models.Plugin, error) {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, ref, patch)
	}
	return models.Plugin{}, nil
}

func (m *mockPluginRepository) DeletePlugin(ctx context.Context, ref string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, ref)
	}
	return nil
}

func (m *mockPluginRepository) CreateVersion(ctx context.Context, ref string, version models.PluginVersion) (models.PluginVersion, error) {
	if m.createVersionFunc != nil {
		return m.createVersionFunc(ctx, ref, version)
	}
	return version, nil
}

type listResponse struct {
	Data       []models.Plugin    `json:"data"`
	Pagination service.Pagination `json:"pagination"`
}

type pluginResponse struct {
	Data models.Plugin `json:"data"`
}

type versionResponse struct {
	Data models.PluginVersion `json:"data"`
}

type versionsResponse struct {
	Data []models.PluginVersion `json:"data"`
}

type errorResponse struct {
	Error ApiError `json:"error"`
}

func TestListPluginsSuccess(t *testing.T) {
	repo := &mockPluginRepository{
		listFunc: func(_ context.Context, params service.ListPluginsParams) ([]models.Plugin, int64, error) {
			assert.Equal(t, 1, params.Page)
			assert.Equal(t, 20, params.Limit)
			return []models.Plugin{{ID: 1, Name: "provider-github", LatestVersion: "0.1.0"}}, 1, nil
		},
	}

	resp := performRequest(t, newPluginTestRouter(repo), http.MethodGet, "/api/v1/plugins", nil, "")
	assert.Equal(t, http.StatusOK, resp.Code)

	var payload listResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
	assert.Len(t, payload.Data, 1)
	assert.Equal(t, int64(1), payload.Pagination.Total)
}

func TestListPluginsPassesPaginationAndFilters(t *testing.T) {
	repo := &mockPluginRepository{listFunc: func(_ context.Context, params service.ListPluginsParams) ([]models.Plugin, int64, error) {
		assert.Equal(t, 2, params.Page)
		assert.Equal(t, 10, params.Limit)
		assert.Equal(t, "provider", params.Category)
		assert.Equal(t, "github", params.Search)
		assert.Equal(t, "updated_at", params.Sort)
		return []models.Plugin{}, 0, nil
	}}

	resp := performRequest(t, newPluginTestRouter(repo), http.MethodGet, "/api/v1/plugins?page=2&limit=10&category=provider&search=github&sort=updated_at", nil, "")
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestListPluginsRejectsInvalidPage(t *testing.T) {
	repo := &mockPluginRepository{}
	resp := performRequest(t, newPluginTestRouter(repo), http.MethodGet, "/api/v1/plugins?page=abc", nil, "")
	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assertErrorCode(t, resp, "VALIDATION_ERROR")
}

func TestListPluginsRejectsInvalidSort(t *testing.T) {
	repo := &mockPluginRepository{}
	resp := performRequest(t, newPluginTestRouter(repo), http.MethodGet, "/api/v1/plugins?sort=unknown", nil, "")
	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assertErrorCode(t, resp, "VALIDATION_ERROR")
}

func TestGetPluginByIDSuccess(t *testing.T) {
	repo := &mockPluginRepository{getFunc: func(_ context.Context, ref string) (models.Plugin, error) {
		assert.Equal(t, "1", ref)
		return samplePlugin(), nil
	}}

	resp := performRequest(t, newPluginTestRouter(repo), http.MethodGet, "/api/v1/plugins/1", nil, "")
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestGetPluginByNameSuccess(t *testing.T) {
	repo := &mockPluginRepository{getFunc: func(_ context.Context, ref string) (models.Plugin, error) {
		assert.Equal(t, "provider-github", ref)
		return samplePlugin(), nil
	}}

	resp := performRequest(t, newPluginTestRouter(repo), http.MethodGet, "/api/v1/plugins/provider-github", nil, "")
	assert.Equal(t, http.StatusOK, resp.Code)

	var payload pluginResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
	assert.Equal(t, "provider-github", payload.Data.Name)
}

func TestGetPluginNotFound(t *testing.T) {
	repo := &mockPluginRepository{getFunc: func(_ context.Context, _ string) (models.Plugin, error) {
		return models.Plugin{}, appErrors.ErrPluginNotFound
	}}

	resp := performRequest(t, newPluginTestRouter(repo), http.MethodGet, "/api/v1/plugins/missing", nil, "")
	assert.Equal(t, http.StatusNotFound, resp.Code)
	assertErrorCode(t, resp, "NOT_FOUND")
}

func TestListPluginVersionsSuccess(t *testing.T) {
	repo := &mockPluginRepository{listVersionsFunc: func(_ context.Context, ref string, _, _ int) ([]models.PluginVersion, error) {
		assert.Equal(t, "1", ref)
		return []models.PluginVersion{sampleVersion()}, nil
	}}

	resp := performRequest(t, newPluginTestRouter(repo), http.MethodGet, "/api/v1/plugins/1/versions", nil, "")
	assert.Equal(t, http.StatusOK, resp.Code)

	var payload versionsResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
	assert.Len(t, payload.Data, 1)
}

func TestListPluginVersionsRejectsInvalidOffset(t *testing.T) {
	repo := &mockPluginRepository{}
	resp := performRequest(t, newPluginTestRouter(repo), http.MethodGet, "/api/v1/plugins/1/versions?offset=oops", nil, "")
	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestCreatePluginSuccess(t *testing.T) {
	setAdminToken(t, "secret")
	repo := &mockPluginRepository{createFunc: func(_ context.Context, plugin models.Plugin) (models.Plugin, error) {
		assert.Equal(t, "provider-github", plugin.Name)
		plugin.ID = 42
		return plugin, nil
	}}

	resp := performRequest(t, newPluginTestRouter(repo), http.MethodPost, "/api/v1/plugins", map[string]any{"name": "provider-github", "description": "GitHub Provider", "category": "provider"}, "secret")
	assert.Equal(t, http.StatusCreated, resp.Code)
	assert.Equal(t, "/api/v1/plugins/42", resp.Header().Get("Location"))
}

func TestCreatePluginValidationError(t *testing.T) {
	setAdminToken(t, "secret")
	repo := &mockPluginRepository{}
	resp := performRequest(t, newPluginTestRouter(repo), http.MethodPost, "/api/v1/plugins", map[string]any{"description": "missing name", "category": "provider"}, "secret")
	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assertErrorCode(t, resp, "VALIDATION_ERROR")
}

func TestCreatePluginDuplicateConflict(t *testing.T) {
	setAdminToken(t, "secret")
	repo := &mockPluginRepository{createFunc: func(_ context.Context, _ models.Plugin) (models.Plugin, error) {
		return models.Plugin{}, appErrors.ErrDuplicatePlugin
	}}

	resp := performRequest(t, newPluginTestRouter(repo), http.MethodPost, "/api/v1/plugins", map[string]any{"name": "provider-github", "category": "provider"}, "secret")
	assert.Equal(t, http.StatusConflict, resp.Code)
	assertErrorCode(t, resp, "CONFLICT")
}

func TestCreatePluginUnauthorized(t *testing.T) {
	setAdminToken(t, "secret")
	repo := &mockPluginRepository{}
	resp := performRequest(t, newPluginTestRouter(repo), http.MethodPost, "/api/v1/plugins", map[string]any{"name": "provider-github", "category": "provider"}, "")
	assert.Equal(t, http.StatusUnauthorized, resp.Code)
	assertErrorCode(t, resp, "UNAUTHORIZED")
}

func TestUpdatePluginSuccess(t *testing.T) {
	setAdminToken(t, "secret")
	repo := &mockPluginRepository{updateFunc: func(_ context.Context, ref string, patch models.PluginPatch) (models.Plugin, error) {
		assert.Equal(t, "1", ref)
		if assert.NotNil(t, patch.Description) {
			assert.Equal(t, "Updated", *patch.Description)
		}
		return samplePlugin(), nil
	}}

	resp := performRequest(t, newPluginTestRouter(repo), http.MethodPut, "/api/v1/plugins/1", map[string]any{"description": "Updated"}, "secret")
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestUpdatePluginNotFound(t *testing.T) {
	setAdminToken(t, "secret")
	repo := &mockPluginRepository{updateFunc: func(_ context.Context, _ string, _ models.PluginPatch) (models.Plugin, error) {
		return models.Plugin{}, appErrors.ErrPluginNotFound
	}}

	resp := performRequest(t, newPluginTestRouter(repo), http.MethodPut, "/api/v1/plugins/1", map[string]any{"description": "Updated"}, "secret")
	assert.Equal(t, http.StatusNotFound, resp.Code)
}

func TestUpdatePluginValidationError(t *testing.T) {
	setAdminToken(t, "secret")
	repo := &mockPluginRepository{}
	resp := performRequest(t, newPluginTestRouter(repo), http.MethodPut, "/api/v1/plugins/1", map[string]any{}, "secret")
	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestDeletePluginSuccess(t *testing.T) {
	setAdminToken(t, "secret")
	repo := &mockPluginRepository{deleteFunc: func(_ context.Context, ref string) error {
		assert.Equal(t, "1", ref)
		return nil
	}}

	resp := performRequest(t, newPluginTestRouter(repo), http.MethodDelete, "/api/v1/plugins/1", nil, "secret")
	assert.Equal(t, http.StatusNoContent, resp.Code)
}

func TestDeletePluginNotFound(t *testing.T) {
	setAdminToken(t, "secret")
	repo := &mockPluginRepository{deleteFunc: func(_ context.Context, _ string) error {
		return appErrors.ErrPluginNotFound
	}}

	resp := performRequest(t, newPluginTestRouter(repo), http.MethodDelete, "/api/v1/plugins/1", nil, "secret")
	assert.Equal(t, http.StatusNotFound, resp.Code)
}

func TestDeletePluginUnauthorized(t *testing.T) {
	setAdminToken(t, "secret")
	repo := &mockPluginRepository{}
	resp := performRequest(t, newPluginTestRouter(repo), http.MethodDelete, "/api/v1/plugins/1", nil, "")
	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestCreateVersionSuccess(t *testing.T) {
	setAdminToken(t, "secret")
	repo := &mockPluginRepository{createVersionFunc: func(_ context.Context, ref string, version models.PluginVersion) (models.PluginVersion, error) {
		assert.Equal(t, "1", ref)
		assert.Equal(t, "1.2.3", version.Version)
		version.ID = 7
		return version, nil
	}}

	resp := performRequest(t, newPluginTestRouter(repo), http.MethodPost, "/api/v1/plugins/1/versions", map[string]any{"version": "1.2.3", "downloadUrl": "https://example.test/plugin.tar.gz", "checksums": map[string]string{"linux-amd64:sha256": "abc123"}}, "secret")
	assert.Equal(t, http.StatusCreated, resp.Code)

	var payload versionResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
	assert.Equal(t, int64(7), payload.Data.ID)
}

func TestCreateVersionValidationError(t *testing.T) {
	setAdminToken(t, "secret")
	repo := &mockPluginRepository{}
	resp := performRequest(t, newPluginTestRouter(repo), http.MethodPost, "/api/v1/plugins/1/versions", map[string]any{"downloadUrl": "https://example.test/plugin.tar.gz"}, "secret")
	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assertErrorCode(t, resp, "VALIDATION_ERROR")
}

func newPluginTestRouter(repo *mockPluginRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ErrorHandler(), CORSMiddleware())

	handler := NewPluginHandler(service.NewPluginService(&mockRepositoryAdapter{mock: repo}))
	api := router.Group("/api/v1")
	api.GET("/plugins", handler.ListPlugins)
	api.GET("/plugins/:id", handler.GetPlugin)
	api.GET("/plugins/:id/versions", handler.ListPluginVersions)

	protected := api.Group("")
	protected.Use(RequireAdminToken())
	protected.POST("/plugins", handler.CreatePlugin)
	protected.PUT("/plugins/:id", handler.UpdatePlugin)
	protected.DELETE("/plugins/:id", handler.DeletePlugin)
	protected.POST("/plugins/:id/versions", handler.CreatePluginVersion)

	return router
}

func performRequest(t *testing.T, router *gin.Engine, method, target string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()

	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		require.NoError(t, err)
		requestBody = bytes.NewReader(payload)
	}

	req := httptest.NewRequest(method, target, requestBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func assertErrorCode(t *testing.T, resp *httptest.ResponseRecorder, code string) {
	t.Helper()
	var payload errorResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
	assert.Equal(t, code, payload.Error.Code)
}

func setAdminToken(t *testing.T, token string) {
	t.Helper()
	previous, exists := os.LookupEnv("ADMIN_TOKEN")
	require.NoError(t, os.Setenv("ADMIN_TOKEN", token))
	t.Cleanup(func() {
		if exists {
			_ = os.Setenv("ADMIN_TOKEN", previous)
			return
		}
		_ = os.Unsetenv("ADMIN_TOKEN")
	})
}

func samplePlugin() models.Plugin {
	now := time.Date(2026, 5, 24, 15, 0, 0, 0, time.UTC)
	return models.Plugin{
		ID:            1,
		Name:          "provider-github",
		Description:   "GitHub Provider",
		Author:        "GoSemantics",
		Category:      "provider",
		LatestVersion: "0.1.0",
		UpdatedAt:     now,
		CreatedAt:     now,
		Versions:      []models.PluginVersion{sampleVersion()},
	}
}

func sampleVersion() models.PluginVersion {
	now := time.Date(2026, 5, 24, 15, 0, 0, 0, time.UTC)
	return models.PluginVersion{
		ID:          1,
		PluginID:    1,
		Version:     "0.1.0",
		ReleaseDate: &now,
		DownloadURL: "https://example.test/plugin.tar.gz",
		Checksums:   map[string]string{"linux-amd64:sha256": "abc123"},
		CreatedAt:   now,
	}
}
