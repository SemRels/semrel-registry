package naming

import (
	"net/url"
	"path"
	"strings"
)

const FirstPartyNamespace = "@semrel"

type FirstPartyPlugin struct {
	Repository string
	Category   string
	Name       string
	Aliases    []string
}

var firstPartyRepositories = []string{
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

// legacyShortTargets records the historical owner of every short first-party
// reference. It is intentionally explicit: notably, "npm" belongs to
// updater-npm and must never resolve according to iteration or insertion order.
var legacyShortTargets = map[string]string{
	"conventional": "analyzer-conventional", "default": "analyzer-default",
	"bitbucket-pipelines": "condition-bitbucket-pipelines", "circleci": "condition-circleci",
	"generic": "condition-generic", "gitea-actions": "condition-gitea-actions",
	"github-actions": "condition-github-actions", "gitlab-ci": "condition-gitlab-ci",
	"changelog-html": "generator-changelog-html", "changelog-md": "generator-changelog-md",
	"release-notes": "generator-release-notes",
	"discord":       "hook-discord", "email": "hook-email", "gitplugin": "hook-gitplugin",
	"jira": "hook-jira", "matrix": "hook-matrix", "slack": "hook-slack", "teams": "hook-teams",
	"nfpm":      "packager-nfpm",
	"bitbucket": "provider-bitbucket", "git": "provider-git", "gitea": "provider-gitea",
	"github": "provider-github", "gitlab": "provider-gitlab",
	"crates": "publisher-crates", "generic-http": "publisher-generic-http",
	"oci": "publisher-oci", "pypi": "publisher-pypi",
	"cargo": "updater-cargo", "composer": "updater-composer", "docker": "updater-docker",
	"go": "updater-go", "gradle": "updater-gradle", "helm": "updater-helm",
	"homebrew": "updater-homebrew", "maven": "updater-maven", "npm": "updater-npm",
	"nuget": "updater-nuget", "pubspec": "updater-pubspec", "python": "updater-python",
	"terraform": "updater-terraform",
}

var firstPartyByRepository = buildFirstPartyIndex()
var firstPartyByRef = buildFirstPartyRefIndex()

func buildFirstPartyIndex() map[string]FirstPartyPlugin {
	result := make(map[string]FirstPartyPlugin, len(firstPartyRepositories))
	for _, repository := range firstPartyRepositories {
		category, short, ok := strings.Cut(repository, "-")
		if !ok {
			panic("invalid first-party repository: " + repository)
		}
		aliases := []string{repository}
		if legacyShortTargets[short] == repository {
			aliases = append(aliases, FirstPartyNamespace+"/"+short, short)
		}
		result[repository] = FirstPartyPlugin{
			Repository: repository,
			Category:   category,
			Name:       repository,
			Aliases:    aliases,
		}
	}
	return result
}

func FirstPartyPlugins() []FirstPartyPlugin {
	result := make([]FirstPartyPlugin, 0, len(firstPartyRepositories))
	for _, repository := range firstPartyRepositories {
		plugin := firstPartyByRepository[repository]
		plugin.Aliases = append([]string(nil), plugin.Aliases...)
		result = append(result, plugin)
	}
	return result
}

func buildFirstPartyRefIndex() map[string]FirstPartyPlugin {
	result := make(map[string]FirstPartyPlugin, len(firstPartyRepositories)*3)
	for _, plugin := range firstPartyByRepository {
		refs := append([]string{FirstPartyNamespace + "/" + plugin.Name}, plugin.Aliases...)
		for _, ref := range refs {
			key := strings.ToLower(ref)
			if existing, ok := result[key]; ok && existing.Name != plugin.Name {
				panic("ambiguous first-party alias: " + ref)
			}
			result[key] = plugin
		}
	}
	return result
}

func FirstPartyByRepository(repository string) (FirstPartyPlugin, bool) {
	name := repositoryName(repository)
	return firstPartyByName(name)
}

// FirstPartyByRepositoryURL only accepts an exact GitHub URL for an allowlisted
// SemRels repository. Database maintenance must not interpret bare names or
// community repository paths as first-party identities.
func FirstPartyByRepositoryURL(repository string) (FirstPartyPlugin, bool) {
	parsed, err := url.Parse(strings.TrimSpace(repository))
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") ||
		!strings.EqualFold(parsed.Host, "github.com") ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery ||
		parsed.Fragment != "" || parsed.RawPath != "" ||
		strings.Contains(parsed.EscapedPath(), "%") {
		return FirstPartyPlugin{}, false
	}
	parts := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	if len(parts) != 2 || !strings.EqualFold(parts[0], "SemRels") {
		return FirstPartyPlugin{}, false
	}
	name, err := url.PathUnescape(parts[1])
	if err != nil || strings.Contains(name, "/") {
		return FirstPartyPlugin{}, false
	}
	return firstPartyByName(strings.TrimSuffix(strings.ToLower(name), ".git"))
}

func firstPartyByName(name string) (FirstPartyPlugin, bool) {
	plugin, ok := firstPartyByRepository[name]
	if !ok {
		return FirstPartyPlugin{}, false
	}
	plugin.Aliases = append([]string(nil), plugin.Aliases...)
	return plugin, true
}

func ResolveFirstPartyRef(ref string) (FirstPartyPlugin, bool) {
	plugin, ok := firstPartyByRef[strings.ToLower(strings.TrimSpace(ref))]
	if !ok {
		return FirstPartyPlugin{}, false
	}
	plugin.Aliases = append([]string(nil), plugin.Aliases...)
	return plugin, true
}

func repositoryName(repository string) string {
	repository = strings.TrimSpace(repository)
	if parsed, err := url.Parse(repository); err == nil && parsed.Host != "" {
		if !strings.EqualFold(parsed.Host, "github.com") {
			return ""
		}
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) != 2 || !strings.EqualFold(parts[0], "SemRels") {
			return ""
		}
		return strings.TrimSuffix(parts[1], ".git")
	}
	return strings.TrimSuffix(path.Base(strings.ReplaceAll(repository, `\`, "/")), ".git")
}
