# semrel-registry

Central registry for go-semrel plugins.

## Related repositories

- [go-semrel](https://github.com/SemRels/semrel) - the core release tool
- [go-semrel-plugins](https://github.com/SemRels/semrel-plugins) - plugin SDK and official plugins
- [go-semrel-docs](https://github.com/SemRels/semrel-docs) - documentation site

## How the registry works

The registry is the canonical source for published go-semrel plugins.

1. Plugin authors publish versioned GitHub Releases in [`SemRels/go-semrel-plugins`](https://github.com/SemRels/go-semrel-plugins).
2. Release tags, binary asset names, and checksum files follow the registry naming conventions.
3. GitHub Actions validates release metadata against the registry schema.
4. GitHub Actions rebuilds `plugins.json` and publishes it from the repository root.
5. Consumers can fetch the index via GitHub Pages today and `registry.semrel.io` later.

`plugins.json` is intentionally kept in the repository root so it can be served directly by GitHub Pages.

## For plugin authors

See the [release guide](docs/release-guide.md) for the expected publish flow.

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
