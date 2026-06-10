package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SemRels/semrel-registry/api/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestRouter(mw gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.GET("/test", mw, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return r
}

// TestRateLimitDisabled verifies the middleware is a no-op when Enabled=false.
func TestRateLimitDisabled(t *testing.T) {
	cfg := middleware.RateLimitConfig{Enabled: false, PublicRPM: 2}
	mw := middleware.RateLimit(cfg, cfg.PublicRPM)
	router := newTestRouter(mw)

	// 10 requests should all succeed because limiter is disabled.
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should pass when disabled", i+1)
	}
}

// TestRateLimitBlocks verifies that requests above the RPM limit receive 429.
func TestRateLimitBlocks(t *testing.T) {
	// RPM=3 means burst=3 tokens. After 3 requests from the same IP, the next
	// one should be blocked.
	cfg := middleware.RateLimitConfig{Enabled: true, PublicRPM: 3, TrustProxy: false}
	mw := middleware.RateLimit(cfg, 3)
	router := newTestRouter(mw)

	allowed := 0
	blocked := 0
	for i := 0; i < 6; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		router.ServeHTTP(w, req)
		if w.Code == http.StatusOK {
			allowed++
		} else {
			blocked++
		}
	}
	assert.Equal(t, 3, allowed, "should allow first 3 requests (burst=RPM)")
	assert.Equal(t, 3, blocked, "should block remaining 3 requests")
}

// TestRateLimitRetryAfterHeader ensures the Retry-After header is set on 429.
func TestRateLimitRetryAfterHeader(t *testing.T) {
	cfg := middleware.RateLimitConfig{Enabled: true, PublicRPM: 1, TrustProxy: false}
	mw := middleware.RateLimit(cfg, 1)
	router := newTestRouter(mw)

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "10.0.0.2:5678"
		router.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			require.Equal(t, "60", w.Header().Get("Retry-After"), "Retry-After should be 60")
			return
		}
	}
	t.Fatal("expected at least one 429 response")
}

// TestRateLimitPerIP verifies that different IPs have independent buckets.
func TestRateLimitPerIP(t *testing.T) {
	cfg := middleware.RateLimitConfig{Enabled: true, PublicRPM: 1, TrustProxy: false}
	mw := middleware.RateLimit(cfg, 1)
	router := newTestRouter(mw)

	// IP A uses its token.
	wA := httptest.NewRecorder()
	reqA, _ := http.NewRequest(http.MethodGet, "/test", nil)
	reqA.RemoteAddr = "192.168.1.1:100"
	router.ServeHTTP(wA, reqA)
	assert.Equal(t, http.StatusOK, wA.Code, "IP A first request should be OK")

	// IP B should still have its own full bucket.
	wB := httptest.NewRecorder()
	reqB, _ := http.NewRequest(http.MethodGet, "/test", nil)
	reqB.RemoteAddr = "192.168.1.2:100"
	router.ServeHTTP(wB, reqB)
	assert.Equal(t, http.StatusOK, wB.Code, "IP B should be independent from IP A")
}

// TestRateLimitTrustProxy verifies that X-Forwarded-For is used when TrustProxy=true.
func TestRateLimitTrustProxy(t *testing.T) {
	cfg := middleware.RateLimitConfig{Enabled: true, PublicRPM: 1, TrustProxy: true}
	mw := middleware.RateLimit(cfg, 1)
	router := newTestRouter(mw)

	// First request from a forwarded IP — consumes the token.
	w1 := httptest.NewRecorder()
	r1, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r1.RemoteAddr = "10.0.0.5:9999"
	r1.Header.Set("X-Forwarded-For", "203.0.113.42")
	router.ServeHTTP(w1, r1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Second request with same X-Forwarded-For should be blocked.
	w2 := httptest.NewRecorder()
	r2, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r2.RemoteAddr = "10.0.0.5:9999"
	r2.Header.Set("X-Forwarded-For", "203.0.113.42")
	router.ServeHTTP(w2, r2)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)

	// Same physical IP but different forwarded header — independent bucket.
	w3 := httptest.NewRecorder()
	r3, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r3.RemoteAddr = "10.0.0.5:9999"
	r3.Header.Set("X-Forwarded-For", "203.0.113.99")
	router.ServeHTTP(w3, r3)
	assert.Equal(t, http.StatusOK, w3.Code, "different X-Forwarded-For should have its own bucket")
}
