# 「货殖」运行时镜像：二进制与前端产物均由 CI 预编译后拼装。
#
# 编译期依赖（node_modules、Go 工具链）全部留在 CI，不进镜像。
# 前端产物不打进二进制，运行时由 HZ_STATIC_DIR 指向 /app/static。
#
# 容器内布局：
#   /app/huozhi-server   服务二进制
#   /app/config.yaml     配置文件（镜像内置，可用环境变量覆盖各项）
#   /app/static/         前端构建产物
#   /app/data/           唯一数据卷：SQLite 数据库、上传附件、JWT 密钥
FROM alpine:3.24 AS runtime

LABEL org.opencontainers.image.authors="huozhi"
LABEL description="Huozhi Personal Finance App (Go + SvelteKit 单进程)"

ENV TZ=Asia/Shanghai \
    GIN_MODE=release \
    HZ_UPLOAD_PATH=/app/data/uploads \
    HZ_STATIC_DIR=/app/static

# 单进程 Go 服务，前端静态文件由后端直接托管，无需 nginx
RUN apk add --no-cache ca-certificates tzdata curl \
    && cp /usr/share/zoneinfo/${TZ} /etc/localtime \
    && echo ${TZ} > /etc/timezone \
    && mkdir -p /app/data

WORKDIR /app

COPY --chmod=755 bin/huozhi-server /app/huozhi-server
# 前端产物（SvelteKit adapter-static 输出到 build/）
COPY dist /app/static

# 后端配置（默认 sqlite，可通过环境变量切 postgres）
COPY backend/config.example.yaml /app/config.yaml

COPY scripts/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -fsS http://127.0.0.1:8080/api/health || exit 1

EXPOSE 8080
VOLUME ["/app/data"]

ENTRYPOINT ["/app/entrypoint.sh"]
