# ===== Stage 1: 构建 Huozhi Server (Go) =====
FROM golang:1.27-alpine AS go-builder
LABEL stage=go-builder

WORKDIR /src/backend
ENV CGO_ENABLED=1 GOOS=linux

# CGO for sqlite
RUN apk add --no-cache build-base git ca-certificates tzdata

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN go build -trimpath -ldflags="-s -w" \
    -o /out/huozhi-server ./cmd/huozhi-server


# ===== Stage 2: 构建 Frontend (React + Vite) =====
FROM node:26-alpine AS fe-builder
LABEL stage=fe-builder
WORKDIR /src/frontend

COPY frontend/package.json frontend/package-lock.json* ./
COPY frontend/scripts scripts
RUN npm install --no-audit --no-fund --registry=https://registry.npmmirror.com || \
    npm install --no-audit --no-fund

COPY frontend/ ./
ENV NODE_ENV=production
RUN npm run build


# ===== Stage 3: Runtime =====
# 容器内统一布局：应用文件全部收敛在 /app 下
#   /app/huozhi-server   服务二进制
#   /app/config.yaml     配置文件（镜像内置，可用环境变量覆盖各项）
#   /app/static/         前端构建产物（镜像内置，随镜像升级替换）
#   /app/data/           唯一数据卷：SQLite 数据库、上传附件、JWT 密钥
FROM alpine:3.24 AS runtime
LABEL org.opencontainers.image.authors="huozhi"
LABEL description="Huozhi Personal Finance App (Go + React 单进程)"

ENV TZ=Asia/Shanghai \
    GIN_MODE=release \
    HZ_UPLOAD_PATH=/app/data/uploads \
    HZ_STATIC_DIR=/app/static

# 依赖（单进程 Go 服务，前端静态文件由后端直接托管，无需 nginx）
RUN apk add --no-cache ca-certificates tzdata curl \
    && cp /usr/share/zoneinfo/${TZ} /etc/localtime \
    && echo ${TZ} > /etc/timezone \
    && mkdir -p /app/data

WORKDIR /app

# 拷贝 Server 二进制
COPY --from=go-builder /out/huozhi-server /app/huozhi-server
RUN chmod +x /app/huozhi-server

# 拷贝前端产物
COPY --from=fe-builder /src/frontend/dist /app/static

# 后端配置（默认 sqlite，可通过环境变量切 postgres）
COPY backend/config.example.yaml /app/config.yaml

# 启动脚本
COPY scripts/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -fsS http://127.0.0.1:8080/api/health || exit 1

EXPOSE 8080
VOLUME ["/app/data"]

ENTRYPOINT ["/app/entrypoint.sh"]
