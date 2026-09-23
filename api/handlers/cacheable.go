package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// pluginsJSONMaxAge is how long a client may reuse the catalogue without
// revalidating. The catalogue changes only when a plugin is published, so a few
// minutes of staleness costs nothing and saves the registry from serving the
// whole index to every `semrel plugin install` in a CI fleet.
const pluginsJSONMaxAge = 5 * time.Minute

// writeCacheableJSON serialises payload, tags it with a strong ETag derived
// from the bytes, and answers 304 when the client already has that version.
//
// The catalogue is the hottest endpoint in the registry — every CLI run fetches
// it — and it was previously sent in full on every request, with no validator
// and no cache directive.
func writeCacheableJSON(c *gin.Context, payload any, maxAge time.Duration) {
	body, err := json.Marshal(payload)
	if err != nil {
		InternalServerError(c, "failed to encode response", err)
		return
	}

	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`

	header := c.Writer.Header()
	header.Set("ETag", etag)
	header.Set("Cache-Control", "public, max-age="+durationSeconds(maxAge)+", stale-while-revalidate=60")
	header.Add("Vary", "Accept-Encoding")

	if matchesETag(c.GetHeader("If-None-Match"), etag) {
		c.Status(http.StatusNotModified)
		return
	}

	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// matchesETag implements the If-None-Match comparison: a list of candidates, a
// literal "*", and weak validators that differ from ours only by the W/ prefix.
func matchesETag(ifNoneMatch, etag string) bool {
	ifNoneMatch = strings.TrimSpace(ifNoneMatch)
	if ifNoneMatch == "" {
		return false
	}
	if ifNoneMatch == "*" {
		return true
	}
	for _, candidate := range strings.Split(ifNoneMatch, ",") {
		candidate = strings.TrimSpace(candidate)
		candidate = strings.TrimPrefix(candidate, "W/")
		if candidate == etag {
			return true
		}
	}
	return false
}

func durationSeconds(d time.Duration) string {
	seconds := int64(d.Seconds())
	if seconds < 0 {
		seconds = 0
	}
	return strconv.FormatInt(seconds, 10)
}
