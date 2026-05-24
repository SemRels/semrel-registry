package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/SemRels/semrel-registry/api/database"
	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubPluginService struct{}

func (stubPluginService) ListPlugins(_ context.Context, params service.ListPluginsParams) (service.PluginListResult, error) {
	return service.PluginListResult{Data: []models.Plugin{}, Pagination: service.Pagination{Page: params.Page, Limit: params.Limit}}, nil
}

func (stubPluginService) GetPlugin(_ context.Context, _ string) (models.Plugin, error) {
	return models.Plugin{}, nil
}

func (stubPluginService) ListVersions(_ context.Context, _ string, _, _ int) ([]models.PluginVersion, error) {
	return []models.PluginVersion{}, nil
}

func (stubPluginService) CreatePlugin(_ context.Context, plugin models.Plugin) (models.Plugin, error) {
	return plugin, nil
}

func (stubPluginService) UpdatePlugin(_ context.Context, _ string, _ models.PluginPatch) (models.Plugin, error) {
	return models.Plugin{}, nil
}

func (stubPluginService) DeletePlugin(_ context.Context, _ string) error {
	return nil
}

func (stubPluginService) CreateVersion(_ context.Context, _ string, version models.PluginVersion) (models.PluginVersion, error) {
	return version, nil
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func TestServerStartup(t *testing.T) {
	server := httptest.NewServer(newRouter(stubPluginService{}))
	defer server.Close()

	assert.NotEmpty(t, server.URL)
}

func TestNewRouterRegistersCoreRoutes(t *testing.T) {
	router := newRouter(stubPluginService{})

	routes := router.Routes()
	assert.NotEmpty(t, routes)

	var routePaths []string
	for _, route := range routes {
		routePaths = append(routePaths, route.Path)
	}

	assert.Contains(t, routePaths, "/health")
	assert.Contains(t, routePaths, "/api/v1/plugins")
	assert.Contains(t, routePaths, "/api/v1/plugins/:id")
	assert.Contains(t, routePaths, "/api/v1/plugins/:id/versions")
}

func TestHealthEndpoint(t *testing.T) {
	server := httptest.NewServer(newRouter(stubPluginService{}))
	defer server.Close()

	response, err := http.Get(server.URL + "/health")
	require.NoError(t, err)
	defer response.Body.Close()

	assert.Equal(t, http.StatusOK, response.StatusCode)

	var payload map[string]string
	err = json.NewDecoder(response.Body).Decode(&payload)
	require.NoError(t, err)
	assert.Equal(t, "ok", payload["status"])
}

func TestDatabaseConnection(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration database test")
	}

	db, err := database.Connect(databaseURL)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})

	err = db.Health()
	require.NoError(t, err)
}
