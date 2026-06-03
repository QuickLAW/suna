#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="$ROOT_DIR/dist"

mkdir -p "$DIST_DIR"

BUILD_TIME="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"

GOOS=windows GOARCH=amd64 go build \
  -trimpath \
  -ldflags "-s -w -X 'github.com/alanchenchen/suna/internal/tui.appVersion=dev+$BUILD_TIME'" \
  -o "$DIST_DIR/suna-windows-amd64.exe" \
  "$ROOT_DIR"

(
  cd "$DIST_DIR"
  rm -f "suna-windows-amd64.zip"
  zip -9 "suna-windows-amd64.zip" "suna-windows-amd64.exe"
)

ls -lh "$DIST_DIR/suna-windows-amd64.exe" "$DIST_DIR/suna-windows-amd64.zip"
