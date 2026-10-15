#!/usr/bin/env bash
# Local prod simulation: build the binary and run it without debug.
# Real Pi operation: scripts/install.sh --system --systemd.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=scripts/common.sh
source "$ROOT/scripts/common.sh"

prepare_start

export SRA_DRIVER="${SRA_DRIVER:-sonos}"

echo ">> Prod mode (local): driver=${SRA_DRIVER}, no -debug"
echo ">> Building binary to build/sonos-radio-agent …"
mkdir -p "${ROOT}/build"
go build -trimpath -ldflags="-s -w" -o "$BINARY" ./cmd/server

echo ">> Starting binary, GUI: http://localhost:$(read_listen_port)"
echo ">> Note: on the Pi use scripts/install.sh --system --systemd and systemctl instead."

exec "$BINARY" \
  -config "$CONFIG" \
  -driver="$SRA_DRIVER"
