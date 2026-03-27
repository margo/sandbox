#!/bin/bash
# modules/wfm/oras.sh - ORAS CLI installation

source "$(dirname "${BASH_SOURCE[0]}")/../lib/common.sh" 

install_oras() {
  echo "🔄 Installing ORAS CLI..."

  if command_exists oras; then
    ORAS_VERSION="$(oras version | head -n 1 | cut -d ':' -f 2 | sed 's/[[:space:]]*//')"
    echo "⚡️ ORAS ${ORAS_VERSION} already installed, skipping installation"
    return 0
  fi

  cd /tmp
  local ORAS_VERSION="1.1.0"
  wget "https://github.com/oras-project/oras/releases/download/v${ORAS_VERSION}/oras_${ORAS_VERSION}_linux_amd64.tar.gz"
  tar -xzf "oras_${ORAS_VERSION}_linux_amd64.tar.gz"
  sudo mv oras /usr/local/bin/
  rm "oras_${ORAS_VERSION}_linux_amd64.tar.gz"

  ORAS_VERSION="$(oras version | head -n 1 | cut -d ':' -f 2 | sed 's/[[:space:]]*//')"
  echo "✅ ORAS ${ORAS_VERSION} installed"
}
