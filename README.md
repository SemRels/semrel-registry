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

Supported plugin categories in the registry are currently `provider`, `analyzer`, `generator`, `condition`, `hook`, `updater`, plus parity-foundation categories `packager` and `publisher`.

## For plugin authors

See the [registry API docs](https://semrel.io/api/registry) for the full endpoint reference.

## For contributors

See the [contributing guide](CONTRIBUTING.md) for contribution rules and review expectations.

## Repository layout

- `api/` - Go web service skeleton for the upcoming dynamic registry backend
- `admin/` - Nginx-served SPA (admin UI) that proxies `/schemas/` and `/api/` to the API container
- `api/handlers/schemas/` - embedded core and canonical first-party plugin configuration schemas served at `/schemas/`
- `schemas/plugin-metadata.json` - metadata schema used to validate the generated registry index
- `docs/` - contributor, API, and publishing documentation
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

## Admin UI

The `admin/` directory contains an nginx-served SPA that acts as the public entry point for `registry.semrel.io`. It proxies:

- `/schemas/` → API container (serves embedded JSON schemas)
- `/api/` → API container (REST endpoints)
- Everything else → SPA (`index.html`)

### Building the admin container

```bash
docker build -f admin/Dockerfile -t semrel-registry-admin .
```

### Runtime configuration

| Environment variable | Default | Description |
|---|---|---|
| `API_URL` | `http://api:8080` | Origin URL of the Go API as reachable from the admin container. |

The default works only when the admin and API containers share a network where the API has the DNS name `api`. For a separate deployment, set `API_URL` to a reachable internal or public API origin (without a path) and either attach both services to a shared network or provide working DNS and routing. For example, if the API service is named `registry`, set `API_URL=http://registry:8080`. A permanently incorrect hostname continues to return `502`; there is no fallback backend.

The image uses the official nginx entrypoint's local resolver discovery and resolves the API hostname at request time. This lets nginx start before the API DNS record exists and recover after it appears. Only `API_URL` and the discovered resolver list are substituted into the template; nginx request variables remain intact. The image health check verifies that nginx can serve the SPA, not that the API backend is ready.

## Web app development

```bash
cd web
npm install
npm run dev
```

The Astro site runs on `http://localhost:3000`, builds static files into `web/dist`, and mirrors the repository root `plugins.json` into `web/public/plugins.json` during install/build.
