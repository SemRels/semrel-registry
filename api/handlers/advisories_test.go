package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/repository"
	"github.com/SemRels/semrel-registry/api/service"
)

func TestConvertGHSARange(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"single upper bound", "< 1.2.4", "<1.2.4"},
		{"lower and upper", ">= 1.0.0, < 2.0.3", ">=1.0.0 <2.0.3"},
		{"exact version", "= 1.2.3", "=1.2.3"},
		{"already tight", ">=1.0.0", ">=1.0.0"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, convertGHSARange(tc.raw))
		})
	}
}

func TestGHSecurityAdvisoryToModel(t *testing.T) {
	raw := `{
		"ghsa_id": "GHSA-aaaa-bbbb-cccc",
		"cve_id": "CVE-2024-0001",
		"summary": "Arbitrary file write",
		"severity": "high",
		"html_url": "https://github.com/acme/plugin/security/advisories/GHSA-aaaa-bbbb-cccc",
		"published_at": "2024-01-15T00:00:00Z",
		"withdrawn_at": null,
		"vulnerabilities": [
			{"vulnerable_version_range": ">= 1.0.0, < 1.2.3", "patched_versions": "1.2.3"}
		]
	}`
	var g ghSecurityAdvisory
	require.NoError(t, json.Unmarshal([]byte(raw), &g))

	advisory := g.toModel()

	assert.Equal(t, "GHSA-aaaa-bbbb-cccc", advisory.GHSAID)
	assert.Equal(t, "CVE-2024-0001", advisory.CVEID)
	assert.Equal(t, "high", advisory.Severity)
	assert.Equal(t, ">=1.0.0 <1.2.3", advisory.VulnerableRange)
	assert.Equal(t, "1.2.3", advisory.PatchedVersion)
	require.NotNil(t, advisory.PublishedAt)
	assert.Nil(t, advisory.WithdrawnAt)
	assert.False(t, advisory.Withdrawn())
}

func TestGHSecurityAdvisoryToModel_Withdrawn(t *testing.T) {
	raw := `{"ghsa_id": "GHSA-x", "withdrawn_at": "2024-02-01T00:00:00Z", "vulnerabilities": []}`
	var g ghSecurityAdvisory
	require.NoError(t, json.Unmarshal([]byte(raw), &g))

	advisory := g.toModel()

	require.NotNil(t, advisory.WithdrawnAt)
	assert.True(t, advisory.Withdrawn())
	assert.Empty(t, advisory.VulnerableRange)
}

func TestAdvisoryAffects(t *testing.T) {
	inRange := models.SecurityAdvisory{VulnerableRange: ">=1.0.0 <1.2.3"}
	assert.True(t, AdvisoryAffects(inRange, "1.1.0"))
	assert.False(t, AdvisoryAffects(inRange, "1.2.3"))
	assert.False(t, AdvisoryAffects(inRange, "0.9.0"))
}

func TestAdvisoryAffects_WithdrawnNeverAffects(t *testing.T) {
	now := time.Now()
	advisory := models.SecurityAdvisory{VulnerableRange: ">=1.0.0", WithdrawnAt: &now}
	assert.False(t, AdvisoryAffects(advisory, "1.5.0"))
}

func TestAdvisoryAffects_NoRangeIsConservative(t *testing.T) {
	assert.True(t, AdvisoryAffects(models.SecurityAdvisory{}, "1.5.0"))
}

func TestAdvisoryAffects_UnparseableRangeIsConservative(t *testing.T) {
	advisory := models.SecurityAdvisory{VulnerableRange: "not a range"}
	assert.True(t, AdvisoryAffects(advisory, "1.5.0"))
}

func TestAuditPluginsReportsAffectingAdvisoriesOnly(t *testing.T) {
	fileRepo, err := repository.NewFileRepository(t.TempDir())
	require.NoError(t, err)
	svc := service.NewPluginService(fileRepo)

	id, err := fileRepo.Create(context.Background(), &models.Plugin{
		Name: "analyzer-conventional", Category: "analyzer",
		Repository: "https://github.com/acme/analyzer-conventional", Status: models.StatusActive,
	})
	require.NoError(t, err)
	require.NoError(t, fileRepo.SetSecurityAdvisories(context.Background(), id, []models.SecurityAdvisory{
		{GHSAID: "GHSA-old", VulnerableRange: "<1.0.0"},             // does not affect 1.5.0
		{GHSAID: "GHSA-current", VulnerableRange: ">=1.0.0 <2.0.0"}, // affects 1.5.0
	}))

	other, err := fileRepo.Create(context.Background(), &models.Plugin{
		Name: "generator-changelog", Category: "generator",
		Repository: "https://github.com/acme/generator-changelog", Status: models.StatusActive,
	})
	require.NoError(t, err)
	require.NoError(t, fileRepo.SetSecurityAdvisories(context.Background(), other, nil))

	handler := NewPluginHandler(svc)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/audit", handler.AuditPlugins)

	body := `{"plugins":[
		{"ref":"analyzer-conventional","version":"1.5.0"},
		{"ref":"generator-changelog","version":"1.0.0"},
		{"ref":"does-not-exist","version":"1.0.0"}
	]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	var payload struct {
		Data []struct {
			Ref        string                    `json:"ref"`
			Version    string                    `json:"version"`
			Advisories []models.SecurityAdvisory `json:"advisories"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
	require.Len(t, payload.Data, 1)
	assert.Equal(t, "analyzer-conventional", payload.Data[0].Ref)
	require.Len(t, payload.Data[0].Advisories, 1)
	assert.Equal(t, "GHSA-current", payload.Data[0].Advisories[0].GHSAID)
}
