#!/usr/bin/env bash
# shellcheck shell=bash
# Shared helpers for scripts/dev.sh, prod.sh and start.sh.
# Do not run directly — loaded via source.

if [[ -z "${ROOT:-}" ]]; then
  ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fi

CONFIG="${ROOT}/config.json"
CONFIG_EXAMPLE="${ROOT}/config.example.json"
# shellcheck disable=SC2034  # used by prod.sh
BINARY="${ROOT}/build/sonos-radio-agent"
DEV_PID_FILE="${ROOT}/.run/sonos-agent-dev.pid"

load_env() {
  local env_file="${ROOT}/.env"
  if [[ -f "$env_file" ]]; then
    set -a
    # shellcheck disable=SC1090
    source "$env_file"
    set +a
  fi
}

ensure_config() {
  if [[ -f "$CONFIG" ]]; then
    return 0
  fi
  if [[ ! -f "$CONFIG_EXAMPLE" ]]; then
    echo "ERROR: Neither config.json nor config.example.json found."
    exit 1
  fi
  cp "$CONFIG_EXAMPLE" "$CONFIG"
  echo ">> config.json created from config.example.json."
  echo ">> Please set api_token in config.json (the placeholder BITTE-AENDERN is rejected)."
  echo ">> Alternatively: set SRA_API_TOKEN in .env (see .env.example)."
}

read_listen_port() {
  local listen=":8080"
  if [[ -f "$CONFIG" ]]; then
    listen="$(grep -E '"listen"' "$CONFIG" | head -1 | sed -E 's/.*"listen"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')"
  fi
  if [[ "$listen" =~ :([0-9]+)$ ]]; then
    echo "${BASH_REMATCH[1]}"
  elif [[ "$listen" =~ ^[0-9]+$ ]]; then
    echo "$listen"
  else
    echo "8080"
  fi
}

check_port_free() {
  local port
  port="$(read_listen_port)"
  if ! command -v lsof >/dev/null 2>&1; then
    return 0
  fi
  if lsof -iTCP:"$port" -sTCP:LISTEN -P -n >/dev/null 2>&1; then
    echo "ERROR: Port ${port} is already in use."
    lsof -iTCP:"$port" -sTCP:LISTEN -P -n 2>/dev/null | head -5 || true
    echo "Stop the existing process or change listen in config.json."
    exit 1
  fi
}

prepare_start() {
  cd "$ROOT" || exit 1
  load_env
  ensure_config
  check_port_free
}

# Stops the local dev agent (PID file and/or listen port).
stop_dev_server() {
  local port pid killed=0
  port="$(read_listen_port)"

  if [[ -f "$DEV_PID_FILE" ]]; then
    pid="$(tr -d '[:space:]' < "$DEV_PID_FILE")"
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      echo ">> Stopping dev agent (PID ${pid}) …"
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
      killed=1
    fi
    rm -f "$DEV_PID_FILE"
  fi

  if command -v lsof >/dev/null 2>&1; then
    # Only stop our own agent processes (command exactly sonos-radio-agent, go run binary "server"),
    # never foreign listeners on the port.
    local cmd lpid
    while read -r cmd lpid; do
      [[ -n "$lpid" ]] || continue
      case "$cmd" in
        sonos-radio-agent|server)
          echo ">> Stopping ${cmd} (PID ${lpid}) on port ${port}"
          kill "$lpid" 2>/dev/null || true
          killed=1
          ;;
        *) echo ">> Port ${port} is in use by a foreign process ${cmd} (PID ${lpid}) -- not stopped." ;;
      esac
    done < <(lsof +c 0 -iTCP:"${port}" -sTCP:LISTEN -P -n 2>/dev/null | awk 'NR>1 {print $1, $2}' || true)
  fi

  if [[ "$killed" -eq 0 ]]; then
    echo ">> No dev agent found on port ${port}."
  fi
}

# Frees the listen port if an old go run / binary is listening.
free_stale_dev_port() {
  local port info
  port="$(read_listen_port)"
  if ! command -v lsof >/dev/null 2>&1; then
    return 0
  fi
  if ! lsof -iTCP:"$port" -sTCP:LISTEN -P -n >/dev/null 2>&1; then
    return 0
  fi
  info="$(lsof +c 0 -iTCP:"$port" -sTCP:LISTEN -P -n 2>/dev/null | tail -n +2 || true)"
  if echo "$info" | awk '{print $1}' | grep -qxE 'sonos-radio-agent|server'; then
    echo ">> Port ${port} is in use by an old agent — stopping it."
    stop_dev_server
    return 0
  fi
  return 1
}

prepare_dev_start() {
  cd "$ROOT" || exit 1
  load_env
  ensure_config
  if ! free_stale_dev_port; then
    check_port_free
  fi
}
