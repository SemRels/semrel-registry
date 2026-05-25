const fs = require('fs');
const releases = JSON.parse(fs.readFileSync('.github/.cache/releases.json', 'utf8'));

console.log('Type:', Array.isArray(releases) ? 'Array' : typeof releases);
if (Array.isArray(releases)) {
  console.log(`Items: ${releases.length}`);
  releases.forEach((r, i) => {
    console.log(`  [${i}] ${r.tag_name || 'NO TAG'}`);
  });
}
