#!/usr/bin/env bash
# video2md skill 的统一入口：定位随包二进制 → 加载凭证 → 前置检查 → 执行。
# 不依赖任何固定的 AI 环境目录，Claude Code / Codex / WorkBuddy 装到哪都能跑。
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${VIDEO2MD_ENV_FILE:-$HOME/.video2md-cli.env}"

# 1) 选二进制：优先随包携带的，其次系统里已装的
pick_binary() {
  if [[ -n "${VIDEO2MD_CLI_BIN:-}" && -x "${VIDEO2MD_CLI_BIN}" ]]; then
    echo "$VIDEO2MD_CLI_BIN"; return
  fi
  local os arch name
  os="$(uname -s)"; arch="$(uname -m)"
  case "$os" in
    Darwin) case "$arch" in
              arm64|aarch64) name="mp4-md-darwin-arm64" ;;
              x86_64)        name="mp4-md-darwin-amd64" ;;
            esac ;;
    Linux)  name="mp4-md-linux-amd64" ;;
    MINGW*|MSYS*|CYGWIN*) name="mp4-md-windows-amd64.exe" ;;
  esac
  if [[ -n "${name:-}" && -f "$SKILL_DIR/bin/$name" ]]; then
    chmod +x "$SKILL_DIR/bin/$name" 2>/dev/null || true
    echo "$SKILL_DIR/bin/$name"; return
  fi
  if [[ -x "$HOME/.video2md-cli/bin/mp4-md" ]]; then
    echo "$HOME/.video2md-cli/bin/mp4-md"; return
  fi
  command -v mp4-md 2>/dev/null || true
}

BIN="$(pick_binary)"
if [[ -z "$BIN" ]]; then
  echo "找不到可用的 mp4-md 二进制（当前系统：$(uname -s)/$(uname -m)）。" >&2
  echo "此 skill 包内置 macOS 与 Windows 版本；其它平台请从源码构建：" >&2
  echo "  https://github.com/wangjialiang678/video2md-cli" >&2
  exit 127
fi

# 2) 前置依赖：ffmpeg（本地转码用，必须有）
if ! command -v ffmpeg >/dev/null 2>&1 && [[ -z "${MP4MD_FFMPEG:-}" ]]; then
  echo "缺少 ffmpeg —— 本工具在本地抽取音频需要它。" >&2
  echo "  macOS:   brew install ffmpeg" >&2
  echo "  Windows: winget install Gyan.FFmpeg" >&2
  exit 127
fi

# 3) 凭证：只需要一个 DASHSCOPE_API_KEY
if [[ -f "$ENV_FILE" ]]; then
  set -a; set +u
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set -u; set +a
fi

if [[ -z "${DASHSCOPE_API_KEY:-}" ]]; then
  cat >&2 <<EOF
缺少 DASHSCOPE_API_KEY。写入 $ENV_FILE 即可：

  echo 'export DASHSCOPE_API_KEY=sk-你的key' > $ENV_FILE
  chmod 600 $ENV_FILE

Key 的两个来源：
  1. 找 Michael 要（团队共用）
  2. 自己申请：https://bailian.console.aliyun.com/ → 右上角 API-KEY
EOF
  exit 1
fi

exec "$BIN" "$@"
