package handlers

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed schemas
var schemaFS embed.FS

// SchemaHandler serves versioned JSON Schema documents.
type SchemaHandler struct{}

func NewSchemaHandler() *SchemaHandler { return &SchemaHandler{} }

// GetCoreSchema serves GET /schemas/core/v{N}.json.
func (h *SchemaHandler) GetCoreSchema(c *gin.Context) {
	version := c.Param("version")
	path := "schemas/core/" + schemaVersionFile(version)
	h.serveSchema(c, path)
}

// GetPluginSchema serves GET /schemas/plugins/{name}/v{N}.json.
func (h *SchemaHandler) GetPluginSchema(c *gin.Context) {
	name := c.Param("name")
	version := c.Param("version")
	if isLatestSchema(version) {
		c.Redirect(http.StatusMovedPermanently,
			strings.Replace(c.Request.URL.Path, "/latest.json", "/v1.json", 1))
		return
	}
	path := "schemas/plugins/" + name + "/" + schemaVersionFile(version)
	h.serveSchema(c, path)
}

// GetNamespacedPluginSchema serves GET /schemas/plugins/@{namespace}/{name}/v{N}.json.
func (h *SchemaHandler) GetNamespacedPluginSchema(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	version := c.Param("version")
	if isLatestSchema(version) {
		c.Redirect(http.StatusMovedPermanently,
			strings.Replace(c.Request.URL.Path, "/latest.json", "/v1.json", 1))
		return
	}
	path := "schemas/plugins/@" + namespace + "/" + name + "/" + schemaVersionFile(version)
	h.serveSchema(c, path)
}

func (h *SchemaHandler) serveSchema(c *gin.Context, path string) {
	data, err := fs.ReadFile(schemaFS, path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "schema not found", "path": path})
		return
	}
	c.Header("Content-Type", "application/schema+json")
	c.Header("Cache-Control", "public, max-age=86400")
	c.Data(http.StatusOK, "application/schema+json", data)
}

func schemaVersionFile(version string) string {
	if strings.HasSuffix(version, ".json") {
		return version
	}
	return version + ".json"
}

func isLatestSchema(version string) bool {
	return version == "latest" || version == "latest.json"
}
