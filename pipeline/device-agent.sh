#!/bin/bash
set -e

# ----------------------------
# Environment & Validation Functions
# ----------------------------

#--- Github Settings to pull the code (can be overridden via env)
GITHUB_USER="${GITHUB_USER:-}"  # Set via env or leave empty
GITHUB_TOKEN="${GITHUB_TOKEN:-}"  # Set via env or leave empty

#--- branch details (can be overridden via env)
DEV_REPO_BRANCH="${DEV_REPO_BRANCH:-dev-sprint-6}"
WFM_IP="${WFM_IP:-127.0.0.1}"
WFM_PORT="${WFM_PORT:-8082}"

# variables for observability stack
NAMESPACE_OBSERVABILITY="observability"
PROMTAIL_RELEASE="promtail"
OTEL_RELEASE="otel-collector"

validate_required_vars() {
  local required_vars=("GITHUB_USER" "GITHUB_TOKEN" "DEV_REPO_BRANCH" "WFM_IP" "WFM_PORT")
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
  apt update && apt install curl -y
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
  echo 'Updating wfm.sbiUrl in agent config...'
  sed -i "s|sbiUrl:.*|sbiUrl: http://$WFM_IP:$WFM_PORT/v1alpha2/margo/sbi/v1|" "$HOME/dev-repo/poc/device/agent/config/config.yaml"
}

update_agent_kubepath() {
  echo 'Updating kubeconfigPath in agent config...'
  sed -i "s|kubeconfigPath:.*|kubeconfigPath: $HOME/.kube/config|" "$HOME/dev-repo/poc/device/agent/config/config.yaml"
}

update_agent_capabilities_path() {
  echo 'Updating capabilities.readFromFile in agent config...'
  sed -i "s|readFromFile:.*|readFromFile: $HOME/dev-repo/poc/device/agent/config/capabilities.json|" "$HOME/dev-repo/poc/device/agent/config/config.yaml"
}

update_agent_config() {
  update_agent_sbi_url
  update_agent_capabilities_path
  update_agent_kubepath
  echo 'Config updates completed.'
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

# ----------------------------
# Device Agent Build Functions
# ----------------------------
build_device_agent() {
  cd "$HOME/dev-repo/poc/device/agent/"
  echo 'Building device-agent...'
  go build -o device-agent
}

# ----------------------------
# Device Agent Service Functions
# ----------------------------
start_device_agent_service() {
  echo 'Starting device-agent...'
  cd "$HOME/dev-repo"
  nohup sudo ./poc/device/agent/device-agent --config poc/device/agent/config/config.yaml > "$HOME/device-agent.log" 2>&1 &
  echo $! > "$HOME/device-agent.pid"
}

verify_device_agent_running() {
  ps -eo user,pid,ppid,tty,time,cmd | grep '[d]evice-agent'
  sleep 10
  tail -n 50 "$HOME/device-agent.log"
}

stop_device_agent_service() {
  echo "Stopping device-agent..."
  
  if [ -f "$HOME/device-agent.pid" ]; then
    local pid=$(cat "$HOME/device-agent.pid")
    if kill "$pid" 2>/dev/null; then
      echo "device-agent stopped (PID: $pid)"
    else
      echo "Failed to stop device-agent with PID: $pid"
    fi
    rm -f "$HOME/device-agent.pid"
  else
    echo 'No PID file found. Attempting to find and kill device-agent processes...'
    pkill -f "device-agent" && echo "device-agent processes killed" || echo "No device-agent processes found"
  fi
}

cleanup_device_agent() {
  echo "Cleaning up device-agent files..."
  [ -f "$HOME/device-agent.log" ] && rm -f "$HOME/device-agent.log" && echo "Removed device-agent.log"
  [ -f "$HOME/dev-repo/poc/device/agent/device-agent" ] && rm -f "$HOME/dev-repo/poc/device/agent/device-agent" && echo "Removed device-agent binary"
}

# ----------------------------
# Main Orchestration Functions
# ----------------------------
start_device_agent() {
  echo "Installing k3s and starting device-agent ..."
  
  validate_required_vars
  
  install_go
  clone_dev_repo
  setup_k3s
  update_agent_config
  
  build_device_agent
  start_device_agent_service
  verify_device_agent_running
  
  echo 'setup completed and device-agent started'
}

stop_device_agent() {
  echo "Stopping device-agent on VM2 ($VM2_HOST)..."
  
  stop_device_agent_service
  cleanup_device_agent
  
  echo "Device Agent stopped"
}

show_status() {
  echo "Device Agent Status:"
  echo "==================="
  
  if [ -f "$HOME/device-agent.pid" ]; then
    local pid=$(cat "$HOME/device-agent.pid")
    if ps -p "$pid" > /dev/null 2>&1; then
      echo "✅ Device Agent is running (PID: $pid)"
      ps -p "$pid" -o pid,ppid,cmd --no-headers
    else
      echo "❌ Device Agent PID file exists but process is not running"
    fi
  else
    echo "❌ Device Agent PID file not found"
  fi
  
  # Check for any device-agent processes
  local processes=$(ps aux | grep '[d]evice-agent' | wc -l)
  if [ "$processes" -gt 0 ]; then
    echo "Found $processes device-agent process(es):"
    ps aux | grep '[d]evice-agent'
  fi
  
  # Show recent logs if available
  if [ -f "$HOME/device-agent.log" ]; then
    echo ""
    echo "Recent logs (last 10 lines):"
    tail -n 10 "$HOME/device-agent.log"
  fi
}

install_otel_collector_promtail() {
  echo "Installing OTEL Collector and Promtail..."
  cd "$HOME/dev-repo/pipeline/observability" || { echo '❌ observability dir missing'; exit 1; }
  



}

unnstall_otel_collector_promtail() {
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



show_menu() {
  echo "Choose an option:"
  echo "1) device-agent-start"
  echo "2) device-agent-stop"
  echo "3) device-agent-status"
  echo "4) otel-collector-promtail-installation"
  echo "5) otel-collector-promtail-uninstallation"
  read -rp "Enter choice [1-3]: " choice
  case $choice in
    1) start_device_agent ;;
    2) stop_device_agent ;;
    3) show_status ;;
    4) install_otel_collector_promtail ;;
    5) unnstall_otel_collector_promtail ;;
    *) echo "Invalid choice" ;;
  esac
}

# ----------------------------
# Main Script Execution
# ----------------------------
if [ -z "$1" ]; then
  show_menu
else
  case $1 in
    start) start_device_agent ;;
    stop) stop_device_agent ;;
    status) show_status ;;
    install_otel_collector_promtail) install_otel_collector_promtail ;;
    unnstall_otel_collector_promtail) unnstall_otel_collector_promtail ;;
    *) echo "Usage: $0 {start|stop|status|install_otel_collector_promtail|unnstall_otel_collector_promtail}" ;;
  esac
fi
