package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/SemRels/semrel-registry/api/models"
	"github.com/gin-gonic/gin"
)

// PUT /api/v1/plugins/:id/versions/:versionId/yank
// DELETE /api/v1/plugins/:id/versions/:versionId/yank
//
// Yanking is how a publisher retracts a broken or compromised release without
// breaking the builds that already pin it. Deletion, the only previous option,
// breaks them.
func (h *PluginHandler) YankVersion(c *gin.Context) {
	h.setVersionYank(c, true)
}

func (h *PluginHandler) UnyankVersion(c *gin.Context) {
	h.setVersionYank(c, false)
}

func (h *PluginHandler) setVersionYank(c *gin.Context, yanked bool) {
	ref := c.Param("id")
	versionID, err := strconv.ParseInt(c.Param("versionId"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid version id", gin.H{"issue": "must be an integer"})
		return
	}

	var request models.VersionYankRequest
	// An empty body is valid when lifting a yank, where no reason is needed.
	_ = c.ShouldBindJSON(&request)

	plugin, err := h.service.GetPlugin(c.Request.Context(), ref)
	if err != nil {
		HandleError(c, err)
		return
	}

	// Same rule as every other write: publishers manage their own plugins,
	// admins manage all of them.
	if isAdmin, _ := c.Get("isAdmin"); isAdmin != true {
		login, _ := c.Get("login")
		loginStr, _ := login.(string)
		if !strings.EqualFold(plugin.Author, loginStr) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":  "you can only yank versions of your own plugins",
				"author": plugin.Author,
			})
			return
		}
	}

	version, err := h.service.YankVersion(c.Request.Context(), ref, versionID, yanked, request, currentDeleteActor(c))
	if err != nil {
		HandleError(c, err)
		return
	}

	event := models.WebhookEventVersionYanked
	if !yanked {
		event = models.WebhookEventVersionUnyanked
	}
	DeliverWebhookEvent(h.webhooks, plugin.Ref(), event, version)

	c.JSON(http.StatusOK, gin.H{"data": version})
}
