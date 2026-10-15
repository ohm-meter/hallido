#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2016
# Unit tests for the install layout (ADR 0017), without root and without Docker:
# .install-meta parser (no source), trust check, unit rewriting.
# Migration with real paths: scripts/test/install-migration-docker.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=../sonos-agent-cli.sh
source "${ROOT}/scripts/sonos-agent-cli.sh"
set +e

PASS=0
FAIL=0
ok() { PASS=$((PASS + 1)); echo "  ok   $*"; }
nok() { FAIL=$((FAIL + 1)); echo "  FAIL $*"; }
check() { local name="$1"; shift; if "$@"; then ok "$name"; else nok "$name"; fi; }

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

echo "=== install-layout: .install-meta ==="

meta="${WORK}/meta/.install-meta"
mkdir -p "${WORK}/meta"
chmod 0755 "${WORK}/meta"
cat > "$meta" <<METAEOF
INSTALL_ROOT=/usr/local/sonos-agent
REPO_DIR=\$(touch ${WORK}/pwned)
CONFIG_PATH=/var/lib/sonos-agent/config.json
DATA_DIR=/var/lib/sonos-agent
PATH=/tmp/evil
EVIL=\`touch ${WORK}/pwned2\`
  GIT_COMMIT=indented-ignored
GIT_COMMIT=abc1234
METAEOF
chmod 0644 "$meta"

(
  parse_meta "$meta"
  [[ "$INSTALL_ROOT" == "/usr/local/sonos-agent" ]] || exit 11
  [[ "$REPO_DIR" == "\$(touch ${WORK}/pwned)" ]] || exit 12
  [[ "$GIT_COMMIT" == "abc1234" ]] || exit 13
  [[ "$PATH" != "/tmp/evil" ]] || exit 14
  [[ -z "${EVIL:-}" ]] || exit 15
)
check "parse_meta reads values literally, known keys only" test $? -eq 0
check "parse_meta executes nothing" test ! -e "${WORK}/pwned" -a ! -e "${WORK}/pwned2"

check "own file 0644 is trusted" meta_trusted "$meta"

chmod 0666 "$meta"
if meta_trusted "$meta"; then nok "world-writable rejected"; else ok "world-writable rejected"; fi
chmod 0644 "$meta"

chmod 0777 "${WORK}/meta"
if meta_trusted "$meta"; then nok "world-writable directory rejected"; else ok "world-writable directory rejected"; fi
chmod 0755 "${WORK}/meta"

ln -s "$meta" "${WORK}/meta/link"
if is_trusted_path "${WORK}/meta/link"; then nok "symlink rejected"; else ok "symlink rejected"; fi

# load_meta via SONOS_AGENT_INSTALL_ROOT: reads valid metadata.
cat > "$meta" <<METAEOF
INSTALL_ROOT=${WORK}/meta
REPO_DIR=${WORK}/repo
BINARY_PATH=${WORK}/meta/bin/x
CONFIG_PATH=${WORK}/meta/config.json
METAEOF
(
  export SONOS_AGENT_INSTALL_ROOT="${WORK}/meta"
  load_meta >/dev/null
  [[ "$DATA_DIR" == "${WORK}/meta" && "$REPO_DIR" == "${WORK}/repo" ]]
)
check "load_meta: DATA_DIR falls back to the config directory (user mode)" test $? -eq 0

chmod 0666 "$meta"
out="$(SONOS_AGENT_INSTALL_ROOT="${WORK}/meta" bash "${ROOT}/scripts/sonos-agent-cli.sh" version 2>&1)"
rc=$?
check "CLI refuses a tamperable .install-meta" test "$rc" -ne 0
check "error message names the reason" grep -q "is not loaded" <<<"$out"
chmod 0644 "$meta"

out="$(SONOS_AGENT_INSTALL_ROOT="${WORK}/meta" bash "${ROOT}/scripts/sonos-agent-cli.sh" version 2>&1)"
check "CLI version with valid .install-meta" grep -q "Data:       ${WORK}/meta" <<<"$out"

echo "=== install-layout: systemd unit ==="

legacy="${ROOT}/scripts/test/fixtures/sonos-agent.legacy.service"
new="$(rewrite_unit < "$legacy")"
check "ExecStart points to /var/lib" grep -qx 'ExecStart=/usr/local/sonos-agent/bin/sonos-radio-agent -config /var/lib/sonos-agent/config.json' <<<"$new"
check "WorkingDirectory in the data directory" grep -qx 'WorkingDirectory=/var/lib/sonos-agent' <<<"$new"
check "ReadWritePaths on code removed" bash -c '! grep -q "^ReadWritePaths=/usr/local" <<<"$1"' _ "$new"
check "StateDirectory set" grep -qx 'StateDirectory=sonos-agent' <<<"$new"
check "StateDirectory right after [Service]" bash -c 'grep -A1 -x "\[Service\]" <<<"$1" | grep -qx "StateDirectory=sonos-agent"' _ "$new"
check "custom Environment line is kept" grep -qx 'Environment=SRA_API_TOKEN=lokal-gesetzt-1234567890' <<<"$new"
check "rewriting is idempotent" test "$(rewrite_unit <<<"$new")" == "$new"
check "template in the repo needs no rewriting" test "$(rewrite_unit < "${ROOT}/deploy/systemd/sonos-agent.service")" == "$(cat "${ROOT}/deploy/systemd/sonos-agent.service")"

echo "=== install-layout: git clone check ==="

repo="${WORK}/clone"
mkdir -p "${repo}/.git/hooks"
touch "${repo}/.git/hooks/pre-commit.sample"
printf '[remote "origin"]\n\turl = https://example.org/x.git\n' > "${repo}/.git/config"
check "clean clone passes" check_repo_untampered "$repo"
touch "${repo}/.git/hooks/post-merge"
if check_repo_untampered "$repo" 2>/dev/null; then nok "hook detected"; else ok "hook detected"; fi
rm "${repo}/.git/hooks/post-merge"
printf '[core]\n\tfsmonitor = /tmp/x\n' >> "${repo}/.git/config"
if check_repo_untampered "$repo" 2>/dev/null; then nok "core.fsmonitor detected"; else ok "core.fsmonitor detected"; fi

echo
echo "install-layout: ${PASS} ok, ${FAIL} failed"
[[ "$FAIL" -eq 0 ]]
