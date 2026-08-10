// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

const fs = require('fs');
const path = require('path');

function parseArgs(argv) {
  const args = {};
  for (let index = 2; index < argv.length; index += 1) {
    const current = argv[index];
    if (!current.startsWith('--')) continue;
    const next = argv[index + 1];
    args[current.slice(2)] = next && !next.startsWith('--') ? next : true;
    if (next && !next.startsWith('--')) index += 1;
  }
  return args;
}

async function fetchJson(url, token) {
  const response = await fetch(url, {
    headers: {
      Accept: 'application/vnd.github+json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      'User-Agent': 'semrel-registry-schema-sync'
    }
  });
  if (!response.ok) throw new Error(`${url}: HTTP ${response.status}`);
  return response.json();
}

async function fetchText(url, token) {
  const response = await fetch(url, {
    headers: {
      Accept: 'application/vnd.github.raw+json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      'User-Agent': 'semrel-registry-schema-sync'
    }
  });
  if (!response.ok) throw new Error(`${url}: HTTP ${response.status}`);
  return response.text();
}

function repositoryParts(repository) {
  const match = String(repository || '').match(/github\.com\/([^/]+)\/([^/#]+)(?:[#/].*)?$/);
  return match ? { owner: match[1], repo: match[2].replace(/\.git$/, '') } : null;
}

function writeSchema(directory, schema) {
  fs.mkdirSync(directory, { recursive: true });
  fs.writeFileSync(path.join(directory, 'v1.json'), `${JSON.stringify(schema, null, 2)}\n`);
  fs.writeFileSync(path.join(directory, 'latest.json'), `${JSON.stringify(schema, null, 2)}\n`);
}

async function syncSchema({ sourceOwner, sourceRepo, sourcePath, outputDir, schemaId }) {
  let schemaText;
  let lastError;
  for (const branch of ['main', 'master']) {
    try {
      schemaText = await fetchText(
        `https://raw.githubusercontent.com/${sourceOwner}/${sourceRepo}/${branch}/${sourcePath}`
      );
      break;
    } catch (error) {
      lastError = error;
    }
  }
  if (!schemaText) throw lastError;
  const schema = JSON.parse(schemaText);
  schema.$id = schemaId;
  writeSchema(outputDir, schema);
}

async function main() {
  const args = parseArgs(process.argv);
  const pluginsPath = path.resolve(args.plugins || 'plugins.json');
  const outputRoot = path.resolve(args['schemas-dir'] || 'api/handlers/schemas');
  const token = process.env.GITHUB_TOKEN;
  const registry = JSON.parse(fs.readFileSync(pluginsPath, 'utf8'));
  const plugins = Array.isArray(registry.plugins) ? registry.plugins : [];

  await syncSchema({
    sourceOwner: args['core-owner'] || 'SemRels',
    sourceRepo: args['core-repo'] || 'semrel',
    sourcePath: args['core-path'] || 'docs/schema/semrel-config.v1.json',
    outputDir: path.join(outputRoot, 'core'),
    schemaId: 'https://registry.semrel.io/schemas/core/v1.json'
  });

  let synced = 0;
  for (const plugin of plugins) {
    const repository = repositoryParts(plugin.repository);
    if (!repository || !plugin.name) {
      console.warn(`Skipping schema for ${plugin.name || 'unknown plugin'}: no GitHub repository URL.`);
      continue;
    }
    try {
      await syncSchema({
        sourceOwner: repository.owner,
        sourceRepo: repository.repo,
        sourcePath: 'schema/v1.json',
        outputDir: path.join(outputRoot, 'plugins', plugin.name),
        schemaId: `https://registry.semrel.io/schemas/plugins/${plugin.name}/v1.json`
      });
      synced += 1;
    } catch (error) {
      console.warn(`Skipping schema for ${plugin.name}: ${error.message}`);
    }
  }

  console.log(`Synced ${synced} plugin schemas and the core schema from source repositories.`);
}

main().catch((error) => {
  console.error(`Schema sync failed: ${error.message}`);
  process.exitCode = 1;
});
