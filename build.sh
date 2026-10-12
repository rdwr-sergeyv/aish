#!/usr/bin/env bash
# Build static Linux binaries into dist/. Works from Linux, WSL or Git Bash.
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p dist
for arch in amd64 arm64; do
  GOOS=linux GOARCH=$arch CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "dist/aish-linux-$arch" .
done
ls -l dist
