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


start_symphony_api_container() {
  cd "$HOME/symphony/api"

  echo "Stopping and removing existing symphony-api-container if present..."
  COMPOSE_CMD="docker compose -f docker-compose.yaml"

  if [[ "${CI}" == "true" && -n "${RUNNER_IP}" ]]; then
    COMPOSE_CMD="$COMPOSE_CMD -f docker-compose.ci.yaml"
  fi

  $COMPOSE_CMD down


  # Copy Harbor CA if in CI environment
    HARBOR_CERT="$HOME/sandbox/scripts/harbor/certs/harbor.crt"
    if [ -f "$HARBOR_CERT" ]; then
      cp "$HARBOR_CERT" "$HOME/symphony/api/certificates/harbor-ca.crt"
      echo "✅ Harbor CA copied to Symphony certificates"
    fi


  # $HOME/symphony/api/mis`). That directory must exist and be bind-mounted into the
  # container as /mis, or the mTLS/SBI HttpBinding's mis config fails to load
  # (./mis/... resolves against the container root, since neither the image nor this
  # docker run set a working directory).
  mkdir -p "$HOME/symphony/api/mis"

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
  
  $COMPOSE_CMD up -d

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
      echo "📡 Container is running on port 8082 (host network), for NBI (Easy-CLI)"
      echo "📡 Container is running on port 8084 (host network), for Margo Management Interface"
  else
      echo "❌ Failed to start Symphony API container"
      docker logs symphony-api-container || true
      return 1
  fi
}
