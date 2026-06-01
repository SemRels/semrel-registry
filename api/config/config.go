package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	DatabaseURL    string
	MigrateDir     string
	Environment    string
	StorageBackend string // "postgres" (default) or "file"
	StorageDir     string // path used when StorageBackend == "file"
}

// Load reads configuration from environment variables.
// It searches for a .env file in the current directory and up to two parent
// directories, so the API can be started from the project root, the api/
// subdirectory, or any other working directory within the repository.
func Load() *Config {
	loadDotEnv()

	cfg := &Config{
		Port:           getEnv("PORT", ":8080"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://dev:dev@localhost:5432/semrel_registry?sslmode=disable"),
		MigrateDir:     getEnv("MIGRATE_DIR", resolveMigrateDir()),
		Environment:    getEnv("ENVIRONMENT", "dev"),
		StorageBackend: getEnv("STORAGE_BACKEND", "postgres"),
		StorageDir:     getEnv("STORAGE_DIR", "./data"),
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

func normalizePort(port string) string {
	if port == "" {
		return ":8080"
	}
	if strings.HasPrefix(port, ":") {
		return port
	}
	return ":" + port
}
