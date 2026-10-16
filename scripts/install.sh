#!/usr/bin/env bash
#
# Interactive installer for minilab-dashboard.
#
# Downloads the latest GitHub release binary for linux/arm64, then walks
# through setting it up as a systemd service. Safe to re-run to update an
# existing install.
#
# Usage: run this directly on the target device (rocket):
#   ./scripts/install.sh
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

LOG_LEVEL="INFO"
DEFAULT_HTTP_LISTEN_ADDR=":8080"
DEFAULT_SQLITE_PATH="./minilab-dashboard.db"
DEFAULT_MDNS_AGENT_SERVICE_NAME="_minilab-agent._tcp"
DEFAULT_DISCOVERY_INTERVAL="1m"
DEFAULT_POLL_INTERVAL="15s"
DEFAULT_UNREACHABLE_THRESHOLD="3"

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

main() {
  require_cmd curl
  require_cmd sudo
  require_cmd systemctl

  local goarch
  goarch=$(detect_arch)
  info "detected architecture: ${goarch}"

  info "looking up the latest minilab-dashboard release..."
  local release_json tag asset_url
  release_json=$(fetch_latest_release_json)
  tag=$(release_tag "$release_json")
  asset_url=$(release_asset_url "$release_json" "$goarch")
  info "latest release: ${tag}"

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

  echo
  info "configuration (press enter to accept the default)"
  local http_listen_addr sqlite_path mdns_agent_service_name tls_ca tls_cert tls_key
  local discovery_interval poll_interval unreachable_threshold
  http_listen_addr=$(prompt_with_default "HTTP listen addr" "$DEFAULT_HTTP_LISTEN_ADDR")
  sqlite_path=$(prompt_with_default "SQLite path (relative to ${DATA_DIR})" "$DEFAULT_SQLITE_PATH")
  mdns_agent_service_name=$(prompt_with_default "mDNS agent service name" "$DEFAULT_MDNS_AGENT_SERVICE_NAME")
  discovery_interval=$(prompt_with_default "Discovery interval" "$DEFAULT_DISCOVERY_INTERVAL")
  poll_interval=$(prompt_with_default "Poll interval" "$DEFAULT_POLL_INTERVAL")
  unreachable_threshold=$(prompt_with_default "Unreachable threshold (consecutive failed polls)" "$DEFAULT_UNREACHABLE_THRESHOLD")
  info "mutual TLS (leave all three empty to run plain HTTP)"
  tls_ca=$(prompt_with_default "TLS CA certificate file" "")
  tls_cert=$(prompt_with_default "TLS certificate file (this dashboard)" "")
  tls_key=$(prompt_with_default "TLS private key file (this dashboard)" "")

  echo
  info "about to install minilab-dashboard as a systemd service (${SERVICE_NAME}) with:"
  echo "    http addr:    ${http_listen_addr}"
  echo "    sqlite path:  ${sqlite_path}"
  echo "    data dir:     ${DATA_DIR}"
  echo "    binary path:  ${INSTALL_PATH}"
  echo

  local service_was_active="false"
  if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
    service_was_active="true"
    if confirm "${SERVICE_NAME} is currently running. Stop it before updating?"; then
      sudo systemctl stop "$SERVICE_NAME"
    else
      warn "leaving the running service in place -- the binary underneath it will still be replaced"
    fi
  fi

  if confirm "Install the binary to ${INSTALL_PATH}?"; then
    sudo cp "$tmp_binary" "$INSTALL_PATH"
    sudo chmod +x "$INSTALL_PATH"
    echo "$tag" | sudo tee "$VERSION_MARKER" >/dev/null
    info "binary installed (${tag})"
  else
    warn "skipped installing the binary -- aborting, nothing else to do"
    exit 0
  fi

  if confirm "Create the data directory at ${DATA_DIR} (owned by $USER, holds the SQLite database)?"; then
    sudo mkdir -p "$DATA_DIR"
    sudo chown "$USER" "$DATA_DIR"
    info "data directory ready at ${DATA_DIR}"
  else
    warn "skipped creating ${DATA_DIR} -- the service will fail to start unless it already exists and is writable by $USER"
  fi

  if confirm "Write the systemd unit at ${UNIT_PATH}?"; then
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
Environment=LOG_LEVEL=${LOG_LEVEL}
Environment=HTTP_LISTEN_ADDR=${http_listen_addr}
Environment=SQLITE_PATH=${sqlite_path}
Environment=MDNS_AGENT_SERVICE_NAME=${mdns_agent_service_name}
Environment=DISCOVERY_INTERVAL=${discovery_interval}
Environment=POLL_INTERVAL=${poll_interval}
Environment=UNREACHABLE_THRESHOLD=${unreachable_threshold}
Environment=TLS_CA_FILE=${tls_ca}
Environment=TLS_CERT_FILE=${tls_cert}
Environment=TLS_KEY_FILE=${tls_key}

[Install]
WantedBy=multi-user.target
EOF
    info "systemd unit written"
  else
    warn "skipped writing the systemd unit -- aborting, nothing more to configure"
    exit 0
  fi

  if confirm "Reload systemd, enable, and start ${SERVICE_NAME} now?"; then
    sudo systemctl daemon-reload
    sudo systemctl enable "$SERVICE_NAME"
    sudo systemctl start "$SERVICE_NAME"
    info "${SERVICE_NAME} started"
  else
    warn "skipped starting the service -- run 'sudo systemctl daemon-reload && sudo systemctl enable --now ${SERVICE_NAME}' manually when ready"
    exit 0
  fi

  echo
  info "done. check status with: systemctl status ${SERVICE_NAME}"
  info "tail logs with:          journalctl -u ${SERVICE_NAME} -f"
}

main "$@"
