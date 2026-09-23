package middleware

import (
	"net/http"

	"github.com/SemRels/semrel-registry/api/handlers"
	"github.com/gin-gonic/gin"
)

// RequireAdmin gates routes that only org owners/maintainers (role=admin) may access.
// Accepts: GitHub session (JWT with IsAdmin==true) | static ADMIN_TOKEN, where configured.
func RequireAdmin(authHandler *handlers.AuthHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		bearer := authHandler.TokenFromRequest(c)
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
			setIdentity(c, claims, claims.Login, true)
			c.Next()
			return
		}

		// Fallback: static ADMIN_TOKEN for local dev (disabled in production).
		if authHandler.IsStaticAdminToken(bearer) {
			setIdentity(c, nil, "admin-token", true)
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
	}
}

// RequireAuth gates routes that require any authenticated user (admin or regular).
// Accepts: any valid GitHub session | static ADMIN_TOKEN, where configured.
// Sets "claims", "login", "isAdmin" in context.
func RequireAuth(authHandler *handlers.AuthHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		bearer := authHandler.TokenFromRequest(c)
		if bearer == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required — sign in with GitHub"})
			return
		}

		claims, err := authHandler.ValidateJWT(bearer)
		if err == nil {
			setIdentity(c, claims, claims.Login, claims.IsAdmin)
			c.Next()
			return
		}

		if authHandler.IsStaticAdminToken(bearer) {
			setIdentity(c, nil, "admin-token", true)
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
	}
}

// OptionalAuth attaches claims if a valid session or ADMIN_TOKEN is present,
// but doesn't block anonymous callers.
func OptionalAuth(authHandler *handlers.AuthHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		bearer := authHandler.TokenFromRequest(c)
		if bearer == "" {
			c.Next()
			return
		}
		if claims, err := authHandler.ValidateJWT(bearer); err == nil {
			setIdentity(c, claims, claims.Login, claims.IsAdmin)
		} else if authHandler.IsStaticAdminToken(bearer) {
			setIdentity(c, nil, "admin-token", true)
		}
		c.Next()
	}
}

func setIdentity(c *gin.Context, claims *handlers.Claims, login string, isAdmin bool) {
	if claims != nil {
		c.Set("claims", claims)
	}
	c.Set("login", login)
	c.Set("isAdmin", isAdmin)
}
