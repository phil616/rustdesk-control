#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
client="${1:-$root/rustdesk-managed-client}"
commit=6c578292e8ebbbec708b76986ba8c4bc7c509747
if [[ ! -e "$client/.git" ]]; then
  git clone --depth 1 --branch 1.4.9 https://github.com/rustdesk/rustdesk.git "$client"
fi
[[ "$(git -C "$client" rev-parse HEAD)" == "$commit" ]] || { echo 'Client checkout does not match the pinned upstream commit' >&2; exit 1; }
if ! git -C "$client" show-ref --verify --quiet refs/heads/rustdesk-control/1.4.9; then
  git -C "$client" switch -c rustdesk-control/1.4.9
fi
git -C "$client" submodule update --init --depth 1
apply_once() {
  local directory="$1" patch="$2"
  if git -C "$directory" apply --check "$patch" 2>/dev/null; then
    git -C "$directory" apply "$patch"
  elif git -C "$directory" apply --reverse --check "$patch" 2>/dev/null; then
    echo "Already applied: $(basename "$patch")"
  else
    echo "Patch conflict: $patch; preserve and review local changes" >&2
    exit 1
  fi
}
apply_once "$client" "$root/managed-client/patches/rustdesk.patch"
apply_once "$client/libs/hbb_common" "$root/managed-client/patches/hbb_common.patch"
mkdir -p "$client/src/managed_control"
cp "$root"/managed-client/core/* "$client/src/managed_control/"
cp "$root/managed-client/NOTICE" "$client/NOTICE"
echo "Prepared modified RustDesk 1.4.9 at $client"
