package handlers

import (
	"context"
	"encoding/json"
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

	// Visibility rules:
	//   admin       → all statuses; any author filter accepted
	//   auth user   → own plugins only (all statuses); author forced to own login
	//   public      → active only; any author filter accepted
	var statuses []string
	var forcedAuthor string

	isAdmin, _ := c.Get("isAdmin")
	login, _    := c.Get("login")
	loginStr, _ := login.(string)

	if isAdmin == true {
		// Admin: honour requested status filter (empty = all statuses).
		if s := strings.TrimSpace(c.Query("status")); s != "" {
			statuses = []string{s}
		}
	} else if loginStr != "" {
		// Authenticated non-admin: always scope to own plugins only.
		forcedAuthor = loginStr
		// No status restriction — they can see pending/rejected of their own.
	} else {
		// Unauthenticated (public registry): active only.
		statuses = []string{models.StatusActive}
	}

	// Resolve author: forced author (for non-admin auth) takes priority over query param.
	author := strings.TrimSpace(c.Query("author"))
	if forcedAuthor != "" {
		author = forcedAuthor
	}

	result, err := h.service.ListPlugins(c.Request.Context(), service.ListPluginsParams{
		Page:     page,
		Limit:    limit,
		Category: strings.TrimSpace(c.Query("category")),
		Search:   strings.TrimSpace(c.Query("search")),
		Sort:     strings.TrimSpace(c.Query("sort")),
		Author:   author,
		Statuses: statuses,
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

	// Enrich each version with per-platform download URLs derived from the stored base URL.
	type versionResponse struct {
		models.PluginVersion
		DownloadURLs map[string]string `json:"downloadUrls,omitempty"`
	}
	out := make([]versionResponse, len(versions))
	for i, v := range versions {
		out[i] = versionResponse{
			PluginVersion: v,
			DownloadURLs:  deriveDownloadURLs(v.DownloadURL, v.Checksums),
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *PluginHandler) CreatePlugin(c *gin.Context) {
	var plugin models.Plugin
	if err := c.ShouldBindJSON(&plugin); err != nil {
		BadRequest(c, "Invalid request body", gin.H{"issue": err.Error()})
		return
	}

	// Non-admin users can only create plugins attributed to themselves.
	if isAdmin, _ := c.Get("isAdmin"); isAdmin != true {
		login, _ := c.Get("login")
		if loginStr, ok := login.(string); ok && loginStr != "" {
			plugin.Author = loginStr
		}
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

	// Non-admin users can only update their own plugins.
	if isAdmin, _ := c.Get("isAdmin"); isAdmin != true {
		login, _ := c.Get("login")
		loginStr, _ := login.(string)
		existing, err := h.service.GetPlugin(c.Request.Context(), c.Param("id"))
		if err != nil {
			HandleError(c, err)
			return
		}
		if !strings.EqualFold(existing.Author, loginStr) {
			c.JSON(http.StatusForbidden, gin.H{"error": "you can only edit your own plugins", "author": existing.Author})
			return
		}
		// Non-admin cannot change the author field to someone else.
		if patch.Author != nil && !strings.EqualFold(*patch.Author, loginStr) {
			c.JSON(http.StatusForbidden, gin.H{"error": "you cannot change the author to another user"})
			return
		}
	}

	updated, err := h.service.UpdatePlugin(c.Request.Context(), c.Param("id"), patch)
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": updated})
}

func (h *PluginHandler) DeletePlugin(c *gin.Context) {
	// Non-admin users can only delete their own plugins.
	if isAdmin, _ := c.Get("isAdmin"); isAdmin != true {
		login, _ := c.Get("login")
		loginStr, _ := login.(string)
		existing, err := h.service.GetPlugin(c.Request.Context(), c.Param("id"))
		if err != nil {
			HandleError(c, err)
			return
		}
		if !strings.EqualFold(existing.Author, loginStr) {
			c.JSON(http.StatusForbidden, gin.H{"error": "you can only delete your own plugins", "author": existing.Author})
			return
		}
	}

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

// SubmitPlugin handles community plugin submissions.
// POST /api/v1/plugins/submit — requires auth; creates plugin with status=pending.
func (h *PluginHandler) SubmitPlugin(c *gin.Context) {
	var plugin models.Plugin
	if err := c.ShouldBindJSON(&plugin); err != nil {
		BadRequest(c, "Invalid request body", gin.H{"issue": err.Error()})
		return
	}

	// Force author to submitter's GitHub login.
	login, _ := c.Get("login")
	if loginStr, ok := login.(string); ok && loginStr != "" {
		plugin.Author = loginStr
	}

	created, err := h.service.SubmitPlugin(c.Request.Context(), plugin)
	if err != nil {
		HandleError(c, err)
		return
	}

	// Run validation checks asynchronously — results stored in DB when done.
	go func(id int64, repoURL string) {
		owner, repo := ownerRepoFromURL(repoURL)
		if owner == "" || repo == "" {
			return
		}
		result := validatePluginStandards(owner, repo)
		raw, err := json.Marshal(result)
		if err != nil {
			return
		}
		_ = h.service.UpdateValidationChecks(context.Background(), id, raw)
	}(created.ID, created.Repository)

	c.Header("Location", fmt.Sprintf("/api/v1/plugins/%d", created.ID))
	c.JSON(http.StatusCreated, gin.H{"data": created})
}

// RevalidatePlugin re-runs validation checks on a plugin (admin only).
// POST /api/v1/admin/plugins/:id/revalidate
func (h *PluginHandler) RevalidatePlugin(c *gin.Context) {
	plugin, err := h.service.GetPlugin(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}

	owner, repo := ownerRepoFromURL(plugin.Repository)
	if owner == "" || repo == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "plugin has no valid GitHub repository URL"})
		return
	}

	result := validatePluginStandards(owner, repo)
	raw, err := json.Marshal(result)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal result"})
		return
	}
	if err := h.service.UpdateValidationChecks(c.Request.Context(), plugin.ID, raw); err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// ApprovePlugin approves a pending plugin submission (admin only).
// PUT /api/v1/admin/plugins/:id/approve
func (h *PluginHandler) ApprovePlugin(c *gin.Context) {
	updated, err := h.service.ApprovePlugin(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": updated})
}

// RejectPlugin rejects a pending plugin submission (admin only).
// PUT /api/v1/admin/plugins/:id/reject
func (h *PluginHandler) RejectPlugin(c *gin.Context) {
	updated, err := h.service.RejectPlugin(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": updated})
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
