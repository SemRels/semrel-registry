#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const {
  REQUIRED_CHECKSUM_KEYS,
  deriveCategory,
  buildDescription,
  buildTags,
  sortVersionsDescending,
  validatePlugin
} = require('./registry-utils');

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
  } catch (error) {
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

async function fetchText(url, token, summary, label) {
  const response = await fetchWithRetry(
    url,
    {
      headers: {
        Accept: 'application/octet-stream',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        'User-Agent': 'go-semrel-registry-sync'
      }
    },
    3,
    summary,
    label
  );

  return response.text();
}

async function fetchRepoMetadata(owner, repo, token, summary) {
  const response = await fetchWithRetry(
    `https://api.github.com/repos/${owner}/${repo}`,
    {
      headers: {
        Accept: 'application/vnd.github+json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        'User-Agent': 'go-semrel-registry-sync'
      }
    },
    3,
    summary,
    `Repository metadata fetch for ${owner}/${repo}`
  );

  return response.json();
}

function extractChecksum(content) {
  const match = String(content || '').match(/[A-Fa-f0-9]{64}/);
  return match ? match[0].toLowerCase() : null;
}

function detectPlatform(assetName, pluginName) {
  const escaped = pluginName.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const match = assetName.match(new RegExp(`^${escaped}-(linux|darwin|windows)-(amd64|arm64)(?:\\.exe)?$`));
  if (!match) {
    return null;
  }

  return `${match[1]}_${match[2]}`;
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
    const rawContent = typeof checksumAsset.content === 'string'
      ? checksumAsset.content
      : await fetchText(
          checksumAsset.browser_download_url || checksumAsset.url,
          token,
          summary,
          `Checksum download for ${pluginName} ${release.tag_name} ${key}`
        );

    const checksum = extractChecksum(rawContent);
    if (!checksum) {
      return { error: `Checksum asset ${checksumAsset.name} did not contain a SHA-256 value.` };
    }

    checksums[key] = checksum;
  }

  return { checksums, binaryUrls };
}

function buildPluginShell(pluginName, repoMetadata, defaults) {
  const category = deriveCategory(pluginName);
  if (!category) {
    return { error: `Could not derive category from plugin name ${pluginName}.` };
  }

  return {
    name: pluginName,
    description: buildDescription(pluginName, category),
    author: repoMetadata.owner?.login || defaults.owner,
    homepage: repoMetadata.homepage || repoMetadata.html_url || defaults.repositoryUrl,
    repository: repoMetadata.html_url || defaults.repositoryUrl,
    license: repoMetadata.license?.spdx_id && repoMetadata.license.spdx_id !== 'NOASSERTION'
      ? repoMetadata.license.spdx_id
      : defaults.license,
    category,
    tags: buildTags(pluginName, category),
    versions: []
  };
}

function mergePlugin(existingPlugin, shellPlugin) {
  if (!existingPlugin) {
    return shellPlugin;
  }

  return {
    ...existingPlugin,
    description: existingPlugin.description || shellPlugin.description,
    author: existingPlugin.author || shellPlugin.author,
    homepage: existingPlugin.homepage || shellPlugin.homepage,
    repository: existingPlugin.repository || shellPlugin.repository,
    license: existingPlugin.license || shellPlugin.license,
    category: existingPlugin.category || shellPlugin.category,
    tags: Array.isArray(existingPlugin.tags) && existingPlugin.tags.length > 0 ? existingPlugin.tags : shellPlugin.tags,
    versions: Array.isArray(existingPlugin.versions) ? existingPlugin.versions : []
  };
}

async function main() {
  const args = parseArgs(process.argv);
  const releasesPath = path.resolve(args.releases || '.github/.cache/releases.json');
  const pluginsPath = path.resolve(args.plugins || 'plugins.json');
  const summaryPath = path.resolve(args.summary || '.github/.cache/sync-summary.json');
  const schemaPath = path.resolve(args.schema || 'schemas/plugin-metadata.json');
  const repoMetadataPath = args['repo-metadata'] ? path.resolve(args['repo-metadata']) : null;

  const owner = process.env.PLUGIN_SOURCE_OWNER || 'SemRels';
  const repo = process.env.PLUGIN_SOURCE_REPO || 'go-semrel-plugins';
  const token = process.env.GITHUB_TOKEN || process.env.GH_TOKEN || '';

  const summary = {
    sourceRepository: `${owner}/${repo}`,
    processedReleases: 0,
    syncedPlugins: 0,
    newVersions: [],
    skippedReleases: [],
    warnings: []
  };

  if (!fs.existsSync(schemaPath)) {
    warning(summary, `Schema file not found at ${schemaPath}. Validation will use built-in checks only.`);
  }

  const releases = readJson(releasesPath, []);
  const registry = readJson(pluginsPath, { schemaVersion: 1, generatedAt: null, plugins: [] });
  const repoMetadata = repoMetadataPath
    ? readJson(repoMetadataPath, {})
    : await fetchRepoMetadata(owner, repo, token, summary).catch(() => ({}));

  if (!Array.isArray(releases)) {
    warning(summary, `Release payload at ${releasesPath} is not an array. Nothing to sync.`);
    writeJson(summaryPath, summary);
    return;
  }

  const pluginsMap = new Map();
  for (const plugin of Array.isArray(registry.plugins) ? registry.plugins : []) {
    if (plugin && typeof plugin.name === 'string') {
      pluginsMap.set(plugin.name, JSON.parse(JSON.stringify(plugin)));
    }
  }

  const defaults = {
    owner,
    repositoryUrl: `https://github.com/${owner}/${repo}`,
    license: 'NOASSERTION'
  };

  const releasePattern = /^([a-z0-9]+(?:[a-z0-9-]*[a-z0-9])?)-v((0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*))*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?)$/;

  for (const release of releases) {
    summary.processedReleases += 1;

    if (release?.draft) {
      summary.skippedReleases.push(`${release.tag_name || 'unknown-tag'} (draft)`);
      continue;
    }

    const tag = release?.tag_name || '';
    const match = tag.match(releasePattern);
    if (!match) {
      summary.skippedReleases.push(`${tag || 'unknown-tag'} (invalid tag pattern)`);
      continue;
    }

    const pluginName = match[1];
    const version = match[2];
    const shellPlugin = buildPluginShell(pluginName, repoMetadata, defaults);
    if (shellPlugin.error) {
      warning(summary, shellPlugin.error);
      summary.skippedReleases.push(`${tag} (${shellPlugin.error})`);
      continue;
    }

    const checksumResult = await resolveChecksums(release, pluginName, token, summary).catch((error) => ({
      error: error.message
    }));

    if (checksumResult.error) {
      warning(summary, checksumResult.error);
      summary.skippedReleases.push(`${tag} (${checksumResult.error})`);
      continue;
    }

    const plugin = mergePlugin(pluginsMap.get(pluginName), shellPlugin);
    const versionEntry = {
      version,
      releaseDate: release.published_at || release.created_at || new Date().toISOString(),
      downloadUrl: release.html_url || checksumResult.binaryUrls.linux_amd64,
      checksums: checksumResult.checksums,
      prerelease: Boolean(release.prerelease)
    };

    const changelog = String(release.body || '').trim();
    if (changelog) {
      versionEntry.changelog = changelog;
    }

    const versionsByNumber = new Map();
    for (const existingVersion of plugin.versions || []) {
      versionsByNumber.set(existingVersion.version, existingVersion);
    }

    if (!versionsByNumber.has(version)) {
      summary.newVersions.push(`${pluginName}@${version}`);
    }

    versionsByNumber.set(version, versionEntry);
    plugin.versions = sortVersionsDescending(Array.from(versionsByNumber.values()));

    const validationErrors = validatePlugin(plugin, pluginName);
    if (validationErrors.length > 0) {
      warning(summary, `Skipping ${pluginName} because validation failed: ${validationErrors.join(' | ')}`);
      summary.skippedReleases.push(`${tag} (validation failed)`);
      continue;
    }

    pluginsMap.set(pluginName, plugin);
  }

  const finalPlugins = Array.from(pluginsMap.values())
    .filter((plugin) => validatePlugin(plugin, plugin.name).length === 0)
    .sort((left, right) => left.name.localeCompare(right.name));

  summary.syncedPlugins = finalPlugins.length;

  const nextRegistry = {
    schemaVersion: 1,
    generatedAt: new Date().toISOString(),
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
