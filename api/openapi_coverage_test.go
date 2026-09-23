package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/SemRels/semrel-registry/api/handlers"
	"github.com/SemRels/semrel-registry/api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// routesNotInSpec are endpoints deliberately left undocumented: the spec
// describes the API surface consumers program against, and these are neither
// stable nor useful to them.
var routesNotInSpec = map[string]bool{
	"GET /docs":         true, // the rendered viewer for the spec itself
	"GET /openapi.json": true, // the spec cannot sensibly describe itself
}

// TestOpenAPICoversEveryRoute is what keeps a hand-written specification
// honest. Without it, the document silently becomes a description of an API
// that no longer exists — which is worse than having no document at all,
// because callers trust it.
func TestOpenAPICoversEveryRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newRouter(stubPluginService{})

	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	require.NoError(t, json.Unmarshal(handlers.OpenAPISpecBytes(), &spec))

	documented := make(map[string]bool)
	for path, operations := range spec.Paths {
		for method := range operations {
			switch strings.ToUpper(method) {
			case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
				documented[strings.ToUpper(method)+" "+path] = true
			}
		}
	}

	var missing []string
	for _, route := range router.Routes() {
		key := route.Method + " " + specPath(route.Path)
		if routesNotInSpec[key] || documented[key] {
			continue
		}
		missing = append(missing, key+"  (registered as "+route.Method+" "+route.Path+")")
	}

	require.Emptyf(t, missing, "routes missing from api/handlers/openapi.json:\n  %s", strings.Join(missing, "\n  "))
}

// TestOpenAPIDocumentsNoPhantomRoutes catches the opposite drift: an endpoint
// described in the spec that the server does not serve.
func TestOpenAPIDocumentsNoPhantomRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newRouter(stubPluginService{})

	registered := make(map[string]bool)
	for _, route := range router.Routes() {
		registered[route.Method+" "+specPath(route.Path)] = true
	}

	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	require.NoError(t, json.Unmarshal(handlers.OpenAPISpecBytes(), &spec))

	var phantom []string
	for path, operations := range spec.Paths {
		for method := range operations {
			upper := strings.ToUpper(method)
			switch upper {
			case "GET", "POST", "PUT", "DELETE", "PATCH":
			default:
				continue
			}
			if !registered[upper+" "+path] {
				phantom = append(phantom, upper+" "+path)
			}
		}
	}

	require.Emptyf(t, phantom, "documented but not served:\n  %s", strings.Join(phantom, "\n  "))
}

// specPath converts Gin's ":param" and "@:namespace" segments into the
// "{param}" form OpenAPI uses.
func specPath(ginPath string) string {
	segments := strings.Split(ginPath, "/")
	for i, segment := range segments {
		switch {
		case strings.HasPrefix(segment, "@:"):
			segments[i] = "@{" + strings.TrimPrefix(segment, "@:") + "}"
		case strings.HasPrefix(segment, ":"):
			segments[i] = "{" + strings.TrimPrefix(segment, ":") + "}"
		case strings.HasPrefix(segment, "*"):
			segments[i] = "{" + strings.TrimPrefix(segment, "*") + "}"
		}
	}
	return strings.Join(segments, "/")
}

var _ service.PluginManager = stubPluginService{}
