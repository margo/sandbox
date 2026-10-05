#!/bin/bash
# modules/helm.sh - Helm installation

source "$(dirname "${BASH_SOURCE[0]}")/../lib/common.sh" 

install_helm() {
  cd $HOME
  local HELM_VERSION="3.15.1"
  local ARCH
  ARCH="$(detect_arch)" || exit 1
  local HELM_TAR="helm-v${HELM_VERSION}-linux-${ARCH}.tar.gz"
  local HELM_BIN_DIR="/usr/local/bin"

  echo "🔄 Installing Helm..."
  if command_exists helm && [[ "$(helm version --short | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')" == "${HELM_VERSION}" ]]; then
      echo "⚡️ Helm version ${HELM_VERSION} already installed, skipping installation"
  else
      echo "Downloading Helm version ${HELM_VERSION}..."
      wget -q "https://get.helm.sh/${HELM_TAR}" || { echo "Failed to download Helm."; exit 1; }
      tar -xzf "${HELM_TAR}" || { echo "Failed to extract Helm."; exit 1; }
      sudo mv "linux-${ARCH}/helm" "${HELM_BIN_DIR}/" || { echo "Failed to move Helm."; exit 1; }
      rm "${HELM_TAR}"
      rm -rf "linux-${ARCH}/"
      echo '✅ Helm installed'
  fi
}
