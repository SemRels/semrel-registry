//go:build container

package testutil

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func DatabaseURL(tb testing.TB, baseDir string) string {
	tb.Helper()
	_ = baseDir

	if dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")); dsn != "" {
		return dsn
	}

	ctx := context.Background()
	container, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("semrel_registry"),
		postgres.WithUsername("semrel"),
		postgres.WithPassword("semrel"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	if err != nil {
		tb.Fatalf("start PostgreSQL testcontainer (the container suite requires Docker): %v", err)
	}
	tb.Cleanup(func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			tb.Errorf("terminate PostgreSQL testcontainer: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		tb.Fatalf("get PostgreSQL testcontainer DSN: %v", err)
	}
	return dsn
}
