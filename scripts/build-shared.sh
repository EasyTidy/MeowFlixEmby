#!/usr/bin/env bash
# Build the MeowFlixEmby C-shared library (FFI) for the host platform.
#
# Requires a C toolchain (CGO_ENABLED=1):
#   Windows: mingw-w64 gcc on PATH
#   Linux:   gcc/clang
#   macOS:   Xcode command line tools
#
# Output: dist/{meowflix.dll|libmeowflix.so|libmeowflix.dylib} + meowflix.h
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

PKG="./api/ffi"
OUT_DIR="dist"
mkdir -p "$OUT_DIR"

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
LDFLAGS="-s -w -X main.version=${VERSION}"

GOOS="$(go env GOOS)"
case "$GOOS" in
  windows) OUT="${OUT_DIR}/meowflix.dll" ;;
  darwin)  OUT="${OUT_DIR}/libmeowflix.dylib" ;;
  *)       OUT="${OUT_DIR}/libmeowflix.so" ;;
esac

echo "==> building shared library ${OUT} (version=${VERSION})"
CGO_ENABLED=1 go build -buildmode=c-shared -trimpath \
  -ldflags "$LDFLAGS" -o "$OUT" "$PKG"

echo "==> done."
ls -la "$OUT" "${OUT%.*}.h" 2>/dev/null || ls -la "$OUT" "${OUT_DIR}/meowflix.h"
