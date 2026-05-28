package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/SemRels/semrel-registry/api/handlers"
	"github.com/gin-gonic/gin"
)

// RequireAdmin checks for a valid admin credential:
//   1. JWT issued via GitHub OAuth (IsAdmin == true)
//   2. Legacy static ADMIN_TOKEN in Authorization header (dev fallback)
func RequireAdmin(authHandler *handlers.AuthHandler) gin.HandlerFunc {
	adminToken := os.Getenv("ADMIN_TOKEN")

	return func(c *gin.Context) {
		bearer := extractBearer(c)
		if bearer == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		// Try JWT first.
		claims, err := authHandler.ValidateJWT(bearer)
		if err == nil {
			if !claims.IsAdmin {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "GitHub account is not an admin (not an org member)",
					"login": claims.Login,
				})
				return
			}
			c.Set("claims", claims)
			c.Set("login", claims.Login)
			c.Next()
			return
		}

		// Fallback: static ADMIN_TOKEN for local dev.
		if adminToken != "" && bearer == adminToken {
			c.Set("login", "admin-token")
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
	}
}

// OptionalAuth attaches claims if a valid JWT is present, but doesn't block.
func OptionalAuth(authHandler *handlers.AuthHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		bearer := extractBearer(c)
		if bearer == "" {
			c.Next()
			return
		}
		claims, err := authHandler.ValidateJWT(bearer)
		if err == nil {
			c.Set("claims", claims)
			c.Set("login", claims.Login)
		}
		c.Next()
	}
}

func extractBearer(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if after, ok := strings.CutPrefix(h, "Bearer "); ok {
		return strings.TrimSpace(after)
	}
	// Also accept ?token= query param (for OAuth redirect).
	return c.Query("token")
}
