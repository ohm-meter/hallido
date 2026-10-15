#!/usr/bin/env bash
# shellcheck shell=bash
# Shared helpers for scripts/test/*.sh
# Do not run directly — loaded via source.

if [[ -z "${ROOT:-}" ]]; then
  ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
fi

TEST_DIR="${ROOT}/scripts/test"
FIXTURE_CONFIG="${SRA_TEST_CONFIG:-${TEST_DIR}/fixtures/config.test.json}"
DEFAULT_TEST_PORT="18080"
DEFAULT_TEST_TOKEN="integration-test-token"

# Base URL of the agent (e.g. http://127.0.0.1:18080).
SRA_TEST_BASE_URL="${SRA_TEST_BASE_URL:-}"
# Token for bearer auth; takes precedence over config.
SRA_TEST_TOKEN="${SRA_TEST_TOKEN:-}"
# 1 = start the server if needed (default), 0 = only use an existing agent.
SRA_TEST_AUTO_START="${SRA_TEST_AUTO_START:-1}"

_MANAGED_PID=""
_PASS=0
_FAIL=0

red() { printf '\033[31m%s\033[0m\n' "$*"; }
green() { printf '\033[32m%s\033[0m\n' "$*"; }

read_token_from_config() {
  local cfg="$1"
  if [[ ! -f "$cfg" ]]; then
    return 1
  fi
  grep -E '"api_token"' "$cfg" | head -1 \
    | sed -E 's/.*"api_token"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/'
}

resolve_test_token() {
  if [[ -n "$SRA_TEST_TOKEN" ]]; then
    echo "$SRA_TEST_TOKEN"
    return 0
  fi
  local from_fixture
  from_fixture="$(read_token_from_config "$FIXTURE_CONFIG" || true)"
  if [[ -n "$from_fixture" && "$from_fixture" != "BITTE-AENDERN" ]]; then
    echo "$from_fixture"
    return 0
  fi
  if [[ -f "${ROOT}/config.json" ]]; then
    local from_user
    from_user="$(read_token_from_config "${ROOT}/config.json" || true)"
    if [[ -n "$from_user" && "$from_user" != "BITTE-AENDERN" ]]; then
      echo "$from_user"
      return 0
    fi
  fi
  echo "$DEFAULT_TEST_TOKEN"
}

resolve_base_url() {
  if [[ -n "$SRA_TEST_BASE_URL" ]]; then
    echo "${SRA_TEST_BASE_URL%/}"
    return 0
  fi
  echo "http://127.0.0.1:${DEFAULT_TEST_PORT}"
}

server_reachable() {
  local base
  base="$(resolve_base_url)"
  curl -sf -o /dev/null -m 3 "${base}/api/v1/health" 2>/dev/null
}

wait_for_server() {
  local base attempts=60
  base="$(resolve_base_url)"
  while (( attempts > 0 )); do
    if curl -sf -o /dev/null -m 2 "${base}/api/v1/health" 2>/dev/null; then
      return 0
    fi
    sleep 0.5
    attempts=$((attempts - 1))
  done
  red "ERROR: Agent at ${base} is unreachable."
  return 1
}

start_managed_server() {
  if [[ -n "$_MANAGED_PID" ]]; then
    return 0
  fi
  if [[ ! -f "$FIXTURE_CONFIG" ]]; then
    red "ERROR: Fixture config missing: ${FIXTURE_CONFIG}"
    exit 1
  fi
  local base
  base="$(resolve_base_url)"
  green ">> Starting mock agent (${base}) …"
  (
    cd "$ROOT" || exit 1
    SRA_API_TOKEN="$(resolve_test_token)" \
      go run ./cmd/server \
        -config "$FIXTURE_CONFIG" \
        -driver=mock \
        >/dev/null 2>&1
  ) &
  _MANAGED_PID=$!
  if ! wait_for_server; then
    stop_managed_server
    exit 1
  fi
}

stop_managed_server() {
  if [[ -n "$_MANAGED_PID" ]]; then
    kill "$_MANAGED_PID" 2>/dev/null || true
    wait "$_MANAGED_PID" 2>/dev/null || true
    _MANAGED_PID=""
  fi
}

kill_port_listener() {
  local port="$1"
  local pids
  pids="$(lsof -ti "tcp:${port}" 2>/dev/null || true)"
  if [[ -n "$pids" ]]; then
    # shellcheck disable=SC2086  # multiple PIDs intended
    kill $pids 2>/dev/null || true
    sleep 0.5
  fi
}

server_has_config_api() {
  local token code
  token="$(resolve_test_token)"
  code="$(http_code GET "/api/v1/config" "$token")"
  [[ "$code" == "200" ]]
}

server_has_favorites_api() {
  local token code
  token="$(resolve_test_token)"
  code="$(http_code GET "/api/v1/sonos/favorites" "$token")"
  [[ "$code" == "200" ]]
}

server_has_radio_api() {
  local token code
  token="$(resolve_test_token)"
  code="$(http_code GET "/api/v1/radio/search" "$token")"
  [[ "$code" == "400" ]]
}

ensure_server() {
  if server_reachable; then
    if server_has_config_api && server_has_favorites_api && server_has_radio_api; then
      green ">> Agent reachable at $(resolve_base_url)"
      return 0
    fi
    if [[ -n "$SRA_TEST_BASE_URL" ]]; then
      red "ERROR: Agent at $(resolve_base_url) has no /api/v1/config — rebuild/restart."
      exit 1
    fi
    green ">> Stale agent on port ${DEFAULT_TEST_PORT} — restarting …"
    kill_port_listener "$DEFAULT_TEST_PORT"
  fi
  if [[ "$SRA_TEST_AUTO_START" == "0" ]]; then
    red "ERROR: No agent at $(resolve_base_url) — SRA_TEST_AUTO_START=0."
    exit 1
  fi
  start_managed_server
}

setup_test_env() {
  trap stop_managed_server EXIT
  ensure_server
}

http_code() {
  local method="$1" path="$2" token="${3:-}"
  local base url args=(-s -o /dev/null -w "%{http_code}" -m 10 -X "$method")
  base="$(resolve_base_url)"
  url="${base}${path}"
  if [[ -n "$token" ]]; then
    args+=(-H "Authorization: Bearer ${token}")
  fi
  curl "${args[@]}" "$url" 2>/dev/null || echo "000"
}

http_body() {
  local method="$1" path="$2" token="${3:-}"
  local base url args=(-s -m 10 -X "$method")
  base="$(resolve_base_url)"
  url="${base}${path}"
  if [[ -n "$token" ]]; then
    args+=(-H "Authorization: Bearer ${token}")
  fi
  curl "${args[@]}" "$url" 2>/dev/null || true
}

assert_status() {
  local name="$1" expected="$2" actual="$3"
  if [[ "$actual" == "$expected" ]]; then
    green "PASS  ${name} (${actual})"
    _PASS=$((_PASS + 1))
  else
    red "FAIL  ${name} (expected ${expected}, got ${actual})"
    _FAIL=$((_FAIL + 1))
  fi
}

first_scene_id() {
  local body token="$1"
  body="$(http_body GET "/api/v1/scenes" "$token")"
  if command -v jq >/dev/null 2>&1; then
    echo "$body" | jq -r '.scenes[0].id // empty'
    return 0
  fi
  echo "$body" | grep -Eo '"id"[[:space:]]*:[[:space:]]*"[^"]+"' | head -1 \
    | sed -E 's/.*"id"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/'
}

first_speaker_name() {
  local body token="$1"
  body="$(http_body GET "/api/v1/speakers" "$token")"
  if command -v jq >/dev/null 2>&1; then
    echo "$body" | jq -r '.speakers[0].name // empty'
    return 0
  fi
  echo "$body" | grep -Eo '"name"[[:space:]]*:[[:space:]]*"[^"]+"' | head -1 \
    | sed -E 's/.*"name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/'
}

print_summary() {
  echo ""
  echo "Result: ${_PASS} passed, ${_FAIL} failed"
  if (( _FAIL > 0 )); then
    exit 1
  fi
}
