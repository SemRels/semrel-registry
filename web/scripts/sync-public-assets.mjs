import { copyFileSync, existsSync, lstatSync, mkdirSync, readlinkSync, rmSync, symlinkSync } from 'node:fs';
import { dirname, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const scriptDir = dirname(fileURLToPath(import.meta.url));
const projectRoot = resolve(scriptDir, '..');
const source = resolve(projectRoot, '..', 'plugins.json');
const targetDir = resolve(projectRoot, 'public');
const target = resolve(targetDir, 'plugins.json');

mkdirSync(targetDir, { recursive: true });

if (existsSync(target)) {
  const stats = lstatSync(target);

  if (stats.isSymbolicLink()) {
    const linkedPath = resolve(targetDir, readlinkSync(target));

    if (linkedPath === source) {
      console.log('public/plugins.json already points to ../plugins.json');
      process.exit(0);
    }
  }

  rmSync(target, { force: true });
}

try {
  symlinkSync(relative(targetDir, source), target, 'file');
  console.log('Linked public/plugins.json to ../plugins.json');
} catch (error) {
  copyFileSync(source, target);
  console.warn(`Symlink unavailable, copied plugins.json instead (${error instanceof Error ? error.message : 'unknown error'}).`);
}
