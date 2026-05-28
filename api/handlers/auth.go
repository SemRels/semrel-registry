package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

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
type Claims struct {
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	IsAdmin   bool   `json:"is_admin"`
	jwt.RegisteredClaims
}

// AuthHandler handles GitHub OAuth2 and JWT issuance.
type AuthHandler struct {
	oauthConfig *oauth2.Config
	jwtSecret   []byte
	allowedOrgs []string // empty = allow any GitHub user as read; admin = org member
	adminUsers  []string // individual logins that always get admin
	frontendURL string
}

func NewAuthHandler() *AuthHandler {
	clientID     := os.Getenv("GITHUB_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	jwtSecret    := os.Getenv("JWT_SECRET")
	frontendURL  := os.Getenv("FRONTEND_URL")
	allowedOrgs  := splitEnv("ALLOWED_GITHUB_ORGS")
	adminUsers   := splitEnv("ADMIN_GITHUB_USERS")

	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	if jwtSecret == "" {
		jwtSecret = "dev-jwt-secret-change-in-production"
	}

	cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"read:user", "user:email", "read:org"},
		Endpoint:     oauthgithub.Endpoint,
	}

	return &AuthHandler{
		oauthConfig: cfg,
		jwtSecret:   []byte(jwtSecret),
		allowedOrgs: allowedOrgs,
		adminUsers:  adminUsers,
		frontendURL: frontendURL,
	}
}

// GET /auth/github — redirect to GitHub OAuth consent page.
func (h *AuthHandler) Redirect(c *gin.Context) {
	state := randomState()
	c.SetCookie("oauth_state", state, 300, "/", "", false, true)
	url := h.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOnline)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// GET /auth/github/callback — exchange code for token, issue JWT, redirect to frontend.
func (h *AuthHandler) Callback(c *gin.Context) {
	// Verify state to prevent CSRF.
	cookieState, err := c.Cookie("oauth_state")
	if err != nil || cookieState != c.Query("state") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid OAuth state"})
		return
	}
	c.SetCookie("oauth_state", "", -1, "/", "", false, true)

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

	user, err := h.fetchGitHubUser(token.AccessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch GitHub user"})
		return
	}

	isAdmin := h.isAdmin(user.Login, token.AccessToken)

	jwtToken, err := h.issueJWT(user, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue JWT"})
		return
	}

	// Redirect to admin frontend with token in query param.
	// The frontend will store it in localStorage and strip it from the URL.
	c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s?token=%s", h.frontendURL, jwtToken))
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

// ── helpers ──────────────────────────────────────────────────────────────────

func (h *AuthHandler) fetchGitHubUser(accessToken string) (*GitHubUser, error) {
	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
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

func (h *AuthHandler) isAdmin(login, accessToken string) bool {
	// Explicit admin user list.
	for _, u := range h.adminUsers {
		if strings.EqualFold(u, login) {
			return true
		}
	}

	// Check org membership.
	for _, org := range h.allowedOrgs {
		url := fmt.Sprintf("https://api.github.com/orgs/%s/members/%s", org, login)
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Accept", "application/vnd.github+json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusNoContent {
			return true
		}
	}
	return false
}

func (h *AuthHandler) issueJWT(user *GitHubUser, isAdmin bool) (string, error) {
	claims := Claims{
		Login:     user.Login,
		Name:      user.Name,
		AvatarURL: user.AvatarURL,
		IsAdmin:   isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.Login,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(h.jwtSecret)
}

// ValidateJWT parses and validates a JWT, returning claims on success.
func (h *AuthHandler) ValidateJWT(tokenStr string) (*Claims, error) {
	t, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return h.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

func randomState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
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
