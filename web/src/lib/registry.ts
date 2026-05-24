import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

export type PluginCategory =
  | 'provider'
  | 'ci-condition'
  | 'commit-analyzer'
  | 'changelog-generator'
  | 'files-updater'
  | 'hooks';

export type PluginVersion = {
  version: string;
  releaseDate: string;
  downloadUrl: string;
  changelog?: string;
  prerelease?: boolean;
  compatibility?: {
    minSemrelVersion?: string;
    gRPCVersion?: string;
  };
  checksums: Record<string, string>;
};

export type Plugin = {
  name: string;
  description: string;
  author: string;
  homepage?: string;
  repository?: string;
  license: string;
  category: PluginCategory;
  tags?: string[];
  versions: PluginVersion[];
};

export type RegistryPayload = {
  schemaVersion: number;
  generatedAt: string | null;
  plugins: Plugin[];
};

export const CATEGORY_LABELS: Record<PluginCategory, string> = {
  provider: 'Provider',
  'ci-condition': 'CI Condition',
  'commit-analyzer': 'Commit Analyzer',
  'changelog-generator': 'Changelog Generator',
  'files-updater': 'Files Updater',
  hooks: 'Hooks',
};

const registryPath = resolve(process.cwd(), '..', 'plugins.json');

export function loadRegistry(): RegistryPayload {
  const raw = readFileSync(registryPath, 'utf-8');
  const parsed = JSON.parse(raw) as Partial<RegistryPayload>;

  return {
    schemaVersion: Number(parsed.schemaVersion ?? 1),
    generatedAt: parsed.generatedAt ?? null,
    plugins: Array.isArray(parsed.plugins) ? parsed.plugins : [],
  };
}

function compareIdentifiers(left: string, right: string): number {
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

function compareSemver(left: string, right: string): number {
  const [leftMain, leftPre = ''] = left.split('-', 2);
  const [rightMain, rightPre = ''] = right.split('-', 2);
  const leftMainParts = leftMain.split('.').map((part) => Number(part));
  const rightMainParts = rightMain.split('.').map((part) => Number(part));

  for (let index = 0; index < Math.max(leftMainParts.length, rightMainParts.length); index += 1) {
    const difference = (leftMainParts[index] ?? 0) - (rightMainParts[index] ?? 0);

    if (difference !== 0) {
      return difference;
    }
  }

  if (!leftPre && !rightPre) {
    return 0;
  }

  if (!leftPre) {
    return 1;
  }

  if (!rightPre) {
    return -1;
  }

  const leftPreParts = leftPre.split('.');
  const rightPreParts = rightPre.split('.');

  for (let index = 0; index < Math.max(leftPreParts.length, rightPreParts.length); index += 1) {
    const leftPart = leftPreParts[index];
    const rightPart = rightPreParts[index];

    if (leftPart === undefined) {
      return -1;
    }

    if (rightPart === undefined) {
      return 1;
    }

    const difference = compareIdentifiers(leftPart, rightPart);

    if (difference !== 0) {
      return difference;
    }
  }

  return 0;
}

export function getSortedVersions(plugin: Plugin): PluginVersion[] {
  return [...plugin.versions].sort((left, right) => {
    const dateDifference = new Date(right.releaseDate).getTime() - new Date(left.releaseDate).getTime();

    if (!Number.isNaN(dateDifference) && dateDifference !== 0) {
      return dateDifference;
    }

    return compareSemver(right.version, left.version);
  });
}

export function getLatestVersion(plugin: Plugin): PluginVersion | undefined {
  return getSortedVersions(plugin)[0];
}

export function getCategories(plugins: Plugin[]): PluginCategory[] {
  return [...new Set(plugins.map((plugin) => plugin.category))].sort() as PluginCategory[];
}

export function getCategoryLabel(category: PluginCategory): string {
  return CATEGORY_LABELS[category] ?? category;
}

export function getReleaseChannel(plugin: Plugin): 'stable' | 'prerelease' {
  return getLatestVersion(plugin)?.prerelease ? 'prerelease' : 'stable';
}

export function formatReleaseDate(releaseDate?: string): string {
  if (!releaseDate) {
    return 'TBD';
  }

  const date = new Date(releaseDate);

  if (Number.isNaN(date.getTime())) {
    return releaseDate;
  }

  return new Intl.DateTimeFormat('en', { dateStyle: 'medium' }).format(date);
}
