# Project Cleanup And Refactor Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 清理历史需求变动留下的临时产物和弱契约代码，补齐检查网，并把前后端模块接口收敛到更清晰、可验证、可维护的形态。

**Architecture:** 当前项目是 Vue 3/Vite 前端、Go `net/http` 后端、SQLite 本地存储。后端已从旧 Python 方案迁移到 `server-go/`，按 `auth`、`images`、`admin`、`db`、`static` feature package 组织；本轮重构优先稳定 API/DB/storage 契约，再拆分过大的前端页面和 Go 仓储返回结构。

**Tech Stack:** Vue 3, Vite, lucide-vue-next, Go 1.25, net/http, database/sql, modernc.org/sqlite, SQLite, Docker.

---

## Current Architecture Map

- `client/src/App.vue`: 基于 `window.location.pathname === '/admin'` 在首页和管理后台之间切换。
- `client/src/services/api.js`: 前端唯一 API 适配层；生产环境默认同源，开发环境默认 `http://127.0.0.1:8000`。
- `client/src/components/HomeView.vue`: 首页画廊、登录注册、生成/编辑工作台、SSE 队列订阅、预览弹窗、轻量管理动作都集中在一个 SFC。
- `client/src/components/AdminView.vue`: 管理员登录、仪表盘、用户管理、生成记录、模型提供商和并发设置集中在一个 SFC。
- `server-go/internal/app/app.go`: 应用装配层，初始化 DB、auth、images queue、admin、static routes 和 CORS。
- `server-go/internal/db/db.go`: SQLite schema 创建和默认设置/默认 provider seed。
- `server-go/internal/auth/*`: 验证码、用户注册登录、用户 session cookie。
- `server-go/internal/images/*`: 图片任务记录、参数校验、图库查询、SSE、队列、provider 请求、本地图片落盘。
- `server-go/internal/admin/*`: 管理员 session、用户/生成记录/provider/settings 管理。
- `server-go/internal/static/static.go`: `/media/*`、构建后的 `/assets/*` 和 SPA fallback。

## Interface Contract Inventory

Keep these route contracts stable unless a later task explicitly updates both frontend and backend:

- Auth: `GET /api/auth/captcha`, `POST /api/auth/register`, `POST /api/auth/login`, `POST /api/auth/logout`, `GET /api/auth/me`.
- Gallery/images: `GET /api/images`, `GET /api/images/mine`, `POST /api/images`, `POST /api/images/edit`, `GET /api/images/{id}/events`, `POST /api/images/{id}/like`, `POST /api/images/{id}/favorite`.
- Admin: `POST /api/admin/login`, `POST /api/admin/logout`, `GET /api/admin/me`, `GET /api/admin/dashboard`, `GET /api/admin/users`, `PUT /api/admin/users/{id}/admin`, `GET /api/admin/generations`, `DELETE /api/admin/generations/{id}`, `PUT /api/admin/generations/{id}/hidden`, `GET /api/admin/providers`, `POST /api/admin/providers`, `PUT /api/admin/providers/{id}`, `GET /api/admin/settings`, `PUT /api/admin/settings/concurrency`.
- Static: `GET /media/*`, `GET /assets/*`, `GET /`, `GET /admin`, `GET /health`.
- Cookies: `gallery_session`, `gallery_admin_session`.
- Storage paths: DB defaults to `server/app.db` locally and `/data/app.db` in Docker; image storage defaults to `server/storage/images` locally and `/data/images` in Docker; returned media URLs use `/media/<relative-path>`.

## Findings

- `client/src/components/HomeView.vue` is 1146 lines and mixes state, API orchestration, image-size math, upload validation, SSE, gallery rendering, auth modal, preview modal, and admin actions. This is the largest cleanup target.
- `client/src/assets/styles.css` and `client/src/assets/tech-theme.css` are both loaded globally and together exceed 4100 lines. This makes stale selectors from prior UI variants hard to identify.
- `server-go/internal/images/repository.go` and `server-go/internal/images/scan.go` return `map[string]any` for core API responses. This weakens the frontend/backend contract and leads to runtime type assertions in `images/handlers.go` and `images/queue.go`.
- `server-go/internal/admin/repository.go` and `server-go/internal/admin/scan.go` also return `map[string]any`, so admin payload shape is implicit.
- `server-go/internal/images/repository.go` has a dynamic table-name path in `ToggleRelation`; current callers pass fixed constants, but the repository API makes unsafe use possible.
- Existing DB file `server/app.db` may come from an older schema. Runtime code should not keep old-data compatibility paths; schema/data changes are handled by the explicit `server-go/cmd/db-upgrade` command.
- Current `go test ./...` fails in the default shell because Go is configured for `GOOS=linux GOARCH=amd64` while the host is `darwin/arm64`. With `GOOS=darwin GOARCH=arm64`, tests pass.
- `npm run build` passes.
- Untracked local artifacts exist at repo root: `claude-style-desktop-check.png`, `homepage-desktop-check*.png`, `homepage-gallery-first-check.png`, `homepage-mobile-check*.png`, `playground-desktop-check.png`, `playground-mobile-check.png`.
- Tracked Python backend files are already absent. Local `server/app.db` and `server/storage/images/*` remain as runtime data, which is expected but must stay ignored.
- `server-go/internal/app/app.go` had a broad self-origin CORS fallback using substring matching. This has been tightened to parsed host/port checks while keeping explicit local dev origins.
- `config.Config.ProjectRoot` was a migration-era public field after path resolution moved inside `config.Load`; runtime code did not read it, so it has been removed.
- `images` queries now require `image_params` via an inner join. Older image rows must be upgraded first instead of receiving runtime default params in `scanImage`.

## Task 1: Establish Baseline Checks

**Files:**
- Read: `server-go/go.mod`
- Read: `client/package.json`
- Read: `.gitignore`
- Read: `.dockerignore`
- Modify only if needed: `README.md`

- [x] **Step 1: Record current git state**

Run:

```bash
git status --short
```

Expected: only intentional local artifacts are listed. Do not delete untracked screenshots or runtime DB/images without explicit user approval.

- [x] **Step 2: Run backend tests with host override**

Run:

```bash
cd server-go
GOOS=darwin GOARCH=arm64 go test ./...
```

Expected: all packages pass; packages without tests report `[no test files]`.

- [x] **Step 3: Document Go env caveat**

Run:

```bash
cd server-go
go env GOOS GOARCH GOHOSTOS GOHOSTARCH
```

Expected on this machine before environment cleanup: `linux`, `amd64`, `darwin`, `arm64`. Add a README note only if the developer environment is meant to keep cross-compilation defaults.

- [x] **Step 4: Run frontend build**

Run:

```bash
cd client
npm run build
```

Expected: Vite build succeeds and writes `client/dist`.

- [x] **Step 5: Run schema health check on a copied DB**

Run:

```bash
tmpdb="$(mktemp /tmp/img2gallery-db-XXXXXX.db)"
cp server/app.db "$tmpdb"
cd server-go
DATABASE_PATH="$tmpdb" IMAGE_STORAGE_DIR="$(mktemp -d /tmp/img2gallery-images-XXXXXX)" GOOS=darwin GOARCH=arm64 go run ./cmd/db-upgrade
sqlite3 "$tmpdb" '.tables'
rm -f "$tmpdb"
```

Expected: copied DB contains `image_params`, missing image params rows are backfilled, old user/admin sessions are cleared, and `PRAGMA user_version` equals the current schema version.

Actual cleanup pass added `db.Upgrade` and `cmd/db-upgrade`; the copied DB upgrade path is covered by tests before touching the default local DB.

## Task 2: Clean Non-Source Artifacts

**Files:**
- Inspect: `.gitignore`
- Inspect: `.dockerignore`
- Candidate cleanup, user-approved only: root `*.png`, `client/dist/`, local `server/app.db`, local `server/storage/images/*`

- [ ] **Step 1: Classify artifacts**

Run:

```bash
find . -maxdepth 2 -type f \( -name '*.png' -o -name '*.jpg' -o -name '*.jpeg' -o -name '*.webp' -o -name '*.log' -o -name '*.db' \) -print | sort
```

Expected current cleanup candidates:

```text
./claude-style-desktop-check.png
./homepage-desktop-check-2.png
./homepage-desktop-check.png
./homepage-gallery-first-check.png
./homepage-mobile-check-2.png
./homepage-mobile-check.png
./playground-desktop-check.png
./playground-mobile-check.png
./server/app.db
```

- [ ] **Step 2: Confirm deletion scope with user**

Do not delete runtime data by default. Ask for explicit approval before removing screenshots, DB, or local image files.

- [ ] **Step 3: If approved, remove only selected artifacts**

Run only after approval:

```bash
rm ./claude-style-desktop-check.png ./homepage-desktop-check-2.png ./homepage-desktop-check.png ./homepage-gallery-first-check.png ./homepage-mobile-check-2.png ./homepage-mobile-check.png ./playground-desktop-check.png ./playground-mobile-check.png
```

Expected: `git status --short` no longer lists those screenshots.

## Task 3: Add Backend Contract Tests

**Files:**
- Create: `server-go/internal/app/app_test.go`
- Create: `server-go/internal/images/repository_test.go`
- Create: `server-go/internal/auth/service_test.go`
- Modify if needed: `server-go/internal/db/db.go`

- [x] **Step 1: Add app route smoke tests**

Test cases:

- `GET /health` returns `200` and JSON `{"status":"ok"}`.
- `GET /api/auth/captcha` returns `200`, a non-empty `token`, and `image` beginning with `data:image/svg+xml;base64,`.
- `GET /api/images?sort=latest&offset=0&limit=24` returns `200` and `[]` on an empty DB.
- `GET /admin` returns SPA fallback when `client/dist/index.html` exists, or JSON 404 `"Frontend has not been built"` when it does not.

Run:

```bash
cd server-go
GOOS=darwin GOARCH=arm64 go test ./internal/app -run Test -v
```

Expected: route smoke tests pass.

- [x] **Step 2: Add image parameter normalization tests**

Cover:

- invalid size becomes `auto`;
- `1536x1024` remains valid;
- `png` clears `output_compression`;
- `jpeg/webp` clamp compression to `0..100`;
- invalid quality and moderation become `auto`.

Run:

```bash
cd server-go
GOOS=darwin GOARCH=arm64 go test ./internal/images -run 'TestNormalizeParams|TestRepository' -v
```

Expected: parameter and repository tests pass.

- [x] **Step 3: Add DB init/schema and upgrade tests**

Use `t.TempDir()` for SQLite and assert these tables exist after `db.Init`: `users`, `sessions`, `images`, `image_params`, `image_likes`, `image_favorites`, `admin_sessions`, `app_settings`, `model_providers`.

Also cover:

- `db.Upgrade` creates current schema on an existing DB;
- missing `image_params` rows are backfilled;
- user/admin sessions are cleared so old login state is not reused;
- existing unversioned DBs must run `cmd/db-upgrade` before server startup.

Run:

```bash
cd server-go
GOOS=darwin GOARCH=arm64 go test ./internal/db -run TestInitCreatesExpectedTables -v
```

Expected: schema and upgrade tests pass.

- [x] **Step 4: Add auth hashing tests**

Cover:

- `HashPassword` returns `<salt>$<digest>`;
- `VerifyPassword` accepts the right password;
- `VerifyPassword` rejects the wrong password;
- username normalization lowers and trims username.

Run:

```bash
cd server-go
GOOS=darwin GOARCH=arm64 go test ./internal/auth -run Test -v
```

Expected: auth tests pass.

## Task 4: Replace Weak Backend Maps With Typed DTOs

**Files:**
- Create: `server-go/internal/images/types.go`
- Modify: `server-go/internal/images/scan.go`
- Modify: `server-go/internal/images/repository.go`
- Modify: `server-go/internal/images/handlers.go`
- Modify: `server-go/internal/images/queue.go`
- Create: `server-go/internal/admin/types.go`
- Modify: `server-go/internal/admin/scan.go`
- Modify: `server-go/internal/admin/repository.go`
- Modify: `server-go/internal/admin/handlers.go`

- [x] **Step 1: Introduce images DTOs**

Define explicit structs for `Image`, `ImageAuthor`, `ImageParams`, `QueueCounts`, and `QueueEvent`. JSON names must match current frontend keys: `image_url`, `task_type`, `source_image_path`, `source_image_url`, `is_hidden`, `provider_name`, `queued_at`, `started_at`, `completed_at`, `liked_by_me`, `favorited_by_me`.

- [x] **Step 2: Change image repository return types**

Change:

```go
ListImages(...) ([]Image, error)
ListUserImages(...) ([]Image, error)
GetImage(...) (Image, bool, error)
QueuePosition(...) *int
QueueCounts() QueueCounts
```

Keep JSON output unchanged.

- [x] **Step 3: Replace runtime type assertions**

Remove type assertions like `image["author"].(map[string]any)` and access typed fields instead.

- [x] **Step 4: Lock relation table names**

Replace `ToggleRelation(table string, ...)` with explicit methods:

```go
ToggleLike(imageID, userID int) (bool, error)
ToggleFavorite(imageID, userID int) (bool, error)
```

Expected behavior: no caller can pass an arbitrary table name.

- [x] **Step 5: Introduce admin DTOs**

Define explicit structs for dashboard, user overview, generation record, provider response, settings response, and admin identity. JSON keys must match current frontend usage.

- [x] **Step 6: Run contract checks**

Run:

```bash
cd server-go
GOOS=darwin GOARCH=arm64 go test ./...
cd ../client
npm run build
```

Expected: backend tests and frontend build pass.

## Task 5: Split HomeView Into Focused Frontend Modules

**Files:**
- Create: `client/src/composables/useAuth.js`
- Create: `client/src/composables/useGallery.js`
- Create: `client/src/composables/useImageJob.js`
- Create: `client/src/utils/imageParams.js`
- Create: `client/src/components/AuthModal.vue`
- Create: `client/src/components/GalleryGrid.vue`
- Create: `client/src/components/ImagePreview.vue`
- Create: `client/src/components/ImageWorkbench.vue`
- Modify: `client/src/components/HomeView.vue`

- [ ] **Step 1: Extract image parameter utilities**

Move pure functions from `HomeView.vue` into `client/src/utils/imageParams.js`: `parseSize`, `parseRatio`, `calculateImageSize`, `normalizeImageSize`, `findSizePreset`, `roundToMultiple`, `floorToMultiple`, `ceilToMultiple`, `normalizedGenerationParams`.

- [ ] **Step 2: Add utility tests if a test runner is introduced**

If no frontend test runner is added in this cleanup pass, verify by `npm run build` and backend parity tests; do not add a dependency without approval.

- [ ] **Step 3: Extract auth state**

Move `user`, `adminAccess`, auth form state, captcha loading, login/register/logout, and admin-access refresh into `useAuth.js`.

- [ ] **Step 4: Extract gallery state**

Move gallery list, pagination, sort, my-images list, infinite scroll loading guards, and `mergeImages` into `useGallery.js`.

- [ ] **Step 5: Extract job/SSE state**

Move `sourceFile`, `sourcePreview`, `generationParams`, `submitImageJob`, `watchJob`, `closeEvents`, and upload validation into `useImageJob.js`.

- [ ] **Step 6: Extract view components**

Split template sections into `AuthModal.vue`, `GalleryGrid.vue`, `ImagePreview.vue`, and `ImageWorkbench.vue`. Keep event names explicit: `login`, `register`, `logout`, `submit-job`, `like`, `favorite`, `delete-image`, `hide-image`, `open-preview`.

- [ ] **Step 7: Keep behavior parity**

Run:

```bash
cd client
npm run build
```

Expected: build succeeds. Manual browser smoke should cover login modal, gallery pagination, create/edit form, SSE completion display, preview modal, like/favorite, and admin hide/delete buttons.

## Task 6: Split AdminView Into Focused Frontend Modules

**Files:**
- Create: `client/src/composables/useAdminData.js`
- Create: `client/src/components/admin/AdminLogin.vue`
- Create: `client/src/components/admin/AdminStats.vue`
- Create: `client/src/components/admin/ProviderSettings.vue`
- Create: `client/src/components/admin/UserTable.vue`
- Create: `client/src/components/admin/GenerationTable.vue`
- Modify: `client/src/components/AdminView.vue`

- [ ] **Step 1: Extract admin polling and loading**

Move `bootstrap`, `loadAdminData`, `startAutoRefresh`, `stopAutoRefresh`, and shared `message/saving/busyItem` state into `useAdminData.js`.

- [ ] **Step 2: Extract provider form**

Move `defaultProvider`, `saveProvider`, `editProvider`, and `newProvider` into `ProviderSettings.vue` with props for providers and current form.

- [ ] **Step 3: Extract tables**

Move users table and generation table into dedicated components. Keep row actions emitted upward so API calls remain centralized in `useAdminData.js`.

- [ ] **Step 4: Verify admin build**

Run:

```bash
cd client
npm run build
```

Expected: build succeeds. Manual browser smoke should cover admin password login, provider save preserving key when API key is blank, concurrency update, user admin toggle, generation hide/delete.

## Task 7: Consolidate CSS And Remove Dead Selectors

**Files:**
- Inspect: `client/src/assets/styles.css`
- Inspect: `client/src/assets/tech-theme.css`
- Modify: `client/src/main.js`
- Modify or split: `client/src/assets/*.css`

- [x] **Step 1: Inventory selectors**

Run:

```bash
cd client
npx vite build
```

Expected: production CSS generated. Use browser smoke and source search before deleting selectors.

- [ ] **Step 2: Split by surface**

Target structure:

```text
client/src/assets/base.css
client/src/assets/home.css
client/src/assets/admin.css
```

Update `client/src/main.js` imports in that order.

- [x] **Step 3: Remove obsolete duplicate theme rules**

Delete selectors only when they are not present in current Vue templates and not part of shared base styling.

- [ ] **Step 4: Visual verification**

Run local app and capture desktop/mobile checks:

```bash
cd server-go
ADDR=127.0.0.1:8000 GOOS=darwin GOARCH=arm64 go run ./cmd/server
cd ../client
npm run dev
```

Expected smoke targets: homepage desktop, homepage mobile, playground desktop, playground mobile, admin desktop.

## Task 8: Final Integration Gate

**Files:**
- Read: all changed files
- Modify if needed: `README.md`

- [ ] **Step 1: Run full local verification**

Run:

```bash
cd server-go
GOOS=darwin GOARCH=arm64 go test ./...
cd ../client
npm run build
```

Expected: both pass.

- [ ] **Step 2: Run API smoke**

Start server with temp DB:

```bash
tmpdb="$(mktemp /tmp/img2gallery-smoke-XXXXXX.db)"
tmpimg="$(mktemp -d /tmp/img2gallery-smoke-images-XXXXXX)"
cd server-go
DATABASE_PATH="$tmpdb" IMAGE_STORAGE_DIR="$tmpimg" ADDR=127.0.0.1:8000 GOOS=darwin GOARCH=arm64 go run ./cmd/server
```

In a second terminal:

```bash
curl -i http://127.0.0.1:8000/health
curl -i http://127.0.0.1:8000/api/images
curl -i http://127.0.0.1:8000/api/auth/captcha
```

Expected: health and public gallery endpoints return `200`; captcha returns token and data URL.

- [ ] **Step 3: Check repository cleanliness**

Run:

```bash
git status --short
```

Expected: only planned source/docs changes remain. Runtime DB, generated images, `client/dist`, `client/node_modules`, and approved screenshots are not staged.

- [ ] **Step 4: Update README only if behavior changed**

README should mention:

- backend test command may need `GOOS=darwin GOARCH=arm64` on this machine if global Go env is set to cross-compile;
- model API credentials stay in admin UI/SQLite, not `.env`;
- local runtime data should remain untracked.

## Recommended Order

1. Task 1 and Task 2: establish baseline and explicitly decide artifact cleanup.
2. Task 3: add tests before structural changes.
3. Task 4: type backend DTOs and remove weak string/map contracts.
4. Task 5 and Task 6: split frontend modules after backend payloads are stable.
5. Task 7: CSS cleanup after templates are split.
6. Task 8: final integration gate.

## Quality Gate

Do not claim completion until these commands have run and their result is reported:

```bash
cd server-go && GOOS=darwin GOARCH=arm64 go test ./...
cd client && npm run build
git status --short
```

If browser-visible frontend files changed, also verify desktop and mobile layouts in the in-app browser.
