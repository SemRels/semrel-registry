#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const { validateRegistryDocument } = require('./registry-utils');

function main() {
  const pluginsPath = path.resolve(process.argv[2] || 'plugins.json');
  const schemaPath = path.resolve(process.argv[3] || 'schemas/plugin-metadata.json');

  if (!fs.existsSync(pluginsPath)) {
    console.error(`plugins.json not found at ${pluginsPath}`);
    process.exit(1);
  }

  if (!fs.existsSync(schemaPath)) {
    console.error(`Schema not found at ${schemaPath}`);
    process.exit(1);
  }

  JSON.parse(fs.readFileSync(schemaPath, 'utf8'));
  const document = JSON.parse(fs.readFileSync(pluginsPath, 'utf8'));
  const errors = validateRegistryDocument(document);

  if (errors.length > 0) {
    for (const error of errors) {
      console.error(`- ${error}`);
    }
    process.exit(1);
  }

  console.log(`Validated ${Array.isArray(document.plugins) ? document.plugins.length : 0} plugin entries.`);
}

main();
