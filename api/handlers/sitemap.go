package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/service"
	"github.com/gin-gonic/gin"
)

const sitemapBase = "https://registry.semrel.io"

// SitemapHandler serves /sitemap.xml with all active plugin detail pages.
type SitemapHandler struct {
	service service.PluginManager
}

func NewSitemapHandler(svc service.PluginManager) *SitemapHandler {
	return &SitemapHandler{service: svc}
}

func (h *SitemapHandler) Sitemap(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Fetch all active plugins (up to 2000 — plenty for a registry sitemap)
	result, err := h.service.ListPlugins(ctx, service.ListPluginsParams{
		Page:     1,
		Limit:    2000,
		Statuses: []string{models.StatusActive},
	})
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC().Format("2006-01-02")

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString("\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)
	b.WriteString("\n")

	// Static pages
	staticPages := []struct {
		loc        string
		priority   string
		changefreq string
	}{
		{sitemapBase + "/", "1.0", "daily"},
	}

	for _, p := range staticPages {
		b.WriteString(fmt.Sprintf(
			"  <url>\n    <loc>%s</loc>\n    <lastmod>%s</lastmod>\n    <changefreq>%s</changefreq>\n    <priority>%s</priority>\n  </url>\n",
			p.loc, now, p.changefreq, p.priority,
		))
	}

	// Plugin detail pages
	for _, plugin := range result.Data {
		pluginRef := plugin.Name
		if plugin.Namespace != "" {
			pluginRef = "@" + plugin.Namespace + "/" + plugin.Name
		}
		loc := sitemapBase + "/plugins/" + pluginRef
		lastmod := now
		if !plugin.UpdatedAt.IsZero() {
			lastmod = plugin.UpdatedAt.UTC().Format("2006-01-02")
		}
		b.WriteString(fmt.Sprintf(
			"  <url>\n    <loc>%s</loc>\n    <lastmod>%s</lastmod>\n    <changefreq>weekly</changefreq>\n    <priority>0.8</priority>\n  </url>\n",
			loc, lastmod,
		))
	}

	b.WriteString("</urlset>\n")

	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=3600")
	c.String(http.StatusOK, b.String())
}
