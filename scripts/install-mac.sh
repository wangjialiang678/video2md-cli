#!/usr/bin/env bash
set -euo pipefail

REPO="${VIDEO2MD_GITHUB_REPO:-wangjialiang678/video2md-cli}"
INSTALL_DIR="${VIDEO2MD_INSTALL_DIR:-$HOME/.video2md-cli}"
BIN_DIR="$INSTALL_DIR/bin"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# skill 装到所有已存在的 AI 环境：Claude Code / Codex / WorkBuddy
SKILL_DESTS=(
  "$HOME/.claude/skills"
  "${CODEX_HOME:-$HOME/.codex}/skills"
  "$HOME/.agents/skills"
)

arch="$(uname -m)"
case "$arch" in
  arm64|aarch64) target="darwin-arm64" ;;
  x86_64|amd64) target="darwin-amd64" ;;
  *) echo "unsupported Mac architecture: $arch" >&2; exit 1 ;;
esac

mkdir -p "$BIN_DIR"

install_local_binary() {
  local local_bin="$ROOT_DIR/dist/local/mp4-md-$target"
  if [[ -x "$local_bin" ]]; then
    echo "Installing local binary $local_bin"
    install -m 0755 "$local_bin" "$BIN_DIR/mp4-md"
    return 0
  fi
  return 1
}

download_release() {
  local url="https://github.com/$REPO/releases/latest/download/video2md-cli-$target.tar.gz"
  local tmp
  tmp="$(mktemp -d /tmp/video2md-install-XXXXXX)"
  echo "Downloading $url"
  if ! curl -fsSL "$url" -o "$tmp/video2md.tar.gz"; then
    rm -rf "$tmp"
    return 1
  fi
  tar -xzf "$tmp/video2md.tar.gz" -C "$tmp"
  install -m 0755 "$tmp/mp4-md" "$BIN_DIR/mp4-md"
  rm -rf "$tmp"
}

build_local() {
  if ! command -v go >/dev/null 2>&1; then
    echo "Release download failed and Go is not installed. Install from GitHub Releases or install Go, then rerun." >&2
    exit 1
  fi
  echo "Building local CLI with Go"
  (cd "$ROOT_DIR" && go build -o "$BIN_DIR/mp4-md" ./cmd/mp4-md)
}

if install_local_binary; then
  :
elif ! download_release; then
  build_local
fi
install -m 0755 "$ROOT_DIR/scripts/video2md" "$BIN_DIR/video2md"

if [[ -d "$ROOT_DIR/skills/video2md" ]]; then
  for skills_dir in "${SKILL_DESTS[@]}"; do
    # 只装到已经存在的 AI 环境，不给用户凭空造目录
    [[ -d "$skills_dir" ]] || continue
    dest="$skills_dir/video2md"
    rm -rf "$dest"
    cp -R "$ROOT_DIR/skills/video2md" "$dest"
    chmod +x "$dest/scripts/video2md.sh"
    [[ -d "$dest/bin" ]] && chmod +x "$dest/bin/"* 2>/dev/null || true
    echo "Installed skill to $dest"
  done
fi

if [[ ! -f "$HOME/.video2md-cli.env" ]]; then
  cp "$ROOT_DIR/.env.example" "$HOME/.video2md-cli.env"
  chmod 0600 "$HOME/.video2md-cli.env"
fi

if ! command -v ffmpeg >/dev/null 2>&1 && [[ -z "${MP4MD_FFMPEG:-}" ]]; then
  echo "Warning: ffmpeg was not found. Install it with: brew install ffmpeg"
fi

echo "Installed mp4-md to $BIN_DIR/mp4-md"
echo "Installed wrapper to $BIN_DIR/video2md"
echo "Set DASHSCOPE_API_KEY in $HOME/.video2md-cli.env  (that is the only required credential)"
echo "Restart your AI tool to pick up the skill."
