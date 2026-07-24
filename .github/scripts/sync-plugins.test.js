// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

const assert = require('node:assert/strict');
const test = require('node:test');
const { OFFICIAL_PLUGINS, buildPluginEntry } = require('./sync-plugins');

test('bare Docker catalog name remains exclusive to updater-docker', () => {
  const updater = OFFICIAL_PLUGINS.find((plugin) => plugin.repo === 'updater-docker');
  const publisher = OFFICIAL_PLUGINS.find((plugin) => plugin.repo === 'publisher-docker');

  assert.ok(updater);
  assert.ok(publisher);
  assert.equal(buildPluginEntry(updater, 'SemRels', null, null).name, 'docker');
  assert.equal(buildPluginEntry(publisher, 'SemRels', null, null).name, 'publisher-docker');
  assert.deepEqual(
    OFFICIAL_PLUGINS.filter((plugin) => plugin.name === 'docker').map((plugin) => plugin.repo),
    ['updater-docker']
  );
});
