#!/bin/bash
# modules/symphony.sh - Symphony API management functions (cleaned + production-safe)

source "$(dirname "${BASH_SOURCE[0]}")/../lib/common.sh"

SYMPHONY_DIR="$HOME/symphony/api"
CERT_DIR="$SYMPHONY_DIR/certificates"
HARBOR_CERT_SRC="$HOME/sandbox/scripts/harbor/certs/harbor.crt"
HARBOR_CERT_DEST="$CERT_DIR/harbor-ca.crt"

# ------------------------------------------------------------------------------
# Build Maestro CLI
# ------------------------------------------------------------------------------
build_maestro_cli() {
  local cli_dir="$HOME/symphony/cli"
  if [ -d "$cli_dir" ]; then
    cd "$cli_dir" || return
    go mod tidy
    go build -o maestro
    echo "✅ Maestro CLI built"
  fi
}

# ------------------------------------------------------------------------------
# Ensure Harbor certificate exists for Symphony
# ------------------------------------------------------------------------------
ensure_harbor_cert() {
  mkdir -p "$CERT_DIR"

  if [ -f "$HARBOR_CERT_SRC" ]; then
    cp -f "$HARBOR_CERT_SRC" "$HARBOR_CERT_DEST"
    echo "✅ Harbor CA prepared for Symphony"
  else
    echo "⚠️ Harbor CA not found at $HARBOR_CERT_SRC"
  fi
}

# ------------------------------------------------------------------------------
# Create systemd service (FIXED VERSION)
# ------------------------------------------------------------------------------
create_symphony_api_systemd_service() {
  echo "🔧 Creating systemd service for Symphony API..."

  ensure_harbor_cert

  sudo tee /etc/systemd/system/symphony-api.service > /dev/null <<EOF
[Unit]
Description=Margo Symphony API Server
Requires=docker.service redis-server.service
After=docker.service redis-server.service network-online.target harbor.service
Wants=network-online.target

[Service]
Type=simple
RemainAfterExit=yes
WorkingDirectory=${SYMPHONY_DIR}

ExecStartPre=/bin/sleep 15
ExecStartPre=-/usr/bin/docker stop symphony-api-container
ExecStartPre=-/usr/bin/docker rm symphony-api-container

# Ensure Harbor certificate is present before container start
ExecStartPre=/bin/bash -c 'cp -f ${HARBOR_CERT_SRC} ${HARBOR_CERT_DEST} || true'

ExecStart=/usr/bin/docker run --rm --name symphony-api-container \
    --network host \
    -p 8082:8082 \
    -v ${CERT_DIR}:/certificates \
    -v ${SYMPHONY_DIR}:/configs \
    -e LOG_LEVEL=Debug \
    -e CONFIG=symphony-api-margo.json \
    -e NODE_EXTRA_CA_CERTS=/certificates/harbor-ca.crt \
    -v /etc/ssl/certs:/etc/ssl/certs:ro \
    -v /usr/local/share/ca-certificates:/usr/local/share/ca-certificates:ro \
    ${SYMPHONY_IMAGE_REF} \
    sh -c '
      echo "🔐 Setting up Harbor CA trust...";
      if [ -f /certificates/harbor-ca.crt ]; then
        mkdir -p /usr/local/share/ca-certificates;
        cp /certificates/harbor-ca.crt /usr/local/share/ca-certificates/harbor.crt;
        update-ca-certificates;
        echo "✅ Harbor CA installed";
      else
        echo "⚠️ Harbor CA not found";
      fi;

      echo "🚀 Starting Symphony API...";
      exec "\$0" "\$@"
    '

ExecStop=/usr/bin/docker stop symphony-api-container

TimeoutStartSec=0
Restart=on-failure
RestartSec=10s

[Install]
WantedBy=multi-user.target
EOF

  sudo systemctl daemon-reload
  sudo systemctl enable symphony-api.service

  echo "✅ Symphony API systemd service created"
}

# ------------------------------------------------------------------------------
# Remove systemd service
# ------------------------------------------------------------------------------
remove_symphony_api_systemd_service() {
  echo "🧹 Removing Symphony API systemd service..."

  local service="symphony-api"

  sudo systemctl stop "${service}" 2>/dev/null || true
  sudo systemctl disable "${service}" 2>/dev/null || true
  sudo rm -f "/etc/systemd/system/${service}.service"

  sudo systemctl daemon-reload
  sudo systemctl reset-failed 2>/dev/null || true

  echo "✅ Removed successfully"
}

# ------------------------------------------------------------------------------
# Manual start (for dev/debug only)
# ------------------------------------------------------------------------------
start_symphony_api_container() {
  cd "$SYMPHONY_DIR" || return

  echo "🧹 Cleaning existing container..."
  docker stop symphony-api-container 2>/dev/null || true
  docker rm symphony-api-container 2>/dev/null || true

  ensure_harbor_cert

  echo "🚀 Starting Symphony API container..."

  docker run -dit --name symphony-api-container \
      --network host \
      -p 8082:8082 \
      -e LOG_LEVEL=Debug \
      -e CONFIG=symphony-api-margo.json \
      -e NODE_EXTRA_CA_CERTS=/certificates/harbor-ca.crt \
      -v "$CERT_DIR:/certificates" \
      -v "$SYMPHONY_DIR:/configs" \
      "${SYMPHONY_IMAGE_REF}"

  sleep 5

  if docker ps --format '{{.Names}}' | grep -q symphony-api-container; then
    echo "✅ Symphony API running"
  else
    echo "❌ Failed to start"
    docker logs symphony-api-container || true
    return 1
  fi
}
