package handlers

import (
	"crypto/subtle"
	"log"
	"net/http"
	"os"
	"strings"
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

// CORSMiddleware applies the default cross-origin policy: public reads are
// world-readable, everything else is same-origin only.
//
// Deprecated: prefer CORS with explicit options so deployments can name the
// origins their admin UI is served from.
func CORSMiddleware() gin.HandlerFunc {
	return CORS(CORSOptions{})
}

// RequireAdminToken gates a route on the static ADMIN_TOKEN.
//
// The token is compared in constant time: a byte-wise comparison leaks how much
// of a guess was correct through response timing, which turns a secret into
// something guessable one character at a time.
func RequireAdminToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		expectedToken := strings.TrimSpace(os.Getenv("ADMIN_TOKEN"))
		if expectedToken == "" {
			ServiceUnavailable(c, "Admin token is not configured", nil)
			return
		}

		presented := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(expectedToken), []byte(strings.TrimSpace(presented))) != 1 {
			Unauthorized(c, "Unauthorized", nil)
			return
		}

		c.Set("login", "admin-token")
		c.Set("isAdmin", true)
		c.Next()
	}
}

// LimitRequestBody caps how much of a request body the server will read.
//
// Without a cap, any endpoint that decodes JSON will happily allocate whatever
// a client sends — one request can then consume the memory of the whole
// process. maxKiB <= 0 disables the limit.
func LimitRequestBody(maxKiB int64) gin.HandlerFunc {
	if maxKiB <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	maxBytes := maxKiB << 10

	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}
