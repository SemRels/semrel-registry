package service

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"sync"
	"time"
)

// DownloadDedupWindow is how long one client's repeated download of the same
// version counts as a single download.
//
// CI jobs re-install the same plugin on every run, and the tracking endpoint is
// unauthenticated, so without a window the counter measures build frequency —
// or whatever a script feels like reporting — rather than adoption. An hour is
// long enough to collapse a retry storm and short enough that genuine daily use
// still registers.
const DownloadDedupWindow = time.Hour

// maxDedupEntries bounds the table. Beyond it, the oldest half is dropped:
// over-counting after a flood is a better failure than exhausting memory.
const maxDedupEntries = 200_000

// DownloadDeduplicator answers whether a (client, version) pair has already
// been counted recently. It is safe for concurrent use.
type DownloadDeduplicator struct {
	mu      sync.Mutex
	seen    map[string]time.Time
	window  time.Duration
	lastGC  time.Time
	nowFunc func() time.Time
}

func NewDownloadDeduplicator(window time.Duration) *DownloadDeduplicator {
	if window <= 0 {
		window = DownloadDedupWindow
	}
	return &DownloadDeduplicator{
		seen:    make(map[string]time.Time),
		window:  window,
		lastGC:  time.Now(),
		nowFunc: time.Now,
	}
}

// ShouldCount reports whether this download should increment the counter, and
// records it when it should.
//
// The client is identified by a salted hash of its address and user agent: the
// raw address is never stored, so the table cannot become a log of who
// downloaded what.
func (d *DownloadDeduplicator) ShouldCount(clientIP, userAgent string, versionID int64) bool {
	if d == nil {
		return true
	}

	key := dedupKey(clientIP, userAgent, versionID)

	d.mu.Lock()
	defer d.mu.Unlock()

	now := d.nowFunc()
	d.collectLocked(now)

	if last, ok := d.seen[key]; ok && now.Sub(last) < d.window {
		return false
	}
	d.seen[key] = now
	return true
}

func (d *DownloadDeduplicator) collectLocked(now time.Time) {
	if now.Sub(d.lastGC) >= d.window/2 {
		for key, at := range d.seen {
			if now.Sub(at) >= d.window {
				delete(d.seen, key)
			}
		}
		d.lastGC = now
	}

	if len(d.seen) <= maxDedupEntries {
		return
	}
	// Emergency shed: drop everything older than half the window, then give up
	// and clear if that was not enough.
	cutoff := now.Add(-d.window / 2)
	for key, at := range d.seen {
		if at.Before(cutoff) {
			delete(d.seen, key)
		}
	}
	if len(d.seen) > maxDedupEntries {
		d.seen = make(map[string]time.Time, maxDedupEntries/2)
	}
}

func dedupKey(clientIP, userAgent string, versionID int64) string {
	sum := sha256.New()
	sum.Write([]byte(clientIP))
	sum.Write([]byte{0})
	sum.Write([]byte(userAgent))
	sum.Write([]byte{0})
	sum.Write([]byte(strconv.FormatInt(versionID, 10)))
	return hex.EncodeToString(sum.Sum(nil)[:16])
}
