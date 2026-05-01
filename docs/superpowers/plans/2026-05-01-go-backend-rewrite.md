# Go Backend Rewrite Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the FastAPI backend with a Go backend while keeping the Vue frontend, current SQLite data, image files, public API paths, cookies, Docker image name, and deployment workflow compatible.

**Architecture:** Build a feature-first Go service under `server-go/` with `net/http`, `database/sql`, `modernc.org/sqlite`, signed-cookie-style opaque sessions stored in SQLite, an in-process image generation queue, and static frontend serving. Keep the existing SQLite schema as the baseline and remove Python-only migration compatibility after the Go server proves compatible.

**Tech Stack:** Go 1.24+, standard `net/http`, `database/sql`, `modernc.org/sqlite`, `golang.org/x/crypto/pbkdf2`, Vue/Vite frontend, SQLite, Docker multi-stage build.

---

## Compatibility Contract

- Keep these cookies unchanged: `gallery_session`, `gallery_admin_session`.
- Keep these routes unchanged:
  - `GET /health`
  - `GET /api/auth/captcha`
  - `POST /api/auth/register`
  - `POST /api/auth/login`
  - `POST /api/auth/logout`
  - `GET /api/auth/me`
  - `GET /api/images`
  - `GET /api/images/mine`
  - `POST /api/images`
  - `POST /api/images/edit`
  - `GET /api/images/{id}/events`
  - `POST /api/images/{id}/like`
  - `POST /api/images/{id}/favorite`
  - `POST /api/admin/login`
  - `POST /api/admin/logout`
  - `GET /api/admin/me`
  - `GET /api/admin/dashboard`
  - `GET /api/admin/users`
  - `PUT /api/admin/users/{id}/admin`
  - `GET /api/admin/generations`
  - `DELETE /api/admin/generations/{id}`
  - `PUT /api/admin/generations/{id}/hidden`
  - `GET /api/admin/providers`
  - `POST /api/admin/providers`
  - `PUT /api/admin/providers/{id}`
  - `GET /api/admin/settings`
  - `PUT /api/admin/settings/concurrency`
  - `GET /media/*`
  - SPA fallback for `/` and `/admin`.
- Keep existing SQLite tables and columns compatible with the current Python version.
- Keep model provider API key only in SQLite/admin UI, not in `.env` or source.
- Keep local image files under `IMAGE_STORAGE_DIR` and keep returned URLs as `/media/<relative-path>`.
- Write user-visible timestamps in `Asia/Shanghai` local format `YYYY-MM-DD HH:MM:SS`.

## Files To Create

- `server-go/go.mod`: Go module and dependencies.
- `server-go/cmd/server/main.go`: application entrypoint and graceful shutdown.
- `server-go/internal/config/config.go`: environment and path config.
- `server-go/internal/timeutil/time.go`: app-local timestamp helpers.
- `server-go/internal/httpx/http.go`: JSON, errors, cookies, routing helpers.
- `server-go/internal/db/db.go`: SQLite connection, schema creation, seed defaults.
- `server-go/internal/auth/captcha.go`: image captcha token store and PNG/SVG data URL generator.
- `server-go/internal/auth/service.go`: password hashing, login, user sessions.
- `server-go/internal/auth/handlers.go`: auth HTTP handlers.
- `server-go/internal/admin/repository.go`: admin queries and provider/settings operations.
- `server-go/internal/admin/handlers.go`: admin HTTP handlers.
- `server-go/internal/images/repository.go`: image queries, likes, favorites, visibility.
- `server-go/internal/images/provider.go`: OpenAI-compatible image generation/edit client.
- `server-go/internal/images/queue.go`: worker loop, concurrency, SSE job status.
- `server-go/internal/images/handlers.go`: gallery, upload, create, SSE, reactions.
- `server-go/internal/static/static.go`: `/media`, `/assets`, SPA fallback.
- `server-go/internal/app/app.go`: wires repositories, handlers, queue, routes.

## Files To Modify

- `Dockerfile`: replace Python runtime with Go build/runtime.
- `docker-compose.yml`: keep image and volume, keep `TZ`/`APP_TIMEZONE`.
- `.dockerignore`: stop excluding files needed by Go build if necessary.
- `README.md`: update stack and local development commands after Go backend is complete.
- `.github/workflows/docker-image.yml`: keep workflow unless Dockerfile path changes.

## Files To Delete After Verification

- `server/app/**`
- `server/requirements.txt`

Do not delete Python code until Go endpoints pass local smoke tests against the same frontend.

---

## Task 1: Scaffold Go Server And Config

**Files:**
- Create: `server-go/go.mod`
- Create: `server-go/cmd/server/main.go`
- Create: `server-go/internal/config/config.go`
- Create: `server-go/internal/timeutil/time.go`
- Create: `server-go/internal/httpx/http.go`

- [ ] **Step 1: Initialize module**

Run:

```bash
cd server-go
go mod init github.com/QWiseDev/Img2Gallery/server-go
go get modernc.org/sqlite golang.org/x/crypto/pbkdf2
```

Expected: `server-go/go.mod` and `server-go/go.sum` exist.

- [ ] **Step 2: Implement config**

`server-go/internal/config/config.go` must expose:

```go
type Config struct {
    Addr            string
    AppSecret       string
    AdminPassword   string
    DatabasePath    string
    ImageStorageDir string
    ClientOrigin    string
    AppTimezone     string
    ProjectRoot     string
    FrontendDist    string
}

func Load() Config
```

Defaults:

```text
ADDR=0.0.0.0:8000
APP_SECRET=dev-secret-change-me
ADMIN_PASSWORD=admin123456
DATABASE_PATH=server/app.db
IMAGE_STORAGE_DIR=server/storage/images
CLIENT_ORIGIN=http://localhost:5173
APP_TIMEZONE=Asia/Shanghai
```

- [ ] **Step 3: Implement HTTP helpers**

`server-go/internal/httpx/http.go` must provide:

```go
func JSON(w http.ResponseWriter, status int, payload any)
func Error(w http.ResponseWriter, status int, detail string)
func DecodeJSON(r *http.Request, dst any) bool
func ClientIP(r *http.Request) string
func SetCookie(w http.ResponseWriter, name, value string, maxAge int)
func ClearCookie(w http.ResponseWriter, name string)
```

Error response format must stay:

```json
{"detail":"错误信息"}
```

- [ ] **Step 4: Implement app time**

`server-go/internal/timeutil/time.go` must provide:

```go
func LocalTimestamp(locationName string) string
```

It returns `YYYY-MM-DD HH:MM:SS` in `Asia/Shanghai` if config is missing or invalid.

- [ ] **Step 5: Add minimal main**

`cmd/server/main.go` starts an HTTP server with `/health` returning `{"status":"ok"}`.

- [ ] **Step 6: Verify**

Run:

```bash
cd server-go
go test ./...
go run ./cmd/server
curl http://127.0.0.1:8000/health
```

Expected: tests pass and `{"status":"ok"}`.

---

## Task 2: SQLite Schema Baseline And Repository Foundation

**Files:**
- Create: `server-go/internal/db/db.go`

- [ ] **Step 1: Implement SQLite open**

Use `database/sql` and blank import `modernc.org/sqlite`.

`Open(cfg config.Config) (*sql.DB, error)` must:
- Create parent directory for `DATABASE_PATH`.
- Open SQLite.
- Execute `PRAGMA foreign_keys = ON`.
- Return `*sql.DB`.

- [ ] **Step 2: Implement schema creation**

`Init(db *sql.DB, cfg config.Config) error` creates the current final schema only:

```sql
users(id, username, display_name, password_hash, avatar_color, is_admin, last_login_ip, last_login_at, created_at)
sessions(token, user_id, expires_at, created_at)
images(id, user_id, prompt, image_path, task_type, source_image_path, is_hidden, status, error, request_ip, provider_name, model, queued_at, started_at, completed_at, created_at)
image_likes(image_id, user_id, created_at)
image_favorites(image_id, user_id, created_at)
admin_sessions(token, expires_at, created_at)
app_settings(key, value, updated_at)
model_providers(id, name, provider_type, model, api_base, api_key, enabled, is_default, created_at, updated_at)
```

Keep `CREATE TABLE IF NOT EXISTS`; do not include old Python migration code.

- [ ] **Step 3: Seed defaults**

Insert if missing:

```text
app_settings.generation_concurrency = 1
model_providers default: GPT Image 2 / openai_compatible / gpt-image-2 / empty api_base / empty api_key
```

Use app-local timestamp helper.

- [ ] **Step 4: Verify against existing DB**

Run:

```bash
cd server-go
DATABASE_PATH=../server/app.db IMAGE_STORAGE_DIR=../server/storage/images go test ./...
```

Expected: schema init does not fail on existing DB.

---

## Task 3: Auth And Captcha

**Files:**
- Create: `server-go/internal/auth/captcha.go`
- Create: `server-go/internal/auth/service.go`
- Create: `server-go/internal/auth/handlers.go`

- [ ] **Step 1: Implement password compatibility**

Python password hash format is:

```text
<salt_hex>$<pbkdf2_sha256_hex>
```

Go must verify PBKDF2-SHA256 with 120000 iterations and generate the same format for new users.

- [ ] **Step 2: Implement captcha**

Keep API-compatible payload:

```json
{"token":"...","image":"data:image/svg+xml;base64,..."}
```

Use in-memory one-time token store with 5 minute expiry. The frontend only needs an image data URL; SVG is acceptable and removes Python image dependencies.

- [ ] **Step 3: Implement user session**

Cookie:

```text
gallery_session
```

Session stored in `sessions`, 14 days, `HttpOnly`, `SameSite=Lax`.

- [ ] **Step 4: Implement routes**

Implement:
- `GET /api/auth/captcha`
- `POST /api/auth/register`
- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/auth/me`

- [ ] **Step 5: Verify**

Use `httptest` or curl:

```bash
curl http://127.0.0.1:8000/api/auth/captcha
curl http://127.0.0.1:8000/api/auth/me
```

Expected:
- captcha returns token/image.
- unauthenticated `/me` returns 401 `{"detail":"请先登录"}`.

---

## Task 4: Admin API

**Files:**
- Create: `server-go/internal/admin/repository.go`
- Create: `server-go/internal/admin/handlers.go`

- [ ] **Step 1: Implement admin auth**

Accept either:
- user session where `users.is_admin = 1`
- admin password session cookie `gallery_admin_session`

Admin password is `ADMIN_PASSWORD`.

- [ ] **Step 2: Implement dashboard and users**

Keep response shape currently consumed by `AdminView.vue`:
- `dashboard()` returns users count, images counts, concurrency, providers.
- `users_overview()` returns login IP/time and usage counters.

- [ ] **Step 3: Implement provider management**

Keep key preservation behavior:
- If `api_key` is null/empty on update, keep existing key.
- If `is_default` is true, clear `is_default` on other providers first.

- [ ] **Step 4: Implement generation management**

Implement:
- list generation records
- delete generation and delete local result/source files inside storage root
- hide/restore generation

- [ ] **Step 5: Verify**

Run:

```bash
curl -i http://127.0.0.1:8000/api/admin/me
curl -i -X POST http://127.0.0.1:8000/api/admin/login -H 'Content-Type: application/json' -d '{"password":"admin123456"}'
```

Expected:
- unauthenticated `/me` is 401.
- login returns 200 and sets `gallery_admin_session`.

---

## Task 5: Images, Queue, Provider Client, SSE

**Files:**
- Create: `server-go/internal/images/repository.go`
- Create: `server-go/internal/images/provider.go`
- Create: `server-go/internal/images/queue.go`
- Create: `server-go/internal/images/handlers.go`

- [ ] **Step 1: Implement image repository**

Keep serialized response fields exactly:

```json
id, prompt, image_url, task_type, source_image_url, is_hidden, status, error,
request_ip, provider_name, model, queued_at, started_at, completed_at,
created_at, author, likes, favorites, liked_by_me, favorited_by_me
```

Public gallery filters `images.is_hidden = 0`.
My images includes hidden images.

- [ ] **Step 2: Implement create image and edit image**

`POST /api/images` creates a queued `generate` row.
`POST /api/images/edit` validates:
- prompt length 2..4000
- content type `image/png`, `image/jpeg`, `image/webp`
- max size 10MB

Save edit source to `IMAGE_STORAGE_DIR/sources/<random>.<ext>`.

- [ ] **Step 3: Implement OpenAI-compatible provider**

Generation:

```text
POST {api_base}/v1/images/generations
Authorization: Bearer <api_key>
JSON: model, prompt, n=1, response_format=b64_json
```

Edit:

```text
POST {api_base}/v1/images/edits
multipart: model, prompt, n=1, image=@file
```

Accept `b64_json` or `url` response.

- [ ] **Step 4: Implement queue**

On startup:
- reset `running` to `queued`.
- run loop every second.
- concurrency reads `generation_concurrency`.
- mark running/ready/failed.
- timeout 720 seconds.

- [ ] **Step 5: Implement SSE**

`GET /api/images/{id}/events` emits:

```text
data: {"status":"queued","position":1,"queue":{"queued":1,"running":0},"image":{...}}
```

Stop when status is `ready` or `failed`.

- [ ] **Step 6: Verify**

Use a test DB and no provider:
- create image returns queued row.
- event stream returns queued payload.
- missing provider eventually marks failed.

---

## Task 6: Static Files, Frontend Integration, Docker

**Files:**
- Create: `server-go/internal/static/static.go`
- Create: `server-go/internal/app/app.go`
- Modify: `Dockerfile`
- Modify: `docker-compose.yml`

- [ ] **Step 1: Serve media**

Serve `/media/*` from `IMAGE_STORAGE_DIR`.
Prevent path traversal by relying on `http.FileServer` rooted at storage dir.

- [ ] **Step 2: Serve frontend**

Serve:
- `/assets/*` from `client/dist/assets`
- requested static files if present
- fallback to `client/dist/index.html` for `/`, `/admin`, and unknown frontend routes

- [ ] **Step 3: Wire app**

`internal/app/app.go` creates:
- DB
- repositories
- handlers
- queue manager
- router

- [ ] **Step 4: Update Dockerfile**

Build stages:
1. Node builds Vue.
2. Go builds static binary.
3. Runtime copies binary and `client/dist`.

Runtime env defaults:

```text
DATABASE_PATH=/data/app.db
IMAGE_STORAGE_DIR=/data/images
CLIENT_ORIGIN=http://localhost:8000
TZ=Asia/Shanghai
APP_TIMEZONE=Asia/Shanghai
```

- [ ] **Step 5: Verify Docker locally**

Run:

```bash
docker build -t img2gallery-go-local .
docker run --rm -p 8002:8000 -e ADMIN_PASSWORD=admin123456 img2gallery-go-local
curl http://127.0.0.1:8002/health
curl http://127.0.0.1:8002/admin
```

Expected: health is OK and admin HTML is served.

---

## Task 7: Remove Python Backend And Update Docs

**Files:**
- Delete: `server/app/**`
- Delete: `server/requirements.txt`
- Modify: `README.md`
- Modify: `.gitignore`
- Modify: `.dockerignore`

- [ ] **Step 1: Remove Python backend after Go smoke passes**

Delete only tracked Python backend files. Do not delete local `.venv`, `server/app.db`, or image storage.

- [ ] **Step 2: Update docs**

README must say:
- Stack is Vue + Go + SQLite.
- Local backend command is `cd server-go && go run ./cmd/server`.
- Docker usage remains `docker compose pull && docker compose up -d`.
- Model provider remains admin-only.
- Timezone defaults to Asia/Shanghai.

- [ ] **Step 3: Verify final repo**

Run:

```bash
go test ./server-go/...
cd client && npm run build
docker build -t img2gallery-go-local .
git diff --check
```

Expected: all pass.

---

## Rollback Plan

- `py` branch points to the last Python backend state: `608365a`.
- If Go replacement fails in production, deploy from `py` by building that branch or reverting `main` to the backup branch commit.
- SQLite and image files are preserved; Go does not perform destructive migrations.

## Execution Mode

User requested phased execution with testing. Execute this plan inline on `main`, committing after stable milestones:

1. Go scaffold and health.
2. DB/auth/admin.
3. images/queue/provider/SSE.
4. Docker/static/frontend.
5. Python cleanup/docs.
