package handlers

import (
	"net"
	"net/http"
	"time"
)

// githubHTTPClient is the shared client for outbound GitHub API calls.
//
// http.DefaultClient has no timeout: a GitHub endpoint that accepts the
// connection and then stalls holds the goroutine — and the inbound request
// behind it — open indefinitely. Under a slow upstream that is enough to
// exhaust the server. Every bound below is deliberate rather than inherited.
var githubHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          50,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
	},
}
