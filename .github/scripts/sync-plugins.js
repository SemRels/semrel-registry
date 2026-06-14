// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

const fs = require('fs');
const path = require('path');
const {
  REQUIRED_CHECKSUM_KEYS,
  sortVersionsDescending,
  validatePlugin
} = require('./registry-utils');

const CATEGORY_ORDER = ['provider', 'analyzer', 'generator', 'condition', 'hook', 'updater'];
const OFFICIAL_PLUGINS = [
  { repo: 'provider-github', name: 'github', category: 'provider', description: 'GitHub releases provider', tags: ['github', 'provider', 'releases'] },
  { repo: 'provider-gitlab', name: 'gitlab', category: 'provider', description: 'GitLab releases provider', tags: ['gitlab', 'provider', 'releases'] },
  { repo: 'provider-gitea', name: 'gitea', category: 'provider', description: 'Gitea releases provider', tags: ['gitea', 'provider', 'releases'] },
  { repo: 'provider-git', name: 'git', category: 'provider', description: 'Git provider', tags: ['git', 'provider', 'repository'] },
  { repo: 'provider-bitbucket', name: 'bitbucket', category: 'provider', description: 'Bitbucket releases provider', tags: ['bitbucket', 'provider', 'releases'] },
  { repo: 'analyzer-conventional', name: 'conventional', category: 'analyzer', description: 'Conventional commits analyzer', tags: ['analyzer', 'commits', 'conventional'] },
  { repo: 'analyzer-default', name: 'default', category: 'analyzer', description: 'Default version analyzer', tags: ['analyzer', 'default', 'semver'] },
  { repo: 'generator-changelog-md', name: 'changelog-md', category: 'generator', description: 'Markdown changelog generator', tags: ['changelog', 'markdown', 'generator'] },
  { repo: 'generator-changelog-html', name: 'changelog-html', category: 'generator', description: 'HTML changelog generator', tags: ['changelog', 'html', 'generator'] },
  { repo: 'generator-release-notes', name: 'release-notes', category: 'generator', description: 'Release notes generator', tags: ['release-notes', 'markdown', 'generator'] },
  { repo: 'condition-github-actions', name: 'github-actions', category: 'condition', description: 'GitHub Actions condition', tags: ['condition', 'github-actions', 'ci'] },
  { repo: 'condition-generic', name: 'generic', category: 'condition', description: 'Generic external condition', tags: ['condition', 'generic', 'workflow'] },
  { repo: 'condition-gitea-actions', name: 'gitea-actions', category: 'condition', description: 'Gitea Actions condition', tags: ['condition', 'gitea-actions', 'ci'] },
  { repo: 'condition-gitlab-ci', name: 'gitlab-ci', category: 'condition', description: 'GitLab CI condition', tags: ['condition', 'gitlab-ci', 'ci'] },
  { repo: 'hook-slack', name: 'slack', category: 'hook', description: 'Slack notification hook', tags: ['hook', 'notifications', 'slack'] },
  { repo: 'hook-email', name: 'email', category: 'hook', description: 'Email notification hook', tags: ['email', 'hook', 'notifications'] },
  { repo: 'hook-matrix', name: 'matrix', category: 'hook', description: 'Matrix notification hook', tags: ['hook', 'matrix', 'notifications'] },
  { repo: 'hook-jira', name: 'jira', category: 'hook', description: 'Jira integration hook', tags: ['hook', 'integration', 'jira'] },
  { repo: 'hook-teams', name: 'teams', category: 'hook', description: 'Microsoft Teams notification hook', tags: ['hook', 'notifications', 'teams'] },
  { repo: 'hook-gitplugin', name: 'gitplugin', category: 'hook', description: 'Git plugin execution hook', tags: ['git', 'hook', 'plugin'] },
  { repo: 'updater-go', name: 'go', category: 'updater', description: 'Go module updater', tags: ['go', 'modules', 'updater'] },
  { repo: 'updater-npm', name: 'npm', category: 'updater', description: 'npm package updater', tags: ['npm', 'packages', 'updater'] },
  { repo: 'updater-docker', name: 'docker', category: 'updater', description: 'Docker image updater', tags: ['docker', 'images', 'updater'] },
  { repo: 'updater-gradle', name: 'gradle', category: 'updater', description: 'Gradle dependency updater', tags: ['dependencies', 'gradle', 'updater'] },
  { repo: 'updater-python', name: 'python', category: 'updater', description: 'Python dependency updater', tags: ['python', 'dependencies', 'updater'] },
  { repo: 'updater-helm', name: 'helm', category: 'updater', description: 'Helm chart updater', tags: ['helm', 'charts', 'updater'] },
  { repo: 'updater-cargo', name: 'cargo', category: 'updater', description: 'Cargo dependency updater', tags: ['cargo', 'dependencies', 'updater'] },
  { repo: 'updater-maven', name: 'maven', category: 'updater', description: 'Maven dependency updater', tags: ['dependencies', 'maven', 'updater'] },
  { repo: 'updater-nuget', name: 'nuget', category: 'updater', description: 'NuGet package updater', tags: ['nuget', 'packages', 'updater'] },
  { repo: 'updater-homebrew', name: 'homebrew', category: 'updater', description: 'Homebrew formula updater', tags: ['formula', 'homebrew', 'updater'] },
  { repo: 'updater-terraform', name: 'terraform', category: 'updater', description: 'Terraform module updater', tags: ['terraform', 'modules', 'updater'] }
];
const SEMVER_SOURCE = '(0|[1-9]\\d*)\\.(0|[1-9]\\d*)\\.(0|[1-9]\\d*)(?:-((?:0|[1-9]\\d*|\\d*[A-Za-z-][0-9A-Za-z-]*)(?:\\.(?:0|[1-9]\\d*|\\d*[A-Za-z-][0-9A-Za-z-]*))*))?(?:\\+([0-9A-Za-z-]+(?:\\.[0-9A-Za-z-]+)*))?';

function parseArgs(argv) {
  const args = {};
  for (let index = 2; index < argv.length; index += 1) {
    const current = argv[index];
    if (!current.startsWith('--')) {
      continue;
    }

    const next = argv[index + 1];
    args[current.slice(2)] = next && !next.startsWith('--') ? next : true;
    if (next && !next.startsWith('--')) {
      index += 1;
    }
  }
  return args;
}

function readJson(filePath, fallback) {
  try {
    return JSON.parse(fs.readFileSync(filePath, 'utf8'));
  } catch {
    return fallback;
  }
}

function writeJson(filePath, value) {
  fs.mkdirSync(path.dirname(filePath), { recursive: true });
  fs.writeFileSync(filePath, JSON.stringify(value, null, 2) + '\n');
}

function escapeWorkflowMessage(message) {
  return String(message)
    .replace(/%/g, '%25')
    .replace(/\r/g, '%0D')
    .replace(/\n/g, '%0A');
}

function warning(summary, message) {
  summary.warnings.push(message);
  process.stdout.write(`::warning::${escapeWorkflowMessage(message)}\n`);
}

function escapeRegExp(value) {
  return String(value).replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

async function fetchWithRetry(url, options, retries, summary, label) {
  let lastError = null;

  for (let attempt = 1; attempt <= retries; attempt += 1) {
    try {
      const response = await fetch(url, options);
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }
      return response;
    } catch (error) {
      lastError = error;
      warning(summary, `${label} failed on attempt ${attempt}/${retries}: ${error.message}`);
      await new Promise((resolve) => setTimeout(resolve, attempt * 1000));
    }
  }

  throw lastError;
}

async function fetchJson(url, token, summary, label) {
  const response = await fetchWithRetry(
    url,
    {
      headers: {
        Accept: 'application/vnd.github+json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        'User-Agent': 'semrel-registry-sync'
      }
    },
    3,
    summary,
    label
  );

  return response.json();
}

async function fetchText(url, token, summary, label) {
  const response = await fetchWithRetry(
    url,
    {
      headers: {
        Accept: 'application/octet-stream',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        'User-Agent': 'semrel-registry-sync'
      }
    },
    3,
    summary,
    label
  );

  return response.text();
}

async function fetchRepoMetadata(owner, repo, token, summary) {
  return fetchJson(
    `https://api.github.com/repos/${owner}/${repo}`,
    token,
    summary,
    `Repository metadata fetch for ${owner}/${repo}`
  );
}

async function fetchReleases(owner, repo, token, summary) {
  const releases = [];

  for (let page = 1; page <= 10; page += 1) {
    const pageData = await fetchJson(
      `https://api.github.com/repos/${owner}/${repo}/releases?per_page=100&page=${page}`,
      token,
      summary,
      `Release fetch for ${owner}/${repo} page ${page}`
    ).catch((error) => {
      warning(summary, `Skipping ${owner}/${repo} because releases could not be fetched: ${error.message}`);
      return null;
    });

    if (!Array.isArray(pageData)) {
      break;
    }

    releases.push(...pageData);
    if (pageData.length < 100) {
      break;
    }
  }

  return releases;
}

function extractChecksum(content) {
  const match = String(content || '').match(/[A-Fa-f0-9]{64}/);
  return match ? match[0].toLowerCase() : null;
}

function detectPlatform(assetName, pluginName) {
  const escapedName = escapeRegExp(pluginName);
  const patterns = [
    new RegExp(`^semrel-plugin-${escapedName}_(linux|darwin|windows)_(amd64|arm64)(?:\\.exe)?$`),
    new RegExp(`^plugin-(linux|darwin|windows)-(amd64|arm64)(?:\\.exe)?$`)
  ];

  for (const pattern of patterns) {
    const match = assetName.match(pattern);
    if (match) {
      return `${match[1]}_${match[2]}`;
    }
  }

  return null;
}

async function resolveChecksums(release, pluginName, token, summary) {
  const binaryUrls = {};
  const checksumAssets = {};

  for (const asset of release.assets || []) {
    const name = asset.name || '';
    if (name.endsWith('.sha256')) {
      const platform = detectPlatform(name.slice(0, -7), pluginName);
      if (platform) {
        checksumAssets[platform] = asset;
      }
      continue;
    }

    const platform = detectPlatform(name, pluginName);
    if (platform) {
      binaryUrls[platform] = asset.browser_download_url;
    }
  }

  for (const key of REQUIRED_CHECKSUM_KEYS) {
    if (!binaryUrls[key]) {
      return { error: `Missing binary asset for ${pluginName} ${release.tag_name} (${key}).` };
    }
    if (!checksumAssets[key]) {
      return { error: `Missing checksum asset for ${pluginName} ${release.tag_name} (${key}).` };
    }
  }

  const checksums = {};
  for (const key of REQUIRED_CHECKSUM_KEYS) {
    const checksumAsset = checksumAssets[key];
    const checksum = extractChecksum(
      await fetchText(
        checksumAsset.browser_download_url || checksumAsset.url,
        token,
        summary,
        `Checksum download for ${pluginName} ${release.tag_name} ${key}`
      ).catch((error) => {
        throw new Error(`Could not read ${checksumAsset.name}: ${error.message}`);
      })
    );

    if (!checksum) {
      return { error: `Checksum asset ${checksumAsset.name} did not contain a SHA-256 value.` };
    }

    checksums[key] = checksum;
  }

  return { checksums, binaryUrls };
}

function parseVersionFromTag(tag, repo, pluginName) {
  const candidates = [
    new RegExp(`^v(${SEMVER_SOURCE})$`),
    new RegExp(`^${escapeRegExp(repo)}-v(${SEMVER_SOURCE})$`),
    new RegExp(`^${escapeRegExp(pluginName)}-v(${SEMVER_SOURCE})$`)
  ];

  for (const candidate of candidates) {
    const match = String(tag || '').match(candidate);
    if (match) {
      return match[1];
    }
  }

  return null;
}

function buildPluginEntry(spec, owner, repoMetadata, existingPlugin) {
  const repository = repoMetadata?.html_url || `https://github.com/${owner}/${spec.repo}`;
  return {
    name: spec.name,
    description: spec.description,
    author: existingPlugin?.author || 'semrel Authors',
    license: existingPlugin?.license || (repoMetadata?.license?.spdx_id && repoMetadata.license.spdx_id !== 'NOASSERTION' ? repoMetadata.license.spdx_id : 'Apache-2.0'),
    category: spec.category,
    repository,
    tags: Array.isArray(existingPlugin?.tags) && existingPlugin.tags.length > 0 ? existingPlugin.tags : spec.tags,
    versions: Array.isArray(existingPlugin?.versions) ? existingPlugin.versions : []
  };
}

function samePluginData(previousRegistry, nextPlugins) {
  const previousPlugins = Array.isArray(previousRegistry?.plugins) ? previousRegistry.plugins : [];
  return JSON.stringify(previousPlugins) === JSON.stringify(nextPlugins);
}

function comparePlugin(left, right) {
  const leftCategory = CATEGORY_ORDER.indexOf(left.category);
  const rightCategory = CATEGORY_ORDER.indexOf(right.category);
  if (leftCategory !== rightCategory) {
    return leftCategory - rightCategory;
  }
  return left.name.localeCompare(right.name);
}

async function main() {
  const args = parseArgs(process.argv);
  const pluginsPath = path.resolve(args.plugins || 'plugins.json');
  const summaryPath = path.resolve(args.summary || '.github/.cache/sync-summary.json');
  const schemaPath = path.resolve(args.schema || 'schemas/plugin-metadata.json');
  const owner = process.env.PLUGIN_SOURCE_OWNER || 'SemRels';
  const token = process.env.GITHUB_TOKEN || process.env.GH_TOKEN || '';
  const generatedAtOverride = process.env.PLUGIN_GENERATED_AT || '';

  const summary = {
    sourceOrganization: owner,
    processedRepositories: 0,
    processedReleases: 0,
    syncedPlugins: 0,
    newVersions: [],
    skippedReleases: [],
    warnings: []
  };

  if (!fs.existsSync(schemaPath)) {
    warning(summary, `Schema file not found at ${schemaPath}. Validation will use built-in checks only.`);
  }

  const registry = readJson(pluginsPath, { schemaVersion: 1, generatedAt: null, plugins: [] });
  const pluginsMap = new Map();
  for (const plugin of Array.isArray(registry.plugins) ? registry.plugins : []) {
    if (plugin && typeof plugin.name === 'string') {
      pluginsMap.set(plugin.name, JSON.parse(JSON.stringify(plugin)));
    }
  }

  for (const spec of OFFICIAL_PLUGINS) {
    summary.processedRepositories += 1;
    const existingPlugin = pluginsMap.get(spec.name);
    const repoMetadata = await fetchRepoMetadata(owner, spec.repo, token, summary).catch(() => ({}));
    const plugin = buildPluginEntry(spec, owner, repoMetadata, existingPlugin);
    const versionsByNumber = new Map((plugin.versions || []).map((version) => [version.version, version]));

    const releases = await fetchReleases(owner, spec.repo, token, summary);
    for (const release of releases) {
      summary.processedReleases += 1;

      if (release?.draft) {
        summary.skippedReleases.push(`${spec.repo}:${release.tag_name || 'unknown-tag'} (draft)`);
        continue;
      }

      const version = parseVersionFromTag(release?.tag_name, spec.repo, spec.name);
      if (!version) {
        summary.skippedReleases.push(`${spec.repo}:${release?.tag_name || 'unknown-tag'} (invalid tag pattern)`);
        continue;
      }

      const checksumResult = await resolveChecksums(release, spec.name, token, summary).catch((error) => ({
        error: error.message
      }));
      if (checksumResult.error) {
        warning(summary, checksumResult.error);
        summary.skippedReleases.push(`${spec.repo}:${release.tag_name} (${checksumResult.error})`);
        continue;
      }

      const versionEntry = {
        version,
        releaseDate: release.published_at || release.created_at || new Date().toISOString(),
        downloadUrl: checksumResult.binaryUrls.linux_amd64 || Object.values(checksumResult.binaryUrls)[0] || release.html_url,
        checksums: checksumResult.checksums,
        prerelease: Boolean(release.prerelease)
      };

      const changelog = String(release.body || '').trim();
      if (changelog) {
        versionEntry.changelog = changelog;
      }

      if (!versionsByNumber.has(version)) {
        summary.newVersions.push(`${spec.name}@${version}`);
      }
      versionsByNumber.set(version, versionEntry);
    }

    plugin.versions = sortVersionsDescending(Array.from(versionsByNumber.values()));
    const validationErrors = validatePlugin(plugin, spec.name);
    if (validationErrors.length > 0) {
      warning(summary, `Skipping ${spec.name} because validation failed: ${validationErrors.join(' | ')}`);
      continue;
    }

    pluginsMap.set(spec.name, plugin);
  }

  const finalPlugins = OFFICIAL_PLUGINS
    .map((spec) => pluginsMap.get(spec.name))
    .filter(Boolean)
    .sort(comparePlugin);

  summary.syncedPlugins = finalPlugins.length;
  const nextRegistry = {
    schemaVersion: 1,
    generatedAt: samePluginData(registry, finalPlugins)
      ? (registry.generatedAt === undefined ? null : registry.generatedAt)
      : (generatedAtOverride || new Date().toISOString()),
    plugins: finalPlugins
  };

  writeJson(pluginsPath, nextRegistry);
  writeJson(summaryPath, summary);
  process.stdout.write(`Synced ${summary.syncedPlugins} plugins and ${summary.newVersions.length} versions.\n`);
}

main().catch((error) => {
  process.stdout.write(`::warning::${escapeWorkflowMessage(`Plugin sync terminated early: ${error.message}`)}\n`);
  process.exitCode = 0;
});