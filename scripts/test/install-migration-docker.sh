#!/usr/bin/env bash
# Migration and installation test with real system paths in a throwaway
# container (Debian, no systemd). Verifies ADR 0017:
#   - sonos-agent migrate: old layout -> code owned by root, config in /var/lib
#   - symlink trap and tampered clone are rejected
#   - install.sh --system --systemd creates the new layout or migrates
#   - an admin without root reads .install-meta only if root owns it
#
# Usage (host, Docker required):  ./scripts/test/install-migration-docker.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
IMAGE="${SRA_TEST_IMAGE:-debian:bookworm-slim}"

if [[ "${1:-}" != "--inside" ]]; then
  command -v docker >/dev/null 2>&1 || { echo "Docker missing — test skipped." >&2; exit 0; }
  exec docker run --rm -v "${ROOT}:/repo:ro" "$IMAGE" bash /repo/scripts/test/install-migration-docker.sh --inside
fi

# --- from here on inside the container (root) --------------------------------------------
CLI=/repo/scripts/sonos-agent-cli.sh
R=/usr/local/sonos-agent
D=/var/lib/sonos-agent
U=/etc/systemd/system/sonos-agent.service
PASS=0
FAIL=0
ok() { PASS=$((PASS + 1)); echo "  ok   $*"; }
nok() { FAIL=$((FAIL + 1)); echo "  FAIL $*"; }
check() { local name="$1"; shift; if "$@"; then ok "$name"; else nok "$name"; fi; }
owner() { stat -c '%U:%G' "$1"; }
mode() { stat -c '%a' "$1"; }
no_foreign_owner() { [[ -z "$(find "$1" ! -user root -print -quit)" ]]; }
no_group_world_write() { [[ -z "$(find "$1" ! -type l -perm /022 -print -quit)" ]]; }

useradd --system --no-create-home --shell /usr/sbin/nologin sonos
useradd --create-home admin

# Old layout as created by install.sh before ADR 0017 (chown -R sonos:sonos).
make_legacy() {
  rm -rf "$R" "$D" "$U" "${U}.pre-migrate" /tmp/pwned
  mkdir -p "${R}/bin" "${R}/src/.git/hooks" /etc/systemd/system
  printf '#!/bin/sh\necho agent\n' > "${R}/bin/sonos-radio-agent"
  chmod 0755 "${R}/bin/sonos-radio-agent"
  echo '{"api_token":"geheim-token-1234567890"}' > "${R}/config.json"
  echo '{"api_token":"alt"}' > "${R}/config.json.bak"
  echo 'tmp' > "${R}/.config-123.tmp"
  cp /repo/config.example.json "${R}/config.example.json"
  touch "${R}/src/.git/hooks/pre-commit.sample"
  printf '[remote "origin"]\n\turl = https://example.org/hallido.git\n' > "${R}/src/.git/config"
  cat > "${R}/.install-meta" <<META
INSTALL_ROOT=${R}
BIN_DIR=/usr/local/bin
REPO_DIR=${R}/src
BINARY_PATH=${R}/bin/sonos-radio-agent
CONFIG_PATH=${R}/config.json
CLI_PATH=/usr/local/bin/sonos-agent
GIT_BRANCH=master
GIT_COMMIT=0ld0ld0
INSTALLED_AT=2026-10-01T00:00:00Z
OS=\$(touch /tmp/pwned)
ARCH=armv7l
META
  chown -R sonos:sonos "$R"
  chmod 0640 "${R}/config.json"
  install -m 0644 /repo/scripts/test/fixtures/sonos-agent.legacy.service "$U"
  install -m 0755 "$CLI" /usr/local/bin/sonos-agent
}

echo "=== 1: sonos-agent migrate (old layout) ==="
make_legacy
out="$(su admin -c 'sonos-agent version' 2>&1)" && rc=0 || rc=$?
check "admin: service-owned .install-meta is not loaded" test "$rc" -ne 0
check "hint to run migrate" grep -q "sudo sonos-agent migrate" <<<"$out"
check "need detected" bash -c "source $CLI; system_needs_migration"

bash "$CLI" migrate
check "config.json moved" test -f "${D}/config.json" -a ! -e "${R}/config.json"
check "config.json content preserved" grep -q geheim-token "${D}/config.json"
check "config.json sonos:sonos 0600" test "$(owner "${D}/config.json") $(mode "${D}/config.json")" == "sonos:sonos 600"
check "config.json.bak moved" test -f "${D}/config.json.bak" -a ! -e "${R}/config.json.bak"
check "data directory sonos:sonos 0750" test "$(owner "$D") $(mode "$D")" == "sonos:sonos 750"
check "temp files removed" test ! -e "${R}/.config-123.tmp"
check "code directory fully owned by root" no_foreign_owner "$R"
check "code not group/world-writable" no_group_world_write "$R"
check "meta: new CONFIG_PATH" grep -qx "CONFIG_PATH=${D}/config.json" "${R}/.install-meta"
check "meta: DATA_DIR + LAYOUT" grep -qx "LAYOUT=2" "${R}/.install-meta"
check "meta: commit preserved" grep -qx "GIT_COMMIT=0ld0ld0" "${R}/.install-meta"
check "meta values never executed" test ! -e /tmp/pwned
check "unit: new ExecStart" grep -q -- "-config ${D}/config.json" "$U"
check "unit: StateDirectory" grep -qx "StateDirectory=sonos-agent" "$U"
check "unit: token line preserved" grep -qx "Environment=SRA_API_TOKEN=lokal-gesetzt-1234567890" "$U"
check "unit: backup created" test -f "${U}.pre-migrate"
out="$(su admin -c 'sonos-agent version' 2>&1)"
check "admin: version after migration" grep -q "Data:       ${D}" <<<"$out"
out="$(su admin -c 'sonos-agent doctor' 2>&1 || true)"
check "admin: doctor reports new layout" grep -q "Layout: code owned by root" <<<"$out"
check "admin: doctor recognizes service-owned config" grep -q "Config readable only by the service" <<<"$out"
check "service user can write config" su -s /bin/sh sonos -c "touch ${D}/.config-x.tmp && rm ${D}/.config-x.tmp"
check "service user cannot modify code" su -s /bin/sh sonos -c "! touch ${R}/bin/sonos-radio-agent 2>/dev/null && ! touch ${R}/.install-meta 2>/dev/null"
out="$(bash "$CLI" migrate)"
check "second run: nothing to do" grep -q "Already on the new layout" <<<"$out"

echo "=== 2: symlink trap ==="
make_legacy
rm "${R}/config.json"
ln -s /etc/shadow "${R}/config.json"
chown -h sonos:sonos "${R}/config.json"
shadow_before="$(owner /etc/shadow) $(mode /etc/shadow)"
bash "$CLI" migrate >/dev/null 2>&1
check "/etc/shadow unchanged" test "$(owner /etc/shadow) $(mode /etc/shadow)" == "$shadow_before"
check "symlink not migrated" test ! -e "${D}/config.json" -a ! -L "${D}/config.json"

echo "=== 3: tampered clone (hook) ==="
make_legacy
printf '#!/bin/sh\ntouch /tmp/pwned\n' > "${R}/src/.git/hooks/post-merge"
chown sonos:sonos "${R}/src/.git/hooks/post-merge"
bash "$CLI" migrate >/dev/null 2>&1 && rc=0 || rc=$?
check "migration aborts" test "$rc" -ne 0
check "nothing changed (config stays)" test -f "${R}/config.json" -a ! -e "$D"

echo "=== 4: fresh install install.sh --system --systemd ==="
rm -rf "$R" "$D" "$U" "${U}.pre-migrate" /usr/local/bin/sonos-agent
mkdir -p "${R}/bin"
printf '#!/bin/sh\necho agent\n' > "${R}/bin/sonos-radio-agent"
chmod 0755 "${R}/bin/sonos-radio-agent"
SONOS_AGENT_REPO_DIR=/repo bash /repo/scripts/install.sh --system --systemd --no-build </dev/null >/dev/null
check "config in ${D}" test -f "${D}/config.json"
check "config sonos:sonos 0600" test "$(owner "${D}/config.json") $(mode "${D}/config.json")" == "sonos:sonos 600"
check "code owned by root" no_foreign_owner "$R"
check "no config in the code directory" test ! -e "${R}/config.json"
check "unit from template" cmp -s /repo/deploy/systemd/sonos-agent.service "$U"
out="$(su admin -c 'sonos-agent doctor' 2>&1 || true)"
check "doctor: layout ok" grep -q "Layout: code owned by root" <<<"$out"

echo "=== 5: install.sh over the old layout (local repo) ==="
make_legacy
SONOS_AGENT_REPO_DIR=/repo bash /repo/scripts/install.sh --system --systemd --no-build </dev/null >/dev/null
check "config migrated, content preserved" grep -q geheim-token "${D}/config.json"
check "code owned by root" no_foreign_owner "$R"
check "unit kept (token line)" grep -qx "Environment=SRA_API_TOKEN=lokal-gesetzt-1234567890" "$U"
check "unit migrated" grep -q -- "-config ${D}/config.json" "$U"
check "meta: REPO_DIR = local repo" grep -qx "REPO_DIR=/repo" "${R}/.install-meta"

echo "=== 6: sonos-agent update converts the old layout first ==="
make_legacy
out="$(sonos-agent update 2>&1)" || true
check "update calls migrate" grep -q "Old install layout detected" <<<"$out"
check "config migrated afterwards" test -f "${D}/config.json" -a ! -e "${R}/config.json"

echo "=== 7: admin's REPO_DIR (local clone as on the Pi) ==="
make_legacy
mkdir -p /home/admin/hallido/.git
chown -R admin:admin /home/admin/hallido
sed -i "s|^REPO_DIR=.*|REPO_DIR=/home/admin/hallido|" "${R}/.install-meta"
SUDO_UID="$(id -u admin)" bash "$CLI" migrate >/dev/null
check "caller's (sudo) clone stays REPO_DIR" grep -qx "REPO_DIR=/home/admin/hallido" "${R}/.install-meta"
make_legacy
sed -i "s|^REPO_DIR=.*|REPO_DIR=/home/admin/hallido|" "${R}/.install-meta"
bash "$CLI" migrate >/dev/null 2>&1
check "foreign clone (without sudo caller) is ignored" grep -qx "REPO_DIR=${R}/src" "${R}/.install-meta"

echo
echo "install-migration-docker: ${PASS} ok, ${FAIL} failed"
[[ "$FAIL" -eq 0 ]]
