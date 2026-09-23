package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// readmeCacheTTL bounds how stale a README may be. READMEs change rarely and
	// this endpoint spends the registry's shared GitHub API budget, so caching
	// is what keeps a popular plugin page from exhausting it.
	readmeCacheTTL = 30 * time.Minute
	// maxReadmeBytes caps what is relayed to the browser. GitHub serves READMEs
	// of arbitrary size, and the page renders this inline.
	maxReadmeBytes = 512 << 10
)

type cachedReadme struct {
	markdown  string
	source    string
	fetchedAt time.Time
	err       error
}

var (
	readmeCacheMu sync.RWMutex
	readmeCache   = map[string]cachedReadme{}
)

// GET /api/v1/plugins/:id/readme
//
// Relays the plugin repository's README so the detail page can show what the
// plugin actually does. Until now the only description available was the
// one-line summary from the catalogue.
//
// The registry proxies this rather than having the browser fetch GitHub
// directly: the browser has no API token and would hit the unauthenticated
// 60-requests-per-hour limit, and a direct fetch would disclose every visitor's
// address to GitHub.
func (h *PluginHandler) PluginReadme(c *gin.Context) {
	plugin, err := h.service.GetPlugin(c.Request.Context(), c.Param("id"))
	if err != nil {
		HandleError(c, err)
		return
	}

	owner, repo := ownerRepoFromURL(plugin.Repository)
	if owner == "" || repo == "" || !isGitHubRepository(plugin.Repository) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"code":    "README_UNAVAILABLE",
				"message": "This plugin has no GitHub repository on record.",
			},
		})
		return
	}

	markdown, source, fetchErr := fetchReadme(c.Request.Context(), owner, repo)
	if fetchErr != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"code":    "README_UNAVAILABLE",
				"message": "The repository has no README the registry can read.",
			},
		})
		return
	}

	// The body is untrusted author content; the client sanitises it before
	// rendering. Served as data, never as a document.
	c.Header("Cache-Control", "public, max-age=1800")
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"markdown": markdown,
		"source":   source,
	}})
}

// isGitHubRepository guards the owner/repo split, which is pure string
// manipulation and would otherwise happily turn any URL into a GitHub API call.
func isGitHubRepository(repository string) bool {
	lowered := strings.ToLower(strings.TrimSpace(repository))
	return strings.HasPrefix(lowered, "https://github.com/") ||
		strings.HasPrefix(lowered, "http://github.com/")
}

func fetchReadme(ctx context.Context, owner, repo string) (markdown, source string, err error) {
	key := owner + "/" + repo

	readmeCacheMu.RLock()
	entry, ok := readmeCache[key]
	readmeCacheMu.RUnlock()
	if ok && time.Since(entry.fetchedAt) < readmeCacheTTL {
		return entry.markdown, entry.source, entry.err
	}

	markdown, source, err = fetchReadmeFromGitHub(ctx, owner, repo)

	readmeCacheMu.Lock()
	// Failures are cached too, for a shorter effective life: without that, a
	// repository with no README turns every page view into a GitHub request.
	readmeCache[key] = cachedReadme{markdown: markdown, source: source, fetchedAt: time.Now(), err: err}
	if len(readmeCache) > 2000 {
		readmeCache = map[string]cachedReadme{key: readmeCache[key]}
	}
	readmeCacheMu.Unlock()

	return markdown, source, err
}

func fetchReadmeFromGitHub(ctx context.Context, owner, repo string) (string, string, error) {
	body, status, err := ghRequest(ctx, fmt.Sprintf("https://api.github.com/repos/%s/%s/readme", owner, repo))
	if err != nil {
		return "", "", err
	}
	if status != http.StatusOK {
		return "", "", fmt.Errorf("github returned HTTP %d", status)
	}

	var payload struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
		HTMLURL  string `json:"html_url"`
		Name     string `json:"name"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", err
	}
	if payload.Encoding != "base64" {
		return "", "", fmt.Errorf("unexpected encoding %q", payload.Encoding)
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(payload.Content, "\n", ""))
	if err != nil {
		return "", "", err
	}
	if len(decoded) > maxReadmeBytes {
		decoded = decoded[:maxReadmeBytes]
	}

	return string(decoded), payload.HTMLURL, nil
}
