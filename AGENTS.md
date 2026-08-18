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
npm test                             # vitest unit tests (api silent refresh/dedup/errors, i18n, utils, ConfirmDialog, Login)

# E2E tests (requires docker compose up -d with user-net)
bash e2e/run.sh

# Production image (pulled from ghcr.io/wjw1-evan/opencode-cloud:latest via docker-compose.yml)
# Dev environment (builds locally, serves on :18080 with defaults)
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
- **Image management**: `internal/api/handlers_images.go` + `internal/docker/images.go` — list (shows in-use status + referrers)/pull/import (docker save tar ≤2GB)/delete (in-use images refused with 409)/inspect via Docker SDK; offline-friendly
- **Deploy**: `docker-compose.yml` is zero-config (defaults: `JWT_SECRET=dev-secret-change-me`, `POSTGRES_PASSWORD=devcapsule`, `COOKIE_SECURE=0`; override via env), joins `devcapsule_user-net`; `docker-compose.dev.yml` builds locally with defaults on host port 18080

## Testing Notes

- `integration_test.go` in `internal/docker/` requires Docker daemon; set `TEST_DOCKER=0` to skip
- E2e tests use `ADMIN_USERNAME`/`ADMIN_PASSWORD` env vars, default `admin`/`admin-e2e-pass`
- No lint/typecheck commands configured; use `go vet` for static analysis, `npm test` (vitest) for frontend
- Run backend tests with `-race` before pushing changes

## 开发工作流

- **修复问题后，主动查找同类问题**：修复某个 bug / 缺陷 / 样式问题时，必须搜索代码库中是否存在相同模式的其它位置，一并修复或至少评估，避免「只修一处」。常见排查方向：
  - 修复某个页面的响应式 / 样式问题时，检查其余页面是否有同样问题（卡片宽度、表格溢出、弹窗间距等）
  - 修复某个 API handler 的错误处理 / 边界条件时，检查同文件其它 handler 以及同类型的路由
  - 新增或修改 i18n 文案时，检查中英文是否同步、旧 key 是否有未使用残留
  - 修复某处安全 / 性能问题时，检查全站是否存在同类风险（Cookie 标志、轮询频率、动画开销等）
- 每次修改后运行对应测试（后端 `go test -race ./...`、前端 `npm test` + `npm run build`），涉及全链路时跑一次 `e2e/run.sh`

## Conventions

- Go standard library `net/http` for HTTP, no framework
- PostgreSQL 17 (pgx driver), migrations run at startup
- JWT stored in HttpOnly cookies, 30min access / 24h refresh; access/refresh tokens are type-isolated (refresh cannot authenticate, access cannot refresh); `POST /platform/auth/refresh` rotates both cookies; frontend silently refreshes on 401 and replays the request once (see `frontend/src/api.js`)
- Passwords: argon2 hash, plain text stored in `password_plain` for admin viewing
- Containers: each user gets isolated container with random password, CPU/mem/PID limits
- Templates: `internal_port` + `extra_ports[]`, `run_user`/`cap_add` for images needing root/caps (e.g. dify s6-overlay); 4 system templates seeded at startup, not deletable
- Batch actions (`users/batch/action`): `start | restart | stop | delete`; container rebuild via `containers/batch` with `force`
