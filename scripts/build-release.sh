#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="$ROOT_DIR/dist/release"
VERSION="${1:-snapshot}"

mkdir -p "$DIST_DIR"

build_unix() {
  local goos="$1"
  local goarch="$2"
  local target="$goos-$goarch"
  local work="$DIST_DIR/video2md-cli-$target"
  rm -rf "$work"
  mkdir -p "$work"
  GOOS="$goos" GOARCH="$goarch" go build -o "$work/mp4-md" ./cmd/mp4-md
  cp scripts/video2md "$work/video2md"
  chmod +x "$work/video2md"
  cp README.md "$work/README.md"
  cp LICENSE "$work/LICENSE"
  cp .env.example "$work/.env.example"
  (cd "$work" && tar -czf "$DIST_DIR/video2md-cli-$target.tar.gz" mp4-md video2md README.md LICENSE .env.example)
}

build_windows() {
  local target="windows-amd64"
  local work="$DIST_DIR/video2md-cli-$target"
  rm -rf "$work"
  mkdir -p "$work"
  GOOS=windows GOARCH=amd64 go build -o "$work/mp4-md.exe" ./cmd/mp4-md
  cp scripts/video2md.ps1 "$work/video2md.ps1"
  cp README.md "$work/README.md"
  cp LICENSE "$work/LICENSE"
  cp .env.example "$work/.env.example"
  (cd "$work" && zip -q -r "$DIST_DIR/video2md-cli-$target.zip" mp4-md.exe video2md.ps1 README.md LICENSE .env.example)
}

cd "$ROOT_DIR"
go test ./...
build_unix darwin arm64
build_unix darwin amd64
build_windows

cat > "$DIST_DIR/checksums-$VERSION.txt" <<EOF
$(cd "$DIST_DIR" && shasum -a 256 video2md-cli-darwin-arm64.tar.gz)
$(cd "$DIST_DIR" && shasum -a 256 video2md-cli-darwin-amd64.tar.gz)
$(cd "$DIST_DIR" && shasum -a 256 video2md-cli-windows-amd64.zip)
EOF

echo "Release artifacts written to $DIST_DIR"
