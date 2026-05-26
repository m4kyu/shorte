# shorte

Fast URL shortener service with:
- Go API (`net/http`)
- Redis for redirect cache and click queue
- PostgreSQL for durable links and aggregated stats
- Minimal WebUI served by API

## Requirements

- Go `>= 1.25` for local builds/runs (because `github.com/jackc/pgx/v5@v5.9.2` requires it).

## Run

### Full stack with Docker Compose

1. Start all services:
   - `docker compose up --build`
2. Open WebUI:
   - `http://localhost:8080`

Notes:
- PostgreSQL schema is auto-initialized from `migrations/001_init.sql` on first DB boot.
- If `pgdata` volume already exists and you need a fresh DB init:
  - `docker compose down -v`
  - `docker compose up --build`

### Local Go run (without API/worker containers)

1. Start dependencies only:
   - `docker compose up -d postgres redis`
2. Apply SQL migration from `migrations/001_init.sql` to your `shorte` database.
3. Export env vars from `.env.example`.
4. Start API: `go run ./cmd/api`
5. Start worker: `go run ./cmd/worker`

## Endpoints

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/logout`
- `POST /api/v1/links`
- `GET /api/v1/links`
- `GET /api/v1/links/{code}`
- `PATCH /api/v1/links/{code}`
- `DELETE /api/v1/links/{code}`
- `GET /api/v1/links/{code}/stats?from=YYYY-MM-DD&to=YYYY-MM-DD`
- `GET /r/{code}`
- `GET /health/live`
- `GET /health/ready`
