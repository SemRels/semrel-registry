package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SemRels/semrel-registry/api/models"
	"github.com/SemRels/semrel-registry/api/naming"
	"github.com/SemRels/semrel-registry/api/repository"
	"github.com/SemRels/semrel-registry/api/service"
	"github.com/gin-gonic/gin"
)

func TestPluginNameFromRepo(t *testing.T) {
	tests := []struct {
		name     string
		repoName string
		want     string
	}{
		{name: "provider repo", repoName: "provider-bitbucket", want: "provider-bitbucket"},
		{name: "analyzer repo", repoName: "analyzer-conventional", want: "analyzer-conventional"},
		{name: "packager repo", repoName: "packager-nfpm", want: "packager-nfpm"},
		{name: "publisher repo", repoName: "publisher-npm", want: "publisher-npm"},
		{name: "unknown prefix", repoName: "tool-foo", want: "tool-foo"},
		{name: "already simplified", repoName: "bitbucket", want: "bitbucket"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := pluginNameFromRepo(tc.repoName)
			if got != tc.want {
				t.Fatalf("pluginNameFromRepo(%q)=%q, want %q", tc.repoName, got, tc.want)
			}
		})
	}
}

func TestPluginNameValidationAllowsCurrentFirstPartyCategories(t *testing.T) {
	for _, repo := range []string{"publisher-crates", "packager-nfpm"} {
		parts := strings.SplitN(repo, "-", 2)
		if len(parts) != 2 || !naming.IsFirstPartyCategory(parts[0]) || !pluginNameRE.MatchString(parts[1]) {
			t.Fatalf("expected %q to pass first-party naming validation", repo)
		}
	}
}

func TestNamespaceForOrg(t *testing.T) {
	t.Run("uses explicit env var", func(t *testing.T) {
		t.Setenv("GITHUB_ORG_NAMESPACE", "@custom")
		if got := namespaceForOrg("SemRels"); got != "@custom" {
			t.Fatalf("namespaceForOrg returned %q, want %q", got, "@custom")
		}
	})

	t.Run("defaults semrels to at-semrel", func(t *testing.T) {
		t.Setenv("GITHUB_ORG_NAMESPACE", "")
		if got := namespaceForOrg("SemRels"); got != "@semrel" {
			t.Fatalf("namespaceForOrg returned %q, want %q", got, "@semrel")
		}
	})

	t.Run("other org without env remains empty", func(t *testing.T) {
		t.Setenv("GITHUB_ORG_NAMESPACE", "")
		if got := namespaceForOrg("OtherOrg"); got != "" {
			t.Fatalf("namespaceForOrg returned %q, want empty", got)
		}
	})
}

func TestFirstPartyMigrationRequiresExactMatchingRepository(t *testing.T) {
	tests := []struct {
		name       string
		discovered string
		stored     string
		want       bool
	}{
		{
			name: "exact official repository", want: true,
			discovered: "https://github.com/SemRels/provider-github",
			stored:     "https://github.com/SemRels/provider-github",
		},
		{
			name:       "community discovery",
			discovered: "https://github.com/community/provider-github",
			stored:     "https://github.com/SemRels/provider-github",
		},
		{
			name:       "community stored repository",
			discovered: "https://github.com/SemRels/provider-github",
			stored:     "https://github.com/community/provider-github",
		},
		{
			name:       "example path",
			discovered: "https://github.com/SemRels/provider-github",
			stored:     "https://example.com/SemRels/provider-github",
		},
		{
			name:       "wrong github host",
			discovered: "https://github.com/SemRels/provider-github",
			stored:     "https://github.com.evil/SemRels/provider-github",
		},
		{
			name:       "different official repository",
			discovered: "https://github.com/SemRels/provider-github",
			stored:     "https://github.com/SemRels/provider-git",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchesDiscoveredFirstPartyRepository(tc.discovered, tc.stored); got != tc.want {
				t.Fatalf("matchesDiscoveredFirstPartyRepository(%q, %q)=%v, want %v",
					tc.discovered, tc.stored, got, tc.want)
			}
		})
	}
}

func TestOrgSyncDiscoveryRequiresAllowlistedFirstPartyRepository(t *testing.T) {
	for _, tc := range []struct {
		org, repository string
		want            bool
	}{
		{org: "SemRels", repository: "provider-github", want: true},
		{org: "SemRels", repository: "provider-experimental", want: false},
		{org: "community", repository: "provider-github", want: false},
	} {
		_, _, got := discoverFirstPartyRepository(tc.org, tc.repository)
		if got != tc.want {
			t.Fatalf("%s/%s discovery=%v, want %v", tc.org, tc.repository, got, tc.want)
		}
	}
}

func TestOrgSyncIgnoresUnknownAndNeverUpdatesMismatchedCanonicalRow(t *testing.T) {
	t.Setenv("GITHUB_ORG_NAMESPACE", "")
	repo, err := repository.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	existing := models.Plugin{
		Namespace:  "@semrel",
		Name:       "provider-github",
		Category:   "provider",
		Repository: "https://github.com/community/provider-github",
		Status:     models.StatusActive,
	}
	if _, err := repo.Create(context.Background(), &existing); err != nil {
		t.Fatal(err)
	}

	results, err := NewSyncHandler(service.NewPluginService(repo)).syncOrgRepositories(
		context.Background(), "SemRels", []ghRepo{
			{Name: "provider-experimental"},
			{Name: "provider-github", Description: "must not be applied"},
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Repo != "provider-github" ||
		results[0].Action != "skipped" {
		t.Fatalf("unexpected results: %+v", results)
	}
	persisted, err := repo.GetByID(context.Background(), existing.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Repository != existing.Repository || persisted.Description != "" ||
		len(persisted.Versions) != 0 {
		t.Fatalf("mismatched canonical row was updated: %+v", persisted)
	}
}

func TestPluginsJSONEmitsCanonicalIdentityAndAliases(t *testing.T) {
	repo, err := repository.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	manager := service.NewPluginService(repo)
	if _, err := manager.CreatePlugin(context.Background(), models.Plugin{
		Name: "npm", Category: "updater",
		Repository: "https://github.com/SemRels/updater-npm",
		Status:     models.StatusActive,
	}); err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	router.GET("/plugins.json", NewSyncHandler(manager).PluginsJSON)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/plugins.json", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("PluginsJSON status=%d, body=%s", response.Code, response.Body)
	}
	var payload struct {
		SchemaVersion int `json:"schemaVersion"`
		Plugins       []struct {
			Namespace string   `json:"namespace"`
			Name      string   `json:"name"`
			Aliases   []string `json:"aliases"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.SchemaVersion != 2 || len(payload.Plugins) != 1 {
		t.Fatalf("unexpected registry payload: %+v", payload)
	}
	plugin := payload.Plugins[0]
	if plugin.Namespace != "@semrel" || plugin.Name != "updater-npm" {
		t.Fatalf("unexpected canonical identity: %+v", plugin)
	}
	if len(plugin.Aliases) != 3 {
		t.Fatalf("unexpected aliases: %v", plugin.Aliases)
	}
}
