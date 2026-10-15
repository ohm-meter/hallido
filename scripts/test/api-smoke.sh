#!/usr/bin/env bash
# Minimal API smoke check (3 requests) — for CI and the local gate.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/test/common.sh
source "$ROOT/scripts/test/common.sh"

TOKEN="$(resolve_test_token)"

echo "=== API-Smoke ==="
echo "Base: $(resolve_base_url)"
echo ""

setup_test_env

assert_status "GET /api/v1/health" "200" "$(http_code GET "/api/v1/health")"
assert_status "GET /api/v1/scenes without token" "401" "$(http_code GET "/api/v1/scenes")"
assert_status "GET /api/v1/scenes with token" "200" "$(http_code GET "/api/v1/scenes" "$TOKEN")"

print_summary
