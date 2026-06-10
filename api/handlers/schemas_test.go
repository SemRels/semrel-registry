package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaHandlerCoreSchema(t *testing.T) {
	resp := performSchemaRequest(t, http.MethodGet, "/schemas/core/v1.json")

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, "application/schema+json", resp.Header().Get("Content-Type"))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
	assert.Equal(t, "https://registry.semrel.io/schemas/core/v1.json", payload["$id"])
}

func TestSchemaHandlerPluginSchema(t *testing.T) {
	resp := performSchemaRequest(t, http.MethodGet, "/schemas/plugins/analyzer-conventional/v1.json")

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, "application/schema+json", resp.Header().Get("Content-Type"))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
	assert.Equal(t, "analyzer-conventional plugin schema", payload["title"])
	assert.Equal(t, "https://registry.semrel.io/schemas/plugins/analyzer-conventional/v1.json", payload["$id"])
}

func TestSchemaHandlerMissingPluginSchema(t *testing.T) {
	resp := performSchemaRequest(t, http.MethodGet, "/schemas/plugins/nonexistent/v1.json")

	require.Equal(t, http.StatusNotFound, resp.Code)
	assert.Contains(t, resp.Body.String(), "schema not found")
}

func TestSchemaHandlerLatestPluginRedirect(t *testing.T) {
	resp := performSchemaRequest(t, http.MethodGet, "/schemas/plugins/analyzer-conventional/latest.json")

	require.Equal(t, http.StatusMovedPermanently, resp.Code)
	assert.Equal(t, "/schemas/plugins/analyzer-conventional/v1.json", resp.Header().Get("Location"))
}

func schemaTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewSchemaHandler()
	router.GET("/schemas/core/:version", handler.GetCoreSchema)
	router.GET("/schemas/plugins/:name/:version", handler.GetPluginSchema)
	router.GET("/schemas/plugins/@:namespace/:name/:version", handler.GetNamespacedPluginSchema)
	return router
}

func performSchemaRequest(t *testing.T, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	resp := httptest.NewRecorder()
	schemaTestRouter().ServeHTTP(resp, req)
	return resp
}
