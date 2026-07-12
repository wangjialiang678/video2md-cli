#!/usr/bin/env bash
# 构建三平台二进制 → 打进 skill 包 → 上传 SuperBrain SkillHub。
# 同事上不了 GitHub，所以二进制随 skill 包一起分发，装完即用。
#
# 需要 HUB_URL / HUB_WRITE_TOKEN（缺省从 ~/.claude/api-vault.env 读取）。
# 只想打包不上传：  ./scripts/pack-skill.sh --no-upload
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL_DIR="$ROOT_DIR/skills/video2md"
VAULT="$HOME/.claude/api-vault.env"
UPLOAD=1
[[ "${1:-}" == "--no-upload" ]] && UPLOAD=0

load() {
  local key="$1" value="${!1:-}"
  [[ -n "$value" ]] && { echo "$value"; return; }
  [[ -f "$VAULT" ]] && grep -E "^(export )?$key=" "$VAULT" | tail -1 | sed -E "s/^(export )?$key=//; s/^\"//; s/\"$//" || true
}

echo "==> 构建二进制"
mkdir -p "$SKILL_DIR/bin"
( cd "$ROOT_DIR"
  GOOS=darwin  GOARCH=arm64 go build -ldflags="-s -w" -o "$SKILL_DIR/bin/mp4-md-darwin-arm64"      ./cmd/mp4-md
  GOOS=darwin  GOARCH=amd64 go build -ldflags="-s -w" -o "$SKILL_DIR/bin/mp4-md-darwin-amd64"      ./cmd/mp4-md
  GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "$SKILL_DIR/bin/mp4-md-windows-amd64.exe" ./cmd/mp4-md )
chmod +x "$SKILL_DIR/bin/"* "$SKILL_DIR/scripts/video2md.sh"

VERSION="$(sed -n '/^---$/,/^---$/p' "$SKILL_DIR/SKILL.md" | grep '^version:' | head -1 | cut -d: -f2- | xargs)"
echo "==> 打包 skill v${VERSION}"
TMP="$(mktemp -d)"
tar -czf "$TMP/video2md.tar.gz" -C "$(dirname "$SKILL_DIR")" "$(basename "$SKILL_DIR")"
echo "    $TMP/video2md.tar.gz  ($(du -h "$TMP/video2md.tar.gz" | cut -f1))"

if [[ "$UPLOAD" -eq 0 ]]; then
  echo "==> --no-upload，包留在 $TMP/video2md.tar.gz"
  exit 0
fi

HUB_URL="$(load HUB_URL)"
HUB_WRITE_TOKEN="$(load HUB_WRITE_TOKEN)"
[[ -z "$HUB_URL" ]] && { echo "缺少 HUB_URL" >&2; exit 1; }
[[ -z "$HUB_WRITE_TOKEN" ]] && { echo "缺少 HUB_WRITE_TOKEN（上传密码）" >&2; exit 1; }

echo "==> 上传到 $HUB_URL"
curl -fsSL --noproxy '*' -X POST "$HUB_URL/api/skill" \
  -H "X-Write-Token: $HUB_WRITE_TOKEN" \
  -F "file=@$TMP/video2md.tar.gz" \
  -F "author=michael" \
  -F "platform=universal" \
  -F "category=content" \
  -F "tags=视频转文字,转写,说话人分离,ASR" \
  -F "changelog=${CHANGELOG:-v${VERSION}}"
echo
rm -rf "$TMP"
echo "==> 完成"
