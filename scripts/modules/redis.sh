#!/bin/bash
# modules/wfm/redis.sh - Redis installation

source "$(dirname "${BASH_SOURCE[0]}")/../lib/common.sh"

COMPOSE_FILE="$(dirname "${BASH_SOURCE[0]}")/../compose-files/redis-compose.yaml"

install_redis() {
  local REDIS_VERSION="7.0.15"
  local REDIS_CONTAINER_NAME="redis"

  echo "🔄 Installing Redis ${REDIS_VERSION}..."

  if ! command_exists docker; then
    echo "❌ Docker is not installed. Please install Docker first."
    return 1
  fi

  if ! docker compose version &>/dev/null; then
    echo "❌ Docker Compose plugin is not installed. Please install it first."
    return 1
  fi

  # Disable system Redis service if running
  if systemctl is-active --quiet redis-server 2>/dev/null || systemctl is-active --quiet redis 2>/dev/null; then
    echo "⚠️  System Redis service detected, disabling it..."
    sudo systemctl stop redis-server 2>/dev/null || sudo systemctl stop redis 2>/dev/null
    sudo systemctl disable redis-server 2>/dev/null || sudo systemctl disable redis 2>/dev/null
    echo "✅ System Redis service disabled"
  fi

  if docker ps --format '{{.Names}}' | grep -q "^${REDIS_CONTAINER_NAME}$"; then
    REDIS_VERSION="$(docker exec ${REDIS_CONTAINER_NAME} redis-server --version | cut -d ' ' -f 3 | cut -d '=' -f 2)"
    echo "⚡️ Redis ${REDIS_VERSION} already running as container, skipping installation"
  else
    docker compose -f "${COMPOSE_FILE}" up -d

    REDIS_VERSION="$(docker exec ${REDIS_CONTAINER_NAME} redis-server --version | cut -d ' ' -f 3 | cut -d '=' -f 2)"
    echo "✅ Redis ${REDIS_VERSION} installed and running via Docker Compose"
  fi
}
