#!/bin/bash
# modules/go.sh - Go installation

source "$(dirname "${BASH_SOURCE[0]}")/../lib/common.sh"



install_go() {
  cd $HOME
  echo "🔄 Installing Go..."
  if [[ "$PATH" != *"/usr/local/go/bin"* ]] ; then
    export PATH=$PATH:/usr/local/go/bin
  fi

  if command_exists go; then
    GO_VERSION="$(go version | cut -d ' ' -f 3 | cut -c 3-)"
    echo "⚡️ Go ${GO_VERSION} already installed, skipping installation"
  else
    sudo rm -rf /usr/local/go /usr/bin/go
    CPU_ARCH=$(uname -m)
    if [[ "$CPU_ARCH" == "aarch64" ]]; then
      wget "https://go.dev/dl/go1.25.10.linux-arm64.tar.gz" -O go.tar.gz
    elif [[ "$CPU_ARCH" == "amd64" ]]; then
      wget "https://go.dev/dl/go1.25.10.linux-amd64.tar.gz" -O go.tar.gz
    else
      echo "Architecture $CPU_ARCH currently unsupported. Please install Go manually."
      exit 1
    fi
    sudo tar -C /usr/local -xzf go.tar.gz
    rm go.tar.gz
    which go
    go version
    echo "✅ Go installed"
  fi
}
