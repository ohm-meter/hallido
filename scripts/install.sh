#!/usr/bin/env bash
# Hallido — one-line installer (curl | bash) and local installation.
#
# Remote:
#   /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/ohm-meter/hallido/master/scripts/install.sh)"
#
# Local (inside the repo):
#   ./scripts/install.sh
#
# Options:
#   --user          install under ~/.local (default without root)
#   --system        install under /usr/local (root or sudo)
#   --systemd       install the systemd unit (Linux only, --system only)
#   --no-build      only copy files (binary must exist)
#
# Layout --system (ADR 0017): code under /usr/local/sonos-agent is owned by root,
# config and backup live in /var/lib/sonos-agent (service user sonos).
# Existing installations on the old layout are migrated on the way.
set -euo pipefail

readonly PRODUCT_NAME="Hallido"
readonly CLI_NAME="sonos-agent"
readonly BINARY_NAME="sonos-radio-agent"
readonly REPO_URL="${SRA_REPO_URL:-https://github.com/ohm-meter/hallido.git}"
readonly REPO_BRANCH="master"

INSTALL_MODE=""
WANT_SYSTEMD=0
SKIP_BUILD=0

usage() {
  cat <<EOF
${PRODUCT_NAME} — Installer

Usage:
  ./scripts/install.sh [--user|--system] [--systemd] [--no-build]

Default without root: --user (~/.local/share/sonos-agent)
With root/sudo:       --system (/usr/local/sonos-agent, data in /var/lib/sonos-agent)

One-liner (remote):
  /bin/bash -c "\$(curl -fsSL https://raw.githubusercontent.com/ohm-meter/hallido/master/scripts/install.sh)"
EOF
}

log() { echo ">> $*" >&2; }
die() { echo "ERROR: $*" >&2; exit 1; }

# Find Go and set PATH for the build.
# sudo often uses a secure_path without /usr/local/go/bin, so check common paths explicitly.
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

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --user)    INSTALL_MODE="user"; shift ;;
      --system)  INSTALL_MODE="system"; shift ;;
      --systemd) WANT_SYSTEMD=1; shift ;;
      --no-build) SKIP_BUILD=1; shift ;;
      -h|--help) usage; exit 0 ;;
      *) die "Unknown option: $1" ;;
    esac
  done
}

is_piped_remote() {
  local src="${BASH_SOURCE[0]:-}"
  [[ "$src" == "bash" || "$src" == "-bash" || "$src" == "-" || "$src" == "/dev/fd/"* || "$src" == "/dev/stdin" ]]
}

resolve_repo_dir() {
  if [[ -n "${SONOS_AGENT_REPO_DIR:-}" && -d "${SONOS_AGENT_REPO_DIR}" ]]; then
    echo "${SONOS_AGENT_REPO_DIR}"
    return 0
  fi
  local src="${BASH_SOURCE[0]:-}"
  if [[ -n "$src" && -f "$src" ]]; then
    local maybe
    maybe="$(cd "$(dirname "$src")/.." && pwd)"
    if [[ -f "${maybe}/config.example.json" && -f "${maybe}/Makefile" ]]; then
      echo "$maybe"
      return 0
    fi
  fi
  return 1
}

detect_platform() {
  OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
  ARCH="$(uname -m)"
  case "${OS}/${ARCH}" in
    darwin/arm64|darwin/x86_64|darwin/amd64|linux/aarch64|linux/arm64|linux/armv7l|linux/armv6l|linux/x86_64|linux/amd64)
      ;;
    *)
      die "Platform ${OS}/${ARCH} is not supported."
      ;;
  esac
}

choose_install_paths() {
  if [[ -z "$INSTALL_MODE" ]]; then
    if [[ "$(id -u)" -eq 0 ]]; then
      INSTALL_MODE="system"
    else
      INSTALL_MODE="user"
    fi
  fi

  case "$INSTALL_MODE" in
    system)
      INSTALL_ROOT="/usr/local/sonos-agent"
      BIN_DIR="/usr/local/bin"
      DATA_DIR="/var/lib/sonos-agent"
      NEED_SUDO=""
      if [[ "$(id -u)" -ne 0 ]]; then
        if command -v sudo >/dev/null 2>&1; then
          NEED_SUDO="sudo"
        else
          die "A system install needs root or sudo."
        fi
      fi
      ;;
    user)
      INSTALL_ROOT="${HOME}/.local/share/sonos-agent"
      BIN_DIR="${HOME}/.local/bin"
      DATA_DIR="${INSTALL_ROOT}"
      NEED_SUDO=""
      mkdir -p "$BIN_DIR" "$INSTALL_ROOT"
      ;;
    *)
      die "Internal error: unknown INSTALL_MODE=${INSTALL_MODE}"
      ;;
  esac

  REPO_DIR="${INSTALL_ROOT}/src"
  BINARY_PATH="${INSTALL_ROOT}/bin/${BINARY_NAME}"
  CONFIG_PATH="${DATA_DIR}/config.json"
  CONFIG_EXAMPLE="${INSTALL_ROOT}/config.example.json"
  CLI_PATH="${BIN_DIR}/${CLI_NAME}"
  META_FILE="${INSTALL_ROOT}/.install-meta"
}

run_privileged() {
  if [[ -n "$NEED_SUDO" ]]; then
    "$NEED_SUDO" "$@"
  else
    "$@"
  fi
}

# Calling user (with sudo: SUDO_USER, otherwise the current user).
invoke_user() {
  if [[ -n "${SUDO_USER:-}" && "${SUDO_USER}" != "root" ]]; then
    echo "$SUDO_USER"
  else
    id -un
  fi
}

# Owner of the repo directory (for a local clone usually the normal user).
repo_dir_owner() {
  if [[ -d "$REPO_DIR" ]]; then
    stat -c '%U' "$REPO_DIR" 2>/dev/null || stat -f '%Su' "$REPO_DIR"
  else
    invoke_user
  fi
}

# Hand build/ back to the repo owner after a sudo build, otherwise sonos-agent update fails.
fix_repo_build_ownership() {
  local owner="${1:-$(repo_dir_owner)}"
  [[ -d "${REPO_DIR}/build" ]] || return 0
  if [[ "$(id -u)" -eq 0 && "$owner" != "root" ]]; then
    local group
    group="$(id -gn "$owner" 2>/dev/null || echo "$owner")"
    chown -R "${owner}:${group}" "${REPO_DIR}/build"
  fi
}

# System user for systemd (unprivileged, no login).
ensure_sonos_user() {
  if id -u sonos >/dev/null 2>&1; then
    return 0
  fi
  run_privileged useradd --system --no-create-home --shell /usr/sbin/nologin sonos
}

# The service user owns only the data directory (systemd installation).
service_owns_data() {
  [[ "$INSTALL_MODE" == "system" && "$WANT_SYSTEMD" -eq 1 ]]
}

have_systemd() {
  [[ -d /run/systemd/system ]] && command -v systemctl >/dev/null 2>&1
}

# Create the data directory: with systemd sonos:sonos 0750, otherwise root 0700.
ensure_data_dir() {
  [[ "$INSTALL_MODE" == "system" ]] || return 0
  [[ -L "$DATA_DIR" ]] && die "${DATA_DIR} is a symlink — please check."
  if service_owns_data; then
    ensure_sonos_user
    run_privileged install -d -m 0750 -o sonos -g sonos "$DATA_DIR"
  else
    run_privileged install -d -m 0700 "$DATA_DIR"
  fi
}

# Code is root-owned and not writable by others; the service writes only to
# the data directory (ADR 0017).
apply_install_permissions() {
  [[ "$INSTALL_MODE" == "system" ]] || return 0
  log "File permissions: ${INSTALL_ROOT} owned by root, data in ${DATA_DIR}"
  run_privileged chown -R -h root:0 "$INSTALL_ROOT"
  run_privileged chmod -R go-w "$INSTALL_ROOT"
  if [[ -f "$CONFIG_EXAMPLE" ]]; then
    run_privileged chmod 0644 "$CONFIG_EXAMPLE"
  fi
  if [[ -f "$META_FILE" ]]; then
    run_privileged chmod 0644 "$META_FILE"
  fi
  if [[ -f "$BINARY_PATH" ]]; then
    run_privileged chmod 0755 "$BINARY_PATH"
  fi
  ensure_data_dir
  if [[ -f "$CONFIG_PATH" && ! -L "$CONFIG_PATH" ]]; then
    if service_owns_data; then
      run_privileged chown -h sonos:sonos "$CONFIG_PATH"
    fi
    run_privileged chmod 0600 "$CONFIG_PATH"
  fi
}

install_config_file() {
  local src="$1"
  if service_owns_data; then
    ensure_sonos_user
    run_privileged install -m 0600 -o sonos -g sonos "$src" "$CONFIG_PATH"
  else
    run_privileged install -m 0600 "$src" "$CONFIG_PATH"
  fi
}

# Convert an old system installation (config in the code directory, service
# user owns /usr/local/sonos-agent) to the new layout. The logic lives in the
# CLI (sonos-agent migrate) so installer and update do the same thing.
migrate_legacy_layout() {
  [[ "$INSTALL_MODE" == "system" ]] || return 0
  [[ -e "${INSTALL_ROOT}/.install-meta" || -e "${INSTALL_ROOT}/config.json" ]] || return 0
  log "Checking/migrating existing system installation"
  run_privileged bash "${REPO_DIR}/scripts/sonos-agent-cli.sh" migrate
}

# Remote mode: a clone under INSTALL_ROOT that is not owned by root (old
# layout, service user) is not reused — root would otherwise run hooks or
# build files the service could have modified.
set_aside_untrusted_clone() {
  [[ "$INSTALL_MODE" == "system" ]] || return 0
  [[ "$REPO_DIR" == "${INSTALL_ROOT}/src" && -e "$REPO_DIR" ]] || return 0
  local uid
  uid="$(stat -c '%u' "$REPO_DIR" 2>/dev/null || stat -f '%u' "$REPO_DIR")"
  [[ "$uid" == "0" ]] && return 0
  local aside
  aside="${INSTALL_ROOT}/src.pre-migrate-$(date +%Y%m%d%H%M%S)"
  log "Clone ${REPO_DIR} is not owned by root — set aside: ${aside} (review and delete)"
  run_privileged mv "$REPO_DIR" "$aside"
  run_privileged chown -R -h root:0 "$aside"
  run_privileged chmod -R go-rwx "$aside"
}

ensure_safe_install_root() {
  case "$INSTALL_ROOT" in
    /usr/local/sonos-agent|"${HOME}/.local/share/sonos-agent")
      ;;
    *)
      die "INSTALL_ROOT outside allowed paths: ${INSTALL_ROOT}"
      ;;
  esac
}

clone_or_update_repo() {
  if [[ -d "${REPO_DIR}/.git" ]]; then
    log "Updating repository: ${REPO_DIR}"
    run_privileged git -C "$REPO_DIR" fetch origin "$REPO_BRANCH"
    run_privileged git -C "$REPO_DIR" checkout "$REPO_BRANCH"
    run_privileged git -C "$REPO_DIR" pull --ff-only origin "$REPO_BRANCH" || true
    return 0
  fi

  log "Cloning repository to ${REPO_DIR}"
  run_privileged mkdir -p "$(dirname "$REPO_DIR")"
  if [[ -n "$NEED_SUDO" && "$(id -u)" -ne 0 ]]; then
    run_privileged git clone --depth 1 --branch "$REPO_BRANCH" "$REPO_URL" "$REPO_DIR"
  else
    git clone --depth 1 --branch "$REPO_BRANCH" "$REPO_URL" "$REPO_DIR"
  fi
}

detect_build_target() {
  case "${OS}/${ARCH}" in
    darwin/arm64|darwin/x86_64|darwin/amd64|linux/x86_64|linux/amd64)
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
    *)
      die "No build target for ${OS}/${ARCH}"
      ;;
  esac
}

build_binary() {
  local build_target built owner
  build_target="$(detect_build_target)"
  log "Building binary (${build_target}) …"
  if ! prepare_go_path; then
    die "Go not found. Install Go 1.22+ (e.g. /usr/local/go) or put a prebuilt binary at ${BINARY_PATH} and use --no-build. Note: sudo sanitizes PATH — see docs/how-to/install-raspberry-pi.md"
  fi
  owner="$(repo_dir_owner)"
  # Remove the old artifact, otherwise a failed build silently installs the old binary.
  case "$build_target" in
    build-pi*) built="${REPO_DIR}/build/${BINARY_NAME}-pi" ;;
    *)         built="${REPO_DIR}/build/${BINARY_NAME}" ;;
  esac
  run_privileged rm -f "$built"
  (
    cd "$REPO_DIR"
    if [[ "$(id -u)" -ne 0 && ! -w "$REPO_DIR" ]]; then
      # root-owned clone (remote install under /usr/local/sonos-agent/src)
      # shellcheck disable=SC2086
      run_privileged env PATH="${PATH}" make $build_target >&2
    elif [[ "$(id -u)" -eq 0 && "$owner" != "root" ]]; then
      fix_repo_build_ownership "$owner"
      local user_home
      user_home="$(getent passwd "$owner" 2>/dev/null | cut -d: -f6 || echo "/home/${owner}")"
      sudo -u "$owner" env PATH="${PATH}" HOME="${user_home}" mkdir -p build
      # shellcheck disable=SC2086
      sudo -u "$owner" env PATH="${PATH}" HOME="${user_home}" make $build_target >&2
    else
      # shellcheck disable=SC2086
      make $build_target >&2
    fi
  )
  fix_repo_build_ownership "$owner"
  [[ -f "$built" ]] || die "Build failed: ${built} is missing"
  echo "$built"
}

install_files() {
  local built="${1:-}"

  log "Creating directories under ${INSTALL_ROOT}"
  run_privileged mkdir -p "${INSTALL_ROOT}/bin"
  ensure_data_dir

  if [[ -n "$built" ]]; then
    log "Installing binary"
    run_privileged install -m 0755 "$built" "$BINARY_PATH"
  elif [[ -f "$BINARY_PATH" ]]; then
    log "Keeping existing binary: ${BINARY_PATH}"
  else
    die "No binary found. Run the build or omit --no-build."
  fi

  log "Installing CLI ${CLI_NAME}"
  run_privileged install -m 0755 "${REPO_DIR}/scripts/sonos-agent-cli.sh" "$CLI_PATH"

  if ! run_privileged test -f "$CONFIG_PATH"; then
    log "Creating config.json from example"
    install_config_file "${REPO_DIR}/config.example.json"
  else
    log "Keeping existing config.json"
  fi
  run_privileged install -m 0644 "${REPO_DIR}/config.example.json" "$CONFIG_EXAMPLE"

  local git_commit=""
  if [[ -d "${REPO_DIR}/.git" ]]; then
    git_commit="$(git -C "$REPO_DIR" rev-parse --short HEAD 2>/dev/null || true)"
  fi

  log "Writing install metadata"
  run_privileged tee "$META_FILE" >/dev/null <<METAEOF
INSTALL_ROOT=${INSTALL_ROOT}
BIN_DIR=${BIN_DIR}
REPO_DIR=${REPO_DIR}
BINARY_PATH=${BINARY_PATH}
DATA_DIR=${DATA_DIR}
CONFIG_PATH=${CONFIG_PATH}
CLI_PATH=${CLI_PATH}
GIT_BRANCH=${REPO_BRANCH}
GIT_COMMIT=${git_commit}
INSTALLED_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
OS=${OS}
ARCH=${ARCH}
LAYOUT=2
METAEOF
  run_privileged chmod 0644 "$META_FILE"
}

install_systemd_linux() {
  [[ "$OS" == "linux" ]] || return 0
  [[ "$INSTALL_MODE" == "system" ]] || return 0
  [[ "$WANT_SYSTEMD" -eq 1 ]] || return 0

  local service_name="sonos-agent"
  local unit_src="${REPO_DIR}/deploy/systemd/sonos-agent.service"
  [[ -f "$unit_src" ]] || die "systemd unit missing: ${unit_src}"

  local unit_dst="/etc/systemd/system/${service_name}.service"
  ensure_sonos_user
  if [[ -f "$unit_dst" ]]; then
    # Do not overwrite custom lines (e.g. Environment=SRA_API_TOKEN=…);
    # migrate has already adjusted the paths.
    log "systemd unit exists, keeping it: ${unit_dst} (template: ${unit_src})"
  else
    log "Installing systemd unit (${service_name})"
    run_privileged install -m 0644 "$unit_src" "$unit_dst"
  fi
  if have_systemd; then
    run_privileged systemctl daemon-reload
  fi
  log "Enable the service with:"
  echo "   sudo systemctl enable --now ${service_name}"
}

maybe_prompt_token() {
  if ! run_privileged test -f "$CONFIG_PATH"; then
    return 0
  fi
  if ! run_privileged grep -q 'BITTE-AENDERN' "$CONFIG_PATH" 2>/dev/null; then
    return 0
  fi

  echo
  echo "NOTE: api_token in ${CONFIG_PATH} is still BITTE-AENDERN."
  if [[ -t 0 ]]; then
    read -r -p "Set your own API token now? [y/N] " answer || true
    case "${answer:-}" in
      [jJyY]*)
      read -r -s -p "New token (input hidden, empty = generate a random one): " token
      echo
      if [[ -z "$token" ]] && command -v openssl >/dev/null 2>&1; then
        token="$(openssl rand -hex 24)"
        echo "Random token generated (stored in ${CONFIG_PATH})."
      fi
      if [[ -n "$token" && "$token" != "BITTE-AENDERN" && ${#token} -ge 16 && "$token" =~ ^[A-Za-z0-9._~+=-]+$ ]]; then
        local tmp
        tmp="$(mktemp)"
        chmod 0600 "$tmp"
        run_privileged sed "s/BITTE-AENDERN/${token}/" "$CONFIG_PATH" > "$tmp"
        install_config_file "$tmp"
        rm -f "$tmp"
        log "api_token set."
      else
        echo "Token left unchanged (needs at least 16 characters from A-Z a-z 0-9 . _ ~ + = -)."
      fi
      ;;
    esac
  fi
}

print_next_steps() {
  echo
  echo "=== ${PRODUCT_NAME} installed ==="
  echo
  echo "  Binary:  ${BINARY_PATH}"
  echo "  CLI:     ${CLI_PATH}"
  echo "  Config:  ${CONFIG_PATH}"
  echo
  if [[ "$INSTALL_MODE" == "user" ]]; then
    case ":${PATH}:" in
      *":${BIN_DIR}:"*) ;;
      *)
        echo "  Add to PATH (in ~/.bashrc or ~/.zshrc):"
        echo "    export PATH=\"${BIN_DIR}:\$PATH\""
        echo
        ;;
    esac
  fi
  if run_privileged grep -q 'BITTE-AENDERN' "$CONFIG_PATH" 2>/dev/null; then
    echo "  Next step: set api_token"
    if [[ "$INSTALL_MODE" == "system" ]]; then
      echo "    sudo nano ${CONFIG_PATH}"
    else
      echo "    nano ${CONFIG_PATH}"
    fi
    echo "    or: export SRA_API_TOKEN=your-token"
    echo
  fi
  echo "  Commands:"
  echo "    ${CLI_NAME} version"
  echo "    ${CLI_NAME} doctor"
  echo "    ${CLI_NAME} update"
  echo
  if [[ "$OS" == "linux" && "$INSTALL_MODE" == "system" && "$WANT_SYSTEMD" -eq 1 ]]; then
    echo "  Driver: sonos (active in /etc/systemd/system/sonos-agent.service)"
    echo "    sudo systemctl enable --now sonos-agent"
    echo
  elif [[ "$OS" == "darwin" ]]; then
    echo "  Run locally (dev):"
    echo "    SRA_DRIVER=sonos ${BINARY_PATH} -config ${CONFIG_PATH}"
    echo "    or in the cloned repo: make dev"
    echo
  fi
  echo "  Docs: docs/how-to/install-one-liner.md"
}

main() {
  parse_args "$@"
  detect_platform
  choose_install_paths
  ensure_safe_install_root

  local repo_from_env=""
  if repo_from_env="$(resolve_repo_dir 2>/dev/null)"; then
    REPO_DIR="$repo_from_env"
    log "Local repository: ${REPO_DIR}"
  else
    if is_piped_remote; then
      log "Remote installation detected"
    fi
    if ! command -v git >/dev/null 2>&1; then
      die "git not found — required for a remote install."
    fi
    set_aside_untrusted_clone
    clone_or_update_repo
  fi

  [[ -f "${REPO_DIR}/scripts/sonos-agent-cli.sh" ]] || die "Repo incomplete (CLI script missing)."

  local built=""
  if [[ "$SKIP_BUILD" -eq 0 ]]; then
    built="$(build_binary)"
  fi

  migrate_legacy_layout
  install_files "$built"
  install_systemd_linux
  maybe_prompt_token
  apply_install_permissions
  print_next_steps
}

main "$@"
