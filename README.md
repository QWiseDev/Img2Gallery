# AI 创意画廊

Vue + FastAPI + SQLite 的本地 AI 生图画廊。

## 运行

后端：

```bash
cd server
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
cp ../.env.example ../.env
uvicorn app.main:app --reload --port 8000
```

前端：

```bash
cd client
npm install
npm run dev
```

浏览器访问 `http://localhost:5173`。

## 管理后台

项目根目录 `.env` 只需要配置管理员密码：

```bash
ADMIN_PASSWORD=change-this-admin-password
```

启动后访问 `http://localhost:5173/admin`。模型提供商、API 地址、API Key、默认模型和生成并发数都在管理后台配置。

## Docker

```bash
cp .env.example .env
docker compose up --build
```

容器启动后访问 `http://localhost:8000`。容器内 SQLite 和图片文件保存在 `/data`，`docker-compose.yml` 默认使用 named volume 持久化。

## GitHub Actions

仓库推送到 `main` 后会运行 `.github/workflows/docker-image.yml`，自动构建镜像并推送到：

```text
ghcr.io/qwisedev/img2gallery
```

镜像标签包含 `latest` 和当前提交的 `sha-<commit>`。
