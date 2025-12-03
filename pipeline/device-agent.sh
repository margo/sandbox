#!/bin/bash
set -e
export PATH="$PATH:/usr/local/go/bin"
# ----------------------------
# Environment & Validation Functions
# ----------------------------

#--- Github Settings to pull the code (can be overridden via env)
GITHUB_USER="${GITHUB_USER:-}"  # Set via env or leave empty
GITHUB_TOKEN="${GITHUB_TOKEN:-}"  # Set via env or leave empty

#--- harbor settings (can be overridden via env)
EXPOSED_HARBOR_IP="${EXPOSED_HARBOR_IP:-127.0.0.1}"
EXPOSED_HARBOR_PORT="${EXPOSED_HARBOR_PORT:-8081}"

#--- branch details (can be overridden via env)
DEV_REPO_BRANCH="${DEV_REPO_BRANCH:-dev-sprint-6}"
WFM_IP="${WFM_IP:-127.0.0.1}"
WFM_PORT="${WFM_PORT:-8082}"

#--- Registry settings (can be overridden via env)
REGISTRY_URL="${REGISTRY_URL:-http://${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}}"
REGISTRY_USER="${REGISTRY_USER:-admin}"
REGISTRY_PASS="${REGISTRY_PASS:-Harbor12345}"

# variables for observability stack
NAMESPACE_OBSERVABILITY="observability"
PROMTAIL_RELEASE="promtail"
OTEL_RELEASE="otel-collector"

validate_pre_required_vars() {
  local required_vars=("GITHUB_USER" "GITHUB_TOKEN" "DEV_REPO_BRANCH" "WFM_IP" "WFM_PORT")
  for var in "${required_vars[@]}"; do
    if [ -z "${!var}" ]; then
      echo "Error: Required environment variable $var is not set"
      exit 1
    fi
  done
}

validate_start_required_vars() {
  local required_vars=("WFM_IP" "WFM_PORT")
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
  # apt update && apt install curl -y
  sudo apt update -y
  sudo apt install -y curl git
  echo "Installation complete: curl and git are installed."

  INSTALL_HELM_V3_15_1=true
  HELM_VERSION="3.15.1"
  HELM_TAR="helm-v${HELM_VERSION}-linux-amd64.tar.gz"
  HELM_BIN_DIR="/usr/local/bin"

  apt update && apt install -y curl dos2unix build-essential gcc libc6-dev
  install_helm
}

install_docker_compose_v2() {

  if ! command -v docker >/dev/null 2>&1; then
    echo 'Docker not found. Installing Docker...'
    apt-get remove -y docker docker-engine docker.io containerd runc || true
    curl -fsSL "https://get.docker.com" -o get-docker.sh; sh get-docker.sh
    usermod -aG docker $USER
  else
    echo 'Docker already installed.'
  fi

  echo "Installing Docker Compose V2 plugin..."

  # Correct path for get.docker.com installation
  sudo mkdir -p /usr/libexec/docker/cli-plugins

# Get latest Tag
COMPOSE_VERSION=$(curl -s https://api.github.com/repos/docker/compose/releases/latest | grep tag_name | cut -d'"' -f4)
echo "Using Docker Compose version: $COMPOSE_VERSION"

sudo curl -L \
  "https://github.com/docker/compose/releases/download/${COMPOSE_VERSION}/docker-compose-Linux-x86_64" \
  -o /usr/libexec/docker/cli-plugins/docker-compose

					  
sudo chmod +x /usr/libexec/docker/cli-plugins/docker-compose

					   
  docker compose version
  echo "✅ Docker Compose V2 installed successfully"
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

configure_helm_registries() {
  echo "Configuring Helm registry authentication..."
  
  # Login to ghcr.io if GitHub credentials are available
  if [ -n "$GITHUB_USER" ] && [ -n "$GITHUB_TOKEN" ]; then
    echo "Authenticating Helm with GitHub Container Registry..."
    echo "$GITHUB_TOKEN" | helm registry login ghcr.io -u "$GITHUB_USER" --password-stdin
    
    if [ $? -eq 0 ]; then
      echo "✅ Helm authenticated with ghcr.io"
    else
      echo "⚠️ Failed to authenticate with ghcr.io (continuing anyway)"
    fi
  else
    echo "⚠️ GitHub credentials not set, skipping ghcr.io authentication"
    echo "   Set GITHUB_USER and GITHUB_TOKEN to enable external registry access"
  fi
  
  # Verify Helm config was created
  if [ -f "$HOME/.config/helm/registry/config.json" ]; then
    echo "✅ Helm registry config created at $HOME/.config/helm/registry/config.json"
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
  echo "Cloning dev-repo on ($VM2_HOST)..."
  cd $HOME
  sudo rm -rf dev-repo
  git clone "https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/margo/dev-repo.git"
  cd dev-repo
  git checkout ${DEV_REPO_BRANCH}
  cd ..
}

# ----------------------------
# Configuration Functions
# ----------------------------
update_agent_sbi_url() {
  echo 'Updating wfm.sbiUrl in agent config ...'
  sed -i "s|sbiUrl:.*|sbiUrl: https://$WFM_IP:$WFM_PORT/v1alpha2/margo|" "$HOME/dev-repo/poc/device/agent/config/config.yaml"
}

# ----------------------------
# K3s Installation Functions
# ----------------------------
check_k3s_installed() {
  if command -v k3s >/dev/null 2>&1; then
    echo 'k3s already installed, skipping installation.'
    k3s --version
    return 0
  else
    return 1
  fi
}

install_k3s_dependencies() {
  echo 'Installing k3s dependencies...'
  sudo apt update && sudo apt upgrade -y
  sudo apt install -y curl
}

install_k3s() {
  if ! check_k3s_installed; then
    echo 'Installing k3s...'
    install_k3s_dependencies
    curl -sfL https://get.k3s.io | sh -
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
# Device Agent Runtime Configuration update based on Docker or K8s
#-----------------------------------------------------------------

enable_kubernetes_runtime() {
  CONFIG_FILE="$HOME/dev-repo/helmchart/config/config.yaml"
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
  CONFIG_FILE="$HOME/dev-repo/docker-compose/config/config.yaml"
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
# Device Agent Build Functions
# ----------------------------
build_device_agent_docker() {
  cd "$HOME/dev-repo"
  echo 'Checking if device-agent image already exists...'

# Check if the image exists
  if docker images -q margo.org/device-agent:latest | grep -q .; then
    echo "device-agent image already exists. Skipping build."
  else
    echo 'Building device-agent...'
    docker build -f poc/device/agent/Dockerfile . -t margo.org/device-agent:latest
  fi
  echo 'device-agent image build complete.'     
}


# ----------------------------
# Device Agent Service Functions
# ----------------------------

start_device_agent_docker_service() {
  echo 'Starting device-agent...'
  cd "$HOME/dev-repo/docker-compose"
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

  mkdir -p data
  enable_docker_runtime
  docker compose up -d
}


stop_device_agent_service_docker() {
  echo "Stopping device-agent..."
  cd "$HOME/dev-repo/docker-compose"
  docker compose down
  
  # Prompt user to delete /data folder
  echo ""
  echo "⚠️  Warning: Deleting /data folder will require device re-onboarding"
  read -p "Do you want to delete data folder at $HOME/dev-repo/docker-compose/data? (y/n): " delete_data
  
  if [[ "$delete_data" =~ ^[Yy]$ ]]; then
    echo "Deleting data folder..."
    if rm -rf "$HOME/dev-repo/docker-compose/data"; then
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
    cd "$HOME/dev-repo"
    echo "Building and deploying device-agent on Kubernetes..."
    
    # Step 1: Build the Docker image if it doesn't exist
    echo "Checking if device-agent image exists..."
    if ! docker images | grep -q "margo.org/device-agent"; then
      echo "Building device-agent Docker image..."
      docker build -f poc/device/agent/Dockerfile . -t margo.org/device-agent:latest
      if [ $? -ne 0 ]; then
        echo "❌ Failed to build device-agent image"
        return 1
      fi
      echo "✅ Device-agent image built successfully"
    else
      echo "✅ Device-agent image already exists"
    fi
    
    # Step 2: Save and import image to k3s
    echo "Importing image to k3s cluster..."
    docker save -o device-agent.tar margo.org/device-agent:latest
    
						   
    if command -v k3s >/dev/null 2>&1; then
      k3s ctr -n k8s.io image import device-agent.tar
      echo "✅ Image imported to k3s cluster"
    elif command -v ctr >/dev/null 2>&1; then
      ctr -n k8s.io image import device-agent.tar
      echo "✅ Image imported to k3s cluster"
    else
      echo "❌ Neither k3s nor ctr command found"
      return 1
    fi
    
					   
    rm -f device-agent.tar
    
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
   							 
        kubectl delete secret device-agent-certs --namespace=default 2>/dev/null || true
        
        kubectl create secret generic device-agent-certs \
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

    # Step 5.1: Create Helm registry config secret (For ghcr.io access)
    if [ -f "$HOME/.config/helm/registry/config.json" ]; then
        echo "Creating Helm registry config secret..."
        kubectl delete secret helm-registry-config --namespace=default 2>/dev/null || true
        
        kubectl create secret generic helm-registry-config \
            --from-file=config.json="$HOME/.config/helm/registry/config.json" \
            --namespace=default
        
        echo "✅ Helm registry config secret created"
    fi
    
														
    # Step 6: Clean up old resources 
    echo "Cleaning up any existing resources..."
    kubectl delete clusterrole device-agent-role 2>/dev/null || true
    kubectl delete clusterrolebinding device-agent-binding 2>/dev/null || true
    
    helm uninstall device-agent -n default 2>/dev/null || true
    
											
    sleep 5

    # Step 7: Install with Helm 
    echo "Installing device-agent with persistent storage..."
    helm install device-agent . \
        --set serviceAccount.create=true \
        --set secrets.create=false \
        --set secrets.existingSecret=device-agent-certs \
        --set persistence.enabled=true \
        --set persistence.size=1Gi \
        --debug \
        --wait

    if [ $? -ne 0 ]; then
																	   
		
        echo "❌ Helm installation failed"
        return 1
    fi
    
    echo "✅ Helm installation successful with persistent storage"
    
    # Step 8: Verify deployment (NO PATCHING NEEDED!)
    echo "🔍 Verifying deployment..."
    
    # Verify RBAC permissions 
    if kubectl auth can-i create secrets --as=system:serviceaccount:default:device-agent-sa -n default | grep -q "yes"; then
      echo "✅ RBAC permissions verified"
    else
      echo "⚠️ RBAC permissions may need manual verification"
    fi
    
    # Verify PVC (FIXED NAME)
												 
    if kubectl get pvc -n default | grep -q "device-agent-data"; then
      echo "✅ Persistent volume claim created successfully"
      kubectl get pvc -n default | grep device-agent
    else
      echo "⚠️ PVC not found, checking for errors..."
      kubectl get events -n default --sort-by='.lastTimestamp' | tail -10
    fi
    
    echo "✅ Device-agent deployed successfully"
    
    # Show deployment status
    echo ""
    echo "Deployment Summary:"
    kubectl get pods -n default | grep device-agent
    kubectl get serviceaccount -n default | grep device-agent
    kubectl get pvc -n default | grep device-agent
	
}

stop_device_agent_kubernetes() {
  echo "Stopping device-agent..."
  cd "$HOME/dev-repo"

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
    if kubectl get pvc device-agent-data -n default >/dev/null 2>&1; then
      kubectl annotate pvc device-agent-data -n default \
        "helm.sh/resource-policy=keep" \
        --overwrite 2>/dev/null && echo "✅ PVC annotated for preservation" || echo "⚠️ Could not annotate PVC"
    else
      echo "⚠️ PVC not found, nothing to preserve"
    fi
  fi
  
  # Check if Helm release exists and uninstall
  if helm list -A | grep -q "device-agent"; then
    echo "Uninstalling device-agent Helm release..."
    helm uninstall device-agent --namespace default
    
    if [ $? -eq 0 ]; then
      echo "✅ Device-agent Helm release uninstalled successfully"
    else
      echo "❌ Failed to uninstall Helm release"
      return 1
    fi
  else
    echo "No device-agent Helm release found, trying direct kubectl deletion..."
    kubectl delete deployment device-agent-deploy -n default 2>/dev/null || echo "No deployment found"
  fi
  
  # Clean up ServiceAccount and RBAC resources
  echo "Cleaning up ServiceAccount and RBAC resources..."
  kubectl delete serviceaccount device-agent-sa -n default 2>/dev/null || echo "No serviceaccount found"
  kubectl delete clusterrole device-agent-role 2>/dev/null || echo "No clusterrole found"
  kubectl delete clusterrolebinding device-agent-binding 2>/dev/null || echo "No clusterrolebinding found"
  
  # Clean up ConfigMaps and Secrets
  echo "Cleaning up configmaps and secrets..."
  kubectl delete configmap device-agent-cm -n default 2>/dev/null || echo "No configmap found"
  kubectl delete secret device-agent-certs -n default 2>/dev/null || echo "No secret found"
  
  # NOW handle PVC based on user choice
  if [ "$DELETE_PVC" = true ]; then
    echo "Deleting PVC as requested..."
    kubectl delete pvc device-agent-data -n default 2>/dev/null || echo "No PVC found"
    echo "✅ PVC deleted - device will re-onboard on next start"
  else
    # Verify PVC was preserved
    if kubectl get pvc device-agent-data -n default >/dev/null 2>&1; then
      echo "✅ PVC preserved successfully - device will resume with existing ID on next start"
      kubectl get pvc device-agent-data -n default
      
      # Remove the keep annotation for next deployment
      kubectl annotate pvc device-agent-data -n default \
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
  if kubectl get pods -n default 2>/dev/null | grep -q "device-agent"; then
    echo "⚠️ Some device-agent pods may still be terminating"
    kubectl get pods -n default | grep device-agent
  else
    echo "✅ All device-agent pods stopped"
  fi
  
  # Show remaining resources
  echo ""
  echo "Remaining device-agent resources:"
  kubectl get all,pvc,sa,cm,secrets -n default 2>/dev/null | grep device-agent || echo "✅ No device-agent resources found (except possibly PVC if preserved)"
  
  # Check for remaining RBAC resources
  echo ""
  echo "Remaining RBAC resources:"
  kubectl get clusterroles,clusterrolebindings 2>/dev/null | grep device-agent || echo "✅ No device-agent RBAC resources found"
  
  echo ""
  echo "✅ Device-agent cleanup complete"
}



cleanup_device_agent() {
  echo "Cleaning up device-agent files..."
  
  # Check if device-agent container exists and remove it
  if docker ps -a --format "{{.Names}}" | grep -q "^device-agent$"; then
    echo "Stopping and removing device-agent container..."
    docker stop device-agent 2>/dev/null || true
    docker rm device-agent 2>/dev/null || true
    echo "Removed device-agent container"
  else
    echo "No device-agent container found"
  fi
 
  #If using Helm deployment, uninstall the release
  if helm list --short 2>/dev/null | grep -q "^device-agent$"; then
    echo "Uninstalling device-agent Helm release..."
    helm uninstall device-agent 2>/dev/null || true
    echo "Removed device-agent Helm release"
  else
    echo "No device-agent Helm release found"
  fi


}

add_container_registry_mirror_to_k3s() {
  echo "Configuring container registry mirror for k3s..."

  # ---------------------------------------------------
  # Load registry settings from environment variables
  # ---------------------------------------------------
  registry_url="${REGISTRY_URL:-http://${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}}"
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
  "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}":
    endpoint:
      - "${registry_url}"

configs:
  "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}":
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
  install_and_enable_ssh
  install_basic_utilities
  install_docker_compose_v2 
  clone_dev_repo
  setup_k3s
  add_container_registry_mirror_to_k3s
  configure_helm_registries
  echo 'prerequisites installation completed.'
}


start_device_agent_docker() {
  echo "Building and starting device-agent ..."
  validate_start_required_vars
  update_agent_sbi_url
  build_device_agent_docker
  start_device_agent_docker_service
   echo 'device-agent-docker-container started'
}

start_device_agent_kubernetes() {
  echo "Building and starting device-agent with ServiceAccount authentication..."
  validate_start_required_vars
  build_start_device_agent_k3s_service
  echo '✅ device-agent-pod started with ServiceAccount authentication'
}

stop_device_agent_docker() {
  echo "Stopping device-agent on VM2 ($VM2_HOST)..."
  stop_device_agent_service_docker
  echo "Device Agent stopped"
}


uninstall_prerequisites() {
  cleanup_device_agent
}

show_status() {
  echo "Device Agent Status:"
  echo "==================="
  
  # Check Docker first
  if docker ps --format "{{.Names}}" | grep -q "^device-agent$"; then
    echo "✅ Device Agent Docker Container is running."
    
    # Show container details
    echo "Container Details:"
    docker ps --filter "name=device-agent" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}\t{{.Image}}"
    
    return 0
  fi
  
  # Check Kubernetes if Docker is not running (check device-agent namespace)
  if kubectl get pods -n default --no-headers 2>/dev/null | grep -q "device-agent"; then
  echo "✅ Device Agent Kubernetes Pod is running."
  
  # Show pod details
  echo "Pod Details:"
  kubectl get pods -n default -o wide | grep -E "(NAME|device-agent)"
  
  # Add ServiceAccount verification
  echo "ServiceAccount Details:"
  kubectl get serviceaccount -n default | grep device-agent || echo "No ServiceAccount found"
  
  return 0
  fi
  
  # If neither is running
  echo "❌ Device Agent is not running on Docker or Kubernetes."
  echo "Available device-agent containers:"
  docker ps --format "table {{.Names}}\t{{.Status}}" | grep -E "(NAMES|device-agent)" || echo "No device-agent containers found"

  
  if command -v kubectl >/dev/null 2>&1; then
    echo ""
    echo "Available pods in device-agent namespace:"
    kubectl get pods -n default --no-headers 2>/dev/null | head -5 || echo "No device-agent namespace or pods found"
    
    
  fi
}




function install_promtail() {
  echo "📦 Installing Promtail to push logs to Loki at $WFM_IP..."

  cat <<EOF > promtail-values.yaml
config:
  server:
    http_listen_port: 9080
    grpc_listen_port: 0

  positions:
    filename: /tmp/positions.yaml

  clients:
    - url: http://${WFM_IP}:32100/loki/api/v1/push

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

  helm install $PROMTAIL_RELEASE grafana/promtail -f promtail-values.yaml --namespace $NAMESPACE_OBSERVABILITY

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
    otlp:
      endpoint: ${WFM_IP}:30417
      tls:
        insecure: true

    prometheus:
      endpoint: "0.0.0.0:8899"

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

      metrics:
        receivers: [otlp, hostmetrics, kubeletstats]
        processors: [batch]
        exporters: [prometheus, debug]
EOF

  helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
  helm repo update

  helm install $OTEL_RELEASE open-telemetry/opentelemetry-collector -f otel-values.yaml --namespace $NAMESPACE_OBSERVABILITY

  echo "🔧 Patching OTEL Collector service to expose Prometheus metrics on NodePort 30999..."
  sudo kubectl patch svc otel-collector-opentelemetry-collector \
    -n $NAMESPACE_OBSERVABILITY \
    --type='json' \
    -p='[
      {
        "op": "add",
        "path": "/spec/ports/-",
        "value": {
          "name": "prometheus-metrics",
          "port": 8899,
          "protocol": "TCP",
          "targetPort": 8899,
          "nodePort": 30999
        }
      },
      {
        "op": "replace",
        "path": "/spec/type",
        "value": "NodePort"
      }
    ]'

  echo "✅ OTEL Collector installed and Prometheus metrics exposed at NodePort 30999"
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


install_otel_collector_promtail() {
  echo "Installing OTEL Collector and Promtail..."
  cd "$HOME/dev-repo/pipeline/observability" || { echo '❌ observability dir missing'; exit 1; }
  create_observability_namespace
  install_promtail
  install_otel_collector
  echo "✅ OTEL Collector and Promtail installation completed."
}

uninstall_otel_collector_promtail() {
  echo "🧹 Uninstalling Promtail and OTEL Collector..."
  cd "$HOME/dev-repo/pipeline/observability" || { echo '❌ observability dir missing'; exit 1; }
   
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
  rm -rf "$HOME/dev-repo"
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


show_menu() {
  echo "Choose an option:"
  echo "1) Install-prerequisites"
  echo "2) Uninstall-prerequisites"
  echo "3) Device-agent-Start(docker-compose-device)"
  echo "4) Device-agent-Stop(docker-compose-device)"
  echo "5) Device-agent-Start(k3s-device)"
  echo "6) Device-agent-Stop(k3s-device)"
  echo "7) Device-agent-Status"
  echo "8) OTEL-collector-promtail-installation"
  echo "9) OTEL-collector-promtail-uninstallation"
  echo "10) cleanup-residual"
  echo "11) create_device_rsa_certs"
  echo "12) create_device_ecdsa_certs"
  read -rp "Enter choice [1-12]: " choice
  case $choice in
    1) install_prerequisites;;
    2) uninstall_prerequisites;;
    3) start_device_agent_docker ;;
    4) stop_device_agent_docker ;;
    5) start_device_agent_kubernetes ;;
    6) stop_device_agent_kubernetes ;;
    7) show_status ;;
    8) install_otel_collector_promtail ;;
    9) uninstall_otel_collector_promtail ;;
    10) cleanup_residual;;
    11) create_device_rsa_certs ;;
    12) create_device_ecdsa_certs ;;
    *) echo "Invalid choice" ;;
  esac
}

# ----------------------------
# Main Script Execution
# ----------------------------
if [ -z "$1" ]; then
  show_menu
fi