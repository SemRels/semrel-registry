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
- `api/handlers/schemas/` - embedded core and first-party plugin configuration schemas served at `/schemas/`; the scheduled sync fetches them from the semrel core and plugin repositories
- `schemas/plugin-metadata.json` - metadata schema used to validate the generated registry index
- `docs/` - contributor, API, and publishing documentation
- `.github/workflows/` - automation for validation, synchronization, and web deployment
- `plugins.json` - generated registry index served via GitHub Pages

Version entries may optionally declare `compatibility.semrelCore` as a
space-separated semver range (for example `>=0.25.0 <1.0.0`). Missing metadata
remains valid for backward compatibility with existing plugins and clients.

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
- `/auth/` → API container (GitHub OAuth entry point and callback)
- `/api/` → API container (REST endpoints)
- Everything else → SPA (`index.html`)

The admin login and authenticated plugin-management screens surface Terms, Privacy, and Imprint links. Destructive plugin and version removals are intentionally gated in the SPA with typed confirmations before the existing authenticated delete endpoints are called.

### Targeted admin frontend checks

```bash
cd admin
npm install
npm run test
npm run build
```

### Building the admin container

```bash
docker build -f admin/Dockerfile -t semrel-registry-admin .
```

### Runtime configuration

| Environment variable | Default | Description |
|---|---|---|
| `API_URL` | `http://api:8080` | Origin URL of the Go API as reachable from the admin container. |

> **The admin container listens on port 8080, not 80.** It runs as an
> unprivileged user (uid 101), which cannot bind a privileged port. Publish it
> with `-p 80:8080` (or point your ingress at 8080).

The default works only when the admin and API containers share a network where the API has the DNS name `api`. For a separate deployment, set `API_URL` to a reachable internal or public API origin (without a path) and either attach both services to a shared network or provide working DNS and routing. For example, if the API service is named `registry`, set `API_URL=http://registry:8080`. A permanently incorrect hostname continues to return `502`; there is no fallback backend.

The image uses the official nginx entrypoint's local resolver discovery and resolves the API hostname at request time. This lets nginx start before the API DNS record exists and recover after it appears. Only `API_URL` and the discovered resolver list are substituted into the template; nginx request variables remain intact. The image health check verifies that nginx can serve the SPA, not that the API backend is ready.

## Web app development

```bash
cd web
npm install
npm run dev
```

The Astro site runs on `http://localhost:3000`, builds static files into `web/dist`, and mirrors the repository root `plugins.json` into `web/public/plugins.json` during install/build.

## Production configuration

The API refuses to start in `ENVIRONMENT=prod` unless it can authenticate
callers properly. Each check exists because failing it silently downgrades
authentication to something forgeable:

| Requirement | Why |
|---|---|
| `JWT_SECRET` set, ≥ 32 characters | The development fallback is published in this repository; with it, anyone can mint an admin session. |
| `WEBHOOK_SECRET` set, ≥ 32 characters | Without it `POST /api/v1/webhooks/release` accepts unauthenticated calls that can trigger an organisation-wide sync. |
| `ADMIN_TOKEN` unused | A static, non-expiring, identity-less admin credential. It is ignored in production even if set. |
| `ALLOWED_ORIGINS` without `*` | Credentialed endpoints would otherwise be readable cross-origin by any site. |

Generate secrets with `openssl rand -base64 48`. See `.env.example` for the
complete list, including session-cookie, trusted-proxy and rate-limit settings.

### Sessions

The OAuth callback sets an `HttpOnly`, `SameSite=Lax` session cookie rather than
returning the token in the redirect URL. Browser clients therefore send no
`Authorization` header; API clients (the `semrel` CLI, CI jobs) continue to use
`Authorization: Bearer <token>`. Signing out revokes the session server-side, and
deleting an account requires an interactive GitHub sign-in within the last five
minutes.

### Release webhook

Plugin repositories authenticate release notifications by signing the request
body:

```
X-Hub-Signature-256: sha256=<hmac-sha256 of the raw body, keyed with WEBHOOK_SECRET>
```

The older `X-Webhook-Secret: <secret>` header still works and is compared in
constant time, but it transmits the secret on every call and is deprecated.

## Retracting a release

Deleting a version breaks every build that pins it. **Yanking** is the safe
retraction:

```bash
curl -X PUT https://registry.semrel.io/api/v1/plugins/@semrel/provider-github/versions/42/yank \
  -H 'Content-Type: application/json' \
  --cookie 'semrel_session=…' \
  -d '{"reason":"The linux-amd64 binary was built from the wrong commit."}'
```

A yanked version:

- stays resolvable, so `semrel plugin install name@1.2.3` keeps working;
- is never returned as `latestVersion` and never chosen as an update target;
- carries `"yanked": true` and `"yankedReason"` in `plugins.json`, so clients
  can warn the people already using it.

`DELETE` on the same path lifts the yank. Publishers may yank their own
plugins' versions; admins may yank any.

## Feeds and caching

- `GET /feed.atom` — the 50 most recent releases across the registry.
- `GET /plugins.json` carries a strong `ETag` and `Cache-Control`. Clients that
  send `If-None-Match` get a `304` instead of the whole catalogue.
- `POST /api/v1/plugins/:id/versions/:version/downloads` counts one download per
  client per version per hour, so the figure reflects adoption rather than how
  often a CI pipeline ran.

## Review notifications

A submitter may give an address on the submission form. If they do — and
`SMTP_HOST` and `SMTP_FROM` are configured — the registry emails them once,
when their plugin is approved or rejected, with the reviewer's reason.

The address is deliberately not derived from the GitHub profile and is not part
of the plugin record: it lives in a separate column (or, on the file backend, a
separate `0600` file), is returned by no endpoint, and is erased when the
account is deleted. Without SMTP configured the outcome is still recorded and
shown on the author's plugin list; only the delivery is skipped.

## Repository ownership

Forcing the `author` field to the submitter's login records who submitted a
plugin; it proves nothing about whether they control it. Submitting therefore
requires showing control of the repository, checked in increasing order of
effort and using only public GitHub endpoints:

1. the repository is on the submitter's own account;
2. the submitter is a **public** member of the owning organisation;
3. the default branch carries a `.semrel-registry-claim` file containing the
   submitter's login on a line of its own.

The claim file is the fallback that always works — a private org membership, a
collaborator who is not a member, a repository owned by a bot account — and
needs only the write access a maintainer already has. Admins are exempt, since
they import first-party plugins on the organisation's behalf.

`POST /api/v1/plugins/verify-ownership` runs the same check on demand, which is
what the submission form calls before asking for the rest of the details.

## Build provenance

A checksum proves an artifact's bytes were not altered in transit; it proves
nothing about who produced them. A publisher whose token has been stolen can
compute a perfectly correct checksum for a malicious binary, and the registry
would serve it happily.

When a version is published, the registry looks up GitHub's
[artifact attestation](https://docs.github.com/en/actions/security-guides/using-artifact-attestations-to-establish-provenance-for-builds)
for its artifact digest, in the background, and records:

- whether an attestation exists at all for these exact bytes;
- which repository and workflow the attestation says built them;
- whether that repository matches the one the plugin claims to come from.

A repository mismatch is the result that matters most — it means the artifact
was built somewhere other than where the plugin says it comes from — so it is
recorded as `verified: false` with an `issue`, not silently dropped. Provenance
is included in a version's API representation and in `plugins.json` once a
lookup has actually been attempted; a version that predates this feature, or
whose repository publishes no attestations, simply omits it.

`POST /api/v1/plugins/:id/versions/:version/reverify-provenance` re-runs the
check on demand — the automatic lookup on publish can run before GitHub has
finished generating the attestation, so a manual recheck shortly after often
succeeds where the first one didn't. Publishers may reverify their own
plugins' versions; admins may reverify any.

This verifies the attestation's *subject* — that one exists for this digest
and names the expected repository — not the Sigstore signature bundle itself,
which needs the full transparency-log client. What it rules out is the case
that matters most in a registry: an artifact whose bytes no build in the
claimed repository ever produced.
