FROM node:22-alpine AS frontend

WORKDIR /app/client
COPY client/package*.json ./
RUN npm ci
COPY client/ ./
RUN npm run build

FROM python:3.12-slim AS runtime

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PYTHONPATH=/app/server \
    DATABASE_PATH=/data/app.db \
    IMAGE_STORAGE_DIR=/data/images \
    CLIENT_ORIGIN=http://localhost:8000 \
    TZ=Asia/Shanghai \
    APP_TIMEZONE=Asia/Shanghai

WORKDIR /app

RUN useradd --create-home --shell /usr/sbin/nologin appuser \
    && mkdir -p /data/images \
    && chown -R appuser:appuser /data

COPY server/requirements.txt ./server/requirements.txt
RUN pip install --no-cache-dir -r server/requirements.txt

COPY server/app ./server/app
COPY --from=frontend /app/client/dist ./client/dist

RUN chown -R appuser:appuser /app

USER appuser
EXPOSE 8000

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
