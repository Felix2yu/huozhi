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
RUN npm install --no-audit --no-fund --registry=https://registry.npmmirror.com || \
    npm install --no-audit --no-fund

COPY frontend/ ./
ENV NODE_ENV=production
RUN npm run build


# ===== Stage 3: Runtime =====
FROM alpine:3.24 AS runtime
LABEL org.opencontainers.image.authors="huozhi"
LABEL description="Huozhi Personal Finance App (Go + React + Nginx)"

ENV TZ=Asia/Shanghai \
    GIN_MODE=release

# 依赖
RUN apk add --no-cache ca-certificates tzdata curl nginx nginx-mod-http-headers-more \
    && cp /usr/share/zoneinfo/${TZ} /etc/localtime \
    && echo ${TZ} > /etc/timezone \
    && mkdir -p /var/lib/huozhi/data /var/lib/huozhi/uploads /var/www/html \
    && chown -R nginx:nginx /var/lib/huozhi /var/www/html /var/lib/nginx /var/log/nginx /run/nginx

# 拷贝 Server 二进制
COPY --from=go-builder /out/huozhi-server /usr/local/bin/huozhi-server
RUN chmod +x /usr/local/bin/huozhi-server

# 拷贝前端产物
COPY --from=fe-builder /src/frontend/dist /var/www/html

# Nginx 配置
COPY docker/nginx/nginx.conf /etc/nginx/nginx.conf
COPY docker/nginx/conf.d/default.conf /etc/nginx/http.d/default.conf

# 后端配置（默认 sqlite，可通过环境变量切 postgres）
COPY backend/config.example.yaml /etc/huozhi/config.yaml

# 启动脚本
COPY scripts/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -fsS http://127.0.0.1:8080/api/health || exit 1

EXPOSE 80 443
VOLUME ["/var/lib/huozhi"]

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
