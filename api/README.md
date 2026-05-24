# semrel-registry API

Go-based backend skeleton for the SemRels registry. The current MVP still serves `plugins.json` via GitHub Pages; this service prepares the backend for richer registry features on `registry.semrel.io`.

## Quick Start

1. Start PostgreSQL for local development:
   ```bash
   docker compose up -d
   ```
2. Copy the example environment file:
   ```bash
   cp .env.example .env
   ```
3. Run the API:
   ```bash
   go run main.go
   ```
4. Verify the health endpoint:
   ```bash
   curl http://localhost:8080/health
   ```

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `:8080` | Gin listen address |
| `DATABASE_URL` | `postgres://dev:dev@localhost:5432/semrel_registry?sslmode=disable` | PostgreSQL connection string |
| `MIGRATE_DIR` | `./database/migrations` | Migration directory used on startup |
| `ENVIRONMENT` | `dev` | Runtime environment |
| `ADMIN_TOKEN` | _unset_ | Optional bearer token for future admin routes |

## Database Setup

- Local development uses `docker-compose.yml` with PostgreSQL 16.
- Application startup runs embedded SQL migrations automatically via `golang-migrate`.
- Schema covers plugins, plugin versions, and version checksums with PostgreSQL array tags and soft deletes.

## Testing

Run the API test suite:

```bash
go test ./...
```

To run the optional database integration test, start PostgreSQL and set:

```bash
TEST_DATABASE_URL=postgres://dev:dev@localhost:5432/semrel_registry?sslmode=disable
```

## Build

```bash
go build ./...
```

## Container Image

Build the production image from the `api/` directory:

```bash
docker build -t semrel-registry-api .
```

> Note: the Docker image uses Go 1.25 to match the resolved module toolchain in `go.mod`.
