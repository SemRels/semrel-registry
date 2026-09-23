package handlers

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSOptions configures cross-origin access.
type CORSOptions struct {
	// AllowedOrigins are the browser origins permitted to read authenticated
	// responses. Empty means "no cross-origin credentialed access".
	AllowedOrigins []string
	// PublicPrefixes are path prefixes that stay world-readable (Access-Control-
	// Allow-Origin: *) because they serve the public catalogue the CLI and third
	// parties consume anonymously.
	PublicPrefixes []string
	MaxAge         int
}

// DefaultPublicPrefixes are the anonymous, cacheable read paths.
var DefaultPublicPrefixes = []string{
	"/plugins.json",
	"/schemas/",
	"/sitemap.xml",
	"/health",
	"/api/v1/plugins",
}

// CORS answers preflight requests and sets the response headers.
//
// The public catalogue keeps "*" so any site or tool can read it without
// credentials. Everything else — auth, admin, writes — is limited to the
// configured origins and echoes a single concrete origin, which is what allows
// the browser to send credentials at all.
func CORS(opts CORSOptions) gin.HandlerFunc {
	allowed := make([]string, 0, len(opts.AllowedOrigins))
	for _, origin := range opts.AllowedOrigins {
		if normalized := normalizeOrigin(origin); normalized != "" {
			allowed = append(allowed, normalized)
		}
	}
	publicPrefixes := opts.PublicPrefixes
	if len(publicPrefixes) == 0 {
		publicPrefixes = DefaultPublicPrefixes
	}
	maxAge := opts.MaxAge
	if maxAge <= 0 {
		maxAge = 600
	}

	return func(c *gin.Context) {
		origin := normalizeOrigin(c.GetHeader("Origin"))
		header := c.Writer.Header()
		header.Add("Vary", "Origin")

		switch {
		case origin != "" && originIn(origin, allowed):
			header.Set("Access-Control-Allow-Origin", origin)
			header.Set("Access-Control-Allow-Credentials", "true")
		case isPublicPath(c.Request.URL.Path, publicPrefixes) && isSafeMethod(c.Request.Method):
			// Anonymous, read-only access to the public catalogue.
			header.Set("Access-Control-Allow-Origin", "*")
		}

		if c.Request.Method == http.MethodOptions {
			header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			header.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Hub-Signature-256, X-Webhook-Secret")
			header.Set("Access-Control-Max-Age", strconv.Itoa(maxAge))
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// SecurityHeaders sets the response headers that limit what a browser will do
// with an API response — chiefly, refusing to sniff a JSON body into something
// executable and refusing to frame it.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.Writer.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "no-referrer")
		header.Set("Cross-Origin-Opener-Policy", "same-origin")
		// API responses are data, never a document: forbid every capability.
		header.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		c.Next()
	}
}

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

func isPublicPath(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func originIn(origin string, allowed []string) bool {
	for _, candidate := range allowed {
		if strings.EqualFold(candidate, origin) {
			return true
		}
	}
	return false
}

func normalizeOrigin(origin string) string {
	origin = strings.TrimSpace(origin)
	if origin == "" || origin == "null" || origin == "*" {
		return ""
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}
