package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// bucket is a simple token-bucket rate limiter for a single IP.
type bucket struct {
	tokens    float64
	lastRefil time.Time
	mu        sync.Mutex
}

// allow returns true when the request should be allowed (consumes 1 token).
func (b *bucket) allow(ratePerMin float64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefil).Seconds()
	b.lastRefil = now

	// Refill tokens (cap at burst = ratePerMin).
	b.tokens += elapsed * (ratePerMin / 60.0)
	if b.tokens > ratePerMin {
		b.tokens = ratePerMin
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// rateLimiter holds per-IP buckets.
type rateLimiter struct {
	mu          sync.Mutex
	buckets     map[string]*bucket
	ratePerMin  float64
	trustProxy  bool
	lastCleanup time.Time
}

func newRateLimiter(ratePerMin float64, trustProxy bool) *rateLimiter {
	return &rateLimiter{
		buckets:     make(map[string]*bucket),
		ratePerMin:  ratePerMin,
		trustProxy:  trustProxy,
		lastCleanup: time.Now(),
	}
}

func (rl *rateLimiter) clientIP(c *gin.Context) string {
	if rl.trustProxy {
		if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
			return xff
		}
		if xri := c.GetHeader("X-Real-IP"); xri != "" {
			return xri
		}
	}
	return c.ClientIP()
}

func (rl *rateLimiter) getBucket(ip string) *bucket {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Evict buckets older than 5 minutes every minute.
	if time.Since(rl.lastCleanup) > time.Minute {
		for k, b := range rl.buckets {
			b.mu.Lock()
			idle := time.Since(b.lastRefil)
			b.mu.Unlock()
			if idle > 5*time.Minute {
				delete(rl.buckets, k)
			}
		}
		rl.lastCleanup = time.Now()
	}

	if b, ok := rl.buckets[ip]; ok {
		return b
	}
	b := &bucket{tokens: rl.ratePerMin, lastRefil: time.Now()}
	rl.buckets[ip] = b
	return b
}

func (rl *rateLimiter) middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := rl.clientIP(c)
		if !rl.getBucket(ip).allow(rl.ratePerMin) {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": 60,
			})
			return
		}
		c.Next()
	}
}

// RateLimitConfig holds configuration for the rate limiting middleware.
type RateLimitConfig struct {
	// Enabled controls whether rate limiting is applied.
	Enabled bool
	// PublicRPM is the requests-per-minute limit for public read endpoints.
	PublicRPM float64
	// PluginsRPM is the separate (lower) limit for /plugins.json.
	PluginsRPM float64
	// AuthRPM limits auth/OAuth endpoints.
	AuthRPM float64
	// TrustProxy honours X-Forwarded-For / X-Real-IP headers.
	TrustProxy bool
}

// RateLimit returns a Gin middleware function that applies the given per-minute
// request rate limit keyed on client IP.  When cfg.Enabled is false the
// returned middleware is a no-op.
func RateLimit(cfg RateLimitConfig, ratePerMin float64) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) { c.Next() }
	}
	return newRateLimiter(ratePerMin, cfg.TrustProxy).middleware()
}
