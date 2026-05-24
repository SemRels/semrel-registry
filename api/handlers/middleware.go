package handlers

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		log.Printf("%s %s status=%d duration=%s", c.Request.Method, path, c.Writer.Status(), time.Since(start).Round(time.Millisecond))
	}
}

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("panic recovered: %v", recovered)
				InternalServerError(c, "Internal server error", nil)
			}
		}()

		c.Next()
	}
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func RequireAdminToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		expectedToken := os.Getenv("ADMIN_TOKEN")
		if expectedToken == "" {
			ServiceUnavailable(c, "Admin token is not configured", nil)
			return
		}

		if c.GetHeader("Authorization") != "Bearer "+expectedToken {
			Unauthorized(c, "Unauthorized", nil)
			return
		}

		c.Next()
	}
}
