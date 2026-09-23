package middleware

import (
	"net/http"
	"strconv"
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

// allowWithRemaining consumes a token and reports how many whole tokens are
// left, for the X-RateLimit-Remaining header.
func (b *bucket) allowWithRemaining(ratePerMin float64) (allowed bool, remaining int) {
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
		return false, 0
	}
	b.tokens--
	return true, int(b.tokens)
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

// maxBuckets caps the per-IP table. Without a cap, a client rotating source
// addresses (trivial over IPv6) grows the map until the process runs out of
// memory — turning the rate limiter itself into the denial-of-service vector.
const maxBuckets = 50_000

// clientIP returns the address the limiter buckets on.
//
// It deliberately relies on gin's c.ClientIP(), which only honours
// X-Forwarded-For when the immediate peer is one of the engine's trusted
// proxies. Reading the header directly — as this used to — lets any caller
// pick its own bucket by sending a different value on every request, which
// makes the limit unenforceable.
func (rl *rateLimiter) clientIP(c *gin.Context) string {
	return c.ClientIP()
}

func (rl *rateLimiter) getBucket(ip string) *bucket {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Evict buckets older than 5 minutes every minute.
	if time.Since(rl.lastCleanup) > time.Minute {
		rl.evictIdleLocked(5 * time.Minute)
		rl.lastCleanup = time.Now()
	}

	if b, ok := rl.buckets[ip]; ok {
		return b
	}

	// Under pressure, evict aggressively before admitting a new bucket; if that
	// is not enough, drop the table entirely rather than grow without bound.
	if len(rl.buckets) >= maxBuckets {
		rl.evictIdleLocked(time.Minute)
		if len(rl.buckets) >= maxBuckets {
			rl.buckets = make(map[string]*bucket, maxBuckets/2)
		}
	}

	b := &bucket{tokens: rl.ratePerMin, lastRefil: time.Now()}
	rl.buckets[ip] = b
	return b
}

// evictIdleLocked removes buckets untouched for longer than idleFor.
// Callers hold rl.mu.
func (rl *rateLimiter) evictIdleLocked(idleFor time.Duration) {
	for key, b := range rl.buckets {
		b.mu.Lock()
		idle := time.Since(b.lastRefil)
		b.mu.Unlock()
		if idle > idleFor {
			delete(rl.buckets, key)
		}
	}
}

func (rl *rateLimiter) middleware() gin.HandlerFunc {
	limit := strconv.FormatFloat(rl.ratePerMin, 'f', -1, 64)

	return func(c *gin.Context) {
		ip := rl.clientIP(c)
		b := rl.getBucket(ip)
		allowed, remaining := b.allowWithRemaining(rl.ratePerMin)

		// Advertise the budget so clients can pace themselves instead of
		// discovering the limit by being cut off.
		c.Header("X-RateLimit-Limit", limit)
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if !allowed {
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
	// WriteRPM limits authenticated write endpoints and the release webhook.
	WriteRPM float64
	// TrustProxy controls whether the engine trusts forwarding headers at all.
	// The set of peers those headers are accepted from is configured on the Gin
	// engine itself (SetTrustedProxies), not here.
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
