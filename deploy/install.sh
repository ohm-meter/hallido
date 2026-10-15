#!/usr/bin/env bash
# DEPRECATED: the standard installer is scripts/install.sh (--system --systemd,
# target /usr/local/sonos-agent, service "sonos-agent", CLI "sonos-agent").
# This script only redirects there. Existing installations under
# /opt/sonos-radio-agent keep running (deploy/systemd/sonos-radio-agent.service)
# but are no longer set up anew.
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"

echo ">> NOTE: deploy/install.sh is deprecated -- use scripts/install.sh." >&2
if [[ -d /opt/sonos-radio-agent ]]; then
  echo ">> Found: existing installation /opt/sonos-radio-agent." >&2
  echo ">> The new installer sets up /usr/local/sonos-agent; avoid two services on port 8080:" >&2
  echo ">>   sudo systemctl disable --now sonos-radio-agent   # before starting sonos-agent" >&2
  echo ">>   copy config.json from /opt/sonos-radio-agent to /var/lib/sonos-agent (owner sonos, 0600)." >&2
fi
exec "$REPO_DIR/scripts/install.sh" --system --systemd "$@"
