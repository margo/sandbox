#!/bin/bash

# Integration test script for Mock WFM Server with Device Agent
# This script tests the mock-wfm server using curl commands similar to what
# device-agent would do and targets the current TLS listener port.

set -e

# Configuration
MOCK_WFM_HOST="${1:-localhost}"
MOCK_WFM_PORT="${2:-9090}"
CA_CERT_PATH="${3:-./ca-cert.pem}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Helper functions
print_test() {
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo ""
    echo -e "${BLUE}[TEST $TOTAL_TESTS]${NC} $1"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

check_response() {
    local expected_status=$1
    local actual_status=$2
    local test_name=$3
    local response=$4

    if [[ "$actual_status" == "$expected_status" ]]; then
        PASSED_TESTS=$((PASSED_TESTS + 1))
        echo -e "${GREEN}✅ PASSED${NC}: Expected HTTP $expected_status, got $actual_status"
        if [[ -n "$response" ]]; then
            echo -e "Response: $(echo "$response" | head -c 200)..."
        fi
        return 0
    else
        FAILED_TESTS=$((FAILED_TESTS + 1))
        echo -e "${RED}❌ FAILED${NC}: Expected HTTP $expected_status, got $actual_status"
        if [[ -n "$response" ]]; then
            echo -e "Response: $response"
        fi
        return 1
    fi
}

# Banner
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Mock WFM Server - Device Agent Integration Test${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "📋 Test Configuration:"
echo "   Mock WFM Server: https://$MOCK_WFM_HOST:$MOCK_WFM_PORT/v1alpha2/margo"
echo "   CA Certificate: $CA_CERT_PATH"
echo ""

# Check if CA certificate exists
if [[ ! -f "$CA_CERT_PATH" ]]; then
    echo -e "${RED}❌ ERROR: CA certificate not found at: $CA_CERT_PATH${NC}"
    echo "Please generate certificates or provide correct path:"
    echo "  bash generate-certs.sh ."
    exit 1
fi

echo -e "${GREEN}✅ CA certificate found${NC}"
echo ""

# Test 1: Check HTTPS connectivity
print_test "HTTPS Connectivity - GET /v1alpha2/margo/api/v1/onboarding/certificate"

RESPONSE=$(curl -s -w "\n%{http_code}" \
    --cacert "$CA_CERT_PATH" \
    "https://$MOCK_WFM_HOST:$MOCK_WFM_PORT/v1alpha2/margo/api/v1/onboarding/certificate" 2>&1 || echo "000")

HTTP_STATUS=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | sed '$d')

check_response "200" "$HTTP_STATUS" "HTTPS Certificate Endpoint" "$BODY"

# Test 2: POST Onboarding
print_test "POST Onboarding Request"

RESPONSE=$(curl -s -w "\n%{http_code}" -X POST \
    --cacert "$CA_CERT_PATH" \
    -H "Content-Type: application/json" \
    -H "X-Payload-Signature: test-signature" \
    -d '{
        "apiVersion": "onboarding.margo.org/v1alpha1",
        "kind": "OnboardingRequest",
        "certificate": "test-cert"
    }' \
    "https://$MOCK_WFM_HOST:$MOCK_WFM_PORT/v1alpha2/margo/api/v1/onboarding" 2>&1 || echo "000")

HTTP_STATUS=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | sed '$d')

check_response "201" "$HTTP_STATUS" "Onboarding POST" "$BODY"

# Test 3: Device Capabilities PUT
print_test "PUT Device Capabilities"

DEVICE_ID="device-integration-test"
RESPONSE=$(curl -s -w "\n%{http_code}" -X PUT \
    --cacert "$CA_CERT_PATH" \
    -H "Content-Type: application/json" \
    -H "X-Payload-Signature: test-signature" \
    -d '{
        "apiVersion": "v1",
        "kind": "DeviceCapabilitiesManifest",
        "properties": {
            "id": "'$DEVICE_ID'",
            "vendor": "Margo Test",
            "modelNumber": "Model-X",
            "serialNumber": "SN-123456",
            "roles": ["Standalone Device"],
            "resources": {
                "cpu": {
                    "architecture": "amd64",
                    "cores": 8
                },
                "memory": "16GB",
                "storage": "512GB",
                "peripherals": [
                    {"name": "GPU-1", "type": "gpu"}
                ],
                "interfaces": [
                    {"name": "eth0", "type": "ethernet"}
                ]
            }
        }
    }' \
    "https://$MOCK_WFM_HOST:$MOCK_WFM_PORT/v1alpha2/margo/api/v1/clients/$DEVICE_ID/capabilities" 2>&1 || echo "000")

HTTP_STATUS=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | sed '$d')

check_response "201" "$HTTP_STATUS" "Device Capabilities PUT" "$BODY"

# Test 4: GET Deployments
print_test "GET Deployments List"

RESPONSE=$(curl -s -w "\n%{http_code}" \
    --cacert "$CA_CERT_PATH" \
    "https://$MOCK_WFM_HOST:$MOCK_WFM_PORT/v1alpha2/margo/api/v1/clients/$DEVICE_ID/deployments" 2>&1 || echo "000")

HTTP_STATUS=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | sed '$d')

check_response "200" "$HTTP_STATUS" "GET Deployments" "$BODY"

DEPLOYMENT_ID=$(echo "$BODY" | grep -oP '"deploymentId":"\K[^"]+' | head -1)
DEPLOYMENT_DIGEST=$(echo "$BODY" | grep -oP '"deployments":\[\{"deploymentId":"[^"]+","digest":"\K[^"]+' | head -1)
BUNDLE_DIGEST=$(echo "$BODY" | grep -oP '"bundle":\{"mediaType":"[^"]+","digest":"\K[^"]+' | head -1)

if [[ -z "$DEPLOYMENT_ID" || -z "$DEPLOYMENT_DIGEST" || -z "$BUNDLE_DIGEST" ]]; then
    echo -e "${RED}❌ FAILED${NC}: Unable to extract deployment or bundle references from manifest"
    exit 1
fi

# Test 5: POST Deployment Status
print_test "POST Deployment Status"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST \
    --cacert "$CA_CERT_PATH" \
    -H "Content-Type: application/json" \
    -H "X-Payload-Signature: test-signature" \
    -d '{
        "apiVersion": "v1",
        "kind": "DeploymentStatusManifest",
        "deploymentId": "'$DEPLOYMENT_ID'",
        "status": {
            "state": "installed"
        },
        "components": [
            {"name": "component-1", "state": "installed"}
        ]
    }' \
    "https://$MOCK_WFM_HOST:$MOCK_WFM_PORT/v1alpha2/margo/api/v1/clients/$DEVICE_ID/deployments/$DEPLOYMENT_ID/status" 2>&1 || echo "000")

HTTP_STATUS=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | sed '$d')

check_response "200" "$HTTP_STATUS" "Deployment Status POST" "$BODY"

# Test 6: GET Single Deployment
print_test "GET Single Deployment"

RESPONSE=$(curl -s -w "\n%{http_code}" \
    --cacert "$CA_CERT_PATH" \
    "https://$MOCK_WFM_HOST:$MOCK_WFM_PORT/v1alpha2/margo/api/v1/clients/$DEVICE_ID/deployments/$DEPLOYMENT_ID/$DEPLOYMENT_DIGEST" 2>&1 || echo "000")

HTTP_STATUS=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | sed '$d')

check_response "200" "$HTTP_STATUS" "GET Single Deployment" "$BODY"

# Test 7: GET Bundle
print_test "GET Bundle"

RESPONSE=$(curl -s -w "\n%{http_code}" \
    --cacert "$CA_CERT_PATH" \
    "https://$MOCK_WFM_HOST:$MOCK_WFM_PORT/v1alpha2/margo/api/v1/clients/$DEVICE_ID/bundles/$BUNDLE_DIGEST" 2>&1 || echo "000")

HTTP_STATUS=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | sed '$d')

check_response "200" "$HTTP_STATUS" "GET Bundle" "$BODY"

# Test 8: POST Device Capabilities
print_test "POST Device Capabilities"

RESPONSE=$(curl -s -w "\n%{http_code}" -X POST \
    --cacert "$CA_CERT_PATH" \
    -H "Content-Type: application/json" \
    -H "X-Payload-Signature: test-signature" \
    -d '{
        "apiVersion": "v1",
        "kind": "DeviceCapabilitiesManifest",
        "properties": {
            "id": "'$DEVICE_ID'-post",
            "vendor": "Margo Test",
            "modelNumber": "Model-Y",
            "serialNumber": "SN-789012",
            "roles": ["Cluster Leader"],
            "resources": {
                "cpu": {
                    "architecture": "amd64",
                    "cores": 16
                },
                "memory": "32GB",
                "storage": "1TB",
                "peripherals": [],
                "interfaces": []
            }
        }
    }' \
    "https://$MOCK_WFM_HOST:$MOCK_WFM_PORT/v1alpha2/margo/api/v1/clients/$DEVICE_ID-post/capabilities" 2>&1 || echo "000")

HTTP_STATUS=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | sed '$d')

check_response "201" "$HTTP_STATUS" "Device Capabilities POST" "$BODY"

# Test 9: Error Handling - Invalid State
print_test "Negative Test - Invalid Deployment State"

RESPONSE=$(curl -s -w "\n%{http_code}" -X POST \
    --cacert "$CA_CERT_PATH" \
    -H "Content-Type: application/json" \
    -H "X-Payload-Signature: test-signature" \
    -d '{
        "apiVersion": "v1",
        "kind": "DeploymentStatusManifest",
        "deploymentId": "'$DEPLOYMENT_ID'",
        "status": {
            "state": "invalid_state"
        },
        "components": []
    }' \
    "https://$MOCK_WFM_HOST:$MOCK_WFM_PORT/v1alpha2/margo/api/v1/clients/$DEVICE_ID/deployments/$DEPLOYMENT_ID/status" 2>&1 || echo "000")

HTTP_STATUS=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | sed '$d')

check_response "400" "$HTTP_STATUS" "Invalid State Error Handling" "$BODY"

# Test 10: Certificate Verification
print_test "HTTPS Certificate Verification"

CERT_INFO=$(echo | openssl s_client -servername $MOCK_WFM_HOST \
    -connect $MOCK_WFM_HOST:$MOCK_WFM_PORT \
    -CAfile "$CA_CERT_PATH" 2>/dev/null | openssl x509 -noout -subject -issuer 2>/dev/null || echo "FAILED")

if echo "$CERT_INFO" | grep -qi '^subject='; then
    PASSED_TESTS=$((PASSED_TESTS + 1))
    echo -e "${GREEN}✅ PASSED${NC}: Certificate verified successfully"
    echo "Certificate Details:"
    echo "$CERT_INFO"
else
    FAILED_TESTS=$((FAILED_TESTS + 1))
    echo -e "${RED}❌ FAILED${NC}: Certificate verification failed"
fi

# Summary
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Test Summary${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "📊 Results:"
echo "   Total Tests:  $TOTAL_TESTS"
echo -e "   ${GREEN}Passed: $PASSED_TESTS${NC}"
echo -e "   ${RED}Failed: $FAILED_TESTS${NC}"

if [[ $FAILED_TESTS -eq 0 ]]; then
    echo ""
    echo -e "${GREEN}✅ All tests passed!${NC}"
    echo ""
    echo "🎉 Mock WFM Server is working correctly with HTTPS!"
    echo ""
    echo "📝 Device Agent Configuration:"
    echo "   sbiUrl: https://$MOCK_WFM_HOST:$MOCK_WFM_PORT/v1alpha2/margo"
    echo "   caKeyRef.path: ./config/ca-cert.pem"
    echo ""
    exit 0
else
    echo ""
    echo -e "${RED}❌ Some tests failed!${NC}"
    echo ""
    echo "🔍 Troubleshooting:"
    echo "   1. Check if mock server is running: netstat -tlnp | grep 9090"
    echo "   2. Verify CA certificate is correct: openssl x509 -in $CA_CERT_PATH -noout -text"
    echo "   3. Check server logs for errors"
    echo ""
    exit 1
fi
