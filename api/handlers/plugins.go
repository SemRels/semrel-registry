package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/service"
	"github.com/gin-gonic/gin"
)

type PluginHandler struct {
	service service.PluginManager
	metrics service.MetricsRecorder
}

func NewPluginHandler(pluginService service.PluginManager, metrics ...service.MetricsRecorder) *PluginHandler {
	recorder := service.NewNoopMetricsRecorder()
	if len(metrics) > 0 && metrics[0] != nil {
		recorder = metrics[0]
	}
	return &PluginHandler{service: pluginService, metrics: recorder}
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
	login, _ := c.Get("login")
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
		Page:      page,
		Limit:     limit,
		Category:  strings.TrimSpace(c.Query("category")),
		Search:    strings.TrimSpace(c.Query("search")),
		Sort:      strings.TrimSpace(c.Query("sort")),
		SortDir:   strings.TrimSpace(c.Query("order")),
		Namespace: strings.TrimSpace(c.Query("namespace")),
		Author:    author,
		Statuses:  statuses,
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
	h.metrics.Record(service.MetricEvent{PluginID: plugin.ID, Type: service.MetricTypeView, Source: "plugin-detail"})

	c.JSON(http.StatusOK, gin.H{"data": plugin})
}

// GetPluginByNamespace handles GET /api/v1/plugins/@:namespace/:name
func (h *PluginHandler) GetPluginByNamespace(c *gin.Context) {
	ref := "@" + c.Param("namespace") + "/" + c.Param("name")
	plugin, err := h.service.GetPlugin(c.Request.Context(), ref)
	if err != nil {
		HandleError(c, err)
		return
	}
	h.metrics.Record(service.MetricEvent{PluginID: plugin.ID, Type: service.MetricTypeView, Source: "plugin-detail"})
	c.JSON(http.StatusOK, gin.H{"data": plugin})
}

// ListPluginVersionsByNamespace handles GET /api/v1/plugins/@:namespace/:name/versions
func (h *PluginHandler) ListPluginVersionsByNamespace(c *gin.Context) {
	ref := "@" + c.Param("namespace") + "/" + c.Param("name")
	limit, ok := parseQueryInt(c, "limit", 20)
	if !ok {
		return
	}
	offset, ok := parseQueryInt(c, "offset", 0)
	if !ok {
		return
	}

	versions, err := h.service.ListVersions(c.Request.Context(), ref, limit, offset)
	if err != nil {
		HandleError(c, err)
		return
	}

	type versionResponse struct {
		models.PluginVersion
		DownloadURLs map[string]string `json:"downloadUrls,omitempty"`
	}
	out := make([]versionResponse, len(versions))
	for i, v := range versions {
		out[i] = versionResponse{
			PluginVersion: v,
			DownloadURLs:  buildNamespacedDownloadURLs(c, c.Param("namespace"), c.Param("name"), v),
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
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
			DownloadURLs:  buildPluginDownloadURLs(c, c.Param("id"), v),
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *PluginHandler) DownloadPluginVersion(c *gin.Context) {
	h.downloadVersionByRef(c, c.Param("id"))
}

func (h *PluginHandler) DownloadPluginVersionByNamespace(c *gin.Context) {
	ref := "@" + c.Param("namespace") + "/" + c.Param("name")
	h.downloadVersionByRef(c, ref)
}

// TrackDownload handles POST /api/v1/plugins/:id/versions/:version/downloads
// and POST /api/v1/plugins/@:namespace/:name/versions/:version/downloads.
// It records a download metric without redirecting so direct GitHub downloads
// can still be tracked by the CLI.
func (h *PluginHandler) TrackDownload(c *gin.Context) {
	h.trackDownloadByRef(c, c.Param("id"))
}

func (h *PluginHandler) TrackDownloadByNamespace(c *gin.Context) {
	ref := "@" + c.Param("namespace") + "/" + c.Param("name")
	h.trackDownloadByRef(c, ref)
}

func (h *PluginHandler) downloadVersionByRef(c *gin.Context, ref string) {
	version, found, err := h.findVersion(c.Request.Context(), ref, c.Param("version"))
	if err != nil {
		HandleError(c, err)
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "version not found"})
		return
	}

	target := version.DownloadURL
	if platform := strings.TrimSpace(c.Query("platform")); platform != "" {
		if derived := deriveDownloadURLs(version.DownloadURL, version.Checksums); len(derived) > 0 {
			if perPlatform, ok := derived[platform]; ok {
				target = perPlatform
			}
		}
	}

	h.metrics.Record(service.MetricEvent{PluginID: version.PluginID, VersionID: version.ID, Type: service.MetricTypeDownload, Source: "download-redirect"})
	c.Redirect(http.StatusTemporaryRedirect, target)
}

func (h *PluginHandler) trackDownloadByRef(c *gin.Context, ref string) {
	version, found, err := h.findVersion(c.Request.Context(), ref, c.Param("version"))
	if err != nil {
		HandleError(c, err)
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "version not found"})
		return
	}

	h.metrics.Record(service.MetricEvent{
		PluginID:  version.PluginID,
		VersionID: version.ID,
		Type:      service.MetricTypeDownload,
		Source:    "cli-install",
	})
	c.Status(http.StatusNoContent)
}

func (h *PluginHandler) findVersion(ctx context.Context, ref, versionTag string) (models.PluginVersion, bool, error) {
	offset := 0
	for {
		batch, err := h.service.ListVersions(ctx, ref, 100, offset)
		if err != nil {
			return models.PluginVersion{}, false, err
		}
		for _, v := range batch {
			if v.Version == versionTag {
				return v, true, nil
			}
		}
		if len(batch) < 100 {
			break
		}
		offset += 100
	}
	return models.PluginVersion{}, false, nil
}

func buildPluginDownloadURLs(c *gin.Context, pluginID string, version models.PluginVersion) map[string]string {
	return buildTrackedDownloadURLs(c, deriveDownloadURLs(version.DownloadURL, version.Checksums),
		fmt.Sprintf("/api/v1/plugins/%s/versions/%s/download", url.PathEscape(pluginID), url.PathEscape(version.Version)))
}

func buildNamespacedDownloadURLs(c *gin.Context, namespace, name string, version models.PluginVersion) map[string]string {
	return buildTrackedDownloadURLs(c, deriveDownloadURLs(version.DownloadURL, version.Checksums),
		fmt.Sprintf("/api/v1/plugins/@%s/%s/versions/%s/download", url.PathEscape(namespace), url.PathEscape(name), url.PathEscape(version.Version)))
}

func buildTrackedDownloadURLs(_ *gin.Context, source map[string]string, endpoint string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	tracked := make(map[string]string, len(source))
	for platform := range source {
		tracked[platform] = endpoint + "?platform=" + url.QueryEscape(platform)
	}
	return tracked
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

func (h *PluginHandler) DeletePluginVersion(c *gin.Context) {
	versionID, err := strconv.ParseInt(c.Param("versionId"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid version ID", gin.H{"issue": "versionId must be an integer"})
		return
	}

	plugin, err := h.service.GetPlugin(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}

	// Non-admins can only delete versions of their own plugins.
	if isAdmin, _ := c.Get("isAdmin"); isAdmin != true {
		login, _ := c.Get("login")
		loginStr, _ := login.(string)
		if !strings.EqualFold(plugin.Author, loginStr) {
			c.JSON(http.StatusForbidden, gin.H{"error": "you can only delete versions of your own plugins"})
			return
		}
	}

	if err := h.service.DeleteVersion(c.Request.Context(), plugin.ID, versionID); err != nil {
		HandleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
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
