#!/usr/bin/env bash
# Development start: Sonos driver, debug logging, go run.
#
# Usage:
#   ./scripts/dev.sh          — start (foreground)
#   ./scripts/dev.sh start    — same as above
#   ./scripts/dev.sh stop     — stop the process on the listen port or from the PID file
#   ./scripts/dev.sh restart  — stop, then start
#
# Stop finds the running instance via .run/sonos-agent-dev.pid or lsof on
# the port from config.json (default 8080). Before start/restart a stale
# go run / binary on the port is stopped automatically.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=scripts/common.sh
source "$ROOT/scripts/common.sh"

usage() {
  echo "Usage: $0 [start|stop|restart]"
  echo "  start   — go run, Sonos driver, -debug (default)"
  echo "  stop    — stop the dev agent on the listen port"
  echo "  restart — stop, then start (after a code change)"
  exit 1
}

cmd_start() {
  prepare_dev_start

  export SRA_DRIVER="${SRA_DRIVER:-sonos}"

  echo ">> Dev mode: driver=${SRA_DRIVER}, -debug, listen=$(read_listen_port)"
  echo ">> GUI: http://localhost:$(read_listen_port)"

  mkdir -p "$(dirname "$DEV_PID_FILE")"

  local pid
  go run ./cmd/server \
    -config "$CONFIG" \
    -driver="$SRA_DRIVER" \
    -debug &
  pid=$!
  echo "$pid" > "$DEV_PID_FILE"

  trap 'kill "$pid" 2>/dev/null || true; rm -f "$DEV_PID_FILE"; exit 130' INT TERM

  wait "$pid" || true
  rm -f "$DEV_PID_FILE"
}

cmd_stop() {
  load_env
  stop_dev_server
}

cmd_restart() {
  cmd_stop
  cmd_start
}

case "${1:-start}" in
  start|"")
    cmd_start
    ;;
  stop)
    cmd_stop
    ;;
  restart)
    cmd_restart
    ;;
  -h|--help|help)
    usage
    ;;
  *)
    usage
    ;;
esac
