#!/usr/bin/env bash
set -euo pipefail

echo "[build] building t6-assets for local platform"
go build -ldflags="-s -w" -o t6-assets ./cmd/t6-assets
sudo mv ./t6-assets /usr/local/bin/t6-assets
echo "[build] done: ./t6-assets"
