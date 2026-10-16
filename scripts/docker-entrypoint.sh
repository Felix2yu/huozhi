#!/bin/sh
set -eu

# ===== 准备目录（唯一数据卷：/app/data）=====
mkdir -p /app/data

# ===== 后端配置 =====
CONFIG_FILE="/app/config.yaml"

# 数据库等配置通过环境变量覆盖（Go 端 config.Load 已支持）。

# 生成默认的 JWT secret（如未设置），持久化到数据卷
if [ -z "${HZ_JWT_SECRET:-}" ]; then
  if [ -f "/app/data/.jwt_secret" ]; then
    HZ_JWT_SECRET=$(cat /app/data/.jwt_secret)
  else
    HZ_JWT_SECRET=$(tr -dc 'A-Za-z0-9._~-' </dev/urandom | head -c 48)
    echo "$HZ_JWT_SECRET" > /app/data/.jwt_secret
    chmod 600 /app/data/.jwt_secret
  fi
  export HZ_JWT_SECRET
fi

echo "[entrypoint] 启动 Huozhi..."
echo "  - DB_DRIVER:  ${HZ_DB_DRIVER:-sqlite}"
echo "  - JWT_ISSUER: ${HZ_JWT_ISSUER:-huozhi}"
echo "  - TIMEZONE:   ${TZ:-Asia/Shanghai}"
echo "  - UPLOAD_DIR: ${HZ_UPLOAD_PATH:-未设置（用 config 中的 upload.path）}"
echo "  - STATIC_DIR: ${HZ_STATIC_DIR:-未设置（仅 API 模式）}"

# ===== 前台启动 Go 后端（前端静态文件由其直接托管，容器内无 nginx） =====
export HZ_PORT=8080
export HZ_SERVER_MODE=release
exec /app/huozhi-server -c "$CONFIG_FILE"
