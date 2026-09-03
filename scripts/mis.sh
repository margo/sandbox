
#!/bin/bash
set -e

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"



# Load environment
load_mis_env() {
  local env_file="$SCRIPT_DIR/mis.env"
  if [[ ! -f "$env_file" ]]; then
    echo "[WARN] mis.env not found at: $env_file"
    return 1
  fi
  echo "[INFO] Loading environment from: $env_file"
  set -a
  source "$env_file"
  set +a
}
load_mis_env || true

GITHUB_USER="${GITHUB_USER:-}"
GITHUB_TOKEN="${GITHUB_TOKEN:-}"
SANDBOX_REPO_BRANCH="${SANDBOX_REPO_BRANCH:-main}"
MIS_HOST="${EXPOSED_MIS_HOST:-mis.margo.org}"
MIS_PORT="${EXPOSED_MIS_PORT:-9443}"

# ----------------------------
# GHCR Image References
# ----------------------------
GHCR_REGISTRY="ghcr.io"
GHCR_ORG="margo"
mis_IMAGE="margo.org/margo-identity-service"
mis_IMAGE_TAG="latest"
mis_IMAGE_REF="${GHCR_REGISTRY}/${GHCR_ORG}/${mis_IMAGE}:${mis_IMAGE_TAG}"
deploy_dir="$HOME/mis-deployment"
certs_dir="$deploy_dir/certs"



# Load shared library
source "${SCRIPT_DIR}/lib/common.sh"
source "${SCRIPT_DIR}/modules/repositories.sh"
source "${SCRIPT_DIR}/modules/go.sh"

# ----------------------------
# Install Pre-requisites 
# ----------------------------

install_prerequisites() {
  echo "Running all pre-req setup tasks..."
  install_basic_utilities
  install_go
  install_docker_and_compose
}

install_basic_utilities() {
  local PACKAGES="curl dos2unix build-essential gcc libc6-dev jq make"

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

}

# ----------------------------
# Uninstall Functions (Reverse Chronological Order)
# ----------------------------
uninstall_prerequisites() {
  echo "Running complete uninstallation in reverse chronological order..."

  # Step 1: Remove cloned repositories
  remove_cloned_repositories

  # Step 2: Uninstall Docker and Docker Compose
  uninstall_docker_compose

  # Step 3: Uninstall Go
  uninstall_go

  # Step 4: Remove basic utilities and cleanup
  cleanup_basic_utilities

  echo "Complete uninstallation finished"
}


remove_cloned_repositories() {
  echo "1. Removing cloned repositories..."

  # Remove sandbox
  [ -d "$HOME/sandbox" ] && sudo rm -rf "$HOME/sandbox" && echo "✅ Removed sandbox repository"

}

uninstall_docker_compose() {
  echo "2. Uninstalling Docker and Docker Compose..."

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
  echo "3. Uninstalling Go..."

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
  echo "4. Final cleanup of basic utilities..."

  # Remove temporary files
  rm -f /tmp/go.tar.gz /tmp/resp.json /tmp/headers.txt get-docker.sh 2>/dev/null && echo "✅ Removed temporary files"

  # Clear exported variables
  unset EXPOSED_MIS_HOST EXPOSED_MIS_PORT

  # Note: Not removing curl as it might be needed by system
  echo "⚠️ Basic utilities (curl) left installed as they may be system dependencies"

  echo "✅ Environment cleanup completed"

  cleanup_docker_resources

  echo ""
  echo "🔄 Please restart your shell or run 'source ~/.bashrc' to apply PATH changes"
}

cleanup_docker_resources() {
    if ! command -v docker >/dev/null 2>&1; then
        echo "Docker is not installed. Skipping cleanup."
        return 0
    fi

    echo "Removing exited containers..."
    docker container prune -f >/dev/null 2>&1

    echo "Removing unused volumes..."
    docker volume prune -f >/dev/null 2>&1

    echo "Docker cleanup completed."
}

# ----------------------------
# Setup Initial Trust 
# ----------------------------

setup_factory() {
    invoke_pki_gen
    cp -r ./certs "$deploy_dir"
    echo "placed certificates in $certs_dir"
}

invoke_pki_gen() {
  local script_path="$SCRIPT_DIR/mis/utils/pki_gen.sh"

  if [[ -z "$script_path" ]]; then
      echo "Error: No script path provided."
      return 1
  fi

  if [[ ! -f "$script_path" ]]; then
      echo "Error: Script not found at '$script_path'."
      return 1
  fi

  if [[ ! -x "$script_path" ]]; then
      echo "Warning: Script is not executable. Attempting to run with bash..."
      bash "$script_path" --automated
  else
      "$script_path" --automated
  fi
}


# ----------------------------
# MIS Installation 
# ----------------------------
install_mis() {
  # TODO: Add Github CI related changes here

  # If it is not Github CI then: 
  setup_mis_deployment
  update_config "$MIS_HOST" "$MIS_PORT" "$deploy_dir/configuration.json"
  start_mis_deployment
}

update_config() {
    local host="$1"
    local port="$2"
    local config_path="$3"

    # Validate inputs
    if [[ -z "$host" || -z "$port" || -z "$config_path" ]]; then
        echo "Error: Missing arguments."
        echo "Usage: update_config <host> <port> <config_path>"
        return 1
    fi

    # Check if config file exists
    if [[ ! -f "$config_path" ]]; then
        echo "Error: Configuration file not found at '$config_path'."
        return 1
    fi

    # Check if jq is installed
    if ! command -v jq &>/dev/null; then
        echo "Error: 'jq' is required but not installed."
        return 1
    fi

    # Extract trust domain by stripping the first subdomain (e.g., mis.margo.org -> margo.org)
    local trust_domain
    trust_domain=$(echo "$host" | cut -d'.' -f2-)

    # Update the config file using jq
    local tmp_file
    tmp_file=$(mktemp)

    jq --arg port ":${port}" \
       --arg trust_domain "$trust_domain" \
       '.https.addr = $port | .trustDomain = $trust_domain' \
       "$config_path" > "$tmp_file" && mv "$tmp_file" "$config_path"

    if [[ $? -eq 0 ]]; then
        echo "Configuration updated successfully:"
        echo "  https.addr   => :${port}"
        echo "  trustDomain  => ${trust_domain}"
    else
        echo "Error: Failed to update configuration."
        rm -f "$tmp_file"
        return 1
    fi
}


start_mis_deployment() {

    echo "[INFO] Changing directory to: $deploy_dir"
    cd "$deploy_dir" || { echo "[ERROR] Failed to change directory to '$deploy_dir'. Aborting."; return 1; }

    echo "[INFO] Checking for docker-compose.yaml in $deploy_dir"
    if [[ ! -f "docker-compose.yaml" ]]; then
        echo "[ERROR] docker-compose.yaml not found in '$deploy_dir'. Aborting."
        return 1
    fi
    echo "[INFO] docker-compose.yaml found."

    if [[ -d "$certs_dir" ]]; then
        echo "[INFO] Certs directory found, validating certificates..."

        local crt_pair_count
        local key_pair_count
        local standalone_crt_count

        crt_pair_count=$(find "$certs_dir" -maxdepth 1 -name "*.crt" | wc -l)
        key_pair_count=$(find "$certs_dir" -maxdepth 1 -name "*.key" | wc -l)
        standalone_crt_count=$crt_pair_count

        if [[ "$crt_pair_count" -lt 2 ]] || [[ "$key_pair_count" -lt 2 ]] || [[ "$standalone_crt_count" -lt 1 ]]; then
            echo "[ERROR] Certs directory exists but required certificates are missing."
            echo "[ERROR] Expected: at least 2 .crt files, 2 .key files, and 1 standalone .crt. Aborting."
            return 1
        fi

        echo "[INFO] Certificate check passed (found $crt_pair_count .crt, $key_pair_count .key files)."
    else
        echo "[ERROR] No certs directory found, Aborting."
        return 1
    fi

    echo "[INFO] Starting Margo Identity Service Docker Container"
    if ! docker compose -f docker-compose.yaml up -d; then
        echo "[ERROR] 'docker compose up -d' failed. Check Docker logs for details."
        return 1
    fi

    echo "[INFO] Margo Identity Service started successfully."
}


# ----------------------------
# MIS Uninstallation 
# ----------------------------

uninstall_mis(){
  # TODO: Add Github CI related changes here

  # If it is not Github CI then: 
  echo "[INFO] Changing directory to: $deploy_dir"
  cd "$deploy_dir" || { echo "[ERROR] Failed to change directory to '$deploy_dir'. Aborting."; return 1; }

  echo "[INFO] Checking for docker-compose.yaml in $deploy_dir"
  if [[ ! -f "docker-compose.yaml" ]]; then
      echo "[ERROR] docker-compose.yaml not found in '$deploy_dir'. Aborting."
      return 1
  fi
  echo "[INFO] docker-compose.yaml found."

  echo "[INFO] Starting Margo Identity Service Docker Container"
  if ! docker compose -f docker-compose.yaml down; then
      echo "[ERROR] 'docker compose down' failed. Check Docker logs for details."
      return 1
  fi

  echo "[INFO] Margo Identity Service stopped successfully."

 # Cleanup
  rm -rf $deploy_dir
  
}

# ----------------------------
# Main Menu 
# ----------------------------


show_menu() {
  clear
  load_mis_env || true
  echo "Choose an option:"
  echo "1) PreRequisites: Setup"
  echo "2) PreRequisites: Cleanup"
  echo "3) Factory Bootstrap: Generate Root CAs"
  echo "4) Margo Identity Service: Install"
  echo "5) Margo Identity Service: Uninstall"
  echo "6) Exit"
  read -p "Enter choice [1-5]: " choice
  case $choice in
    1) install_prerequisites ;;
    2) uninstall_prerequisites ;;
    3) setup_factory ;;
    4) install_mis ;;
    5) uninstall_mis ;;
    6) echo "👋 Goodbye!"; exit 0 ;;
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
  load_mis_env || true
  case "$1" in
    install) install_prerequisites ;;
    uninstall) uninstall_prerequisites ;;
    setup-factory) setup_initial_trust ;;
    mis-install) install_mis ;;
    mis-uninstall) uninstall_mis ;;
    *)
      echo "Usage: $0 {install|uninstall|setup-factory|mis-install|mis-uninstall}"
      exit 1
      ;;
  esac
fi

 