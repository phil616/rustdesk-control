#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
(cd web && npm run build)
test_dir="$(mktemp -d)"
go build -o "$test_dir/browser-server" ./scripts/browser
"$test_dir/browser-server" > "$test_dir/url" 2> "$test_dir/server.log" &
server_pid=$!
trap 'kill "$server_pid" 2>/dev/null || true; wait "$server_pid" 2>/dev/null || true; rm -rf "$test_dir"' EXIT
for attempt in {1..100}; do
  [[ -s "$test_dir/url" ]] && break
  kill -0 "$server_pid" 2>/dev/null || { cat "$test_dir/server.log"; exit 1; }
  sleep .1
done
export RDC_TEST_URL="$(head -n 1 "$test_dir/url")"
[[ "$RDC_TEST_URL" == https://* ]] || exit 1
(cd web && npx playwright test)
