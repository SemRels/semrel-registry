package handlers

import "testing"

func TestPluginNameFromRepo(t *testing.T) {
	tests := []struct {
		name     string
		repoName string
		want     string
	}{
		{name: "provider repo", repoName: "provider-bitbucket", want: "bitbucket"},
		{name: "analyzer repo", repoName: "analyzer-conventional", want: "conventional"},
		{name: "packager repo", repoName: "packager-nfpm", want: "nfpm"},
		{name: "publisher repo", repoName: "publisher-oci", want: "oci"},
		{name: "publisher-npm disambiguated from updater-npm", repoName: "publisher-npm", want: "publisher-npm"},
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
