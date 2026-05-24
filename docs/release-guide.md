# Plugin release guide

This guide outlines the expected publish flow for go-semrel plugin authors.

## 1. Prepare your plugin repository

- keep the plugin code in its own repository
- document installation and usage
- ensure releases are versioned and reproducible

## 2. Create a release

- tag a new version in your plugin repository
- publish release notes describing compatibility and changes
- verify release artifacts, if any, are available

## 3. Prepare registry metadata

- collect the plugin name, repository URL, version, and compatibility details
- format the metadata according to the registry schema once it is available
- validate the metadata locally before opening a PR

## 4. Submit to the registry

- open a pull request against `semrel-registry`
- include links to the plugin repository and release
- mention any breaking changes or runtime requirements

## 5. Verify publication

After the pull request is merged, confirm that your plugin appears in `plugins.json` and is reachable through the published registry endpoint.

## Need help?

Start with the [submission overview](contribute.md) and the repository-wide [contributing guide](../CONTRIBUTING.md).
