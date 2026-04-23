#!/bin/bash
set -e

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load environment
load_wfm_env() {
  local env_file="$SCRIPT_DIR/wfm.env"
  if [[ ! -f "$env_file" ]]; then
    echo "[WARN] wfm.env not found at: $env_file"
    return 1
  fi
  echo "[INFO] Loading environment from: $env_file"
  set -a
  source "$env_file"
  set +a
}

load_wfm_env || true

# Environment defaults
GITHUB_USER="${GITHUB_USER:-}"
GITHUB_TOKEN="${GITHUB_TOKEN:-}"
SYMPHONY_BRANCH="${SYMPHONY_BRANCH:-main}"
SANDBOX_REPO_BRANCH="${SANDBOX_REPO_BRANCH:-main}"
EXPOSED_HARBOR_HOST="${EXPOSED_HARBOR_HOST:-127.0.0.1}"
EXPOSED_HARBOR_PORT="${EXPOSED_HARBOR_PORT:-8443}"
EXPOSED_SYMPHONY_HOST="${EXPOSED_SYMPHONY_HOST:-127.0.0.1}"
EXPOSED_SYMPHONY_PORT="${EXPOSED_SYMPHONY_PORT:-8082}"
REGISTRY_URL="${REGISTRY_URL:-https://${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}}"
REGISTRY_USER="${REGISTRY_USER:-admin}"
REGISTRY_PASS="${REGISTRY_PASS:-Harbor12345}"
OCI_ORGANIZATION="${OCI_ORGANIZATION:-library}"
NAMESPACE_OBSERVABILITY="observability"
CERT_DIR="$HOME/symphony/api/certificates"
K3S_VERSION="${K3S_VERSION:-v1.31.4+k3s1}"
GHCR_REGISTRY="ghcr.io"
GHCR_ORG="margo"
SYMPHONY_IMAGE="margo-symphony-api"
SYMPHONY_TAG="latest"
SYMPHONY_IMAGE_REF="${GHCR_REGISTRY}/${GHCR_ORG}/${SYMPHONY_IMAGE}:${SYMPHONY_TAG}"
SYMPHONY_IMAGE_REF="${SYMPHONY_IMAGE_REF:-${GHCR_REGISTRY}/${GHCR_ORG}/${SYMPHONY_IMAGE}:${SYMPHONY_TAG}}"

# Load shared library
source "${SCRIPT_DIR}/lib/common.sh"

# Load all WFM modules
source "${SCRIPT_DIR}/modules/docker.sh"
source "${SCRIPT_DIR}/modules/go.sh"
source "${SCRIPT_DIR}/modules/helm.sh"
source "${SCRIPT_DIR}/modules/redis.sh"
source "${SCRIPT_DIR}/modules/oras.sh"
source "${SCRIPT_DIR}/modules/repositories.sh"
source "${SCRIPT_DIR}/modules/k3s.sh"
source "${SCRIPT_DIR}/modules/harbor.sh"
source "${SCRIPT_DIR}/modules/certificates.sh"
source "${SCRIPT_DIR}/modules/symphony.sh"
source "${SCRIPT_DIR}/modules/observability.sh"
source "${SCRIPT_DIR}/modules/packages.sh"


# Main orchestration
install_prerequisites() {
  echo "Running all pre-req setup tasks..."
  install_basic_utilities
  install_go
  install_docker_and_compose
  setup_k3s
  install_redis
  install_oras
  clone_symphony_repo
  clone_dev_repo

  setup_harbor
  trust_harbor_certificate
  build_custom_otel_container_images

  echo ""
  echo "-----------------------------------------------------------------------"
  echo "📦 Pushing pre-existing test-bed application packages to OCI Registry..."
  push_nextcloud_to_oci
  push_custom_otel_to_oci
  echo "✅ Setup completed!"
  echo "-----------------------------------------------------------------------"
}

install_basic_utilities() {
  local PACKAGES="curl dos2unix build-essential gcc libc6-dev jq"

  echo "🔄 Installing Basic utilities..."
  INSTALLATION_NEEDED="false"
  for pkg in $PACKAGES ; do
    if ! package_installed $pkg; then
      INSTALLATION_NEEDED="true"
      break
    fi
  done

  if [[ "${INSTALLATION_NEEDED}" == "true" ]] ; then
    sudo apt update && sudo apt install -y $PACKAGES
    echo "✅ Basic utilities installed"
  else
    echo "⚡️ Basic utilities already installed"
  fi

  install_helm
}

enable_tls_in_symphony_api() {
  cd $HOME
  echo "Enabling tls in symphony API server (will generate certs and seed their settings in symphony-api-margo.json)..."
  collect_certs_info
  generate_server_certs
  # replace value of "tls": false, to "tls": true
  sed -i "s|\"tls\": false|\"tls\": true|" "$HOME/symphony/api/symphony-api-margo.json"
  echo "TLS Config is setup and seeded in symphony-api-margo.json"
}


observability_stack_install(){
echo "Observability stack installation started"

  create_observability_namespace
  install_jaeger
  install_prometheus
  install_grafana
  install_loki
  echo "Observability stack installation completed"
}

observability_stack_uninstall(){
    echo "Observability stack uninstall started"
    cd "$HOME/sandbox/scripts/observability" || { echo '❌ observability dir missing'; exit 1; }

    # Uninstall helm releases only if they exist
    for release in $PROM_RELEASE $JAEGER_RELEASE $GRAFANA_RELEASE $LOKI_RELEASE; do
        if helm status $release -n "$NAMESPACE_OBSERVABILITY" >/dev/null 2>&1; then
            echo "🗑️ Uninstalling $release..."
            helm uninstall $release --namespace "$NAMESPACE_OBSERVABILITY"
        else
            echo "⏭️ $release not found, skipping..."
        fi
    done

    # Wait for pods to be completely terminated
    echo "Waiting for pods to be terminated..."
    kubectl wait --for=delete pods -l app.kubernetes.io/instance=jaeger --timeout=300s || true
    kubectl wait --for=delete pods -l app.kubernetes.io/instance=grafana --timeout=300s || true
    kubectl wait --for=delete pods -l app.kubernetes.io/instance=loki --timeout=300s || true
    kubectl wait --for=delete pods -l app.kubernetes.io/instance=prometheus --timeout=300s || true

    # CHANGED: Remove only prometheus-values.yaml and loki-values.yaml
    rm -f prometheus-values.yaml loki-values.yaml

    echo "Observability stack uninstall completed"
}

# ----------------------------
# Uninstall Functions (Reverse Chronological Order)
# ----------------------------
uninstall_prerequisites() {
  echo "Running complete uninstallation in reverse chronological order..."

  # Step 1: Remove Symphony binaries and builds
  cleanup_symphony_builds

  # Step 2: Remove cloned repositories
  remove_cloned_repositories

  # Step 3: Uninstall Rust
  uninstall_rust

  # Step 4: Uninstall Docker and Docker Compose
  uninstall_docker_compose

  # Step 5: Uninstall Go
  uninstall_go

  # Step 6: Stop harbor service
  stop_harbor_service

  # Step 7: Remove basic utilities and cleanup
  cleanup_basic_utilities

  echo "Complete uninstallation finished"
}


cleanup_symphony_builds() {
  echo "1. Cleaning up Symphony builds..."

  # Remove built binaries
  [ -f "$HOME/symphony/api/symphony-api" ] && rm -f "$HOME/symphony/api/symphony-api" && echo "✅ Removed symphony-api binary"
  [ -f "$HOME/symphony/cli/maestro" ] && rm -f "$HOME/symphony/cli/maestro" && echo "✅ Removed maestro CLI binary"

  # Clean Rust build artifacts
  RUST_DIR="$HOME/symphony/api/pkg/apis/v1alpha1/providers/target/rust"
  if [ -d "$RUST_DIR/target" ]; then
    rm -rf "$RUST_DIR/target" && echo "✅ Removed Rust build artifacts"
  fi

  # remove the generated server cerificates as well
  rm -rf $CERT_DIR

  # Clean Go build cache
  if command -v go >/dev/null 2>&1; then
    go clean -cache -modcache 2>/dev/null && echo "✅ Cleaned Go build cache"
  fi
}


remove_cloned_repositories() {
  echo "2. Removing cloned repositories..."

  # Remove sandbox
  [ -d "$HOME/sandbox" ] && sudo rm -rf "$HOME/sandbox" && echo "✅ Removed sandbox repository"

  # Remove symphony repo
  [ -d "$HOME/symphony" ] && sudo rm -rf "$HOME/symphony" && echo "✅ Removed symphony repository"
}

uninstall_rust() {
  echo "3. Uninstalling Rust..."

  if [ -d "$HOME/.cargo" ]; then
    # Remove Rust installation
    if command -v rustup >/dev/null 2>&1; then
      rustup self uninstall -y && echo "✅ Uninstalled Rust via rustup"
    else
      rm -rf "$HOME/.cargo" "$HOME/.rustup" && echo "✅ Removed Rust directories manually"
    fi

    # Remove from PATH in shell profiles
    sed -i '/\.cargo\/env/d' "$HOME/.bashrc" "$HOME/.profile" 2>/dev/null
    echo "✅ Removed Rust from shell profiles"
  else
    echo "ℹ️ Rust was not installed"
  fi
}

uninstall_docker_compose() {
  echo "4. Uninstalling Docker and Docker Compose..."

  # Stop Docker daemon
  systemctl stop docker 2>/dev/null && echo "✅ Stopped Docker daemon"
  systemctl disable docker 2>/dev/null && echo "✅ Disabled Docker daemon"

  # Remove Docker Compose
  [ -f "/usr/local/bin/docker-compose" ] && rm -f "/usr/local/bin/docker-compose" && echo "✅ Removed Docker Compose"

  # Remove Docker (optional - uncomment if you want complete removal)
  # apt-get remove -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  # rm -rf /var/lib/docker /etc/docker
  # groupdel docker 2>/dev/null
  # echo "✅ Completely removed Docker"

  echo "⚠️ Docker engine left installed (remove manually if needed)"
}

uninstall_go() {
  echo "5. Uninstalling Go..."

  # Remove Go installation
  [ -d "/usr/local/go" ] && rm -rf "/usr/local/go" && echo "✅ Removed Go from /usr/local/go"

  # Remove Go from PATH in shell profiles
  sed -i '/\/usr\/local\/go\/bin/d' "$HOME/.bashrc" "$HOME/.profile" 2>/dev/null

  # Remove GOPATH and other Go environment variables
  sed -i '/GOPATH\|GOROOT\|GOPRIVATE/d' "$HOME/.bashrc" "$HOME/.profile" 2>/dev/null

  # Clear Go environment for current session
  unset GOPATH GOROOT GOPRIVATE

  echo "✅ Removed Go installation and environment variables"
}


cleanup_basic_utilities() {
  echo "7. Final cleanup of basic utilities..."

  # Remove temporary files
  rm -f /tmp/go.tar.gz /tmp/resp.json /tmp/headers.txt get-docker.sh 2>/dev/null && echo "✅ Removed temporary files"

  # Clear exported variables
  unset EXPOSED_HARBOR_HOST EXPOSED_HARBOR_PORT EXPOSED_SYMPHONY_HOST EXPOSED_SYMPHONY_PORT

  # Note: Not removing curl as it might be needed by system
  echo "⚠️ Basic utilities (curl) left installed as they may be system dependencies"

  echo "✅ Environment cleanup completed"
  echo ""
  echo "🔄 Please restart your shell or run 'source ~/.bashrc' to apply PATH changes"
}


start_symphony() {
  echo "Starting Symphony API server on..."
  export PATH="$PATH:/usr/local/go/bin"; # TODO: remove this line as this is being set while installing go

  export GOINSECURE='github.com/margo/*'
  export GONOPROXY='github.com/margo/*'
  export GONOSUMDB='github.com/margo/*'
  export GOPRIVATE='github.com/margo/*'

  if [[ -n "$GITHUB_USER" && -n "$GITHUB_TOKEN" ]]; then
    git config --global url."https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/";
    echo "Using GitHub credentials for user: $GITHUB_USER"
  fi

  go env -w GOPRIVATE="github.com/margo/*";

  # Build phase
  build_maestro_cli
  # verify_symphony_api
  enable_tls_in_symphony_api
  start_symphony_api_container
}

stop_symphony() {
  # TODO: this piece of code require corrections
  echo "Stopping and removing Symphony API container..."

  # Stop the container if running
  if docker ps --format '{{.Names}}' | grep -q "symphony-api-container"; then
    docker stop symphony-api-container && echo '✅ Symphony API container stopped'
  fi

  # Remove the container if it exists
  if docker ps -a --format '{{.Names}}' | grep -q "symphony-api-container"; then
    docker rm symphony-api-container && echo '✅ Symphony API container removed'
  else
    echo 'ℹ️ Symphony API container not found'
  fi

  echo "proceeding to remove the systemd files..."
  remove_symphony_api_systemd_service

  # Prompt user to delete Redis data
  echo ""
  echo "⚠️  Warning: Deleting Redis data will require device re-onboarding"
  read -p "Do you want to delete Redis data? (y/n): " delete_redis

  if [[ "$delete_redis" =~ ^[Yy]$ ]]; then
    echo "Flushing Redis data..."
    if redis-cli flushall; then
      echo '✅ Redis data deleted successfully'
      echo 'ℹ️ Device re-onboarding will be required'
    else
      echo '❌ Failed to delete Redis data'
    fi
  else
    echo 'ℹ️ Redis data preserved'
  fi
}


# Update the show_menu function to include uninstall option
show_menu() {
  clear
  load_wfm_env || true
  echo "Choose an option:"
  echo "1) PreRequisites: Setup"
  echo "2) PreRequisites: Cleanup"
  echo "3) Symphony: Start"
  echo "4) Symphony: Stop"
  echo "5) ObservabilityStack: Start"
  echo "6) ObservabilityStack: Stop"
  echo "7) Exit"
  read -p "Enter choice [1-7]: " choice
  case $choice in
    1) install_prerequisites ;;
    2) uninstall_prerequisites ;;
    3) start_symphony ;;
    4) stop_symphony ;;
    5) observability_stack_install ;;
    6) observability_stack_uninstall ;;
    7) echo "👋 Goodbye!"; exit 0 ;;
    *) echo "⚠️ Invalid choice"; sleep 2 ;;
  esac

  pause
}

# ----------------------------
# Main Script Execution
# ----------------------------
main_loop() {
  while true; do
    show_menu
  done
}

if [[ -z "$1" ]]; then
  main_loop
else
  load_wfm_env || true
  case "$1" in
    install) install_prerequisites ;;
    uninstall) uninstall_prerequisites ;;
    start) start_symphony ;;
    stop) stop_symphony ;;
    obs-install) observability_stack_install ;;
    obs-uninstall) observability_stack_uninstall ;;
    *)
      echo "Usage: $0 {install|uninstall|start|stop|obs-install|obs-uninstall}"
      exit 1
      ;;
  esac
fi
