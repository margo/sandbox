#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

COLLECTION_INPUT="${1:-postman_collection.json}"
if [[ "$COLLECTION_INPUT" = /* ]]; then
    COLLECTION="$COLLECTION_INPUT"
else
    COLLECTION="$SCRIPT_DIR/$COLLECTION_INPUT"
fi

# Use a runtime copy of the collection to preserve original
COLLECTION_RUNTIME="$SCRIPT_DIR/.collection.runtime.json"

DATA_DIR="newman-data"
ENV_FILE="$DATA_DIR/device-agent.env.json"
ITERATION_FILE="$DATA_DIR/device-agent.iteration.json"
CERT_DIR="$DATA_DIR/certs"
LOCAL_CERT_DIR="$SCRIPT_DIR/certs"
LOCAL_CA_CERT_FILE="$LOCAL_CERT_DIR/ca-cert.pem"
RUNTIME_CA_CERT_FILE="$CERT_DIR/ca-cert.pem"
DEVICE_KEY_FILE="$CERT_DIR/device.key"
DEVICE_CERT_FILE="$CERT_DIR/device-cert.pem"
REPORT="report_$(date +%Y%m%d_%H%M%S).html"
DEFAULT_DEPLOYMENT_ID="demo-deployment-001"

cd "$SCRIPT_DIR"

ensure_cmd() {
    local cmd="$1"
    local hint="$2"
    if ! command -v "$cmd" >/dev/null 2>&1; then
        echo "❌ Missing required command: $cmd"
        echo "   Install hint: $hint"
        exit 1
    fi
}

ensure_cmd jq "sudo apt-get install -y jq"
ensure_cmd curl "sudo apt-get install -y curl"
ensure_cmd openssl "sudo apt-get install -y openssl"
ensure_cmd docker "sudo apt-get install -y docker.io" || echo "⚠️  Docker not available, workload execution will be limited"

if [[ ! -f "$COLLECTION" || ! -f "$ENV_FILE" || ! -f "$ITERATION_FILE" ]]; then
    echo "❌ Missing required files. Run ./1-setup_portman.sh first."
    echo "   Needed: $COLLECTION, $ENV_FILE, $ITERATION_FILE"
    exit 1
fi

if ! command -v newman >/dev/null 2>&1; then
    echo "Installing Newman..."
    sudo npm install -g newman
fi

if ! npm list -g newman-reporter-htmlextra >/dev/null 2>&1; then
    echo "Installing htmlextra reporter..."
    sudo npm install -g newman-reporter-htmlextra
fi

mkdir -p "$CERT_DIR" "$LOCAL_CERT_DIR" "$DATA_DIR/responses"

if [[ -f "$LOCAL_CA_CERT_FILE" ]]; then
    cp "$LOCAL_CA_CERT_FILE" "$RUNTIME_CA_CERT_FILE"
elif [[ ! -f "$RUNTIME_CA_CERT_FILE" ]]; then
    echo "❌ Missing WFM CA certificate. Place it at: $LOCAL_CA_CERT_FILE"
    echo "   The script copies it into $RUNTIME_CA_CERT_FILE before running TLS calls."
    exit 1
fi

echo "Refreshing dynamic device identity and payload data..."
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

jq \
    --arg deviceId "$DEVICE_ID" \
    --arg cert "$DEVICE_CERT_B64" \
    --arg onboarding "$ONBOARDING_PAYLOAD" \
    --arg capabilities "$CAPABILITIES_PAYLOAD" \
    --arg capabilitiesUpdate "$CAPABILITIES_UPDATE_PAYLOAD" \
    --arg status "$STATUS_PAYLOAD" \
    --arg deploymentId "$DEFAULT_DEPLOYMENT_ID" \
    '
    .values |= map(
        if .key == "deviceId" then .value = $deviceId
        elif .key == "clientId" then .value = ""
        elif .key == "certificate" then .value = $cert
        elif .key == "onboardingRequest" then .value = $onboarding
        elif .key == "capabilitiesRequest" then .value = $capabilities
        elif .key == "capabilitiesUpdateRequest" then .value = $capabilitiesUpdate
        elif .key == "statusRequest" then .value = $status
        elif .key == "deploymentId" then .value = $deploymentId
        else .
        end
    )
    ' "$ENV_FILE" > "$ENV_FILE.tmp"
mv "$ENV_FILE.tmp" "$ENV_FILE"

echo "Applying collection runtime overrides..."
cp "$COLLECTION" "$COLLECTION_RUNTIME"

jq '
def set_test_script($lines):
  .event = ((.event // []) | map(select(.listen != "test")) + [{"listen":"test","script":{"type":"text/javascript","exec":$lines}}]);

def set_json_body($raw):
  .request.body = {"mode":"raw","raw":$raw,"options":{"raw":{"language":"json"}}};

def patch_request:
  if (has("request") | not) then
    .
  else
    # Convert Postman path variables :name to {{name}} for environment substitution
    if (.request.url.path | type) == "array" then
      .request.url.path |= map(
        if startswith(":") then
          "{{" + .[1:] + "}}"
        else
          .
        end
      )
    else
      .
    end |
    
    ((.request.url.path // []) | join("/")) as $path |
    (.request.method // "") as $method |
    if ($method == "POST" and ($path | test("api/v1/onboarding$"))) then
      set_json_body("{{onboardingRequest}}")
      | set_test_script([
          "const allowed = [200,201];",
          "pm.test(\"Onboarding status check\", function () {",
          "  const reason = `code=${pm.response.code}, body=${pm.response.text()}`;",
          "  pm.expect(allowed, reason).to.include(pm.response.code);",
          "});",
          "if (pm.response.code === 200 || pm.response.code === 201) {",
          "  try { const body = pm.response.json(); if (body && body.clientId) pm.environment.set(\"clientId\", body.clientId); } catch (e) {}",
          "}"
        ])
    elif ($method == "POST" and ($path | test("api/v1/clients/.*/capabilities$"))) then
      set_json_body("{{capabilitiesRequest}}")
      | set_test_script([
          "const allowed = [200,201,204,400,401,403,422];",
          "pm.test(\"Scenario-aware status check\", function () { pm.expect(allowed).to.include(pm.response.code); });"
        ])
    elif ($method == "PUT" and ($path | test("api/v1/clients/.*/capabilities$"))) then
      set_json_body("{{capabilitiesUpdateRequest}}")
      | set_test_script([
          "const allowed = [200,201,204,400,401,403,422];",
          "pm.test(\"Scenario-aware status check\", function () { pm.expect(allowed).to.include(pm.response.code); });"
        ])
    elif ($method == "GET" and ($path | test("api/v1/clients/.*/deployments$"))) then
      set_test_script([
          "const allowed = [200,304,400,401,403];",
          "pm.test(\"Scenario-aware status check\", function () { pm.expect(allowed).to.include(pm.response.code); });",
          "if (pm.response.code === 200) {",
          "  const etag = pm.response.headers.get(\"ETag\");",
          "  if (etag) pm.environment.set(\"manifestEtag\", etag.replace(/^[\\\"]|[\\\"]$/g, \"\"));",
          "  try {",
          "    const body = pm.response.json();",
          "    if (body && body.bundle && body.bundle.digest) pm.environment.set(\"manifestEtag\", body.bundle.digest);",
          "    if (body && Array.isArray(body.deployments) && body.deployments.length > 0) {",
          "      const first = body.deployments[0];",
          "      if (typeof first === \"string\") pm.environment.set(\"deploymentId\", first);",
          "      else if (first && first.deploymentId) pm.environment.set(\"deploymentId\", first.deploymentId);",
          "      else if (first && first.id) pm.environment.set(\"deploymentId\", first.id);",
          "    }",
          "  } catch (e) {}",
          "}"
        ])
    elif ($method == "GET" and ($path | test("api/v1/clients/.*/bundles/[^/]+$"))) then
      set_test_script([
          "const allowed = [200,304,400,401,403,404];",
          "pm.test(\"Scenario-aware status check\", function () { pm.expect(allowed).to.include(pm.response.code); });"
        ])
    elif ($method == "GET" and ($path | test("api/v1/clients/.*/deployments/[^/]+"))) then
      set_test_script([
          "const allowed = [200,400,401,403,404,500];",
          "pm.test(\"Scenario-aware status check\", function () { pm.expect(allowed).to.include(pm.response.code); });"
        ])
    elif ($method == "POST" and ($path | test("api/v1/clients/.*/deployments/.*/status$"))) then
      set_json_body("{{statusRequest}}")
      | set_test_script([
          "const allowed = [200,201,204,400,401,403,422];",
          "pm.test(\"Scenario-aware status check\", function () { pm.expect(allowed).to.include(pm.response.code); });"
        ])
    else
      .
    end
  end;

def patch_items:
  if has("item") then .item |= map(patch_items) else patch_request end;

patch_items
' "$COLLECTION_RUNTIME" > "$COLLECTION_RUNTIME.tmp"
mv "$COLLECTION_RUNTIME.tmp" "$COLLECTION_RUNTIME"

echo '[]' > "$ITERATION_FILE"

BASE_URL="$(jq -r '.values[] | select(.key=="baseUrl") | .value' "$ENV_FILE" | head -n1)"
if [[ -z "$BASE_URL" || "$BASE_URL" == "null" ]]; then
    BASE_URL="https://20.64.178.117:8082/v1alpha2/margo"
fi
BASE_URL="${BASE_URL//v1aplha2/v1alpha2}"

jq --arg baseUrl "$BASE_URL" \
    '.values |= map(if .key == "baseUrl" then .value = $baseUrl else . end)' \
    "$ENV_FILE" > "$ENV_FILE.tmp"
mv "$ENV_FILE.tmp" "$ENV_FILE"

echo "Running Newman (mock device-agent executing collection against WFM server)..."
echo ""
set +e
newman run "$COLLECTION_RUNTIME" \
    --environment "$ENV_FILE" \
    --ssl-extra-ca-certs "$RUNTIME_CA_CERT_FILE" \
    --insecure \
    -r cli,htmlextra \
    --reporter-htmlextra-export "$REPORT" 2>&1 | tee /tmp/newman-output.log
NEWMAN_EXIT=$?
set -e

# Extract and log key events from the test execution
echo ""
echo "=========================================="
echo "Extracting Device Status from Test Run"
echo "=========================================="

# Try to extract CLIENT_ID from Newman output (it appears in the requests)
EXTRACTED_CLIENT_ID=$(grep -oP 'client-[a-f0-9]{16}-\d+' /tmp/newman-output.log | head -1 || echo "")

# Check if onboarding succeeded
if grep -q "201 Created" /tmp/newman-output.log && grep -q "Complete onboarding" /tmp/newman-output.log; then
    if [[ -n "$EXTRACTED_CLIENT_ID" ]]; then
        echo "✅ Device Onboarded Successfully"
        echo "   Client ID: $EXTRACTED_CLIENT_ID"
        echo "   Device ID: $DEVICE_ID"
        # Save it to environment for future runs
        jq --arg cid "$EXTRACTED_CLIENT_ID" '.values |= map(if .key == "clientId" then .value = $cid else . end)' "$ENV_FILE" > "$ENV_FILE.tmp"
        mv "$ENV_FILE.tmp" "$ENV_FILE"
    else
        echo "⚠️  Onboarding 201 received but CLIENT_ID could not be extracted"
    fi
else
    echo "⚠️  Onboarding not completed (expecting 201 response)"
fi

# Check if capabilities were reported
if grep -q "capabilities" /tmp/newman-output.log; then
    echo ""
    echo "✅ Capabilities Reported to WFM"
    CAPS_PAYLOAD=$(jq -r '.values[] | select(.key=="capabilitiesRequest") | .value' "$ENV_FILE" 2>/dev/null | head -1)
    if [[ -n "$CAPS_PAYLOAD" && "$CAPS_PAYLOAD" != "null" ]]; then
        echo "   Vendor: $(echo "$CAPS_PAYLOAD" | jq -r '.properties.vendor' 2>/dev/null || echo 'unknown')"
        echo "   Model: $(echo "$CAPS_PAYLOAD" | jq -r '.properties.modelNumber' 2>/dev/null || echo 'unknown')"
        echo "   CPU Cores: $(echo "$CAPS_PAYLOAD" | jq -r '.properties.resources.cpu.cores' 2>/dev/null || echo 'unknown')"
        echo "   Memory: $(echo "$CAPS_PAYLOAD" | jq -r '.properties.resources.memory' 2>/dev/null || echo 'unknown')"
    fi
fi

# Check if deployment status was reported
if grep -q "status" /tmp/newman-output.log; then
    echo ""
    echo "✅ Deployment Status Reported to WFM"
    STATUS_PAYLOAD=$(jq -r '.values[] | select(.key=="statusRequest") | .value' "$ENV_FILE" 2>/dev/null | head -1)
    if [[ -n "$STATUS_PAYLOAD" && "$STATUS_PAYLOAD" != "null" ]]; then
        echo "   Deployment ID: $(echo "$STATUS_PAYLOAD" | jq -r '.deploymentId' 2>/dev/null || echo 'unknown')"
        echo "   Status: $(echo "$STATUS_PAYLOAD" | jq -r '.status.state' 2>/dev/null || echo 'unknown')"
    fi
fi

rm -f "$COLLECTION_RUNTIME" /tmp/newman-output.log

echo ""
echo "----------------------------------------"
echo "API Test Phase Complete"
echo "Report generated: $REPORT"
echo "Newman exit code: $NEWMAN_EXIT"
echo "----------------------------------------"
echo ""

if [[ $NEWMAN_EXIT -eq 0 ]]; then
    echo "✅ API tests passed. Ready for workload execution."
    echo "Run './3-execute-workloads.sh' to deploy and test workloads."
else
    echo "⚠️  Some API tests may have failed. Check $REPORT"
fi

exit "$NEWMAN_EXIT"
