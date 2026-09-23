package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                 string
	DatabaseURL          string
	MigrateDir           string
	Environment          string
	StorageBackend       string // "postgres" (default) or "file"
	StorageDir           string // path used when StorageBackend == "file"
	MetricsQueueSize     int
	MetricsBatchSize     int
	MetricsFlushInterval time.Duration
	// Rate limiting
	RateLimitEnabled    bool
	RateLimitPublicRPM  float64
	RateLimitPluginsRPM float64
	RateLimitAuthRPM    float64
	RateLimitWriteRPM   float64
	RateLimitTrustProxy bool
	// TrustedProxies lists the CIDRs Gin accepts X-Forwarded-For from. Any
	// other peer's forwarding headers are ignored, so a client cannot choose
	// its own rate-limit bucket by spoofing the header.
	TrustedProxies []string

	// Secrets and origins
	JWTSecret      string
	AdminToken     string
	WebhookSecret  string
	AllowedOrigins []string

	// Session cookie
	SessionTTL    time.Duration
	CookieSecure  bool
	CookieDomain  string
	CookieName    string
	FrontendURL   string
	MaxRequestKiB int64

	// AllowedDownloadHosts restricts the hosts a published artifact may live on.
	AllowedDownloadHosts []string

	// Review notifications. Optional: without a relay the outcome is still
	// recorded and shown in the UI, it just is not delivered.
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
}

// Load reads configuration from environment variables.
// It searches for a .env file in the current directory and up to two parent
// directories, so the API can be started from the project root, the api/
// subdirectory, or any other working directory within the repository.
func Load() *Config {
	loadDotEnv()

	cfg := &Config{
		Port:                 getEnv("PORT", ":8080"),
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://dev:dev@localhost:5432/semrel_registry?sslmode=disable"),
		MigrateDir:           getEnv("MIGRATE_DIR", resolveMigrateDir()),
		Environment:          getEnv("ENVIRONMENT", "dev"),
		StorageBackend:       getEnv("STORAGE_BACKEND", "postgres"),
		StorageDir:           getEnv("STORAGE_DIR", "./data"),
		MetricsQueueSize:     getEnvInt("METRICS_QUEUE_SIZE", 2048),
		MetricsBatchSize:     getEnvInt("METRICS_BATCH_SIZE", 200),
		MetricsFlushInterval: getEnvDuration("METRICS_FLUSH_INTERVAL", 2*time.Second),
		// Rate limiting — disabled by default; enabled in prod via RATE_LIMIT_ENABLED=true.
		RateLimitEnabled:    getEnvBool("RATE_LIMIT_ENABLED", false),
		RateLimitPublicRPM:  getEnvFloat("RATE_LIMIT_PUBLIC_RPM", 60),
		RateLimitPluginsRPM: getEnvFloat("RATE_LIMIT_PLUGINS_JSON_RPM", 10),
		RateLimitAuthRPM:    getEnvFloat("RATE_LIMIT_AUTH_RPM", 20),
		RateLimitWriteRPM:   getEnvFloat("RATE_LIMIT_WRITE_RPM", 30),
		RateLimitTrustProxy: getEnvBool("RATE_LIMIT_TRUST_PROXY", true),
		TrustedProxies:      getEnvList("TRUSTED_PROXIES", defaultTrustedProxies),

		JWTSecret:      strings.TrimSpace(os.Getenv("JWT_SECRET")),
		AdminToken:     strings.TrimSpace(os.Getenv("ADMIN_TOKEN")),
		WebhookSecret:  strings.TrimSpace(os.Getenv("WEBHOOK_SECRET")),
		AllowedOrigins: getEnvList("ALLOWED_ORIGINS", nil),

		SessionTTL:    getEnvDuration("SESSION_TTL", 24*time.Hour),
		CookieDomain:  getEnv("COOKIE_DOMAIN", ""),
		CookieName:    getEnv("SESSION_COOKIE_NAME", "semrel_session"),
		FrontendURL:   getEnv("FRONTEND_URL", "http://localhost:5173"),
		MaxRequestKiB: int64(getEnvInt("MAX_REQUEST_KIB", 1024)),

		AllowedDownloadHosts: getEnvList("ALLOWED_DOWNLOAD_HOSTS", defaultDownloadHosts),

		SMTPHost:     getEnv("SMTP_HOST", ""),
		SMTPPort:     getEnvInt("SMTP_PORT", 587),
		SMTPUsername: getEnv("SMTP_USERNAME", ""),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:     getEnv("SMTP_FROM", ""),
	}

	cfg.Port = normalizePort(cfg.Port)

	// Production defaults differ from development defaults: throttling on,
	// cookies Secure-only. Both stay overridable by an explicit env var.
	if cfg.IsProduction() {
		cfg.RateLimitEnabled = getEnvBool("RATE_LIMIT_ENABLED", true)
		cfg.CookieSecure = getEnvBool("COOKIE_SECURE", true)
	} else {
		cfg.CookieSecure = getEnvBool("COOKIE_SECURE", false)
	}

	if cfg.JWTSecret == "" {
		cfg.JWTSecret = DevJWTSecret
	}

	return cfg
}

// resolveMigrateDir returns the migration directory path relative to CWD,
// handling both "run from api/" and "run from project root" cases.
func resolveMigrateDir() string {
	// Check if ./database/migrations exists (running from api/).
	if _, err := os.Stat("./database/migrations"); err == nil {
		return "./database/migrations"
	}
	// Check if ./api/database/migrations exists (running from project root).
	if _, err := os.Stat("./api/database/migrations"); err == nil {
		return "./api/database/migrations"
	}
	return "./database/migrations"
}

// loadDotEnv loads .env files. It walks UP from CWD to find the project root
// .env (loaded first, lower precedence), then loads any local .env in the CWD
// as an override (higher precedence). This means running from either the
// project root or the api/ subdirectory both work correctly.
func loadDotEnv() {
	cwd, err := os.Getwd()
	if err != nil {
		return
	}

	// Highest precedence first: godotenv.Load keeps the first value it sees for
	// a key and never replaces one already in the environment.
	//
	// This used to call Overload, which meant a .env file beat the real process
	// environment — so a stale file baked into an image or left in a working
	// copy silently overrode the variables the deployment actually set, and
	// setting PORT or DATABASE_URL on the command line did nothing.
	candidates := []string{
		filepath.Join(cwd, ".env"),       // closest to the process
		filepath.Join(cwd, "../.env"),    // project root when run from api/
		filepath.Join(cwd, "../../.env"), // one level further out
	}

	var files []string
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			files = append(files, path)
		}
	}
	if len(files) == 0 {
		return
	}

	_ = godotenv.Load(files...)
}

// defaultTrustedProxies covers loopback plus the RFC1918 ranges Docker and
// Kubernetes assign to ingress containers, which is where the reverse proxy
// sits in every deployment topology this repository ships.
var defaultTrustedProxies = []string{
	"127.0.0.1/32", "::1/128",
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7",
}

// defaultDownloadHosts are the hosts that may serve plugin artifacts. GitHub
// release assets and their redirect targets are the only ones the registry
// itself publishes.
var defaultDownloadHosts = []string{
	"github.com",
	"objects.githubusercontent.com",
	"release-assets.githubusercontent.com",
}

// getEnvList splits a comma-separated variable into trimmed, non-empty
// entries, falling back when the variable is unset or blank.
func getEnvList(key string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func getEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func getEnvFloat(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func normalizePort(port string) string {
	if port == "" {
		return ":8080"
	}
	if strings.HasPrefix(port, ":") {
		return port
	}
	return ":" + port
}
