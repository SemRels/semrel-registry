const fs = require('fs');
const releases = JSON.parse(fs.readFileSync('.github/.cache/releases.json', 'utf8'));

const release = releases[0];
const pluginName = 'provider-test';

console.log('Release:', release.tag_name);
console.log(`Assets (${release.assets.length}):`);

release.assets.forEach(asset => {
  console.log(`  - ${asset.name}`);
  
  // Test Platform Detection
  const escaped = pluginName.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const binPattern = new RegExp(`^${escaped}-(linux|darwin|windows)-(amd64|arm64)(?:\\.exe)?$`);
  const checksumPattern = /[A-Fa-f0-9]{64}/;
  
  if (asset.name.match(binPattern)) {
    console.log(`    ? Binary matched`);
  } else if (asset.name.endsWith('.sha256')) {
    const noExt = asset.name.slice(0, -7);
    if (noExt.match(binPattern)) {
      console.log(`    ? Checksum for binary`);
    } else {
      console.log(`    ? Checksum for unknown binary`);
    }
  } else {
    console.log(`    ? Not recognized`);
  }
});
