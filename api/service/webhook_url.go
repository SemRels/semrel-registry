package service

import (
	"context"
	"net"
	"net/url"
	"strings"

	appErrors "github.com/SemRels/semrel-registry/api/internal"
)

// ValidateWebhookURL checks that a consumer-supplied webhook URL is a
// plausible external HTTPS endpoint, not a way to make the registry issue
// signed, secret-bearing requests into its own internal network.
//
// Unlike an artifact URL, a webhook URL cannot be pinned to an allowlist — it
// is whatever server a consumer runs. The check goes the other way: reject
// anything other than https, reject embedded credentials, and reject any host
// that resolves to a loopback, private, link-local, or unspecified address.
// Callers that deliver to a previously-validated URL should call this again
// immediately before dialing, since DNS answers can change between the two.
func ValidateWebhookURL(ctx context.Context, raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return &appErrors.ValidationError{Field: "url", Issue: "must be a valid URL"}
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return &appErrors.ValidationError{Field: "url", Issue: "must use https"}
	}
	if parsed.User != nil {
		return &appErrors.ValidationError{Field: "url", Issue: "must not contain embedded credentials"}
	}
	host := parsed.Hostname()
	if host == "" {
		return &appErrors.ValidationError{Field: "url", Issue: "must include a host"}
	}

	if ip := net.ParseIP(host); ip != nil {
		if !publiclyRoutable(ip) {
			return &appErrors.ValidationError{Field: "url", Issue: "must not point at a private or internal address"}
		}
		return nil
	}

	addrs, err := (&net.Resolver{}).LookupIPAddr(ctx, host)
	if err != nil || len(addrs) == 0 {
		return &appErrors.ValidationError{Field: "url", Issue: "host does not resolve"}
	}
	for _, addr := range addrs {
		if !publiclyRoutable(addr.IP) {
			return &appErrors.ValidationError{Field: "url", Issue: "must not point at a private or internal address"}
		}
	}
	return nil
}

func publiclyRoutable(ip net.IP) bool {
	return !ip.IsLoopback() &&
		!ip.IsPrivate() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsUnspecified() &&
		!ip.IsMulticast()
}
