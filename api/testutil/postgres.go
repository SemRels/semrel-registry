package testutil

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
)

func DatabaseURL(tb testing.TB, baseDir string) string {
	tb.Helper()

	if dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")); dsn != "" {
		return dsn
	}

	port := freePort(tb)
	rootDir, err := filepath.Abs(baseDir)
	if err != nil {
		tb.Fatalf("resolve embedded postgres root: %v", err)
	}

	workDir := filepath.Join(rootDir, ".embedded-postgres", fmt.Sprintf("%d", port))
	cacheDir := filepath.Join(workDir, "cache")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		tb.Fatalf("create embedded postgres workdir: %v", err)
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		tb.Fatalf("create embedded postgres cachedir: %v", err)
	}

	postgres := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Username("dev").
		Password("dev").
		Database("semrel_registry").
		Port(uint32(port)).
		CachePath(cacheDir).
		RuntimePath(filepath.Join(workDir, "runtime")).
		DataPath(filepath.Join(workDir, "data")).
		BinariesPath(filepath.Join(workDir, "binaries")).
		StartTimeout(45 * time.Second))
	if err := postgres.Start(); err != nil {
		tb.Fatalf("start embedded postgres: %v", err)
	}

	tb.Cleanup(func() {
		_ = postgres.Stop()
		_ = os.RemoveAll(workDir)
	})

	return fmt.Sprintf("postgres://dev:dev@localhost:%d/semrel_registry?sslmode=disable", port)
}

func freePort(tb testing.TB) uint32 {
	tb.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("reserve tcp port: %v", err)
	}
	defer listener.Close()

	return uint32(listener.Addr().(*net.TCPAddr).Port)
}
