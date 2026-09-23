package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// VerifyOrigin rejects state-changing requests whose Origin header names a site
// that is not allowed to drive this API.
//
// Session cookies are SameSite=Lax, which already keeps them off cross-site
// form posts, but Lax is a browser-side promise: it does nothing for older
// browsers, and nothing for request shapes browsers treat as top-level
// navigations. Checking Origin server-side turns that promise into a rule.
//
// Requests without an Origin header are allowed through: non-browser clients
// (the semrel CLI, curl, CI jobs) do not send one, and they authenticate with a
// bearer token rather than an ambient cookie, so they are not subject to CSRF.
func VerifyOrigin(allowedOrigins []string) gin.HandlerFunc {
	allowed := normalizeOrigins(allowedOrigins)

	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}

		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin == "" || origin == "null" {
			// Not a browser-issued cross-origin request.
			c.Next()
			return
		}

		if originAllowed(origin, allowed) || sameOrigin(origin, c.Request) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":    "ORIGIN_NOT_ALLOWED",
				"message": "Request origin is not allowed to perform this action",
			},
		})
	}
}

// sameOrigin reports whether the Origin header matches the host the request was
// addressed to, which is the common case for the admin SPA served next to the
// API behind one reverse proxy.
func sameOrigin(origin string, req *http.Request) bool {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Host, req.Host)
}

func originAllowed(origin string, allowed []string) bool {
	for _, candidate := range allowed {
		if strings.EqualFold(candidate, origin) {
			return true
		}
	}
	return false
}

// normalizeOrigins reduces each configured entry to its scheme://host[:port]
// form so that a trailing slash or path in configuration does not silently stop
// an origin from matching.
func normalizeOrigins(origins []string) []string {
	out := make([]string, 0, len(origins))
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin == "" || origin == "*" {
			continue
		}
		if parsed, err := url.Parse(origin); err == nil && parsed.Scheme != "" && parsed.Host != "" {
			out = append(out, parsed.Scheme+"://"+parsed.Host)
			continue
		}
		out = append(out, strings.TrimSuffix(origin, "/"))
	}
	return out
}
