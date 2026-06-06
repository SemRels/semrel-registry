package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
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
	// Digest is provided by GitHub API as "sha256:hexhash" for each asset.
	Digest string `json:"digest"`
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
			if isGitHubRateLimitError(syncErr) {
				writeError(c, http.StatusTooManyRequests, "GITHUB_RATE_LIMIT", "GitHub API rate limit exceeded. Configure GITHUB_TOKEN for higher limits.", syncErr)
				return
			}
			r.Error = syncErr.Error()
			results = append(results, r)
			continue
		}
		results = append(results, r)
	}

	c.JSON(http.StatusOK, gin.H{"results": results})
}

// GET /plugins.json
// Returns all active plugins with their versions in the semrel registry metadata format.
// semrel CLI fetches this via SEMREL_REGISTRY_URL, e.g.:
//
//	SEMREL_REGISTRY_URL=http://localhost:8080 semrel plugin install analyzer-conventional
func (h *SyncHandler) PluginsJSON(c *gin.Context) {
	ctx := c.Request.Context()

	params := service.ListPluginsParams{
		Statuses: []string{"active"},
		Limit:    100,
	}
	result, err := h.svc.ListPlugins(ctx, params)
	if err != nil {
		InternalServerError(c, "failed to list plugins", err)
		return
	}

	type semrelPluginVersion struct {
		Version      string            `json:"version"`
		ReleaseDate  string            `json:"releaseDate"`
		Changelog    string            `json:"changelog,omitempty"`
		DownloadURL  string            `json:"downloadUrl"`
		DownloadURLs map[string]string `json:"downloadUrls,omitempty"`
		Checksums    map[string]string `json:"checksums"`
		Prerelease   bool              `json:"prerelease,omitempty"`
	}
	type semrelPlugin struct {
		Namespace   string                `json:"namespace,omitempty"`
		Name        string                `json:"name"`
		Description string                `json:"description"`
		Author      string                `json:"author"`
		License     string                `json:"license"`
		Category    string                `json:"category"`
		Repository  string                `json:"repository,omitempty"`
		Tags        []string              `json:"tags,omitempty"`
		Versions    []semrelPluginVersion `json:"versions"`
	}
	type semrelRegistry struct {
		Plugins []semrelPlugin `json:"plugins"`
	}

	registry := semrelRegistry{Plugins: make([]semrelPlugin, 0, len(result.Data))}
	for _, p := range result.Data {
		// Use the canonical ref so namespaced plugins resolve correctly.
		versions, listErr := h.svc.ListVersions(ctx, p.Ref(), 100, 0)
		if listErr != nil {
			continue
		}
		svs := make([]semrelPluginVersion, 0, len(versions))
		for _, v := range versions {
			rd := ""
			if v.ReleaseDate != nil {
				rd = v.ReleaseDate.Format(time.RFC3339)
			}
			svs = append(svs, semrelPluginVersion{
				Version:      v.Version,
				ReleaseDate:  rd,
				Changelog:    v.Changelog,
				DownloadURL:  v.DownloadURL,
				DownloadURLs: deriveDownloadURLs(v.DownloadURL, v.Checksums),
				Checksums:    v.Checksums,
				Prerelease:   v.Prerelease,
			})
		}
		tags := p.Tags
		if tags == nil {
			tags = []string{}
		}
		registry.Plugins = append(registry.Plugins, semrelPlugin{
			Namespace:   p.Namespace,
			Name:        p.Name,
			Description: p.Description,
			Author:      p.Author,
			License:     p.License,
			Category:    p.Category,
			Repository:  p.Repository,
			Tags:        tags,
			Versions:    svs,
		})
	}

	c.JSON(http.StatusOK, registry)
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
	// Build the canonical plugin ref. If GITHUB_ORG_NAMESPACE is configured,
	// map repo names like "provider-bitbucket" to namespaced plugin refs like
	// "@semrel/bitbucket" to match the seeded naming convention.
	orgNS := namespaceForOrg(payload.Owner)
	pluginName := pluginNameFromRepo(payload.Repository)
	pluginRef := pluginName
	if orgNS != "" {
		pluginRef = orgNS + "/" + pluginName
	}
	plugin, err := h.svc.GetPlugin(ctx, pluginRef)
	if err != nil && orgNS != "" {
		// Fallback: try both bare forms for community plugins without a namespace.
		plugin, err = h.svc.GetPlugin(ctx, pluginName)
		if err != nil {
			plugin, err = h.svc.GetPlugin(ctx, payload.Repository)
		}
	}
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

// POST /api/v1/admin/sync-github-org
// Discovers all SemRels org repos matching the plugin naming convention and
// upserts them into the registry with status=active. Also syncs versions.
func (h *SyncHandler) SyncGitHubOrg(c *gin.Context) {
	org := os.Getenv("ALLOWED_GITHUB_ORGS")
	if org == "" {
		org = "SemRels"
	}
	// Use only the first org if multiple are set.
	if idx := strings.Index(org, ","); idx != -1 {
		org = strings.TrimSpace(org[:idx])
	}

	repos, err := fetchOrgRepos(org)
	if err != nil {
		if isGitHubRateLimitError(err) {
			writeError(c, http.StatusTooManyRequests, "GITHUB_RATE_LIMIT", "GitHub API rate limit exceeded. Configure GITHUB_TOKEN for higher limits.", err)
			return
		}
		InternalServerError(c, "failed to fetch org repos: "+err.Error(), nil)
		return
	}

	// Valid plugin name pattern: <category>-<name>
	// Categories: analyzer, condition, generator, hook, provider, updater
	validCategory := regexp.MustCompile(`^(analyzer|condition|generator|hook|provider|updater)-(.+)$`)

	ctx := c.Request.Context()

	// The GITHUB_ORG_NAMESPACE env var maps a GitHub org to a plugin namespace,
	// e.g. org="SemRels" → namespace="@semrel". Plugins from this org are stored
	// and looked up as "@semrel/analyzer-default", etc.
	orgNS := namespaceForOrg(org)

	type syncResult struct {
		Repo     string `json:"repo"`
		Action   string `json:"action"` // "created", "updated", "skipped", "error"
		Error    string `json:"error,omitempty"`
		Versions int    `json:"versions,omitempty"`
	}
	var results []syncResult

	for _, repo := range repos {
		if repo.Private || repo.Archived || repo.Fork {
			continue
		}
		m := validCategory.FindStringSubmatch(repo.Name)
		if m == nil {
			continue
		}
		category := m[1]
		pluginName := m[2]
		repoURL := fmt.Sprintf("https://github.com/%s/%s", org, repo.Name)

		// Build the canonical lookup ref: "@semrel/default" or bare "default".
		// The seed normalizes SemRels repo names by stripping category prefixes.
		pluginRef := pluginName
		if orgNS != "" {
			pluginRef = orgNS + "/" + pluginName
		}

		existing, getErr := h.svc.GetPlugin(ctx, pluginRef)
		if getErr != nil && orgNS != "" {
			// Namespaced lookup failed. Check whether old bare-name entries exist
			// and migrate one to the configured namespace instead of duplicating.
			bare, bareErr := h.svc.GetPlugin(ctx, pluginName)
			if bareErr != nil {
				bare, bareErr = h.svc.GetPlugin(ctx, repo.Name)
			}
			if bareErr == nil && bare.Namespace == "" && strings.Contains(bare.Repository, "/"+org+"/") {
				migrated, patchErr := h.svc.UpdatePlugin(ctx, bare.Ref(), models.PluginPatch{Namespace: &orgNS})
				if patchErr != nil {
					results = append(results, syncResult{Repo: repo.Name, Action: "error", Error: "namespace migration: " + patchErr.Error()})
					continue
				}
				createdV, _, _ := h.syncPluginReleases(ctx, &migrated)
				results = append(results, syncResult{Repo: repo.Name, Action: "migrated", Versions: createdV})
				continue
			}
		}
		if getErr != nil {
			// Plugin does not exist yet — create it with the correct namespace.
			created, createErr := h.svc.CreatePlugin(ctx, models.Plugin{
				Namespace:   orgNS,
				Name:        pluginName,
				Description: repo.Description,
				Author:      org,
				Category:    category,
				Repository:  repoURL,
				License:     "Apache-2.0",
				Status:      models.StatusActive,
				Tags:        []string{category},
			})
			if createErr != nil {
				results = append(results, syncResult{Repo: repo.Name, Action: "error", Error: createErr.Error()})
				continue
			}
			createdV, _, _ := h.syncPluginReleases(ctx, &created)
			results = append(results, syncResult{Repo: repo.Name, Action: "created", Versions: createdV})
		} else {
			// Plugin exists — sync versions only.
			createdV, _, syncErr := h.syncPluginReleases(ctx, &existing)
			if syncErr != nil {
				results = append(results, syncResult{Repo: repo.Name, Action: "error", Error: syncErr.Error()})
				if isGitHubRateLimitError(syncErr) {
					break
				}
				continue
			}
			results = append(results, syncResult{Repo: repo.Name, Action: "updated", Versions: createdV})
		}
	}

	c.JSON(http.StatusOK, gin.H{"org": org, "results": results, "total": len(results)})
}

// pluginNameFromRepo maps plugin repository names to canonical plugin names.
// Example: "provider-bitbucket" -> "bitbucket".
func pluginNameFromRepo(repoName string) string {
	parts := strings.SplitN(strings.TrimSpace(repoName), "-", 2)
	if len(parts) != 2 {
		return strings.TrimSpace(repoName)
	}
	category := parts[0]
	if category == "analyzer" || category == "condition" || category == "generator" || category == "hook" || category == "provider" || category == "updater" {
		return parts[1]
	}
	return strings.TrimSpace(repoName)
}

// namespaceForOrg resolves the namespace used for plugins synced from a GitHub org.
// If GITHUB_ORG_NAMESPACE is not configured, SemRels defaults to @semrel.
func namespaceForOrg(org string) string {
	ns := strings.TrimSpace(os.Getenv("GITHUB_ORG_NAMESPACE"))
	if ns != "" {
		return ns
	}
	if strings.EqualFold(strings.TrimSpace(org), "SemRels") {
		return "@semrel"
	}
	return ""
}

type ghRepo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Private     bool   `json:"private"`
	Archived    bool   `json:"archived"`
	Fork        bool   `json:"fork"`
}

func fetchOrgRepos(org string) ([]ghRepo, error) {
	token := os.Getenv("GITHUB_TOKEN")
	var all []ghRepo
	for page := 1; ; page++ {
		url := fmt.Sprintf("https://api.github.com/orgs/%s/repos?type=public&per_page=100&page=%d", org, page)
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("Accept", "application/vnd.github+json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, githubAPIError("org repos", org, resp.StatusCode, body)
		}

		var repos []ghRepo
		if err := json.Unmarshal(body, &repos); err != nil {
			return nil, fmt.Errorf("parse repos: %w", err)
		}
		all = append(all, repos...)
		if len(repos) < 100 {
			break
		}
	}
	return all, nil
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
	ref := p.Ref()

	// Check if already exists.
	existing, err := h.svc.ListVersions(ctx, ref, 100, 0)
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
		Checksums:   pickChecksums(rel.Assets),
		Prerelease:  rel.Prerelease,
	}
	_, createErr := h.svc.CreateVersion(ctx, ref, ver)
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

	// Detect language by presence of go.mod or Cargo.toml.
	hasGoMod, _    := checkGHFile(owner, repo, "go.mod")
	hasCargo, _    := checkGHFile(owner, repo, "Cargo.toml")

	var lang string
	switch {
	case hasGoMod:
		lang = "go"
	case hasCargo:
		lang = "rust"
	default:
		lang = "unknown"
	}

	// Language-specific manifest + entry-point checks.
	var manifestCheck, entrypointCheck check
	switch lang {
	case "rust":
		manifestCheck = check{
			"lang_manifest", "Cargo.toml present (Rust)",
			func() (bool, string) { return checkGHFile(owner, repo, "Cargo.toml") },
		}
		entrypointCheck = check{
			"lang_entrypoint", "src/main.rs or src/lib.rs present",
			func() (bool, string) {
				if ok, _ := checkGHFile(owner, repo, "src/main.rs"); ok {
					return true, ""
				}
				if ok, _ := checkGHFile(owner, repo, "src/lib.rs"); ok {
					return true, ""
				}
				return false, "neither src/main.rs nor src/lib.rs found"
			},
		}
	default: // "go" or unknown — default to Go expectations
		manifestCheck = check{
			"lang_manifest", "go.mod present (Go)",
			func() (bool, string) { return checkGHFile(owner, repo, "go.mod") },
		}
		entrypointCheck = check{
			"lang_entrypoint", "cmd/plugin/ entry point present",
			func() (bool, string) { return checkGHFile(owner, repo, "cmd/plugin") },
		}
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
		manifestCheck,
		entrypointCheck,
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

	langLabel := map[string]string{"go": "Go", "rust": "Rust", "unknown": "unknown language"}[lang]
	result.Valid = allPassed
	if allPassed {
		result.Summary = fmt.Sprintf("%s/%s passes all SemRels standards ✓ (%s)", owner, repo, langLabel)
	} else {
		failed := 0
		for _, ch := range result.Checks {
			if !ch.Passed {
				failed++
			}
		}
		result.Summary = fmt.Sprintf("%d check(s) failed (%s) — plugin does not meet SemRels standards", failed, langLabel)
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
		return nil, githubAPIError(owner, repo, status, body)
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
		return nil, githubAPIError(owner, repo+"@"+tag, status, body)
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
		return nil, githubAPIError(owner, repo+"/latest", status, body)
	}
	var rel ghRelease
	return &rel, json.Unmarshal(body, &rel)
}

func githubAPIError(owner, repo string, status int, body []byte) error {
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && strings.TrimSpace(payload.Message) != "" {
		return fmt.Errorf("GitHub API %d for %s/%s: %s", status, owner, repo, payload.Message)
	}
	return fmt.Errorf("GitHub API %d for %s/%s", status, owner, repo)
}

func isGitHubRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "rate limit") || (strings.Contains(msg, "github api 403") && !errors.Is(err, context.Canceled))
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

// pickChecksums builds a platform→sha256hash map from release binary assets.
// GitHub provides a "digest" field ("sha256:hexhash") for each asset, which
// we use directly — no need to download the individual .sha256 files.
// Keys use the semrel convention: "linux_amd64", "darwin_arm64", etc.
func pickChecksums(assets []ghAsset) map[string]string {
	checksums := make(map[string]string)
	for _, a := range assets {
		// Skip checksum text files and non-binary assets.
		if strings.HasSuffix(a.Name, ".sha256") || strings.HasSuffix(a.Name, ".txt") {
			continue
		}
		if a.Digest == "" {
			continue
		}
		// Parse platform from name: "plugin-linux-amd64" or "plugin-windows-arm64.exe"
		base := strings.TrimPrefix(a.Name, "plugin-")
		base = strings.TrimSuffix(base, ".exe")
		// Convert "linux-amd64" → "linux_amd64" (semrel convention)
		parts := strings.SplitN(base, "-", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0] + "_" + parts[1]
		// Strip "sha256:" prefix from digest
		hash := strings.TrimPrefix(a.Digest, "sha256:")
		checksums[key] = hash
	}
	return checksums
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

// deriveDownloadURLs is defined in download_urls.go (shared with the versions handler).
