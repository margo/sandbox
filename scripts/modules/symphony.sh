#!/bin/bash
# modules/symphony.sh - Symphony API management functions

source "$(dirname "${BASH_SOURCE[0]}")/../lib/common.sh"

# Certificate paths
HARBOR_CERT_CI="$HOME/sandbox/scripts/harbor/certs/harbor.crt"
HARBOR_CERT_PRODUCTION="/data/cert/harbor.crt"
HARBOR_CERT_DEST="$HOME/symphony/api/certificates/harbor-ca.crt"

build_maestro_cli() {
  CLI_DIR="$HOME/symphony/cli"
  if [ -d "$CLI_DIR" ]; then
    cd "$CLI_DIR"
    go mod tidy
    go build -o maestro
  fi
}

# Helper function to locate and copy Harbor certificate
copy_harbor_certificate() {
  local dest="$1"
  
  # Check CI environment path first
  if [ -f "$HARBOR_CERT_CI" ]; then
    cp "$HARBOR_CERT_CI" "$dest"
    echo "✅ Harbor CA copied from CI path"
    return 0
  fi
  
  # Check production path
  if [ -f "$HARBOR_CERT_PRODUCTION" ]; then
    cp "$HARBOR_CERT_PRODUCTION" "$dest"
    echo "✅ Harbor CA copied from production path"
    return 0
  fi
  
  echo "⚠️  Harbor certificate not found in any location"
  return 1
}


create_symphony_api_systemd_service() {
  echo "🔧 Creating systemd service for Symphony API auto-start..."

  local symphony_dir="$HOME/symphony/api"
  local harbor_cert_ci="$HARBOR_CERT_CI"
  local harbor_cert_prod="$HARBOR_CERT_PRODUCTION"

  create_harbor_ca_install_script

  sudo tee /etc/systemd/system/symphony-api.service > /dev/null <<EOF
[Unit]
Description=Margo Symphony API Server
Requires=docker.service redis-server.service harbor.service
After=docker.service redis-server.service harbor.service network-online.target
Wants=network-online.target

[Service]
Type=simple
RemainAfterExit=yes
WorkingDirectory=${symphony_dir}
Environment="SYMPHONY_IMAGE_REF=ghcr.io/margo/margo-symphony-api:latest"
ExecStartPre=/bin/sleep 15
ExecStartPre=-/usr/bin/docker stop symphony-api-container
ExecStartPre=-/usr/bin/docker rm symphony-api-container
ExecStartPre=/bin/mkdir -p ${symphony_dir}/certificates
# Try CI path first, then production path for Harbor certificate
ExecStartPre=/bin/bash -c 'if [ -f "${harbor_cert_ci}" ]; then cp "${harbor_cert_ci}" ${symphony_dir}/certificates/harbor-ca.crt; elif [ -f "${harbor_cert_prod}" ]; then cp "${harbor_cert_prod}" ${symphony_dir}/certificates/harbor-ca.crt; else for i in {1..60}; do [ -f "${harbor_cert_prod}" ] && cp "${harbor_cert_prod}" ${symphony_dir}/certificates/harbor-ca.crt && exit 0; sleep 2; done; exit 1; fi'
ExecStart=/usr/bin/docker run --rm --name symphony-api-container \
    --network host \
    -p 8082:8082 \
    -e LOG_LEVEL=Debug \
    -e CONFIG=symphony-api-margo.json \
    -e NODE_EXTRA_CA_CERTS=/certificates/harbor-ca.crt \
    -v ${symphony_dir}/certificates:/certificates \
    -v ${symphony_dir}:/configs \
    \${SYMPHONY_IMAGE_REF}
ExecStartPost=${symphony_dir}/install-harbor-ca.sh
ExecStop=/usr/bin/docker stop symphony-api-container
TimeoutStartSec=0
Restart=on-failure
RestartSec=10s

[Install]
WantedBy=multi-user.target
EOF

  sudo systemctl daemon-reload
  sudo systemctl enable symphony-api.service

  echo "✅ Symphony API systemd service created and enabled"
  echo "📋 Service will start Symphony API automatically on boot"
  echo "📁 Working directory: ${symphony_dir}"
}

remove_symphony_api_systemd_service() {
  echo "Removing Symphony API systemd service..."

  local service_name="symphony-api"
  local service_file="/etc/systemd/system/${service_name}.service"

  if [[ ! -f "$service_file" ]]; then
    echo "Symphony API service does not exist"
    return 0
  fi

  if sudo systemctl is-active --quiet "${service_name}.service"; then
    echo "Stopping ${service_name}..."
    sudo systemctl stop "${service_name}.service"
  fi

  if sudo systemctl is-enabled --quiet "${service_name}.service" 2>/dev/null; then
    echo "Disabling ${service_name}..."
    sudo systemctl disable "${service_name}.service"
  fi

  echo "Removing service file..."
  sudo rm -f "$service_file"

  sudo systemctl daemon-reload
  sudo systemctl reset-failed 2>/dev/null || true

  echo "✅ Symphony API systemd service removed successfully"
}

start_symphony_api_container() {
  cd "$HOME/symphony/api" || { echo "❌ Failed to change directory"; return 1; }

  echo "Stopping and removing existing symphony-api-container if present..."
  docker stop symphony-api-container 2>/dev/null || true
  docker rm symphony-api-container 2>/dev/null || true
  pkill -f "symphony-api" 2>/dev/null || true

  # Ensure certificates directory exists
  mkdir -p "$HOME/symphony/api/certificates"

  # Copy Harbor CA certificate from appropriate location
  if ! copy_harbor_certificate "$HARBOR_CERT_DEST"; then
    echo "⚠️  Warning: Harbor certificate not available, container may have SSL issues"
  fi

  # Only check/pull from GHCR if not using a local image
  if [[ "${SYMPHONY_IMAGE_REF}" != *":ci-test" ]] && [[ "${SYMPHONY_IMAGE_REF}" == ghcr.io/* ]]; then
    echo "Checking GHCR image: ${SYMPHONY_IMAGE_REF}"
    if docker manifest inspect "${SYMPHONY_IMAGE_REF}" >/dev/null 2>&1; then
      echo "Pulling image: ${SYMPHONY_IMAGE_REF}"
      docker pull "${SYMPHONY_IMAGE_REF}"
    else
      echo "Image does NOT exist in GHCR"
      return 1
    fi
  else
    echo "Using local image: ${SYMPHONY_IMAGE_REF}"
  fi

  echo "🚀 Starting Symphony API container..."

  # Build docker run command with CI-specific options
  DOCKER_RUN_CMD="docker run -dit --name symphony-api-container \
      --network host \
      -p 8082:8082 \
      -e LOG_LEVEL=Debug \
      -v $HOME/symphony/api/certificates:/certificates \
      -v $HOME/symphony/api:/configs \
      -e CONFIG=symphony-api-margo.json"

  # Add NODE_EXTRA_CA_CERTS for CA certificate handling
  DOCKER_RUN_CMD="$DOCKER_RUN_CMD \
    -e NODE_EXTRA_CA_CERTS=/certificates/harbor-ca.crt"

  # Add CI-specific environment and host mappings
  if [[ "${CI}" == "true" && -n "${RUNNER_IP}" ]]; then
    DOCKER_RUN_CMD="$DOCKER_RUN_CMD \
      --add-host harbor.machine:${RUNNER_IP} \
      --add-host symphony.machine:${RUNNER_IP}"
  fi

  # Execute docker run
  eval "$DOCKER_RUN_CMD ${SYMPHONY_IMAGE_REF}"

  # Wait for container to start
  sleep 5

  if docker ps --format '{{.Names}}' | grep -q symphony-api-container; then
    echo "📋 Installing Harbor CA in container..."
    docker exec symphony-api-container sh -c '
      if [ -f /certificates/harbor-ca.crt ]; then
        mkdir -p /usr/local/share/ca-certificates
        cp /certificates/harbor-ca.crt /usr/local/share/ca-certificates/harbor.crt
        update-ca-certificates
        echo "✅ Harbor CA installed in Symphony container"
      else
        echo "⚠️  Harbor CA certificate not found in /certificates"
      fi
    ' || echo "⚠️  Warning: Failed to install CA certificate in container"

    echo "🔄 Restarting Symphony container to reload CA trust..."
    sleep 2
    docker restart symphony-api-container

    sleep 10
  else
    echo "❌ Container failed to start"
    docker logs symphony-api-container || true
    return 1
  fi

  if docker ps --format '{{.Names}}' | grep -q symphony-api-container; then
      echo "✅ Symphony API container started successfully"
      echo "📡 Container is running on port 8082 (host network)"
      create_symphony_api_systemd_service
  else
      echo "❌ Failed to start Symphony API container"
      docker logs symphony-api-container || true
      return 1
  fi
}


create_harbor_ca_install_script() {
  cat > "$HOME/symphony/api/install-harbor-ca.sh" <<'EOF'
#!/bin/bash

echo "Installing Harbor CA into Symphony container..."

for i in $(seq 1 30); do
    docker exec symphony-api-container test -f /certificates/harbor-ca.crt >/dev/null 2>&1 && break
    sleep 2
done

docker exec symphony-api-container sh -c '
mkdir -p /usr/local/share/ca-certificates &&
cp /certificates/harbor-ca.crt /usr/local/share/ca-certificates/harbor.crt &&
update-ca-certificates
'

echo "Harbor CA installation completed"
EOF

  chmod +x "$HOME/symphony/api/install-harbor-ca.sh"
}