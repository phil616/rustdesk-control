#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
(cd web && npm ci && npm run build)
python3 scripts/collect-notices.py --output web/dist/THIRD-PARTY-NOTICES.txt
python3 scripts/package-source.py --output web/dist/source.tar.gz
cp LICENSE web/dist/LICENSE
mkdir -p bin
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o bin/rustdesk-control ./cmd/rustdesk-control
