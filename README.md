# go-semrel-registry

Central registry for go-semrel plugins.

## Related repositories

- [go-semrel](https://github.com/SemRels/semrel) - the core release tool
- [go-semrel-plugins](https://github.com/SemRels/semrel-plugins) - plugin SDK and official plugins
- [go-semrel-docs](https://github.com/SemRels/semrel-docs) - documentation site

## How the registry works

The registry is the canonical source for published go-semrel plugins.

1. Plugin authors publish versioned releases from their plugin repository.
2. Plugin metadata is submitted to this repository.
3. CI validates submitted metadata against the registry schema.
4. CI rebuilds `plugins.json` and publishes it from the repository root.
5. Consumers can fetch the index via GitHub Pages today and `registry.semrel.io` later.

`plugins.json` is intentionally kept in the repository root so it can be served directly by GitHub Pages.

## For plugin authors

See the [release guide](docs/release-guide.md) for the expected publish flow.

## For contributors

See the [contributing guide](CONTRIBUTING.md) for contribution rules and review expectations.

## Repository layout

- `schemas/` - JSON schemas for registry payloads
- `docs/` - contributor, API, and publishing documentation
- `.github/workflows/` - automation for validation and registry synchronization
- `plugins.json` - generated registry index served via GitHub Pages
