#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-deploy/docker-compose.prod.yml}"

cd "$ROOT_DIR"

if ! command -v docker >/dev/null 2>&1; then
  echo "缺少 docker 命令" >&2
  exit 1
fi

if [ ! -f ".env" ]; then
  echo "未发现 .env。生产部署前请先执行：cp .env.example .env，并修改密钥、域名和端口。" >&2
  exit 1
fi

docker compose --env-file .env -f "$COMPOSE_FILE" up -d --build
