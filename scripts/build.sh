#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
(cd web && npm ci && npm run build)
mkdir -p bin
CGO_ENABLED=0 go build -trimpath -o bin/rustdesk-control ./cmd/rustdesk-control
