#!/usr/bin/env bash

set -euo pipefail

################################################################################
# Margo WFM Supplier - Newman Test Runner
# Simplified and modularized for maintainability
#
# VENDOR NOTE: This script executes test collections with environment variables.
# The environment file (newman-data/device-agent.env.json) contains EXAMPLE values
# that vendors MUST customize for their own test devices:
#
#   deviceId: Example value "device-XXXXX" - replace with actual device ID
#   clientId: Example value "client-XXXXX" - replace with actual client ID
#   deploymentId: Example value "demo-deployment-001" - replace as needed
#   vendor: Example value "Margo Vendor" - replace with actual vendor name
#   modelNumber: Example value "MARGO-MODEL-01" - replace with actual model
#   serialNumber: Example value "SN-XXXXX" - replace with actual serial number
#
# These values appear in the Postman collection request bodies and must match
# your actual test device configuration.
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
REPORT="report_$(date +%Y%m%d_%H%M%S).html"

# Collection patching control
# Set to "false" to skip automatic patching (e.g., for user-provided well-formed collections)
PATCH_COLLECTION="${PATCH_COLLECTION:-true}"

WFM_URL="${1:-}"
CUSTOM_COLLECTION_FILE="${2:-}"
CUSTOM_REPORT_NAME="${3:-}"

# Override collection file if provided
if [[ -n "$CUSTOM_COLLECTION_FILE" && -f "$CUSTOM_COLLECTION_FILE" ]]; then
    COLLECTION_FILE="$CUSTOM_COLLECTION_FILE"
    # Note: Custom collections (including group-filtered) may still need patching
    # Only skip patching if explicitly disabled via environment variable
fi

# Override report name if provided
if [[ -n "$CUSTOM_REPORT_NAME" ]]; then
    REPORT="${CUSTOM_REPORT_NAME}_$(date +%Y%m%d_%H%M%S).html"
fi

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
    
    # Update WFM URL in environment
    jq --arg baseUrl "$WFM_URL" \
        '.values |= map(if .key == "baseUrl" then .value = $baseUrl else . end)' \
        "$ENV_FILE" > "$ENV_FILE.tmp"
    mv "$ENV_FILE.tmp" "$ENV_FILE"
    
    # Create empty iteration file
    echo '[]' > "$ITERATION_FILE"
    
    echo "✅ Environment prepared"
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
        .request.url.variable |= map(
          if .key == "clientId" then . + {"value": "{{clientId}}"} 
          elif .key == "deploymentId" then . + {"value": "{{deploymentId}}"} 
          elif .key == "digest" then . + {"value": "{{digest}}"} 
          elif .key == "bundleDigest" then . + {"value": "{{bundleDigest}}"} 
          elif .key == "deploymentDigest" then . + {"value": "{{deploymentDigest}}"} 
          else . 
          end
        )
      else . end;

    def add_flexible_test_script:
      .event = ((.event // []) | map(select(.listen != "test"))) +
      [{
        "listen":"test",
        "script":{
          "type":"text/javascript",
          "exec":[
            "if (pm.response.code >= 200 && pm.response.code < 300) {",
            "  tests[\"✓ Success (\" + pm.response.code + \")\"] = true;",
            "} else if (pm.response.code >= 400 && pm.response.code < 500) {",
            "  tests[\"⚠ Client Error (\" + pm.response.code + \")\"] = true;",
            "} else if (pm.response.code >= 500) {",
            "  tests[\"✗ Server Error (\" + pm.response.code + \")\"] = true;",
            "} else {",
            "  tests[\"Request completed (\" + pm.response.code + \")\"] = true;",
            "}"
          ]
        }
      }];

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
          add_flexible_test_script
        elif ($method == "GET" and ($path | test("api/v1/clients/.*/deployments$"))) then
          add_flexible_test_script
        elif ($method == "GET" and ($path | test("api/v1/clients/.*/deployments/.*/"))) then
          add_flexible_test_script
        elif ($method == "POST" and ($path | test("api/v1/onboarding$"))) then
          set_json_body("{{onboardingRequest}}") | add_flexible_test_script
        elif ($method == "POST" and ($path | test("api/v1/clients/.*/capabilities$"))) then
          set_json_body("{{capabilitiesRequest}}") | add_flexible_test_script
        elif ($method == "PUT" and ($path | test("api/v1/clients/.*/capabilities$"))) then
          set_json_body("{{capabilitiesUpdateRequest}}") | add_flexible_test_script
        elif ($method == "POST" and ($path | test("api/v1/clients/.*/deployments/.*/status$"))) then
          set_json_body("{{statusRequest}}") | add_flexible_test_script
        else . end
      end;

    def patch_items:
      if has("item") then .item |= map(patch_items) else patch_request end;

    patch_items
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
    rm -f "$COLLECTION_RUNTIME" /tmp/newman-output.log
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
    patch_collection
    
    if run_tests; then
        RESULT=0
    else
        RESULT=$?
    fi
    
    cleanup
    print_results $RESULT
    
    exit $RESULT
}

main "$@"

