package service

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"

	appErrors "github.com/SemRels/semrel-registry/api/internal"
)

// allowedArtifactHosts are the hosts a plugin artifact may be served from.
//
// This matters more than it looks: /api/v1/plugins/:id/versions/:version/download
// redirects to whatever URL the version carries, so an unvalidated field turns
// the registry into both an open redirect and a way to hand out arbitrary
// binaries from a trusted registry.semrel.io URL.
var (
	artifactHostMu       sync.RWMutex
	allowedArtifactHosts = []string{
		"github.com",
		"objects.githubusercontent.com",
		"release-assets.githubusercontent.com",
	}
)

// SetAllowedArtifactHosts replaces the artifact host allowlist. Call it during
// start-up, before the server begins serving requests.
func SetAllowedArtifactHosts(hosts []string) {
	cleaned := make([]string, 0, len(hosts))
	for _, host := range hosts {
		if trimmed := strings.ToLower(strings.TrimSpace(host)); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	if len(cleaned) == 0 {
		return
	}
	artifactHostMu.Lock()
	defer artifactHostMu.Unlock()
	allowedArtifactHosts = cleaned
}

// AllowedArtifactHosts returns the current allowlist.
func AllowedArtifactHosts() []string {
	artifactHostMu.RLock()
	defer artifactHostMu.RUnlock()
	return append([]string(nil), allowedArtifactHosts...)
}

// validateArtifactURL checks that a download URL is an absolute HTTPS URL on an
// allowed host, with no embedded credentials.
func validateArtifactURL(field, raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return &appErrors.ValidationError{Field: field, Issue: "must be a valid URL"}
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return &appErrors.ValidationError{Field: field, Issue: "must use https"}
	}
	if parsed.User != nil {
		return &appErrors.ValidationError{Field: field, Issue: "must not contain embedded credentials"}
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return &appErrors.ValidationError{Field: field, Issue: "must include a host"}
	}
	if !hostAllowed(host) {
		return &appErrors.ValidationError{
			Field: field,
			Issue: fmt.Sprintf("host %q is not an allowed artifact host (allowed: %s)", host, strings.Join(AllowedArtifactHosts(), ", ")),
		}
	}
	return nil
}

// hostAllowed matches a host exactly or as a subdomain of an allowed entry, so
// that "github.com" also covers "codeload.github.com" but never
// "github.com.evil.example".
func hostAllowed(host string) bool {
	for _, allowed := range AllowedArtifactHosts() {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}

// sha256Pattern matches a bare hex SHA-256 digest.
var sha256Pattern = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

// validateChecksum accepts a SHA-256 digest, with or without the "sha256:"
// prefix GitHub uses. A checksum that is not a well-formed digest cannot verify
// anything, so accepting one would give clients false assurance.
func validateChecksum(value string) error {
	digest := strings.TrimPrefix(strings.TrimPrefix(value, "sha256:"), "SHA256:")
	if !sha256Pattern.MatchString(digest) {
		return &appErrors.ValidationError{
			Field: "checksums",
			Issue: "each checksum must be a hex-encoded SHA-256 digest (64 hex characters, optionally prefixed with \"sha256:\")",
		}
	}
	return nil
}
