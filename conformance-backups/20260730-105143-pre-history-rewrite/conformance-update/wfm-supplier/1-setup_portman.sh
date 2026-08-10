#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

echo "==============================================="
echo " Margo WFM Supplier Test Setup"
echo "==============================================="
echo ""
echo "This script prepares test data needed to run"
echo "conformance tests against a WFM server."
echo ""
echo "Usage:"
echo "  ./1-setup_portman.sh                    # Uses default OpenAPI spec"
echo "  ./1-setup_portman.sh <spec-url>         # Uses custom OpenAPI spec URL"
echo ""

################################################################################
# Configuration
################################################################################

# OpenAPI Specification - can be provided as first argument or use default
DEFAULT_SPEC_URL="https://raw.githubusercontent.com/margo/specification/pre-draft/system-design/specification/margo-management-interface/workload-management-api-1.0.0.yaml"
SPEC_URL="${1:-$DEFAULT_SPEC_URL}"
SPEC_FILE="spec.yaml"
COLLECTION_FILE="postman_collection.json"

# Directory structure for test data
DATA_DIR="newman-data"
CERT_DIR="$DATA_DIR/certs"
LOCAL_CERT_DIR="$SCRIPT_DIR/certs"

# File paths
ENV_FILE="$DATA_DIR/device-agent.env.json"
ITERATION_FILE="$DATA_DIR/device-agent.iteration.json"
LOCAL_CA_CERT_FILE="$LOCAL_CERT_DIR/ca-cert.pem"
RUNTIME_CA_CERT_FILE="$CERT_DIR/ca-cert.pem"
DEVICE_KEY_FILE="$CERT_DIR/device.key"
DEVICE_CERT_FILE="$CERT_DIR/device-cert.pem"

# Test data defaults
DEFAULT_DEPLOYMENT_ID="demo-deployment-001"

################################################################################
# Helper Functions
################################################################################

# Check if a system command is installed (e.g., curl, jq, openssl)
install_system_command() {
    local command_name="$1"
    local install_hint="$2"
    
    if ! command -v "$command_name" >/dev/null 2>&1; then
        echo "❌ Missing required command: $command_name"
        echo "   How to install: $install_hint"
        exit 1
    fi
}

# Check if an npm package is installed globally, install if missing
install_npm_package() {
    local package_name="$1"
    local npm_package="$2"
    
    if ! command -v "$package_name" >/dev/null 2>&1; then
        echo "📦 Installing $package_name..."
        sudo npm install -g "$npm_package"
    fi
}

################################################################################
# Step 1: Check System Requirements
################################################################################

echo "Step 1: Checking system requirements..."

install_system_command curl "sudo apt-get install -y curl"
install_system_command jq "sudo apt-get install -y jq"
install_system_command openssl "sudo apt-get install -y openssl"

# Check if Node.js and npm are installed, install if missing
if ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then
    echo "📦 Installing Node.js and npm..."
    curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
    sudo apt-get install -y nodejs
fi

# Install Portman (converts OpenAPI specs to Postman collections)
install_npm_package portman @apideck/portman

echo "✅ All system requirements satisfied"
echo ""

################################################################################
# Step 2: Verify WFM CA Certificate
################################################################################

echo "Step 2: Verifying WFM CA certificate..."

if [[ ! -f "$LOCAL_CA_CERT_FILE" ]]; then
    echo "❌ Missing WFM CA certificate: $LOCAL_CA_CERT_FILE"
    echo ""
    echo "Please copy your WFM CA certificate to this location:"
    echo "  $LOCAL_CA_CERT_FILE"
    echo ""
    echo "You can get the CA certificate from your WFM server administrator."
    exit 1
fi

mkdir -p "$DATA_DIR" "$CERT_DIR"
cp "$LOCAL_CA_CERT_FILE" "$RUNTIME_CA_CERT_FILE"
echo "✅ WFM CA certificate copied"
echo ""

################################################################################
# Step 3: Download OpenAPI Specification
################################################################################

echo "Step 3: Downloading OpenAPI specification..."
if [[ "$SPEC_URL" == "$DEFAULT_SPEC_URL" ]]; then
    echo "   Using default Margo WFM specification"
else
    echo "   Using custom specification: $SPEC_URL"
fi
curl -fsSL "$SPEC_URL" -o "$SPEC_FILE"
echo "✅ Specification downloaded: $SPEC_FILE"
echo ""

################################################################################
# Step 4: Generate Postman Collection
################################################################################

echo "Step 4: Generating Postman collection from OpenAPI spec..."
echo "   (Collection is portable - server URL provided at runtime)"
portman -l "$SPEC_FILE" -o "$COLLECTION_FILE"
echo "✅ Collection generated: $COLLECTION_FILE"
echo ""

################################################################################
# Step 5: Generate Device Certificate and Keys
################################################################################

echo "Step 5: Generating device certificate and cryptographic keys..."
DEVICE_ID="device-$(date +%s)"

openssl ecparam -name prime256v1 -genkey -noout -out "$DEVICE_KEY_FILE" >/dev/null 2>&1
openssl req -new -x509 -days 365 \
    -key "$DEVICE_KEY_FILE" \
    -out "$DEVICE_CERT_FILE" \
    -subj "/C=IN/ST=GGN/L=Sector48/O=Margo/OU=Conformance/CN=$DEVICE_ID" >/dev/null 2>&1

echo "✅ Device certificate generated"
echo "✅ Cryptographic keys generated"
echo ""

################################################################################
# Step 6: Generate Test Data Payloads
################################################################################

echo "Step 6: Generating test data payloads..."

DEVICE_CERT_B64="$(base64 -w0 < "$DEVICE_CERT_FILE")"

# Create payloads for common test scenarios
ONBOARDING_PAYLOAD="$(jq -cn --arg cert "$DEVICE_CERT_B64" '{apiVersion:"onboarding.margo.org/v1alpha1",kind:"OnboardingRequest",certificate:$cert}')"
CAPABILITIES_PAYLOAD="$(jq -cn --arg id "$DEVICE_ID" '{apiVersion:"device.margo.org/v1alpha1",kind:"DeviceCapabilitiesManifest",properties:{id:$id,vendor:"Margo Vendor",modelNumber:"MARGO-MODEL-01",serialNumber:("SN-"+$id),roles:["Standalone Device"],resources:{cpu:{cores:4,architecture:"arm64"},memory:"8Gi",storage:"64Gi",interfaces:[{type:"ethernet"}],peripherals:[]}}}')"
CAPABILITIES_UPDATE_PAYLOAD="$(jq -cn --arg id "$DEVICE_ID" '{apiVersion:"device.margo.org/v1alpha1",kind:"DeviceCapabilitiesManifest",properties:{id:$id,vendor:"Margo Vendor",modelNumber:"MARGO-MODEL-01",serialNumber:("SN-"+$id),roles:["Standalone Device","Cluster Leader"],resources:{cpu:{cores:8,architecture:"amd64"},memory:"16Gi",storage:"128Gi",interfaces:[{type:"ethernet"},{type:"wifi"}],peripherals:[]}}}')"
STATUS_PAYLOAD="$(jq -cn --arg dep "$DEFAULT_DEPLOYMENT_ID" '{apiVersion:"deployment.margo.org/v1alpha1",kind:"DeploymentStatusManifest",deploymentId:$dep,components:[{name:"app-component-1",state:"installed"}],status:{state:"installed"}}')"

echo "✅ Test data payloads generated"
echo ""

################################################################################
# Step 7: Create Environment File for Newman
################################################################################

echo "Step 7: Creating test environment file..."
echo ""
echo "NOTE: You will be prompted for the WFM server URL below."
echo "      This is the server you want to run tests against."
echo "      You can change this later by editing: $ENV_FILE"
echo ""

DEFAULT_BASE_URL="https://localhost:3001/v1alpha2/margo"
BASE_URL="${1:-}"
if [[ -z "$BASE_URL" ]]; then
    read -r -p "Enter WFM Server Base URL [$DEFAULT_BASE_URL]: " BASE_URL
    BASE_URL="${BASE_URL:-$DEFAULT_BASE_URL}"
fi

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

echo "✅ Environment file created with server URL: $BASE_URL"
echo ""

################################################################################
# Step 8: Create Iteration Data File
################################################################################

echo '[]' > "$ITERATION_FILE"
echo "✅ Iteration data file created"
echo ""

################################################################################
# Summary
################################################################################

echo "==============================================="
echo "✅ Setup Complete!"
echo "==============================================="
echo ""
echo "Generated Files:"
echo "  📄 $COLLECTION_FILE"
echo "     → Postman test collection (portable, works with any WFM server)"
echo ""
echo "  ⚙️  $ENV_FILE"
echo "     → Test environment variables"
echo "     → WFM Server URL: $BASE_URL"
echo "     → Device ID: $DEVICE_ID"
echo ""
echo "  🔑 $DEVICE_CERT_FILE"
echo "     → Device certificate for signing requests"
echo ""
echo "  📝 $ITERATION_FILE"
echo "     → Test iteration data"
echo ""
echo "Next Steps:"
echo "  1. Verify your WFM server is running and accessible at:"
echo "     $BASE_URL"
echo ""
echo "  2. Run the tests:"
echo "     ./2-run_newman.sh"
echo ""
echo "==============================================="
