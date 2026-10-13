#!/usr/bin/env bash
# Build ODM. Default: Windows amd64 executable in ./dist.
#
# The Windows UI uses Fyne, which needs cgo, so a Windows cross-build requires a
# mingw-w64 toolchain (CC defaults to x86_64-w64-mingw32-gcc). Override with the
# CC_WINDOWS environment variable.
set -euo pipefail
cd "$(dirname "$0")/.."

OUT="${OUT:-dist}"
mkdir -p "$OUT"

build() {
  local goos="$1" goarch="$2" name="$3" ldflags="-s -w" cgo=0
  if [ "$goos" = "windows" ]; then
    # GUI subsystem: no console window.
    ldflags="-s -w -H=windowsgui"
    cgo=1
  fi
  echo "building $goos/$goarch -> $OUT/$name"
  if [ "$cgo" = "1" ]; then
    CGO_ENABLED=1 CC="${CC_WINDOWS:-x86_64-w64-mingw32-gcc}" \
      GOOS="$goos" GOARCH="$goarch" \
      go build -trimpath -ldflags "$ldflags" -o "$OUT/$name" ./cmd/odm
  else
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
      go build -trimpath -ldflags "$ldflags" -o "$OUT/$name" ./cmd/odm
  fi
  echo "built $OUT/$name"
}

case "${1:-windows}" in
  windows) build windows amd64 ODM.exe ;;
  linux) build linux amd64 odm ;;
  darwin) build darwin arm64 odm ;;
  all)
    build windows amd64 ODM.exe
    build linux amd64 odm
    build darwin arm64 odm
    ;;
  *) echo "usage: $0 [windows|linux|darwin|all]" >&2; exit 2 ;;
esac
