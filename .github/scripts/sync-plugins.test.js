// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

const assert = require('node:assert/strict');
const test = require('node:test');
const { OFFICIAL_PLUGINS, buildPluginEntry } = require('./sync-plugins');

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
