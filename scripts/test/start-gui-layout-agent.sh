#!/usr/bin/env bash
# Mock agent for the Playwright web app layout tests (webServer in playwright.config.js).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

export SRA_API_TOKEN="${SRA_TEST_TOKEN:-integration-test-token}"
CONFIG="${SRA_TEST_CONFIG:-${ROOT}/scripts/test/fixtures/config.gui-layout.json}"
BIN="${ROOT}/build/sonos-radio-agent"

if [[ -x "$BIN" ]]; then
  exec "$BIN" -config "$CONFIG" -driver=mock
fi
exec go run ./cmd/server -config "$CONFIG" -driver=mock
