package database

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func (d *Database) RunMigrations(dir string) error {
	if d == nil {
		return fmt.Errorf("database is not initialized")
	}
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("migration directory is required")
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve migration directory: %w", err)
	}

	connConfig, err := pgx.ParseConfig(d.DSN())
	if err != nil {
		return fmt.Errorf("parse migration database config: %w", err)
	}

	sqlDB := stdlib.OpenDB(*connConfig)
	defer sqlDB.Close()

	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("create postgres migration driver: %w", err)
	}

	sourceURL := migrationSourceURL(absDir)
	migrator, err := migrate.NewWithDatabaseInstance(sourceURL, "postgres", driver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() {
		_, _ = migrator.Close()
	}()

	if err := migrator.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}

func migrationSourceURL(dir string) string {
	cleanDir := filepath.ToSlash(dir)
	if runtime.GOOS == "windows" && !strings.HasPrefix(cleanDir, "/") {
		cleanDir = "/" + cleanDir
	}

	return "file://" + cleanDir
}
