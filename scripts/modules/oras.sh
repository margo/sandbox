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
  local ORAS_ARCH
  ORAS_ARCH="$(resolve_target_arch)" || return 1
  local ORAS_TARBALL="oras_${ORAS_VERSION}_linux_${ORAS_ARCH}.tar.gz"

  wget "https://github.com/oras-project/oras/releases/download/v${ORAS_VERSION}/${ORAS_TARBALL}"
  tar -xzf "${ORAS_TARBALL}"
  sudo mv oras /usr/local/bin/
  rm "${ORAS_TARBALL}"

  ORAS_VERSION="$(oras version | head -n 1 | cut -d ':' -f 2 | sed 's/[[:space:]]*//')"
  echo "✅ ORAS ${ORAS_VERSION} installed"
}
