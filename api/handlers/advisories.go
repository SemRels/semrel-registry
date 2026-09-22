package handlers

// Security advisories.
//
// A checksum and a build attestation both describe an artifact as it was
// published; neither says anything about vulnerabilities discovered in it
// afterwards. GitHub already collects those — as Security Advisories (GHSA)
// published against the plugin's own repository — so the registry imports
// them: a consumer sees a plugin's known vulnerabilities without leaving the
// registry, and `semrel plugin audit` has something to check installed
// versions against.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/repository"
	"github.com/SemRels/semrel-registry/api/service"
)

type ghSecurityAdvisory struct {
	GHSAID          string `json:"ghsa_id"`
	CVEID           string `json:"cve_id"`
	Summary         string `json:"summary"`
	Severity        string `json:"severity"`
	HTMLURL         string `json:"html_url"`
	PublishedAt     string `json:"published_at"`
	WithdrawnAt     string `json:"withdrawn_at"`
	Vulnerabilities []struct {
		VulnerableVersionRange string `json:"vulnerable_version_range"`
		PatchedVersions        string `json:"patched_versions"`
	} `json:"vulnerabilities"`
}

// FetchSecurityAdvisories imports the published security advisories for a
// plugin's own GitHub repository. A repository with none, or whose visibility
// hides the endpoint, returns an empty slice rather than an error — that is
// the overwhelmingly common case, not a failure.
func FetchSecurityAdvisories(owner, repo string) ([]models.SecurityAdvisory, error) {
	if owner == "" || repo == "" {
		return []models.SecurityAdvisory{}, nil
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/security-advisories?per_page=100&state=published", owner, repo)
	body, status, err := ghRequest(url)
	if err != nil {
		return nil, fmt.Errorf("could not reach github: %w", err)
	}
	if status == http.StatusNotFound || status == http.StatusForbidden {
		return []models.SecurityAdvisory{}, nil
	}
	if status != http.StatusOK {
		return nil, githubAPIError(owner, repo, status, body)
	}

	var raw []ghSecurityAdvisory
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("advisories response could not be read: %w", err)
	}

	advisories := make([]models.SecurityAdvisory, 0, len(raw))
	for _, a := range raw {
		advisories = append(advisories, a.toModel())
	}
	return advisories, nil
}

func (a ghSecurityAdvisory) toModel() models.SecurityAdvisory {
	out := models.SecurityAdvisory{
		GHSAID:   a.GHSAID,
		CVEID:    a.CVEID,
		Summary:  a.Summary,
		Severity: a.Severity,
		URL:      a.HTMLURL,
	}
	// An advisory can list several affected packages; a semrel plugin publishes
	// one binary per repository, so the first entry is the one that matters.
	if len(a.Vulnerabilities) > 0 {
		out.VulnerableRange = convertGHSARange(a.Vulnerabilities[0].VulnerableVersionRange)
		out.PatchedVersion = a.Vulnerabilities[0].PatchedVersions
	}
	if t, err := time.Parse(time.RFC3339, a.PublishedAt); err == nil {
		out.PublishedAt = &t
	}
	if t, err := time.Parse(time.RFC3339, a.WithdrawnAt); err == nil {
		out.WithdrawnAt = &t
	}
	return out
}

// convertGHSARange rewrites GitHub's comma-separated range syntax
// (">= 1.0.0, < 2.0.3") into the space-separated, unspaced form this
// registry's own range parser accepts (">=1.0.0 <2.0.3"). See
// service.ParseRange.
func convertGHSARange(raw string) string {
	parts := strings.Split(raw, ",")
	terms := make([]string, 0, len(parts))
	for _, p := range parts {
		term := strings.Join(strings.Fields(p), "")
		if term != "" {
			terms = append(terms, term)
		}
	}
	return strings.Join(terms, " ")
}

// AdvisoryAffects reports whether an advisory's vulnerable range covers a
// version. A withdrawn advisory never affects anything. A range this
// registry cannot parse, or that was never recorded, is treated as affecting
// every version — the conservative direction for a security check.
func AdvisoryAffects(advisory models.SecurityAdvisory, version string) bool {
	if advisory.Withdrawn() {
		return false
	}
	if advisory.VulnerableRange == "" {
		return true
	}
	affected, err := service.SatisfiesRange(version, advisory.VulnerableRange)
	if err != nil {
		return true
	}
	return affected
}

// triggerAdvisoryRefresh re-imports a plugin's security advisories in the
// background, the same fire-and-forget shape as the validation-checks and
// provenance triggers: it never blocks or fails the caller. webhooks may be
// nil, in which case delivery is skipped.
func triggerAdvisoryRefresh(svc service.PluginManager, webhooks repository.WebhookRepository, pluginID int64, repositoryURL string) {
	owner, repo := ownerRepoFromURL(repositoryURL)
	if owner == "" || repo == "" {
		return
	}
	go func() {
		advisories, err := FetchSecurityAdvisories(owner, repo)
		if err != nil {
			return
		}
		ctx := context.Background()
		ref := fmt.Sprintf("%d", pluginID)
		before, _ := svc.GetPlugin(ctx, ref)
		if err := svc.SetSecurityAdvisories(ctx, pluginID, advisories); err != nil {
			return
		}
		notifyNewAdvisories(webhooks, before.Ref(), before.SecurityAdvisories, advisories)
	}()
}

// notifyNewAdvisories delivers advisory.published only for advisories that
// were not present before, so a periodic re-import does not re-notify every
// subscriber about the same advisory each time it runs.
func notifyNewAdvisories(webhooks repository.WebhookRepository, ref string, before, after []models.SecurityAdvisory) {
	seen := make(map[string]bool, len(before))
	for _, a := range before {
		seen[a.GHSAID] = true
	}
	for _, a := range after {
		if !seen[a.GHSAID] {
			DeliverWebhookEvent(webhooks, ref, models.WebhookEventAdvisoryPublished, a)
		}
	}
}

// RefreshSecurityAdvisories re-imports advisories for one plugin, synchronously
// — someone calling this is waiting for the answer.
// POST /api/v1/plugins/:id/advisories/refresh
func (h *PluginHandler) RefreshSecurityAdvisories(c *gin.Context) {
	plugin, err := h.service.GetPlugin(c.Request.Context(), c.Param("id"))
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
				"error":  "you can only refresh advisories for your own plugins",
				"author": plugin.Author,
			})
			return
		}
	}

	owner, repo := ownerRepoFromURL(plugin.Repository)
	if owner == "" || repo == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "plugin has no valid GitHub repository URL"})
		return
	}

	advisories, err := FetchSecurityAdvisories(owner, repo)
	if err != nil {
		if isGitHubRateLimitError(err) {
			writeError(c, http.StatusTooManyRequests, "GITHUB_RATE_LIMIT", "GitHub API rate limit exceeded. Configure GITHUB_TOKEN for higher limits.", err)
			return
		}
		InternalServerError(c, "failed to fetch security advisories", err)
		return
	}

	if err := h.service.SetSecurityAdvisories(c.Request.Context(), plugin.ID, advisories); err != nil {
		HandleError(c, err)
		return
	}
	notifyNewAdvisories(h.webhooks, plugin.Ref(), plugin.SecurityAdvisories, advisories)

	c.JSON(http.StatusOK, gin.H{"data": advisories})
}

// AuditPlugins checks a set of installed plugin@version pairs against known
// security advisories — the endpoint `semrel plugin audit` calls. Unlisted or
// unknown plugins are silently skipped: this reports what it knows, not what
// it could not find.
// POST /api/v1/audit
// Body: {"plugins":[{"ref":"analyzer-conventional","version":"1.2.0"}, ...]}
func (h *PluginHandler) AuditPlugins(c *gin.Context) {
	var body struct {
		Plugins []struct {
			Ref     string `json:"ref"`
			Version string `json:"version"`
		} `json:"plugins"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		BadRequest(c, "Invalid request body", gin.H{"issue": err.Error()})
		return
	}
	if len(body.Plugins) > 200 {
		BadRequest(c, "Too many plugins in one audit request", gin.H{"limit": 200})
		return
	}

	type finding struct {
		Ref        string                    `json:"ref"`
		Version    string                    `json:"version"`
		Advisories []models.SecurityAdvisory `json:"advisories"`
	}
	findings := make([]finding, 0)

	for _, entry := range body.Plugins {
		plugin, err := h.service.GetPlugin(c.Request.Context(), entry.Ref)
		if err != nil {
			continue
		}
		var affecting []models.SecurityAdvisory
		for _, advisory := range plugin.SecurityAdvisories {
			if AdvisoryAffects(advisory, entry.Version) {
				affecting = append(affecting, advisory)
			}
		}
		if len(affecting) > 0 {
			findings = append(findings, finding{Ref: entry.Ref, Version: entry.Version, Advisories: affecting})
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": findings})
}
