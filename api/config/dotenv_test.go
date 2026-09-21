package config

import (
	"os"
	"path/filepath"
	"testing"
)

// The process environment is the deployment's actual configuration. A .env file
// that overrides it means a stale file — baked into an image, or left in a
// working copy — silently wins over what the platform set, and a variable
// passed on the command line does nothing.
func TestEnvironmentBeatsDotEnvFile(t *testing.T) {
	// Load copies file values into the real process environment, so tests have
	// to start from a known state or they inherit each other's files.
	clearEnv(t, "PORT", "ENVIRONMENT", "STORAGE_DIR")

	dir := t.TempDir()
	writeEnvFile(t, filepath.Join(dir, ".env"), "PORT=:9999\nENVIRONMENT=from-file\n")
	chdir(t, dir)

	t.Setenv("PORT", ":8091")

	cfg := Load()

	if cfg.Port != ":8091" {
		t.Fatalf("the process environment must win: got %q, want \":8091\"", cfg.Port)
	}
	// A variable the environment does not set still comes from the file.
	if cfg.Environment != "from-file" {
		t.Fatalf("unset variables should fall back to the file: got %q", cfg.Environment)
	}
}

// A .env next to the process beats one further up the tree.
func TestNearestDotEnvWins(t *testing.T) {
	clearEnv(t, "PORT", "ENVIRONMENT", "STORAGE_DIR")

	root := t.TempDir()
	writeEnvFile(t, filepath.Join(root, ".env"), "ENVIRONMENT=root\nSTORAGE_DIR=/root-data\n")

	nested := filepath.Join(root, "api")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	writeEnvFile(t, filepath.Join(nested, ".env"), "ENVIRONMENT=nested\n")
	chdir(t, nested)

	cfg := Load()

	if cfg.Environment != "nested" {
		t.Fatalf("the closest file should win: got %q", cfg.Environment)
	}
	if cfg.StorageDir != "/root-data" {
		t.Fatalf("values only in the outer file should still apply: got %q", cfg.StorageDir)
	}
}

func writeEnvFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

// chdir moves into dir for the duration of the test. Load reads relative paths,
// so the working directory is part of its input.
func chdir(t *testing.T, dir string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
}

// clearEnv unsets keys for the duration of the test and restores them after,
// so one test's .env cannot leak into the next.
func clearEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		previous, existed := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if existed {
				_ = os.Setenv(key, previous)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
}
