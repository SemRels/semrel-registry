const REQUIRED_CHECKSUM_KEYS = [
  'linux_amd64',
  'linux_arm64',
  'darwin_amd64',
  'darwin_arm64',
  'windows_amd64',
  'windows_arm64'
];

const VALID_CATEGORIES = new Set([
  'provider',
  'ci-condition',
  'commit-analyzer',
  'changelog-generator',
  'files-updater',
  'hooks'
]);

const SEMVER_PATTERN = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*))*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$/;
const PLUGIN_NAME_PATTERN = /^[a-z0-9]+(?:[a-z0-9-]*[a-z0-9])?$/;
const HTTP_URL_PATTERN = /^https?:\/\//;
const SPDX_PATTERN = /^[A-Za-z0-9.-]+(?:\+[A-Za-z0-9.-]+)?$/;
const SHA256_PATTERN = /^[A-Fa-f0-9]{64}$/;

function deriveCategory(pluginName) {
  for (const category of VALID_CATEGORIES) {
    if (pluginName === category || pluginName.startsWith(category + '-')) {
      return category;
    }
  }

  return null;
}

function buildDescription(pluginName, category) {
  const subject = pluginName.startsWith(category + '-')
    ? pluginName.slice(category.length + 1).replace(/-/g, ' ')
    : pluginName.replace(/-/g, ' ');

  return `Official ${category} plugin for ${subject}.`;
}

function buildTags(pluginName, category) {
  const tags = new Set([category, ...pluginName.split('-')]);
  return Array.from(tags);
}

function parseSemver(version) {
  const match = String(version || '').match(SEMVER_PATTERN);
  if (!match) {
    return null;
  }

  return {
    major: Number(match[1]),
    minor: Number(match[2]),
    patch: Number(match[3]),
    prerelease: match[4] ? match[4].split('.') : [],
    build: match[5] || ''
  };
}

function compareIdentifier(left, right) {
  const leftNumeric = /^\d+$/.test(left);
  const rightNumeric = /^\d+$/.test(right);

  if (leftNumeric && rightNumeric) {
    return Number(left) - Number(right);
  }

  if (leftNumeric) {
    return -1;
  }

  if (rightNumeric) {
    return 1;
  }

  return left.localeCompare(right);
}

function compareSemver(left, right) {
  if (!left || !right) {
    return 0;
  }

  for (const key of ['major', 'minor', 'patch']) {
    if (left[key] !== right[key]) {
      return left[key] - right[key];
    }
  }

  const leftPre = left.prerelease;
  const rightPre = right.prerelease;

  if (leftPre.length === 0 && rightPre.length === 0) {
    return 0;
  }

  if (leftPre.length === 0) {
    return 1;
  }

  if (rightPre.length === 0) {
    return -1;
  }

  const length = Math.max(leftPre.length, rightPre.length);
  for (let index = 0; index < length; index += 1) {
    const leftPart = leftPre[index];
    const rightPart = rightPre[index];

    if (leftPart === undefined) {
      return -1;
    }

    if (rightPart === undefined) {
      return 1;
    }

    const comparison = compareIdentifier(leftPart, rightPart);
    if (comparison !== 0) {
      return comparison;
    }
  }

  return 0;
}

function sortVersionsDescending(versions) {
  return [...versions].sort((left, right) => {
    const leftParsed = parseSemver(left.version);
    const rightParsed = parseSemver(right.version);

    if (!leftParsed || !rightParsed) {
      return String(right.version).localeCompare(String(left.version));
    }

    return compareSemver(rightParsed, leftParsed);
  });
}

function isIsoDateTime(value) {
  if (typeof value !== 'string' || value.trim() === '') {
    return false;
  }

  const parsed = Date.parse(value);
  return !Number.isNaN(parsed);
}

function validateVersion(version, pluginName, index) {
  const errors = [];
  const prefix = `plugins[${pluginName}].versions[${index}]`;

  if (!version || typeof version !== 'object' || Array.isArray(version)) {
    errors.push(`${prefix} must be an object.`);
    return errors;
  }

  const parsedVersion = parseSemver(version.version);
  if (!parsedVersion) {
    errors.push(`${prefix}.version must be a valid semver string.`);
  }

  if (!isIsoDateTime(version.releaseDate)) {
    errors.push(`${prefix}.releaseDate must be an ISO-8601 date-time.`);
  }

  if (typeof version.downloadUrl !== 'string' || !HTTP_URL_PATTERN.test(version.downloadUrl)) {
    errors.push(`${prefix}.downloadUrl must be an HTTP(S) URL.`);
  }

  if (version.changelog !== undefined && (typeof version.changelog !== 'string' || version.changelog.trim() === '')) {
    errors.push(`${prefix}.changelog must be a non-empty string when present.`);
  }

  if (version.prerelease !== undefined && typeof version.prerelease !== 'boolean') {
    errors.push(`${prefix}.prerelease must be a boolean when present.`);
  }

  const checksums = version.checksums;
  if (!checksums || typeof checksums !== 'object' || Array.isArray(checksums)) {
    errors.push(`${prefix}.checksums must be an object.`);
    return errors;
  }

  for (const checksumKey of REQUIRED_CHECKSUM_KEYS) {
    if (typeof checksums[checksumKey] !== 'string' || !SHA256_PATTERN.test(checksums[checksumKey])) {
      errors.push(`${prefix}.checksums.${checksumKey} must be a SHA-256 hex string.`);
    }
  }

  return errors;
}

function validatePlugin(plugin, index) {
  const errors = [];
  const pluginLabel = typeof plugin?.name === 'string' ? plugin.name : index;
  const prefix = `plugins[${pluginLabel}]`;

  if (!plugin || typeof plugin !== 'object' || Array.isArray(plugin)) {
    return [`${prefix} must be an object.`];
  }

  if (typeof plugin.name !== 'string' || !PLUGIN_NAME_PATTERN.test(plugin.name)) {
    errors.push(`${prefix}.name must be kebab-case.`);
  }

  if (typeof plugin.description !== 'string' || plugin.description.trim() === '') {
    errors.push(`${prefix}.description must be a non-empty string.`);
  }

  if (typeof plugin.author !== 'string' || plugin.author.trim() === '') {
    errors.push(`${prefix}.author must be a non-empty string.`);
  }

  if (typeof plugin.license !== 'string' || !SPDX_PATTERN.test(plugin.license)) {
    errors.push(`${prefix}.license must be a valid SPDX-like identifier.`);
  }

  if (typeof plugin.category !== 'string' || !VALID_CATEGORIES.has(plugin.category)) {
    errors.push(`${prefix}.category must be one of ${Array.from(VALID_CATEGORIES).join(', ')}.`);
  }

  if (plugin.homepage !== undefined && (typeof plugin.homepage !== 'string' || !HTTP_URL_PATTERN.test(plugin.homepage))) {
    errors.push(`${prefix}.homepage must be an HTTP(S) URL when present.`);
  }

  if (plugin.repository !== undefined && (typeof plugin.repository !== 'string' || !HTTP_URL_PATTERN.test(plugin.repository))) {
    errors.push(`${prefix}.repository must be an HTTP(S) URL when present.`);
  }

  if (plugin.tags !== undefined) {
    if (!Array.isArray(plugin.tags)) {
      errors.push(`${prefix}.tags must be an array when present.`);
    } else {
      const tags = new Set();
      for (const tag of plugin.tags) {
        if (typeof tag !== 'string' || tag.trim() === '') {
          errors.push(`${prefix}.tags entries must be non-empty strings.`);
          break;
        }
        tags.add(tag);
      }
      if (tags.size !== plugin.tags.length) {
        errors.push(`${prefix}.tags must be unique.`);
      }
    }
  }

  if (!Array.isArray(plugin.versions)) {
    errors.push(`${prefix}.versions must be an array.`);
  } else {
    if (plugin.versions.length === 0) {
      errors.push(`${prefix}.versions must not be empty.`);
    }

    const seenVersions = new Set();
    plugin.versions.forEach((version, versionIndex) => {
      for (const error of validateVersion(version, plugin.name || pluginLabel, versionIndex)) {
        errors.push(error);
      }

      if (version?.version) {
        if (seenVersions.has(version.version)) {
          errors.push(`${prefix}.versions contains duplicate version ${version.version}.`);
        }
        seenVersions.add(version.version);
      }
    });
  }

  return errors;
}

function validateRegistryDocument(document) {
  const errors = [];

  if (!document || typeof document !== 'object' || Array.isArray(document)) {
    return ['Registry document must be an object.'];
  }

  if (document.schemaVersion !== undefined && document.schemaVersion !== 1) {
    errors.push('schemaVersion must be 1 when present.');
  }

  if (document.generatedAt !== null && document.generatedAt !== undefined && !isIsoDateTime(document.generatedAt)) {
    errors.push('generatedAt must be null or an ISO-8601 date-time string.');
  }

  if (!Array.isArray(document.plugins)) {
    errors.push('plugins must be an array.');
    return errors;
  }

  document.plugins.forEach((plugin, index) => {
    for (const error of validatePlugin(plugin, index)) {
      errors.push(error);
    }
  });

  return errors;
}

module.exports = {
  REQUIRED_CHECKSUM_KEYS,
  VALID_CATEGORIES,
  SEMVER_PATTERN,
  deriveCategory,
  buildDescription,
  buildTags,
  parseSemver,
  sortVersionsDescending,
  validatePlugin,
  validateRegistryDocument
};
