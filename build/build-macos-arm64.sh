#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="$ROOT_DIR/dist"

mkdir -p "$DIST_DIR"

GOOS=darwin GOARCH=arm64 go build \
  -trimpath \
  -ldflags "-s -w" \
  -o "$DIST_DIR/suna-darwin-arm64" \
  "$ROOT_DIR"

(
  cd "$DIST_DIR"
  rm -f "suna-darwin-arm64.zip"
  zip -9 "suna-darwin-arm64.zip" "suna-darwin-arm64"
)

ls -lh "$DIST_DIR/suna-darwin-arm64" "$DIST_DIR/suna-darwin-arm64.zip"
