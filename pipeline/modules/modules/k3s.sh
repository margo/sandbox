#!/bin/bash
# modules/k3s.sh - K3s Kubernetes installation and configuration

source "$(dirname "${BASH_SOURCE[0]}")/../../lib/common.sh"

check_k3s_installed() {
  if command -v k3s >/dev/null 2>&1; then
    echo 'k3s already installed.'

    # Check current version
    CURRENT_K3S_VERSION=$(k3s --version | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+\+k3s[0-9]+' | head -1)
    echo "Current k3s version: $CURRENT_K3S_VERSION"

    if [ "$CURRENT_K3S_VERSION" != "$K3S_VERSION" ]; then
      echo "⚠️  Expected k3s version: $K3S_VERSION"
      echo "ℹ️  To upgrade/downgrade, uninstall current k3s and run installation again"
    fi

    return 0
  else
    return 1
  fi
}

install_k3s_dependencies() {
  echo 'Installing k3s dependencies...'
  sudo apt update
  sudo apt install -y curl
}

install_k3s() {
  echo "🔄 Installing k3s ${K3S_VERSION}..."
  if check_k3s_installed; then
    echo "⚡️ k3s ${K3S_VERSION} already installed, skipping installation"
  else
    install_k3s_dependencies

    # Install specific k3s version
    curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION="${K3S_VERSION}" sh -

    echo "✅ k3s ${K3S_VERSION} installed"
  fi
}

verify_k3s_status() {
  echo 'Verifying k3s status...'
  sudo systemctl status k3s --no-pager || true
  sudo k3s kubectl get nodes || true
}

setup_kubeconfig() {
  echo 'Setting up kubeconfig...'
  mkdir -p "$HOME/.kube"
  sudo cp /etc/rancher/k3s/k3s.yaml "$HOME/.kube/config"
  sudo chown $(id -u):$(id -g) "$HOME/.kube/config"
  export KUBECONFIG="$HOME/.kube/config"
  echo 'Kubeconfig setup complete.'
  kubectl get nodes || true
}

setup_k3s() {
  install_k3s
  verify_k3s_status
  setup_kubeconfig

  echo "✅ k3s ${K3S_VERSION} setup complete"
}
