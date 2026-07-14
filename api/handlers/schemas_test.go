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
	resp := performSchemaRequest(t, http.MethodGet, "/schemas/plugins/conventional/v1.json")

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, "application/schema+json", resp.Header().Get("Content-Type"))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
	assert.Equal(t, "https://registry.semrel.io/schemas/plugins/conventional/v1.json", payload["$id"])
}

func TestSchemaHandlerMissingPluginSchema(t *testing.T) {
	resp := performSchemaRequest(t, http.MethodGet, "/schemas/plugins/nonexistent/v1.json")

	require.Equal(t, http.StatusNotFound, resp.Code)
	assert.Contains(t, resp.Body.String(), "schema not found")
}

func TestSchemaHandlerLatestPluginRedirect(t *testing.T) {
	resp := performSchemaRequest(t, http.MethodGet, "/schemas/plugins/conventional/latest.json")

	require.Equal(t, http.StatusMovedPermanently, resp.Code)
	assert.Equal(t, "/schemas/plugins/conventional/v1.json", resp.Header().Get("Location"))
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

func TestSchemaHandlerAllOfficialPlugins(t *testing.T) {
	// Verify that every official plugin has an embedded schema served correctly.
	plugins := []string{
		"github", "gitlab", "gitea", "git", "bitbucket",
		"github-actions", "gitlab-ci", "gitea-actions", "generic", "circleci", "bitbucket-pipelines",
		"conventional", "default",
		"changelog-md", "changelog-html", "release-notes",
		"slack", "teams", "email", "jira", "matrix", "gitplugin", "discord",
		"go", "npm", "cargo", "docker", "helm", "gradle", "maven", "composer",
		"python", "terraform", "homebrew", "nuget", "pubspec",
		"nfpm", "generic-http", "oci", "crates", "pypi", "publisher-npm",
	}
	for _, name := range plugins {
		t.Run(name, func(t *testing.T) {
			resp := performSchemaRequest(t, http.MethodGet, "/schemas/plugins/"+name+"/v1.json")
			require.Equal(t, http.StatusOK, resp.Code, "plugin %s: expected 200", name)
			assert.Equal(t, "application/schema+json", resp.Header().Get("Content-Type"))

			var payload map[string]any
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload), "plugin %s: invalid JSON", name)
			expectedID := "https://registry.semrel.io/schemas/plugins/" + name + "/v1.json"
			assert.Equal(t, expectedID, payload["$id"], "plugin %s: wrong $id", name)
		})
	}
}
