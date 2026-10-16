#!/usr/bin/env bash
# NAS project file: renders the template and checks the hardening settings
# (ADR 0021). With Docker installed also "docker compose config".
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
pass=0
fail=0
ok() { echo "  ok   $1"; pass=$((pass + 1)); }
bad() { echo "  FAIL $1"; fail=$((fail + 1)); }

DIGEST="sha256:$(printf '%064d' 0 | tr 0 a)"
OUT="$("$ROOT/scripts/render-compose.sh" "ghcr.io/ohm-meter/hallido@${DIGEST}" v9.9.9)"

check() { # name, extended regex that must match a line
  if grep -Eq -- "$2" <<<"$OUT"; then ok "$1"; else bad "$1"; fi
}
check "image pinned by digest" "^    image: ghcr\.io/ohm-meter/hallido@${DIGEST}$"
check "host network (SSDP)" "^    network_mode: host$"
check "non-root user" '^    user: "65532:65532"$'
check "read-only root filesystem" "^    read_only: true$"
check "drop all capabilities" "^      - ALL$"
check "no-new-privileges" "^      - no-new-privileges:true$"
check "memory limit" "^    mem_limit: "
check "pids limit" "^    pids_limit: "
check "only the data volume" "^      - hallido-data:/data$"
check "version in header" "Hallido v9\.9\.9"
if grep -q "__" <<<"$OUT"; then bad "no placeholder left"; else ok "no placeholder left"; fi
if grep -Eq "privileged|cap_add|/var/run/docker.sock|ports:" <<<"$OUT"; then
  bad "no privileged/cap_add/docker.sock/ports"
else
  ok "no privileged/cap_add/docker.sock/ports"
fi

for args in "nginx:latest v1" "ghcr.io/ohm-meter/hallido@sha256:abc v1" "ghcr.io/x@${DIGEST} 'v1;rm'"; do
  # shellcheck disable=SC2086
  if eval "\"$ROOT/scripts/render-compose.sh\" $args" >/dev/null 2>&1; then
    bad "rejects invalid input: $args"
  else
    ok "rejects invalid input: $args"
  fi
done

if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
  if docker compose -f - config -q <<<"$OUT" 2>/dev/null; then ok "docker compose config"; else bad "docker compose config"; fi
else
  echo "  skip docker compose config (Docker missing)"
fi

echo "nas-compose: ${pass} ok, ${fail} failed"
[[ $fail -eq 0 ]]
