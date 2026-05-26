# shorte

`shorte` is a URL shortener project built mainly as a playground to experiment with vibe coding.

The goal is not just to ship a working shortener, but to use a small, concrete service to test how far fast iterative coding can go with:
- Go backend code with minimal dependencies
- Redis for hot-path caching and queued click events
- PostgreSQL for durable storage
- A simple WebUI for basic link management

## What it does

- Shortens long URLs into short codes
- Redirects fast through Redis first, then PostgreSQL on cache miss
- Lets authenticated users create and manage links
- Collects basic click stats
- Ships with a minimal WebUI

## Requirements

- Go `>= 1.25` for local builds and runs
- Docker and Docker Compose for the full stack

## Run Everything

Start the full stack with:

```bash
docker compose up --build
```

Then open:

- `http://localhost:8080`

PostgreSQL schema is initialized automatically on first boot from `migrations/001_init.sql`.

If you need a clean reset:

```bash
docker compose down -v
docker compose up --build
```

## Local Development

If you want to run the Go binaries locally instead of in containers:

```bash
docker compose up -d postgres redis
go run ./cmd/api
go run ./cmd/worker
```

Use the values from `.env.example` for local environment variables.

## Endpoints

- `GET /`
- `GET /login`
- `GET /register`
- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/logout`
- `GET /api/v1/auth/me`
- `POST /api/v1/links`
- `GET /api/v1/links`
- `GET /api/v1/links/{code}`
- `PATCH /api/v1/links/{code}`
- `DELETE /api/v1/links/{code}`
- `GET /api/v1/links/{code}/stats?from=YYYY-MM-DD&to=YYYY-MM-DD`
- `GET /r/{code}`
- `GET /health/live`
- `GET /health/ready`
