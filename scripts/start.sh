#!/usr/bin/env bash
# Wrapper: ./scripts/start.sh dev|prod
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
mode="${1:-}"

case "$mode" in
  dev)
    shift
    exec "$ROOT/scripts/dev.sh" "$@"
    ;;
  prod|prod-local)
    exec "$ROOT/scripts/prod.sh"
    ;;
  *)
    echo "Usage: $0 dev|prod"
    echo "  dev   — go run, Sonos driver, -debug"
    echo "  prod  — build and start the binary (local prod simulation)"
    exit 1
    ;;
esac
