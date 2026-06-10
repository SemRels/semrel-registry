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
	RateLimitTrustProxy bool
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
		RateLimitTrustProxy: getEnvBool("RATE_LIMIT_TRUST_PROXY", true),
	}

	cfg.Port = normalizePort(cfg.Port)
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

	// Collect .env files from root → CWD (root loads first = lowest precedence).
	var files []string
	for _, rel := range []string{"../.env", "../../.env"} {
		path := filepath.Join(cwd, rel)
		if _, err := os.Stat(path); err == nil {
			files = append([]string{path}, files...) // prepend (root first)
		}
	}
	// CWD .env overrides root (appended last = highest precedence for Overload).
	if local := filepath.Join(cwd, ".env"); func() bool {
		_, err := os.Stat(local)
		return err == nil
	}() {
		files = append(files, local)
	}

	if len(files) == 0 {
		return
	}
	// Load in order: root first (sets defaults), local last (overrides).
	// godotenv.Load does NOT override already-set env vars; use Overload so
	// later files in the list take precedence over earlier ones.
	_ = godotenv.Overload(files...)
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
