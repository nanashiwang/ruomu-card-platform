#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
export ENV_FILE="${ENV_FILE:-$ROOT_DIR/.env}"
export COMPOSE_FILE="${COMPOSE_FILE:-$ROOT_DIR/deploy/docker-compose.prod.yml}"

if ! command -v python3 >/dev/null 2>&1; then
  echo "缺少 python3 命令（部署检查使用 Python 3.9+ 标准库）" >&2
  exit 1
fi
if [[ "${1:-}" != "" && "${1:-}" != "--check" ]]; then
  echo "用法：./scripts/deploy.sh [--check]" >&2
  exit 1
fi

python3 scripts/check-deployment.py
if [[ "${1:-}" == "--check" ]]; then
  exit 0
fi

docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d --build --wait --wait-timeout 180
# 公网 DNS / 可信 TLS / 回调路径一并检查；失败不声称部署成功，也不删除已有数据。
python3 scripts/check-deployment.py --online
echo "生产部署与 HTTPS 路由验证完成。请在管理后台核对渠道并完成真实支付验收。"
