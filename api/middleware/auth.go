package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/SemRels/semrel-registry/api/handlers"
	"github.com/gin-gonic/gin"
)

// RequireAdmin gates routes that only org owners/maintainers (role=admin) may access.
// Accepts: GitHub JWT (IsAdmin==true) | legacy ADMIN_TOKEN (dev fallback).
func RequireAdmin(authHandler *handlers.AuthHandler) gin.HandlerFunc {
	adminToken := os.Getenv("ADMIN_TOKEN")

	return func(c *gin.Context) {
		bearer := extractBearer(c)
		if bearer == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		claims, err := authHandler.ValidateJWT(bearer)
		if err == nil {
			if !claims.IsAdmin {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "admin access required — only SemRels org owners/maintainers are admins",
					"role":  claims.Role,
					"login": claims.Login,
				})
				return
			}
			c.Set("claims", claims)
			c.Set("login", claims.Login)
			c.Set("isAdmin", true)
			c.Next()
			return
		}

		// Fallback: static ADMIN_TOKEN for local dev.
		if adminToken != "" && bearer == adminToken {
			c.Set("login", "admin-token")
			c.Set("isAdmin", true)
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
	}
}

// RequireAuth gates routes that require any authenticated user (admin or regular).
// Accepts: any valid GitHub JWT | legacy ADMIN_TOKEN.
// Sets "claims", "login", "isAdmin" in context.
func RequireAuth(authHandler *handlers.AuthHandler) gin.HandlerFunc {
	adminToken := os.Getenv("ADMIN_TOKEN")

	return func(c *gin.Context) {
		bearer := extractBearer(c)
		if bearer == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required — sign in with GitHub"})
			return
		}

		claims, err := authHandler.ValidateJWT(bearer)
		if err == nil {
			c.Set("claims", claims)
			c.Set("login", claims.Login)
			c.Set("isAdmin", claims.IsAdmin)
			c.Next()
			return
		}

		// Fallback: static ADMIN_TOKEN for local dev.
		if adminToken != "" && bearer == adminToken {
			c.Set("login", "admin-token")
			c.Set("isAdmin", true)
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
			c.Set("isAdmin", claims.IsAdmin)
		}
		c.Next()
	}
}

func extractBearer(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if after, ok := strings.CutPrefix(h, "Bearer "); ok {
		return strings.TrimSpace(after)
	}
	return c.Query("token")
}
