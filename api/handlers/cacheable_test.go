package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cacheableRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/catalogue", func(c *gin.Context) {
		writeCacheableJSON(c, map[string]any{"schemaVersion": 2, "plugins": []string{"a", "b"}}, 5*time.Minute)
	})
	return r
}

// The catalogue is fetched by every `semrel plugin install`; serving it in full
// each time, with no validator, is the difference between a 304 and a few
// hundred kilobytes per CI job.
func TestCacheableJSONServesValidatorAndCacheControl(t *testing.T) {
	w := httptest.NewRecorder()
	cacheableRouter().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/catalogue", nil))

	require.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("ETag"))
	assert.Contains(t, w.Header().Get("Cache-Control"), "max-age=300")
}

func TestCacheableJSONAnswers304ForKnownETag(t *testing.T) {
	router := cacheableRouter()

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/catalogue", nil))
	etag := first.Header().Get("ETag")
	require.NotEmpty(t, etag)

	second := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/catalogue", nil)
	req.Header.Set("If-None-Match", etag)
	router.ServeHTTP(second, req)

	assert.Equal(t, http.StatusNotModified, second.Code)
	assert.Empty(t, second.Body.String())
}

func TestCacheableJSONMatchesWeakAndListedETags(t *testing.T) {
	router := cacheableRouter()
	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/catalogue", nil))
	etag := first.Header().Get("ETag")

	for name, header := range map[string]string{
		"weak validator": "W/" + etag,
		"in a list":      `"other", ` + etag,
		"wildcard":       "*",
	} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/catalogue", nil)
			req.Header.Set("If-None-Match", header)
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusNotModified, w.Code)
		})
	}
}

func TestCacheableJSONSendsBodyForStaleETag(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/catalogue", nil)
	req.Header.Set("If-None-Match", `"outdated"`)
	cacheableRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "schemaVersion")
}
