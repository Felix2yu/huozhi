#!/bin/sh
set -eu

# ===== 准备目录 =====
mkdir -p /var/lib/huozhi/data /var/lib/huozhi/uploads
chown -R nginx:nginx /var/lib/huozhi || true

# ===== 替换后端配置占位（若选择 PostgreSQL）=====
CONFIG_FILE="/etc/huozhi/config.yaml"

# 如果 HZ_DB_DRIVER 设置了但配置文件用的是 sqlite，我们直接用环境变量覆盖，
# 因为 Go 端的 config.Load 已经支持环境变量。

# 生成默认的 JWT secret（如未设置）
if [ -z "${HZ_JWT_SECRET:-}" ]; then
  if [ -f "/var/lib/huozhi/.jwt_secret" ]; then
    HZ_JWT_SECRET=$(cat /var/lib/huozhi/.jwt_secret)
  else
    HZ_JWT_SECRET=$(tr -dc 'A-Za-z0-9._~-' </dev/urandom | head -c 48)
    echo "$HZ_JWT_SECRET" > /var/lib/huozhi/.jwt_secret
    chmod 600 /var/lib/huozhi/.jwt_secret
  fi
  export HZ_JWT_SECRET
fi

# 启动日志
echo "[entrypoint] 启动 Huozhi..."
echo "  - DB_DRIVER:  ${HZ_DB_DRIVER:-sqlite}"
echo "  - JWT_ISSUER: ${HZ_JWT_ISSUER:-huozhi}"
echo "  - TIMEZONE:   ${TZ:-Asia/Shanghai}"

# ===== 启动 Go 后端 =====
export HZ_PORT=8080
export HZ_SERVER_MODE=release

/usr/local/bin/huozhi-server -c "$CONFIG_FILE" \
  > /var/log/huozhi-server.log 2>&1 &
SERVER_PID=$!
echo "[entrypoint] backend started (pid=$SERVER_PID)"

# 等待后端就绪
for i in $(seq 1 30); do
  if curl -fsS http://127.0.0.1:8080/api/health >/dev/null 2>&1; then
    break
  fi
  echo "[entrypoint] waiting backend... ($i/30)"
  sleep 1
done

# ===== 启动 Nginx (前台) =====
echo "[entrypoint] 启动 Nginx..."
exec nginx -g "daemon off;"
