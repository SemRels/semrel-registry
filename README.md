# semrel-registry

Central registry for [semrel](https://github.com/SemRels/semrel) plugins — a Go-based REST API that stores, validates, and serves plugin metadata.

## Related repositories

- [semrel](https://github.com/SemRels/semrel) — the core release tool
- [semrel-plugins](https://github.com/SemRels/semrel-plugins) — official plugin catalog
- [semrel-docs](https://github.com/SemRels/semrel-docs) — documentation site

## How the registry works

The registry is the canonical source for published semrel plugins.

1. Plugin authors publish versioned GitHub Releases in their own repositories.
2. A `repository_dispatch` webhook notifies the registry (`POST /api/v1/webhooks/release`).
3. The registry validates metadata, stores plugin and version records, and updates `plugins.json`.
4. Consumers fetch the index via `GET /plugins.json` or browse individual plugins via the REST API.
5. The `semrel` CLI respects `SEMREL_REGISTRY_URL` to discover plugins from a custom registry.

For update-aware clients, each plugin's `versions` array is the source for version checks; clients are expected to select the highest stable release (`prerelease: false`) as the default update target.

## For plugin authors

See the [registry API docs](https://semrel.io/api/registry) for the full endpoint reference.

## For contributors

See the [contributing guide](CONTRIBUTING.md) for contribution rules and review expectations.

## Repository layout

- `api/` - Go web service skeleton for the upcoming dynamic registry backend
- `schemas/` - JSON schemas for registry payloads
- `docs/` - contributor, API, and publishing documentation
- `web/` - Astro-based landing page and documentation for `registry.semrel.io`
- `.github/workflows/` - automation for validation, synchronization, and web deployment
- `plugins.json` - generated registry index served via GitHub Pages

## Quick start (no database)

The simplest way to run the registry locally is with the **file storage backend** — no Postgres required.

```bash
cp .env.example .env          # set JWT_SECRET and ADMIN_TOKEN
docker compose -f docker-compose.file.yml up -d
```

The registry stores all plugin data as JSON files in a named Docker volume (`registry_data`).  
This is ideal for self-hosting with small to medium plugin catalogues.

> **Choose PostgreSQL** when you need full-text search, concurrent writes, or plan to host more than ~10 000 plugins.

## Web app development

```bash
cd web
npm install
npm run dev
```

The Astro site runs on `http://localhost:3000`, builds static files into `web/dist`, and mirrors the repository root `plugins.json` into `web/public/plugins.json` during install/build.
