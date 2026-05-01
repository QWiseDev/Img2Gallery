FROM node:22-alpine AS frontend

WORKDIR /app/client
COPY client/package*.json ./
RUN npm ci
COPY client/ ./
RUN npm run build

FROM golang:1.25-alpine AS backend

WORKDIR /app/server-go
COPY server-go/go.mod server-go/go.sum ./
RUN go mod download
COPY server-go/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/img2gallery ./cmd/server

FROM alpine:3.22 AS runtime

ENV ADDR=0.0.0.0:8000 \
    DATABASE_PATH=/data/app.db \
    IMAGE_STORAGE_DIR=/data/images \
    CLIENT_ORIGIN=http://localhost:8000 \
    TZ=Asia/Shanghai \
    APP_TIMEZONE=Asia/Shanghai

WORKDIR /app

RUN adduser -D -H -s /sbin/nologin appuser \
    && mkdir -p /data/images \
    && chown -R appuser:appuser /data

COPY --from=backend /out/img2gallery /app/img2gallery
COPY --from=frontend /app/client/dist /app/client/dist

RUN chown -R appuser:appuser /app

USER appuser
EXPOSE 8000

CMD ["/app/img2gallery"]
