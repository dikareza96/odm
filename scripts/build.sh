#!/usr/bin/env bash
# Build ODM. Default: Windows amd64 executable in ./dist.
set -euo pipefail
cd "$(dirname "$0")/.."

OUT="${OUT:-dist}"
mkdir -p "$OUT"

build() {
  local goos="$1" goarch="$2" name="$3" ldflags="-s -w"
  # Windows builds are GUI apps: no console window is shown.
  [ "$goos" = "windows" ] && ldflags="-s -w -H=windowsgui"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags "$ldflags" -o "$OUT/$name" ./cmd/odm
  echo "built $OUT/$name"
}

case "${1:-windows}" in
  windows) build windows amd64 ODM.exe ;;
  windows-arm64) build windows arm64 ODM-arm64.exe ;;
  linux) build linux amd64 odm ;;
  darwin) build darwin arm64 odm ;;
  all)
    build windows amd64 ODM.exe
    build linux amd64 odm
    build darwin arm64 odm
    ;;
  *) echo "usage: $0 [windows|windows-arm64|linux|darwin|all]" >&2; exit 2 ;;
esac
