# AGENTS.md

## Project Overview

DevCapsule - Docker-based container management platform for isolated dev environments. Go backend + Vue3 frontend + PostgreSQL.

## Key Commands

```bash
# Backend (from backend/)
go test ./...                        # unit + integration tests
TEST_DOCKER=0 go test ./...          # skip Docker integration tests
go vet ./...                         # static analysis
go build ./cmd/server                # build binary
go run ./cmd/server                  # run locally (requires DATABASE_URL, JWT_SECRET, Docker)

# Frontend (from frontend/)
npm install && npm run dev           # Vite dev server (http://localhost:5173, proxies /api /dc-static /platform to :8080)
npm run build                        # builds to backend/internal/api/web/dist (embedded into Go binary)

# E2E tests (requires docker compose up -d with user-net)
bash e2e/run.sh

# Production image (pulled from ghcr.io/wjw1-evan/opencode-cloud:latest via docker-compose.yml)
# Dev environment (builds locally, serves on :80 with defaults)
docker compose -f docker-compose.dev.yml up -d --build
docker compose -f docker-compose.dev.yml down   # stop
```

## Architecture

- **Entry point**: `backend/cmd/server/main.go` → config → migrate → seed system templates → Docker client → HTTP server on :8080
- **API routes**: `/platform/auth/initialized|initialize` (first-run admin setup), `/platform/auth/*` (auth), `/platform/admin/*` (admin: users/containers/templates/images/stats), `/*` (proxy to user containers)
- **SPA routes**: `/portal`, `/admin`, `/initialize`, `/dc-static/*` always serve the platform UI; everything else → logged-in student → proxy, else SPA
- **Proxy**: JWT-authenticated reverse proxy to user containers via Docker network `user-net`
- **Frontend**: Vue3 SPA (zh/en i18n in `frontend/src/i18n.js`) built to `backend/internal/api/web/dist/` → embed in Go binary. Admin UI pages: Dashboard / Users & Containers / Templates / Images / Help
- **First-run init**: `GET /platform/auth/initialized` detects no admin; `POST /platform/auth/initialize` creates one (only once). No `ADMIN_PASSWORD` env var anymore.
- **Image management**: `internal/api/handlers_images.go` + `internal/docker/images.go` — list/pull/import (docker save tar ≤2GB)/delete/inspect via Docker SDK; offline-friendly
- **Deploy**: `docker-compose.yml` requires `JWT_SECRET` (`${JWT_SECRET:?}`), joins `devcapsule_user-net`; `docker-compose.dev.yml` builds locally with defaults

## Testing Notes

- `integration_test.go` in `internal/docker/` requires Docker daemon; set `TEST_DOCKER=0` to skip
- E2e tests use `ADMIN_USERNAME`/`ADMIN_PASSWORD` env vars, default `admin`/`admin-e2e-pass`
- No lint/typecheck commands configured; use `go vet` for static analysis

## Conventions

- Go standard library `net/http` for HTTP, no framework
- PostgreSQL 17 (pgx driver), migrations run at startup
- JWT stored in HttpOnly cookies, 30min access / 24h refresh
- Passwords: argon2 hash, plain text stored in `password_plain` for admin viewing
- Containers: each user gets isolated container with random password, CPU/mem/PID limits
- Templates: `internal_port` + `extra_ports[]`, `run_user`/`cap_add` for images needing root/caps (e.g. dify s6-overlay); 4 system templates seeded at startup, not deletable
- Batch actions (`users/batch/action`): `start | restart | stop | delete`; container rebuild via `containers/batch` with `force`
