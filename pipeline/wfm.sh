#!/bin/bash
set -e

# ----------------------------
# Environment & Validation
# ----------------------------

#--- Github Settings to pull the code (can be overridden via env)
GITHUB_USER="${GITHUB_USER:-}"  # Set via env or leave empty
GITHUB_TOKEN="${GITHUB_TOKEN:-}"  # Set via env or leave empty

#--- branch details (can be overridden via env)
SYMPHONY_BRANCH="${SYMPHONY_BRANCH:-margo-dev-sprint-6}"
DEV_REPO_BRANCH="${DEV_REPO_BRANCH:-dev-sprint-6}"

#--- harbor settings (can be overridden via env)
EXPOSED_HARBOR_IP="${EXPOSED_HARBOR_IP:-127.0.0.1}"
EXPOSED_HARBOR_PORT="${EXPOSED_HARBOR_PORT:-8081}"

#--- symphony settings (can be overridden via env)
EXPOSED_SYMPHONY_IP="${EXPOSED_SYMPHONY_IP:-127.0.0.1}"
EXPOSED_SYMPHONY_PORT="${EXPOSED_SYMPHONY_PORT:-8082}"

#--  device node IP (can be overridden via env) for prometheus to scrape metrics 
DEVICE_NODE_IP="${DEVICE_NODE_IP:-127.0.0.1}"

#--- keycloak settings (can be overridden via env)
EXPOSED_KEYCLOAK_IP="${EXPOSED_KEYCLOAK_IP:-127.0.0.1}"
EXPOSED_KEYCLOAK_PORT="${EXPOSED_KEYCLOAK_PORT:-8083}"

#--- gogs settings (can be overridden via env)
EXPOSED_GOGS_IP="${EXPOSED_GOGS_IP:-127.0.0.1}"
EXPOSED_GOGS_PORT="${EXPOSED_GOGS_PORT:-8084}"



# variables for observability stack
NAMESPACE_OBSERVABILITY="observability"
JAEGER_RELEASE="jaeger"
PROM_RELEASE="prometheus"
GRAFANA_RELEASE="grafana"
LOKI_RELEASE="loki"

# ----------------------------
# Utility Functions
# ----------------------------
validate_passwordless_sudo() {
  local username="${1:-$(whoami)}"
  local exit_code=0
  
  echo "Validating passwordless for user: $username"
  echo "==============================================="
  
  # Test 1: Basic test
  echo -n "Test 1 - Basic access: "
  if -n true 2>/dev/null; then
      echo "✓ PASS"
  else
      echo "✗ FAIL"
      exit_code=1
  fi
  
  # Test 2: Specific command test
  echo -n "Test 2 - Command execution: "
  if -n whoami >/dev/null 2>&1; then
      echo "✓ PASS"
  else
      echo "✗ FAIL"
      exit_code=1
  fi
  
  # Test 3: File access test
  echo -n "Test 3 - File access: "
  if -n test -r /etc/shadow 2>/dev/null; then
      echo "✓ PASS"
  else
      echo "✗ FAIL"
      exit_code=1
  fi
  
  # Test 4: Configuration verification
  echo -n "Test 4 - Config verification: "
  if grep -q "$username.*NOPASSWD\|%.*NOPASSWD" /etc/sudoers /etc/sudoers.d/* 2>/dev/null; then
      echo "✓ PASS"
  else
      echo "✗ FAIL"
      exit_code=1
  fi
  
  echo "==============================================="
  if [ $exit_code -eq 0 ]; then
      echo "✓ All tests passed - Passwordless is properly configured"
  else
      echo "✗ Some tests failed - Passwordless may not be fully configured"
  fi
  
  return $exit_code
}

# ----------------------------
# Installation Functions
# ----------------------------
install_basic_utilities() {
  INSTALL_HELM_V3_15_1=true
  HELM_VERSION="3.15.1"
  HELM_TAR="helm-v${HELM_VERSION}-linux-amd64.tar.gz"
  HELM_BIN_DIR="/usr/local/bin"

  apt update && apt install -y curl dos2unix build-essential gcc libc6-dev
  install_helm
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
  cd $HOME
  if which go >/dev/null 2>&1; then
    echo 'Go already installed, skipping installation';
    go version;
  else
    echo 'Go not found, installing...';
    rm -rf /usr/local/go /usr/bin/go
    wget "https://go.dev/dl/go1.23.2.linux-amd64.tar.gz" -O go.tar.gz;
    tar -C /usr/local -xzf go.tar.gz;
    rm go.tar.gz
    export PATH="$PATH:/usr/local/go/bin";
    source ~/.bashrc
    which go;
    go version;
  fi
}

install_docker_compose() {
  cd $HOME
  if ! command -v docker >/dev/null 2>&1; then
    echo 'Docker not found. Installing Docker...';
    apt-get remove -y docker docker-engine docker.io containerd runc || true;
    curl -fsSL "https://get.docker.com" -o get-docker.sh; sh get-docker.sh;
    usermod -aG docker $USER;
  else
    echo 'Docker already installed.';
  fi;

  if ! command -v docker-compose >/dev/null 2>&1; then
    echo 'Docker Compose not found. Installing Docker Compose...';
    curl -L "https://github.com/docker/compose/releases/download/v2.24.6/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose;
    chmod +x /usr/local/bin/docker-compose;
  else
    echo 'Docker Compose already installed.';
  fi
  # Start and enable Docker daemon
  systemctl start docker
  systemctl enable docker
  # Wait for Docker daemon to be active (max 30s)
  for i in $(seq 1 30); do
    if systemctl is-active --quiet docker; then
      echo 'Docker daemon is running.'
      break
    else
      echo 'Waiting for Docker daemon to start... ($i/30)'
      sleep 1
    fi
  done 
}

install_rust() {
  cd "$HOME"
  echo 'Installing Rust...';
  curl --proto "=https" --tlsv1.2 -sSf "https://sh.rustup.rs" | sh -s -- -y;
  source $HOME/.cargo/env
}

# ----------------------------
# Repository Functions
# ----------------------------
clone_symphony_repo() {
  cd "$HOME"
  echo 'Cloning symphony...'
  sudo rm -rf "$HOME/symphony"
  git clone "https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/margo/symphony.git" "$HOME/symphony"
  cd "$HOME/symphony"
  git checkout ${SYMPHONY_BRANCH} || echo 'Branch ${SYMPHONY_BRANCH} not found'
  echo "symphony checkout to branch ${SYMPHONY_BRANCH} done"
}

clone_dev_repo() {
  cd "$HOME"
  sudo rm -rf "$HOME/dev-repo"
  git clone "https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/margo/dev-repo.git"
  cd "$HOME/dev-repo"
  git checkout ${DEV_REPO_BRANCH} || echo 'Branch ${DEV_REPO_BRANCH} not found'
  echo "dev-repo checkout to branch ${DEV_REPO_BRANCH} done"
}

# ----------------------------
# Service Setup Functions
# ----------------------------
setup_keycloak() {
  if docker ps --format '{{.Names}}' | grep -q keycloak; then
    echo 'Keycloak is already running, skipping startup.'
  else
    echo 'Starting Keycloak...'
    cd $HOME/dev-repo/pipeline/keycloak
    chmod +x init.sh
    docker compose up -d
    sleep 60
    docker ps | grep keycloak || echo 'Keycloak did not start properly'
  fi
}

update_keycloak_config() {
  cd $HOME
  echo "Updating keycloak URL in symphony-api-margo.json..."
  sed -i "s|\"keycloakURL\": *\"http://[^\"]*\"|\"keycloakURL\": \"http://"$EXPOSED_KEYCLOAK_IP":$EXPOSED_KEYCLOAK_PORT\"|" "$HOME/symphony/api/symphony-api-margo.json"
  echo "Updated keycloak URL in symphony-api-margo.json"
}

setup_harbor() {
  if docker ps --format '{{.Names}}' | grep -q harbor; then
    echo 'Harbor is already running, skipping startup.'
  else
    cd "$HOME/dev-repo/pipeline/harbor"
    echo 'Starting Harbor...'
    sudo chmod +x install.sh prepare common.sh
    sudo bash install.sh
    docker ps
    sleep 10
    docker ps | grep harbor || echo 'Harbor did not start properly'
  fi
}

setup_gogs_directories() {
  GOGS_BASE_DIR="$HOME/dev-repo/pipeline/gogs"
  DATA_DIR="$GOGS_BASE_DIR/data"
  LOGS_DIR="$GOGS_BASE_DIR/logs"
  APP_INI_PATH="$GOGS_BASE_DIR/app.ini"
  GOGS_IP="$EXPOSED_GOGS_IP"
  GOGS_PORT=$EXPOSED_GOGS_PORT
  
  rm -rf "$DATA_DIR" "$LOGS_DIR"
  mkdir -p "$GOGS_BASE_DIR" "$DATA_DIR" "$LOGS_DIR"
  chmod -R 777 "$DATA_DIR" "$LOGS_DIR"
  # Fix line endings + permissions for entrypoint
  # chmod +x "$GOGS_BASE_DIR/entrypoint.sh"
  # Update template with remote GOGS_IP details
  # Use printf to build the replacement strings safely
  DOMAIN_LINE=$(printf "DOMAIN              = %s" "$GOGS_IP")
  HTTP_PORT_LINE=$(printf "HTTP_PORT        = %s" "$GOGS_PORT")
  EXTERNAL_URL_LINE=$(printf "EXTERNAL_URL  = http://%s:%s/" "$GOGS_IP" "$GOGS_PORT")

  sed -i 's/\r$//' "$APP_INI_PATH"
  sed -i "s/^DOMAIN.*/$DOMAIN_LINE/" "$APP_INI_PATH"
  sed -i "s/^HTTP_PORT.*/$HTTP_PORT_LINE/" "$APP_INI_PATH"
  sed -i "s|^EXTERNAL_URL.*|$EXTERNAL_URL_LINE|" "$APP_INI_PATH"
  # chown 1000:1000 "$APP_INI_PATH"

  echo 'Final runtime app.ini:'
  grep -E 'DOMAIN|HTTP_PORT|EXTERNAL_URL' "$APP_INI_PATH"
}

start_gogs() {
  echo 'Starting Gogs container...'
  GOGS_BASE_DIR="$HOME/dev-repo/pipeline/gogs/"
  cd "$GOGS_BASE_DIR"
  docker-compose down
  docker-compose build --no-cache gogs
  docker-compose -f docker-compose.yml up -d
}

wait_for_gogs() {
  GOGS_BASE_DIR="$HOME/dev-repo/pipeline/gogs/"
  cd "$GOGS_BASE_DIR"
  GOGS_IP=$EXPOSED_GOGS_IP
  GOGS_PORT=$EXPOSED_GOGS_PORT
  for i in {1..32}; do
    if curl -s http://$GOGS_IP:$GOGS_PORT | grep -q "Gogs"; then
      echo "Gogs is up!";
      break;
    fi;
    sleep 2;
  done
}

create_gogs_admin() {
  GOGS_BASE_DIR="$HOME/dev-repo/pipeline/gogs/"
  cd "$GOGS_BASE_DIR"
  GOGS_IP=$EXPOSED_GOGS_IP
  GOGS_PORT=$EXPOSED_GOGS_PORT
  GOGS_CONTAINER=$(docker ps --filter "name=gogs" --format "{{.Names}}" | head -n 1)
  if [ -z "$GOGS_CONTAINER" ]; then
    echo "Gogs container not found! Exiting."
    exit 1
  fi

  docker exec -u git "$GOGS_CONTAINER" /app/gogs/gogs admin create-user \
    --name gogsadmin \
    --password admin123 \
    --email you@example.com \
    --admin || echo "User might already exist, skipping..."
}

create_gogs_token() {
  GOGS_BASE_DIR="$HOME/dev-repo/pipeline/gogs/"
  cd "$GOGS_BASE_DIR"
  GOGS_IP=$EXPOSED_GOGS_IP
  GOGS_PORT=$EXPOSED_GOGS_PORT
  TOKEN_NAME="autogen-$(date +%s)"
  TOKEN_RESP=$(curl -s -X POST -u 'gogsadmin:admin123' -H 'Content-Type: application/json' -d "{\"name\": \"$TOKEN_NAME\"}" "http://$GOGS_IP:$GOGS_PORT/api/v1/users/gogsadmin/tokens")
  echo "TOKEN RESP $TOKEN_RESP"
  TOKEN=$(echo ${TOKEN_RESP} | jq -r '.sha1')
  export GOGS_TOKEN=$TOKEN
}

create_gogs_repositories() {
  # Create nextcloud repo
  GOGS_BASE_DIR="$HOME/dev-repo/pipeline/gogs/"
  cd "$GOGS_BASE_DIR"

  GOGS_IP=$EXPOSED_GOGS_IP
  GOGS_PORT=$EXPOSED_GOGS_PORT
  echo "GOGS TOKEN: $GOGS_TOKEN"
  curl -s -o /tmp/resp.json -w '\nHTTP %{http_code}\n' \
    -H "Authorization: token $GOGS_TOKEN" \
    -H 'Content-Type: application/json' \
    -d '{"name":"nextcloud","private":false}' \
    "http://$GOGS_IP:$GOGS_PORT/api/v1/user/repos"
  cat /tmp/resp.json

  GOGS_IP=$EXPOSED_GOGS_IP
  GOGS_PORT=$EXPOSED_GOGS_PORT
  curl -s -o /tmp/resp.json -w '\nHTTP %{http_code}\n' \
    -H "Authorization: token $GOGS_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"name":"nginx","private":false}' \
    "http://$GOGS_IP:$GOGS_PORT/api/v1/user/repos"
  cat /tmp/resp.json

  # Create nextcloud repo
  GOGS_IP=$EXPOSED_GOGS_IP
  GOGS_PORT=$EXPOSED_GOGS_PORT
  curl -s -o /tmp/resp.json -w '\nHTTP %{http_code}\n' \
    -H "Authorization: token $GOGS_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"name":"otel","private":false}' \
    http://$GOGS_IP:$GOGS_PORT/api/v1/user/repos
  cat /tmp/resp.json

  # Create nextcloud repo
  GOGS_IP=$EXPOSED_GOGS_IP
  GOGS_PORT=$EXPOSED_GOGS_PORT
  curl -s -o /tmp/resp.json -w '\nHTTP %{http_code}\n' \
    -H "Authorization: token $GOGS_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"name":"custom-otel","private":false}' \
    http://$GOGS_IP:$GOGS_PORT/api/v1/user/repos
  cat /tmp/resp.json
}

push_nextcloud_files() {
  cd "$HOME/dev-repo/poc/tests/artefacts/nextcloud-compose/margo-package" || { echo '❌ Nextcloud dir missing'; exit 1; }
  [ ! -d .git ] && git init && \
    git config user.name 'gogsadmin' && \
    git config user.email 'nitin.parihar@capgemini.com'
  git remote remove origin 2>/dev/null || true
  git remote add origin "http://gogsadmin:${GOGS_TOKEN}@${EXPOSED_GOGS_IP}:${EXPOSED_GOGS_PORT}/gogsadmin/nextcloud.git"
  git add margo.yaml resources/ 2>/dev/null || true
  if ! git diff --cached --quiet; then
    git commit -m 'Initial commit with Nextcloud files'
  fi
  git branch -m master
  git push -u origin master --force
}

push_nginx_files() {
  cd "$HOME/dev-repo/poc/tests/artefacts/nginx-helm/margo-package" || { echo '❌ nginx-helm dir missing'; exit 1; }
  [ ! -d .git ] && git init && \
    git config user.name 'gogsadmin' && \
    git config user.email 'nitin.parihar@capgemini.com'
  git remote remove origin 2>/dev/null || true
  git remote add origin "http://gogsadmin:${GOGS_TOKEN}@${EXPOSED_GOGS_IP}:${EXPOSED_GOGS_PORT}/gogsadmin/nginx.git"
  git add margo.yaml resources/ 2>/dev/null || true
  if ! git diff --cached --quiet; then
    git commit -m 'Initial commit with nginx-helm files'
  fi
  git branch -m master
  git push -u origin master --force
}

push_otel_files() {
  cd "$HOME/dev-repo/poc/tests/artefacts/open-telemetry-demo-helm/margo-package" || { echo '❌ OTEL dir missing'; exit 1; }
  [ ! -d .git ] && git init && \
    git config user.name 'gogsadmin' && \
    git config user.email 'nitin.parihar@capgemini.com'
  git remote remove origin 2>/dev/null || true
  git remote add origin "http://gogsadmin:${GOGS_TOKEN}@${EXPOSED_GOGS_IP}:${EXPOSED_GOGS_PORT}/gogsadmin/otel.git"
  git add margo.yaml resources/ 2>/dev/null || true
  if ! git diff --cached --quiet; then
    git commit -m 'Initial commit with OTEL files'
  fi
  git branch -m master
  git push -u origin master --force
}

push_custom_otel_files() {
  cd "$HOME/dev-repo/poc/tests/artefacts/custom-otel-helm-app/margo-package" || { echo '❌ Custom OTEL dir missing'; exit 1; }
  [ ! -d .git ] && git init && \
    git config user.name 'gogsadmin' && \
    git config user.email 'nitin.parihar@capgemini.com'
  git remote remove origin 2>/dev/null || true
  git remote add origin "http://gogsadmin:${GOGS_TOKEN}@${EXPOSED_GOGS_IP}:${EXPOSED_GOGS_PORT}/gogsadmin/custom-otel.git"
  git add margo.yaml resources/ 2>/dev/null || true
  if ! git diff --cached --quiet; then
    git commit -m 'Initial commit with Custom OTEL files'
  fi
  git branch -m master
  git push -u origin master --force
}

# ----------------------------
# Symphony Build Functions
# ----------------------------
build_symphony_api() {
  echo 'Building Symphony API...'
  cd "$HOME/symphony/api"
  export PATH=$PATH:/usr/local/go/bin
  go mod tidy
  go build -o symphony-api .
  echo 'Symphony API build completed'
}

build_symphony_ui() {
  echo 'Building Symphony UI...'
  cd "$HOME/symphony/api"
  npm install
  npm run build
  echo 'Symphony UI build completed'
}

# ----------------------------
# Build Functions
# ----------------------------
build_rust() {
  source "$HOME/.cargo/env"; 
  RUST_DIR="$HOME/symphony/api/pkg/apis/v1alpha1/providers/target/rust"; 
  if [ -d "$RUST_DIR" ]; then 
    cd "$RUST_DIR"; 
    cargo build --release;
  fi
}

build_symphony_api_server() {
  git config --global url."https://${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/";
  go env -w GOPRIVATE="github.com/margo/*";
  GO_DIR="$HOME/symphony/api";
  if [ -d "$GO_DIR" ]; then
    export LD_LIBRARY_PATH="$HOME/symphony/api/pkg/apis/v1alpha1/providers/target/rust/target/release";
    cd "$GO_DIR";
    go build -o symphony-api;
  fi
}

build_maestro_cli() {
  CLI_DIR="$HOME/symphony/cli";
  if [ -d "$CLI_DIR" ]; then 
    cd "$CLI_DIR"; 
    go mod tidy; 
    go build -o maestro; 
  fi
}

verify_symphony_api() {
  file "$HOME/symphony/api/symphony-api";
  ls -l "$HOME/symphony/api/symphony-api";
}

start_symphony_api() {
  cd "$HOME/symphony/api" || exit 1
  echo 'Starting Symphony API...'
  nohup ./symphony-api -c ./symphony-api-margo.json -l Debug > $HOME/symphony-api.log 2>&1 &
  sleep 5
  echo '--- Symphony API logs ---'
  tail -n 50 $HOME/symphony-api.log
}

function install_jaeger() {
  echo "🔄 Refreshing Jaeger Helm repo..."
  helm repo remove jaegertracing || true
  helm repo add jaegertracing https://jaegertracing.github.io/helm-charts
  helm repo update

  echo "🚀 Installing Jaeger with OTLP and NodePort configuration..."
  helm install $JAEGER_RELEASE jaegertracing/jaeger \
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

function install_prometheus() {
  cd "$HOME/dev-repo/pipeline/observability"
  echo "📡 Setting up Prometheus to expose metrics for OTEL Collector..."

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
            - targets: ['${DEVICE_NODE_IP}:30999']
EOF

  helm repo remove prometheus-community || true
  helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
  helm repo update

  helm install $PROM_RELEASE prometheus-community/prometheus \
    --namespace $NAMESPACE_OBSERVABILITY \
    -f prometheus-values.yaml

  echo "✅ Prometheus setup complete!"
  echo "📊 Prometheus UI: NodePort 30900"
  echo "📡 Metrics exposed at ${DEVICE_NODE_IP}:30999"

  patch_prometheus_configmap
}

function patch_prometheus_configmap() {
  cd "$HOME/dev-repo/pipeline/observability"
  echo "🛠 Applying Prometheus ConfigMap with DEVICE_NODE_IP..."

  CM_SOURCE="collector-scrape-cm-change.txt"
  CM_TARGET="collector-scrape-cm-change.yaml"

  if [ ! -f "$CM_SOURCE" ]; then
    echo "❌ Source ConfigMap file '$CM_SOURCE' not found."
    exit 1
  fi

  sed "s|__DEVICE_NODE_IP__|${DEVICE_NODE_IP}|g" "$CM_SOURCE" > "$CM_TARGET"

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



function install_loki() {
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

  helm install $LOKI_RELEASE grafana/loki -f loki-values.yaml --namespace $NAMESPACE_OBSERVABILITY

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

function install_grafana() {
  echo "📊 Installing Grafana..."
  helm repo remove grafana || true
  helm repo add grafana https://grafana.github.io/helm-charts
  helm repo update

  helm install $GRAFANA_RELEASE grafana/grafana \
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
if [ ! -f "$HOME/dev-repo/pipeline/observability/collector-scrape-cm-change.txt" ]; then
    echo "Error: collector-scrape-cm-change.txt file not found in $HOME/dev-repo/pipeline/observability"
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
    cd "$HOME/dev-repo/pipeline/observability" || { echo '❌ observability dir missing'; exit 1; }

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

# ----------------------------
# Uninstall Functions (Reverse Chronological Order)
# ----------------------------
uninstall_prerequisites() {
  echo "Running complete uninstallation in reverse chronological order..."
  
  # Step 1: Stop Symphony API (Last thing that would be running)
  # stop_symphony_api_process
  
  # Step 2: Remove Symphony binaries and builds
  cleanup_symphony_builds
  
  # Step 3: Remove Git repositories and pushed files
  cleanup_git_repositories
  
  # Step 4: Remove Gogs repositories
  remove_gogs_repositories
  
  # Step 5: Remove Gogs admin user and token
  cleanup_gogs_admin
  
  # Step 6: Stop and remove Gogs container
  stop_gogs_service
  
  # Step 7: Cleanup Gogs directories and configuration
  cleanup_gogs_directories
  
  # Step 8: Revert Keycloak configuration
  revert_keycloak_config
  
  # Step 9: Stop and remove Keycloak
  stop_keycloak_service

  # Step 10: Stop and remove Keycloak
  stop_harbor_service
  
  # Step 11: Remove cloned repositories
  remove_cloned_repositories
  
  # Step 12: Uninstall Rust
  uninstall_rust
  
  # Step 13: Uninstall Docker and Docker Compose
  uninstall_docker_compose
  
  # Step 14: Uninstall Go
  uninstall_go
  
  # Step 15: Remove basic utilities and cleanup
  cleanup_basic_utilities
  
  echo "Complete uninstallation finished"
}

# Individual uninstall functions
stop_symphony_api_process() {
  echo "1. Stopping Symphony API process..."
  PID=$(ps -ef | grep '[s]ymphony-api-margo.json' | awk '{print $2}')
  if [ -n "$PID" ]; then
    kill -9 $PID && echo "✅ Symphony API stopped (PID: $PID)"
  else
    echo "ℹ️ Symphony API was not running"
  fi
  
  # Remove log file
  [ -f "$HOME/symphony-api.log" ] && rm -f "$HOME/symphony-api.log" && echo "✅ Removed symphony-api.log"
}

cleanup_symphony_builds() {
  echo "2. Cleaning up Symphony builds..."
  
  # Remove built binaries
  [ -f "$HOME/symphony/api/symphony-api" ] && rm -f "$HOME/symphony/api/symphony-api" && echo "✅ Removed symphony-api binary"
  [ -f "$HOME/symphony/cli/maestro" ] && rm -f "$HOME/symphony/cli/maestro" && echo "✅ Removed maestro CLI binary"
  
  # Clean Rust build artifacts
  RUST_DIR="$HOME/symphony/api/pkg/apis/v1alpha1/providers/target/rust"
  if [ -d "$RUST_DIR/target" ]; then
    rm -rf "$RUST_DIR/target" && echo "✅ Removed Rust build artifacts"
  fi
  
  # Clean Go build cache
  if command -v go >/dev/null 2>&1; then
    go clean -cache -modcache 2>/dev/null && echo "✅ Cleaned Go build cache"
  fi
}

cleanup_git_repositories() {
  echo "3. Cleaning up local Git repositories..."
  
  # Clean up pushed file directories
  local dirs=(
    "$HOME/dev-repo/poc/tests/artefacts/nextcloud-compose"
    "$HOME/dev-repo/poc/tests/artefacts/nginx-helm"
    "$HOME/dev-repo/poc/tests/artefacts/open-telemetry-demo-helm"
    "$HOME/dev-repo/poc/tests/artefacts/custom-otel-helm-app"
  )
  
  for dir in "${dirs[@]}"; do
    if [ -d "$dir/.git" ]; then
      cd "$dir" && git remote remove origin 2>/dev/null && echo "✅ Removed git remote from $(basename $dir)"
      rm -rf "$dir/.git" && echo "✅ Removed .git directory from $(basename $dir)"
    fi
  done
}

remove_gogs_repositories() {
  echo "4. Removing Gogs repositories..."
  
  if [ -n "$GOGS_TOKEN" ] && [ -n "$EXPOSED_GOGS_IP" ]; then
    local repos=("nextcloud" "nginx" "otel" "custom-otel")
    GOGS_IP=$EXPOSED_GOGS_IP
    GOGS_PORT=$EXPOSED_GOGS_PORT
    
    for repo in "${repos[@]}"; do
      echo "Attempting to delete repository: $repo"
      curl -s -X DELETE \
        -H "Authorization: token $GOGS_TOKEN" \
        "http://$GOGS_IP:$GOGS_PORT/api/v1/repos/gogsadmin/$repo" && \
        echo "✅ Deleted repository: $repo" || \
        echo "⚠️ Failed to delete repository: $repo"
    done
  else
    echo "⚠️ Cannot delete Gogs repositories - missing token or host"
  fi
}

cleanup_gogs_admin() {
  echo "5. Cleaning up Gogs admin user..."
  
  GOGS_CONTAINER=$(docker ps --filter "name=gogs" --format "{{.Names}}" | head -n 1)
  if [ -n "$GOGS_CONTAINER" ]; then
    echo "⚠️ Gogs admin user 'gogsadmin' should be manually removed if needed"
  fi
  
  # Clear token from environment
  unset GOGS_TOKEN
  echo "✅ Cleared GOGS_TOKEN from environment"
}

stop_gogs_service() {
  echo "6. Stopping and removing Gogs service..."
  
  GOGS_BASE_DIR="$HOME/dev-repo/pipeline/gogs"
  if [ -d "$GOGS_BASE_DIR" ]; then
    cd "$GOGS_BASE_DIR"
    docker-compose down --remove-orphans --volumes 2>/dev/null && echo "✅ Stopped Gogs containers"
    
    # Remove Gogs images
    # docker images | grep gogs | awk '{print $3}' | xargs -r docker rmi -f && echo "✅ Removed Gogs images"
  fi
}

cleanup_gogs_directories() {
  echo "7. Cleaning up Gogs directories..."
  
  GOGS_BASE_DIR="$HOME/dev-repo/pipeline/gogs"
  DATA_DIR="$GOGS_BASE_DIR/data"
  LOGS_DIR="$GOGS_BASE_DIR/logs"
  
  # Remove data and logs
  [ -d "$DATA_DIR" ] && rm -rf "$DATA_DIR" && echo "✅ Removed Gogs data directory"
  [ -d "$LOGS_DIR" ] && rm -rf "$LOGS_DIR" && echo "✅ Removed Gogs logs directory"
  
  # Restore original app.ini if backup exists
  if [ -f "$GOGS_BASE_DIR/app.ini.backup" ]; then
    mv "$GOGS_BASE_DIR/app.ini.backup" "$GOGS_BASE_DIR/app.ini" && echo "✅ Restored original app.ini"
  fi
}

revert_keycloak_config() {
  echo "8. Reverting Keycloak configuration..."
  
  # Restore original symphony-api-margo.json if backup exists
  if [ -f "$HOME/symphony/api/symphony-api-margo.json.backup" ]; then
    mv "$HOME/symphony/api/symphony-api-margo.json.backup" "$HOME/symphony/api/symphony-api-margo.json" && \
    echo "✅ Restored original symphony-api-margo.json"
  else
    echo "⚠️ No backup found for symphony-api-margo.json"
  fi
}

stop_keycloak_service() {
  echo "9. Stopping and removing Keycloak service..."
  
  # Stop Keycloak container
  if docker ps --format '{{.Names}}' | grep -q keycloak; then
    docker stop keycloak && echo "✅ Stopped Keycloak container"
    docker rm keycloak && echo "✅ Removed Keycloak container"
  fi
  
  # Remove Keycloak compose directory
  [ -d "$HOME/dev-repo/pipeline/keycloak" ] && rm -rf "$HOME/dev-repo/pipeline/keycloak" && echo "✅ Removed Keycloak compose directory"
  
  # Remove Keycloak images
  # docker images | grep keycloak | awk '{print $3}' | xargs -r docker rmi -f && echo "✅ Removed Keycloak images"
}

stop_harbor_service() {
  echo "10. Stopping and removing Harbor service..."
  
  # Stop Harbor container
  if docker ps --format '{{.Names}}' | grep -q harbor; then
    cd "$HOME/dev-repo/pipeline/harbor"
    docker-compose down --remove-orphans --volumes 2>/dev/null && echo "✅ Stopped Harbor containers"
    sleep 10
  fi
  
  # Remove Harbor compose directory
  [ -d "$HOME/dev-repo/pipeline/harbor" ] && rm -rf "$HOME/dev-repo/pipeline/harbor" && echo "✅ Removed Harbor compose directory"
  
  # Remove Harbor images
  # docker images | grep harbor | awk '{print $3}' | xargs -r docker rmi -f && echo "✅ Removed Harbor images"
}

remove_cloned_repositories() {
  echo "10. Removing cloned repositories..."
  
  # Remove dev-repo
  [ -d "$HOME/dev-repo" ] && sudo rm -rf "$HOME/dev-repo" && echo "✅ Removed dev-repo"
  
  # Remove symphony repo
  [ -d "$HOME/symphony" ] && sudo rm -rf "$HOME/symphony" && echo "✅ Removed symphony repository"
}

uninstall_rust() {
  echo "11. Uninstalling Rust..."
  
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
  echo "12. Uninstalling Docker and Docker Compose..."
  
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
  echo "13. Uninstalling Go..."
  
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
  echo "14. Final cleanup of basic utilities..."
  
  # Remove temporary files
  rm -f /tmp/go.tar.gz /tmp/resp.json /tmp/headers.txt get-docker.sh 2>/dev/null && echo "✅ Removed temporary files"
  
  # Clear exported variables
  unset GOGS_TOKEN GITHUB_USER GITHUB_TOKEN SYMPHONY_BRANCH DEV_REPO_BRANCH
  unset EXPOSED_HARBOR_IP EXPOSED_HARBOR_PORT EXPOSED_SYMPHONY_IP EXPOSED_SYMPHONY_PORT
  unset EXPOSED_KEYCLOAK_IP EXPOSED_KEYCLOAK_PORT EXPOSED_GOGS_IP EXPOSED_GOGS_PORT
  
  # Note: Not removing curl as it might be needed by system
  echo "⚠️ Basic utilities (curl) left installed as they may be system dependencies"
  
  echo "✅ Environment cleanup completed"
  echo ""
  echo "🔄 Please restart your shell or run 'source ~/.bashrc' to apply PATH changes"
}

download_nextcloud_container_images_from_external() {
  echo "Downloading Nextcloud container images from external repo..."
  
  # Pull images from external registries
  docker pull nextcloud:apache
  docker pull redis:alpine
  docker pull mariadb:10.5

  # Docker retag them to be pushed to harbor registry
  docker tag nextcloud:apache "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/nextcloud:apache"
  docker tag redis:alpine "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/redis:alpine"
  docker tag mariadb:10.5 "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/mariadb:10.5"

  # Docker login to the harbor registry
  echo "Logging into Harbor registry..."
  docker login "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}" -u admin -p Harbor12345

  # Docker push them to the harbor registry
  echo "Pushing Nextcloud images to Harbor..."
  docker push "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/nextcloud:apache"
  docker push "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/redis:alpine"
  docker push "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/mariadb:10.5"
  
  echo "✅ Nextcloud images successfully pushed to Harbor"
}

download_nginx_container_images_from_external() {
  echo "Downloading Nginx ingress controller images from external source..."
  
  # Pull images from registry.k8s.io
  docker pull registry.k8s.io/ingress-nginx/controller:v1.13.2
  docker pull registry.k8s.io/ingress-nginx/kube-webhook-certgen:v1.6.2
  docker pull registry.k8s.io/defaultbackend-amd64:1.5

  # Docker retag them to be pushed to harbor registry
  docker tag registry.k8s.io/ingress-nginx/controller:v1.13.2 "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/ingress-nginx-controller:v1.13.2"
  docker tag registry.k8s.io/ingress-nginx/kube-webhook-certgen:v1.6.2 "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/ingress-nginx-kube-webhook-certgen:v1.6.2"
  docker tag registry.k8s.io/defaultbackend-amd64:1.5 "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/defaultbackend-amd64:1.5"

  # Docker login to the harbor registry (if not already logged in)
  echo "Ensuring Harbor registry login..."
  docker login "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}" -u admin -p Harbor12345

  # Docker push them to the harbor registry
  echo "Pushing Nginx ingress images to Harbor..."
  docker push "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/ingress-nginx-controller:v1.13.2"
  docker push "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/ingress-nginx-kube-webhook-certgen:v1.6.2"
  docker push "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/defaultbackend-amd64:1.5"
  
  echo "✅ Nginx ingress images successfully pushed to Harbor"
}

download_otel_container_images_from_external() {
  echo "Downloading Otel images from external source..."
  
  # Pull images from registry.k8s.io
  docker pull ghcr.io/open-telemetry/demo:latest
  docker pull otel/opentelemetry-collector-contrib:latest
  docker pull ghcr.io/open-feature/flagd:v0.12.8
  docker pull valkey/valkey:7.2-alpine

  # Docker retag them to be pushed to harbor registry
  docker tag ghcr.io/open-telemetry/demo:latest "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/otel-demo:latest"
  docker tag otel/opentelemetry-collector-contrib:latest "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/otel-contrib:latest"
  docker tag ghcr.io/open-feature/flagd:v0.12.8 "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/flagd:v0.12.8"  
  docker tag valkey/valkey:7.2-alpine "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/valkey:7.2-alpine"

  # Docker login to the harbor registry (if not already logged in)
  echo "Ensuring Harbor registry login..."
  docker login "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}" -u admin -p Harbor12345

  # Docker push them to the harbor registry
  echo "Pushing otel images to Harbor..."
  docker push "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/otel-demo:latest"
  docker push "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/otel-contrib:latest"
  docker push "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/flagd:v0.12.8"  
  docker push "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/valkey:7.2-alpine" 
  echo "✅ otel images successfully pushed to Harbor"
}

download_custom_otel_container_images_from_external() {
  echo "Downloading Custom Otel images from external source..."

  cd "$HOME/dev-repo/poc/tests/artefacts/custom-otel-helm-app/code/app"
  docker build . -t "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/custom-otel-app:latest"
  echo "Ensuring Harbor registry login..."
  docker login "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}" -u admin -p Harbor12345
  # Docker push them to the harbor registry
  echo "Pushing otel images to Harbor..."
  docker push "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/custom-otel-app:latest"

  echo "pushing the custom-otel-app-chart"
  cd "$HOME/dev-repo/poc/tests/artefacts/custom-otel-helm-app/code"
  helm package helm/
  helm push go-otel-service-0.1.0.tgz "oci://${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library" --plain-http
  echo "✅ custom otel images successfully pushed to Harbor"
}


# Optional: Cleanup function to remove local images after pushing
cleanup_local_images() {
  echo "Cleaning up local images..."
  
  # Remove original images
  docker rmi nextcloud:apache redis:alpine mariadb:10.5 2>/dev/null || true
  docker rmi registry.k8s.io/ingress-nginx/controller:v1.13.2 2>/dev/null || true
  docker rmi registry.k8s.io/ingress-nginx/kube-webhook-certgen:v1.6.2 2>/dev/null || true
  docker rmi registry.k8s.io/defaultbackend-amd64:1.5 2>/dev/null || true
  
  # Remove retagged images
  docker rmi "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/nextcloud:apache" 2>/dev/null || true
  docker rmi "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/redis:alpine" 2>/dev/null || true
  docker rmi "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/mariadb:10.5" 2>/dev/null || true
  docker rmi "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/ingress-nginx-controller:v1.13.2" 2>/dev/null || true
  docker rmi "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/ingress-nginx-kube-webhook-certgen:v1.6.2" 2>/dev/null || true
  docker rmi "${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}/library/defaultbackend-amd64:1.5" 2>/dev/null || true
  
  echo "✅ Local images cleaned up"
}

# Alternative simpler version without jq dependency
add_insecure_registry_to_daemon() {
  echo "Adding insecure registry to Docker daemon (simple version)..."
  
  local registry_url="${EXPOSED_HARBOR_IP}:${EXPOSED_HARBOR_PORT}"
  local daemon_config="/etc/docker/daemon.json"
  
  # Create Docker directory if it doesn't exist
  mkdir -p /etc/docker
  
  # Backup existing config if it exists
  [ -f "$daemon_config" ] && cp "$daemon_config" "$daemon_config.backup.$(date +%s)"
  
  # Create or update daemon.json
  tee "$daemon_config" > /dev/null <<EOF
{
  "insecure-registries": ["$registry_url"]
}
EOF
  
  echo "✅ Configured insecure registry: $registry_url"
  echo "Current daemon.json:"
  cat "$daemon_config"
  
  # Restart Docker daemon
  echo "Restarting Docker daemon..."
  systemctl restart docker
  
  # Wait for Docker to be ready
  for i in {1..30}; do
    if systemctl is-active --quiet docker; then
      echo "✅ Docker daemon restarted successfully"
      return 0
    fi
    echo "Waiting for Docker... ($i/30)"
    sleep 10
  done
  
  echo "❌ Docker daemon failed to restart properly"
  return 1
}



# ----------------------------
# Main Orchestration Functions
# ----------------------------
install_prerequisites() {
  echo "Running all pre-req setup tasks..."
  install_basic_utilities
  install_go
  install_docker_compose
  add_insecure_registry_to_daemon

  install_rust
  
  clone_symphony_repo
  clone_dev_repo
  
  
  setup_keycloak
  update_keycloak_config
  
  setup_harbor
  download_custom_otel_container_images_from_external
 
  setup_gogs_directories
  start_gogs
  wait_for_gogs
  create_gogs_admin
  create_gogs_token
  create_gogs_repositories
  push_custom_otel_files  
  echo "setup completed"
}

start_symphony() {
  echo "Starting Symphony API server on..."
  export PATH="$PATH:/usr/local/go/bin";
  # Build phase
  build_rust
  build_symphony_api_server
  build_maestro_cli
  verify_symphony_api
  start_symphony_api
  echo "symphony API server started"
}

stop_symphony() {
  echo "Stopping Symphony API on..."
  PID=$(ps -ef | grep '[s]ymphony-api-margo.json' | awk '{print $2}'); 
  if [ -z "$PID" ]; then 
    echo '❌ Symphony API is not running'; 
  else 
    kill -9 $PID && echo '✅ Symphony API stopped'; 
  fi
}


# Update the show_menu function to include uninstall option
show_menu() {
  echo "Choose an option:"
  echo "1) Prepare-Environment"
  echo "2) Symphony-Start"
  echo "3) Symphony-Stop"
  echo "4) Obeservabiliy Stack-Install"
  echo "5) Obeservabiliy Stack-Uninstall"
  echo "6) Tearup-Environment"
  read -p "Enter choice [1-6]: " choice
  case $choice in
    1) install_prerequisites ;;
    2) start_symphony ;;
    3) stop_symphony ;;
    4) observability_stack_install ;;
    5) observability_stack_uninstall ;;
    6) uninstall_prerequisites ;;
    *) echo "⚠️ Invalid choice"; exit 1 ;;
  esac
}


# ----------------------------
# Main Script Execution
# ----------------------------
# Update the main script execution section
if [[ -z "$1" ]]; then
  show_menu
else
  case "$1" in
    Prepare-Environment) install_prerequisites ;;
    Symphony-Start) start_symphony ;;
    Symphony-Stop) stop_symphony ;;
    Jaeger_Prometheus_Grafana_Loki-Installation) observability_stack_install ;;
    Jaeger_Prometheus_Grafana_Loki-Uninstallation) observability_stack_uninstall ;;
    Tearup-Environment) uninstall_prerequisites ;;
    *) echo "Usage: $0 {prepare-environment|symphony-start|symphony-stop|uninstall-prerequisites|observability_stack_install|observability_stack_uninstall}"; exit 1 ;;
  esac
fi

