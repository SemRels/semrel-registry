package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/service"
	"github.com/gin-gonic/gin"
)

// ── GitHub API types ──────────────────────────────────────────────────────────

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Body        string    `json:"body"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
	PublishedAt string    `json:"published_at"`
	Assets      []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// ── SyncHandler ───────────────────────────────────────────────────────────────

type SyncHandler struct {
	svc service.PluginManager
}

func NewSyncHandler(s service.PluginManager) *SyncHandler {
	return &SyncHandler{svc: s}
}

// POST /api/v1/admin/sync-versions
// Body (optional): {"plugin": "analyzer-conventional"}
func (h *SyncHandler) SyncVersions(c *gin.Context) {
	var body struct {
		Plugin string `json:"plugin"`
	}
	_ = c.ShouldBindJSON(&body)

	ctx := c.Request.Context()

	// Collect all plugins, paginating because service enforces limit ≤ 100.
	var allPlugins []models.Plugin
	page := 1
	for {
		result, err := h.svc.ListPlugins(ctx, service.ListPluginsParams{Page: page, Limit: 100})
		if err != nil {
			InternalServerError(c, "failed to list plugins", err)
			return
		}
		allPlugins = append(allPlugins, result.Data...)
		if page >= result.Pagination.Pages || len(result.Data) == 0 {
			break
		}
		page++
	}

	type result struct {
		Plugin  string `json:"plugin"`
		Created int    `json:"created"`
		Skipped int    `json:"skipped"`
		Error   string `json:"error,omitempty"`
	}
	var results []result

	for i := range allPlugins {
		p := &allPlugins[i]
		if body.Plugin != "" && p.Name != body.Plugin {
			continue
		}
		created, skipped, syncErr := h.syncPluginReleases(ctx, p)
		r := result{Plugin: p.Name, Created: created, Skipped: skipped}
		if syncErr != nil {
			r.Error = syncErr.Error()
		}
		results = append(results, r)
	}

	c.JSON(http.StatusOK, gin.H{"results": results})
}

// POST /api/v1/webhooks/release
// Payload: {"owner":"SemRels","repository":"analyzer-conventional","tag":"v1.2.3"}
// Also handles GitHub repository_dispatch client_payload wrapper.
func (h *SyncHandler) WebhookRelease(c *gin.Context) {
	secret := os.Getenv("WEBHOOK_SECRET")
	if secret != "" && c.GetHeader("X-Webhook-Secret") != secret {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid webhook secret"})
		return
	}

	var payload struct {
		Owner         string `json:"owner"`
		Repository    string `json:"repository"`
		Tag           string `json:"tag"`
		ClientPayload *struct {
			SourceOwner string `json:"source_owner"`
			SourceRepo  string `json:"source_repo"`
			Tag         string `json:"tag"`
		} `json:"client_payload"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if payload.ClientPayload != nil {
		payload.Owner = payload.ClientPayload.SourceOwner
		payload.Repository = payload.ClientPayload.SourceRepo
		payload.Tag = payload.ClientPayload.Tag
	}
	if payload.Repository == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repository is required"})
		return
	}
	if payload.Owner == "" {
		payload.Owner = "SemRels"
	}

	ctx := c.Request.Context()
	plugin, err := h.svc.GetPlugin(ctx, payload.Repository)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("plugin %q not registered", payload.Repository)})
		return
	}

	owner, repo := ownerRepoFromURL(plugin.Repository)
	if repo == "" {
		owner, repo = payload.Owner, payload.Repository
	}

	var rel *ghRelease
	if payload.Tag != "" {
		rel, err = fetchGHRelease(owner, repo, payload.Tag)
	} else {
		rel, err = fetchGHLatestRelease(owner, repo)
	}
	if err != nil {
		InternalServerError(c, "failed to fetch release", err)
		return
	}

	created, upsertErr := h.upsertVersion(ctx, &plugin, rel)
	if upsertErr != nil {
		InternalServerError(c, "failed to upsert version", upsertErr)
		return
	}
	status := "skipped"
	if created {
		status = "created"
	}
	c.JSON(http.StatusOK, gin.H{"plugin": plugin.Name, "version": rel.TagName, "status": status})
}

func (h *SyncHandler) syncPluginReleases(ctx context.Context, p *models.Plugin) (created, skipped int, err error) {
	owner, repo := ownerRepoFromURL(p.Repository)
	if repo == "" {
		return 0, 0, fmt.Errorf("cannot parse repository URL %q", p.Repository)
	}
	releases, err := fetchGHReleases(owner, repo)
	if err != nil {
		return 0, 0, err
	}
	for i := range releases {
		rel := &releases[i]
		if rel.Draft {
			skipped++
			continue
		}
		ok, upsertErr := h.upsertVersion(ctx, p, rel)
		if upsertErr != nil {
			return created, skipped, upsertErr
		}
		if ok {
			created++
		} else {
			skipped++
		}
	}
	return created, skipped, nil
}

func (h *SyncHandler) upsertVersion(ctx context.Context, p *models.Plugin, rel *ghRelease) (bool, error) {
	tag := strings.TrimPrefix(rel.TagName, "v")

	// Check if already exists.
	existing, err := h.svc.ListVersions(ctx, p.Name, 200, 0)
	if err != nil {
		return false, err
	}
	for _, v := range existing {
		if v.Version == tag {
			return false, nil
		}
	}

	var releaseDate *time.Time
	if rel.PublishedAt != "" {
		t, parseErr := time.Parse(time.RFC3339, rel.PublishedAt)
		if parseErr == nil {
			releaseDate = &t
		}
	}

	ver := models.PluginVersion{
		PluginID:    p.ID,
		Version:     tag,
		ReleaseDate: releaseDate,
		Changelog:   rel.Body,
		DownloadURL: pickDownloadURL(rel.Assets),
		Prerelease:  rel.Prerelease,
	}
	_, createErr := h.svc.CreateVersion(ctx, p.Name, ver)
	return createErr == nil, createErr
}

// ── Plugin Standards Validator ────────────────────────────────────────────────

var pluginNameRE = regexp.MustCompile(`^(provider|analyzer|condition|hook|updater|generator)-[a-z0-9][a-z0-9-]*$`)

// POST /api/v1/plugins/validate
// Body: {"repository":"https://github.com/SemRels/analyzer-conventional"}
// or   {"owner":"SemRels","repo":"analyzer-conventional"}
func ValidatePlugin(c *gin.Context) {
	var body struct {
		Repository string `json:"repository"`
		Owner      string `json:"owner"`
		Repo       string `json:"repo"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	owner, repo := body.Owner, body.Repo
	if body.Repository != "" {
		owner, repo = ownerRepoFromURL(body.Repository)
	}
	if owner == "" || repo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provide repository URL or owner+repo"})
		return
	}

	result := validatePluginStandards(owner, repo)
	status := http.StatusOK
	if !result.Valid {
		status = http.StatusUnprocessableEntity
	}
	c.JSON(status, result)
}

type ValidationResult struct {
	Valid   bool              `json:"valid"`
	Plugin  string            `json:"plugin"`
	Owner   string            `json:"owner"`
	Checks  []ValidationCheck `json:"checks"`
	Summary string            `json:"summary"`
}

type ValidationCheck struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

func validatePluginStandards(owner, repo string) ValidationResult {
	result := ValidationResult{Plugin: repo, Owner: owner}

	type check struct {
		id    string
		label string
		fn    func() (bool, string)
	}

	checks := []check{
		{
			"naming",
			"Naming convention: {category}-{name}",
			func() (bool, string) {
				if pluginNameRE.MatchString(repo) {
					return true, ""
				}
				return false, fmt.Sprintf("%q must match {category}-{name}, e.g. updater-pypi", repo)
			},
		},
		{"security_md", "SECURITY.md present", func() (bool, string) { return checkGHFile(owner, repo, "SECURITY.md") }},
		{"contributing_md", "CONTRIBUTING.md present", func() (bool, string) { return checkGHFile(owner, repo, "CONTRIBUTING.md") }},
		{"governance_md", "GOVERNANCE.md present", func() (bool, string) { return checkGHFile(owner, repo, "GOVERNANCE.md") }},
		{"license", "LICENSE file present", func() (bool, string) { return checkGHFile(owner, repo, "LICENSE") }},
		{"release_workflow", ".github/workflows/release.yml present", func() (bool, string) {
			return checkGHFile(owner, repo, ".github/workflows/release.yml")
		}},
		{"security_workflow", ".github/workflows/security.yml present", func() (bool, string) {
			return checkGHFile(owner, repo, ".github/workflows/security.yml")
		}},
		{"go_mod", "go.mod present", func() (bool, string) { return checkGHFile(owner, repo, "go.mod") }},
		{"cmd_plugin", "cmd/plugin/ entry point present", func() (bool, string) {
			return checkGHFile(owner, repo, "cmd/plugin")
		}},
		{
			"registry_sync",
			"release.yml triggers registry sync",
			func() (bool, string) {
				content, err := fetchGHFileContent(owner, repo, ".github/workflows/release.yml")
				if err != nil {
					return false, "could not fetch release.yml"
				}
				if strings.Contains(content, "REGISTRY_SYNC") || strings.Contains(content, "semrel-registry") {
					return true, ""
				}
				return false, "release.yml has no registry sync step — add 'Trigger registry sync' from plugin-template"
			},
		},
	}

	allPassed := true
	for _, ch := range checks {
		passed, msg := ch.fn()
		if !passed {
			allPassed = false
		}
		result.Checks = append(result.Checks, ValidationCheck{ID: ch.id, Label: ch.label, Passed: passed, Message: msg})
	}

	result.Valid = allPassed
	if allPassed {
		result.Summary = fmt.Sprintf("%s/%s passes all SemRels standards ✓", owner, repo)
	} else {
		failed := 0
		for _, ch := range result.Checks {
			if !ch.Passed {
				failed++
			}
		}
		result.Summary = fmt.Sprintf("%d check(s) failed — plugin does not meet SemRels standards", failed)
	}
	return result
}

// ── GitHub API helpers ────────────────────────────────────────────────────────

func ghRequest(url string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}

func fetchGHReleases(owner, repo string) ([]ghRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=100", owner, repo)
	body, status, err := ghRequest(url)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, nil
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("GitHub API %d for %s/%s", status, owner, repo)
	}
	var releases []ghRelease
	return releases, json.Unmarshal(body, &releases)
}

func fetchGHRelease(owner, repo, tag string) (*ghRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", owner, repo, tag)
	body, status, err := ghRequest(url)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("GitHub API %d for %s/%s@%s", status, owner, repo, tag)
	}
	var rel ghRelease
	return &rel, json.Unmarshal(body, &rel)
}

func fetchGHLatestRelease(owner, repo string) (*ghRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	body, status, err := ghRequest(url)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("GitHub API %d for %s/%s/latest", status, owner, repo)
	}
	var rel ghRelease
	return &rel, json.Unmarshal(body, &rel)
}

func checkGHFile(owner, repo, path string) (bool, string) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", owner, repo, path)
	_, status, err := ghRequest(url)
	if err != nil {
		return false, err.Error()
	}
	if status == http.StatusOK {
		return true, ""
	}
	return false, fmt.Sprintf("%s not found (HTTP %d)", path, status)
}

func fetchGHFileContent(owner, repo, path string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", owner, repo, path)
	body, status, err := ghRequest(url)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", status)
	}
	var content struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(body, &content); err != nil {
		return "", err
	}
	if content.Encoding == "base64" {
		clean := strings.ReplaceAll(content.Content, "\n", "")
		decoded, decErr := base64.StdEncoding.DecodeString(clean)
		if decErr != nil {
			return "", decErr
		}
		return string(decoded), nil
	}
	return content.Content, nil
}

func ownerRepoFromURL(repoURL string) (string, string) {
	repoURL = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(repoURL), "/"), ".git")
	parts := strings.Split(repoURL, "/")
	if len(parts) < 2 {
		return "", ""
	}
	return parts[len(parts)-2], parts[len(parts)-1]
}

func pickDownloadURL(assets []ghAsset) string {
	for _, a := range assets {
		if strings.HasSuffix(a.Name, ".sha256") || strings.HasSuffix(a.Name, ".txt") {
			continue
		}
		if strings.Contains(a.Name, "linux") && strings.Contains(a.Name, "amd64") {
			return a.BrowserDownloadURL
		}
	}
	for _, a := range assets {
		if !strings.HasSuffix(a.Name, ".sha256") && !strings.HasSuffix(a.Name, ".txt") {
			return a.BrowserDownloadURL
		}
	}
	return ""
}
