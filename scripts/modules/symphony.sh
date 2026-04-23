#!/bin/bash
# modules/symphony.sh - Symphony API management functions

source "$(dirname "${BASH_SOURCE[0]}")/../lib/common.sh"

build_maestro_cli() {
  CLI_DIR="$HOME/symphony/cli"
  if [ -d "$CLI_DIR" ]; then
    cd "$CLI_DIR"
    go mod tidy
    go build -o maestro
  fi
}

create_symphony_api_systemd_service() {
  echo "🔧 Creating systemd service for Symphony API auto-start..."

  local symphony_dir="$HOME/symphony/api"

  sudo tee /etc/systemd/system/symphony-api.service > /dev/null <<EOF
[Unit]
Description=Margo Symphony API Server
Requires=docker.service redis-server.service
After=docker.service redis-server.service network-online.target
Wants=network-online.target

[Service]
Type=simple
RemainAfterExit=yes
WorkingDirectory=${symphony_dir}
ExecStartPre=/bin/sleep 15
ExecStartPre=-/usr/bin/docker stop symphony-api-container
ExecStartPre=-/usr/bin/docker rm symphony-api-container
ExecStart=/usr/bin/docker run --rm --name symphony-api-container \
    --network host \
    -p 8082:8082 \
    -e LOG_LEVEL=Debug \
    -v ${symphony_dir}/certificates:/certificates \
    -v ${symphony_dir}:/configs \
    -e CONFIG=symphony-api-margo.json \
    ${SYMPHONY_IMAGE_REF}
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
  cd "$HOME/symphony/api"

  echo "Stopping and removing existing symphony-api-container if present..."
  docker stop symphony-api-container 2>/dev/null || true
  docker rm symphony-api-container 2>/dev/null || true
  pkill -f "symphony-api" 2>/dev/null || true

  # Copy Harbor CA if in CI environment
    HARBOR_CERT="$HOME/sandbox/scripts/harbor/certs/harbor.crt"
    if [ -f "$HARBOR_CERT" ]; then
      cp "$HARBOR_CERT" "$HOME/symphony/api/certificates/harbor-ca.crt"
      echo "✅ Harbor CA copied to Symphony certificates"
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

  # Add CI-specific environment and host mappings
  if [[ "${CI}" == "true" && -n "${RUNNER_IP}" ]]; then
    DOCKER_RUN_CMD="$DOCKER_RUN_CMD \
      --add-host harbor.machine:${RUNNER_IP} \
      --add-host symphony.machine:${RUNNER_IP} \
      -e NODE_EXTRA_CA_CERTS=/certificates/harbor-ca.crt"
  fi

  # Execute docker run
  eval "$DOCKER_RUN_CMD ${SYMPHONY_IMAGE_REF}"

  # Wait for container to start
  sleep 5

  if docker ps --format '{{.Names}}' | grep -q symphony-api-container; then
    docker exec symphony-api-container sh -c '
      if [ -f /certificates/harbor-ca.crt ]; then
        mkdir -p /usr/local/share/ca-certificates
        cp /certificates/harbor-ca.crt /usr/local/share/ca-certificates/
        update-ca-certificates 2>/dev/null || true
        echo "✅ Harbor CA trusted in Symphony container"
      fi
    ' || true
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

