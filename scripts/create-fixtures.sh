#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FIXTURE_DIR="$ROOT_DIR/fixtures"

mkdir -p "$FIXTURE_DIR"

if ! command -v ffmpeg >/dev/null 2>&1; then
  echo "ffmpeg is required to generate fixtures" >&2
  exit 1
fi

ffmpeg -hide_banner -loglevel error -y \
  -f lavfi -i "sine=frequency=440:duration=15:sample_rate=16000" \
  -ac 1 -ar 16000 "$FIXTURE_DIR/audio-clip-15s.wav"

ffmpeg -hide_banner -loglevel error -y \
  -f lavfi -i "testsrc=size=320x240:rate=15:duration=15" \
  -pix_fmt yuv420p -an "$FIXTURE_DIR/video-sample.mp4"

ffmpeg -hide_banner -loglevel error -y \
  -f lavfi -i "testsrc=size=320x240:rate=15:duration=15" \
  -f lavfi -i "sine=frequency=660:duration=15:sample_rate=16000" \
  -shortest -pix_fmt yuv420p -c:v libx264 -c:a aac -ac 1 -ar 16000 \
  "$FIXTURE_DIR/video-clip-15s-with-audio.mp4"

cp "$FIXTURE_DIR/video-clip-15s-with-audio.mp4" "$FIXTURE_DIR/video-sample-with-audio.mp4"

echo "Generated fixtures in $FIXTURE_DIR"
