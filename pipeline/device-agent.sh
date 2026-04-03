#!/bin/bash
set -e
export PATH="$PATH:/usr/local/go/bin"

# ----------------------------
# Load environment file
# ----------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"


load_device_agent_env() {

  if [[ -n "$_DEVICE_ENV_LOADED" ]]; then
    return 0
  fi
  export _DEVICE_ENV_LOADED=1

  local device="${1:-${DEVICE_TYPE:-}}"

  if [[ -z "$device" ]]; then
    echo "Select device type:"
    echo "  1) Docker"
    echo "  2) K3s"
    echo -n "Enter choice [1-2]: "
    read -r choice

    case "$choice" in
      1) device="docker" ;;
      2) device="k3s" ;;
      *)
        echo "[ERROR] Invalid choice (expected 1 or 2)"
        return 1
        ;;
    esac
  fi

  device="${device,,}"

  if [[ "$device" != "docker" && "$device" != "k3s" ]]; then
    echo "[ERROR] Invalid device type: '$device'"
    return 1
  fi

  local env_file="$SCRIPT_DIR/device-agent.env"
  if [[ ! -f "$env_file" ]]; then
    echo "[ERROR] Env file not found: $env_file"
    return 1
  fi

  export DEVICE_TYPE="$device"

  echo "[INFO] Device type selected: $DEVICE_TYPE"
  #echo "[INFO] Loading environment: $env_file"

  set -a
  source "$env_file"
  set +a
}
load_device_agent_env "$1" 2>/dev/null || true

# ----------------------------
# Environment & Validation Functions
# ----------------------------

#--- Github Settings to pull the code (can be overridden via env)
GITHUB_USER="${GITHUB_USER:-}"  # Set via env or leave empty
GITHUB_TOKEN="${GITHUB_TOKEN:-}"  # Set via env or leave empty

#--- harbor settings (can be overridden via env)
EXPOSED_HARBOR_HOST="${EXPOSED_HARBOR_HOST:-localhost}"
EXPOSED_HARBOR_PORT="${EXPOSED_HARBOR_PORT:-8081}"

#--- branch details (can be overridden via env)
SANDBOX_REPO_BRANCH="${SANDBOX_REPO_BRANCH:-dev-sprint-6}"
WFM_HOST="${WFM_HOST:-localhost}"
WFM_PORT="${WFM_PORT:-8082}"


#--- Registry settings (can be overridden via env)
REGISTRY_URL="${REGISTRY_URL:-http://${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}}"
REGISTRY_USER="${REGISTRY_USER:-admin}"
REGISTRY_PASS="${REGISTRY_PASS:-Harbor12345}"

# variables for observability stack
NAMESPACE_OBSERVABILITY="observability"
PROMTAIL_RELEASE="promtail"
OTEL_RELEASE="otel-collector"

# Pinned software versions (can be overridden via env)
DOCKER_VERSION="${DOCKER_VERSION:-29.1.2}"
DOCKER_COMPOSE_VERSION="${DOCKER_COMPOSE_VERSION:-5.0.0}"

# Stable version as of December 2024
K3S_VERSION="${K3S_VERSION:-v1.31.4+k3s1}"

# ----------------------------
# GHCR Image References
# ----------------------------
GHCR_REGISTRY="ghcr.io"
GHCR_ORG="margo"
workload_Fleet_Management_Client_IMAGE="margo.org/workload-fleet-management-client"
workload_Fleet_Management_Client_IMAGE_TAG="latest"
workload_Fleet_Management_Client_IMAGE_REF="${GHCR_REGISTRY}/${GHCR_ORG}/${workload_Fleet_Management_Client_IMAGE}:${workload_Fleet_Management_Client_IMAGE_TAG}"

# Load shared library
source "${SCRIPT_DIR}/lib/common.sh"

# Load all WFM modules
source "${SCRIPT_DIR}/modules/docker.sh"
source "${SCRIPT_DIR}/modules/go.sh"
source "${SCRIPT_DIR}/modules/helm.sh"
source "${SCRIPT_DIR}/modules/repositories.sh"
source "${SCRIPT_DIR}/modules/k3s.sh"
source "${SCRIPT_DIR}/modules/harbor.sh"
source "${SCRIPT_DIR}/modules/certificates.sh"
source "${SCRIPT_DIR}/modules/agent.sh"
source "${SCRIPT_DIR}/modules/observability.sh"


export GOINSECURE='github.com/margo/*'
export GONOPROXY='github.com/margo/*'
export GONOSUMDB='github.com/margo/*'
export GOPRIVATE='github.com/margo/*'

validate_pre_required_vars() {
  local required_vars=("SANDBOX_REPO_BRANCH" "WFM_HOST" "WFM_PORT")
  for var in "${required_vars[@]}"; do
    if [ -z "${!var}" ]; then
      echo "Error: Required environment variable $var is not set"
      exit 1
    fi
  done
}

validate_start_required_vars() {
  local required_vars=("WFM_HOST" "WFM_PORT")
  for var in "${required_vars[@]}"; do
    if [ -z "${!var}" ]; then
      echo "Error: Required environment variable $var is not set"
      exit 1
    fi
  done
}

# ----------------------------
# Go Installation Functions
# ----------------------------
install_basic_utilities() {
  sudo apt update -y
  sudo apt install -y curl git dos2unix build-essential gcc libc6-dev
  echo "Installation complete: curl, git, and build tools installed."

  # Only install Helm for k3s device type
  if [ "$DEVICE_TYPE" = "k3s" ]; then
    INSTALL_HELM_V3_15_1=true
    HELM_VERSION="3.15.1"
    HELM_TAR="helm-v${HELM_VERSION}-linux-amd64.tar.gz"
    HELM_BIN_DIR="/usr/local/bin"
    install_helm
    echo "✅ Helm installed for k3s device"
  else
    echo "ℹ️ Skipping Helm installation for docker device type"
  fi
}


# ----------------------------
# Main Orchestration Functions
# ----------------------------
install_prerequisites() {
  echo "Installing prerequisites: k3s and others ..."
  validate_pre_required_vars
  install_go
  install_basic_utilities
  install_docker_and_compose
  clone_dev_repo
  # Only install k3s for k3s device type
  if [ "$DEVICE_TYPE" = "k3s" ]; then
    setup_k3s
    add_container_registry_mirror_to_k3s
  fi

  echo 'prerequisites installation completed.'
}


start_device_agent_docker() {
  echo "Building and starting workload-fleet-management-client ..."
  validate_start_required_vars
  update_agent_sbi_url
  build_device_agent_docker
  start_device_agent_docker_service
  echo 'workload-fleet-management-client docker-container started'
}

start_device_agent_kubernetes() {
  echo "Building and starting workload-fleet-management-client with ServiceAccount authentication..."
  validate_start_required_vars
  build_start_device_agent_k3s_service
  echo '✅ workload-fleet-management-client-pod started with ServiceAccount authentication'
}

stop_device_agent_docker() {
  echo "Stopping workload-fleet-management-client ..."
  stop_device_agent_service_docker
  echo "Device's Workload Fleet Management Client stopped"
}


uninstall_prerequisites() {
  cleanup_device_agent
}

create_observability_systemd_service() {
  echo "🔧 Creating systemd service for observability (OTEL + Promtail) auto-start..."

  local obs_dir="$HOME/sandbox/pipeline/observability"

  # Create systemd unit file
  sudo tee /etc/systemd/system/observability.service > /dev/null <<EOF
[Unit]
Description=Margo Observability Stack (OTEL Collector + Promtail)
Requires=docker.service
After=docker.service network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=${obs_dir}
ExecStartPre=/bin/sleep 10
ExecStart=/usr/bin/docker compose -f docker-compose-observability.yml up -d
ExecStop=/usr/bin/docker compose -f docker-compose-observability.yml down
TimeoutStartSec=0

[Install]
WantedBy=multi-user.target
EOF

  # Reload systemd and enable the service so it runs at boot
  sudo systemctl daemon-reload
  sudo systemctl enable observability.service

  echo "✅ Observability systemd service created and enabled"
  echo "📋 Service will run: /usr/bin/docker compose -f docker-compose-observability.yml up -d"
  echo "📁 Working directory: ${obs_dir}"
}

cleanup_residual() {
  rm -rf "$HOME/sandbox"
  rm -rf "$HOME/symphony"
}

create_device_rsa_certs() {
  CERT_DIR="$HOME/certs"

  # If certs exists but is not a directory, remove it
  if [ -e "$CERT_DIR" ] && [ ! -d "$CERT_DIR" ]; then
    echo "[WARNING] $CERT_DIR exists but is not a directory — removing."
    rm -f "$CERT_DIR"
  fi

  mkdir -p "$CERT_DIR"
  cd "$CERT_DIR" || exit 1

  echo "Generating RSA device certs..."
  # Generate RSA private key (2048-bit)
  openssl genrsa -out device-private.key 2048

  # Generate self-signed certificate
  openssl req -new -x509 -key device-private.key -out device-public.crt -days 365 \
    -subj "/C=IN/ST=GGN/L=Sector 48/O=Margo/CN=margo-device"
  echo "✅ RSA Cert generation has been completed."



}

create_device_ecdsa_certs() {
  CERT_DIR="$HOME/certs"

  if [ ! -d "$CERT_DIR" ]; then
    echo "Cert directory not found. Creating $CERT_DIR ..."
    mkdir -p "$CERT_DIR"
  else
    echo "Using existing cert directory: $CERT_DIR"
  fi

  cd "$CERT_DIR" || exit 1
  echo "Generating ECDSA device certs..."
  # Generate ECDSA private key (P-256 curve)
  openssl ecparam -genkey -name prime256v1 -out device-ecdsa.key

  # Generate self-signed certificate
  openssl req -new -x509 -key device-ecdsa.key -out device-ecdsa.crt -days 365 \
    -subj "/C=IN/ST=GGN/L=Sector 48/O=Margo/CN=margo-device"
  echo "✅ ECDSA Cert generation has been completed."



}
pause() {
  echo
  read -rp "Press Enter to continue..." _
}

# ----------------------------
# Menu Functions
# ----------------------------

show_menu() {
  echo "Choose an option:"
  echo "1) Install-prerequisites"
  echo "2) Uninstall-prerequisites"
  echo "3) WFM-Client-Start(docker-compose-device)"
  echo "4) WFM-Client-Stop(docker-compose-device)"
  echo "5) WFM-Client-Start(k3s-device)"
  echo "6) WFM-Client-Stop(k3s-device)"
  echo "7) WFM-Client-Status"
  echo "8) OTEL-collector-promtail-installation"
  echo "9) OTEL-collector-promtail-uninstallation"
  echo "10) cleanup-residual"
  echo "11) create_device_rsa_certs"
  echo "12) create_device_ecdsa_certs"
  echo "13) Exit"
  read -rp "Enter choice [1-13]: " choice
  case $choice in
    1) install_prerequisites;;
    2) uninstall_prerequisites;;
    3) start_device_agent_docker ;;
    4) stop_device_agent_docker ;;
    5) start_device_agent_kubernetes ;;
    6) stop_device_agent_kubernetes ;;
    7) show_status ;;
    8) install_otel_collector_promtail_wrapper ;;
    9) uninstall_otel_collector_promtail_wrapper ;;
    10) cleanup_residual;;
    11) create_device_rsa_certs ;;
    12) create_device_ecdsa_certs ;;
    13) echo "👋 Goodbye!"; exit 0 ;;
    *) echo "Invalid choice" ;;
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
  # No arguments - prompt for device type FIRST, then run interactive menu
  if ! load_device_agent_env; then
    echo "[ERROR] Failed to load device agent environment"
    exit 1
  fi
  main_loop

elif [[ "$1" == "docker" || "$1" == "k3s" ]] && [[ -z "$2" ]]; then
  # Device type specified but no command - run interactive menu
  if ! load_device_agent_env "$1"; then
    echo "[ERROR] Failed to load device agent environment"
    exit 1
  fi
  main_loop

elif [[ "$1" == "docker" || "$1" == "k3s" ]] && [[ -n "$2" ]]; then
  # Device type + command - execute command
  case "$2" in
    install) install_prerequisites ;;
    uninstall) uninstall_prerequisites ;;
    start-docker) start_device_agent_docker ;;
    stop-docker) stop_device_agent_docker ;;
    start-k3s) start_device_agent_kubernetes ;;
    stop-k3s) stop_device_agent_kubernetes ;;
    status) show_status ;;
    otel-install) install_otel_collector_promtail_wrapper ;;
    otel-uninstall) uninstall_otel_collector_promtail_wrapper ;;
    cleanup) cleanup_residual ;;
    create-rsa-certs) create_device_rsa_certs ;;
    create-ecdsa-certs) create_device_ecdsa_certs ;;
    *)
      echo "[ERROR] Unknown command: $2"
      echo "Available: install, uninstall, start-docker, stop-docker, start-k3s, stop-k3s, status, otel-install, otel-uninstall, cleanup, create-rsa-certs, create-ecdsa-certs"
      exit 1
      ;;
  esac

else
  # Invalid usage - device type is mandatory
  echo "[ERROR] Invalid usage. Device type (docker/k3s) is required."
  echo ""
  echo "Usage Examples:"
  echo "  Interactive mode:"
  echo "    $0              # Prompts for device type"
  echo "    $0 docker       # Interactive menu for docker"
  echo "    $0 k3s          # Interactive menu for k3s"
  echo ""
  echo "  Command mode:"
  echo "    $0 docker install"
  echo "    $0 k3s start-k3s"
  exit 1
fi
