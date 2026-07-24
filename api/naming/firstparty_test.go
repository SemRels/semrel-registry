package naming

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFirstPartyCanonicalMappings(t *testing.T) {
	plugins := FirstPartyPlugins()
	require.Len(t, plugins, 42)
	expected := []string{
		"analyzer-conventional", "analyzer-default",
		"condition-bitbucket-pipelines", "condition-circleci", "condition-generic",
		"condition-gitea-actions", "condition-github-actions", "condition-gitlab-ci",
		"generator-changelog-html", "generator-changelog-md", "generator-release-notes",
		"hook-discord", "hook-email", "hook-gitplugin", "hook-jira", "hook-matrix", "hook-slack", "hook-teams",
		"packager-nfpm",
		"provider-bitbucket", "provider-git", "provider-gitea", "provider-github", "provider-gitlab",
		"publisher-crates", "publisher-generic-http", "publisher-npm", "publisher-oci", "publisher-pypi",
		"updater-cargo", "updater-composer", "updater-docker", "updater-go", "updater-gradle",
		"updater-helm", "updater-homebrew", "updater-maven", "updater-npm", "updater-nuget",
		"updater-pubspec", "updater-python", "updater-terraform",
	}
	actual := make([]string, 0, len(plugins))
	for _, plugin := range plugins {
		actual = append(actual, plugin.Name)
	}
	assert.Equal(t, expected, actual)

	seen := make(map[string]bool, len(plugins))
	for _, plugin := range plugins {
		assert.Equal(t, plugin.Repository, plugin.Name)
		assert.Equal(t, FirstPartyNamespace+"/"+plugin.Repository, FirstPartyNamespace+"/"+plugin.Name)
		assert.NotEmpty(t, plugin.Category)
		assert.False(t, seen[plugin.Name], "duplicate canonical name %q", plugin.Name)
		seen[plugin.Name] = true

		mapped, ok := FirstPartyByRepository("https://github.com/SemRels/" + plugin.Repository)
		require.True(t, ok)
		assert.Equal(t, plugin, mapped)
		assert.Contains(t, plugin.Aliases, plugin.Repository)
	}
}

func TestAmbiguousNPMAliasHasHistoricalTarget(t *testing.T) {
	updater, ok := FirstPartyByRepository("updater-npm")
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"updater-npm", "@semrel/npm", "npm"}, updater.Aliases)

	publisher, ok := FirstPartyByRepository("publisher-npm")
	require.True(t, ok)
	assert.Equal(t, []string{"publisher-npm"}, publisher.Aliases)

	for _, ref := range []string{"npm", "@semrel/npm", "updater-npm", "@semrel/updater-npm"} {
		resolved, found := ResolveFirstPartyRef(ref)
		require.True(t, found)
		assert.Equal(t, "updater-npm", resolved.Name)
	}
	resolved, found := ResolveFirstPartyRef("@semrel/publisher-npm")
	require.True(t, found)
	assert.Equal(t, "publisher-npm", resolved.Name)
}

func TestFirstPartyByRepositoryRejectsOtherOrganizations(t *testing.T) {
	_, ok := FirstPartyByRepository("https://github.com/example/provider-github")
	assert.False(t, ok)
}

func TestFirstPartyByRepositoryURLRequiresExactAllowlistedURL(t *testing.T) {
	for _, repository := range []string{
		"provider-github",
		"SemRels/provider-github",
		"https://github.com/example/provider-github",
		"https://github.com/SemRels/community-plugin",
		"https://github.com/SemRels/provider-github/issues",
		"http://github.com/SemRels/provider-github",
	} {
		_, ok := FirstPartyByRepositoryURL(repository)
		assert.False(t, ok, repository)
	}

	for _, repository := range []string{
		"https://github.com/SemRels/provider-github",
		"https://GITHUB.com/semrels/provider-github.git",
	} {
		plugin, ok := FirstPartyByRepositoryURL(repository)
		require.True(t, ok, repository)
		assert.Equal(t, "provider-github", plugin.Name)
	}
}
