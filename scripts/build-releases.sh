#!/usr/bin/env bash
set -euo pipefail

OUT_DIR="$(pwd)/dist"
mkdir -p "${OUT_DIR}"

build_target() {
  local goos="$1"
  local goarch="$2"
  local name="$3"
  local arch_label="$4"
  local ext="$5"
  
  local output="${OUT_DIR}/t6-assets-${name}-${arch_label}${ext}"
  echo "[build] ${output}"
  GOOS="${goos}" GOARCH="${goarch}" go build -ldflags="-s -w" -o "${output}" ./cmd/t6-assets
}

# Build all platforms
build_target linux amd64 linux x64 ""
build_target linux arm64 linux arm64 ""
build_target darwin amd64 darwin x64 ""
build_target darwin arm64 darwin arm64 ""
build_target windows amd64 win32 x64 ".exe"

echo "[build] all releases built in ${OUT_DIR}/"
ls -la "${OUT_DIR}/"
