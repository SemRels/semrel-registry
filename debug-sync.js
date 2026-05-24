const fs = require('fs');
const releases = JSON.parse(fs.readFileSync('.github/.cache/releases.json', 'utf8'));

const releasePattern = /^([a-z0-9]+(?:[a-z0-9-]*[a-z0-9])?)-v((0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*))*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?)$/;

console.log('Found releases:', releases.length);
releases.forEach(release => {
  const tag = release.tag_name;
  const match = tag.match(releasePattern);
  console.log(`\nTag: ${tag}`);
  console.log(`Match: ${match ? 'YES' : 'NO'}`);
  if (match) {
    console.log(`Plugin: ${match[1]}, Version: ${match[2]}`);
  }
  console.log(`Draft: ${release.draft}`);
  console.log(`Assets: ${release.assets.length}`);
  release.assets.forEach(asset => {
    console.log(`  - ${asset.name}`);
  });
});
