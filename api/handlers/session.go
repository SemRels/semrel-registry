package handlers

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ── Session cookie ────────────────────────────────────────────────────────────

// setSessionCookie stores the session JWT in an HttpOnly cookie.
//
// The token deliberately never travels in a URL: query strings are recorded in
// browser history, proxy logs and Referer headers, so a token placed there
// outlives the session it belongs to. SameSite=Lax keeps the cookie off
// cross-site POST requests, which — together with the Origin check in
// middleware.VerifyOrigin — is what protects the mutating endpoints from CSRF.
func (h *AuthHandler) setSessionCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(h.cookieName, token, int(h.sessionTTL.Seconds()), "/", h.cookieDomain, h.cookieSecure, true)
}

func (h *AuthHandler) clearSessionCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(h.cookieName, "", -1, "/", h.cookieDomain, h.cookieSecure, true)
}

// TokenFromRequest returns the caller's bearer token, preferring the
// Authorization header (CLI, CI, scripts) and falling back to the session
// cookie (browser). Query parameters are intentionally not consulted.
func (h *AuthHandler) TokenFromRequest(c *gin.Context) string {
	if header := strings.TrimSpace(c.GetHeader("Authorization")); header != "" {
		scheme, token, found := strings.Cut(header, " ")
		if found && strings.EqualFold(scheme, "bearer") {
			if token = strings.TrimSpace(token); token != "" {
				return token
			}
		}
	}
	if cookie, err := c.Cookie(h.cookieName); err == nil {
		return strings.TrimSpace(cookie)
	}
	return ""
}

// ── Revocation ────────────────────────────────────────────────────────────────

// revocationList tracks the JWT IDs of sessions that were explicitly ended.
//
// JWTs are stateless, so signing one out would otherwise leave it valid until
// it expires — up to SESSION_TTL after the user pressed "Sign out". Entries are
// dropped once the underlying token would have expired anyway, which bounds the
// memory this costs to the number of sign-outs within one token lifetime.
//
// This store is per-process: with more than one API replica, a revoked token
// stays usable on the replicas that did not observe the sign-out. Moving the
// list to Postgres or Redis is the follow-up for a multi-replica deployment.
type revocationList struct {
	mu      sync.RWMutex
	revoked map[string]time.Time // jti → expiry of the revoked token
	last    time.Time
}

func newRevocationList() *revocationList {
	return &revocationList{revoked: make(map[string]time.Time), last: time.Now()}
}

func (r *revocationList) Revoke(jti string, expiresAt time.Time) {
	if jti == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.revoked[jti] = expiresAt
	r.sweepLocked()
}

func (r *revocationList) IsRevoked(jti string) bool {
	if jti == "" {
		return false
	}
	r.mu.RLock()
	expiry, ok := r.revoked[jti]
	r.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(expiry) {
		r.mu.Lock()
		delete(r.revoked, jti)
		r.mu.Unlock()
		return false
	}
	return true
}

// sweepLocked drops expired entries at most once a minute. Callers hold r.mu.
func (r *revocationList) sweepLocked() {
	if time.Since(r.last) < time.Minute {
		return
	}
	now := time.Now()
	for jti, expiry := range r.revoked {
		if now.After(expiry) {
			delete(r.revoked, jti)
		}
	}
	r.last = now
}
