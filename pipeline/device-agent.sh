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
    echo 'Docker not found. Installing Docker...';
    apt-get remove -y docker docker-engine docker.io containerd runc || true;
    curl -fsSL "https://get.docker.com" -o get-docker.sh; sh get-docker.sh;
    usermod -aG docker $USER;
  else
    echo 'Docker already installed.';
  fi;
    
  echo "Installing Docker Compose V2 plugin..."
  
  # Create the plugins directory
  sudo mkdir -p /usr/local/lib/docker/cli-plugins
  
  # Download the latest Docker Compose V2
  COMPOSE_VERSION=$(curl -s https://api.github.com/repos/docker/compose/releases/latest | grep 'tag_name' | cut -d\" -f4)
  echo "Downloading Docker Compose ${COMPOSE_VERSION}..."
  
  sudo curl -L "https://github.com/docker/compose/releases/download/${COMPOSE_VERSION}/docker-compose-$(uname -s)-$(uname -m)" \
    -o /usr/local/lib/docker/cli-plugins/docker-compose
  
  # Make it executable
  sudo chmod +x /usr/local/lib/docker/cli-plugins/docker-compose
  
  # Verify installation
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
  sed -i "s|sbiUrl:.*|sbiUrl: http://$WFM_IP:$WFM_PORT/v1alpha2/margo/sbi/v1|" "$HOME/dev-repo/poc/device/agent/config/config.yaml"
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
#-----------------------------------------------------------------
# Device Agent Runtime Configuration update based on Docker or K8s
#-----------------------------------------------------------------

enable_kubernetes_runtime() {
  CONFIG_FILE="$HOME/dev-repo/helmchart/config.yaml"
  echo "Enabling Kubernetes section in config.yaml for ServiceAccount authentication..."
  sed -i \
  -e 's/^[[:space:]]*#\s*-\s*type:\s*KUBERNETES/- type: KUBERNETES/' \
  -e 's/^[[:space:]]*#\s*kubernetes:/  kubernetes:/' \
  -e 's/^[[:space:]]*-\s*type:\s*DOCKER/  # - type: DOCKER/' \
  -e 's/^[[:space:]]*docker:/  # docker:/' \
  -e 's/^[[:space:]]*url:/  # url:/' \
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
  cp -r ../poc/device/agent/config/* ./config/
  mkdir -p data
  enable_docker_runtime
  docker compose up -d
   
}


stop_device_agent_service_docker() {
  echo "Stopping device-agent..."
  cd "$HOME/dev-repo/docker-compose"
  docker compose down
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
    
    # Import to k3s cluster
    if command -v k3s >/dev/null 2>&1; then
      k3s ctr -n k8s.io image import device-agent.tar
      echo "✅ Image imported to k3s cluster"
    elif command -v ctr >/dev/null 2>&1; then
      ctr -n k8s.io image import device-agent.tar
      echo "✅ Image imported to k8s cluster"
    else
      echo "❌ Neither k3s nor ctr command found"
      return 1
    fi
    
    # Clean up tar file
    rm -f device-agent.tar
    
    # Step 3: Navigate to helmchart directory
    cd helmchart
    if [ $? -ne 0 ]; then
      echo "❌ Failed to navigate to helmchart directory"
      return 1
    fi
    
    # Step 4: Create namespace
    echo "Creating device-agent namespace..."
    kubectl create namespace device-agent 2>/dev/null || echo "Namespace device-agent already exists"
    
    # Step 5: Copy config files (ServiceAccount approach - no kubeconfig secret needed)
    echo "Copying configuration files..."
    cp -r ../poc/device/agent/config/* .
    if [ $? -eq 0 ]; then
      echo "✅ Configuration files copied successfully"
    else
      echo "❌ Failed to copy configuration files"
      return 1
    fi
    
    # Step 6: Update config.yaml with environment variables
    update_agent_sbi_url
    enable_kubernetes_runtime
    
    # Step 7: Install/upgrade Helm chart (ServiceAccount approach)
    
      echo "Installing new device-agent deployment..."
      helm install device-agent . 
    
    
    #STEP 8: Fix RBAC permissions
    if [ $? -eq 0 ]; then
      echo "🔧 Applying RBAC permissions fix..."
      
      # Force update ClusterRole with secrets permissions
      kubectl patch clusterrole device-agent-device-agent-role --type='json' -p='[
        {
          "op": "add",
          "path": "/rules/-",
          "value": {
            "apiGroups": [""],
            "resources": ["secrets", "configmaps"],
            "verbs": ["create", "get", "list", "update", "patch", "delete"]
          }
        },
        {
          "op": "replace",
          "path": "/rules/1/verbs",
          "value": ["get", "list", "watch", "create", "update", "patch", "delete"]
        },
        {
          "op": "replace",
          "path": "/rules/0/verbs", 
          "value": ["get", "list", "watch", "create", "update", "patch", "delete"]
        }
      ]' 2>/dev/null || echo "⚠️ ClusterRole patch failed, trying alternative method..."
      
      # Alternative: Create namespace-scoped RoleBinding for secrets
      kubectl create rolebinding device-agent-secrets-access \
        --clusterrole=admin \
        --serviceaccount=device-agent:device-agent-device-agent-sa \
        --namespace=device-agent 2>/dev/null || echo "RoleBinding already exists"
      
      # Verify permissions
      echo "🔍 Verifying RBAC permissions..."
      if kubectl auth can-i create secrets --as=system:serviceaccount:device-agent:device-agent-device-agent-sa -n default | grep -q "yes"; then
        echo "✅ RBAC permissions applied successfully"
      else
        echo "⚠️ RBAC permissions may need manual verification"
      fi
      
      echo "✅ Device-agent deployed successfully on Kubernetes with ServiceAccount"
      
      # Verify deployment
      echo "Verifying deployment..."
      kubectl get pods -n default
      kubectl get serviceaccount -n default
      
    else
      echo "❌ Failed to deploy device-agent"
      return 1
    fi
}

stop_device_agent_kubernetes() {
  echo "Stopping device-agent..."
  cd "$HOME/dev-repo"
  
  # Check if Helm release exists
  if helm list -A| grep -q "device-agent"; then
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
    kubectl delete deployment device-agent-device-agent-deploy -n default  2>/dev/null || echo "No deployment found"
  fi
  
  # Clean up ServiceAccount and RBAC resources
  echo "Cleaning up ServiceAccount and RBAC resources..."
  kubectl delete serviceaccount device-agent-device-agent-sa -n default 2>/dev/null || echo "No serviceaccount found"
  kubectl delete clusterrole device-agent-device-agent-role 2>/dev/null || echo "No clusterrole found"
  kubectl delete clusterrolebinding device-agent-device-agent-binding 2>/dev/null || echo "No clusterrolebinding found"
  
  # Clean up ConfigMaps
  echo "Cleaning up configmaps..."
  kubectl delete configmap device-agent-device-agent-cm -n default 2>/dev/null || echo "No configmap found"
  
  # Remove all remaining resources in namespace
  kubectl delete secrets --all -n default 2>/dev/null || echo "No secrets to delete"
  kubectl delete configmaps --all -n default 2>/dev/null || echo "No additional configmaps to delete"
  
  # Verify cleanup
  echo "Verifying cleanup..."
  if kubectl get pods -n default 2>/dev/null | grep -q "device-agent"; then
    echo "⚠️ Some device-agent pods may still be terminating"
    kubectl get pods -n default
  else
    echo "✅ Device-agent stopped successfully"
  fi
  
  # Show remaining resources in namespace
  echo "Checking for remaining resources..."
  kubectl get all,secrets,configmaps,serviceaccounts -n default 2>/dev/null || echo "Namespace may not exist or is empty"
  
  # Check for remaining RBAC resources
  echo "Checking for remaining RBAC resources..."
  kubectl get clusterroles | grep device-agent || echo "No device-agent clusterroles found"
  kubectl get clusterrolebindings | grep device-agent || echo "No device-agent clusterrolebindings found"
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
  
  # Ask for container registry URL or default to https://registry-1.docker.io
  read -p "Enter container registry URL [https://registry-1.docker.io]: " registry_url
  registry_url=${registry_url:-"https://registry-1.docker.io"}
  
  # Ask for registry username, no default
  read -p "Enter registry username: " registry_user
  if [ -z "$registry_user" ]; then
    echo "❌ Registry username is required"
    return 1
  fi
  
  # Ask for registry password, no default (hidden input)
  read -s -p "Enter registry password: " registry_password
  echo  # New line after hidden input
  if [ -z "$registry_password" ]; then
    echo "❌ Registry password is required"
    return 1
  fi
  
  # Create k3s directory if it doesn't exist
  sudo mkdir -p /var/lib/rancher/k3s
  
  # Backup existing registries.yml if it exists
  if [ -f /var/lib/rancher/k3s/registries.yml ]; then
    sudo cp /var/lib/rancher/k3s/registries.yml /var/lib/rancher/k3s/registries.yml.backup.$(date +%s)
    echo "✅ Backed up existing registries.yml"
  fi
  
  # Add docker registry mirror and credentials in /var/lib/rancher/k3s
  cat <<EOF | sudo tee /var/lib/rancher/k3s/registries.yml
mirrors:
  "$EXPOSED_HARBOR_IP:$EXPOSED_HARBOR_PORT":
    endpoint:
      - "$registry_url"

configs:
  "$EXPOSED_HARBOR_IP:$EXPOSED_HARBOR_PORT":
    auth:
      username: "$registry_user"
      password: "$registry_password"
    tls:
    insecure_skip_verify: true
EOF

  cat <<EOF | sudo tee /var/lib/rancher/k3s/registries.yaml
mirrors:
  "$EXPOSED_HARBOR_IP:$EXPOSED_HARBOR_PORT":
    endpoint:
      - "$registry_url"

configs:
  "$EXPOSED_HARBOR_IP:$EXPOSED_HARBOR_PORT":
    auth:
      username: "$registry_user"
      password: "$registry_password"
    tls:
    insecure_skip_verify: true
EOF

# Add docker registry mirror and credentials in /etc/rancher/k3s
cat <<EOF | sudo tee /etc/rancher/k3s/registries.yaml
mirrors:
  "$EXPOSED_HARBOR_IP:$EXPOSED_HARBOR_PORT":
    endpoint:
      - "$registry_url"

configs:
  "$EXPOSED_HARBOR_IP:$EXPOSED_HARBOR_PORT":
    auth:
      username: "$registry_user"
      password: "$registry_password"
EOF

cat <<EOF | sudo tee /etc/rancher/k3s/registries.yml
mirrors:
  "$EXPOSED_HARBOR_IP:$EXPOSED_HARBOR_PORT":
    endpoint:
      - "$registry_url"

configs:
  "$EXPOSED_HARBOR_IP:$EXPOSED_HARBOR_PORT":
    auth:
      username: "$registry_user"
      password: "$registry_password"
EOF

  echo "✅ Created k3s registries configuration"
  
  # Restart k3s to apply changes
  echo "Restarting k3s to apply registry changes..."
  if sudo systemctl restart k3s; then
    echo "✅ k3s restarted successfully"
    
    # Wait for k3s to be ready
    echo "Waiting for k3s to be ready..."
    for i in {1..30}; do
      if sudo systemctl is-active --quiet k3s; then
        echo "✅ k3s is active and running"
        break
      else
        echo "Waiting for k3s... ($i/30)"
        sleep 2
      fi
    done
    
    # Verify k3s is working
    if sudo k3s kubectl get nodes >/dev/null 2>&1; then
      echo "✅ k3s cluster is responding"
    else
      echo "⚠️ k3s cluster may not be fully ready yet"
    fi
  else
    echo "❌ Failed to restart k3s"
    return 1
  fi
  
  echo "✅ Container registry mirror configuration completed"
}



# ----------------------------
# Main Orchestration Functions
# ----------------------------
install_prerequisites() {
  echo "Installing prerequisites: k3s and others ..."
  validate_pre_required_vars
  install_go
  install_basic_utilities
  install_docker_compose_v2 
  clone_dev_repo
  setup_k3s
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
  mkdir -p "$CERT_DIR"
  cd "$CERT_DIR" || exit 1

  echo "Generating RSA device certs..."
  # Generate RSA private key (2048-bit)
  openssl genrsa -out device-private.key 2048

  # Generate self-signed certificate
  openssl req -new -x509 -key device-private.key -out device-public.crt -days 365 \
    -subj "/C=IN/ST=GGN/L=Sector 48/O=Margo/CN=margo-device"
  echo "✅ RSA Cert generation has been completed."

  #echo "Copying RSA Certs in dev-repo"
  #cp device-private.key device-public.crt "$HOME/dev-repo/docker-compose/config/"
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

  #echo "Copying ECDSA Certs in dev-repo"
  #cp device-ecdsa.key device-ecdsa.crt "$HOME/dev-repo/docker-compose/config/"
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
  echo "8) otel-collector-promtail-installation"
  echo "9) otel-collector-promtail-uninstallation"
  echo "10) add-container-registry-mirror-to-k3s"
  echo "11) cleanup-residual"
  echo "12) create_device_rsa_certs"
  echo "13) create_device_ecdsa_certs"
  read -rp "Enter choice [1-13]: " choice
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
    10) add_container_registry_mirror_to_k3s;;
    11) cleanup_residual;;
    12) create_device_rsa_certs ;;
    13) create_device_ecdsa_certs ;;
    *) echo "Invalid choice" ;;
  esac
}

# ----------------------------
# Main Script Execution
# ----------------------------
if [ -z "$1" ]; then
  show_menu
fi
