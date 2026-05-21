#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

"$ROOT_DIR/build/build-macos-arm64.sh"
"$ROOT_DIR/build/build-windows-amd64.sh"

ls -lh "$ROOT_DIR/dist"
