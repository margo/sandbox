#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

echo "==============================================="
echo " Margo WFM Supplier Setup (Simple)"
echo "==============================================="

SPEC_URL="https://raw.githubusercontent.com/margo/specification/pre-draft/system-design/specification/margo-management-interface/workload-management-api-1.0.0.yaml"
SPEC_FILE="spec.yaml"
COLLECTION_FILE="postman_collection.json"
DATA_DIR="newman-data"
ENV_FILE="$DATA_DIR/device-agent.env.json"
ITERATION_FILE="$DATA_DIR/device-agent.iteration.json"
CERT_DIR="$DATA_DIR/certs"
LOCAL_CERT_DIR="$SCRIPT_DIR/certs"
LOCAL_CA_CERT_FILE="$LOCAL_CERT_DIR/ca-cert.pem"
RUNTIME_CA_CERT_FILE="$CERT_DIR/ca-cert.pem"
DEVICE_KEY_FILE="$CERT_DIR/device.key"
DEVICE_CERT_FILE="$CERT_DIR/device-cert.pem"
DEFAULT_DEPLOYMENT_ID="demo-deployment-001"

DEFAULT_BASE_URL="https://localhost:3001/v1alpha2/margo"
BASE_URL="${1:-}"
if [[ -z "$BASE_URL" ]]; then
    read -r -p "Enter WFM BASE_URL [$DEFAULT_BASE_URL]: " BASE_URL
    BASE_URL="${BASE_URL:-$DEFAULT_BASE_URL}"
fi
BASE_URL="${BASE_URL//v1aplha2/v1alpha2}"

ensure_cmd() {
    local cmd="$1"
    local hint="$2"
    if ! command -v "$cmd" >/dev/null 2>&1; then
        echo "❌ Missing required command: $cmd"
        echo "   Install hint: $hint"
        exit 1
    fi
}

ensure_npm_tool() {
    local tool="$1"
    local package="$2"
    if ! command -v "$tool" >/dev/null 2>&1; then
        echo "Installing $tool..."
        sudo npm install -g "$package"
    fi
}

ensure_cmd curl "sudo apt-get install -y curl"
ensure_cmd jq "sudo apt-get install -y jq"
ensure_cmd openssl "sudo apt-get install -y openssl"

if ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then
    echo "Installing Node.js + npm..."
    curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
    sudo apt-get install -y nodejs
fi

ensure_npm_tool portman @apideck/portman

mkdir -p "$DATA_DIR" "$CERT_DIR" "$LOCAL_CERT_DIR"

if [[ ! -f "$LOCAL_CA_CERT_FILE" ]]; then
    echo "❌ Missing WFM CA certificate: $LOCAL_CA_CERT_FILE"
    echo "   Copy the WFM CA certificate into this path before running setup."
    exit 1
fi

cp "$LOCAL_CA_CERT_FILE" "$RUNTIME_CA_CERT_FILE"

echo "Downloading OpenAPI spec..."
curl -fsSL "$SPEC_URL" -o "$SPEC_FILE"

echo "Generating Postman collection from OpenAPI..."
portman -l "$SPEC_FILE" -o "$COLLECTION_FILE" -b "$BASE_URL"

echo "Generating device keypair..."
DEVICE_ID="device-$(date +%s)"
openssl ecparam -name prime256v1 -genkey -noout -out "$DEVICE_KEY_FILE" >/dev/null 2>&1
openssl req -new -x509 -days 365 \
    -key "$DEVICE_KEY_FILE" \
    -out "$DEVICE_CERT_FILE" \
    -subj "/C=IN/ST=GGN/L=Sector48/O=Margo/OU=Conformance/CN=$DEVICE_ID" >/dev/null 2>&1

DEVICE_CERT_B64="$(base64 -w0 < "$DEVICE_CERT_FILE")"

ONBOARDING_PAYLOAD="$(jq -cn --arg cert "$DEVICE_CERT_B64" '{apiVersion:"onboarding.margo.org/v1alpha1",kind:"OnboardingRequest",certificate:$cert}')"
CAPABILITIES_PAYLOAD="$(jq -cn --arg id "$DEVICE_ID" '{apiVersion:"device.margo.org/v1alpha1",kind:"DeviceCapabilitiesManifest",properties:{id:$id,vendor:"Margo Vendor",modelNumber:"MARGO-MODEL-01",serialNumber:("SN-"+$id),roles:["Standalone Device"],resources:{cpu:{cores:4,architecture:"arm64"},memory:"8Gi",storage:"64Gi",interfaces:[{type:"ethernet"}],peripherals:[]}}}')"
CAPABILITIES_UPDATE_PAYLOAD="$(jq -cn --arg id "$DEVICE_ID" '{apiVersion:"device.margo.org/v1alpha1",kind:"DeviceCapabilitiesManifest",properties:{id:$id,vendor:"Margo Vendor",modelNumber:"MARGO-MODEL-01",serialNumber:("SN-"+$id),roles:["Standalone Device","Cluster Leader"],resources:{cpu:{cores:8,architecture:"amd64"},memory:"16Gi",storage:"128Gi",interfaces:[{type:"ethernet"},{type:"wifi"}],peripherals:[]}}}')"
STATUS_PAYLOAD="$(jq -cn --arg dep "$DEFAULT_DEPLOYMENT_ID" '{apiVersion:"deployment.margo.org/v1alpha1",kind:"DeploymentStatusManifest",deploymentId:$dep,components:[{name:"app-component-1",state:"installed"}],status:{state:"installed"}}')"

cat > "$ENV_FILE" <<EOF
{
  "id": "margo-wfm-supplier-env",
  "name": "Margo WFM Supplier",
  "values": [
    { "key": "baseUrl", "value": "$BASE_URL", "enabled": true },
    { "key": "deviceId", "value": "$DEVICE_ID", "enabled": true },
    { "key": "clientId", "value": "", "enabled": true },
    { "key": "certificate", "value": "$DEVICE_CERT_B64", "enabled": true },
    { "key": "onboardingRequest", "value": $(echo "$ONBOARDING_PAYLOAD" | jq -Rs .), "enabled": true },
    { "key": "capabilitiesRequest", "value": $(echo "$CAPABILITIES_PAYLOAD" | jq -Rs .), "enabled": true },
    { "key": "capabilitiesUpdateRequest", "value": $(echo "$CAPABILITIES_UPDATE_PAYLOAD" | jq -Rs .), "enabled": true },
    { "key": "statusRequest", "value": $(echo "$STATUS_PAYLOAD" | jq -Rs .), "enabled": true },
    { "key": "deploymentId", "value": "$DEFAULT_DEPLOYMENT_ID", "enabled": true },
    { "key": "manifestEtag", "value": "", "enabled": true }
  ]
}
EOF

echo '[]' > "$ITERATION_FILE"

echo "----------------------------------------"
echo "Setup complete"
echo "Collection:      $COLLECTION_FILE"
echo "Environment:     $ENV_FILE"
echo "Iteration data:  $ITERATION_FILE"
echo "WFM CA cert:     $RUNTIME_CA_CERT_FILE"
echo "Device cert:     $DEVICE_CERT_FILE"
echo "Next: run ./2-run_newman.sh"
echo "----------------------------------------"
