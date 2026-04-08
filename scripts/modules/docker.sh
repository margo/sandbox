#!/bin/bash
# modules/docker.sh - Docker and Docker Compose installation

source "$(dirname "${BASH_SOURCE[0]}")/../lib/common.sh"  # Changed from ../../ to ../

install_docker_and_compose() {
  cd $HOME

  local DOCKER_VERSION="29.1.2"
  local DOCKER_COMPOSE_VERSION="5.0.0"
  local UBUNTU_CODENAME=$(get_ubuntu_codename)

  echo "🔄 Installing Docker..."
  if command_exists docker; then
    CURRENT_DOCKER_VERSION=$(docker version --format '{{.Server.Version}}')
    echo "⚡️ Docker ${CURRENT_DOCKER_VERSION} already installed, skipping installation"

    if [ "$CURRENT_DOCKER_VERSION" != "$DOCKER_VERSION" ]; then
      echo "⚠️  Current Docker version: $CURRENT_DOCKER_VERSION (expected: $DOCKER_VERSION)"
    fi
  else
    echo "Docker not found. Installing Docker ${DOCKER_VERSION}..."

    sudo apt-get remove -y docker docker-engine docker.io containerd runc 2>/dev/null || true
    sudo apt-get update
    sudo apt-get install -y ca-certificates curl
    sudo install -m 0755 -d /etc/apt/keyrings
    sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
    sudo chmod a+r /etc/apt/keyrings/docker.asc

    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
      ${UBUNTU_CODENAME} stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

    sudo apt-get update
    sudo apt-get install -y \
      docker-ce=5:${DOCKER_VERSION}-1~ubuntu.24.04~${UBUNTU_CODENAME} \
      docker-ce-cli=5:${DOCKER_VERSION}-1~ubuntu.24.04~${UBUNTU_CODENAME} \
      containerd.io=1.7.27-1 \
      docker-buildx-plugin=0.23.0-1~ubuntu.24.04~${UBUNTU_CODENAME}

    sudo usermod -aG docker $USER
    echo "✅ Docker ${DOCKER_VERSION} installed"

    if ! groups | grep -w "docker" &>/dev/null ; then
        sudo usermod -aG docker $USER
        echo "🚨 User added to group 'docker'! You need to restart the installation from a new bash session."
        exit
    fi
  fi

  echo "🔄 Installing Docker Compose plugin ${DOCKER_COMPOSE_VERSION}..."
  if package_installed docker-compose-plugin; then
    CURRENT_COMPOSE_VERSION=$(docker compose version --short 2>/dev/null | sed 's/v//')
    echo "⚡️ Docker Compose ${CURRENT_COMPOSE_VERSION} already installed"
  else
    sudo apt-get update
    sudo apt-get install -y docker-compose-plugin=${DOCKER_COMPOSE_VERSION}-1~ubuntu.24.04~${UBUNTU_CODENAME}
  fi

  rm -f /usr/local/bin/docker-compose /usr/bin/docker-compose 2>/dev/null || true

  for i in $(seq 1 30); do
    if systemctl is-active --quiet docker; then
      echo '✅ Docker daemon is running.'
      break
    else
      sleep 1
    fi
  done

  sudo apt-mark hold docker-ce docker-ce-cli docker-compose-plugin containerd.io docker-buildx-plugin
  echo "✅ Docker ${DOCKER_VERSION} and Docker Compose v${DOCKER_COMPOSE_VERSION} ready"
}
