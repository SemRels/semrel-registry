package service

import (
	"testing"
	"time"
)

// The tracking endpoint is unauthenticated, so without deduplication the
// download counter reflects how often someone called it, not how many people
// installed the plugin.
func TestDeduplicatorCountsOncePerWindow(t *testing.T) {
	d := NewDownloadDeduplicator(time.Hour)

	if !d.ShouldCount("203.0.113.1", "semrel/1.0", 42) {
		t.Fatal("first download must count")
	}
	for i := 0; i < 100; i++ {
		if d.ShouldCount("203.0.113.1", "semrel/1.0", 42) {
			t.Fatalf("repeat download %d must not count again", i)
		}
	}
}

func TestDeduplicatorSeparatesClientsAndVersions(t *testing.T) {
	d := NewDownloadDeduplicator(time.Hour)
	d.ShouldCount("203.0.113.1", "semrel/1.0", 42)

	if !d.ShouldCount("203.0.113.2", "semrel/1.0", 42) {
		t.Fatal("a different client must count")
	}
	if !d.ShouldCount("203.0.113.1", "semrel/1.0", 43) {
		t.Fatal("a different version must count")
	}
	if !d.ShouldCount("203.0.113.1", "curl/8.0", 42) {
		t.Fatal("a different user agent must count")
	}
}

func TestDeduplicatorCountsAgainAfterWindow(t *testing.T) {
	d := NewDownloadDeduplicator(time.Hour)
	now := time.Now()
	d.nowFunc = func() time.Time { return now }

	if !d.ShouldCount("203.0.113.1", "semrel/1.0", 42) {
		t.Fatal("first download must count")
	}

	now = now.Add(2 * time.Hour)
	if !d.ShouldCount("203.0.113.1", "semrel/1.0", 42) {
		t.Fatal("a download after the window must count again")
	}
}

// A nil deduplicator must not silently stop counting.
func TestNilDeduplicatorCounts(t *testing.T) {
	var d *DownloadDeduplicator
	if !d.ShouldCount("203.0.113.1", "semrel/1.0", 42) {
		t.Fatal("nil deduplicator must count everything")
	}
}
