#!/bin/bash
# modules/device/agent.sh - Device agent management functions

source "$(dirname "${BASH_SOURCE[0]}")/../lib/common.sh"

# ----------------------------
# GHCR Image References
# ----------------------------
GHCR_REGISTRY="ghcr.io"
GHCR_ORG="margo"
workload_Fleet_Management_Client_IMAGE="margo.org/workload-fleet-management-client"
workload_Fleet_Management_Client_IMAGE_TAG="latest"
workload_Fleet_Management_Client_IMAGE_REF="${workload_Fleet_Management_Client_IMAGE_REF:-${GHCR_REGISTRY}/${GHCR_ORG}/${workload_Fleet_Management_Client_IMAGE}:${workload_Fleet_Management_Client_IMAGE_TAG}}"

# ----------------------------
# Configuration Functions
# ----------------------------
update_agent_sbi_url() {
  echo 'Updating wfm.sbiUrl in workload-fleet-management-client config ...'
  sed -i "s|sbiUrl:.*|sbiUrl: https://$WFM_HOST:$WFM_PORT/v1alpha2/margo|" "$HOME/sandbox/poc/device/agent/config/config.yaml"
}

set_capabilities_roles() {
  local role="$1"
  local file="$HOME/sandbox/poc/device/agent/config/capabilities.json"

  if [[ -f "$file" ]]; then
    sed -i -E "s|\"roles\"[[:space:]]*:[[:space:]]*\[[^]]*\]|\"roles\": [\"$role\"]|g" "$file"
    echo "capabilities.json roles set to [$role]"
  else
    echo "capabilities.json not found at $file, skipping role update"
  fi
}

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

  sed -i \
    -e 's|pubCertPath:.*|pubCertPath: /certs/device-public.crt|' \
    -e 's|path: "./config/device-private.key"|path: "/certs/device-private.key"|' \
    -e 's|path: "./config/ca-cert.pem"|path: "/certs/ca-cert.pem"|' \
    "$CONFIG_FILE"

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
# Build Functions
# ----------------------------
# Update build_device_agent_docker()
build_device_agent_docker() {
  cd "$HOME/sandbox"
  
  # CI mode: use locally built image
  if [[ "${CI:-false}" == "true" ]]; then
    echo "🔧 CI mode: Using locally built image"
    export workload_Fleet_Management_Client_IMAGE_REF="workload-fleet-management-client:ci-test"
    
    if docker images --format "{{.Repository}}:{{.Tag}}" | grep -q "^${workload_Fleet_Management_Client_IMAGE_REF}$"; then
      echo "✅ Local CI image found: ${workload_Fleet_Management_Client_IMAGE_REF}"
      return 0
    else
      echo "❌ Local CI image not found: ${workload_Fleet_Management_Client_IMAGE_REF}"
      return 1
    fi
  fi
  
  # Production mode: pull from GHCR
  echo 'Checking if workload-fleet-management-client image already exists in GHCR...'
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
# Docker Service Functions
# ----------------------------
create_device_agent_systemd_service() {
  echo "🔧 Creating systemd service for device-agent auto-start..."

  local compose_dir="$HOME/sandbox/docker-compose"

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

  set_capabilities_roles "Standalone Device"

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

  # Export the image reference for docker-compose
  export workload_Fleet_Management_Client_IMAGE_REF="${workload_Fleet_Management_Client_IMAGE_REF}"
  echo "Using image: ${workload_Fleet_Management_Client_IMAGE_REF}"
  

  docker compose up -d

  create_device_agent_systemd_service
}

stop_device_agent_service_docker() {
  cd "$HOME/sandbox/docker-compose"
  docker compose down

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

# ----------------------------
# Kubernetes Service Functions
# ----------------------------
build_start_device_agent_k3s_service() {
    cd "$HOME/sandbox"
    echo "Deploying workload-fleet-management-client on Kubernetes..."

    
    # Determine image reference based on CI mode
    if [[ "${CI:-false}" == "true" ]]; then
      echo "🔧 CI mode: Using locally built image"
      export workload_Fleet_Management_Client_IMAGE_REF="workload-fleet-management-client:ci-test"
      
      # Verify image exists in K3s containerd
      if sudo crictl images | grep -q "workload-fleet-management-client.*ci-test"; then
        echo "✅ CI image found in K3s containerd"
      else
        echo "⚠️ CI image not found in containerd, attempting import..."
        if docker images --format "{{.Repository}}:{{.Tag}}" | grep -q "workload-fleet-management-client:ci-test"; then
          docker save workload-fleet-management-client:ci-test | sudo k3s ctr images import -
          echo "✅ Image imported into K3s"
        else
          echo "❌ CI image not found in Docker either"
          return 1
        fi
      fi
    else
      # Original GHCR logic

    echo "Importing image into k3s..."
    
    if command -v crictl >/dev/null 2>&1; then
        echo "Using crictl to pull image directly into k3s..."
        sudo crictl pull "${workload_Fleet_Management_Client_IMAGE_REF}"
      
        if [ $? -eq 0 ]; then
            echo "✅ Image imported into k3s cluster via crictl"
        else
            echo "❌ Failed to import image via crictl"
            return 1
        fi
    else
        echo "⚠️  crictl not found, skipping import step"
        echo "ℹ️  K3s will pull image directly from GHCR during Helm deployment"
    fi
  fi
    cd helmchart
    if [ $? -ne 0 ]; then
      echo "❌ Failed to navigate to helmchart directory"
      return 1
    fi

    set_capabilities_roles "Standalone Cluster"

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

    echo "Cleaning up any existing resources..."
    kubectl delete clusterrole workload-fleet-management-client-role 2>/dev/null || true
    kubectl delete clusterrolebinding workload-fleet-management-client-binding 2>/dev/null || true

    helm uninstall workload-fleet-management-client -n default 2>/dev/null || true
    sleep 5

    echo "Installing workload-fleet-management-client with persistent storage..."
    
     helm install workload-fleet-management-client . \
        --set image.repository=$(echo "${workload_Fleet_Management_Client_IMAGE_REF}" | cut -d: -f1) \
        --set image.tag=$(echo "${workload_Fleet_Management_Client_IMAGE_REF}" | cut -d: -f2) \
        --set image.pullPolicy=IfNotPresent \
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

    echo ""
    echo "Deployment Summary:"
    kubectl get pods -n default | grep workload-fleet-management-client
    kubectl get serviceaccount -n default | grep workload-fleet-management-client
    kubectl get pvc -n default | grep workload-fleet-management-client
}

stop_device_agent_kubernetes() {
  echo "Stopping workload-fleet-management-client..."
  cd "$HOME/sandbox"

  DELETE_PVC=false
  read -p "Delete persistent data (PVC)? This will require re-onboarding. [y/N]: " -n 1 -r
  echo
  if [[ $REPLY =~ ^[Yy]$ ]]; then
    DELETE_PVC=true
    echo "⚠️ PVC will be deleted after uninstalling Helm release"
  else
    echo "ℹ️ Attempting to preserve PVC..."

    if kubectl get pvc workload-fleet-management-client-data -n default >/dev/null 2>&1; then
      kubectl annotate pvc workload-fleet-management-client-data -n default \
        "helm.sh/resource-policy=keep" \
        --overwrite 2>/dev/null && echo "✅ PVC annotated for preservation" || echo "⚠️ Could not annotate PVC"
    else
      echo "⚠️ PVC not found, nothing to preserve"
    fi
  fi

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

  echo "Cleaning up ServiceAccount and RBAC resources..."
  kubectl delete serviceaccount workload-fleet-management-client-sa -n default 2>/dev/null || echo "No serviceaccount found"
  kubectl delete clusterrole workload-fleet-management-client-role 2>/dev/null || echo "No clusterrole found"
  kubectl delete clusterrolebinding workload-fleet-management-client-binding 2>/dev/null || echo "No clusterrolebinding found"

  echo "Cleaning up configmaps and secrets..."
  kubectl delete configmap workload-fleet-management-client-cm -n default 2>/dev/null || echo "No configmap found"
  kubectl delete secret workload-fleet-management-client-certs -n default 2>/dev/null || echo "No secret found"

  if [ "$DELETE_PVC" = true ]; then
    echo "Deleting PVC as requested..."
    kubectl delete pvc workload-fleet-management-client-data -n default 2>/dev/null || echo "No PVC found"
    echo "✅ PVC deleted - device will re-onboard on next start"
  else
    if kubectl get pvc workload-fleet-management-client-data -n default >/dev/null 2>&1; then
      echo "✅ PVC preserved successfully - device will resume with existing ID on next start"
      kubectl get pvc workload-fleet-management-client-data -n default

      kubectl annotate pvc workload-fleet-management-client-data -n default \
        "helm.sh/resource-policy-" \
        2>/dev/null || true
    else
      echo "⚠️ PVC was not preserved (may have been deleted by Helm)"
    fi
  fi

  echo ""
  echo "Verifying cleanup..."
  if kubectl get pods -n default 2>/dev/null | grep -q "workload-fleet-management-client"; then
    echo "⚠️ Some workload-fleet-management-client pods may still be terminating"
    kubectl get pods -n default | grep workload-fleet-management-client
  else
    echo "✅ All workload-fleet-management-client pods stopped"
  fi

  echo ""
  echo "✅ Device-workload-fleet-management-client cleanup complete"
}

cleanup_device_agent() {
  echo "Cleaning up workload-fleet-management-client files..."

  if docker ps -a --format "{{.Names}}" | grep -q "^workload-fleet-management-client$"; then
    echo "Stopping and removing workload-fleet-management-client container..."
    docker stop workload-fleet-management-client 2>/dev/null || true
    docker rm workload-fleet-management-client 2>/dev/null || true
    echo "Removed workload-fleet-management-client container"
  else
    echo "No workload-fleet-management-client container found"
  fi

  if helm list --short 2>/dev/null | grep -q "^workload-fleet-management-client$"; then
    echo "Uninstalling workload-fleet-management-client Helm release..."
    helm uninstall workload-fleet-management-client 2>/dev/null || true
    echo "Removed workload-fleet-management-client Helm release"
  else
    echo "No workload-fleet-management-client Helm release found"
  fi
}

show_status() {
  echo "Device's Workload Fleet Management Client Status:"
  echo "==================="

  if docker ps --format "{{.Names}}" | grep -q "^workload-fleet-management-client$"; then
    echo "✅ Device's Workload Fleet Management Client Docker Container is running."

    echo "Container Details:"
    docker ps --filter "name=workload-fleet-management-client" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}\t{{.Image}}"

    return 0
  fi

  if kubectl get pods -n default --no-headers 2>/dev/null | grep -q "workload-fleet-management-client"; then
    echo "✅ Device's Workload Fleet Management Client Kubernetes Pod is running."

    echo "Pod Details:"
    kubectl get pods -n default -o wide | grep -E "(NAME|workload-fleet-management-client)"

    echo "ServiceAccount Details:"
    kubectl get serviceaccount -n default | grep workload-fleet-management-client || echo "No ServiceAccount found"

    return 0
  fi

  echo "❌ Device's Workload Fleet Management Client is not running on Docker or Kubernetes."
}
