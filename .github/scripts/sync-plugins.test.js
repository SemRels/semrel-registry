// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

const assert = require('node:assert/strict');
const test = require('node:test');
const { OFFICIAL_PLUGINS, buildPluginEntry } = require('./sync-plugins');
const { validateVersion } = require('./registry-utils');

test('Docker short aliases remain exclusive to updater-docker', () => {
  const updater = OFFICIAL_PLUGINS.find((plugin) => plugin.repo === 'updater-docker');
  const publisher = OFFICIAL_PLUGINS.find((plugin) => plugin.repo === 'publisher-docker');

  assert.ok(updater);
  assert.ok(publisher);
  assert.deepEqual(
    buildPluginEntry(updater, 'SemRels', null, null).aliases,
    ['@semrel/docker', 'docker', 'updater-docker']
  );
  assert.deepEqual(
    buildPluginEntry(publisher, 'SemRels', null, null).aliases,
    ['publisher-docker']
  );
});

test('semrelCore accepts comparator ranges and rejects malformed constraints', () => {
  const base = {
    version: '1.0.0',
    releaseDate: '2026-01-01T00:00:00Z',
    downloadUrl: 'https://example.com/plugin',
    checksums: Object.fromEntries([
      'linux_amd64', 'linux_arm64', 'darwin_amd64', 'darwin_arm64', 'windows_amd64', 'windows_arm64'
    ].map(key => [key, 'a'.repeat(64)])),
    compatibility: { minSemrelVersion: '0.15.0', semrelCore: '>=0.25.0 <1.0.0' }
  };
  assert.deepEqual(validateVersion(base, 'test', 0), []);
  assert.match(validateVersion({ ...base, compatibility: { ...base.compatibility, semrelCore: 'latest' } }, 'test', 0)[0], /semrelCore/);
});
