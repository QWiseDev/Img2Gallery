# Img2Gallery 使用文档

Img2Gallery 是一个 Vue + FastAPI + SQLite 的 AI 生图画廊。它支持账号注册登录、图片验证码、队列生图、首页画廊、个人生成记录、点赞收藏、管理员后台、模型提供商配置和本地图片存储。

## 功能概览

- 账号密码注册 / 登录，登录和注册都需要图片验证码。
- 未登录用户可以浏览画廊，但必须登录后才能提交提示词生成图片。
- 生图任务进入队列，前端实时显示排队位置和生成状态。
- 成功生成的图片保存到本地目录，SQLite 记录提示词、图片地址、模型、状态、请求 IP 等信息。
- 首页画廊支持最新、最多点赞、我的收藏筛选。
- 图片可点击打开大图弹窗，弹窗内支持查看提示词、复制提示词、打开原图。
- 登录用户可查看自己的生成记录，并从记录中打开图片。
- 管理后台支持用户管理、设置用户管理员、查看生成记录、删除画廊作品、配置模型提供商和并发数。

## 环境变量

项目根目录只需要配置管理员密码。不要把模型 API 地址或 API Key 写入环境文件，模型配置请在管理后台维护。

```bash
cp .env.example .env
```

`.env` 示例：

```env
ADMIN_PASSWORD=change-this-admin-password
```

生产环境请务必改成强密码。

## Docker 部署

`docker-compose.yml` 默认使用 GitHub Actions 自动构建的线上镜像：

```text
ghcr.io/qwisedev/img2gallery:latest
```

启动：

```bash
cp .env.example .env
docker compose pull
docker compose up -d
```

访问：

```text
http://localhost:8000
```

更新到最新镜像：

```bash
docker compose pull
docker compose up -d
```

数据持久化：

- SQLite 数据库保存到容器内 `/data/app.db`
- 图片保存到容器内 `/data/images`
- `docker-compose.yml` 默认使用 `img2gallery-data` named volume 持久化 `/data`

## 本地开发

后端：

```bash
cd server
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
cd ..
cp .env.example .env
cd server
uvicorn app.main:app --reload --host 127.0.0.1 --port 8000
```

前端：

```bash
cd client
npm install
cp .env.example .env
npm run dev
```

访问：

```text
http://127.0.0.1:5173
```

Docker 镜像内前端默认使用同源 API，也就是访问 `http://服务器IP:8000` 时会请求同一个域名下的 `/api/...`，不需要额外配置 `VITE_API_BASE`。

## 管理后台

访问：

```text
http://127.0.0.1:5173/admin
```

管理员登录密码来自根目录 `.env` 的 `ADMIN_PASSWORD`。

后台可配置：

- 默认并发数
- 模型提供商名称
- API 地址
- API Key
- 模型名称，默认可配置为 `gpt-image-2`
- 用户是否为管理员

模型服务信息只保存在 SQLite 中，不写入 `.env` 和源码，避免开源泄露密钥。

## 生图使用流程

1. 在首页点击“登录 / 注册”。
2. 输入账号、密码和图片验证码。
3. 登录后在“开启创意之旅”输入提示词。
4. 点击“立即生成”。
5. 页面会显示排队位置、生成中、完成或失败状态。
6. 生成完成后图片会进入首页画廊，并出现在“我的生成记录”中。

## 画廊和图片预览

- 点击画廊图片可打开大图弹窗。
- 弹窗显示大图、提示词、作者和时间。
- 点击“复制提示词”可复制当前图片提示词。
- 点击“打开原图”可在新标签页打开本地图片。
- 按 `Esc` 或点击右上角关闭按钮可关闭弹窗。

## 账号和验证码

- 登录和注册都必须填写图片验证码。
- 验证码由后端 `/api/auth/captcha` 生成，一次性使用，过期后需要刷新。
- 未登录用户点击生成按钮会自动打开登录弹窗。

## GitHub Actions 镜像

推送到 `main` 后会自动构建并推送 Docker 镜像到 GitHub Container Registry：

```text
ghcr.io/qwisedev/img2gallery
```

镜像标签包含：

- `latest`
- `sha-<commit>`

对应 workflow 文件：

```text
.github/workflows/docker-image.yml
```
