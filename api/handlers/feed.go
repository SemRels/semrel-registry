package handlers

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/service"
	"github.com/gin-gonic/gin"
)

// feedEntryLimit bounds how many releases a feed carries. Readers poll these
// frequently, so the response has to stay small regardless of catalogue size.
const feedEntryLimit = 50

// FeedHandler serves an Atom feed of recent plugin releases.
//
// Until now the only way to learn that a plugin had a new version was to poll
// the API and diff it yourself. A feed is what lets people — and bots — follow
// the registry without scraping it.
type FeedHandler struct {
	service service.PluginManager
}

func NewFeedHandler(svc service.PluginManager) *FeedHandler {
	return &FeedHandler{service: svc}
}

type atomFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Title   string      `xml:"title"`
	ID      string      `xml:"id"`
	Updated string      `xml:"updated"`
	Links   []atomLink  `xml:"link"`
	Entries []atomEntry `xml:"entry"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr,omitempty"`
	Type string `xml:"type,attr,omitempty"`
}

type atomEntry struct {
	Title   string     `xml:"title"`
	ID      string     `xml:"id"`
	Updated string     `xml:"updated"`
	Link    atomLink   `xml:"link"`
	Author  atomAuthor `xml:"author"`
	Summary atomText   `xml:"summary"`
}

type atomAuthor struct {
	Name string `xml:"name"`
}

type atomText struct {
	Type string `xml:"type,attr"`
	Body string `xml:",chardata"`
}

// GET /feed.atom — the most recent plugin releases across the registry.
func (h *FeedHandler) Releases(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	plugins, err := h.service.ListPlugins(ctx, service.ListPluginsParams{
		Page:     1,
		Limit:    100,
		Statuses: []string{models.StatusActive},
	})
	if err != nil {
		InternalServerError(c, "failed to build feed", err)
		return
	}

	type release struct {
		plugin  models.Plugin
		version models.PluginVersion
		at      time.Time
	}

	releases := make([]release, 0, feedEntryLimit)
	for _, plugin := range plugins.Data {
		versions, versionErr := h.service.ListVersions(ctx, plugin.Ref(), 10, 0)
		if versionErr != nil {
			continue
		}
		for _, version := range versions {
			at := version.CreatedAt
			if version.ReleaseDate != nil {
				at = *version.ReleaseDate
			}
			releases = append(releases, release{plugin: plugin, version: version, at: at})
		}
	}

	sort.Slice(releases, func(i, j int) bool { return releases[i].at.After(releases[j].at) })
	if len(releases) > feedEntryLimit {
		releases = releases[:feedEntryLimit]
	}

	updated := time.Now().UTC()
	if len(releases) > 0 {
		updated = releases[0].at.UTC()
	}

	feed := atomFeed{
		Title:   "semrel plugin releases",
		ID:      sitemapBase + "/feed.atom",
		Updated: updated.Format(time.RFC3339),
		Links: []atomLink{
			{Href: sitemapBase + "/feed.atom", Rel: "self", Type: "application/atom+xml"},
			{Href: sitemapBase + "/", Rel: "alternate", Type: "text/html"},
		},
		Entries: make([]atomEntry, 0, len(releases)),
	}

	for _, item := range releases {
		ref := item.plugin.Ref()
		url := fmt.Sprintf("%s/plugins/%s", sitemapBase, ref)
		summary := item.version.Changelog
		if summary == "" {
			summary = fmt.Sprintf("%s %s was published.", ref, item.version.Version)
		}

		feed.Entries = append(feed.Entries, atomEntry{
			Title:   fmt.Sprintf("%s %s", ref, item.version.Version),
			ID:      fmt.Sprintf("%s#%s", url, item.version.Version),
			Updated: item.at.UTC().Format(time.RFC3339),
			Link:    atomLink{Href: url, Rel: "alternate", Type: "text/html"},
			Author:  atomAuthor{Name: firstNonBlank(item.plugin.Author, "semrel")},
			// type="text" rather than "html": changelogs are author-supplied, and
			// declaring them as text means readers escape rather than render them.
			Summary: atomText{Type: "text", Body: summary},
		})
	}

	body, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		InternalServerError(c, "failed to encode feed", err)
		return
	}

	c.Header("Cache-Control", "public, max-age=300")
	c.Data(http.StatusOK, "application/atom+xml; charset=utf-8", append([]byte(xml.Header), body...))
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
