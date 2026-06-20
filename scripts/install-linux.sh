#!/usr/bin/env sh
set -eu

REPO="${DDM_AGENT_REPO:-NUS-MTech-Capstone/ddm-agent}"
VERSION="${DDM_AGENT_VERSION:-latest}"
SERVICE_NAME="${DDM_AGENT_SERVICE_NAME:-ddm-agent}"
INSTALL_DIR="${DDM_AGENT_INSTALL_DIR:-/usr/local/bin}"
CONFIG_DIR="${DDM_AGENT_CONFIG_DIR:-/etc/ddm-agent}"
PORT="${DDM_AGENT_PORT:-8084}"
STS_URL="${DDM_AGENT_STS_URL:-http://localhost:8082}"
HOST_ID="${DDM_AGENT_HOST_ID:-}"

if [ "$(uname -s)" != "Linux" ]; then
  echo "This installer only supports Linux." >&2
  exit 1
fi

if command -v curl >/dev/null 2>&1; then
  DOWNLOADER="curl"
elif command -v wget >/dev/null 2>&1; then
  DOWNLOADER="wget"
else
  echo "curl or wget is required." >&2
  exit 1
fi

if ! command -v systemctl >/dev/null 2>&1; then
  echo "systemd/systemctl is required." >&2
  exit 1
fi

if [ "$(id -u)" -eq 0 ]; then
  SUDO=""
else
  if ! command -v sudo >/dev/null 2>&1; then
    echo "Run as root or install sudo." >&2
    exit 1
  fi
  SUDO="sudo"
fi

case "$(uname -m)" in
  x86_64 | amd64)
    ARCH="amd64"
    ;;
  aarch64 | arm64)
    ARCH="arm64"
    ;;
  *)
    echo "Unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

if [ "$VERSION" = "latest" ]; then
  RELEASE_BASE="https://github.com/${REPO}/releases/latest/download"
else
  RELEASE_BASE="https://github.com/${REPO}/releases/download/${VERSION}"
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

download_file() {
  url="$1"
  out="$2"

  if [ "$DOWNLOADER" = "curl" ]; then
    curl -fsSL "$url" -o "$out"
  else
    wget -q -O "$out" "$url"
  fi
}

download_asset() {
  for name in \
    "ddm-agent-linux-${ARCH}.tar.gz" \
    "ddm-agent-linux-${ARCH}.tgz" \
    "ddm-agent_linux_${ARCH}.tar.gz" \
    "ddm-agent_linux_${ARCH}.tgz" \
    "ddm-agent-linux-${ARCH}.gz" \
    "ddm-agent_linux_${ARCH}.gz" \
    "ddm-agent-linux-${ARCH}" \
    "ddm-agent_linux_${ARCH}" \
    "ddm-agent"; do
    url="${RELEASE_BASE}/${name}"
    out="${TMP_DIR}/${name}"
    if download_file "$url" "$out"; then
      echo "$out"
      return 0
    fi
  done

  echo "Could not download a Linux ${ARCH} release asset from ${RELEASE_BASE}." >&2
  echo "Expected one of: ddm-agent-linux-${ARCH}.tar.gz, ddm-agent_linux_${ARCH}.tar.gz, or a raw ddm-agent binary." >&2
  exit 1
}

ASSET_PATH="$(download_asset)"
ASSET_NAME="$(basename "$ASSET_PATH")"
BINARY_PATH="${TMP_DIR}/ddm-agent"

if download_file "${RELEASE_BASE}/SHA256SUMS" "${TMP_DIR}/SHA256SUMS"; then
  if ! command -v sha256sum >/dev/null 2>&1; then
    echo "SHA256SUMS is available, but sha256sum is not installed." >&2
    exit 1
  fi
  if ! grep "  ${ASSET_NAME}$" "${TMP_DIR}/SHA256SUMS" > "${TMP_DIR}/SHA256SUMS.${ASSET_NAME}"; then
    echo "SHA256SUMS does not contain ${ASSET_NAME}." >&2
    exit 1
  fi
  (cd "$TMP_DIR" && sha256sum -c "SHA256SUMS.${ASSET_NAME}")
else
  echo "SHA256SUMS was not found for this release; continuing without checksum verification." >&2
fi

case "$ASSET_PATH" in
  *.tar.gz | *.tgz)
    tar -xzf "$ASSET_PATH" -C "$TMP_DIR"
    if [ -f "${TMP_DIR}/ddm-agent" ]; then
      BINARY_PATH="${TMP_DIR}/ddm-agent"
    else
      found="$(find "$TMP_DIR" -type f -name ddm-agent | head -n 1)"
      if [ -z "$found" ]; then
        echo "Archive did not contain a ddm-agent binary." >&2
        exit 1
      fi
      BINARY_PATH="$found"
    fi
    ;;
  *.gz)
    gzip -dc "$ASSET_PATH" > "$BINARY_PATH"
    ;;
  *)
    BINARY_PATH="$ASSET_PATH"
    ;;
esac

chmod +x "$BINARY_PATH"

if ! id "$SERVICE_NAME" >/dev/null 2>&1; then
  $SUDO useradd --system --no-create-home --shell /usr/sbin/nologin "$SERVICE_NAME"
fi

$SUDO install -d -m 0755 "$INSTALL_DIR"
$SUDO install -m 0755 "$BINARY_PATH" "${INSTALL_DIR}/ddm-agent"

$SUDO install -d -m 0755 "$CONFIG_DIR"
if [ ! -f "${CONFIG_DIR}/ddm-agent.env" ]; then
  tmp_env="${TMP_DIR}/ddm-agent.env"
  cat > "$tmp_env" <<EOF
PORT=${PORT}
STS_URL=${STS_URL}
DDM_AGENT_HOST_ID=${HOST_ID}
EOF
  $SUDO install -m 0644 "$tmp_env" "${CONFIG_DIR}/ddm-agent.env"
fi

tmp_unit="${TMP_DIR}/ddm-agent.service"
cat > "$tmp_unit" <<EOF
[Unit]
Description=DenDenMushi Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_NAME}
Group=${SERVICE_NAME}
EnvironmentFile=-${CONFIG_DIR}/ddm-agent.env
ExecStart=${INSTALL_DIR}/ddm-agent
Restart=on-failure
RestartSec=5s
NoNewPrivileges=true
CapabilityBoundingSet=
AmbientCapabilities=
PrivateTmp=true
PrivateDevices=true
ProtectSystem=full
ProtectHome=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictSUIDSGID=true

[Install]
WantedBy=multi-user.target
EOF

$SUDO install -m 0644 "$tmp_unit" "/etc/systemd/system/ddm-agent.service"
$SUDO systemctl daemon-reload
$SUDO systemctl enable --now ddm-agent.service

echo "Installed ddm-agent to ${INSTALL_DIR}/ddm-agent"
echo "Config file: ${CONFIG_DIR}/ddm-agent.env"
echo "Service status: systemctl status ddm-agent.service"
