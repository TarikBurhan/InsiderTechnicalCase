#!/bin/sh
# Runs a go command inside the official Go image (for machines without Go).
# Usage: ./scripts/go-docker.sh go test ./...
set -e
ROOT="$(cd "$(dirname "$0")/.." && (pwd -W 2>/dev/null || pwd))"
MSYS_NO_PATHCONV=1 docker run --rm -v "$ROOT:/src" -v gomodcache:/go/pkg/mod -v gobuildcache:/root/.cache/go-build \
  -w /src -e GOFLAGS=-buildvcs=false golang:1.24-alpine "$@"
