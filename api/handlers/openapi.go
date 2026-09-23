package handlers

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

// openAPISpec is the hand-maintained API description.
//
// Embedding it rather than serving a file keeps the binary self-contained, and
// TestOpenAPICoversEveryRoute fails the build when a route is added without a
// corresponding entry — which is what stops a hand-written spec from drifting
// into fiction.
//
//go:embed openapi.json
var openAPISpec []byte

// OpenAPISpec serves the machine-readable API description.
func OpenAPISpec() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=3600")
		c.Data(http.StatusOK, "application/json; charset=utf-8", openAPISpec)
	}
}

// OpenAPISpecBytes exposes the embedded document to tests.
func OpenAPISpecBytes() []byte { return openAPISpec }

// APIDocs serves a rendered reference for the spec above.
//
// The page pulls its viewer from the same CDN the admin container's CSP already
// allows, and renders the spec fetched from this origin.
func APIDocs() gin.HandlerFunc {
	const page = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>semrel registry API</title>
<style>body { margin: 0; }</style>
</head>
<body>
<rapi-doc spec-url="/openapi.json" theme="dark" render-style="read" show-header="false"
          allow-try="true" regular-font="system-ui, sans-serif" primary-color="#6366f1"></rapi-doc>
<script type="module" src="https://cdn.jsdelivr.net/npm/rapidoc@9.3.8/dist/rapidoc-min.js"></script>
</body>
</html>`

	return func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=3600")
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
	}
}
