#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
API_DIR="$ROOT_DIR/apps/api"
USER_DIR="$ROOT_DIR/apps/user"
ADMIN_DIR="$ROOT_DIR/apps/admin"
PNPM_VERSION="${PNPM_VERSION:-10.34.3}"

PIDS=""

log() {
  printf '\033[1;32m[ruomu]\033[0m %s\n' "$*"
}

fail() {
  printf '\033[1;31m[ruomu]\033[0m %s\n' "$*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "缺少命令：$1"
}

check_port() {
  local port="$1"
  if command -v lsof >/dev/null 2>&1 && lsof -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    fail "端口 $port 已被占用，请先释放后再启动"
  fi
}

pnpm_app() {
  npx --yes "pnpm@$PNPM_VERSION" "$@"
}

ensure_frontend_deps() {
  local dir="$1"
  if [ ! -d "$dir/node_modules" ]; then
    log "安装依赖：$dir"
    (cd "$dir" && pnpm_app install --frozen-lockfile)
  fi
}

cleanup() {
  for pid in $PIDS; do
    if kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
    fi
  done
}
trap cleanup INT TERM EXIT

need_cmd go
need_cmd node
need_cmd npx

check_port 8080
check_port 5173
check_port 5174

if [ ! -f "$API_DIR/config.yml" ]; then
  log "创建本地 API 配置：apps/api/config.yml"
  cp "$API_DIR/config.ruomu.dev.yml.example" "$API_DIR/config.yml"
fi

ensure_frontend_deps "$USER_DIR"
ensure_frontend_deps "$ADMIN_DIR"

log "启动 API：http://127.0.0.1:8080"
(cd "$API_DIR" && go run cmd/server/main.go -mode api) &
PIDS="$PIDS $!"

if command -v curl >/dev/null 2>&1; then
  for _ in $(seq 1 60); do
    if curl -fsS "http://127.0.0.1:8080/health" >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done
fi

log "启动用户前台：http://127.0.0.1:5173"
(cd "$USER_DIR" && pnpm_app run dev) &
PIDS="$PIDS $!"

log "启动管理后台：http://127.0.0.1:5174"
(cd "$ADMIN_DIR" && pnpm_app run dev) &
PIDS="$PIDS $!"

cat <<'INFO'

若木云卡本地开发服务已启动：
- API 健康检查：http://127.0.0.1:8080/health
- 用户前台：http://127.0.0.1:5173
- 管理后台：http://127.0.0.1:5174
- 本地默认管理员：admin / Admin123456

按 Ctrl+C 停止全部服务。
INFO

while true; do
  for pid in $PIDS; do
    if ! kill -0 "$pid" >/dev/null 2>&1; then
      wait "$pid"
      exit $?
    fi
  done
  sleep 2
done
