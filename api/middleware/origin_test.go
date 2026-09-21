package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SemRels/semrel-registry/api/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func originRouter(allowed []string) *gin.Engine {
	r := gin.New()
	r.Use(middleware.VerifyOrigin(allowed))
	r.POST("/write", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/read", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func requestWithOrigin(t *testing.T, router *gin.Engine, method, path, origin string) int {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader("{}"))
	req.Host = "registry.semrel.io"
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	router.ServeHTTP(w, req)
	return w.Code
}

// A form on an attacker's page must not be able to drive a write with the
// user's ambient session cookie.
func TestVerifyOriginBlocksForeignOriginWrites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := originRouter([]string{"https://registry.semrel.io"})

	assert.Equal(t, http.StatusForbidden, requestWithOrigin(t, router, http.MethodPost, "/write", "https://evil.example"))
}

func TestVerifyOriginAllowsConfiguredOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := originRouter([]string{"https://admin.semrel.io"})

	assert.Equal(t, http.StatusOK, requestWithOrigin(t, router, http.MethodPost, "/write", "https://admin.semrel.io"))
}

// The admin SPA is served next to the API behind one proxy, so its Origin
// matches the request Host even when no origin is configured explicitly.
func TestVerifyOriginAllowsSameOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := originRouter(nil)

	assert.Equal(t, http.StatusOK, requestWithOrigin(t, router, http.MethodPost, "/write", "https://registry.semrel.io"))
}

// CLI and CI clients send no Origin and authenticate with a bearer token, so
// they are not subject to CSRF and must keep working.
func TestVerifyOriginAllowsNonBrowserClients(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := originRouter([]string{"https://admin.semrel.io"})

	assert.Equal(t, http.StatusOK, requestWithOrigin(t, router, http.MethodPost, "/write", ""))
}

func TestVerifyOriginIgnoresSafeMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := originRouter([]string{"https://admin.semrel.io"})

	assert.Equal(t, http.StatusOK, requestWithOrigin(t, router, http.MethodGet, "/read", "https://evil.example"))
}
