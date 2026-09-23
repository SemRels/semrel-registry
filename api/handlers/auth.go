package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/SemRels/semrel-registry/api/config"
	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/service"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
	oauthgithub "golang.org/x/oauth2/github"
)

// GitHubUser holds the fields we care about from the GitHub /user API.
type GitHubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Email     string `json:"email"`
}

// Claims extends standard JWT claims with GitHub identity.
// Role is "admin" (org owner/maintainer) or "user" (authenticated community member).
type Claims struct {
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Role      string `json:"role"`     // "admin" | "user"
	IsAdmin   bool   `json:"is_admin"` // true when Role == "admin" (kept for backwards compat)
	// AuthTime is the Unix timestamp of the last interactive GitHub sign-in.
	// Destructive operations require it to be recent, so an attacker holding a
	// stolen session cannot delete an account without passing GitHub first.
	AuthTime int64 `json:"auth_time,omitempty"`
	jwt.RegisteredClaims
}

// reauthWindow is how long an interactive GitHub sign-in counts as "fresh"
// for destructive operations such as account deletion.
const reauthWindow = 5 * time.Minute

// FreshlyAuthenticated reports whether the interactive sign-in behind these
// claims happened within the re-authentication window.
func (c *Claims) FreshlyAuthenticated() bool {
	if c == nil || c.AuthTime == 0 {
		return false
	}
	return time.Since(time.Unix(c.AuthTime, 0)) <= reauthWindow
}

// AuthOptions carries the authentication settings resolved from configuration.
// Zero values fall back to the corresponding environment variable, so tests and
// local tooling can keep constructing an AuthHandler without wiring a Config.
type AuthOptions struct {
	ClientID     string
	ClientSecret string
	JWTSecret    string
	AdminToken   string
	AllowedOrgs  []string
	AdminUsers   []string
	FrontendURL  string
	SessionTTL   time.Duration
	CookieName   string
	CookieDomain string
	CookieSecure bool
}

// AuthHandler handles GitHub OAuth2 and JWT issuance.
type AuthHandler struct {
	oauthConfig *oauth2.Config
	jwtSecret   []byte
	adminToken  string   // static break-glass credential; empty disables it
	allowedOrgs []string // empty = allow any GitHub user as read; admin = org member
	adminUsers  []string // individual logins that always get admin
	frontendURL string

	sessionTTL   time.Duration
	cookieName   string
	cookieDomain string
	cookieSecure bool
	revoked      *revocationList

	plugins service.PluginManager
}

// NewAuthHandler builds a handler from the process environment.
func NewAuthHandler(pluginManagers ...service.PluginManager) *AuthHandler {
	return NewAuthHandlerWithOptions(AuthOptions{}, pluginManagers...)
}

// NewAuthHandlerWithOptions builds a handler from explicit options, falling
// back to the environment for anything left unset.
func NewAuthHandlerWithOptions(opts AuthOptions, pluginManagers ...service.PluginManager) *AuthHandler {
	firstNonEmpty := func(value, envKey, fallback string) string {
		if v := strings.TrimSpace(value); v != "" {
			return v
		}
		if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
			return v
		}
		return fallback
	}

	clientID := firstNonEmpty(opts.ClientID, "GITHUB_CLIENT_ID", "")
	clientSecret := firstNonEmpty(opts.ClientSecret, "GITHUB_CLIENT_SECRET", "")
	jwtSecret := firstNonEmpty(opts.JWTSecret, "JWT_SECRET", config.DevJWTSecret)
	adminToken := firstNonEmpty(opts.AdminToken, "ADMIN_TOKEN", "")
	frontendURL := firstNonEmpty(opts.FrontendURL, "FRONTEND_URL", "http://localhost:5173")
	cookieName := firstNonEmpty(opts.CookieName, "SESSION_COOKIE_NAME", "semrel_session")
	cookieDomain := firstNonEmpty(opts.CookieDomain, "COOKIE_DOMAIN", "")

	allowedOrgs := opts.AllowedOrgs
	if len(allowedOrgs) == 0 {
		allowedOrgs = splitEnv("ALLOWED_GITHUB_ORGS")
	}
	adminUsers := opts.AdminUsers
	if len(adminUsers) == 0 {
		adminUsers = splitEnv("ADMIN_GITHUB_USERS")
	}
	sessionTTL := opts.SessionTTL
	if sessionTTL <= 0 {
		sessionTTL = 24 * time.Hour
	}

	cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"read:user", "user:email", "read:org"},
		Endpoint:     oauthgithub.Endpoint,
	}

	var pluginManager service.PluginManager
	if len(pluginManagers) > 0 {
		pluginManager = pluginManagers[0]
	}

	return &AuthHandler{
		oauthConfig:  cfg,
		jwtSecret:    []byte(jwtSecret),
		adminToken:   adminToken,
		allowedOrgs:  allowedOrgs,
		adminUsers:   adminUsers,
		frontendURL:  frontendURL,
		sessionTTL:   sessionTTL,
		cookieName:   cookieName,
		cookieDomain: cookieDomain,
		cookieSecure: opts.CookieSecure,
		revoked:      newRevocationList(),
		plugins:      pluginManager,
	}
}

// IsStaticAdminToken reports whether tok matches the configured break-glass
// ADMIN_TOKEN. The comparison is constant-time so that a caller cannot recover
// the token byte by byte from response timings, and an unset token never
// matches — not even an empty Authorization header.
func (h *AuthHandler) IsStaticAdminToken(tok string) bool {
	if h.adminToken == "" || tok == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(h.adminToken), []byte(tok)) == 1
}

// GET /auth/github — redirect to GitHub OAuth consent page.
// An optional ?next= carries a path inside the admin app to return to; it is
// signed into the OAuth state and re-validated on the way back, so it cannot
// be used to bounce a signed-in user to an attacker's site.
func (h *AuthHandler) Redirect(c *gin.Context) {
	state := h.signedState(safeReturnPath(c.Query("next")))
	url := h.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOnline)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// safeReturnPath accepts only same-site absolute paths. Anything else — an
// absolute URL, a scheme-relative "//evil.example" or a traversal attempt —
// collapses to the empty string, which means "use the configured frontend".
func safeReturnPath(next string) string {
	next = strings.TrimSpace(next)
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return ""
	}
	if strings.Contains(next, "\\") || strings.Contains(next, "..") {
		return ""
	}
	if len(next) > 512 {
		return ""
	}
	return next
}

// GET /auth/github/callback — exchange code for token, issue JWT, redirect to frontend.
// Also registered as GET /oauth/callback to match common GitHub App callback URL patterns.
func (h *AuthHandler) Callback(c *gin.Context) {
	next, stateOK := h.verifySignedState(c.Query("state"))
	if !stateOK {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid OAuth state"})
		return
	}

	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing code"})
		return
	}

	token, err := h.oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token exchange failed: " + err.Error()})
		return
	}

	user, err := h.fetchGitHubUser(c.Request.Context(), token.AccessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch GitHub user"})
		return
	}

	role := h.resolveRole(c.Request.Context(), user.Login, token.AccessToken)

	jwtToken, err := h.issueJWT(user, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue JWT"})
		return
	}

	// The session travels in an HttpOnly cookie, never in the redirect URL:
	// a token in a query string is copied into browser history, proxy logs and
	// outgoing Referer headers, where it stays readable long after sign-out.
	h.setSessionCookie(c, jwtToken)
	c.Redirect(http.StatusTemporaryRedirect, h.returnTarget(next))
}

// returnTarget resolves the post-login destination. next is already known to be
// a same-site path (or empty), so joining it onto the configured frontend
// origin cannot leave that origin.
func (h *AuthHandler) returnTarget(next string) string {
	if next == "" {
		return h.frontendURL
	}
	return strings.TrimSuffix(h.frontendURL, "/") + next
}

// POST /api/v1/auth/logout — revoke the current session and clear the cookie.
func (h *AuthHandler) Logout(c *gin.Context) {
	if claims, ok := c.Get("claims"); ok {
		if typed, isClaims := claims.(*Claims); isClaims && typed.ID != "" {
			expiry := time.Now().Add(h.sessionTTL)
			if typed.ExpiresAt != nil {
				expiry = typed.ExpiresAt.Time
			}
			h.revoked.Revoke(typed.ID, expiry)
		}
	}
	h.clearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "signed out"}})
}

// GET /auth/me — return the current user from JWT (or dev-token fallback).
func (h *AuthHandler) Me(c *gin.Context) {
	claims, ok := c.Get("claims")
	if ok {
		c.JSON(http.StatusOK, gin.H{"user": claims})
		return
	}
	// Dev ADMIN_TOKEN has no JWT claims; return a synthetic identity.
	login, _ := c.Get("login")
	c.JSON(http.StatusOK, gin.H{"user": gin.H{
		"login":   login,
		"name":    "Dev Admin",
		"isAdmin": true,
	}})
}

// GET /auth/config — return public OAuth config for the frontend.
func (h *AuthHandler) Config(c *gin.Context) {
	configured := h.oauthConfig.ClientID != ""
	c.JSON(http.StatusOK, gin.H{
		"githubOAuthEnabled": configured,
		"loginURL":           "/auth/github",
	})
}

func (h *AuthHandler) DeleteAccount(c *gin.Context) {
	if h.plugins == nil {
		ServiceUnavailable(c, "Account deletion is not configured", nil)
		return
	}

	var request models.AccountDeletionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		BadRequest(c, "Invalid request body", gin.H{"issue": err.Error()})
		return
	}

	login, _ := c.Get("login")
	loginStr, _ := login.(string)
	if err := h.requireRecentAuth(c, loginStr, request.ReauthToken); err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "REAUTH_REQUIRED",
				"message": "Reauthentication required",
				"details": gin.H{
					"issue":     err.Error(),
					"signInURL": "/auth/github?next=/admin/account",
				},
			},
		})
		return
	}

	result, err := h.plugins.DeleteAccount(c.Request.Context(), request, currentDeleteActor(c))
	if err != nil {
		HandleError(c, err)
		return
	}

	// The account is gone; the session that deleted it must not outlive it.
	if claims, ok := c.Get("claims"); ok {
		if typed, isClaims := claims.(*Claims); isClaims && typed.ExpiresAt != nil {
			h.revoked.Revoke(typed.ID, typed.ExpiresAt.Time)
		}
	}
	h.clearSessionCookie(c)

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// requireRecentAuth enforces step-up authentication before a destructive,
// irreversible account action.
//
// The browser path is a fresh GitHub sign-in: the session cookie is HttpOnly,
// so there is no token for the user to copy into a form even if we asked. API
// clients that authenticate with a bearer token instead may present a matching
// token explicitly, which proves possession the same way.
func (h *AuthHandler) requireRecentAuth(c *gin.Context, login, reauthToken string) error {
	if claimsValue, ok := c.Get("claims"); ok {
		if claims, isClaims := claimsValue.(*Claims); isClaims {
			if claims.FreshlyAuthenticated() {
				return nil
			}
			if reauthToken == "" {
				return fmt.Errorf("sign in with GitHub again to confirm this action")
			}
		}
	}

	reauthToken = strings.TrimSpace(reauthToken)
	if reauthToken == "" {
		return fmt.Errorf("sign in with GitHub again to confirm this action")
	}
	if h.IsStaticAdminToken(reauthToken) {
		return nil
	}
	claims, err := h.ValidateJWT(reauthToken)
	if err != nil {
		return fmt.Errorf("reauthentication token is not valid")
	}
	if !strings.EqualFold(claims.Login, login) {
		return fmt.Errorf("reauthenticated user does not match current account")
	}
	if !claims.FreshlyAuthenticated() {
		return fmt.Errorf("reauthentication token is older than %s", reauthWindow)
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func (h *AuthHandler) fetchGitHubUser(ctx context.Context, accessToken string) (*GitHubUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var user GitHubUser
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// resolveRole determines the user's role:
//
//	"admin" — org owner/maintainer or in ADMIN_GITHUB_USERS list
//	"user"  — any other authenticated GitHub user
func (h *AuthHandler) resolveRole(ctx context.Context, login, accessToken string) string {
	// Explicit admin list always wins.
	for _, u := range h.adminUsers {
		if strings.EqualFold(u, login) {
			return "admin"
		}
	}

	// Check if the user is an owner/maintainer in any allowed org.
	for _, org := range h.allowedOrgs {
		if h.isOrgOwnerOrMaintainer(ctx, org, login, accessToken) {
			return "admin"
		}
	}
	return "user"
}

// isOrgOwnerOrMaintainer checks whether the user has role "admin" (org owner)
// in the given GitHub org, using the user's own OAuth token.
func (h *AuthHandler) isOrgOwnerOrMaintainer(ctx context.Context, org, login, accessToken string) bool {
	// GET /orgs/{org}/memberships/{username} — the user can read their own membership.
	url := fmt.Sprintf("https://api.github.com/orgs/%s/memberships/%s", org, login)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// 404 = not a member, 403 = private org (insufficient scope) — not admin.
		return false
	}

	var membership struct {
		Role  string `json:"role"`  // "admin" (owner) | "member"
		State string `json:"state"` // "active" | "pending"
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &membership); err != nil {
		return false
	}
	// Only "admin" role in an org maps to our admin role.
	// Regular org members are not admins in the registry.
	return membership.Role == "admin" && membership.State == "active"
}

func (h *AuthHandler) issueJWT(user *GitHubUser, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		Login:     user.Login,
		Name:      user.Name,
		AvatarURL: user.AvatarURL,
		Role:      role,
		IsAdmin:   role == "admin",
		AuthTime:  now.Unix(),
		RegisteredClaims: jwt.RegisteredClaims{
			// A unique ID per session is what makes sign-out effective: without
			// it there is nothing to put on the revocation list.
			ID:        newTokenID(),
			Subject:   user.Login,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(h.sessionTTL)),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(h.jwtSecret)
}

func newTokenID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// A collision-prone ID is still better than an unrevocable session.
		return fmt.Sprintf("t%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// ValidateJWT parses and validates a JWT, returning claims on success.
// It rejects tokens without an expiry and tokens whose session was signed out.
func (h *AuthHandler) ValidateJWT(tokenStr string) (*Claims, error) {
	t, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return h.jwtSecret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	claims, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	if h.revoked.IsRevoked(claims.ID) {
		return nil, fmt.Errorf("session has been signed out")
	}
	return claims, nil
}

// signedState generates an HMAC-signed OAuth state parameter carrying a random
// nonce and the post-login return path.
// Format: base64url(<16-byte-random-hex>:<return-path>).<hmac-sha256-hex>
// This is stateless — no cookie or server-side session needed.
func (h *AuthHandler) signedState(next string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	payload := base64.RawURLEncoding.EncodeToString([]byte(hex.EncodeToString(b) + ":" + next))
	return payload + "." + h.stateSignature(payload)
}

func (h *AuthHandler) stateSignature(payload string) string {
	mac := hmac.New(sha256.New, h.jwtSecret)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// verifySignedState validates an HMAC-signed state parameter and returns the
// return path it carries.
func (h *AuthHandler) verifySignedState(state string) (next string, ok bool) {
	payload, sig, found := strings.Cut(state, ".")
	if !found || payload == "" {
		return "", false
	}
	if !hmac.Equal([]byte(h.stateSignature(payload)), []byte(sig)) {
		return "", false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", false
	}
	_, next, _ = strings.Cut(string(decoded), ":")
	return safeReturnPath(next), true
}

func splitEnv(key string) []string {
	val := os.Getenv(key)
	if val == "" {
		return nil
	}

	parts := strings.Split(val, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}
