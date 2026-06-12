#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# 同一次 release 使用同一个版本字符串，避免不同平台包显示的 dev 时间不一致。
export SUNA_BUILD_VERSION="${SUNA_BUILD_VERSION:-dev+$(date -u '+%Y-%m-%dT%H:%M:%SZ')}"

"$ROOT_DIR/build/build-darwin.sh"
"$ROOT_DIR/build/build-linux.sh"
"$ROOT_DIR/build/build-windows.sh"

ls -lh "$ROOT_DIR/dist"
