package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/service"
	"github.com/gin-gonic/gin"
)

type PluginHandler struct {
	service service.PluginManager
}

func NewPluginHandler(pluginService service.PluginManager) *PluginHandler {
	return &PluginHandler{service: pluginService}
}

func Health() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func (h *PluginHandler) ListPlugins(c *gin.Context) {
	page, ok := parseQueryInt(c, "page", 1)
	if !ok {
		return
	}
	limit, ok := parseQueryInt(c, "limit", 20)
	if !ok {
		return
	}

	result, err := h.service.ListPlugins(c.Request.Context(), service.ListPluginsParams{
		Page:     page,
		Limit:    limit,
		Category: strings.TrimSpace(c.Query("category")),
		Search:   strings.TrimSpace(c.Query("search")),
		Sort:     strings.TrimSpace(c.Query("sort")),
	})
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result.Data, "pagination": result.Pagination})
}

func (h *PluginHandler) GetPlugin(c *gin.Context) {
	plugin, err := h.service.GetPlugin(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": plugin})
}

func (h *PluginHandler) ListPluginVersions(c *gin.Context) {
	limit, ok := parseQueryInt(c, "limit", 20)
	if !ok {
		return
	}
	offset, ok := parseQueryInt(c, "offset", 0)
	if !ok {
		return
	}

	versions, err := h.service.ListVersions(c.Request.Context(), c.Param("id"), limit, offset)
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": versions})
}

func (h *PluginHandler) CreatePlugin(c *gin.Context) {
	var plugin models.Plugin
	if err := c.ShouldBindJSON(&plugin); err != nil {
		BadRequest(c, "Invalid request body", gin.H{"issue": err.Error()})
		return
	}

	created, err := h.service.CreatePlugin(c.Request.Context(), plugin)
	if err != nil {
		HandleError(c, err)
		return
	}

	c.Header("Location", fmt.Sprintf("/api/v1/plugins/%d", created.ID))
	c.JSON(http.StatusCreated, gin.H{"data": created})
}

func (h *PluginHandler) UpdatePlugin(c *gin.Context) {
	var patch models.PluginPatch
	if err := c.ShouldBindJSON(&patch); err != nil {
		BadRequest(c, "Invalid request body", gin.H{"issue": err.Error()})
		return
	}

	updated, err := h.service.UpdatePlugin(c.Request.Context(), c.Param("id"), patch)
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": updated})
}

func (h *PluginHandler) DeletePlugin(c *gin.Context) {
	if err := h.service.DeletePlugin(c.Request.Context(), c.Param("id")); err != nil {
		HandleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *PluginHandler) CreatePluginVersion(c *gin.Context) {
	var version models.PluginVersion
	if err := c.ShouldBindJSON(&version); err != nil {
		BadRequest(c, "Invalid request body", gin.H{"issue": err.Error()})
		return
	}

	created, err := h.service.CreateVersion(c.Request.Context(), c.Param("id"), version)
	if err != nil {
		HandleError(c, err)
		return
	}

	c.Header("Location", fmt.Sprintf("/api/v1/plugins/%s/versions/%d", c.Param("id"), created.ID))
	c.JSON(http.StatusCreated, gin.H{"data": created})
}

func parseQueryInt(c *gin.Context, name string, defaultValue int) (int, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return defaultValue, true
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		BadRequest(c, "Invalid request parameters", gin.H{"field": name, "issue": "must be an integer"})
		return 0, false
	}

	return value, true
}
