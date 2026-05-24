const fs = require('fs');
const content = fs.readFileSync('.github/.cache/releases.json', 'utf8');

console.log('First 500 chars:');
console.log(content.substring(0, 500));

const releases = JSON.parse(content);
console.log('\nParsed type:', Array.isArray(releases) ? 'Array' : typeof releases);
console.log('Length:', Array.isArray(releases) ? releases.length : 1);

if (Array.isArray(releases)) {
  releases.forEach((r, i) => {
    console.log(`[${i}] tag_name: ${r.tag_name}`);
  });
} else {
  console.log('Root tag_name:', releases.tag_name);
}
