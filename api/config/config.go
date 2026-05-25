package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
	MigrateDir  string
	Environment string
}

func Load() *Config {
	_ = godotenv.Load()

	cfg := &Config{
		Port:        getEnv("PORT", ":8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://dev:dev@localhost:5432/semrel_registry?sslmode=disable"),
		MigrateDir:  getEnv("MIGRATE_DIR", "./database/migrations"),
		Environment: getEnv("ENVIRONMENT", "dev"),
	}

	cfg.Port = normalizePort(cfg.Port)
	return cfg
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
