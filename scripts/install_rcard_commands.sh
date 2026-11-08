#!/usr/bin/env bash
set -euo pipefail

SOURCE="${1:-}"
LIB_DIR="${RCARD_LIB_DIR:-/usr/local/lib/ruomu-card-platform}"
BIN_DIR="${RCARD_BIN_DIR:-/usr/local/bin}"
TARGET="$LIB_DIR/container_release.py"
COMMANDS=(rcard rcard-update rcard-stop rcard-start rcard-restart rcard-reopen rcard-uninstall)

if [[ "$(id -u)" -ne 0 ]]; then
  echo "请使用 sudo 安装快捷命令。" >&2
  exit 1
fi
if [[ -z "$SOURCE" || ! -f "$SOURCE" ]]; then
  echo "用法: sudo bash install_rcard_commands.sh /path/to/verified/container_release.py" >&2
  exit 1
fi
SOURCE="$(realpath "$SOURCE")"
if ! head -n 1 "$SOURCE" | grep -q '^#!/usr/bin/env python3$' || ! grep -q 'Pinned three-image release' "$SOURCE"; then
  echo "拒绝安装：目标文件不像固定版本发布工具。" >&2
  exit 1
fi
if [[ -L "$LIB_DIR" || -L "$TARGET" ]]; then
  echo "拒绝写入符号链接目标: $LIB_DIR" >&2
  exit 1
fi
if [[ -e "$TARGET" ]] && ! grep -q 'Pinned three-image release' "$TARGET"; then
  echo "拒绝覆盖非若木发布工具: $TARGET" >&2
  exit 1
fi

for command in "${COMMANDS[@]}"; do
  link="$BIN_DIR/$command"
  if [[ -e "$link" && ! -L "$link" ]]; then
    echo "拒绝覆盖现有非符号链接命令: $link" >&2
    exit 1
  fi
  if [[ -L "$link" && "$(readlink "$link")" != "$TARGET" ]]; then
    echo "拒绝替换指向其他目标的符号链接: $link" >&2
    exit 1
  fi
done

install -d -m 0755 "$LIB_DIR" "$BIN_DIR"
tmp="$LIB_DIR/.container_release.py.$$"
trap 'rm -f "$tmp"' EXIT
install -m 0755 "$SOURCE" "$tmp"
mv -f "$tmp" "$TARGET"
for command in "${COMMANDS[@]}"; do
  ln -sfn "$TARGET" "$BIN_DIR/$command"
done
trap - EXIT
echo "快捷命令已安装: ${COMMANDS[*]}"
