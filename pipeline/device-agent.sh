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

install_docker_and_compose() {
  cd $HOME

  # Define pinned versions
  local DOCKER_VERSION="${DOCKER_VERSION:-29.1.2}"
  local DOCKER_COMPOSE_VERSION="${DOCKER_COMPOSE_VERSION:-5.0.0}"
  local UBUNTU_CODENAME=$(lsb_release -cs 2>/dev/null || echo "noble")

  # Install Docker if not present
  if ! command -v docker >/dev/null 2>&1; then
    echo "Docker not found. Installing Docker ${DOCKER_VERSION}..."

    # Remove old Docker packages
    apt-get remove -y docker docker-engine docker.io containerd runc 2>/dev/null || true

    # Add Docker's official GPG key and repository
    apt-get update
    apt-get install -y ca-certificates curl
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
    chmod a+r /etc/apt/keyrings/docker.asc

    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
      ${UBUNTU_CODENAME} stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null

    apt-get update

    # Install specific Docker version
    apt-get install -y \
      docker-ce=5:${DOCKER_VERSION}-1~ubuntu.24.04~${UBUNTU_CODENAME} \
      docker-ce-cli=5:${DOCKER_VERSION}-1~ubuntu.24.04~${UBUNTU_CODENAME} \
      containerd.io=1.7.27-1 \
      docker-buildx-plugin=0.23.0-1~ubuntu.24.04~${UBUNTU_CODENAME}

    usermod -aG docker $USER
    echo "✅ Docker ${DOCKER_VERSION} installed successfully"
  else
    echo 'Docker already installed.'
    CURRENT_DOCKER_VERSION=$(docker version --format '{{.Server.Version}}' 2>/dev/null)
    echo "Current Docker version: $CURRENT_DOCKER_VERSION"
  fi

  # Install specific Docker Compose plugin version
  if ! apt list --installed 2>/dev/null | grep -q docker-compose-plugin; then
    echo "Installing Docker Compose plugin ${DOCKER_COMPOSE_VERSION}..."
    apt-get update
    apt-get install -y docker-compose-plugin=${DOCKER_COMPOSE_VERSION}-1~ubuntu.24.04~${UBUNTU_CODENAME}
  else
    echo 'Docker Compose plugin already installed.'
    CURRENT_COMPOSE_VERSION=$(docker compose version --short 2>/dev/null | sed 's/v//')
    echo "Current Docker Compose version: v$CURRENT_COMPOSE_VERSION"
  fi

  # Remove old standalone binaries
  echo 'Cleaning up old docker-compose binaries...'
  rm -f /usr/local/bin/docker-compose /usr/bin/docker-compose 2>/dev/null || true

  # Verify versions
  echo ""
  echo "Docker version:"
  docker version | grep -E "Version|API version" | head -4
  echo ""
  echo "Docker Compose version:"
  docker compose version

  # Hold packages at current versions
  echo "🔒 Holding Docker packages at current versions..."
  apt-mark hold docker-ce docker-ce-cli docker-compose-plugin containerd.io docker-buildx-plugin

  echo "✅ Docker ${DOCKER_VERSION} and Docker Compose v${DOCKER_COMPOSE_VERSION} ready"
}




# Helm install/uninstall
install_helm() {
  cd $HOME
  if [ "${INSTALL_HELM_V3_15_1}" == "true" ]; then
    echo "Helm Setup"
    if command -v helm >/dev/null 2>&1 && [[ "$(helm version --short | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')" == "${HELM_VERSION}" ]]; then
        echo "Helm version ${HELM_VERSION} is already installed. Skipping."
    else
        echo "Downloading Helm version ${HELM_VERSION}..."
        if ! wget -q "https://get.helm.sh/${HELM_TAR}" ; then
            echo "Failed to download Helm."
            exit 1
        fi
        echo "Extracting Helm..."
        if ! tar -xzf "${HELM_TAR}" ; then
            echo "Failed to extract Helm tarball."
            exit 1
        fi
        echo "Moving Helm to ${HELM_BIN_DIR}..."
        if ! sudo mv "linux-amd64/helm" "${HELM_BIN_DIR}/" ; then
            echo "Failed to move Helm."
            exit 1
        fi
        echo "Helm binary moved successfully."
        echo "Cleaning up..."
        rm "${HELM_TAR}"
        rm -rf linux-amd64/
    fi
  fi
}


install_go() {
  if which go >/dev/null 2>&1; then
    echo 'Go already installed, skipping installation';
    go version;
  else
    echo 'Go not found, installing...';
    # rm -rf /usr/local/go /usr/bin/go
    wget "https://go.dev/dl/go1.23.2.linux-amd64.tar.gz" -O go.tar.gz;
    tar -C /usr/local -xzf go.tar.gz;
    rm go.tar.gz
    export PATH="$PATH:/usr/local/go/bin";
    which go;
    go version;
  fi
}

# ----------------------------
# Repository Functions
# ----------------------------
clone_dev_repo() {
  echo "Cloning sandbox on ($VM2_HOST)..."
  cd $HOME
  sudo rm -rf sandbox
  echo "Cloning sandbox branch: $SANDBOX_REPO_BRANCH"
 if [[ -n "$GITHUB_USER" && -n "$GITHUB_TOKEN" ]]; then 
    git clone \
          --branch "$SANDBOX_REPO_BRANCH" \
          --single-branch \
          --depth 1 \
          "https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/margo/sandbox.git" \
          "$HOME/sandbox"   
  else
    git clone \
          --branch "$SANDBOX_REPO_BRANCH" \
          --single-branch \
          --depth 1 \
          "https://github.com/margo/sandbox.git" \
          "$HOME/sandbox"
  fi
  cd "$HOME/sandbox"
  # git fetch --depth 1 --update-head-ok origin ${SANDBOX_REPO_BRANCH}:${SANDBOX_REPO_BRANCH} || echo 'Unable to fetch ${SANDBOX_REPO_BRANCH}'
  # git checkout ${SANDBOX_REPO_BRANCH} || echo 'Branch ${SANDBOX_REPO_BRANCH} not found'
  echo "sandbox repo checkout to branch ${SANDBOX_REPO_BRANCH} done!"
  cd ..
}

# ----------------------------
# Configuration Functions
# ----------------------------
update_agent_sbi_url() {
  echo 'Updating wfm.sbiUrl in workload-fleet-management-client config ...'
  sed -i "s|sbiUrl:.*|sbiUrl: https://$WFM_HOST:$WFM_PORT/v1alpha2/margo|" "$HOME/sandbox/poc/device/agent/config/config.yaml"
}

# ----------------------------
# K3s Installation Functions
# ----------------------------
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
  if ! check_k3s_installed; then
    echo "Installing k3s ${K3S_VERSION}..."
    install_k3s_dependencies

    # Install specific k3s version
    curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION="${K3S_VERSION}" sh -

    echo "✅ k3s ${K3S_VERSION} installed successfully"
  fi
}

verify_k3s_status() {
  echo 'Verifying k3s status...'
  sudo systemctl status k3s --no-pager || true
  sudo k3s kubectl get nodes || true

  # Show installed version
  echo ""
  echo "Installed k3s version:"
  k3s --version | head -1
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

install_vim() {
  echo "[INFO] Checking if Vim editor is installed..."
  if command -v vim >/dev/null 2>&1; then
    echo "[INFO] Vim is already installed."
    return
  fi

  echo "[INFO] Installing Vim..."
  if command -v apt >/dev/null 2>&1; then
    sudo apt update -y
    sudo apt install -y vim
  else
    sudo yum install -y vim || sudo dnf install -y vim
  fi

  echo "[SUCCESS] Vim installed and ready to use."
}


install_and_enable_ssh() {
  echo "[INFO] Checking OS type..."

  # Detect package manager
  if command -v apt >/dev/null 2>&1; then
    OS="debian"
    echo $OS
  elif command -v yum >/dev/null 2>&1 || command -v dnf >/dev/null 2>&1; then
    OS="rhel"
    echo $OS
  else
    echo "[ERROR] Unsupported OS. Only Debian/Ubuntu & RHEL/CentOS supported."
    return 1
  fi

  echo "[INFO] Installing OpenSSH Server..."
  if [ "$OS" = "debian" ]; then
    sudo apt update -y
    sudo apt install -y openssh-server
  else
    sudo yum install -y openssh-server || sudo dnf install -y openssh-server
  fi

  echo "[INFO] Enabling and starting SSH service..."
  UNIT=$(systemctl list-unit-files | awk '/^ssh\.service/ {print "ssh"} /^sshd\.service/ {print "sshd"}' | head -n1)

  if [ -z "$UNIT" ]; then
    echo "[ERROR] SSH service unit not found."
    return 1
  fi

  sudo systemctl enable "$UNIT"
  sudo systemctl restart "$UNIT"

  echo "[INFO] Verifying SSH status:"
  sudo sudo systemctl status ssh --no-pager || sudo systemctl status sshd
  echo "[SUCCESS] SSH service installed and running."
}

#-----------------------------------------------------------------
# Device Workload Fleet Management Client Runtime Configuration update based on Docker or K8s
#-----------------------------------------------------------------

enable_kubernetes_runtime() {
  CONFIG_FILE="$HOME/sandbox/helmchart/config/config.yaml"
  echo "Enabling Kubernetes section in config.yaml for ServiceAccount authentication..."
  sed -i \
  -e 's/^[[:space:]]*#\s*-\s*type:\s*KUBERNETES/- type: KUBERNETES/' \
  -e 's/^[[:space:]]*#\s*kubernetes:/  kubernetes:/' \
  -e 's/^[[:space:]]*-\s*type:\s*DOCKER/  # - type: DOCKER/' \
  -e 's/^[[:space:]]*docker:/  # docker:/' \
  -e 's/^[[:space:]]*url:/  # url:/' \
  "$CONFIG_FILE"

  # Fix certificate paths
  sed -i \
    -e 's|pubCertPath:.*|pubCertPath: /certs/device-public.crt|' \
    -e 's|path: "./config/device-private.key"|path: "/certs/device-private.key"|' \
    -e 's|path: "./config/ca-cert.pem"|path: "/certs/ca-cert.pem"|' \
    "$CONFIG_FILE"

  # Set kubeconfigPath to empty string for ServiceAccount authentication
  sed -i 's|kubeconfigPath:.*|kubeconfigPath: ""|' "$CONFIG_FILE"

  echo "✅ Kubernetes runtime enabled with ServiceAccount authentication"
}

enable_docker_runtime() {
  CONFIG_FILE="$HOME/sandbox/docker-compose/config/config.yaml"
  echo "Enabling docker section in config.yaml..."
  sed -i \
    -e 's/^[[:space:]]*#\s*- type: DOCKER/  - type: DOCKER/' \
    -e 's/^[[:space:]]*#\s*docker:/    docker:/' \
    -e 's/^[[:space:]]*#\s*url:/      url:/' \
    -e 's/^[[:space:]]*- type: KUBERNETES/  # - type: KUBERNETES/' \
    -e 's/^[[:space:]]*kubernetes:/    # kubernetes:/' \
    -e 's/^[[:space:]]*kubeconfigPath:/      # kubeconfigPath:/' \
    "$CONFIG_FILE"
}

# ----------------------------
# Device Workload Fleet Management Client Build Functions
# ----------------------------
build_device_agent_docker() {
  cd "$HOME/sandbox"
  echo 'Checking if workload-fleet-management-client image already exists in GHCR...'

  # Check if image already exists
  echo "Checking GHCR image: ${workload_Fleet_Management_Client_IMAGE_REF}"
  if docker manifest inspect "${workload_Fleet_Management_Client_IMAGE_REF}" >/dev/null 2>&1; then
    echo "Image exists in GHCR"
  else
    echo "Image does NOT exist in GHCR"
    return 1
  fi  

  echo "⬇️ Pulling image from GHCR..."
  docker pull "${workload_Fleet_Management_Client_IMAGE_REF}" || return 1

  echo "✅ Image ready locally: ${workload_Fleet_Management_Client_IMAGE_REF}"
     
}


# ----------------------------
# Device Workload Fleet Management Client Service Functions
# ----------------------------

create_device_agent_systemd_service() {
  echo "🔧 Creating systemd service for device-agent auto-start..."

  # Get the actual docker-compose directory path
  local compose_dir="$HOME/sandbox/docker-compose"

  # Create systemd service file
sudo tee /etc/systemd/system/device-agent.service > /dev/null <<EOF
  [Unit]
  Description=Margo Device Agent
  Requires=docker.service
  After=docker.service network-online.target
  Wants=network-online.target

  [Service]
  Type=oneshot
  RemainAfterExit=yes
  WorkingDirectory=${compose_dir}
  ExecStartPre=/bin/sleep 10
  ExecStart=/usr/bin/docker compose up -d
  ExecStop=/usr/bin/docker compose down
  TimeoutStartSec=0

  [Install]
  WantedBy=multi-user.target
EOF

  # Reload systemd and enable the service
  sudo systemctl daemon-reload
  sudo systemctl enable device-agent.service

  echo "✅ Device-agent systemd service created and enabled"
  echo "📋 Service will start device-agent automatically on boot"
  echo "📁 Working directory: ${compose_dir}"
}

start_device_agent_docker_service() {
  echo 'Starting workload-fleet-management-client...'
  cd "$HOME/sandbox/docker-compose"
  mkdir -p config

  if [ -f "$HOME/certs/device-private.key" ] && [ -f "$HOME/certs/device-public.crt" ] && [ -f "$HOME/certs/device-ecdsa.crt" ] && [ -f "$HOME/certs/device-ecdsa.key" ] && [ -f "$HOME/certs/ca-cert.pem" ]; then
    echo "Creating TLS secrets..."
    cp "$HOME/certs/device-private.key"  ./config
    cp "$HOME/certs/device-public.crt"   ./config
    cp "$HOME/certs/device-ecdsa.key"    ./config
    cp "$HOME/certs/device-ecdsa.crt"    ./config
    cp "$HOME/certs/ca-cert.pem"         ./config
    echo "Copied certs from \$HOME/certs to ./config"
      else
    echo "❌ device-start-failed: Required certificates missing in $HOME/certs (ca-cert.pem)"
        return 1
  fi

  cp ../poc/device/agent/config/capabilities.json ./config/
  cp ../poc/device/agent/config/config.yaml ./config/

  mkdir -p data
  enable_docker_runtime
  docker compose up -d

  #systemd service for auto-start if vm reboots
  create_device_agent_systemd_service
}

stop_device_agent_service_docker() {

  cd "$HOME/sandbox/docker-compose"
  docker compose down

  # Prompt user to delete /data folder
  echo ""
  echo "⚠️  Warning: Deleting /data folder will require device re-onboarding"
  read -p "Do you want to delete data folder at $HOME/sandbox/docker-compose/data? (y/n): " delete_data

  if [[ "$delete_data" =~ ^[Yy]$ ]]; then
    echo "Deleting data folder..."
    if rm -rf "$HOME/sandbox/docker-compose/data"; then
      echo '✅ Data folder deleted successfully'
      echo 'ℹ️ Device re-onboarding will be required'
    else
      echo '❌ Failed to delete data folder'
    fi
  else
    echo 'ℹ️ Data folder preserved'
  fi
}

build_start_device_agent_k3s_service() {
    cd "$HOME/sandbox"
    echo "Deploying workload-fleet-management-client on Kubernetes..."
    
    # Step 1: Pull image from GHCR (no local build)
    echo "Checking GHCR image: ${workload_Fleet_Management_Client_IMAGE_REF}"
    if docker manifest inspect "${workload_Fleet_Management_Client_IMAGE_REF}" >/dev/null 2>&1; then
        echo "Image exists in GHCR"
    else
        echo "❌ Image does NOT exist in GHCR: ${workload_Fleet_Management_Client_IMAGE_REF}"																 
        return 1															  
    fi

    echo "⬇️ Pulling image from GHCR..."
    docker pull "${workload_Fleet_Management_Client_IMAGE_REF}"
    echo "✅ Image pulled: ${workload_Fleet_Management_Client_IMAGE_REF}"

    # Step 2: Import into k3s container runtime
    echo "Saving and importing GHCR image into k3s..."
    docker save -o workload-fleet-management-client.tar "${workload_Fleet_Management_Client_IMAGE_REF}"

    if command -v k3s >/dev/null 2>&1; then
        k3s ctr -n k8s.io image import workload-fleet-management-client.tar
        echo "✅ Image imported into k3s cluster"
    elif command -v ctr >/dev/null 2>&1; then
        ctr -n k8s.io image import workload-fleet-management-client.tar
        echo "✅ Image imported into CTR runtime"
    else
        echo "❌ Neither k3s nor ctr command found"
        return 1
    fi
    rm -f workload-fleet-management-client.tar

    # Step 3: Navigate to helmchart directory
    cd helmchart
    if [ $? -ne 0 ]; then
      echo "❌ Failed to navigate to helmchart directory"
      return 1
    fi

    # Step 4: Copy config files
    update_agent_sbi_url

    echo "Copying configuration files..."
    mkdir -p config
    cp -r ../poc/device/agent/config/* ./config

    if [ $? -eq 0 ]; then
      echo "✅ Configuration files copied successfully"
    else
      echo "❌ Failed to copy configuration files"
      return 1
    fi
    enable_kubernetes_runtime

    # Step 5: Create secrets
    if [ -d "$HOME/certs" ] && [ -f "$HOME/certs/device-private.key" ] && [ -f "$HOME/certs/device-public.crt" ] && [ -f "$HOME/certs/device-ecdsa.crt" ] && [ -f "$HOME/certs/device-ecdsa.key" ] && [ -f "$HOME/certs/ca-cert.pem" ]; then
        echo "Creating TLS secrets..."
        kubectl delete secret workload-fleet-management-client-certs --namespace=default 2>/dev/null || true
        kubectl create secret generic workload-fleet-management-client-certs \
            --from-file=device-private.key="$HOME/certs/device-private.key" \
            --from-file=device-public.crt="$HOME/certs/device-public.crt" \
            --from-file=device-ecdsa.key="$HOME/certs/device-ecdsa.key" \
            --from-file=device-ecdsa.crt="$HOME/certs/device-ecdsa.crt" \
            --from-file=ca-cert.pem="$HOME/certs/ca-cert.pem" \
            --namespace=default

        if [ $? -eq 0 ]; then
            echo "✅ TLS secrets created successfully"
        else
            echo "❌ Failed to create TLS secrets"
            return 1
        fi
    else
        echo "❌ device-start-failed: Required certificates missing in $HOME/certs (ca-cert.pem)"
        return 1
    fi
    # Step 6: Clean up old resources
    echo "Cleaning up any existing resources..."
    kubectl delete clusterrole workload-fleet-management-client-role 2>/dev/null || true
    kubectl delete clusterrolebinding workload-fleet-management-client-binding 2>/dev/null || true

    helm uninstall workload-fleet-management-client -n default 2>/dev/null || true
    sleep 5

    # Step 7: Install with Helm
    echo "Installing workload-fleet-management-client with persistent storage..."
    
    helm install workload-fleet-management-client . \
        --set serviceAccount.create=true \
        --set secrets.create=false \
        --set secrets.existingSecret=workload-fleet-management-client-certs \
        --set persistence.enabled=true \
        --set persistence.size=1Gi \
        --debug \
        --wait

    if [ $? -ne 0 ]; then
        echo "❌ Helm installation failed"
        return 1
    fi
    echo "✅ Helm installation successful with persistent storage"
    # Step 8: Verify deployment
    echo "🔍 Verifying deployment..."
							 
    if kubectl auth can-i create secrets --as=system:serviceaccount:default:workload-fleet-management-client-sa -n default | grep -q "yes"; then
      echo "✅ RBAC permissions verified"
    else
      echo "⚠️ RBAC permissions may need manual verification"
    fi

    if kubectl get pvc -n default | grep -q "workload-fleet-management-client-data"; then
      echo "✅ PVC created successfully"
      kubectl get pvc -n default | grep workload-fleet-management-client
    else
      echo "⚠️ PVC not found"
      kubectl get events -n default --sort-by='.lastTimestamp' | tail -10
    fi
    echo "✅ Device-workload-fleet-management-client deployed successfully"		  
    # Show deployment status
    echo ""
    echo "Deployment Summary:"
    kubectl get pods -n default | grep workload-fleet-management-client
    kubectl get serviceaccount -n default | grep workload-fleet-management-client
    kubectl get pvc -n default | grep workload-fleet-management-client
}
																	  

stop_device_agent_kubernetes() {
  echo "Stopping workload-fleet-management-client..."
  cd "$HOME/sandbox"

  # Ask user about PVC deletion FIRST (before Helm uninstall)
  DELETE_PVC=false
  read -p "Delete persistent data (PVC)? This will require re-onboarding. [y/N]: " -n 1 -r
  echo
  if [[ $REPLY =~ ^[Yy]$ ]]; then
    DELETE_PVC=true
    echo "⚠️ PVC will be deleted after uninstalling Helm release"
  else
    echo "ℹ️ Attempting to preserve PVC..."

    # Add Helm annotation to prevent PVC deletion during uninstall
    if kubectl get pvc workload-fleet-management-client-data -n default >/dev/null 2>&1; then
      kubectl annotate pvc workload-fleet-management-client-data -n default \
        "helm.sh/resource-policy=keep" \
        --overwrite 2>/dev/null && echo "✅ PVC annotated for preservation" || echo "⚠️ Could not annotate PVC"
    else
      echo "⚠️ PVC not found, nothing to preserve"
    fi
  fi

  # Check if Helm release exists and uninstall
  if helm list -A | grep -q "workload-fleet-management-client"; then
    echo "Uninstalling workload-fleet-management-client Helm release..."
    helm uninstall workload-fleet-management-client --namespace default

    if [ $? -eq 0 ]; then
      echo "✅ Device-workload-fleet-management-client Helm release uninstalled successfully"
    else
      echo "❌ Failed to uninstall Helm release"
      return 1
    fi
  else
    echo "No workload-fleet-management-client Helm release found, trying direct kubectl deletion..."
    kubectl delete deployment workload-fleet-management-client-deploy -n default 2>/dev/null || echo "No deployment found"
  fi

  # Clean up ServiceAccount and RBAC resources
  echo "Cleaning up ServiceAccount and RBAC resources..."
  kubectl delete serviceaccount workload-fleet-management-client-sa -n default 2>/dev/null || echo "No serviceaccount found"
  kubectl delete clusterrole workload-fleet-management-client-role 2>/dev/null || echo "No clusterrole found"
  kubectl delete clusterrolebinding workload-fleet-management-client-binding 2>/dev/null || echo "No clusterrolebinding found"

  # Clean up ConfigMaps and Secrets
  echo "Cleaning up configmaps and secrets..."
  kubectl delete configmap workload-fleet-management-client-cm -n default 2>/dev/null || echo "No configmap found"
  kubectl delete secret workload-fleet-management-client-certs -n default 2>/dev/null || echo "No secret found"

  # NOW handle PVC based on user choice
  if [ "$DELETE_PVC" = true ]; then
    echo "Deleting PVC as requested..."
    kubectl delete pvc workload-fleet-management-client-data -n default 2>/dev/null || echo "No PVC found"
    echo "✅ PVC deleted - device will re-onboard on next start"
  else
    # Verify PVC was preserved
    if kubectl get pvc workload-fleet-management-client-data -n default >/dev/null 2>&1; then
      echo "✅ PVC preserved successfully - device will resume with existing ID on next start"
      kubectl get pvc workload-fleet-management-client-data -n default

      # Remove the keep annotation for next deployment
      kubectl annotate pvc workload-fleet-management-client-data -n default \
        "helm.sh/resource-policy-" \
        2>/dev/null || true
    else
      echo "⚠️ PVC was not preserved (may have been deleted by Helm)"
      echo "   Ensure PVC template has 'helm.sh/resource-policy: keep' annotation"
    fi
  fi

  # Verify cleanup
  echo ""
  echo "Verifying cleanup..."
  if kubectl get pods -n default 2>/dev/null | grep -q "workload-fleet-management-client"; then
    echo "⚠️ Some workload-fleet-management-client pods may still be terminating"
    kubectl get pods -n default | grep workload-fleet-management-client
  else
    echo "✅ All workload-fleet-management-client pods stopped"
  fi

  # Show remaining resources
  echo ""
  echo "Remaining workload-fleet-management-client resources:"
  kubectl get all,pvc,sa,cm,secrets -n default 2>/dev/null | grep workload-fleet-management-client || echo "✅ No workload-fleet-management-client resources found (except possibly PVC if preserved)"

  # Check for remaining RBAC resources
  echo ""
  echo "Remaining RBAC resources:"
  kubectl get clusterroles,clusterrolebindings 2>/dev/null | grep workload-fleet-management-client || echo "✅ No workload-fleet-management-client RBAC resources found"

  echo ""
  echo "✅ Device-workload-fleet-management-client cleanup complete"
}



cleanup_device_agent() {
  echo "Cleaning up workload-fleet-management-client files..."

  # Check if workload-fleet-management-client container exists and remove it
  if docker ps -a --format "{{.Names}}" | grep -q "^workload-fleet-management-client$"; then
    echo "Stopping and removing workload-fleet-management-client container..."
    docker stop workload-fleet-management-client 2>/dev/null || true
    docker rm workload-fleet-management-client 2>/dev/null || true
    echo "Removed workload-fleet-management-client container"
  else
    echo "No workload-fleet-management-client container found"
  fi

  #If using Helm deployment, uninstall the release
  if helm list --short 2>/dev/null | grep -q "^workload-fleet-management-client$"; then
    echo "Uninstalling workload-fleet-management-client Helm release..."
    helm uninstall workload-fleet-management-client 2>/dev/null || true
    echo "Removed workload-fleet-management-client Helm release"
  else
    echo "No workload-fleet-management-client Helm release found"
  fi


}

add_container_registry_mirror_to_k3s() {
  echo "Configuring container registry mirror for k3s..."

  # ---------------------------------------------------
  # Load registry settings from environment variables
  # ---------------------------------------------------
  registry_url="${REGISTRY_URL:-http://${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}}"
  registry_user="${REGISTRY_USER:-admin}"
  registry_password="${REGISTRY_PASS:-Harbor12345}"

  echo "Using registry mirror: $registry_url"
  echo "Using registry credentials: $registry_user / ******"
  # ---------------------------------------------------
  # Create k3s directory if needed
  # ---------------------------------------------------
  sudo mkdir -p /var/lib/rancher/k3s
  sudo mkdir -p /etc/rancher/k3s

  # Backup existing registries if present
  if [ -f /var/lib/rancher/k3s/registries.yml ]; then
    sudo cp /var/lib/rancher/k3s/registries.yml /var/lib/rancher/k3s/registries.yml.backup.$(date +%s)
    echo "✅ Backed up /var/lib/rancher/k3s/registries.yml"
  fi

  # ---------------------------------------------------
  # Write the registry config
  # ---------------------------------------------------
  cat <<EOF | sudo tee /var/lib/rancher/k3s/registries.yml >/dev/null
mirrors:
  "${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}":
    endpoint:
      - "${registry_url}"

configs:
  "${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}":
    auth:
      username: "${registry_user}"
      password: "${registry_password}"
    tls:
      insecure_skip_verify: true
EOF

  sudo cp /var/lib/rancher/k3s/registries.yml /var/lib/rancher/k3s/registries.yaml
  sudo cp /var/lib/rancher/k3s/registries.yml /etc/rancher/k3s/registries.yml
  sudo cp /var/lib/rancher/k3s/registries.yml /etc/rancher/k3s/registries.yaml

  echo "✅ Created k3s registry mirror configuration"
  # ---------------------------------------------------

  # Restart k3s
  # ---------------------------------------------------
  echo "Restarting k3s..."
  if sudo systemctl restart k3s; then
    echo "✅ k3s restarted successfully"
  else
    echo "❌ Failed to restart k3s"
    return 1
  fi

  # Wait for k3s active
  echo "Waiting for k3s to come up..."
  for i in {1..30}; do
    if sudo systemctl is-active --quiet k3s; then
      echo "✅ k3s is running"
      break
    fi

    sleep 2

  done

  echo "Checking cluster..."
  if sudo k3s kubectl get nodes >/dev/null 2>&1; then
    echo "✅ k3s cluster is responding"
  else
    echo "⚠️ k3s cluster not ready yet"


  fi

  echo "✅ Registry mirror configuration completed."
}

# ----------------------------
# Main Orchestration Functions
# ----------------------------
install_prerequisites() {
  echo "Installing prerequisites: k3s and others ..."
  validate_pre_required_vars
  install_go
  install_vim
  #install_and_enable_ssh
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

show_status() {
  echo "Device's Workload Fleet Management Client Status:"
  echo "==================="

  # Check Docker first
  if docker ps --format "{{.Names}}" | grep -q "^workload-fleet-management-client$"; then
    echo "✅ Device's Workload Fleet Management Client Docker Container is running."

    # Show container details
    echo "Container Details:"
    docker ps --filter "name=workload-fleet-management-client" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}\t{{.Image}}"

    return 0
  fi

  # Check Kubernetes if Docker is not running (check workload-fleet-management-client namespace)
  if kubectl get pods -n default --no-headers 2>/dev/null | grep -q "workload-fleet-management-client"; then
  echo "✅ Device's Workload Fleet Management Client Kubernetes Pod is running."

  # Show pod details
  echo "Pod Details:"
  kubectl get pods -n default -o wide | grep -E "(NAME|workload-fleet-management-client)"

  # Add ServiceAccount verification
  echo "ServiceAccount Details:"
  kubectl get serviceaccount -n default | grep workload-fleet-management-client || echo "No ServiceAccount found"

  return 0
  fi

  # If neither is running
  echo "❌ Device's Workload Fleet Management Client is not running on Docker or Kubernetes."
  echo "Available workload-fleet-management-client containers:"
  docker ps --format "table {{.Names}}\t{{.Status}}" | grep -E "(NAMES|workload-fleet-management-client)" || echo "No workload-fleet-management-client containers found"


  if command -v kubectl >/dev/null 2>&1; then
    echo ""
    echo "Available pods in workload-fleet-management-client namespace:"
    kubectl get pods -n default --no-headers 2>/dev/null | head -5 || echo "No workload-fleet-management-client namespace or pods found"


  fi
}




function install_promtail() {
  echo "📦 Installing Promtail to push logs to Loki at $WFM_HOST..."

  cat <<EOF > promtail-values.yaml
config:
  server:
    http_listen_port: 9080
    grpc_listen_port: 0

  positions:
    filename: /tmp/positions.yaml

  clients:
    - url: http://${WFM_HOST}:32100/loki/api/v1/push

  scrape_configs:
    - job_name: pod-logs
      static_configs:
        - targets:
            - localhost
          labels:
            job: podlogs
            __path__: /var/log/pods/*/*/*.log
EOF

  helm repo add grafana https://grafana.github.io/helm-charts
  helm repo update

  helm install $PROMTAIL_RELEASE grafana/promtail --version 6.17.1 -f promtail-values.yaml --namespace $NAMESPACE_OBSERVABILITY

  echo "✅ Promtail installed and configured to push logs to Loki"
}

function install_otel_collector() {
  echo "📡 Installing OTEL Collector to send metrics and traces to WFM node..."

  cat <<EOF > otel-values.yaml
mode: deployment
image:
  repository: otel/opentelemetry-collector-contrib

extraEnvs:
  - name: KUBE_NODE_NAME
    valueFrom:
      fieldRef:
        fieldPath: spec.nodeName

config:
  receivers:
    otlp:
      protocols:
        http:
          endpoint: 0.0.0.0:4318
        grpc:
          endpoint: 0.0.0.0:4317

    hostmetrics:
      collection_interval: 30s
      scrapers:
        cpu:
        memory:
        disk:
        filesystem:
        load:
        network:
        processes:
        paging:

    kubeletstats:
      collection_interval: 30s
      auth_type: "serviceAccount"
      endpoint: "https://\${KUBE_NODE_NAME}:10250"
      insecure_skip_verify: true
      metric_groups:
        - container
        - pod
        - node

  exporters:
    # Send traces to Jaeger
    otlp:
      endpoint: ${WFM_HOST}:30417
      tls:
        insecure: true

    # CHANGED: Push metrics to Prometheus Remote Write
    prometheusremotewrite:
      endpoint: http://${WFM_HOST}:30909/api/v1/write
      tls:
        insecure: true
      resource_to_telemetry_conversion:
        enabled: true

    debug:
      verbosity: detailed

  processors:
    batch: {}

  service:
    pipelines:
      traces:
        receivers: [otlp]
        processors: [batch]
        exporters: [otlp, debug]

      # CHANGED: Push metrics instead of exposing endpoint
      metrics:
        receivers: [otlp, hostmetrics, kubeletstats]
        processors: [batch]
        exporters: [prometheusremotewrite, debug]
EOF

  helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
  helm repo update

  helm install $OTEL_RELEASE open-telemetry/opentelemetry-collector --version 0.140.0 -f otel-values.yaml --namespace $NAMESPACE_OBSERVABILITY

  echo "✅ OTEL Collector setup complete!"
  echo "🔍 Traces sent to: ${WFM_HOST}:30417"
  echo "📊 Metrics pushed to: ${WFM_HOST}:30909/api/v1/write"
}


# Function to create observability namespace
create_observability_namespace() {
    echo "🔧 Checking observability namespace..."

    if sudo kubectl get namespace $NAMESPACE_OBSERVABILITY >/dev/null 2>&1; then
        echo "✅ Namespace '$NAMESPACE_OBSERVABILITY' already exists"
    else
        echo "🔧 Creating namespace '$NAMESPACE_OBSERVABILITY'..."
        sudo kubectl create namespace $NAMESPACE_OBSERVABILITY
        echo "✅ Namespace '$NAMESPACE_OBSERVABILITY' created successfully"
    fi
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

install_otel_collector_promtail_docker() {
  echo "Installing OTEL Collector v0.140.0 and Promtail v2.9.10 as Docker containers..."
  cd "$HOME/sandbox/pipeline/observability" || { echo '❌ observability dir missing'; exit 1; }

  # Get docker group GID for proper permissions
  DOCKER_GID=$(getent group docker | cut -d: -f3)
  echo "Docker group GID: $DOCKER_GID"

  # Create docker-compose.yml for observability stack
  cat <<EOF > docker-compose-observability.yml
version: '3.8'

services:
  promtail:
    image: grafana/promtail:2.9.10
    container_name: promtail
    volumes:
      - /var/log:/var/log:ro
      - /var/lib/docker/containers:/var/lib/docker/containers:ro
      - ./promtail-config.yml:/etc/promtail/config.yml
    command: -config.file=/etc/promtail/config.yml
    restart: unless-stopped
    network_mode: host

  otel-collector:
    image: otel/opentelemetry-collector-contrib:0.140.0
    container_name: otel-collector
    user: "0:${DOCKER_GID}"
    volumes:
      - ./otel-collector-config.yml:/etc/otel/config.yml
      - /var/run/docker.sock:/var/run/docker.sock
    command: --config=/etc/otel/config.yml
    ports:
      - "4317:4317"   # OTLP gRPC
      - "4318:4318"   # OTLP HTTP
    restart: unless-stopped
    network_mode: host
    environment:
      - HOST_IP=\${HOST_IP:-127.0.0.1}
EOF

  # Create Promtail config
  cat <<EOF > promtail-config.yml
server:
  http_listen_port: 9080
  grpc_listen_port: 0

positions:
  filename: /tmp/positions.yaml

clients:
  - url: http://${WFM_HOST}:32100/loki/api/v1/push

scrape_configs:
  - job_name: docker-logs
    static_configs:
      - targets:
          - localhost
        labels:
          job: dockerlogs
          __path__: /var/lib/docker/containers/*/*.log
EOF

  # Create OTEL Collector config with Prometheus Remote Write
  cat <<EOF > otel-collector-config.yml
receivers:
  # OTLP receiver for application traces/metrics
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4318
      grpc:
        endpoint: 0.0.0.0:4317

  # Host-level metrics
  hostmetrics:
    collection_interval: 30s
    scrapers:
      cpu:
      memory:
      disk:
      filesystem:
      load:
      network:
      processes:
      paging:

  # Docker container metrics
  docker_stats:
    endpoint: unix:///var/run/docker.sock
    collection_interval: 10s
    timeout: 5s
    api_version: "1.44"

exporters:
  # Send traces to Jaeger on WFM server
  otlp/jaeger:
    endpoint: ${WFM_HOST}:30417
    tls:
      insecure: true

  # CHANGED: Push metrics to Prometheus Remote Write
  prometheusremotewrite:
    endpoint: http://${WFM_HOST}:30909/api/v1/write
    tls:
      insecure: true
    resource_to_telemetry_conversion:
      enabled: true

  # Debug output
  debug:
    verbosity: detailed

processors:
  batch:
    timeout: 10s
    send_batch_size: 1024

  # Add resource attributes
  resource:
    attributes:
      - key: device.type
        value: docker
        action: insert
      - key: device.ip
        value: \${HOST_IP}
        action: insert

service:
  pipelines:
    # Traces pipeline - send to Jaeger
    traces:
      receivers: [otlp]
      processors: [batch, resource]
      exporters: [otlp/jaeger, debug]

    # CHANGED: Metrics pipeline - push to Prometheus Remote Write
    metrics:
      receivers: [otlp, hostmetrics, docker_stats]
      processors: [batch, resource]
      exporters: [prometheusremotewrite, debug]
EOF

  # Get host IP for resource attributes
  HOST_IP=$(hostname -I | awk '{print $1}')
  export HOST_IP

  # Start the observability stack
  docker compose -f docker-compose-observability.yml up -d

  # Create & enable systemd unit to start this stack on reboot
  create_observability_systemd_service

  echo "✅ OTEL Collector v0.140.0 and Promtail v2.9.10 installed"
  echo "📡 OTLP gRPC: localhost:4317"
  echo "📡 OTLP HTTP: localhost:4318"
  echo "🔍 Traces sent to Jaeger at: ${WFM_HOST}:30417"
  echo "📊 Metrics pushed to Prometheus at: ${WFM_HOST}:30909/api/v1/write"
  echo "📝 Logs sent to Loki at: ${WFM_HOST}:32100"
}



install_otel_collector_promtail_wrapper() {
  if [ "$DEVICE_TYPE" = "k3s" ]; then
    install_otel_collector_promtail  # Existing k8s-based installation
  else
    install_otel_collector_promtail_docker  # New Docker-based installation
  fi
}

uninstall_otel_collector_promtail_wrapper() {
  if [ "$DEVICE_TYPE" = "k3s" ]; then
    uninstall_otel_collector_promtail  # Existing k8s-based uninstallation
  else
    uninstall_otel_collector_promtail_docker  # New Docker-based uninstallation
  fi
}

uninstall_otel_collector_promtail_docker() {
  echo "🧹 Uninstalling Promtail and OTEL Collector containers..."
  cd "$HOME/sandbox/pipeline/observability" || { echo '❌ observability dir missing'; exit 1; }

  if [ -f "docker-compose-observability.yml" ]; then
    docker compose -f docker-compose-observability.yml down
    rm -f docker-compose-observability.yml promtail-config.yml otel-collector-config.yml
  fi

  echo "✅ Cleanup complete."
}


install_otel_collector_promtail() {
  echo "Installing OTEL Collector and Promtail..."
  cd "$HOME/sandbox/pipeline/observability" || { echo '❌ observability dir missing'; exit 1; }
  create_observability_namespace
  install_promtail
  install_otel_collector
  echo "✅ OTEL Collector and Promtail installation completed."
}

uninstall_otel_collector_promtail() {
  echo "🧹 Uninstalling Promtail and OTEL Collector..."
  cd "$HOME/sandbox/pipeline/observability" || { echo '❌ observability dir missing'; exit 1; }

    # Uninstall helm releases only if they exist
    for release in $PROMTAIL_RELEASE $OTEL_RELEASE; do
        if helm status $release -n "$NAMESPACE_OBSERVABILITY" >/dev/null 2>&1; then
            echo "🗑️ Uninstalling $release..."
            helm uninstall $release --namespace "$NAMESPACE_OBSERVABILITY"
        else
            echo "⏭️ $release not found, skipping..."
        fi
    done


  rm -f promtail-values.yaml otel-values.yaml
  echo "✅ Cleanup complete."

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
  echo "3) Workload-Fleet-Management-Client-Start(docker-compose-device)"
  echo "4) Workload-Fleet-Management-Client-Stop(docker-compose-device)"
  echo "5) Workload-Fleet-Management-Client-Start(k3s-device)"
  echo "6) Workload-Fleet-Management-Client-Stop(k3s-device)"
  echo "7) Workload-Fleet-Management-Client-Status"
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
