#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
client="$root/rustdesk-managed-client"
[[ "$(git -C "$client" rev-parse HEAD)" == 6c578292e8ebbbec708b76986ba8c4bc7c509747 ]]
mkdir -p "$root/managed-client/core" "$root/managed-client/patches"
cp "$client"/src/managed_control/*.rs "$root/managed-client/core/"
cp "$client"/src/managed_control/Cargo.{toml,lock} "$root/managed-client/core/"
git -C "$client" diff --binary -- . ':!libs/hbb_common' > "$root/managed-client/patches/rustdesk.patch"
git -C "$client/libs/hbb_common" diff --binary > "$root/managed-client/patches/hbb_common.patch"
echo 'Exported management sources and upstream patches'
