#!/bin/bash

# Comprehensive Mock WFM Server Test Suite
# Tests positive and negative scenarios for all endpoints

set -e

BASE_URL="http://localhost:8090/v1alpha2/margo"
DEVICE_ID="device-001"
DEPLOYMENT_ID="deployment-123"
DIGEST="abc123def456"
TMP_DIR=$(mktemp -d)

cleanup() {
    rm -rf "$TMP_DIR"
}

trap cleanup EXIT

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Helper function to print test headers
print_test() {
    local test_name="$1"
    local test_type="$2"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Test #$TOTAL_TESTS: $test_name${NC}"
    echo -e "${BLUE}Type: $test_type${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

# Helper function to check response
check_response() {
    local response="$1"
    local expected_status="$2"
    local test_name="$3"
    
    # Extract HTTP status code
    local status_code=$(echo "$response" | sed -n 's/^HTTP\/[0-9.]* \([0-9][0-9][0-9]\).*$/\1/p' | head -1)
    
    if [ "$status_code" = "$expected_status" ]; then
        echo -e "${GREEN}✅ PASSED${NC} - Status: $status_code (Expected: $expected_status)"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo -e "${RED}❌ FAILED${NC} - Status: $status_code (Expected: $expected_status)"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        echo "Response: $response"
    fi

    return 0
}

check_status_code() {
    local status_code="$1"
    local expected_status="$2"
    local context="$3"

    if [ "$status_code" = "$expected_status" ]; then
        echo -e "${GREEN}✅ PASSED${NC} - Status: $status_code (Expected: $expected_status)"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo -e "${RED}❌ FAILED${NC} - Status: $status_code (Expected: $expected_status)"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        echo "Context: $context"
    fi

    return 0
}

check_status_in_set() {
    local status_code="$1"
    local context="$2"
    shift 2

    local expected=("$@")
    local joined_expected="${expected[*]}"
    local match=0
    local candidate
    for candidate in "${expected[@]}"; do
        if [ "$status_code" = "$candidate" ]; then
            match=1
            break
        fi
    done

    if [ "$match" -eq 1 ]; then
        echo -e "${GREEN}✅ PASSED${NC} - Status: $status_code (Expected one of: $joined_expected)"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo -e "${RED}❌ FAILED${NC} - Status: $status_code (Expected one of: $joined_expected)"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        echo "Context: $context"
    fi

    return 0
}

check_not_empty() {
    local value="$1"
    local description="$2"

    if [ -n "$value" ]; then
        echo -e "${GREEN}✅ PASSED${NC} - $description"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo -e "${RED}❌ FAILED${NC} - $description"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi

    return 0
}

check_equal() {
    local actual="$1"
    local expected="$2"
    local description="$3"

    if [ "$actual" = "$expected" ]; then
        echo -e "${GREEN}✅ PASSED${NC} - $description"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo -e "${RED}❌ FAILED${NC} - $description"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        echo "Actual:   $actual"
        echo "Expected: $expected"
    fi

    return 0
}

extract_header_value() {
    local headers_file="$1"
    local header_name="$2"

    awk -v key="$header_name" '
        BEGIN { IGNORECASE = 1 }
        index($0, ":") > 0 {
            name = substr($0, 1, index($0, ":") - 1)
            if (tolower(name) == tolower(key)) {
                value = substr($0, index($0, ":") + 1)
                sub(/^[[:space:]]+/, "", value)
                sub(/\r$/, "", value)
                print value
                exit
            }
        }
    ' "$headers_file"
}

echo ""
echo -e "${YELLOW}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${YELLOW}║  Mock WFM Server - Comprehensive Test Suite                ║${NC}"
echo -e "${YELLOW}║  Base URL: $BASE_URL${NC}"
echo -e "${YELLOW}╚════════════════════════════════════════════════════════════╝${NC}"

# ============================================================================
# 1. ONBOARDING CERTIFICATE TESTS
# ============================================================================

echo ""
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}1. ONBOARDING CERTIFICATE TESTS${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"

# Positive: Valid certificate request
print_test "GET certificate - Valid request" "POSITIVE"
RESPONSE=$(curl -s -i -X GET "$BASE_URL/api/v1/onboarding/certificate" \
  -H "X-Payload-Signature: valid-signature")
check_response "$RESPONSE" "200" "Valid certificate request"

# Positive: Certificate request with Accept header
print_test "GET certificate - With Accept header" "POSITIVE"
RESPONSE=$(curl -s -i -X GET "$BASE_URL/api/v1/onboarding/certificate" \
  -H "Accept: application/octet-stream" \
  -H "X-Payload-Signature: valid-signature")
check_response "$RESPONSE" "200" "Certificate with Accept header"

# ============================================================================
# 2. ONBOARDING POST TESTS
# ============================================================================

echo ""
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}2. ONBOARDING POST TESTS${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"

# Positive: Valid onboarding request
print_test "POST onboarding - Valid request" "POSITIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/onboarding" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "onboarding.margo.org/v1alpha1",
    "kind": "OnboardingRequest",
    "certificate": "-----BEGIN CERTIFICATE-----\nMIIC...\n-----END CERTIFICATE-----"
  }')
check_response "$RESPONSE" "201" "Valid onboarding"

# Positive: Generic but valid apiVersion
print_test "POST onboarding - Generic valid apiVersion" "POSITIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/onboarding" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "demo.test/v1",
    "kind": "OnboardingRequest",
    "certificate": "generic-cert"
  }')
check_response "$RESPONSE" "201" "Generic apiVersion onboarding"

# Negative: Missing apiVersion
print_test "POST onboarding - Missing apiVersion" "NEGATIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/onboarding" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "kind": "OnboardingRequest",
    "certificate": "test-cert"
  }')
check_response "$RESPONSE" "400" "Missing apiVersion"

# Negative: Missing kind
print_test "POST onboarding - Missing kind" "NEGATIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/onboarding" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "onboarding.margo.org/v1alpha1",
    "certificate": "test-cert"
  }')
check_response "$RESPONSE" "400" "Missing kind"

# Negative: Empty body
print_test "POST onboarding - Empty body" "NEGATIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/onboarding" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{}')
check_response "$RESPONSE" "400" "Empty body"

# Negative: Empty certificate
print_test "POST onboarding - Empty certificate" "NEGATIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/onboarding" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "onboarding.margo.org/v1alpha1",
    "kind": "OnboardingRequest",
    "certificate": ""
  }')
check_response "$RESPONSE" "400" "Empty certificate"

# Negative: Invalid JSON
print_test "POST onboarding - Invalid JSON" "NEGATIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/onboarding" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{invalid json}')
check_response "$RESPONSE" "400" "Invalid onboarding JSON"

# Positive: Stable clientId for repeated certificate
print_test "POST onboarding - Repeated certificate returns a clientId" "POSITIVE"
FIRST_ONBOARDING=$(curl -s -X POST "$BASE_URL/api/v1/onboarding" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "demo.test/v1",
    "kind": "OnboardingRequest",
    "certificate": "stable-cert"
  }')
SECOND_ONBOARDING=$(curl -s -X POST "$BASE_URL/api/v1/onboarding" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "demo.test/v1",
    "kind": "OnboardingRequest",
    "certificate": "stable-cert"
  }')
FIRST_CLIENT_ID=$(echo "$FIRST_ONBOARDING" | grep -oP '"clientId":"\K[^"]+' | head -1)
SECOND_CLIENT_ID=$(echo "$SECOND_ONBOARDING" | grep -oP '"clientId":"\K[^"]+' | head -1)
check_not_empty "$FIRST_CLIENT_ID" "First onboarding response contained a clientId"

print_test "POST onboarding - Stable clientId for repeated certificate" "POSITIVE"
check_equal "$SECOND_CLIENT_ID" "$FIRST_CLIENT_ID" "Repeated onboarding returns the same clientId"

# ============================================================================
# 3. DEVICE CAPABILITIES PUT TESTS
# ============================================================================

echo ""
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}3. DEVICE CAPABILITIES PUT TESTS${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"

# Positive: Valid capabilities update
print_test "PUT capabilities - Valid complete payload" "POSITIVE"
RESPONSE=$(curl -s -i -X PUT "$BASE_URL/api/v1/clients/$DEVICE_ID/capabilities" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {
      "id": "device-001",
      "vendor": "TestVendor",
      "modelNumber": "TM-100",
      "serialNumber": "SN-12345",
      "roles": ["Standalone Device"],
      "resources": {
        "cpu": {
          "architecture": "amd64",
          "cores": 4
        },
        "memory": "8GB",
        "storage": "100GB",
        "peripherals": [{"name": "Display", "type": "display"}],
        "interfaces": [{"name": "eth0", "type": "ethernet"}]
      }
    }
  }')
check_response "$RESPONSE" "201" "Valid capabilities"

# Negative: Missing apiVersion
print_test "PUT capabilities - Missing apiVersion" "NEGATIVE"
RESPONSE=$(curl -s -i -X PUT "$BASE_URL/api/v1/clients/$DEVICE_ID/capabilities" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "kind": "DeviceCapabilitiesManifest",
    "properties": {"id": "device-001"}
  }')
check_response "$RESPONSE" "400" "Missing apiVersion in PUT"

# Negative: Missing properties
print_test "PUT capabilities - Missing properties" "NEGATIVE"
RESPONSE=$(curl -s -i -X PUT "$BASE_URL/api/v1/clients/$DEVICE_ID/capabilities" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeviceCapabilitiesManifest"
  }')
check_response "$RESPONSE" "400" "Missing properties"

# Negative: Missing required fields in properties
print_test "PUT capabilities - Missing resources" "NEGATIVE"
RESPONSE=$(curl -s -i -X PUT "$BASE_URL/api/v1/clients/$DEVICE_ID/capabilities" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {
      "id": "device-001",
      "vendor": "TestVendor",
      "serialNumber": "SN-123",
      "roles": ["Standalone Device"]
    }
  }')
check_response "$RESPONSE" "400" "Missing resources"

# Negative: Empty resources
print_test "PUT capabilities - Missing peripherals/interfaces" "NEGATIVE"
RESPONSE=$(curl -s -i -X PUT "$BASE_URL/api/v1/clients/$DEVICE_ID/capabilities" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {
      "id": "device-001",
      "vendor": "TestVendor",
      "modelNumber": "TM-100",
      "serialNumber": "SN-123",
      "roles": ["Standalone Device"],
      "resources": {
        "cpu": {"architecture": "amd64", "cores": 4},
        "memory": "8GB",
        "storage": "100GB"
      }
    }
  }')
check_response "$RESPONSE" "400" "Missing peripherals/interfaces"

# Negative: Invalid role enum
print_test "PUT capabilities - Invalid role enum" "NEGATIVE"
RESPONSE=$(curl -s -i -X PUT "$BASE_URL/api/v1/clients/$DEVICE_ID/capabilities" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {
      "id": "device-001",
      "vendor": "TestVendor",
      "modelNumber": "TM-100",
      "serialNumber": "SN-123",
      "roles": ["Invalid Role"],
      "resources": {
        "cpu": {"architecture": "amd64", "cores": 4},
        "memory": "8GB",
        "storage": "100GB",
        "peripherals": [{"name": "Display", "type": "display"}],
        "interfaces": [{"name": "eth0", "type": "ethernet"}]
      }
    }
  }')
check_response "$RESPONSE" "400" "Invalid role enum"

# Negative: Invalid CPU architecture enum
print_test "PUT capabilities - Invalid CPU architecture" "NEGATIVE"
RESPONSE=$(curl -s -i -X PUT "$BASE_URL/api/v1/clients/$DEVICE_ID/capabilities" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {
      "id": "device-001",
      "vendor": "TestVendor",
      "modelNumber": "TM-100",
      "serialNumber": "SN-123",
      "roles": ["Standalone Device"],
      "resources": {
        "cpu": {"architecture": "sparc", "cores": 4},
        "memory": "8GB",
        "storage": "100GB",
        "peripherals": [{"name": "Display", "type": "display"}],
        "interfaces": [{"name": "eth0", "type": "ethernet"}]
      }
    }
  }')
check_response "$RESPONSE" "400" "Invalid CPU architecture"

# Negative: Invalid peripheral type enum
print_test "PUT capabilities - Invalid peripheral type" "NEGATIVE"
RESPONSE=$(curl -s -i -X PUT "$BASE_URL/api/v1/clients/$DEVICE_ID/capabilities" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {
      "id": "device-001",
      "vendor": "TestVendor",
      "modelNumber": "TM-100",
      "serialNumber": "SN-123",
      "roles": ["Standalone Device"],
      "resources": {
        "cpu": {"architecture": "amd64", "cores": 4},
        "memory": "8GB",
        "storage": "100GB",
        "peripherals": [{"name": "Sensor", "type": "sensor"}],
        "interfaces": [{"name": "eth0", "type": "ethernet"}]
      }
    }
  }')
check_response "$RESPONSE" "400" "Invalid peripheral type"

# Negative: Invalid interface type enum
print_test "PUT capabilities - Invalid interface type" "NEGATIVE"
RESPONSE=$(curl -s -i -X PUT "$BASE_URL/api/v1/clients/$DEVICE_ID/capabilities" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {
      "id": "device-001",
      "vendor": "TestVendor",
      "modelNumber": "TM-100",
      "serialNumber": "SN-123",
      "roles": ["Standalone Device"],
      "resources": {
        "cpu": {"architecture": "amd64", "cores": 4},
        "memory": "8GB",
        "storage": "100GB",
        "peripherals": [{"name": "Display", "type": "display"}],
        "interfaces": [{"name": "zig0", "type": "zigbee"}]
      }
    }
  }')
check_response "$RESPONSE" "400" "Invalid interface type"

# ============================================================================
# 4. DEVICE CAPABILITIES POST TESTS
# ============================================================================

echo ""
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}4. DEVICE CAPABILITIES POST TESTS${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"

# Positive: Valid capabilities report
print_test "POST capabilities - Valid complete payload" "POSITIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/clients/$DEVICE_ID/capabilities" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {
      "id": "device-002",
      "vendor": "TestVendor2",
      "modelNumber": "TM-200",
      "serialNumber": "SN-67890",
      "roles": ["Standalone Device", "Cluster Leader"],
      "resources": {
        "cpu": {
          "architecture": "arm64",
          "cores": 8
        },
        "memory": "16GB",
        "storage": "500GB",
        "peripherals": [{"name": "GPU", "type": "gpu"}],
        "interfaces": [{"name": "eth0", "type": "ethernet"}]
      }
    }
  }')
check_response "$RESPONSE" "201" "Valid POST capabilities"

# Negative: Invalid JSON
print_test "POST capabilities - Invalid JSON" "NEGATIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/clients/$DEVICE_ID/capabilities" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{invalid json}')
check_response "$RESPONSE" "400" "Invalid JSON"

# Negative: Missing kind
print_test "POST capabilities - Missing kind" "NEGATIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/clients/$DEVICE_ID/capabilities" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{"apiVersion": "v1"}')
check_response "$RESPONSE" "400" "Missing kind in POST"

# ============================================================================
# 5. GET DEPLOYMENTS TESTS
# ============================================================================

echo ""
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}5. GET DEPLOYMENTS TESTS${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"

# Positive: Valid deployment list request
print_test "GET deployments - Valid request" "POSITIVE"
RESPONSE=$(curl -s -i -X GET "$BASE_URL/api/v1/clients/$DEVICE_ID/deployments" \
  -H "X-Payload-Signature: valid-signature")
check_response "$RESPONSE" "200" "Valid GET deployments"

# Positive: With Accept header
print_test "GET deployments - With Accept header" "POSITIVE"
RESPONSE=$(curl -s -i -X GET "$BASE_URL/api/v1/clients/$DEVICE_ID/deployments" \
  -H "Accept: application/vnd.margo.manifest.v1+json" \
  -H "X-Payload-Signature: valid-signature")
check_response "$RESPONSE" "200" "GET deployments with Accept"

# Positive: Different device ID
print_test "GET deployments - Different device ID" "POSITIVE"
RESPONSE=$(curl -s -i -X GET "$BASE_URL/api/v1/clients/device-999/deployments" \
  -H "X-Payload-Signature: valid-signature")
check_response "$RESPONSE" "200" "GET deployments different device"

# Negative: Unsupported Accept header
print_test "GET deployments - Unsupported Accept header" "NEGATIVE"
RESPONSE=$(curl -s -i -X GET "$BASE_URL/api/v1/clients/$DEVICE_ID/deployments" \
  -H "Accept: application/xml" \
  -H "X-Payload-Signature: valid-signature")
check_response "$RESPONSE" "406" "GET deployments with unsupported Accept"

print_test "GET deployments - Extract manifest refs and ETag" "POSITIVE"
MANIFEST_HEADERS="$TMP_DIR/manifest.headers"
MANIFEST_BODY="$TMP_DIR/manifest.body"
MANIFEST_STATUS=$(curl -s -D "$MANIFEST_HEADERS" -o "$MANIFEST_BODY" -w "%{http_code}" \
  -X GET "$BASE_URL/api/v1/clients/$DEVICE_ID/deployments" \
  -H "Accept: application/vnd.margo.manifest.v1+json" \
  -H "X-Payload-Signature: valid-signature")
check_status_code "$MANIFEST_STATUS" "200" "Manifest extraction request"
MANIFEST_JSON=$(cat "$MANIFEST_BODY")
DEPLOYMENT_ID=$(echo "$MANIFEST_JSON" | grep -oP '"deploymentId":"\K[^"]+' | head -1)
DEPLOYMENT_DIGEST=$(echo "$MANIFEST_JSON" | grep -oP '"deployments":\[\{"deploymentId":"[^"]+","digest":"\K[^"]+' | head -1)
BUNDLE_DIGEST=$(echo "$MANIFEST_JSON" | grep -oP '"bundle":\{"mediaType":"[^"]+","digest":"\K[^"]+' | head -1)
MANIFEST_ETAG=$(extract_header_value "$MANIFEST_HEADERS" "ETag")

if [[ -z "$DEPLOYMENT_ID" || -z "$DEPLOYMENT_DIGEST" || -z "$BUNDLE_DIGEST" ]]; then
  echo -e "${RED}❌ FAILED${NC} - Unable to extract deployment or bundle references from manifest"
  exit 1
fi

print_test "GET deployments - Manifest returned an ETag" "POSITIVE"
check_not_empty "$MANIFEST_ETAG" "Manifest response returned an ETag"

print_test "GET deployments - Manifest included deploymentId" "POSITIVE"
check_not_empty "$DEPLOYMENT_ID" "Manifest included a deploymentId"

print_test "GET deployments - Manifest included deployment digest" "POSITIVE"
check_not_empty "$DEPLOYMENT_DIGEST" "Manifest included a deployment digest"

print_test "GET deployments - Manifest included bundle digest" "POSITIVE"
check_not_empty "$BUNDLE_DIGEST" "Manifest included a bundle digest"

print_test "GET deployments - Conditional request with matching ETag" "POSITIVE"
MANIFEST_CONDITIONAL_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -X GET "$BASE_URL/api/v1/clients/$DEVICE_ID/deployments" \
  -H "Accept: application/vnd.margo.manifest.v1+json" \
  -H "If-None-Match: $MANIFEST_ETAG" \
  -H "X-Payload-Signature: valid-signature")
check_status_code "$MANIFEST_CONDITIONAL_STATUS" "304" "Manifest conditional GET"

# ============================================================================
# 6. GET SINGLE DEPLOYMENT TESTS
# ============================================================================

echo ""
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}6. GET SINGLE DEPLOYMENT TESTS${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"

# Positive: Valid single deployment request
print_test "GET deployment by digest - Valid request" "POSITIVE"
RESPONSE=$(curl -s -i -X GET "$BASE_URL/api/v1/clients/$DEVICE_ID/deployments/$DEPLOYMENT_ID/$DEPLOYMENT_DIGEST" \
  -H "X-Payload-Signature: valid-signature")
check_response "$RESPONSE" "200" "Valid GET single deployment"

# Positive: Different client ID still resolves content-addressable deployment
print_test "GET deployment - Different client ID" "POSITIVE"
RESPONSE=$(curl -s -i -X GET "$BASE_URL/api/v1/clients/device-xyz/deployments/$DEPLOYMENT_ID/$DEPLOYMENT_DIGEST" \
  -H "X-Payload-Signature: valid-signature")
check_response "$RESPONSE" "200" "Different client deployment fetch"

# Negative: Different digests
print_test "GET deployment - Different digest" "NEGATIVE"
RESPONSE=$(curl -s -i -X GET "$BASE_URL/api/v1/clients/$DEVICE_ID/deployments/$DEPLOYMENT_ID/xyz789" \
  -H "X-Payload-Signature: valid-signature")
check_response "$RESPONSE" "404" "Different digest"

# Negative: Different deployment ID
print_test "GET deployment - Different deployment ID" "NEGATIVE"
RESPONSE=$(curl -s -i -X GET "$BASE_URL/api/v1/clients/$DEVICE_ID/deployments/deploy-999/$DEPLOYMENT_DIGEST" \
  -H "X-Payload-Signature: valid-signature")
check_response "$RESPONSE" "404" "Different deployment ID"

print_test "GET deployment - Fetch body for digest verification" "POSITIVE"
DEPLOYMENT_HEADERS="$TMP_DIR/deployment.headers"
DEPLOYMENT_BODY="$TMP_DIR/deployment.body"
DEPLOYMENT_STATUS=$(curl -s -D "$DEPLOYMENT_HEADERS" -o "$DEPLOYMENT_BODY" -w "%{http_code}" \
  -X GET "$BASE_URL/api/v1/clients/$DEVICE_ID/deployments/$DEPLOYMENT_ID/$DEPLOYMENT_DIGEST" \
  -H "X-Payload-Signature: valid-signature")
check_status_code "$DEPLOYMENT_STATUS" "200" "Deployment fetch for digest verification"
DEPLOYMENT_ETAG=$(extract_header_value "$DEPLOYMENT_HEADERS" "ETag")
DEPLOYMENT_CACHE_CONTROL=$(extract_header_value "$DEPLOYMENT_HEADERS" "Cache-Control")
DEPLOYMENT_VARY=$(extract_header_value "$DEPLOYMENT_HEADERS" "Vary")
DEPLOYMENT_BODY_DIGEST="sha256:$(sha256sum "$DEPLOYMENT_BODY" | awk '{print $1}')"

print_test "GET deployment - Body digest matches advertised digest" "POSITIVE"
check_equal "$DEPLOYMENT_BODY_DIGEST" "$DEPLOYMENT_DIGEST" "Deployment body digest matches manifest digest"

print_test "GET deployment - ETag matches digest" "POSITIVE"
check_equal "$DEPLOYMENT_ETAG" "\"$DEPLOYMENT_DIGEST\"" "Deployment response ETag matches digest"

print_test "GET deployment - Cache-Control header is immutable" "POSITIVE"
check_equal "$DEPLOYMENT_CACHE_CONTROL" "public, max-age=31536000, immutable" "Deployment response Cache-Control matches immutable policy"

print_test "GET deployment - Vary header is Accept-Encoding" "POSITIVE"
check_equal "$DEPLOYMENT_VARY" "Accept-Encoding" "Deployment response Vary header matches expected value"

# ============================================================================
# 7. POST DEPLOYMENT STATUS TESTS
# ============================================================================

echo ""
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}7. POST DEPLOYMENT STATUS TESTS${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"

# Positive: Valid deployment status
print_test "POST deployment status - Valid request" "POSITIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/clients/$DEVICE_ID/deployments/$DEPLOYMENT_ID/status" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeploymentStatusManifest",
    "deploymentId": "'$DEPLOYMENT_ID'",
    "status": {
      "state": "installed"
    },
    "components": [
      {
        "name": "component-1",
        "state": "installed"
      }
    ]
  }')
check_response "$RESPONSE" "200" "Valid deployment status"

# Positive: Different status phases
print_test "POST deployment status - Pending phase" "POSITIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/clients/$DEVICE_ID/deployments/$DEPLOYMENT_ID/status" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeploymentStatusManifest",
    "deploymentId": "'$DEPLOYMENT_ID'",
    "status": {
      "state": "pending"
    },
    "components": []
  }')
check_response "$RESPONSE" "200" "Pending phase"

# Positive: Installing state
print_test "POST deployment status - Installing state" "POSITIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/clients/$DEVICE_ID/deployments/$DEPLOYMENT_ID/status" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeploymentStatusManifest",
    "deploymentId": "'$DEPLOYMENT_ID'",
    "status": {
      "state": "installing"
    },
    "components": [
      {
        "name": "container-1",
        "state": "installing"
      }
    ]
  }')
check_response "$RESPONSE" "200" "Installing state"

# Positive: Installed state
print_test "POST deployment status - Installed state" "POSITIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/clients/$DEVICE_ID/deployments/$DEPLOYMENT_ID/status" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeploymentStatusManifest",
    "deploymentId": "'$DEPLOYMENT_ID'",
    "status": {
      "state": "installed"
    },
    "components": [
      {
        "name": "component-1",
        "state": "installed"
      },
      {
        "name": "component-2",
        "state": "installed"
      }
    ]
  }')
check_response "$RESPONSE" "200" "Installed state"

# Positive: Failed state with error details
print_test "POST deployment status - Failed state with error" "POSITIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/clients/$DEVICE_ID/deployments/$DEPLOYMENT_ID/status" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeploymentStatusManifest",
    "deploymentId": "'$DEPLOYMENT_ID'",
    "status": {
      "state": "failed",
      "error": {
        "code": "ERR_DEPLOY_001",
        "message": "Deployment failed due to insufficient disk space"
      }
    },
    "components": [
      {
        "name": "component-1",
        "state": "failed",
        "error": {
          "code": "COMP_ERR_001",
          "message": "Component failed to initialize"
        }
      }
    ]
  }')
check_response "$RESPONSE" "200" "Failed state with error"

# Negative: Invalid state value
print_test "POST deployment status - Invalid state value" "NEGATIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/clients/$DEVICE_ID/deployments/$DEPLOYMENT_ID/status" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeploymentStatusManifest",
    "deploymentId": "'$DEPLOYMENT_ID'",
    "status": {
      "state": "unknown_state"
    },
    "components": []
  }')
check_response "$RESPONSE" "400" "Invalid state value"

# Negative: Missing status field
print_test "POST deployment status - Missing status" "NEGATIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/clients/$DEVICE_ID/deployments/$DEPLOYMENT_ID/status" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeploymentStatusManifest",
    "deploymentId": "'$DEPLOYMENT_ID'",
    "components": []
  }')
check_response "$RESPONSE" "400" "Missing status field"

# Negative: Invalid status format
print_test "POST deployment status - Invalid status format" "NEGATIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/clients/$DEVICE_ID/deployments/$DEPLOYMENT_ID/status" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeploymentStatusManifest",
    "deploymentId": "'$DEPLOYMENT_ID'",
    "status": "running",
    "components": []
  }')
check_response "$RESPONSE" "400" "Invalid status format"

# Negative: Missing state field
print_test "POST deployment status - Missing state field" "NEGATIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/clients/$DEVICE_ID/deployments/$DEPLOYMENT_ID/status" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeploymentStatusManifest",
    "deploymentId": "'$DEPLOYMENT_ID'",
    "status": {},
    "components": []
  }')
check_response "$RESPONSE" "400" "Missing state field"

# Negative: Invalid component state
print_test "POST deployment status - Invalid component state" "NEGATIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/clients/$DEVICE_ID/deployments/$DEPLOYMENT_ID/status" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeploymentStatusManifest",
    "deploymentId": "'$DEPLOYMENT_ID'",
    "status": {
      "state": "installed"
    },
    "components": [
      {
        "name": "component-1",
        "state": "rolling_back"
      }
    ]
  }')
check_response "$RESPONSE" "400" "Invalid component state"

# Negative: Invalid JSON
print_test "POST deployment status - Invalid JSON" "NEGATIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/clients/$DEVICE_ID/deployments/$DEPLOYMENT_ID/status" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{invalid json}')
check_response "$RESPONSE" "400" "Invalid deployment status JSON"

# ============================================================================
# 8. GET BUNDLE TESTS
# ============================================================================

echo ""
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}8. GET BUNDLE TESTS${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"

# Positive: Valid bundle request
print_test "GET bundle - Valid request" "POSITIVE"
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -X GET "$BASE_URL/api/v1/clients/$DEVICE_ID/bundles/$BUNDLE_DIGEST" \
  -H "X-Payload-Signature: valid-signature")
check_status_code "$HTTP_STATUS" "200" "Valid GET bundle"

# Negative: Different digest
print_test "GET bundle - Different digest" "NEGATIVE"
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -X GET "$BASE_URL/api/v1/clients/$DEVICE_ID/bundles/sha256:abc123" \
  -H "X-Payload-Signature: valid-signature")
check_status_code "$HTTP_STATUS" "404" "Different bundle digest"

# Positive: Different client ID
print_test "GET bundle - Different client ID" "POSITIVE"
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -X GET "$BASE_URL/api/v1/clients/device-xyz/bundles/$BUNDLE_DIGEST" \
  -H "X-Payload-Signature: valid-signature")
check_status_code "$HTTP_STATUS" "200" "Different client for bundle"

print_test "GET bundle - Fetch body for digest verification" "POSITIVE"
BUNDLE_HEADERS="$TMP_DIR/bundle.headers"
BUNDLE_BODY="$TMP_DIR/bundle.body"
BUNDLE_STATUS=$(curl -s -D "$BUNDLE_HEADERS" -o "$BUNDLE_BODY" -w "%{http_code}" \
  -X GET "$BASE_URL/api/v1/clients/$DEVICE_ID/bundles/$BUNDLE_DIGEST" \
  -H "X-Payload-Signature: valid-signature")
check_status_code "$BUNDLE_STATUS" "200" "Bundle fetch for digest verification"
BUNDLE_ETAG=$(extract_header_value "$BUNDLE_HEADERS" "ETag")
BUNDLE_BODY_DIGEST="sha256:$(sha256sum "$BUNDLE_BODY" | awk '{print $1}')"

print_test "GET bundle - Body digest matches advertised digest" "POSITIVE"
check_equal "$BUNDLE_BODY_DIGEST" "$BUNDLE_DIGEST" "Bundle body digest matches manifest digest"

print_test "GET bundle - ETag matches digest" "POSITIVE"
check_equal "$BUNDLE_ETAG" "\"$BUNDLE_DIGEST\"" "Bundle response ETag matches digest"

print_test "GET bundle - Conditional request with matching ETag" "POSITIVE"
BUNDLE_CONDITIONAL_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -X GET "$BASE_URL/api/v1/clients/$DEVICE_ID/bundles/$BUNDLE_DIGEST" \
  -H "If-None-Match: $BUNDLE_ETAG" \
  -H "X-Payload-Signature: valid-signature")
check_status_code "$BUNDLE_CONDITIONAL_STATUS" "304" "Bundle conditional GET"

# ============================================================================
# 9. EDGE CASES AND ERROR SCENARIOS
# ============================================================================

echo ""
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}9. EDGE CASES AND ERROR SCENARIOS${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"

# Negative: Missing X-Payload-Signature header
print_test "GET certificate - Missing signature header" "NEGATIVE"
RESPONSE=$(curl -s -i -X GET "$BASE_URL/api/v1/onboarding/certificate")
# Note: Server currently accepts all signatures (no-op), so we expect 200
check_response "$RESPONSE" "200" "Missing signature (permissive mode)"

# Negative: Malformed URL path
print_test "GET invalid endpoint - Malformed path" "NEGATIVE"
RESPONSE=$(curl -s -i -X GET "$BASE_URL/api/v1/clients//invalid" \
  -H "X-Payload-Signature: valid-signature")
# This should return 404
HTTP_STATUS=$(echo "$RESPONSE" | sed -n 's/^HTTP\/[0-9.]* \([0-9][0-9][0-9]\).*$/\1/p' | head -1)
check_status_in_set "$HTTP_STATUS" "Malformed path request" 404 400

# Negative: Unsupported HTTP method
print_test "Capabilities endpoint - Unsupported PATCH method" "NEGATIVE"
RESPONSE=$(curl -s -i -X PATCH "$BASE_URL/api/v1/clients/$DEVICE_ID/capabilities" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{}')
HTTP_STATUS=$(echo "$RESPONSE" | sed -n 's/^HTTP\/[0-9.]* \([0-9][0-9][0-9]\).*$/\1/p' | head -1)
check_status_in_set "$HTTP_STATUS" "Unsupported PATCH request" 405 404

# Negative: Unsupported method on certificate endpoint
print_test "Certificate endpoint - Unsupported POST method" "NEGATIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/onboarding/certificate" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{}')
HTTP_STATUS=$(echo "$RESPONSE" | sed -n 's/^HTTP\/[0-9.]* \([0-9][0-9][0-9]\).*$/\1/p' | head -1)
check_status_in_set "$HTTP_STATUS" "Unsupported POST to certificate endpoint" 405 404

# Positive: Large payload test
print_test "PUT capabilities - Large but valid payload" "POSITIVE"
RESPONSE=$(curl -s -i -X PUT "$BASE_URL/api/v1/clients/device-large/capabilities" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {
      "id": "device-large",
      "vendor": "LargeVendor",
      "modelNumber": "LM-5000",
      "serialNumber": "SN-999999",
      "roles": ["Standalone Device", "Cluster Leader"],
      "resources": {
        "cpu": {
          "architecture": "amd64",
          "cores": 128,
          "model": "Intel Xeon Platinum"
        },
        "memory": "512GB",
        "storage": "10TB",
        "peripherals": [
          {"name": "GPU1", "type": "gpu"},
          {"name": "GPU2", "type": "gpu"},
          {"name": "Display", "type": "display"},
          {"name": "Camera", "type": "camera"},
          {"name": "Microphone", "type": "microphone"}
        ],
        "interfaces": [
          {"name": "eth0", "type": "ethernet"},
          {"name": "eth1", "type": "ethernet"},
          {"name": "wlan0", "type": "wifi"},
          {"name": "usb0", "type": "usb"}
        ]
      }
    }
  }')
check_response "$RESPONSE" "201" "Large valid payload"

# ============================================================================
# 10. CONCURRENT DEVICE TESTS
# ============================================================================

echo ""
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}10. CONCURRENT DEVICE TESTS${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"

# Positive: Multiple devices registering in sequence
print_test "POST capabilities - First device" "POSITIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/clients/multi-device-1/capabilities" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {
      "id": "multi-device-1",
      "vendor": "Vendor1",
      "modelNumber": "M1",
      "serialNumber": "S1",
      "roles": ["Standalone Device"],
      "resources": {
        "cpu": {"architecture": "amd64", "cores": 2},
        "memory": "4GB",
        "storage": "50GB",
        "peripherals": [],
        "interfaces": [{"name": "eth0", "type": "ethernet"}]
      }
    }
  }')
check_response "$RESPONSE" "201" "First device registration"

# Positive: Second device
print_test "POST capabilities - Second device" "POSITIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/clients/multi-device-2/capabilities" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {
      "id": "multi-device-2",
      "vendor": "Vendor2",
      "modelNumber": "M2",
      "serialNumber": "S2",
      "roles": ["Standalone Device"],
      "resources": {
        "cpu": {"architecture": "arm64", "cores": 4},
        "memory": "8GB",
        "storage": "100GB",
        "peripherals": [{"name": "Camera", "type": "camera"}],
        "interfaces": [{"name": "wlan0", "type": "wifi"}]
      }
    }
  }')
check_response "$RESPONSE" "201" "Second device registration"

# Positive: Third device
print_test "POST capabilities - Third device" "POSITIVE"
RESPONSE=$(curl -s -i -X POST "$BASE_URL/api/v1/clients/multi-device-3/capabilities" \
  -H "Content-Type: application/json" \
  -H "X-Payload-Signature: valid-signature" \
  -d '{
    "apiVersion": "v1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {
      "id": "multi-device-3",
      "vendor": "Vendor3",
      "modelNumber": "M3",
      "serialNumber": "S3",
      "roles": ["Standalone Device", "Cluster Leader"],
      "resources": {
        "cpu": {"architecture": "amd64", "cores": 8},
        "memory": "16GB",
        "storage": "250GB",
        "peripherals": [{"name": "GPU", "type": "gpu"}],
        "interfaces": [{"name": "eth0", "type": "ethernet"}, {"name": "usb0", "type": "usb"}]
      }
    }
  }')
check_response "$RESPONSE" "201" "Third device registration"

# ============================================================================
# TEST SUMMARY
# ============================================================================

echo ""
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}TEST SUMMARY${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo ""
echo -e "Total Tests:     ${BLUE}$TOTAL_TESTS${NC}"
echo -e "Passed:          ${GREEN}$PASSED_TESTS${NC}"
echo -e "Failed:          ${RED}$FAILED_TESTS${NC}"
echo ""

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "${GREEN}✅ ALL TESTS PASSED!${NC}"
    exit 0
else
    echo -e "${RED}❌ SOME TESTS FAILED${NC}"
    exit 1
fi
