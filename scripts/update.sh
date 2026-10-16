#!/usr/bin/env bash
#
# Updates an existing minilab-dashboard install to the latest GitHub release.
#
# Preserves whatever's already configured in the systemd unit -- it only
# asks interactively for values that are genuinely missing (e.g. a var
# added by a newer release that this install predates). Run install.sh
# first if minilab-dashboard isn't installed yet.
#
# Usage: run this directly on the target device (rocket):
#   ./scripts/update.sh
#
# Requires: curl, sudo, systemctl. Expects GitHub releases at
# github.com/robotjoosen/minilab-dashboard to publish a binary asset named
# minilab-dashboard-linux-arm64.

set -euo pipefail

REPO="robotjoosen/minilab-dashboard"
SERVICE_NAME="minilab_dashboard"
INSTALL_PATH="/usr/local/bin/minilab-dashboard"
UNIT_PATH="/etc/systemd/system/${SERVICE_NAME}.service"
VERSION_MARKER="/usr/local/bin/.minilab-dashboard-version"
DATA_DIR="/var/lib/minilab-dashboard"

# Canonical set of env vars minilab-dashboard expects, with their defaults
# from .env.dist -- used only for whatever's missing from the existing unit.
ENV_KEYS=(LOG_LEVEL HTTP_LISTEN_ADDR SQLITE_PATH MDNS_AGENT_SERVICE_NAME DISCOVERY_INTERVAL POLL_INTERVAL UNREACHABLE_THRESHOLD)
declare -A ENV_DEFAULTS=(
  [LOG_LEVEL]="INFO"
  [HTTP_LISTEN_ADDR]=":8080"
  [SQLITE_PATH]="./minilab-dashboard.db"
  [MDNS_AGENT_SERVICE_NAME]="_minilab-agent._tcp"
  [DISCOVERY_INTERVAL]="1m"
  [POLL_INTERVAL]="15s"
  [UNREACHABLE_THRESHOLD]="3"
)

info()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn()  { printf '\033[1;33m!!\033[0m %s\n' "$*"; }
error() { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; }

confirm() {
  # confirm "question" -- returns 0 (yes) or 1 (no). Defaults to no.
  # Reads from /dev/tty, not stdin -- when this script runs via
  # `curl ... | bash`, stdin is the script itself, not the terminal.
  local reply
  read -r -p "$1 [y/N] " reply < /dev/tty
  case "$reply" in
    [yY][eE][sS]|[yY]) return 0 ;;
    *) return 1 ;;
  esac
}

prompt_with_default() {
  # prompt_with_default "question" "default" -- echoes the chosen value.
  # Reads from /dev/tty for the same reason as confirm() above.
  local question="$1" default="$2" reply
  read -r -p "$question [$default]: " reply < /dev/tty
  if [ -z "$reply" ]; then
    echo "$default"
  else
    echo "$reply"
  fi
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    error "'$1' is required but not installed. Install it and re-run this script."
    exit 1
  fi
}

detect_arch() {
  case "$(uname -m)" in
    aarch64|arm64) echo "arm64" ;;
    *)
      error "unsupported architecture: $(uname -m) (minilab-dashboard only publishes linux/arm64 binaries)"
      exit 1
      ;;
  esac
}

fetch_latest_release_json() {
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" || {
    error "failed to reach GitHub releases API for ${REPO}"
    exit 1
  }
}

release_tag() {
  printf '%s' "$1" | grep -o '"tag_name": *"[^"]*"' | head -1 | sed -E 's/.*"([^"]+)"$/\1/'
}

release_asset_url() {
  local release_json="$1" goarch="$2" asset_name="minilab-dashboard-linux-${goarch}"
  local url
  url=$(printf '%s' "$release_json" | grep -o "\"browser_download_url\": *\"[^\"]*${asset_name}\"" | head -1 | sed -E 's/.*"([^"]+)"$/\1/')

  if [ -z "$url" ]; then
    error "no release asset named '${asset_name}' found on the latest ${REPO} release"
    error "(the release must publish a binary asset with exactly that name)"
    exit 1
  fi

  echo "$url"
}

# read_current_env UNIT_PATH -- populates the global CURRENT_ENV associative
# array from the Environment=KEY=VALUE lines in an existing systemd unit.
declare -A CURRENT_ENV
read_current_env() {
  local unit_path="$1" line
  while IFS= read -r line; do
    if [[ "$line" =~ ^Environment=([A-Z_]+)=(.*)$ ]]; then
      CURRENT_ENV["${BASH_REMATCH[1]}"]="${BASH_REMATCH[2]}"
    fi
  done < "$unit_path"
}

main() {
  require_cmd curl
  require_cmd sudo
  require_cmd systemctl

  if [ ! -f "$INSTALL_PATH" ] || [ ! -f "$UNIT_PATH" ]; then
    error "minilab-dashboard doesn't look installed yet (missing ${INSTALL_PATH} or ${UNIT_PATH})"
    error "run install.sh first"
    exit 1
  fi

  local goarch
  goarch=$(detect_arch)
  info "detected architecture: ${goarch}"

  info "looking up the latest minilab-dashboard release..."
  local release_json tag asset_url
  release_json=$(fetch_latest_release_json)
  tag=$(release_tag "$release_json")
  asset_url=$(release_asset_url "$release_json" "$goarch")
  info "latest release: ${tag}"

  local current_tag="unknown"
  if [ -f "$VERSION_MARKER" ]; then
    current_tag=$(cat "$VERSION_MARKER")
  fi
  info "currently installed: ${current_tag}"

  if [ "$current_tag" = "$tag" ]; then
    if ! confirm "Already on ${tag}. Reinstall anyway?"; then
      info "nothing to do"
      exit 0
    fi
  fi

  info "reading existing configuration from ${UNIT_PATH}..."
  read_current_env "$UNIT_PATH"

  declare -A final_env
  local missing_any="false"
  for key in "${ENV_KEYS[@]}"; do
    if [ -n "${CURRENT_ENV[$key]:-}" ]; then
      final_env["$key"]="${CURRENT_ENV[$key]}"
    else
      missing_any="true"
      warn "missing from existing config: ${key}"
      final_env["$key"]=$(prompt_with_default "$key" "${ENV_DEFAULTS[$key]}")
    fi
  done

  if [ "$missing_any" = "false" ]; then
    info "existing configuration is complete, nothing to ask"
  fi

  echo
  info "about to update minilab-dashboard (${current_tag} -> ${tag}) with:"
  for key in "${ENV_KEYS[@]}"; do
    printf '    %-26s %s\n' "$key" "${final_env[$key]}"
  done
  echo

  # Not `local` -- the EXIT trap below fires after main() returns, once its
  # local scope is gone, so tmp_binary must still be visible at that point.
  tmp_binary=$(mktemp)
  trap 'rm -f "$tmp_binary"' EXIT

  info "downloading binary..."
  curl -fsSL -o "$tmp_binary" "$asset_url"

  info "verifying checksum..."
  local expected actual
  expected=$(curl -fsSL "${asset_url}.sha256" | cut -d' ' -f1) || { error "could not download the checksum file"; exit 1; }
  actual=$(sha256sum "$tmp_binary" | cut -d' ' -f1)
  if [ -z "$expected" ] || [ "$expected" != "$actual" ]; then
    error "checksum mismatch (expected '${expected}', got '${actual}') -- refusing to install"
    exit 1
  fi
  chmod +x "$tmp_binary"

  if confirm "Stop ${SERVICE_NAME} to apply the update?"; then
    sudo systemctl stop "$SERVICE_NAME"
  else
    warn "leaving the running service in place -- the binary underneath it will still be replaced"
  fi

  if confirm "Install the new binary to ${INSTALL_PATH}?"; then
    sudo cp "$tmp_binary" "$INSTALL_PATH"
    sudo chmod +x "$INSTALL_PATH"
    echo "$tag" | sudo tee "$VERSION_MARKER" >/dev/null
    info "binary installed (${tag})"
  else
    warn "skipped installing the binary -- aborting, service left stopped if it was stopped above"
    exit 0
  fi

  if [ ! -d "$DATA_DIR" ]; then
    warn "${DATA_DIR} doesn't exist -- creating it (owned by $USER) so the service can start"
    sudo mkdir -p "$DATA_DIR"
    sudo chown "$USER" "$DATA_DIR"
  fi

  if confirm "Rewrite ${UNIT_PATH} with the configuration above?"; then
    sudo bash -c "cat > '${UNIT_PATH}'" <<EOF
[Unit]
Description=Mini Lab monitoring dashboard
After=network.target

[Service]
Type=simple
ExecStart=${INSTALL_PATH}
Restart=on-failure
User=${USER}
WorkingDirectory=${DATA_DIR}
Environment=LOG_LEVEL=${final_env[LOG_LEVEL]}
Environment=HTTP_LISTEN_ADDR=${final_env[HTTP_LISTEN_ADDR]}
Environment=SQLITE_PATH=${final_env[SQLITE_PATH]}
Environment=MDNS_AGENT_SERVICE_NAME=${final_env[MDNS_AGENT_SERVICE_NAME]}
Environment=DISCOVERY_INTERVAL=${final_env[DISCOVERY_INTERVAL]}
Environment=POLL_INTERVAL=${final_env[POLL_INTERVAL]}
Environment=UNREACHABLE_THRESHOLD=${final_env[UNREACHABLE_THRESHOLD]}
Environment=TLS_CA_FILE=${CURRENT_ENV[TLS_CA_FILE]:-}
Environment=TLS_CLIENT_CERT_FILE=${CURRENT_ENV[TLS_CLIENT_CERT_FILE]:-}
Environment=TLS_CLIENT_KEY_FILE=${CURRENT_ENV[TLS_CLIENT_KEY_FILE]:-}
Environment=TLS_CERT_FILE=${CURRENT_ENV[TLS_CERT_FILE]:-}
Environment=TLS_KEY_FILE=${CURRENT_ENV[TLS_KEY_FILE]:-}

[Install]
WantedBy=multi-user.target
EOF
    info "systemd unit updated"
  else
    warn "kept the existing unit file as-is"
  fi

  if confirm "Reload systemd and (re)start ${SERVICE_NAME} now?"; then
    sudo systemctl daemon-reload
    sudo systemctl restart "$SERVICE_NAME"
    info "${SERVICE_NAME} restarted"
  else
    warn "skipped restarting -- run 'sudo systemctl daemon-reload && sudo systemctl restart ${SERVICE_NAME}' manually when ready"
    exit 0
  fi

  echo
  info "done. check status with: systemctl status ${SERVICE_NAME}"
  info "tail logs with:          journalctl -u ${SERVICE_NAME} -f"
}

main "$@"
