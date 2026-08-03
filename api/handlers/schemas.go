package handlers

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"

	"github.com/SemRels/semrel-registry/api/naming"
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
	if plugin, ok := naming.ResolveFirstPartyRef(name); ok && name != plugin.Name {
		c.Redirect(http.StatusMovedPermanently,
			"/schemas/plugins/"+plugin.Name+"/"+schemaVersionFile(version))
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
	canonicalID := ""
	if strings.EqualFold("@"+namespace, naming.FirstPartyNamespace) {
		if plugin, ok := naming.ResolveFirstPartyRef("@" + namespace + "/" + name); ok {
			if name != plugin.Name {
				c.Redirect(http.StatusMovedPermanently,
					"/schemas/plugins/@semrel/"+plugin.Name+"/"+schemaVersionFile(version))
				return
			}
			canonicalID = "https://registry.semrel.io/schemas/plugins/@semrel/" + plugin.Name + "/" + schemaVersionFile(version)
			path = "schemas/plugins/" + plugin.Name + "/" + schemaVersionFile(version)
		}
	}
	h.serveSchemaWithID(c, path, canonicalID)
}

func (h *SchemaHandler) serveSchema(c *gin.Context, path string) {
	h.serveSchemaWithID(c, path, "")
}

func (h *SchemaHandler) serveSchemaWithID(c *gin.Context, path, canonicalID string) {
	data, err := fs.ReadFile(schemaFS, path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "schema not found", "path": path})
		return
	}
	if canonicalID != "" {
		var schema map[string]any
		if err := json.Unmarshal(data, &schema); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid embedded schema", "path": path})
			return
		}
		schema["$id"] = canonicalID
		data, err = json.Marshal(schema)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot serialize schema", "path": path})
			return
		}
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
