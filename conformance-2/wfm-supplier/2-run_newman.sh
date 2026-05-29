#!/usr/bin/env bash

set -euo pipefail

################################################################################
# Margo WFM Supplier - Newman Test Runner
# Simplified and modularized for maintainability
################################################################################

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# ============================================================================
# CONFIGURATION
# ============================================================================

COLLECTION_FILE="postman_collection.json"
COLLECTION_RUNTIME=".collection.runtime.json"
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
PROXY_HOST="127.0.0.1"
PROXY_PORT="${WFM_SIGNING_PROXY_PORT:-18082}"
PROXY_BASE_URL="http://${PROXY_HOST}:${PROXY_PORT}"
UPSTREAM_WFM_URL=""
PROXY_PID=""

# Collection patching control
# Set to "false" to skip automatic patching (e.g., for user-provided well-formed collections)
PATCH_COLLECTION="${PATCH_COLLECTION:-true}"

WFM_URL="${1:-}"

# ============================================================================
# FUNCTIONS
# ============================================================================

print_header() {
    echo ""
    echo "==============================================="
    echo " Margo WFM Supplier Test Runner"
    echo "==============================================="
    echo ""
}

print_step() {
    local step_num="$1"
    local step_name="$2"
    echo "Step $step_num: $step_name..."
}

error_exit() {
    local message="$1"
    echo "❌ $message"
    exit 1
}

check_command() {
    local cmd="$1"
    local install_hint="$2"
    
    if ! command -v "$cmd" >/dev/null 2>&1; then
        error_exit "Missing required command: $cmd\n   How to install: $install_hint"
    fi
}

install_npm() {
    local pkg="$1"
    
    if ! command -v "$pkg" >/dev/null 2>&1; then
        echo "  📦 Installing $pkg..."
        sudo npm install -g "$pkg" >/dev/null 2>&1 || true
    fi
}

validate_wfm_url() {
    if [[ -z "$WFM_URL" ]]; then
        if [[ -f "$ENV_FILE" ]]; then
            WFM_URL=$(jq -r '.values[] | select(.key=="baseUrl") | .value' "$ENV_FILE" 2>/dev/null || echo "")
        fi
    fi

    if [[ -z "$WFM_URL" ]]; then
        error_exit "WFM URL not provided.\n\nUsage: ./2-run_newman.sh <wfm-url>\n\nExample:\n  ./2-run_newman.sh https://localhost:3001/v1alpha2/margo\n  ./2-run_newman.sh https://symphony.machine:8082/v1alpha2/margo\n\nEnvironment Variables:\n  PATCH_COLLECTION=false   Skip collection patching (for user-provided collections)"
    fi

    # Normalize URL (fix typo: v1aplha2 -> v1alpha2)
    WFM_URL="${WFM_URL//v1aplha2/v1alpha2}"
    
    echo "WFM Server: $WFM_URL"
    
    if [[ "$PATCH_COLLECTION" == "true" ]]; then
        echo "Collection Patching: enabled (Portman-generated collection)"
    else
        echo "Collection Patching: disabled (user-provided collection)"
    fi
}

verify_setup() {
    print_step 1 "Verifying setup files"
    
    if [[ ! -f "$COLLECTION_FILE" ]]; then
        error_exit "Missing $COLLECTION_FILE. Run './1-setup_portman.sh' first."
    fi
    
    if [[ ! -f "$ENV_FILE" ]]; then
        error_exit "Missing $ENV_FILE. Run './1-setup_portman.sh' first."
    fi
    
    if [[ ! -f "$ITERATION_FILE" ]]; then
        error_exit "Missing $ITERATION_FILE. Run './1-setup_portman.sh' first."
    fi
    
    echo "✅ Setup files verified"
}

install_requirements() {
    print_step 2 "Installing system requirements"
    
    check_command jq "sudo apt-get install -y jq"
    check_command curl "sudo apt-get install -y curl"
    check_command openssl "sudo apt-get install -y openssl"
    check_command go "Install Go and ensure it is available on PATH"
    
    install_npm newman
    install_npm newman-reporter-htmlextra
    
    echo "✅ All requirements satisfied"
}

prepare_certificates() {
    print_step 3 "Preparing CA certificate"
    
    mkdir -p "$CERT_DIR"
    
    if [[ -f "$LOCAL_CA_CERT_FILE" ]]; then
        cp "$LOCAL_CA_CERT_FILE" "$RUNTIME_CA_CERT_FILE"
    elif [[ ! -f "$RUNTIME_CA_CERT_FILE" ]]; then
        error_exit "Missing WFM CA certificate\n   Copy to: $LOCAL_CA_CERT_FILE"
    fi
    
    echo "✅ CA certificate ready"
}

prepare_environment() {
    print_step 4 "Preparing test environment"

    local device_id="device-$(date +%s)-$$"
    openssl ecparam -name prime256v1 -genkey -noout -out "$DEVICE_KEY_FILE" >/dev/null 2>&1
    openssl req -new -x509 -days 365 \
        -key "$DEVICE_KEY_FILE" \
        -out "$DEVICE_CERT_FILE" \
        -subj "/C=IN/ST=GGN/L=Sector48/O=Margo/OU=Conformance/CN=$device_id" >/dev/null 2>&1

    local device_cert_b64
    device_cert_b64="$(base64 -w0 < "$DEVICE_CERT_FILE")"

    local onboarding_payload capabilities_payload capabilities_update_payload status_payload
    onboarding_payload="$(jq -cn --arg cert "$device_cert_b64" '{apiVersion:"onboarding.margo.org/v1alpha1",kind:"OnboardingRequest",certificate:$cert}')"
    capabilities_payload="$(jq -cn --arg id "$device_id" '{apiVersion:"device.margo.org/v1alpha1",kind:"DeviceCapabilitiesManifest",properties:{id:$id,vendor:"Margo Vendor",modelNumber:"MARGO-MODEL-01",serialNumber:("SN-"+$id),roles:["Standalone Device"],resources:{cpu:{cores:4,architecture:"arm64"},memory:"8Gi",storage:"64Gi",interfaces:[{type:"ethernet"}],peripherals:[]}}}')"
    capabilities_update_payload="$(jq -cn --arg id "$device_id" '{apiVersion:"device.margo.org/v1alpha1",kind:"DeviceCapabilitiesManifest",properties:{id:$id,vendor:"Margo Vendor",modelNumber:"MARGO-MODEL-01",serialNumber:("SN-"+$id),roles:["Standalone Device","Cluster Leader"],resources:{cpu:{cores:8,architecture:"amd64"},memory:"16Gi",storage:"128Gi",interfaces:[{type:"ethernet"},{type:"wifi"}],peripherals:[]}}}')"
    status_payload="$(jq -cn '{apiVersion:"deployment.margo.org/v1alpha1",kind:"DeploymentStatusManifest",deploymentId:"{{deploymentId}}",components:[{name:"app-component-1",state:"installed"}],status:{state:"installed"}}')"

    UPSTREAM_WFM_URL="$WFM_URL"
    WFM_URL="${PROXY_BASE_URL}$(python3 - <<PY2
from urllib.parse import urlparse
print(urlparse("$UPSTREAM_WFM_URL").path.rstrip("/"))
PY2
)"

    jq --arg baseUrl "$WFM_URL" \
       --arg deviceId "$device_id" \
       --arg certificate "$device_cert_b64" \
       --arg onboardingRequest "$onboarding_payload" \
       --arg capabilitiesRequest "$capabilities_payload" \
       --arg capabilitiesUpdateRequest "$capabilities_update_payload" \
       --arg statusRequest "$status_payload" \
       '.values |= map(
          if .key == "baseUrl" then .value = $baseUrl
          elif .key == "deviceId" then .value = $deviceId
          elif .key == "clientId" then .value = ""
          elif .key == "certificate" then .value = $certificate
          elif .key == "onboardingRequest" then .value = $onboardingRequest
          elif .key == "capabilitiesRequest" then .value = $capabilitiesRequest
          elif .key == "capabilitiesUpdateRequest" then .value = $capabilitiesUpdateRequest
          elif .key == "statusRequest" then .value = $statusRequest
          elif .key == "deploymentId" then .value = ""
          elif .key == "digest" then .value = ""
          elif .key == "bundleDigest" then .value = ""
          elif .key == "deploymentDigest" then .value = ""
          else . end
        )
        | if any(.values[]; .key == "digest") then . else .values += [{"key":"digest","value":"","enabled":true}] end
        | if any(.values[]; .key == "bundleDigest") then . else .values += [{"key":"bundleDigest","value":"","enabled":true}] end
        | if any(.values[]; .key == "deploymentDigest") then . else .values += [{"key":"deploymentDigest","value":"","enabled":true}] end' \
        "$ENV_FILE" > "$ENV_FILE.tmp"
    mv "$ENV_FILE.tmp" "$ENV_FILE"

    echo '[]' > "$ITERATION_FILE"

    echo "✅ Environment prepared"
    echo "   Real WFM URL: $UPSTREAM_WFM_URL"
    echo "   Newman proxy URL: $WFM_URL"
}

start_signing_proxy() {
    print_step 5 "Starting signing proxy"

    local repo_root
    repo_root="$(cd "$SCRIPT_DIR/../.." && pwd)"
    (cd "$repo_root" && go run "$SCRIPT_DIR/signing_proxy.go" \
        -listen "${PROXY_HOST}:${PROXY_PORT}" \
        -target "$UPSTREAM_WFM_URL" \
        -key "$SCRIPT_DIR/$DEVICE_KEY_FILE" \
        -insecure=true > /tmp/wfm-signing-proxy.log 2>&1) &
    PROXY_PID=$!

    for _ in {1..30}; do
        if curl -s "${PROXY_BASE_URL}/__proxy_not_found__" >/dev/null 2>&1; then
            echo "✅ Signing proxy started (PID: $PROXY_PID)"
            return 0
        fi
        if ! kill -0 "$PROXY_PID" 2>/dev/null; then
            cat /tmp/wfm-signing-proxy.log 2>/dev/null || true
            error_exit "Signing proxy failed to start"
        fi
        sleep 1
    done

    cat /tmp/wfm-signing-proxy.log 2>/dev/null || true
    error_exit "Signing proxy did not become ready"
}

patch_collection() {
    print_step 5 "Preparing Postman collection"
    
    cp "$COLLECTION_FILE" "$COLLECTION_RUNTIME"
    
    # Skip patching if disabled (e.g., for user-provided collections)
    if [[ "$PATCH_COLLECTION" != "true" ]]; then
        echo "⚠️  Skipping collection patches (patching disabled)"
        return 0
    fi
    
    jq '
    def set_json_body($raw):
      .request.body = {"mode":"raw","raw":$raw,"options":{"raw":{"language":"json"}}};

    def patch_url_variables:
      if (.request.url.variable | type) == "array" then
        .request.url.variable = []
      else . end;

    def set_test_script($exec):
      .event = ((.event // []) | map(select(.listen != "test"))) +
      [{"listen":"test","script":{"type":"text/javascript","exec":$exec}}];

    def test_success($name):
      set_test_script([
        "pm.test(\"" + $name + " returns a successful status\", function () {",
        "  pm.expect(pm.response.code).to.be.oneOf([200, 201, 202, 204]);",
        "});"
      ]);

    def test_onboarding:
      set_test_script([
        "pm.test(\"onboarding creates a client\", function () {",
        "  pm.expect(pm.response.code).to.eql(201);",
        "  const body = pm.response.json();",
        "  pm.expect(body.clientId).to.be.a(\"string\").and.not.empty;",
        "  pm.environment.set(\"clientId\", body.clientId);",
        "  [\"capabilitiesRequest\", \"capabilitiesUpdateRequest\"].forEach(function (key) {",
        "    const payload = JSON.parse(pm.environment.get(key));",
        "    payload.properties.id = body.clientId;",
        "    payload.properties.serialNumber = \"SN-\" + body.clientId;",
        "    pm.environment.set(key, JSON.stringify(payload));",
        "  });",
        "});"
      ]);

    def test_deployments:
      set_test_script([
        "pm.test(\"deployments returns desired state\", function () {",
        "  pm.expect(pm.response.code).to.eql(200);",
        "  const body = pm.response.json();",
        "  pm.expect(body).to.have.property(\"deployments\");",
        "  if (body.bundle && body.bundle.digest) { pm.environment.set(\"bundleDigest\", body.bundle.digest); pm.environment.set(\"digest\", body.bundle.digest); }",
        "  if (Array.isArray(body.deployments) && body.deployments.length > 0) {",
        "    pm.environment.set(\"deploymentId\", body.deployments[0].deploymentId);",
        "    pm.environment.set(\"deploymentDigest\", body.deployments[0].digest);",
        "  } else {",
        "    postman.setNextRequest(null);",
        "  }",
        "});"
      ]);

    def test_bundle:
      set_test_script([
        "pm.test(\"bundle download succeeds\", function () { pm.expect(pm.response.code).to.eql(200); });",
        "pm.environment.set('digest', pm.environment.get(\"deploymentDigest\") || \"\");"
      ]);

    def test_manifest:
      set_test_script([
        "pm.test(\"deployment manifest download succeeds\", function () { pm.expect(pm.response.code).to.eql(200); });"
      ]);

    def patch_request:
      if (has("request") | not) then .
      else
        patch_url_variables |
        if (.request.url.path | type) == "array" then
          .request.url.path |= map(if startswith(":") then "{{" + .[1:] + "}}" else . end)
        else . end |
        ((.request.url.path // []) | join("/")) as $path |
        (.request.method // "") as $method |
        if ($method == "GET" and ($path | test("api/v1/clients/.*/bundles/"))) then
          test_bundle
        elif ($method == "GET" and ($path | test("api/v1/clients/.*/deployments$"))) then
          test_deployments
        elif ($method == "GET" and ($path | test("api/v1/clients/.*/deployments/.*/"))) then
          test_manifest
        elif ($method == "POST" and ($path | test("api/v1/onboarding$"))) then
          set_json_body("{{onboardingRequest}}") | test_onboarding
        elif ($method == "POST" and ($path | test("api/v1/clients/.*/capabilities$"))) then
          set_json_body("{{capabilitiesRequest}}") | test_success("capabilities POST")
        elif ($method == "PUT" and ($path | test("api/v1/clients/.*/capabilities$"))) then
          set_json_body("{{capabilitiesUpdateRequest}}") | test_success("capabilities PUT")
        elif ($method == "POST" and ($path | test("api/v1/clients/.*/deployments/.*/status$"))) then
          set_json_body("{{statusRequest}}") | test_success("deployment status")
        else . end
      end;

    def patch_items:
      if has("item") then .item |= map(patch_items) else patch_request end;

    patch_items
    | .item |= ([.[] | select(.name == "Download Root CA certificate")]
        + [.[] | select(.name == "Complete onboarding with client certificate")]
        + [.[] | select(.name == "Report device capabilities")]
        + [.[] | select(.name == "Update device capabilities (Update)")]
        + [.[] | select(.name == "Retrieve the complete desired state for all workloads assigned to a device")]
        + [.[] | select(.name == "Retrieve bundle information for a specific device and digest")]
        + [.[] | select(.name == "Retrieve an individual ApplicationDeployment YAML file")]
        + [.[] | select(.name == "Report deployment status")]
        + [.[] | select((.name | IN("Download Root CA certificate", "Complete onboarding with client certificate", "Report device capabilities", "Update device capabilities (Update)", "Retrieve the complete desired state for all workloads assigned to a device", "Retrieve bundle information for a specific device and digest", "Retrieve an individual ApplicationDeployment YAML file", "Report deployment status")) | not)])
    ' "$COLLECTION_RUNTIME" > "$COLLECTION_RUNTIME.tmp"
    
    mv "$COLLECTION_RUNTIME.tmp" "$COLLECTION_RUNTIME"
    
    echo "✅ Collection prepared"
}

run_tests() {
    print_step 6 "Running conformance tests"
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
    
    echo ""
    return $NEWMAN_EXIT
}

cleanup() {
    if [[ -n "${PROXY_PID:-}" ]] && kill -0 "$PROXY_PID" 2>/dev/null; then
        kill "$PROXY_PID" 2>/dev/null || true
        wait "$PROXY_PID" 2>/dev/null || true
    fi
    rm -f "$COLLECTION_RUNTIME" /tmp/newman-output.log /tmp/wfm-signing-proxy.log
}

print_results() {
    local exit_code="$1"
    
    echo "==============================================="
    echo "✅ Test Execution Complete"
    echo "==============================================="
    echo ""
    echo "📊 Report: $REPORT"
    echo "⚙️  Environment: $ENV_FILE"
    echo ""
    
    if [[ $exit_code -eq 0 ]]; then
        echo "✅ All tests passed"
    else
        echo "⚠️  Some tests failed (exit code: $exit_code)"
        echo "   Check HTML report for details: $REPORT"
    fi
    
    echo ""
}

# ============================================================================
# MAIN EXECUTION
# ============================================================================

main() {
    print_header
    validate_wfm_url
    echo ""
    
    verify_setup
    install_requirements
    prepare_certificates
    prepare_environment
    start_signing_proxy
    patch_collection
    trap cleanup EXIT
    
    if run_tests; then
        RESULT=0
    else
        RESULT=$?
    fi
    
    cleanup
    trap - EXIT
    print_results $RESULT
    
    exit $RESULT
}

main "$@"

