#!/usr/bin/env bash
# Cross-platform build for the meowflix daemon.
#
# Usage:
#   scripts/build.sh                 # build for the host platform
#   scripts/build.sh all             # build for windows/linux/darwin (amd64+arm64)
#   GOOS=windows GOARCH=amd64 scripts/build.sh   # build a single explicit target
#
# Output binaries land in dist/.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

PKG="./cmd/meowflix"
OUT_DIR="dist"
mkdir -p "$OUT_DIR"

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo none)}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

LDFLAGS="-s -w \
  -X main.version=${VERSION} \
  -X main.commit=${COMMIT} \
  -X main.buildDate=${BUILD_DATE}"

build_one() {
  local goos="$1" goarch="$2"
  local ext=""
  [ "$goos" = "windows" ] && ext=".exe"
  local out="${OUT_DIR}/meowflix-${goos}-${goarch}${ext}"
  echo "==> building ${out} (version=${VERSION})"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
    go build -trimpath -ldflags "$LDFLAGS" -o "$out" "$PKG"
}

case "${1:-host}" in
  all)
    build_one windows amd64
    build_one windows arm64
    build_one linux   amd64
    build_one linux   arm64
    build_one darwin  amd64
    build_one darwin  arm64
    ;;
  host)
    build_one "$(go env GOOS)" "$(go env GOARCH)"
    ;;
  *)
    # Respect explicitly exported GOOS/GOARCH for a single custom target.
    build_one "$(go env GOOS)" "$(go env GOARCH)"
    ;;
esac

echo "==> done. artifacts in ${OUT_DIR}/"
ls -la "$OUT_DIR"
