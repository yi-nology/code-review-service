#!/usr/bin/env bash
# 从"母版"（CRF 19）压出投放版：1080p 网页友好码率，带 faststart。
# 母版保留归档，投放版用于上传（B站/视频号/官网都会再转一次码）。
set -euo pipefail
cd "$(dirname "$0")/.."
OUT=out
for key in "$@"; do
  src="$OUT/${key}-silent.mp4"
  [ -f "$src" ] || { echo "缺少母版：$src"; exit 1; }
  dst="$OUT/review-service-${key}.mp4"
  ffmpeg -y -v error -i "$src" -i "$OUT/audio-${key}/mixed.wav" \
    -c:v libx264 -preset medium -crf 26 -maxrate 7M -bufsize 14M \
    -pix_fmt yuv420p -c:a aac -b:a 160k -shortest -movflags +faststart "$dst"
  printf '%s  %s\n' "$dst" "$(du -h "$dst" | cut -f1)"
done
