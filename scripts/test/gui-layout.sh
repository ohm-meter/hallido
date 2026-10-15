#!/usr/bin/env bash
# Browser layout tests (Playwright): catch overlapping dialog areas.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/test/common.sh
source "$ROOT/scripts/test/common.sh"

# Custom fixture (e.g. another port for parallel runs) via SRA_TEST_CONFIG
export SRA_TEST_CONFIG="${SRA_TEST_CONFIG:-${ROOT}/scripts/test/fixtures/config.gui-layout.json}"
export SRA_TEST_BASE_URL="${SRA_TEST_BASE_URL:-http://127.0.0.1:18081}"
export PLAYWRIGHT_BASE_URL="${PLAYWRIGHT_BASE_URL:-$SRA_TEST_BASE_URL}"
export SRA_TEST_TOKEN="${SRA_TEST_TOKEN:-$(resolve_test_token)}"

if ! command -v node >/dev/null 2>&1; then
  red "ERROR: Node.js is missing (needed for the Playwright layout tests)."
  exit 1
fi
if [[ ! -d "${ROOT}/node_modules/@playwright/test" ]]; then
  red "ERROR: npm dependencies are missing — in the repo root run: npm ci"
  exit 1
fi

echo "=== GUI-Layout (Playwright) ==="
echo "Base: ${PLAYWRIGHT_BASE_URL}"
echo "Agent: Playwright webServer (start-gui-layout-agent.sh)"
echo ""

cd "$ROOT"
npm run test:gui-layout
