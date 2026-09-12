#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
: "${RUSTDESK_CONTROL_URL:?Set the fixed HTTPS control origin}"
: "${RUSTDESK_MANAGED_SOURCE_URL:?Set the corresponding modified source URL}"
for value in "$RUSTDESK_CONTROL_URL" "$RUSTDESK_MANAGED_SOURCE_URL"; do
  [[ "$value" == https://* ]] || { echo 'Managed URLs must use HTTPS' >&2; exit 1; }
done
cd "$root/rustdesk-managed-client"
if [[ ! -f src/bridge_generated.rs ]]; then
  command -v flutter_rust_bridge_codegen >/dev/null || { echo 'Install flutter_rust_bridge_codegen 1.80.1 with uuid support and initialize Flutter first; see README' >&2; exit 1; }
  flutter_rust_bridge_codegen --rust-input ./src/flutter_ffi.rs --dart-output ./flutter/lib/generated_bridge.dart --c-output ./flutter/macos/Runner/bridge_generated.h
fi
cargo build --release --features managed-control,flutter "$@"
echo 'Rust library built. Package with the upstream Flutter desktop toolchain; see README.'
