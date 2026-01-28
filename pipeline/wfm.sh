#!/bin/bash
set -e

# ----------------------------
# Load environment file
# ----------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
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

# ----------------------------
# Environment & Validation
# ----------------------------

#--- Github Settings to pull the code (can be overridden via env)
GITHUB_USER="${GITHUB_USER:-}"
GITHUB_TOKEN="${GITHUB_TOKEN:-}"

#--- branch details (can be overridden via env)
SYMPHONY_BRANCH="${SYMPHONY_BRANCH:-main}"
SANDBOX_REPO_BRANCH="${SANDBOX_REPO_BRANCH:-main}"

#--- harbor settings (can be overridden via env)
EXPOSED_HARBOR_IP="${EXPOSED_HARBOR_IP:-127.0.0.1}"
EXPOSED_HARBOR_PORT="${EXPOSED_HARBOR_PORT:-8081}"

#--- symphony settings (can be overridden via env)
EXPOSED_SYMPHONY_IP="${EXPOSED_SYMPHONY_IP:-127.0.0.1}"
EXPOSED_SYMPHONY_PORT="${EXPOSED_SYMPHONY_PORT:-8082}"

#--- device node IPs (can be overridden via env) for prometheus to scrape metrics
# Format: "IP1:PORT1,IP2:PORT2" or just "IP1,IP2" (defaults to port 30999 for k3s)
DEVICE_NODE_IPS="${DEVICE_NODE_IPS:-127.0.0.1:30999}"

#--- Registry settings (can be overridden via env)
REGISTRY_URL="${REGISTRY_URL:-http://${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}}"
REGISTRY_USER="${REGISTRY_USER:-admin}"
REGISTRY_PASS="${REGISTRY_PASS:-Harbor12345}"

# OCI Registry organization/namespace
OCI_ORGANIZATION="${OCI_ORGANIZATION:-library}"

# variables for observability stack
NAMESPACE_OBSERVABILITY="observability"
JAEGER_RELEASE="jaeger"
PROM_RELEASE="prometheus"
GRAFANA_RELEASE="grafana"
LOKI_RELEASE="loki"

# Directory to generate and store ssl certs for symphony API
CERT_DIR="$HOME/symphony/api/certificates"

# Stable version as of December 2024
K3S_VERSION="${K3S_VERSION:-v1.31.4+k3s1}"
# ----------------------------
# Utility Functions
# ----------------------------

info() {
    echo "ℹ️  $1"
}

success() {
    echo "✅ $1"
}

pause() {
  echo
  read -rp "Press Enter to continue..." _
}

# ----------------------------
# GHCR Image References
# ----------------------------
GHCR_REGISTRY="ghcr.io"
GHCR_ORG="margo"
SYMPHONY_IMAGE="margo-symphony-api"
SYMPHONY_TAG="latest"
SYMPHONY_IMAGE_REF="${GHCR_REGISTRY}/${GHCR_ORG}/${SYMPHONY_IMAGE}:${SYMPHONY_TAG}"

# ----------------------------
# Installation Functions
# ----------------------------
install_basic_utilities() {
  INSTALL_HELM_V3_15_1=true
  HELM_VERSION="3.15.1"
  HELM_TAR="helm-v${HELM_VERSION}-linux-amd64.tar.gz"
  HELM_BIN_DIR="/usr/local/bin"

  PACKAGES="curl dos2unix build-essential gcc libc6-dev jq"

  echo "🔄 Installing Basic utilities..."
  INSTALLATION_NEEDED="false"
  for pkg in $PACKAGES ; do
    if ! dpkg -s $pkg >/dev/null ; then
      INSTALLATION_NEEDED="true"
      break
    fi
  done
  if [[ "${INSTALLATION_NEEDED}" == "true" ]] ; then
    sudo apt update && sudo apt install -y curl dos2unix build-essential gcc libc6-dev jq
    echo "✅ Basic utilities installed"
  else
    echo "⚡️ Basic utilities already installed, skipping installation"
  fi

  install_helm
}

install_redis() {
  local REDIS_VERSION="7.0.15"  # Stable version

  echo "🔄 Installing Redis ${REDIS_VERSION}..."

  if command -v redis-server >/dev/null 2>&1; then
    REDIS_VERSION="$(redis-server --version | cut -d ' ' -f 3 | cut -d '=' -f 2)"
    echo "⚡️ Redis ${REDIS_VERSION} already installed, skipping installation"
  else
    sudo apt update
    sudo apt install -y redis-server=${REDIS_VERSION}-* || \
    sudo apt install -y redis-server  # Fallback to latest if specific version unavailable

    sudo systemctl enable redis-server
    sudo systemctl start redis-server

    REDIS_VERSION="$(redis-server --version)"
    echo "✅ Redis ${REDIS_VERSION} installed"
  fi
}


# Helm install/uninstall
install_helm() {
  cd $HOME
  if [ "${INSTALL_HELM_V3_15_1}" == "true" ]; then
    echo "🔄 Installing Helm..."
    if command -v helm >/dev/null 2>&1 && [[ "$(helm version --short | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')" == "${HELM_VERSION}" ]]; then
        echo "⚡️ Helm version ${HELM_VERSION} already installed, skipping installation"
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
        echo '✅ Basic utilities installed'
    fi
  fi
}

install_go() {
  cd $HOME
  echo "🔄 Installing Go..."
  if [[ "$PATH" != *"/usr/local/go/bin"* ]] ; then
    export PATH=$PATH:/usr/local/go/bin
  fi
  if command -v go >/dev/null 2>&1; then
    GO_VERSION="$(go version | cut -d ' ' -f 3 | cut -c 3-)"
    echo "⚡️ Go ${GO_VERSION} already installed, skipping installation"
  else
    sudo rm -rf /usr/local/go /usr/bin/go
    wget "https://go.dev/dl/go1.24.4.linux-amd64.tar.gz" -O go.tar.gz
    sudo tar -C /usr/local -xzf go.tar.gz
    rm go.tar.gz
    which go
    go version
    echo "✅ Go installed"
  fi
}

install_docker_and_compose() {
  cd $HOME

  # Define pinned versions
  local DOCKER_VERSION="29.1.2"
  local DOCKER_COMPOSE_VERSION="5.0.0"
  local UBUNTU_CODENAME=$(lsb_release -cs)  # Gets 'noble' for Ubuntu 24.04

  # Install Docker if not present or wrong version
  echo "🔄 Installing Docker..."
  if command -v docker >/dev/null 2>&1; then
    CURRENT_DOCKER_VERSION=$(docker version --format '{{.Server.Version}}')
    echo "⚡️ Docker ${CURRENT_DOCKER_VERSION} already installed, skipping installation"

    # Check if correct version is installed
    if [ "$CURRENT_DOCKER_VERSION" != "$DOCKER_VERSION" ]; then
      echo "⚠️  Current Docker version: $CURRENT_DOCKER_VERSION (expected: $DOCKER_VERSION)"
      echo "ℹ️  To upgrade/downgrade, run: apt-get install docker-ce=5:${DOCKER_VERSION}-1~ubuntu.24.04~${UBUNTU_CODENAME}"
    fi
  else
    echo "Docker not found. Installing Docker ${DOCKER_VERSION}..."

    # Remove old Docker packages
    sudo apt-get remove -y docker docker-engine docker.io containerd runc 2>/dev/null || true

    # Add Docker's official GPG key and repository
    sudo apt-get update
    sudo apt-get install -y ca-certificates curl
    sudo install -m 0755 -d /etc/apt/keyrings
    sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
    sudo chmod a+r /etc/apt/keyrings/docker.asc

    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
      ${UBUNTU_CODENAME} stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

    sudo apt-get update

    # Install specific Docker version
    sudo apt-get install -y \
      docker-ce=5:${DOCKER_VERSION}-1~ubuntu.24.04~${UBUNTU_CODENAME} \
      docker-ce-cli=5:${DOCKER_VERSION}-1~ubuntu.24.04~${UBUNTU_CODENAME} \
      containerd.io=1.7.27-1 \
      docker-buildx-plugin=0.23.0-1~ubuntu.24.04~${UBUNTU_CODENAME}

    # Add user to docker group
    sudo usermod -aG docker $USER

    echo "✅ Docker ${DOCKER_VERSION} installed"

    # Add user to docker group, if missing
    if ! groups | grep -w "docker" &>/dev/null ; then
        sudo usermod -aG docker $USER
        echo "🚨 User added to group 'docker'! You need to restart the installation from a new bash session to have it activatived."
        exit
    fi
  fi

  echo "🔄 Installing Docker Compose plugin ${DOCKER_COMPOSE_VERSION}..."
  # Install specific Docker Compose plugin version
  if dpkg -s docker-compose-plugin >/dev/null ; then
    CURRENT_COMPOSE_VERSION=$(docker compose version --short 2>/dev/null | sed 's/v//')
    echo "⚡️ Docker Compose ${CURRENT_COMPOSE_VERSION} already installed, skipping installation"

    # Check if correct version is installed
    if [ "$CURRENT_COMPOSE_VERSION" != "$DOCKER_COMPOSE_VERSION" ]; then
      echo "⚠️  Current Docker Compose version: v$CURRENT_COMPOSE_VERSION (expected: v$DOCKER_COMPOSE_VERSION)"
      echo "ℹ️  To upgrade/downgrade, run: apt-get install docker-compose-plugin=${DOCKER_COMPOSE_VERSION}-1~ubuntu.24.04~${UBUNTU_CODENAME}"
    fi
  else
    sudo apt-get update
    sudo apt-get install -y docker-compose-plugin=${DOCKER_COMPOSE_VERSION}-1~ubuntu.24.04~${UBUNTU_CODENAME}
  fi

  # CRITICAL: Remove old standalone docker-compose binaries to avoid conflicts
  echo 'Cleaning up old docker-compose binaries...'
  rm -f /usr/local/bin/docker-compose /usr/bin/docker-compose 2>/dev/null || true

  # Verify Docker version
  echo ""
  echo "Docker version:"
  docker version | grep -E "Version|API version" | head -4

  # Verify Docker Compose version
  echo ""
  echo "Docker Compose version:"
  docker compose version

  # Wait for Docker daemon to be active (max 30s)
  for i in $(seq 1 30); do
    if systemctl is-active --quiet docker; then
      echo '✅ Docker daemon is running.'
      break
    else
      echo "Waiting for Docker daemon to start... ($i/30)"
      sleep 1
    fi
  done

  # Final verification
  if docker compose version >/dev/null 2>&1; then
    echo "✅ Docker ${DOCKER_VERSION} and Docker Compose v${DOCKER_COMPOSE_VERSION} ready"
  else
    echo "❌ Docker Compose plugin not working properly"
    return 1
  fi

  # Prevent automatic updates (optional)
  echo "🔒 Holding Docker packages at current versions..."
  sudo apt-mark hold docker-ce docker-ce-cli docker-compose-plugin containerd.io docker-buildx-plugin
  echo "ℹ️  To allow updates later, run: apt-mark unhold docker-ce docker-ce-cli docker-compose-plugin"
}


install_oras() {
  echo "🔄 Installing ORAS CLI..."

  if command -v oras >/dev/null 2>&1; then
    ORAS_VERSION="$(oras version | head -n 1 | cut -d ':' -f 2 | sed 's/[[:space:]]*//')"
    echo "⚡️ ORAS ${ORAS_VERSION} already installed, skipping installation"
    return 0
  fi

  cd /tmp
  ORAS_VERSION="1.1.0"
  wget "https://github.com/oras-project/oras/releases/download/v${ORAS_VERSION}/oras_${ORAS_VERSION}_linux_amd64.tar.gz"
  tar -xzf "oras_${ORAS_VERSION}_linux_amd64.tar.gz"
  sudo mv oras /usr/local/bin/
  rm "oras_${ORAS_VERSION}_linux_amd64.tar.gz"

  ORAS_VERSION="$(oras version | head -n 1 | cut -d ':' -f 2 | sed 's/[[:space:]]*//')"
  echo "✅ ORAS ${ORAS_VERSION} installed"
}


# ----------------------------
# Repository Functions
# ----------------------------
clone_symphony_repo() {
  cd "$HOME"
  if ! test -d "$HOME/symphony/.git" ; then
    rm -rf "$HOME/symphony"
    echo "Cloning symphony branch: $SYMPHONY_BRANCH"
    if [[ -n "$GITHUB_USER" && -n "$GITHUB_TOKEN" ]]; then 
      git clone \
                --branch "$SYMPHONY_BRANCH" \
                --single-branch \
                --depth 1 \
                "https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/margo/symphony.git" \
                "$HOME/symphony"
    else
      git clone \
            --branch "$SYMPHONY_BRANCH" \
            --single-branch \
            --depth 1 \
            "https://github.com/margo/symphony.git" \
            "$HOME/symphony"
    fi
  fi
  cd "$HOME/symphony"
  # git fetch --depth 1 --update-head-ok origin ${SYMPHONY_BRANCH}:${SYMPHONY_BRANCH}
  # git checkout ${SYMPHONY_BRANCH} || echo 'Branch ${SYMPHONY_BRANCH} not found'
  echo "✅ symphony repo checkout to branch ${SYMPHONY_BRANCH} done"
}

clone_dev_repo() {
  cd "$HOME"
  if ! test -d "$HOME/sandbox/.git" ; then
    rm -rf "$HOME/sandbox"
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
  fi
  cd "$HOME/sandbox"
  # git fetch --depth 1 --update-head-ok origin ${SANDBOX_REPO_BRANCH}:${SANDBOX_REPO_BRANCH} || echo 'Unable to fetch ${SANDBOX_REPO_BRANCH}'
  # git checkout ${SANDBOX_REPO_BRANCH} || echo 'Branch ${SANDBOX_REPO_BRANCH} not found'
  echo "sandbox repo checkout to branch ${SANDBOX_REPO_BRANCH} done"
}

# ----------------------------
# Service Setup Functions
# ----------------------------

create_harbor_systemd_service() {
  echo "🔧 Creating systemd service for Harbor auto-start..."

  # Get the actual harbor directory path (not using $HOME variable)
  local harbor_dir="$HOME/sandbox/pipeline/harbor"

  # Create systemd service file with absolute path
  sudo tee /etc/systemd/system/harbor.service > /dev/null <<EOF
  [Unit]
  Description=Harbor Container Registry
  Requires=docker.service
  After=docker.service network-online.target
  Wants=network-online.target

  [Service]
  Type=oneshot
  RemainAfterExit=yes
  WorkingDirectory=${harbor_dir}
  ExecStartPre=/bin/sleep 10
  ExecStart=/usr/bin/docker compose up -d
  ExecStop=/usr/bin/docker compose down
  TimeoutStartSec=0

  [Install]
  WantedBy=multi-user.target
EOF

  # Reload systemd and enable the service
  sudo systemctl daemon-reload
  sudo systemctl enable harbor.service

  echo "✅ Harbor systemd service created and enabled"
  echo "📋 Service will start Harbor automatically on boot"
  echo "📁 Working directory: ${harbor_dir}"
}

configure_harbor_restart_policy() {
  local compose_file="$HOME/sandbox/pipeline/harbor/docker-compose.yml"

  if [ ! -f "$compose_file" ]; then
    echo "⚠️ docker-compose.yml not found, will be generated during install"
    return 0
  fi

  echo "🔧 Replacing restart policies in docker-compose.yml..."

  # Backup original file
  cp "$compose_file" "${compose_file}.backup.$(date +%s)"

  # Replace "restart: always" with "restart: unless-stopped"
  sed -i 's/^\s*restart:\s*always/    restart: unless-stopped/g' "$compose_file"

  echo "✅ Restart policies replaced with unless-stopped"

  # Verify the changes - should show only one restart per service
  echo "📋 Verifying restart policies in docker-compose.yml:"
  grep "restart:" "$compose_file"
}

setup_harbor() {
  if docker ps --format '{{.Names}}' | grep -q harbor; then
    echo 'Harbor is already running, skipping startup.'
  else
    cd "$HOME/sandbox/pipeline/harbor"

    # Update harbor.yml with EXPOSED_HARBOR_IP
    sed -i "s|^hostname: .*|hostname: $EXPOSED_HARBOR_IP|" harbor.yml
    
    echo 'Preparing Harbor configuration...'
    chmod +x prepare
    
    # Run prepare to generate docker-compose.yml
    sudo ./prepare

    # Add restart policies to docker-compose.yml BEFORE starting
    configure_harbor_restart_policy

    # Start Harbor - ensure clean state
    echo 'Starting Harbor with restart policies...'
    sudo docker compose down --remove-orphans 2>/dev/null || true
    sudo docker compose up -d

    # Force update restart policies on all containers
    echo '🔧 Applying restart policies to running containers...'
    sleep 5
    for container in nginx registry registryctl redis harbor-jobservice harbor-core harbor-db harbor-portal harbor-log; do
      if docker ps -a --format "{{.Names}}" | grep -q "^${container}$"; then
        docker update --restart=unless-stopped "$container" 2>/dev/null && echo "✅ Updated: $container"
      fi
    done

    echo 'Waiting for Harbor to initialize...'
    sleep 15

    docker ps

    # Verify all containers are running
    echo ""
    echo "📊 Harbor container status:"
    docker ps --filter "name=harbor" --format "table {{.Names}}\t{{.Status}}"

    # Verify restart policies
    echo ""
    echo "📋 Verifying restart policies:"
    for container in nginx registry registryctl redis $(docker ps -a --filter "name=harbor-" --format "{{.Names}}"); do
      if docker ps -a --format "{{.Names}}" | grep -q "^${container}$"; then
        docker inspect --format='{{.Name}}: {{.HostConfig.RestartPolicy.Name}}' "$container"
      fi
    done

    # Create systemd service for auto-start on boot
    create_harbor_systemd_service

    # Final health check
    echo ""
    echo "⏳ Waiting for all containers to be healthy (this may take 1-2 minutes)..."
    sleep 45

    healthy_count=$(docker ps --filter "name=harbor" --filter "health=healthy" --format "{{.Names}}" | wc -l)
    total_count=$(docker ps --filter "name=harbor" --format "{{.Names}}" | wc -l)

    echo "✅ Harbor status: $healthy_count/$total_count containers healthy"

    if [ "$healthy_count" -eq "$total_count" ] && [ "$total_count" -eq 9 ]; then
      echo "✅ All Harbor containers are running and healthy!"
    else
      echo "⚠️ Some containers may still be initializing. Check with: docker ps | grep harbor"
    fi
  fi
}


# ----------------------------
# OCI Application Package Push Functions (NEW - replaces Git push)
# ----------------------------

push_nextcloud_to_oci() {
  echo "📦 Pushing Nextcloud application package to OCI Registry..."

  local app_dir="$HOME/sandbox/poc/tests/artefacts/nextcloud-compose/margo-package"
  local repository="${OCI_ORGANIZATION}/nextcloud-compose-app-package"
  local tag="latest"

  cd "$app_dir" || { echo "❌ Nextcloud package dir missing"; return 1; }

  echo "$REGISTRY_PASS" | oras login "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}" \
    -u "$REGISTRY_USER" --password-stdin --plain-http

  if [ ! -f "margo.yaml" ]; then
    echo "❌ margo.yaml not found in $app_dir"
    return 1
  fi

  local files=("margo.yaml:application/vnd.margo.app.description.v1+yaml")

  if [ -d "resources" ] && [ "$(ls -A resources 2>/dev/null)" ]; then
    while IFS= read -r file; do
      if [ -f "$file" ]; then
        files+=("$file:application/octet-stream")
      fi
    done < <(find resources -type f 2>/dev/null)
  fi

  echo "Pushing files: ${files[@]}"
  oras push "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/${repository}:${tag}" \
    --artifact-type "application/vnd.margo.app.v1+json" \
    --plain-http \
    "${files[@]}"

  if [ $? -eq 0 ]; then
    echo "✅ Nextcloud package pushed to OCI Registry"
    echo "📍 Location: ${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/${repository}:${tag}"
  else
    echo "❌ Failed to push Nextcloud package"
    return 1
  fi
}


push_custom_otel_to_oci() {
  echo "📦 Pushing Custom OTEL application package to OCI Registry..."

  local app_dir="$HOME/sandbox/poc/tests/artefacts/custom-otel-helm-app/margo-package"
  local repository="${OCI_ORGANIZATION}/custom-otel-helm-app-package"
  local tag="latest"

  cd "$app_dir" || { echo "❌ Custom OTEL package dir missing"; return 1; }

  # Login to Harbor OCI Registry
  echo "$REGISTRY_PASS" | oras login "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}" \
    -u "$REGISTRY_USER" --password-stdin --plain-http

  # Check if margo.yaml exists
  if [ ! -f "margo.yaml" ]; then
    echo "❌ margo.yaml not found in $app_dir"
    return 1
  fi

  # Build file list for ORAS - start with margo.yaml
  local files=("margo.yaml:application/vnd.margo.app.description.v1+yaml")

  # Add resource files if directory exists and has files
  if [ -d "resources" ] && [ "$(ls -A resources 2>/dev/null)" ]; then
    while IFS= read -r file; do
      if [ -f "$file" ]; then
        files+=("$file:application/octet-stream")
      fi
    done < <(find resources -type f 2>/dev/null)
  fi

  # Push to OCI Registry with Margo-specific artifact type
  echo "Pushing files: ${files[@]}"
  oras push "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/${repository}:${tag}" \
    --artifact-type "application/vnd.margo.app.v1+json" \
    --plain-http \
    "${files[@]}"

  if [ $? -eq 0 ]; then
    echo "✅ Custom OTEL package pushed to OCI Registry"
    echo "📍 Location: ${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/${repository}:${tag}"
  else
    echo "❌ Failed to push Custom OTEL package"
    return 1
  fi
}



# ----------------------------
# Build Functions
# ----------------------------

build_maestro_cli() {
  CLI_DIR="$HOME/symphony/cli";
  if [ -d "$CLI_DIR" ]; then
    cd "$CLI_DIR";
    go mod tidy;
    go build -o maestro;
  fi
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

install_jaeger() {
  # Check if Jaeger Helm release exists and is healthy
  if helm status $JAEGER_RELEASE -n "$NAMESPACE_OBSERVABILITY" >/dev/null 2>&1; then
    echo "⚠️ Jaeger Helm release found, checking pod health..."

  fi

  echo "🔄 Refreshing Jaeger Helm repo..."
  helm repo remove jaegertracing || true
  helm repo add jaegertracing https://jaegertracing.github.io/helm-charts
  helm repo update

  echo "🚀 Installing Jaeger v3.4.1 with OTLP and NodePort configuration..."
  helm install $JAEGER_RELEASE jaegertracing/jaeger \
    --version 3.4.1 \
    --namespace $NAMESPACE_OBSERVABILITY \
    --set agent.enabled=false \
    --set collector.enabled=true \
    --set collector.otlp.enabled=true \
    --set collector.service.type=NodePort \
    --set collector.service.nodePort=30417 \
    --set collector.service.additionalPorts[0].name=otlp-grpc \
    --set collector.service.additionalPorts[0].port=4317 \
    --set collector.service.additionalPorts[0].protocol=TCP \
    --set query.enabled=true \
    --set query.service.type=NodePort \
    --set query.service.nodePort=32500

  echo "⏳ Waiting for Jaeger pods to initialize..."
  sleep 10

  echo "🛠 Patching Jaeger Collector Service for OTLP gRPC..."
  sudo kubectl patch svc ${JAEGER_RELEASE}-collector \
    -n $NAMESPACE_OBSERVABILITY \
    --type='json' \
    -p='[
      {
        "op": "add",
        "path": "/spec/ports/-",
        "value": {
          "name": "otlp-grpc",
          "port": 4317,
          "protocol": "TCP",
          "targetPort": 4317,
          "nodePort": 30417
        }
      }
    ]'

  echo "🛠 Patching Jaeger Collector Service for OTLP HTTP..."
  sudo kubectl patch svc ${JAEGER_RELEASE}-collector \
    -n $NAMESPACE_OBSERVABILITY \
    --type='json' \
    -p='[
      {
        "op": "add",
        "path": "/spec/ports/-",
        "value": {
          "name": "otlp-http",
          "port": 4318,
          "protocol": "TCP",
          "targetPort": 4318,
          "nodePort": 30418
        }
      }
    ]'

  echo "✅ Jaeger setup complete!"
  echo "🌐 Query UI: NodePort 32500"
  echo "📡 OTLP gRPC: Port 4317"
  echo "📡 OTLP HTTP: Port 4318"
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

install_prometheus() {
  cd "$HOME/sandbox/pipeline/observability"
  echo "📡 Setting up Prometheus to scrape metrics from multiple devices..."

  # Parse DEVICE_NODE_IPS and build targets array
  TARGETS_ARRAY="["
  if [ -n "$DEVICE_NODE_IPS" ]; then
    IFS=',' read -ra DEVICES <<< "$DEVICE_NODE_IPS"

    for i in "${!DEVICES[@]}"; do
      device=$(echo "${DEVICES[$i]}" | xargs)

      # Check if port is specified
      if [[ "$device" == *":"* ]]; then
        TARGET="'${device}'"
      else
        TARGET="'${device}:30999'"
      fi

      # Add comma separator except for last item
      if [ $i -eq $((${#DEVICES[@]} - 1)) ]; then
        TARGETS_ARRAY="${TARGETS_ARRAY}${TARGET}"
      else
        TARGETS_ARRAY="${TARGETS_ARRAY}${TARGET}, "
      fi
    done
  fi
  TARGETS_ARRAY="${TARGETS_ARRAY}]"

  cat <<EOF > prometheus-values.yaml
server:
  image:
    repository: prom/prometheus
    tag: latest
  service:
    type: NodePort
    nodePort: 30900
  persistentVolume:
    enabled: false
  serverFiles:
    prometheus.yml:
      global:
        scrape_interval: 5s
      scrape_configs:
        - job_name: 'otel-collector'
          static_configs:
            - targets: ${TARGETS_ARRAY}
EOF

  helm repo remove prometheus-community || true
  helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
  helm repo update

  helm install $PROM_RELEASE prometheus-community/prometheus \
    --version 27.49.0 \
    --namespace $NAMESPACE_OBSERVABILITY \
    -f prometheus-values.yaml

  echo "✅ Prometheus setup complete!"
  echo "📊 Prometheus UI: NodePort 30900"
  echo "📡 Monitoring devices: $DEVICE_NODE_IPS"

  patch_prometheus_configmap
}


patch_prometheus_configmap() {
  cd "$HOME/sandbox/pipeline/observability"
  echo "🛠 Applying Prometheus ConfigMap with device targets..."

  CM_SOURCE="collector-scrape-cm-change.txt"
  CM_TARGET="collector-scrape-cm-change.yaml"

  if [ ! -f "$CM_SOURCE" ]; then
    echo "❌ Source ConfigMap file '$CM_SOURCE' not found."
    exit 1
  fi

  # Build targets array from DEVICE_NODE_IPS
  TARGETS_ARRAY="["
  if [ -n "$DEVICE_NODE_IPS" ]; then
    IFS=',' read -ra DEVICES <<< "$DEVICE_NODE_IPS"

    for i in "${!DEVICES[@]}"; do
      device=$(echo "${DEVICES[$i]}" | xargs)

      # Check if port is specified
      if [[ "$device" == *":"* ]]; then
        TARGET="'${device}'"
      else
        TARGET="'${device}:30999'"
      fi

      # Add comma separator except for last item
      if [ $i -eq $((${#DEVICES[@]} - 1)) ]; then
        TARGETS_ARRAY="${TARGETS_ARRAY}${TARGET}"
      else
        TARGETS_ARRAY="${TARGETS_ARRAY}${TARGET}, "
      fi
    done
  fi
  TARGETS_ARRAY="${TARGETS_ARRAY}]"

  echo "📡 Device targets: $TARGETS_ARRAY"

  # Replace placeholder with targets array
  sed "s|__DEVICE_TARGETS__|${TARGETS_ARRAY}|g" "$CM_SOURCE" > "$CM_TARGET"

  echo "📄 Applying ConfigMap with force replace..."

  # Method 1: Force replace (recommended)
  sudo kubectl replace -f "$CM_TARGET" --force --namespace "$NAMESPACE_OBSERVABILITY" || {
    echo "⚠️ Force replace failed, trying server-side apply..."
    # Method 2: Server-side apply (handles conflicts better)
    sudo kubectl apply -f "$CM_TARGET" --server-side --force-conflicts --namespace "$NAMESPACE_OBSERVABILITY" || {
      echo "⚠️ Server-side apply failed, trying delete and recreate..."
      # Method 3: Delete and recreate as last resort
      sudo kubectl delete configmap prometheus-server -n "$NAMESPACE_OBSERVABILITY" --ignore-not-found=true
      sleep 3
      sudo kubectl apply -f "$CM_TARGET" --namespace "$NAMESPACE_OBSERVABILITY"
    }
  }

  echo "🔄 Restarting Prometheus pod to apply new config..."
  sudo kubectl rollout restart deployment prometheus-server -n "$NAMESPACE_OBSERVABILITY" || \
  sudo kubectl delete pod -l app=prometheus,component=server -n "$NAMESPACE_OBSERVABILITY" || \
  echo "⚠️ Pod restart may be needed manually."

  rm -f "$CM_TARGET"
  echo "✅ ConfigMap applied and temporary file removed."
}



install_loki() {
  echo "📦 Installing Loki for log aggregation..."

  cat <<EOF > loki-values.yaml
deploymentMode: SingleBinary
chunksCache:
  enabled: false
loki:
  auth_enabled: false
  commonConfig:
    replication_factor: 1
  limits_config:
    allow_structured_metadata: false
  storage:
    type: filesystem
  schemaConfig:
    configs:
      - from: 2020-10-24
        store: boltdb-shipper
        object_store: filesystem
        schema: v11
        index:
          prefix: index_
          period: 24h
  storage_config:
    boltdb_shipper:
      active_index_directory: /tmp/loki/index
      cache_location: /tmp/loki/cache
    filesystem:
      directory: /tmp/loki/chunks
singleBinary:
  replicas: 1
read:
  replicas: 0
write:
  replicas: 0
backend:
  replicas: 0
EOF

  helm install $LOKI_RELEASE grafana/loki --version 6.46.0 -f loki-values.yaml --namespace $NAMESPACE_OBSERVABILITY

  echo "🔧 Patching Loki service to expose via NodePort 32100..."
  sudo kubectl patch svc loki -n $NAMESPACE_OBSERVABILITY \
    --type='json' \
    -p='[
      {
        "op": "replace",
        "path": "/spec/type",
        "value": "NodePort"
      },
      {
        "op": "add",
        "path": "/spec/ports/0/nodePort",
        "value": 32100
      }
    ]'

  echo "✅ Loki installed and exposed at NodePort 32100"
}

install_grafana() {
  echo "📊 Installing Grafana..."
  helm repo remove grafana || true
  helm repo add grafana https://grafana.github.io/helm-charts
  helm repo update

  helm install $GRAFANA_RELEASE grafana/grafana \
    --version 10.3.0 \
    --namespace $NAMESPACE_OBSERVABILITY \
    --set service.type=NodePort \
    --set service.nodePort=32000 \
    --set adminPassword='admin' \
    --set persistence.enabled=false

  echo "✅ Grafana installed!"
  echo "🌐 Grafana UI available at NodePort 32000"
  echo "🔐 Login with username: admin and password: admin"
}

observability_stack_install(){
echo "Observability stack installation started"

# Check if collector-scrape-cm-change.txt file exists
if [ ! -f "$HOME/sandbox/pipeline/observability/collector-scrape-cm-change.txt" ]; then
    echo "Error: collector-scrape-cm-change.txt file not found in $HOME/sandbox/pipeline/observability"
    echo "Please ensure the file exists before proceeding."
    exit 1
fi

echo "collector-scrape-cm-change.txt file found, proceeding..."
  create_observability_namespace
  install_jaeger
  install_prometheus
  install_grafana
  install_loki
echo "Observability stack installation completed"
}



observability_stack_uninstall(){
    echo "Observability stack uninstall started"
    cd "$HOME/sandbox/pipeline/observability" || { echo '❌ observability dir missing'; exit 1; }

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

    # Wait for specific pods to be deleted
    kubectl wait --for=delete pods -l app.kubernetes.io/instance=jaeger --timeout=300s || true
    kubectl wait --for=delete pods -l app.kubernetes.io/instance=grafana --timeout=300s || true
    kubectl wait --for=delete pods -l app.kubernetes.io/instance=loki --timeout=300s || true
    kubectl wait --for=delete pods -l app.kubernetes.io/instance=prometheus --timeout=300s || true

    rm -f prometheus-values.yaml loki-values.yaml collector-scrape-cm-change.yaml
    echo "Observability stack uninstall completed"
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

stop_harbor_service() {
  echo "6. Stopping and removing Harbor service..."

  # Stop Harbor container
  if docker ps -a --format '{{.Names}}' | grep harbor; then
    cd "$HOME/sandbox/pipeline/harbor"
    sudo docker compose down --remove-orphans --volumes 2>/dev/null && echo "✅ Stopped Harbor containers"
    sleep 10
  fi

  # Remove Harbor compose directory
  [ -d "$HOME/sandbox/pipeline/harbor" ] && sudo rm -rf "$HOME/sandbox/pipeline/harbor" && echo "✅ Removed Harbor compose directory"

  # Remove Harbor images
  # docker images | grep harbor | awk '{print $3}' | xargs -r docker rmi -f && echo "✅ Removed Harbor images"
}

cleanup_basic_utilities() {
  echo "7. Final cleanup of basic utilities..."

  # Remove temporary files
  rm -f /tmp/go.tar.gz /tmp/resp.json /tmp/headers.txt get-docker.sh 2>/dev/null && echo "✅ Removed temporary files"

  # Clear exported variables
  unset EXPOSED_HARBOR_IP EXPOSED_HARBOR_PORT EXPOSED_SYMPHONY_IP EXPOSED_SYMPHONY_PORT

  # Note: Not removing curl as it might be needed by system
  echo "⚠️ Basic utilities (curl) left installed as they may be system dependencies"

  echo "✅ Environment cleanup completed"
  echo ""
  echo "🔄 Please restart your shell or run 'source ~/.bashrc' to apply PATH changes"
}


build_custom_otel_container_images() {
  echo "Building/Downloading Custom Otel images..."

  cd "$HOME/sandbox/poc/tests/artefacts/custom-otel-helm-app/code/app"
  docker build . -t "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/custom-otel-app:latest"
  echo "Ensuring Harbor registry login..."
  docker login "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}" -u admin -p Harbor12345

  # Docker push them to the harbor registry
  echo "Pushing otel images to Harbor..."
  docker push "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/custom-otel-app:latest"

  OTEL_APP_CONTAINER_URL="${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/custom-otel-app"
  deploy_file="$HOME/sandbox/poc/tests/artefacts/custom-otel-helm-app/code/helm/values.yaml"
  tag="latest"

  echo "Preparing Helm chart..."
  cd "$HOME/sandbox/poc/tests/artefacts/custom-otel-helm-app/code"

  # Read existing chart version
  CHART_FILE="$HOME/sandbox/poc/tests/artefacts/custom-otel-helm-app/code/helm/Chart.yaml"
  CHART_VERSION=$(grep "^version:" "$CHART_FILE" | awk '{print $2}')

  echo "Using existing chart version: $CHART_VERSION"

  # Replace placeholders in values.yaml
  echo "Replacing placeholders in values.yaml..."
  sed -i "s|{{REPOSITORY}}|$OTEL_APP_CONTAINER_URL|g" "$deploy_file" 2>/dev/null || true
  sed -i "s|{{TAG}}|$tag|g" "$deploy_file" 2>/dev/null || true

  # Package and push chart with existing version
  echo "Packaging Helm chart version $CHART_VERSION..."
  helm package helm/

  echo "Pushing chart to Harbor..."
  helm push "custom-otel-helm-${CHART_VERSION}.tgz" "oci://${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library" --plain-http

  # Update margo.yaml in package directory with placeholders
  HELM_REPOSITORY="oci://${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/custom-otel-helm"
  HELM_REVISION="$CHART_VERSION"
  helm_deploy_file="$HOME/sandbox/poc/tests/artefacts/custom-otel-helm-app/margo-package/margo.yaml"

  echo "Updating margo.yaml with chart version $CHART_VERSION..."

  # Only replace placeholders if they exist, don't modify existing values
  sed -i "s|{{HELM_REPOSITORY}}|$HELM_REPOSITORY|g" "$helm_deploy_file" 2>/dev/null || true
  sed -i "s|{{HELM_REVISION}}|$HELM_REVISION|g" "$helm_deploy_file" 2>/dev/null || true

  echo "✅ Custom otel chart version $CHART_VERSION successfully pushed to Harbor"
  echo "📦 Chart: oci://${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/custom-otel-helm:$CHART_VERSION"
  echo "🔄 Updated margo.yaml to reference version $CHART_VERSION"

}


# Alternative simpler version without jq dependency
add_insecure_registry_to_daemon() {
  echo "Configuring Docker daemon with insecure registry..."

  local registry_url="${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}"
  local daemon_config="/etc/docker/daemon.json"

  # Stop Docker if running
  sudo systemctl stop docker docker.socket 2>/dev/null || true

  # Create Docker directory
  sudo mkdir -p /etc/docker

  # Backup existing config
  if [ -f "$daemon_config" ]; then
    sudo cp "$daemon_config" "$daemon_config.backup.$(date +%s)"
    echo "✅ Backed up existing daemon.json"
  fi

  # Create daemon.json
  sudo tee "$daemon_config" > /dev/null <<EOF
{
  "insecure-registries": ["$registry_url"]
}
EOF

  echo "✅ Configured insecure registry: $registry_url"



  # Validate JSON
  if ! jq . "$daemon_config" >/dev/null 2>&1; then
    echo "❌ Invalid JSON in daemon.json"
    sudo mv "$daemon_config.backup."* "$daemon_config" 2>/dev/null || true
    return 1
  fi

  # Start Docker
  echo "Starting Docker daemon..."
  sudo systemctl daemon-reload
  sudo systemctl start docker
  sudo systemctl enable docker

  # Wait for Docker
  for i in {1..30}; do
    if sudo systemctl is-active --quiet docker; then
      echo "✅ Docker daemon started successfully"
      docker version
      return 0
    fi
    echo "Waiting for Docker... ($i/30)"
    sleep 2
  done

  echo "❌ Docker failed to start"
  sudo journalctl -xeu docker.service --no-pager | tail -20
  return 1
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



# ----------------------------
# Main Orchestration Functions
# ----------------------------
install_prerequisites() {
  echo "Running all pre-req setup tasks..."
  install_basic_utilities
  install_go
  install_vim
  #install_and_enable_ssh
  install_docker_and_compose
  add_insecure_registry_to_daemon
  setup_k3s
  install_redis
  install_oras
  clone_symphony_repo
  clone_dev_repo
  add_container_registry_mirror_to_k3s
   
  setup_harbor
  build_custom_otel_container_images
  
  echo ""
  echo "-----------------------------------------------------------------------"
  echo "📦 Pushing pre-existing test-bed application packages to OCI Registry(i.e. harbor)..."
  push_nextcloud_to_oci
  push_custom_otel_to_oci

  echo "✅ Setup completed - TestBed Application packages now in OCI Registry!"
  echo "-----------------------------------------------------------------------"
  echo ""
  echo "✅ Workload Fleet Manager pre-requisites installation finished."
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

create_symphony_api_systemd_service() {
  echo "🔧 Creating systemd service for Symphony API auto-start..."

  # Get the actual symphony api directory path
  local symphony_dir="$HOME/symphony/api"

  # Create systemd service file with absolute path
  sudo tee /etc/systemd/system/symphony-api.service > /dev/null <<EOF
[Unit]
Description=Margo Symphony API Server
Requires=docker.service redis-server.service
After=docker.service redis-server.service network-online.target
Wants=network-online.target

[Service]
Type=simple
RemainAfterExit=yes
WorkingDirectory=${symphony_dir}
ExecStartPre=/bin/sleep 15
ExecStartPre=-/usr/bin/docker stop symphony-api-container
ExecStartPre=-/usr/bin/docker rm symphony-api-container
ExecStart=/usr/bin/docker run --rm --name symphony-api-container \
    --network host \
    -p 8082:8082 \
    -e LOG_LEVEL=Debug \
    -v ${symphony_dir}/certificates:/certificates \
    -v ${symphony_dir}:/configs \
    -e CONFIG=symphony-api-margo.json \
    margo-symphony-api:latest
ExecStop=/usr/bin/docker stop symphony-api-container
TimeoutStartSec=0
Restart=on-failure
RestartSec=10s

[Install]
WantedBy=multi-user.target
EOF

  # Reload systemd and enable the service
  sudo systemctl daemon-reload
  sudo systemctl enable symphony-api.service

  echo "✅ Symphony API systemd service created and enabled"
  echo "📋 Service will start Symphony API automatically on boot"
  echo "📁 Working directory: ${symphony_dir}"
}

start_symphony_api_container(){

    cd "$HOME/symphony/api"
    echo "Building Symphony API container..."

    # Stop and remove existing container if present
    echo "Stopping and removing existing symphony-api-container if present..."
    docker stop symphony-api-container 2>/dev/null || true
    docker rm symphony-api-container 2>/dev/null || true
    pkill -f "symphony-api" 2>/dev/null || true

    # Check if image already exists
    echo "Checking GHCR image: ${SYMPHONY_IMAGE_REF}"
    if docker manifest inspect "${SYMPHONY_IMAGE_REF}" >/dev/null 2>&1; then
      echo "Image exists in GHCR"
    else
      echo "Image does NOT exist in GHCR"
      return 1
    fi  

    # Pull the image
    echo "Pulling image: ${SYMPHONY_IMAGE_REF}"
    docker pull "${SYMPHONY_IMAGE_REF}"

    # Run the container
    echo "🚀 Starting Symphony API container..."

    docker run -dit --name symphony-api-container \
        --network host \
        -p 8082:8082 \
        -e LOG_LEVEL=Debug \
        -v "$HOME/symphony/api/certificates:/certificates" \
        -v "$HOME/symphony/api":/configs \
        -e CONFIG=symphony-api-margo.json \
        "${SYMPHONY_IMAGE_REF}"
        
    # Verify container is running
    if docker ps --format '{{.Names}}' | grep -q symphony-api-container; then
        echo "✅ Symphony API container started successfully"
        echo "📡 Container is running on port 8082 (host network)"
        echo "🏷️  Container name: symphony-api-container"
         # Create systemd service for auto-start on boot
        create_symphony_api_systemd_service
    else
        echo "❌ Failed to start Symphony API container"
        docker logs symphony-api-container || true
        return 1
    fi
 
}

stop_symphony() {
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


# Collect certificate information
collect_certs_info() {
    echo "Collecting certificate information..."
    CN="${EXPOSED_SYMPHONY_IP:-localhost}"
    C="IN"
    ST="GGN"
    L="Some ABC Location"
    O="Margo"
    EMAIL="admin@example.com"
    DAYS="365"
    SAN_DOMAINS="${EXPOSED_SYMPHONY_IP:-localhost}"
    SAN_IPS="${EXPOSED_SYMPHONY_IP:-localhost}"

    echo "Using certificate defaults with CN: $CN"
}


# Generate OpenSSL config
generate_config_for_certs() {
    local config_file="$1"
    local cert_type="$2"

    # Create base config
    cat > "$config_file" << EOF
[req]
default_bits = 2048
prompt = no
distinguished_name = dn
$([ "$cert_type" = "server" ] && echo "req_extensions = v3_req")

[dn]
C=$C
ST=$ST
L=$L
O=$O
CN=$CN
emailAddress=$EMAIL

[v3_req]
basicConstraints = CA:FALSE
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = $CN
EOF

    # Initialize counters
    local dns_count=2
    local ip_count=1

    # Add SAN domains if provided
    if [ -n "$SAN_DOMAINS" ]; then
        echo "Adding SAN domains..."
        IFS=',' read -ra DOMAINS <<< "$SAN_DOMAINS"
        for domain in "${DOMAINS[@]}"; do
            echo "DNS.$dns_count = ${domain// /}" >> "$config_file"
            ((dns_count++))
        done
    fi

    # Add SAN IPs if provided
  if [ -n "$SAN_IPS" ]; then
    echo "Adding SAN IPs..."
    IFS=',' read -ra IPS <<< "$SAN_IPS"
    for ip in "${IPS[@]}"; do
        echo "IP.$ip_count = ${ip// /}" >> "$config_file"
        ((ip_count++))
    done
  fi

    # Debug output
    echo "Generated OpenSSL config at $config_file:"
    cat "$config_file"
}


# Generate CA certificate
generate_ca() {
    info "Generating CA certificate..."
    local ca_key="$CERT_DIR/ca-key.pem"
    local ca_cert="$CERT_DIR/ca-cert.pem"
    local ca_config="$CERT_DIR/ca.conf"

    generate_config_for_certs "$ca_config" "ca"

    openssl genrsa -out "$ca_key" 2048
    openssl req -new -x509 -key "$ca_key" -out "$ca_cert" -days "$DAYS" -config "$ca_config"
    chmod 600 "$ca_key"

    success "CA generated: $ca_cert"
}

# Generate server certificate
generate_server_certs() {
    echo "Generating server certificate..."
    if ! mkdir -p "$CERT_DIR"; then
        echo "Error: Failed to create directory $CERT_DIR"
        return 1
    fi

    if [[ ! -w "$CERT_DIR" ]]; then
        echo "Error: Cannot write to $CERT_DIR"
        return 1
    fi
    local server_key="$CERT_DIR/server-key.pem"
    local server_csr="$CERT_DIR/server.csr"
    local server_cert="$CERT_DIR/server-cert.pem"
    local server_config="$CERT_DIR/server.conf"
    generate_ca
    generate_config_for_certs "$server_config" "server"

    openssl genrsa -out "$server_key" 2048
    openssl req -new -key "$server_key" -out "$server_csr" -config "$server_config"

    if [[ -f "$CERT_DIR/ca-cert.pem" ]]; then
        openssl x509 -req -in "$server_csr" -CA "$CERT_DIR/ca-cert.pem" -CAkey "$CERT_DIR/ca-key.pem" \
            -CAcreateserial -out "$server_cert" -days "$DAYS" -extensions v3_req -extfile "$server_config"
        success "Server certificate signed by CA: $server_cert"
    else
        openssl x509 -req -in "$server_csr" -signkey "$server_key" -out "$server_cert" -days "$DAYS" \
            -extensions v3_req -extfile "$server_config"
        success "Self-signed server certificate: $server_cert"
    fi

    rm -f "$server_csr"
    chmod 600 "$server_key"
}
install_vim() {
  echo "🔄 Installing Vim..."
  if command -v vim >/dev/null 2>&1; then
    echo "⚡️ Vim already installed, skipping installation"
    return
  fi

  if command -v apt >/dev/null 2>&1; then
    sudo apt update -y
    sudo apt install -y vim
  else
    sudo yum install -y vim || sudo dnf install -y vim
  fi

  echo "✅ Vim installation completed."
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

  echo "🔄 Installing OpenSSH Server..."
  if [ "$OS" = "debian" ]; then
    if dpkg -s openssh-server >/dev/null ; then
      echo "⚡️ OpenSSH Server already installed, skipping installation"
    else
      sudo apt update -y
      sudo apt install -y openssh-server
      echo "✅ OpenSSH Server installation completed."
    fi
  else
    if rpm -q openssh-server >/dev/null ; then
      echo "⚡️ OpenSSH Server already installed, skipping installation"
    else
      sudo yum install -y openssh-server || sudo dnf install -y openssh-server
      echo "✅ OpenSSH Server installation completed."
    fi
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
  sudo systemctl status ssh --no-pager || systemctl status sshd
  echo "[SUCCESS] SSH service installed and running."
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
