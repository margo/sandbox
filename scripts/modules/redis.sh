#!/bin/bash
# modules/wfm/redis.sh - Redis installation

source "$(dirname "${BASH_SOURCE[0]}")/../lib/common.sh" 

install_redis() {
  local REDIS_VERSION="7.0.15"

  echo "🔄 Installing Redis ${REDIS_VERSION}..."

  if command_exists redis-server; then
    REDIS_VERSION="$(redis-server --version | cut -d ' ' -f 3 | cut -d '=' -f 2)"
    echo "⚡️ Redis ${REDIS_VERSION} already installed, skipping installation"
  else
    sudo apt update
    sudo apt install -y redis-server=${REDIS_VERSION}-* || sudo apt install -y redis-server

    sudo systemctl enable redis-server
    sudo systemctl start redis-server

    REDIS_VERSION="$(redis-server --version)"
    echo "✅ Redis ${REDIS_VERSION} installed"
  fi
}
