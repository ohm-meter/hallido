#!/usr/bin/env bash
# Hallido — CLI wrapper (installed as sonos-agent in PATH).
# Subcommands: update, version, status, doctor, migrate
set -euo pipefail

readonly PRODUCT_NAME="Hallido"
readonly CLI_NAME="sonos-agent"
readonly BINARY_NAME="sonos-radio-agent"
# shellcheck disable=SC2034  # for future update paths
readonly REPO_URL="${SRA_REPO_URL:-https://github.com/ohm-meter/hallido.git}"
readonly REPO_BRANCH="master"

# System layout (ADR 0017): code owned by root, service data kept separate.
readonly SYSTEM_ROOT="/usr/local/sonos-agent"
readonly SYSTEM_DATA_DIR="/var/lib/sonos-agent"
readonly SYSTEM_BIN_DIR="/usr/local/bin"
readonly SYSTEM_UNIT="/etc/systemd/system/sonos-agent.service"
readonly SERVICE_USER="sonos"
readonly LAYOUT_VERSION="2"

# Allowed keys in .install-meta (everything else is ignored).
readonly META_KEYS=" INSTALL_ROOT BIN_DIR REPO_DIR BINARY_PATH CONFIG_PATH DATA_DIR CLI_PATH GIT_BRANCH GIT_COMMIT INSTALLED_AT UPDATED_AT OS ARCH LAYOUT "

die() { echo "ERROR: $*" >&2; exit 1; }
warn() { echo "WARNING: $*" >&2; }

# Find Go and set PATH for the build (sudo secure_path without /usr/local/go/bin).
prepare_go_path() {
  if command -v go >/dev/null 2>&1; then
    return 0
  fi
  local candidates=(
    /usr/local/go/bin/go
    /usr/bin/go
  )
  local sudohome=""
  if [[ -n "${SUDO_USER:-}" && "${SUDO_USER}" != "root" ]]; then
    sudohome="$(getent passwd "$SUDO_USER" 2>/dev/null | cut -d: -f6 || true)"
  fi
  if [[ -n "$sudohome" ]]; then
    candidates+=(
      "${sudohome}/go/bin/go"
      "${sudohome}/.local/go/bin/go"
    )
  elif [[ -n "${HOME:-}" ]]; then
    candidates+=(
      "${HOME}/go/bin/go"
      "${HOME}/.local/go/bin/go"
    )
  fi
  local c
  for c in "${candidates[@]}"; do
    if [[ -x "$c" ]]; then
      PATH="$(dirname "$c"):${PATH}"
      export PATH
      return 0
    fi
  done
  return 1
}

usage() {
  cat <<EOF
${PRODUCT_NAME} — ${CLI_NAME}

Usage:
  ${CLI_NAME} update [--no-restart]
                        Git pull, build, install; restart the systemd service
  ${CLI_NAME} update --no-restart
                        Like update, but without restarting the service
  ${CLI_NAME} version   Show the installed version
  ${CLI_NAME} status    Local health check (GET /api/v1/health)
  ${CLI_NAME} doctor    Check installation and configuration
  ${CLI_NAME} migrate   Convert a system installation to the separated layout
                        (config to ${SYSTEM_DATA_DIR}, code owned by root; root/sudo)

Environment:
  SONOS_AGENT_INSTALL_ROOT  Install directory (overrides auto-detection)
EOF
}

find_meta_file() {
  local candidates=()
  if [[ -n "${SONOS_AGENT_INSTALL_ROOT:-}" ]]; then
    candidates+=("${SONOS_AGENT_INSTALL_ROOT}/.install-meta")
  fi
  candidates+=(
    "/usr/local/sonos-agent/.install-meta"
    "${HOME}/.local/share/sonos-agent/.install-meta"
  )
  local c
  for c in "${candidates[@]}"; do
    if [[ -f "$c" ]]; then
      echo "$c"
      return 0
    fi
  done
  return 1
}

# Read .install-meta as KEY=VALUE — never execute it (no source).
# Known keys only, values taken literally (no expansion).
parse_meta() {
  local file="$1" line key
  while IFS= read -r line || [[ -n "$line" ]]; do
    [[ "$line" =~ ^([A-Z_]+)=(.*)$ ]] || continue
    key="${BASH_REMATCH[1]}"
    [[ "$META_KEYS" == *" ${key} "* ]] || continue
    printf -v "$key" '%s' "${BASH_REMATCH[2]}"
  done < "$file"
}

path_uid() { stat -c '%u' "$1" 2>/dev/null || stat -f '%u' "$1"; }
path_gid() { stat -c '%g' "$1" 2>/dev/null || stat -f '%g' "$1"; }
path_mode() { stat -c '%a' "$1" 2>/dev/null || stat -f '%Lp' "$1"; }

# File/directory is owned by root or the calling user, is not a symlink, is not
# world-writable, and is group-writable only for root or the user's own group.
# Otherwise another user (e.g. the service) could plant values the admin then
# uses.
is_trusted_path() {
  local p="$1" uid gid mode
  [[ -e "$p" && ! -L "$p" ]] || return 1
  uid="$(path_uid "$p")" || return 1
  [[ "$uid" == "0" || "$uid" == "$(id -u)" ]] || return 1
  mode="$(path_mode "$p")" || return 1
  (( (8#$mode & 8#002) == 0 )) || return 1
  if (( (8#$mode & 8#020) != 0 )); then
    gid="$(path_gid "$p")" || return 1
    [[ "$gid" == "0" || "$gid" == "$(id -g)" ]] || return 1
  fi
  return 0
}

meta_trusted() {
  is_trusted_path "$1" && is_trusted_path "$(dirname "$1")"
}

# Is the system installation still on the old layout (config in the code
# directory, or directory/metadata not owned by root)?
system_needs_migration() {
  local meta="${SYSTEM_ROOT}/.install-meta"
  [[ -d "$SYSTEM_ROOT" ]] || return 1
  [[ -e "${SYSTEM_ROOT}/config.json" || -e "${SYSTEM_ROOT}/config.json.bak" ]] && return 0
  [[ -f "$meta" ]] || return 1
  [[ "$(path_uid "$SYSTEM_ROOT")" == "0" && "$(path_uid "$meta")" == "0" ]] || return 0
  meta_trusted "$meta" || return 0
  ( parse_meta "$meta"; [[ "${CONFIG_PATH:-}" != "${SYSTEM_DATA_DIR}/config.json" ]] )
}

load_meta() {
  META_FILE="$(find_meta_file)" || {
    echo "ERROR: No installation found (.install-meta)."
    echo "Reinstall with:"
    # shellcheck disable=SC2016
    echo '  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/ohm-meter/hallido/master/scripts/install.sh)"'
    exit 1
  }
  if ! meta_trusted "$META_FILE"; then
    echo "ERROR: ${META_FILE} is not loaded: file or directory is not owned by root or $(id -un), or is writable by others." >&2
    if [[ "$META_FILE" == "${SYSTEM_ROOT}/.install-meta" ]]; then
      echo "Old install layout (the service user owns the code directory). Convert with:" >&2
      echo "  sudo ${CLI_NAME} migrate" >&2
    fi
    exit 1
  fi
  parse_meta "$META_FILE"
  : "${INSTALL_ROOT:?}"
  : "${REPO_DIR:?}"
  : "${BINARY_PATH:?}"
  : "${CONFIG_PATH:?}"
  DATA_DIR="${DATA_DIR:-$(dirname "$CONFIG_PATH")}"
}

# Content of .install-meta from the current variables.
meta_content() {
  cat <<METAEOF
INSTALL_ROOT=${INSTALL_ROOT}
BIN_DIR=${BIN_DIR:-}
REPO_DIR=${REPO_DIR}
BINARY_PATH=${BINARY_PATH}
DATA_DIR=${DATA_DIR}
CONFIG_PATH=${CONFIG_PATH}
CLI_PATH=${CLI_PATH:-}
GIT_BRANCH=${GIT_BRANCH:-${REPO_BRANCH}}
GIT_COMMIT=${GIT_COMMIT:-}
INSTALLED_AT=${INSTALLED_AT:-}
UPDATED_AT=${UPDATED_AT:-}
OS=${OS:-$(uname -s | tr '[:upper:]' '[:lower:]')}
ARCH=${ARCH:-$(uname -m)}
LAYOUT=${LAYOUT_VERSION}
METAEOF
}

current_user() {
  id -un
}

repo_build_dir() {
  echo "${REPO_DIR}/build"
}

# build/ must not stay root-owned after sudo install.sh.
ensure_repo_build_writable() {
  local build_dir owner
  build_dir="$(repo_build_dir)"
  mkdir -p "$build_dir" 2>/dev/null || true
  if [[ -w "$build_dir" ]]; then
    return 0
  fi
  if [[ -d "$build_dir" ]] && command -v sudo >/dev/null 2>&1; then
    owner="$(stat -c '%U' "$build_dir" 2>/dev/null || stat -f '%Su' "$build_dir")"
    if [[ "$owner" == "root" ]]; then
      echo ">> Repairing root-owned build/ (after sudo install.sh) …"
      sudo chown -R "$(current_user):$(id -gn)" "$build_dir"
      if [[ -w "$build_dir" ]]; then
        return 0
      fi
    fi
  fi
  echo "ERROR: No write permission on ${build_dir} (probably root-owned after sudo install.sh)." >&2
  echo "Quick fix:" >&2
  echo "  sudo chown -R \$(whoami) \"${build_dir}\"" >&2
  echo "Alternatively:" >&2
  echo "  sudo rm -rf \"${build_dir}\"" >&2
  exit 1
}

# System paths (/usr/local/sonos-agent, CLI) belong to root — always write via sudo.
is_system_path() {
  local path="$1"
  case "$path" in
    /usr/local/sonos-agent/*|/usr/local/bin/sonos-agent|/var/lib/sonos-agent/*)
      return 0
      ;;
  esac
  return 1
}

can_install_without_sudo() {
  local dest="$1" dest_dir
  dest_dir="$(dirname "$dest")"
  is_system_path "$dest" && return 1
  [[ -w "$dest_dir" ]] || return 1
  [[ ! -e "$dest" ]] || [[ -w "$dest" ]] || return 1
  return 0
}

install_if_needed() {
  local src="$1" dest="$2" mode="$3"
  if can_install_without_sudo "$dest"; then
    install -m "$mode" "$src" "$dest"
  elif command -v sudo >/dev/null 2>&1; then
    sudo install -m "$mode" "$src" "$dest"
  else
    echo "ERROR: No write permission on ${dest} — sudo required." >&2
    exit 1
  fi
}

write_meta_file() {
  local content="$1"
  if can_install_without_sudo "$META_FILE"; then
    printf '%s\n' "$content" > "$META_FILE"
    chmod 0644 "$META_FILE"
  elif command -v sudo >/dev/null 2>&1; then
    printf '%s\n' "$content" | sudo tee "$META_FILE" >/dev/null
    sudo chmod 0644 "$META_FILE"
  else
    echo "ERROR: No write permission on ${META_FILE} — sudo required." >&2
    exit 1
  fi
}

detect_build_target() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "${os}/${arch}" in
    darwin/arm64|darwin/x86_64|darwin/amd64)
      echo "build"
      ;;
    linux/aarch64|linux/arm64)
      echo "build-pi PI_ARCH=arm64"
      ;;
    linux/armv7l)
      echo "build-pi PI_ARCH=arm PI_ARM=7"
      ;;
    linux/armv6l)
      # Pi Zero/1: ARMv6 — a GOARM=7 binary does not run there
      echo "build-pi PI_ARCH=arm PI_ARM=6"
      ;;
    linux/x86_64|linux/amd64)
      echo "build"
      ;;
    *)
      echo "ERROR: Platform ${os}/${arch} is not supported." >&2
      exit 1
      ;;
  esac
}

cmd_version() {
  load_meta
  echo "${PRODUCT_NAME}"
  echo "  CLI:        ${CLI_NAME}"
  echo "  Binary:     ${BINARY_PATH}"
  echo "  Install:    ${INSTALL_ROOT}"
  echo "  Data:       ${DATA_DIR}"
  if [[ -f "$BINARY_PATH" ]]; then
    echo "  Binary-MD5: $(md5 -q "$BINARY_PATH" 2>/dev/null || md5sum "$BINARY_PATH" | awk '{print $1}')"
  fi
  if [[ -n "${GIT_COMMIT:-}" ]]; then
    echo "  Git:        ${GIT_BRANCH:-?} @ ${GIT_COMMIT}"
  fi
  if [[ -n "${INSTALLED_AT:-}" ]]; then
    echo "  Installed:  ${INSTALLED_AT}"
  fi
}

read_listen_host_port() {
  local listen=":8080"
  if [[ -r "$CONFIG_PATH" ]]; then
    listen="$(grep -E '"listen"' "$CONFIG_PATH" | head -1 | sed -E 's/.*"listen"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')"
  fi
  local host="127.0.0.1" port="8080"
  if [[ "$listen" =~ ^:([0-9]+)$ ]]; then
    port="${BASH_REMATCH[1]}"
  elif [[ "$listen" =~ ^([^:]+):([0-9]+)$ ]]; then
    host="${BASH_REMATCH[1]}"
    port="${BASH_REMATCH[2]}"
  elif [[ "$listen" =~ ^[0-9]+$ ]]; then
    port="$listen"
  fi
  if [[ -z "$host" || "$host" == "0.0.0.0" || "$host" == "[::]" ]]; then
    host="127.0.0.1"
  fi
  echo "${host}:${port}"
}

cmd_status() {
  load_meta
  if ! command -v curl >/dev/null 2>&1; then
    echo "ERROR: curl not found."
    exit 1
  fi
  local endpoint
  endpoint="http://$(read_listen_host_port)/api/v1/health"
  echo ">> Health: ${endpoint}"
  if curl -fsS --connect-timeout 3 "$endpoint"; then
    echo
    exit 0
  fi
  echo
  echo "ERROR: Agent is not responding (is the service running? is the port reachable?)."
  exit 1
}

cmd_doctor() {
  load_meta
  local ok=0 issues=0

  check_ok() { echo "  OK   $*"; }
  check_warn() { echo "  WARN $*"; issues=$((issues + 1)); }
  check_fail() { echo "  FAIL $*"; ok=1; issues=$((issues + 1)); }

  echo ">> ${PRODUCT_NAME} — doctor"
  echo

  if [[ -f "$BINARY_PATH" && -x "$BINARY_PATH" ]]; then
    check_ok "Binary present: ${BINARY_PATH}"
  else
    check_fail "Binary missing or not executable: ${BINARY_PATH}"
  fi

  if [[ -r "$CONFIG_PATH" ]]; then
    check_ok "Config present: ${CONFIG_PATH}"
    if grep -q 'BITTE-AENDERN' "$CONFIG_PATH" 2>/dev/null; then
      check_warn "api_token is still the placeholder (BITTE-AENDERN) — set a token or use SRA_API_TOKEN"
    else
      check_ok "api_token appears to be set (not the placeholder)"
    fi
  elif [[ -d "$DATA_DIR" && ! -x "$DATA_DIR" ]] || [[ -e "$CONFIG_PATH" ]]; then
    check_ok "Config readable only by the service: ${CONFIG_PATH} (check contents: sudo ${CLI_NAME} doctor)"
  else
    check_fail "Config missing: ${CONFIG_PATH}"
  fi

  if [[ "$INSTALL_ROOT" == "$SYSTEM_ROOT" ]]; then
    if system_needs_migration; then
      check_fail "Old install layout — convert with: sudo ${CLI_NAME} migrate"
    else
      check_ok "Layout: code owned by root (${INSTALL_ROOT}), data in ${DATA_DIR}"
    fi
  fi

  if [[ -d "$REPO_DIR/.git" ]]; then
    check_ok "Git source: ${REPO_DIR}"
  else
    check_warn "No git clone under ${REPO_DIR} — sonos-agent update not possible"
  fi

  local port host_port
  host_port="$(read_listen_host_port)"
  port="${host_port##*:}"
  if command -v lsof >/dev/null 2>&1; then
    if lsof -iTCP:"$port" -sTCP:LISTEN -P -n >/dev/null 2>&1; then
      check_ok "Port ${port} is listening"
    else
      check_warn "Port ${port} is not listening — agent probably stopped"
    fi
  else
    check_warn "lsof not available — port check skipped"
  fi

  local driver="${SRA_DRIVER:-}"
  if [[ -z "$driver" && -f /etc/systemd/system/sonos-agent.service ]]; then
    driver="$(grep -E '^Environment=SRA_DRIVER=' /etc/systemd/system/sonos-agent.service 2>/dev/null | head -1 | cut -d= -f2- || true)"
  fi
  driver="${driver:-mock}"
  if [[ "$driver" == "sonos" ]]; then
    check_ok "Driver note: sonos (production)"
  else
    check_warn "Driver note: ${driver} — set SRA_DRIVER=sonos for real speakers"
  fi

  echo
  if [[ "$ok" -ne 0 ]]; then
    echo "Result: ${issues} problem(s) — see FAIL/WARN above."
    exit 1
  fi
  echo "Result: installation looks fine (${issues} note(s))."
}

systemd_service_name() {
  if [[ -f /etc/systemd/system/sonos-agent.service ]]; then
    echo "sonos-agent"
  elif [[ -f /etc/systemd/system/sonos-radio-agent.service ]]; then
    echo "sonos-radio-agent"
  fi
}

run_systemctl() {
  if command -v sudo >/dev/null 2>&1; then
    sudo systemctl "$@"
  else
    systemctl "$@"
  fi
}

restart_service_after_update() {
  local no_restart="$1"
  local svc
  svc="$(systemd_service_name || true)"
  if [[ -z "$svc" ]]; then
    echo ">> No systemd service — restart the agent manually if it is running."
    return 0
  fi
  if [[ "$no_restart" == "1" ]]; then
    echo ">> Restart skipped (--no-restart). Manually:"
    echo "   sudo systemctl restart ${svc}"
    return 0
  fi
  echo ">> Restarting service (${svc}) …"
  run_systemctl restart "$svc"
  sleep 1
  if run_systemctl is-active --quiet "$svc"; then
    echo ">> Service is running (${svc})."
    return 0
  fi
  echo "ERROR: Service ${svc} is not active — check with: sudo systemctl status ${svc}" >&2
  exit 1
}

cmd_update() {
  local no_restart=0
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --no-restart)
        no_restart=1
        shift
        ;;
      -h|--help)
        cat <<EOF
${CLI_NAME} update — git pull, build, install

Options:
  --no-restart   Update binary/CLI, skip the systemd restart
EOF
        exit 0
        ;;
      *)
        echo "Unknown option: $1" >&2
        echo "Usage: ${CLI_NAME} update [--no-restart]" >&2
        exit 1
        ;;
    esac
  done

  if [[ "$(find_meta_file 2>/dev/null || true)" == "${SYSTEM_ROOT}/.install-meta" ]] && system_needs_migration; then
    echo ">> Old install layout detected — converting (sudo ${CLI_NAME} migrate) …"
    run_as_root "$(self_path)" migrate
  fi

  load_meta
  if [[ ! -d "$REPO_DIR/.git" ]]; then
    echo "ERROR: No git repository under ${REPO_DIR}."
    echo "Reinstall with install.sh or check REPO_DIR in .install-meta."
    exit 1
  fi
  if [[ ! -w "$REPO_DIR" && "$(id -u)" -ne 0 ]]; then
    echo "ERROR: No write permission on ${REPO_DIR} (owned by root)."
    echo "Update with: sudo ${CLI_NAME} update"
    exit 1
  fi
  if ! prepare_go_path; then
    echo "ERROR: Go not found — required to update from source."
    echo "Note: sudo sanitizes PATH; install Go under /usr/local/go or see docs/how-to/install-raspberry-pi.md"
    echo "Note: GitHub releases (prebuilt binaries) are planned; until then, install Go."
    exit 1
  fi
  if ! command -v git >/dev/null 2>&1; then
    echo "ERROR: git not found."
    exit 1
  fi

  echo ">> ${PRODUCT_NAME} — update"
  echo ">> Repository: ${REPO_DIR}"
  cd "$REPO_DIR"
  git fetch origin "$REPO_BRANCH"
  git checkout "$REPO_BRANCH"
  git pull --ff-only origin "$REPO_BRANCH"

  local build_target new_commit
  build_target="$(detect_build_target)"
  new_commit="$(git rev-parse --short HEAD)"

  ensure_repo_build_writable
  echo ">> Building (${build_target}) …"
  # shellcheck disable=SC2086
  make $build_target

  local built=""
  case "$build_target" in
    build-pi*)
      built="${REPO_DIR}/build/${BINARY_NAME}-pi"
      ;;
    *)
      built="${REPO_DIR}/build/${BINARY_NAME}"
      ;;
  esac
  if [[ ! -f "$built" ]]; then
    echo "ERROR: Build artifact not found: ${built}"
    exit 1
  fi

  install_if_needed "$built" "$BINARY_PATH" 0755
  echo ">> Binary updated: ${BINARY_PATH}"

  if [[ -n "${CLI_PATH:-}" && -f "${REPO_DIR}/scripts/sonos-agent-cli.sh" ]]; then
    install_if_needed "${REPO_DIR}/scripts/sonos-agent-cli.sh" "$CLI_PATH" 0755
    echo ">> CLI updated: ${CLI_PATH}"
  fi

  GIT_BRANCH="$REPO_BRANCH"
  GIT_COMMIT="$new_commit"
  UPDATED_AT="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  write_meta_file "$(meta_content)"

  echo ">> Version: ${new_commit}"
  restart_service_after_update "$no_restart"
  echo ">> Done."
}

run_as_root() {
  if [[ "$(id -u)" -eq 0 ]]; then
    "$@"
  elif command -v sudo >/dev/null 2>&1; then
    sudo "$@"
  else
    die "root privileges required (sudo missing): $*"
  fi
}

# Absolute path of this script (for the sudo call of migrate).
self_path() {
  local src="${BASH_SOURCE[0]}"
  echo "$(cd "$(dirname "$src")" && pwd)/$(basename "$src")"
}

have_systemd() {
  [[ -d /run/systemd/system ]] && command -v systemctl >/dev/null 2>&1
}

# Rewrite the systemd unit for the new layout (stdin -> stdout). Custom
# lines (e.g. Environment=SRA_API_TOKEN=…) are preserved.
rewrite_unit() {
  awk -v old="$SYSTEM_ROOT" -v data="$SYSTEM_DATA_DIR" '
    function repl(s, from, to,   i) {
      i = index(s, from)
      if (i == 0) return s
      return substr(s, 1, i - 1) to substr(s, i + length(from))
    }
    {
      line = $0
      if (line == "WorkingDirectory=" old || line == "WorkingDirectory=" old "/") {
        line = "WorkingDirectory=" data
      } else if (line == "ReadWritePaths=" old || line == "ReadWritePaths=" old "/") {
        next
      } else if (line ~ /^ExecStart=/) {
        line = repl(line, "-config " old "/config.json", "-config " data "/config.json")
      }
      if (line ~ /^StateDirectory=/) has_state = 1
      out[++n] = line
      if (line == "[Service]") service_at = n
    }
    END {
      for (i = 1; i <= n; i++) {
        print out[i]
        if (i == service_at && !has_state) {
          print "StateDirectory=sonos-agent"
          print "StateDirectoryMode=0750"
        }
      }
    }
  '
}

# A git clone that belonged to the service user must not contain hooks or
# executable git configuration before root takes it over.
check_repo_untampered() {
  local repo="$1" hook bad=0
  [[ -d "${repo}/.git" ]] || return 0
  if [[ -d "${repo}/.git/hooks" ]]; then
    for hook in "${repo}"/.git/hooks/*; do
      [[ -e "$hook" || -L "$hook" ]] || continue
      [[ "$hook" == *.sample ]] && continue
      echo "  suspicious: git hook ${hook}" >&2
      bad=1
    done
  fi
  if [[ -f "${repo}/.git/config" ]] && grep -inE '^[[:space:]]*(hookspath|fsmonitor|sshcommand|pager|editor|askpass|gitproxy|helper|include|path)[[:space:]]*=|^\[(filter|alias|include|includeif|diff|merge)' "${repo}/.git/config" >&2; then
    echo "  suspicious: executable entries in ${repo}/.git/config (see above)" >&2
    bad=1
  fi
  if [[ -f "${repo}/.git/config" ]] && ! grep -qiE '^[[:space:]]*url[[:space:]]*=[[:space:]]*https://' "${repo}/.git/config"; then
    echo "  suspicious: origin is not an HTTPS remote (${repo}/.git/config)" >&2
    bad=1
  fi
  return "$bad"
}

# Take REPO_DIR from the old (service-owned) metadata only if the directory
# itself is trustworthy or is about to become root-owned.
pick_repo_dir() {
  local candidate="$1" uid
  if [[ -z "$candidate" || "$candidate" != /* || "$candidate" == *..* || -L "$candidate" || ! -d "$candidate" ]]; then
    echo "${SYSTEM_ROOT}/src"
    return 0
  fi
  if [[ "$candidate" == "${SYSTEM_ROOT}/src" ]]; then
    echo "$candidate"
    return 0
  fi
  uid="$(path_uid "$candidate")"
  if [[ "$uid" == "0" || "$uid" == "${SUDO_UID:-}" ]] && (( (8#$(path_mode "$candidate") & 8#002) == 0 )); then
    echo "$candidate"
    return 0
  fi
  warn "REPO_DIR=${candidate} from old .install-meta ignored (owned by neither root nor the caller)."
  echo "${SYSTEM_ROOT}/src"
}

# Move one service file (config.json, config.json.bak) into the data directory.
migrate_data_file() {
  local name="$1" owner="$2" src dst
  src="${SYSTEM_ROOT}/${name}"
  dst="${SYSTEM_DATA_DIR}/${name}"
  [[ -e "$src" || -L "$src" ]] || return 0
  if [[ -L "$src" || ! -f "$src" ]]; then
    warn "${src} is not a regular file — not migrated, removed."
    rm -f "$src"
    return 0
  fi
  if [[ -e "$dst" || -L "$dst" ]]; then
    warn "${dst} already exists — old file kept as ${src}.pre-migrate (root, 0600)."
    mv -f "$src" "${src}.pre-migrate"
    chmod 0600 "${src}.pre-migrate"
    return 0
  fi
  mv "$src" "$dst"
  chown -h "$owner" "$dst"
  chmod 0600 "$dst"
  echo ">> ${name} -> ${dst}"
}

cmd_migrate() {
  [[ $# -eq 0 ]] || die "migrate takes no options."
  [[ "$(id -u)" -eq 0 ]] || die "migrate needs root: sudo ${CLI_NAME} migrate"
  [[ -d "$SYSTEM_ROOT" && ! -L "$SYSTEM_ROOT" ]] || die "No system installation under ${SYSTEM_ROOT}."

  echo ">> ${PRODUCT_NAME} — migrate (code owned by root, data in ${SYSTEM_DATA_DIR})"
  if ! system_needs_migration && ! grep -qs "${SYSTEM_ROOT}/config.json" "$SYSTEM_UNIT"; then
    echo ">> Already on the new layout — nothing to do."
    return 0
  fi

  # Only read the old metadata, never execute it; paths are set explicitly.
  local meta="${SYSTEM_ROOT}/.install-meta"
  REPO_DIR="" GIT_COMMIT="" INSTALLED_AT="" UPDATED_AT="" OS="" ARCH=""
  if [[ -f "$meta" && ! -L "$meta" ]]; then
    parse_meta "$meta"
  fi
  REPO_DIR="$(pick_repo_dir "${REPO_DIR:-}")"
  if [[ "$REPO_DIR" == "${SYSTEM_ROOT}/src" && -d "$REPO_DIR" && "$(path_uid "$REPO_DIR")" != "0" ]]; then
    check_repo_untampered "$REPO_DIR" || die "Migration aborted: review or delete ${REPO_DIR} and reinstall (install.sh clones anew). Nothing changed."
  fi
  INSTALL_ROOT="$SYSTEM_ROOT"
  BIN_DIR="$SYSTEM_BIN_DIR"
  BINARY_PATH="${SYSTEM_ROOT}/bin/${BINARY_NAME}"
  DATA_DIR="$SYSTEM_DATA_DIR"
  CONFIG_PATH="${SYSTEM_DATA_DIR}/config.json"
  CLI_PATH="${SYSTEM_BIN_DIR}/${CLI_NAME}"
  GIT_BRANCH="$REPO_BRANCH"
  UPDATED_AT="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

  local was_active=0
  if have_systemd && systemctl is-active --quiet sonos-agent 2>/dev/null; then
    was_active=1
    echo ">> Stopping service sonos-agent …"
    systemctl stop sonos-agent
  fi

  local owner="root:0" dir_mode=0700
  if id -u "$SERVICE_USER" >/dev/null 2>&1; then
    owner="${SERVICE_USER}:${SERVICE_USER}"
    dir_mode=0750
  fi
  if [[ -L "$SYSTEM_DATA_DIR" ]]; then
    die "${SYSTEM_DATA_DIR} is a symlink — please check."
  fi
  install -d -m "$dir_mode" "$SYSTEM_DATA_DIR"
  chown -h "$owner" "$SYSTEM_DATA_DIR"
  chmod "$dir_mode" "$SYSTEM_DATA_DIR"

  migrate_data_file config.json "$owner"
  migrate_data_file config.json.bak "$owner"
  rm -f "${SYSTEM_ROOT}"/.config-*.tmp

  echo ">> Code directory owned by root: ${SYSTEM_ROOT}"
  chown -R -h root:0 "$SYSTEM_ROOT"
  chmod -R go-w "$SYSTEM_ROOT"

  printf '%s\n' "$(meta_content)" > "$meta"
  chmod 0644 "$meta"

  if [[ -f "$SYSTEM_UNIT" ]]; then
    local tmp
    tmp="$(mktemp)"
    rewrite_unit < "$SYSTEM_UNIT" > "$tmp"
    if ! cmp -s "$tmp" "$SYSTEM_UNIT"; then
      [[ -e "${SYSTEM_UNIT}.pre-migrate" ]] || cp -p "$SYSTEM_UNIT" "${SYSTEM_UNIT}.pre-migrate"
      install -m 0644 "$tmp" "$SYSTEM_UNIT"
      echo ">> Unit adjusted: ${SYSTEM_UNIT} (before: ${SYSTEM_UNIT}.pre-migrate)"
    fi
    rm -f "$tmp"
    if ! grep -q "^ExecStart=.*-config ${SYSTEM_DATA_DIR}/config.json" "$SYSTEM_UNIT"; then
      warn "ExecStart in ${SYSTEM_UNIT} not recognized — please set -config ${CONFIG_PATH} by hand."
    fi
    if have_systemd; then
      systemctl daemon-reload
    fi
  fi

  if [[ "$was_active" -eq 1 ]]; then
    echo ">> Starting service sonos-agent …"
    systemctl start sonos-agent
    sleep 1
    systemctl is-active --quiet sonos-agent || die "Service does not start — journalctl -u sonos-agent; rollback: docs/how-to/install-layout-migration.md"
  fi
  echo ">> Migration done. Config: ${CONFIG_PATH}"
}

main() {
  local cmd="${1:-}"
  case "$cmd" in
    update)  shift; cmd_update "$@" ;;
    version) shift; cmd_version "$@" ;;
    status)  shift; cmd_status "$@" ;;
    doctor)  shift; cmd_doctor "$@" ;;
    migrate) shift; cmd_migrate "$@" ;;
    -h|--help|help|"")
      usage
      [[ -z "$cmd" ]] && exit 0 || exit 0
      ;;
    *)
      echo "Unknown command: ${cmd}"
      usage
      exit 1
      ;;
  esac
}

# Do nothing when sourced (tests: source).
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
