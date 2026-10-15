#!/usr/bin/env bash
# Integration tests against the REST API (mock driver).
# Starts an agent on port 18080 if needed, or uses SRA_TEST_BASE_URL.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/test/common.sh
source "$ROOT/scripts/test/common.sh"

TOKEN="$(resolve_test_token)"

echo "=== API-Integration (Mock) ==="
echo "Base: $(resolve_base_url)"
echo ""

setup_test_env

assert_status "GET /api/v1/health" "200" "$(http_code GET "/api/v1/health")"
assert_status "GET /api/v1/scenes without token" "401" "$(http_code GET "/api/v1/scenes")"
assert_status "GET /api/v1/scenes with token" "200" "$(http_code GET "/api/v1/scenes" "$TOKEN")"
assert_status "GET /api/v1/speakers with token" "200" "$(http_code GET "/api/v1/speakers" "$TOKEN")"
assert_status "GET /api/v1/scenes/status with token" "200" "$(http_code GET "/api/v1/scenes/status" "$TOKEN")"
assert_status "GET /api/v1/speakers/now-playing with token" "200" "$(http_code GET "/api/v1/speakers/now-playing" "$TOKEN")"
assert_status "POST /api/v1/discovery/refresh with token" "200" "$(http_code POST "/api/v1/discovery/refresh" "$TOKEN")"
assert_status "GET /api/v1/config with token" "200" "$(http_code GET "/api/v1/config" "$TOKEN")"
assert_status "GET /api/v1/sonos/favorites with token" "200" "$(http_code GET "/api/v1/sonos/favorites" "$TOKEN")"
assert_status "GET /api/v1/radio/search without q" "400" "$(http_code GET "/api/v1/radio/search" "$TOKEN")"

SPEAKER_NAME="$(first_speaker_name "$TOKEN")"
if [[ -n "$SPEAKER_NAME" ]]; then
  ENCODED="$(jq -rn --arg s "$SPEAKER_NAME" '$s|@uri')"
  assert_status "GET /api/v1/speakers/${SPEAKER_NAME}/now-playing" "200" \
    "$(http_code GET "/api/v1/speakers/${ENCODED}/now-playing" "$TOKEN")"
fi

SCENE_ID="$(first_scene_id "$TOKEN")"
if [[ -z "$SCENE_ID" ]]; then
  SCENE_ID="fm4-wohnzimmer"
fi
assert_status "POST /api/v1/scenes/${SCENE_ID}/play" "200" \
  "$(http_code POST "/api/v1/scenes/${SCENE_ID}/play" "$TOKEN")"

print_summary
