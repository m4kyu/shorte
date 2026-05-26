# AGENTS.md

## Purpose

This repository contains `shorte`, a Go-based URL shortener built mainly as a playground to experiment with vibe coding.

## Working Rules

- Keep changes small, direct, and easy to verify.
- Prefer the standard library when it does not materially weaken the implementation.
- Preserve the current architecture unless a change is explicitly requested.
- Do not rewrite unrelated files or revert user changes.
- Use `apply_patch` for manual file edits.
- Prefer `rg` for searches and `rg --files` for file discovery.

## Runtime Notes

- The service is split into an API binary and a worker binary.
- Redis is used for cache and click buffering.
- PostgreSQL is the durable source of truth.
- Docker Compose is the primary local bring-up path.

## Verification

- Run targeted tests after code changes when possible.
- Prefer `go test ./...` for backend changes.
- Confirm Docker changes with `docker compose up --build` when relevant.

## Commit Policy

- Sign commits with `git commit -S`.
- Keep commit messages short and specific.

## Documentation

- Keep `README.md` aligned with the actual run steps and project purpose.
- Update this file if the workflow or repo conventions change.
