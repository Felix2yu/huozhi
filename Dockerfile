# 「货殖」运行时镜像：二进制与前端产物均由 CI 预编译后拼装，
# 前端经 go:embed 内嵌进二进制，镜像里不再单独放一份静态文件。
#
# 编译期依赖（node_modules、Go 工具链）全部留在 CI，不进镜像。
#
# 基镜 ghcr.io/felix2yu/base:alpine-3.24 提供 ca-certificates、tzdata、su-exec。
# 本仓库仍保留自己的 entrypoint.sh（要生成并持久化 JWT secret），它在最后调用
# 基镜的降权入口完成 PUID/PGID 切换。
#
# 容器内布局：
#   /app/huozhi-server   服务二进制（含内嵌前端）
#   /app/config.yaml     配置文件（镜像内置，可用环境变量覆盖各项）
#   /app/data/           唯一数据卷：SQLite 数据库、上传附件、JWT 密钥
FROM ghcr.io/felix2yu/base:alpine-3.24

LABEL org.opencontainers.image.authors="huozhi"
LABEL description="Huozhi Personal Finance App (Go + SvelteKit 单进程)"

ENV TZ=Asia/Shanghai \
    GIN_MODE=release \
    HZ_UPLOAD_PATH=/app/data/uploads \
    APP_DATA_DIR=/app/data

RUN mkdir -p /app/data

WORKDIR /app

COPY --chmod=755 bin/huozhi-server /app/huozhi-server

# 后端配置（默认 sqlite，可通过环境变量切 postgres）
COPY config.example.yaml /app/config.yaml

COPY --chmod=755 scripts/docker-entrypoint.sh /app/entrypoint.sh

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/api/health >/dev/null || exit 1

EXPOSE 8080
VOLUME ["/app/data"]

ENTRYPOINT ["/app/entrypoint.sh"]
