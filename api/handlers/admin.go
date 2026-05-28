package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	appErrors "github.com/SemRels/semrel-registry/api/internal"
	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/service"
	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	service service.PluginManager
}

func NewAdminHandler(pluginService service.PluginManager) *AdminHandler {
	return &AdminHandler{service: pluginService}
}

func (h *AdminHandler) Status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"version":   "1",
		"timestamp": time.Now().UTC(),
	})
}

// SyncPlugins reads a plugins.json payload from the request body and upserts all plugins.
func (h *AdminHandler) SyncPlugins(c *gin.Context) {
	var payload struct {
		Plugins []syncPlugin `json:"plugins"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON with a plugins array", err)
		return
	}

	created, updated, failed := 0, 0, 0
	ctx := c.Request.Context()

	for _, sp := range payload.Plugins {
		existing, err := h.service.GetPlugin(ctx, sp.Name)
		if err != nil && !errors.Is(err, appErrors.ErrPluginNotFound) {
			failed++
			continue
		}

		if existing.ID == 0 {
			_, err = h.service.CreatePlugin(ctx, models.Plugin{
				Name:        sp.Name,
				Description: sp.Description,
				Author:      sp.Author,
				Category:    sp.Category,
				Repository:  sp.Repository,
				License:     sp.License,
				Tags:        sp.Tags,
			})
			if err != nil {
				failed++
				continue
			}
			created++
		} else {
			desc, auth, cat, repo, lic := sp.Description, sp.Author, sp.Category, sp.Repository, sp.License
			_, err = h.service.UpdatePlugin(ctx, sp.Name, models.PluginPatch{
				Description: &desc,
				Author:      &auth,
				Category:    &cat,
				Repository:  &repo,
				License:     &lic,
				Tags:        &sp.Tags,
			})
			if err != nil {
				failed++
				continue
			}
			updated++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"created": created,
		"updated": updated,
		"failed":  failed,
		"total":   len(payload.Plugins),
	})
}

// SyncFromFile reads the plugins.json file on disk and syncs it into the database.
// The file path defaults to ./plugins.json relative to the working directory.
func (h *AdminHandler) SyncFromFile(c *gin.Context) {
	filePath := os.Getenv("PLUGINS_JSON_PATH")
	if filePath == "" {
		filePath = "plugins.json"
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "FILE_ERROR", fmt.Sprintf("cannot read %s", filePath), err)
		return
	}

	var payload struct {
		Plugins []syncPlugin `json:"plugins"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		writeError(c, http.StatusInternalServerError, "PARSE_ERROR", "cannot parse plugins.json", err)
		return
	}

	ctx := c.Request.Context()
	_ = ctx

	created, updated, failed := 0, 0, 0
	for _, sp := range payload.Plugins {
		existing, err := h.service.GetPlugin(c.Request.Context(), sp.Name)
		if err != nil && !errors.Is(err, appErrors.ErrPluginNotFound) {
			failed++
			continue
		}

		if existing.ID == 0 {
			_, err = h.service.CreatePlugin(c.Request.Context(), models.Plugin{
				Name:        sp.Name,
				Description: sp.Description,
				Author:      sp.Author,
				Category:    sp.Category,
				Repository:  sp.Repository,
				License:     sp.License,
				Tags:        sp.Tags,
			})
			if err != nil {
				failed++
				continue
			}
			created++
		} else {
			desc, auth, cat, repo, lic := sp.Description, sp.Author, sp.Category, sp.Repository, sp.License
			_, err = h.service.UpdatePlugin(c.Request.Context(), sp.Name, models.PluginPatch{
				Description: &desc,
				Author:      &auth,
				Category:    &cat,
				Repository:  &repo,
				License:     &lic,
				Tags:        &sp.Tags,
			})
			if err != nil {
				failed++
				continue
			}
			updated++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"created": created,
		"updated": updated,
		"failed":  failed,
		"total":   len(payload.Plugins),
		"source":  filePath,
	})
}

// GetStats returns aggregate statistics about the registry.
func (h *AdminHandler) GetStats(c *gin.Context) {
	result, err := h.service.ListPlugins(c.Request.Context(), service.ListPluginsParams{
		Page:  1,
		Limit: 1,
	})
	if err != nil {
		HandleError(c, err)
		return
	}

	categories := map[string]int64{}
	allResult, err := h.service.ListPlugins(c.Request.Context(), service.ListPluginsParams{
		Page:  1,
		Limit: 100,
	})
	if err == nil {
		for _, p := range allResult.Data {
			categories[p.Category]++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"totalPlugins": result.Pagination.Total,
		"categories":   categories,
		"timestamp":    time.Now().UTC(),
	})
}

type syncPlugin struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	Category    string   `json:"category"`
	Repository  string   `json:"repository"`
	License     string   `json:"license"`
	Tags        []string `json:"tags"`
}

