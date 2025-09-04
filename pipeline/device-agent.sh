#!/bin/bash
set -e

if ! command -v sshpass >/dev/null 2>&1; then
  echo "sshpass not found. Installing..."
  sudo apt-get update
  sudo apt-get install -y sshpass
fi

# Load environment variables
if [ -f .env ]; then
  source .env
else
  echo ".env file not found!"
  exit 1
fi

vm_ssh() {
  local USER=$1
  local HOST=$2
  local PASS=$3
  local CMD=$4
  sshpass -p "$PASS" ssh -o StrictHostKeyChecking=no "$USER@$HOST" "$CMD"
}

# -------------------------
# Start Device Agent
# -------------------------
start_device_agent() {
  echo "Installing k3s and starting device-agent on VM2 ($VM2_HOST)..."

#Install go
  vm_ssh "$VM2_USER" "$VM2_HOST" "$VM2_PASS" \
  "set -e
   sudo apt update -y
   sudo snap install go --classic
   go version"

# Clone dev-repo on VM2
  vm_ssh "$VM2_USER" "$VM2_HOST" "$VM2_PASS" \
  "rm -rf ~/dev-repo; \
   git clone https://$GITHUB_USER:$GITHUB_TOKEN@github.com/margo/dev-repo.git; \
   cd dev-repo; git checkout dev-sprint-6-Automation"

# Update keycloak URL in symphony-api-margo.json
# vm_ssh "$VM1_USER" "$VM1_HOST" "$VM1_PASS" "
#   echo 'Updating keycloak URL in symphony-api-margo.json...'
#   sed -i 's|\"keycloakURL\": *\"http://[^\"]*\"|\"keycloakURL\": \"http://'"$VM_HOST"':8083\"|' ~/symphony/api/symphony-api-margo.json
# "

# Update kubeconfig path in device agent config
#vm_ssh "$VM2_USER" "$VM2_HOST" "$VM2_PASS" "
#  echo 'Updating kubeconfig path in device agent config...'
#  sed -i 's|kubeconfigPath:.*|kubeconfigPath: /home/azureuser/.kube/config|' ~/dev-repo/poc/device/agent/config/config.yaml
#"

# Update wfm.sbiUrl and capabilities.readFromFile in agent config
vm_ssh "$VM2_USER" "$VM2_HOST" "$VM2_PASS" "
  echo 'Updating wfm.sbiUrl in agent config...';
  sed -i \"s|sbiUrl:.*|sbiUrl: http://$VM1_HOST:8082/v1alpha2/margo/sbi/v1|\" \$HOME/dev-repo/poc/device/agent/config/config.yaml;
  echo 'Updating capabilities.readFromFile in agent config...';
  sed -i \"s|readFromFile:.*|readFromFile: \$HOME/dev-repo/poc/device/agent/config/capabilities.json|\" \$HOME/dev-repo/poc/device/agent/config/config.yaml;
  echo 'Config updates completed.'
"

# Install k3s and setup kubeconfig
vm_ssh "$VM2_USER" "$VM2_HOST" "$VM2_PASS" "
  set -e
  if command -v k3s >/dev/null 2>&1; then
    echo 'k3s already installed, skipping installation.'
    k3s --version
  else
    echo 'Installing k3s...'
    sudo apt update && sudo apt upgrade -y
    sudo apt install -y curl
    curl -sfL https://get.k3s.io | sh -
  fi

  # Ensure k3s is running
  sudo systemctl status k3s --no-pager || true
  sudo k3s kubectl get nodes || true

  # Setup kubeconfig
  mkdir -p \$HOME/.kube
  sudo cp /etc/rancher/k3s/k3s.yaml \$HOME/.kube/config
  sudo chown \$(id -u):\$(id -g) \$HOME/.kube/config
  export KUBECONFIG=\$HOME/.kube/config

  echo 'Kubeconfig setup complete.'
  kubectl get nodes || true
"

  # Build and run device-agent
  vm_ssh "$VM2_USER" "$VM2_HOST" "$VM2_PASS" "
    set -e
    export PATH=\$PATH:/usr/local/go/bin:/snap/bin

    cd ~/dev-repo/poc/device/agent/
    echo 'Building device-agent...'
    /usr/local/go/bin/go build -o device-agent &

    echo 'Waiting for device-agent binary to be created...'
    for i in {1..10}; do
      if [ -f device-agent ]; then
        echo 'device-agent binary found.'
        ls -lrt device-agent
        break
      fi
      echo 'device-agent not ready yet, retrying in 10s...'
      sleep 10
    done
    if [ ! -f device-agent ]; then
      echo 'device-agent binary was not created within timeout.'
      exit 1
    fi
    echo 'Starting device-agent...'
    cd ~/dev-repo
    nohup sudo ./poc/device/agent/device-agent --config poc/device/agent/config/config.yaml > \$HOME/device-agent.log 2>&1 &
   # nohup sudo ./poc/device/agent/device-agent --config poc/device/agent/config/config.yaml > $HOME/device-agent.log 2>&1 &
    echo \$! > \$HOME/device-agent.pid
    ps aux | grep '[d]evice-agent'
    sleep 40
    tail -n 50 \$HOME/device-agent.log
    echo 'setup completed and device-agent started'
  "
}


# -------------------------
# Stop Device Agent
# -------------------------
stop_device_agent() {
  echo "Stopping device-agent on VM2 ($VM2_HOST)..."

  vm_ssh "$VM2_USER" "$VM2_HOST" "$VM2_PASS" "
    if [ -f \$HOME/device-agent.pid ]; then
      kill \$(cat \$HOME/device-agent.pid) || true
      rm -f \$HOME/device-agent.pid
      echo 'device-agent stopped.'
    else
      echo 'No PID file found. device-agent might not be running.'
    fi
  "

  echo "Device Agent stopped"
}

# -------------------------
# User Interaction
# -------------------------
if [ -z "$1" ]; then
  echo "Choose an option:"
  echo "1) device-agent-start"
  echo "2) device-agent-stop"
  read -rp "Enter choice [1-2]: " choice
  case $choice in
    1) start_device_agent ;;
    2) stop_device_agent ;;
    *) echo "Invalid choice" ;;
  esac
else
  case $1 in
    start) start_device_agent ;;
    stop) stop_device_agent ;;
    *) echo "Usage: $0 {start|stop}" ;;
  esac
fi
