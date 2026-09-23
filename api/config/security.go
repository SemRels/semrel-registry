package config

import (
	"fmt"
	"strings"
)

// DevJWTSecret is the insecure fallback used when JWT_SECRET is unset.
// It is only ever accepted outside production; Validate rejects it in prod.
const DevJWTSecret = "dev-jwt-secret-change-in-production" // #nosec G101 -- documented, intentionally insecure dev-only default

// minSecretLength is the minimum length for secrets that protect admin access.
// 32 characters ≈ 192 bits of entropy for a base64/hex secret.
const minSecretLength = 32

// IsProduction reports whether the service runs with production guarantees.
func (c *Config) IsProduction() bool {
	switch strings.ToLower(strings.TrimSpace(c.Environment)) {
	case "prod", "production":
		return true
	default:
		return false
	}
}

// Validate enforces the security invariants that must hold before the server
// starts. Outside production it only reports advisory problems (returned as
// warnings); in production every violation is a hard error, because each one
// silently degrades authentication to something an attacker can forge.
func (c *Config) Validate() (warnings []string, err error) {
	prod := c.IsProduction()

	// ── JWT secret ────────────────────────────────────────────────────────
	switch {
	case c.JWTSecret == "" || c.JWTSecret == DevJWTSecret:
		if prod {
			return warnings, fmt.Errorf("JWT_SECRET must be set in production: the built-in development fallback is public, so anyone could mint an admin token")
		}
		warnings = append(warnings, "JWT_SECRET is unset — using the public development fallback. Never run this configuration in production.")
	case len(c.JWTSecret) < minSecretLength:
		if prod {
			return warnings, fmt.Errorf("JWT_SECRET must be at least %d characters in production (got %d)", minSecretLength, len(c.JWTSecret))
		}
		warnings = append(warnings, fmt.Sprintf("JWT_SECRET is shorter than %d characters — acceptable for local development only.", minSecretLength))
	}

	// ── Static admin token ────────────────────────────────────────────────
	// The static token is a break-glass credential with no expiry, no identity
	// and no revocation. It is disabled outright in production.
	if c.AdminToken != "" {
		if prod {
			warnings = append(warnings, "ADMIN_TOKEN is set but ignored in production — use GitHub OAuth. Remove it from the environment.")
			c.AdminToken = ""
		} else {
			warnings = append(warnings, "ADMIN_TOKEN is enabled (development fallback). It grants full admin access without expiry.")
		}
	}

	// ── Webhook secret ────────────────────────────────────────────────────
	if c.WebhookSecret == "" {
		if prod {
			return warnings, fmt.Errorf("WEBHOOK_SECRET must be set in production: POST /api/v1/webhooks/release would otherwise accept unauthenticated requests that can trigger an organisation-wide sync")
		}
		warnings = append(warnings, "WEBHOOK_SECRET is unset — the release webhook accepts unauthenticated requests.")
	} else if len(c.WebhookSecret) < minSecretLength && prod {
		return warnings, fmt.Errorf("WEBHOOK_SECRET must be at least %d characters in production (got %d)", minSecretLength, len(c.WebhookSecret))
	}

	// ── CORS ──────────────────────────────────────────────────────────────
	if prod && len(c.AllowedOrigins) == 0 {
		warnings = append(warnings, "ALLOWED_ORIGINS is unset — cross-origin browser requests are limited to safe public reads.")
	}
	for _, origin := range c.AllowedOrigins {
		if origin == "*" && prod {
			return warnings, fmt.Errorf(`ALLOWED_ORIGINS must not contain "*" in production: credentialed endpoints would be readable by any site`)
		}
	}

	// ── Rate limiting ─────────────────────────────────────────────────────
	if prod && !c.RateLimitEnabled {
		warnings = append(warnings, "RATE_LIMIT_ENABLED=false in production — public endpoints are unthrottled.")
	}

	return warnings, nil
}
