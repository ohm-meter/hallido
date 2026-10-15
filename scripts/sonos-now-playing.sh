#!/usr/bin/env bash
# Reads the now-playing metadata of a speaker via the REST API.
#
# Requirements: curl, jq; a running agent with a token.
#
# Usage:
#   SRA_TOKEN=your-token ./scripts/sonos-now-playing.sh "Living Room"
#   SRA_BASE_URL=http://127.0.0.1:8080 SRA_TOKEN=... ./scripts/sonos-now-playing.sh Kitchen
#
# Environment variables:
#   SRA_TOKEN      (required) bearer token from config.json
#   SRA_BASE_URL   base URL, default http://127.0.0.1:8080
set -euo pipefail

TOKEN="${SRA_TOKEN:-}"
BASE="${SRA_BASE_URL:-http://127.0.0.1:8080}"
SPEAKER="${1:-}"

if [[ -z "$TOKEN" ]]; then
  echo "Error: SRA_TOKEN missing (token from config.json)." >&2
  exit 1
fi
if [[ -z "$SPEAKER" ]]; then
  echo "Usage: SRA_TOKEN=... $0 \"Speaker name\"" >&2
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "Error: jq not found." >&2
  exit 1
fi

ENCODED="$(jq -rn --arg s "$SPEAKER" '$s|@uri')"
URL="${BASE%/}/api/v1/speakers/${ENCODED}/now-playing"

curl -fsS -H "Authorization: Bearer ${TOKEN}" "$URL" | jq .
