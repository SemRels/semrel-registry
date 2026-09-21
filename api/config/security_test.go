package config

import (
	"strings"
	"testing"
)

func TestValidateRejectsDevelopmentJWTSecretInProduction(t *testing.T) {
	cfg := &Config{Environment: "prod", JWTSecret: DevJWTSecret, WebhookSecret: strings.Repeat("w", 32)}

	_, err := cfg.Validate()
	if err == nil {
		t.Fatal("expected production start-up to fail with the public development secret")
	}
	if !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("error should name the offending variable, got: %v", err)
	}
}

func TestValidateRejectsShortJWTSecretInProduction(t *testing.T) {
	cfg := &Config{Environment: "production", JWTSecret: "too-short", WebhookSecret: strings.Repeat("w", 32)}

	if _, err := cfg.Validate(); err == nil {
		t.Fatal("expected a secret below the minimum length to be rejected")
	}
}

func TestValidateRequiresWebhookSecretInProduction(t *testing.T) {
	cfg := &Config{Environment: "prod", JWTSecret: strings.Repeat("j", 32)}

	_, err := cfg.Validate()
	if err == nil {
		t.Fatal("expected production start-up to fail without a webhook secret")
	}
	if !strings.Contains(err.Error(), "WEBHOOK_SECRET") {
		t.Fatalf("error should name the offending variable, got: %v", err)
	}
}

// The static admin token is a non-expiring, identity-less credential. It stays
// available for local development but must never be honoured in production.
func TestValidateDisablesAdminTokenInProduction(t *testing.T) {
	cfg := &Config{
		Environment:   "prod",
		JWTSecret:     strings.Repeat("j", 32),
		WebhookSecret: strings.Repeat("w", 32),
		AdminToken:    "super-secret-admin-token",
	}

	warnings, err := cfg.Validate()
	if err != nil {
		t.Fatalf("configuration should be valid: %v", err)
	}
	if cfg.AdminToken != "" {
		t.Fatal("ADMIN_TOKEN must be cleared in production")
	}
	if !containsSubstring(warnings, "ADMIN_TOKEN") {
		t.Fatalf("operator should be warned that the token is ignored, got: %v", warnings)
	}
}

func TestValidateRejectsWildcardOriginInProduction(t *testing.T) {
	cfg := &Config{
		Environment:    "prod",
		JWTSecret:      strings.Repeat("j", 32),
		WebhookSecret:  strings.Repeat("w", 32),
		AllowedOrigins: []string{"*"},
	}

	if _, err := cfg.Validate(); err == nil {
		t.Fatal("expected a wildcard origin to be rejected in production")
	}
}

func TestValidateOnlyWarnsInDevelopment(t *testing.T) {
	cfg := &Config{Environment: "dev", JWTSecret: DevJWTSecret}

	warnings, err := cfg.Validate()
	if err != nil {
		t.Fatalf("development must not be blocked by advisory problems: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected the insecure development defaults to be reported")
	}
}

func containsSubstring(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
