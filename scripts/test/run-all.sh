#!/usr/bin/env bash
# Runs the API and web app tests in sequence.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

echo "=== run-all: API + GUI ==="
echo ""

"${ROOT}/scripts/test/api-mock.sh"
echo ""
"${ROOT}/scripts/test/gui-static.sh"
echo ""
"${ROOT}/scripts/test/gui-layout.sh"
