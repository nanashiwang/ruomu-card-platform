#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PNPM_VERSION="${PNPM_VERSION:-10.34.3}"
RUN_TESTS="${RUN_TESTS:-1}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

log() {
  printf '\033[1;32m[ruomu]\033[0m %s\n' "$*"
}

pnpm_app() {
  npx --yes "pnpm@$PNPM_VERSION" "$@"
}

log "构建 API"
(
  cd "$ROOT_DIR/apps/api"
  if [ "$RUN_TESTS" = "1" ]; then
    go test ./...
  fi
  go build -trimpath -o "$TMP_DIR/dujiao-api" ./cmd/server
)

log "构建用户前台"
(
  cd "$ROOT_DIR/apps/user"
  pnpm_app install --frozen-lockfile
  pnpm_app run build
)

log "构建管理后台"
(
  cd "$ROOT_DIR/apps/admin"
  pnpm_app install --frozen-lockfile
  pnpm_app run build
)

log "构建完成"
