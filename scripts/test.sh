#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
(cd web && npm ci && npm run build)
go test -race ./...
go vet ./...
cargo test --locked --manifest-path managed-client/core/Cargo.toml
python3 scripts/check-build-env.py
