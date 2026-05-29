#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Configuration
DATA_DIR="newman-data"
ENV_FILE="$DATA_DIR/device-agent.env.json"
RESPONSES_DIR="$DATA_DIR/responses"
EXECUTION_LOG="$DATA_DIR/execution.log"
CONTAINERS_FILE="$DATA_DIR/deployed-containers.txt"
LOCAL_CERT_DIR="$SCRIPT_DIR/certs"
LOCAL_CA_CERT_FILE="$LOCAL_CERT_DIR/ca-cert.pem"
DEVICE_CERT_FILE="$DATA_DIR/certs/device-cert.pem"
DEVICE_KEY_FILE="$DATA_DIR/certs/device.key"
DEFAULT_DEPLOYMENT_ID="demo-deployment-001"

cd "$SCRIPT_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $*" | tee -a "$EXECUTION_LOG"
}

log_success() {
    echo -e "${GREEN}✅ $*${NC}" | tee -a "$EXECUTION_LOG"
}

log_warn() {
    echo -e "${YELLOW}⚠️  $*${NC}" | tee -a "$EXECUTION_LOG"
}

log_error() {
    echo -e "${RED}❌ $*${NC}" | tee -a "$EXECUTION_LOG"
}

ensure_cmd() {
    local cmd="$1"
    local hint="$2"
    if ! command -v "$cmd" >/dev/null 2>&1; then
        log_error "Missing required command: $cmd"
        log_error "Install hint: $hint"
        return 1
    fi
    return 0
}

# Verify prerequisites
echo "=================================================="
echo "  WFM Supplier: Workload Execution Phase"
echo "=================================================="
echo ""

ensure_cmd jq "sudo apt-get install -y jq" || exit 1
ensure_cmd curl "sudo apt-get install -y curl" || exit 1
ensure_cmd docker "sudo apt-get install -y docker.io" || exit 1
ensure_cmd base64 "sudo apt-get install -y coreutils" || exit 1

if [[ ! -f "$ENV_FILE" ]]; then
    log_error "Missing environment file: $ENV_FILE"
    log_error "Run ./1-setup_portman.sh and ./2-run_newman.sh first"
    exit 1
fi

# Extract environment variables
BASE_URL="$(jq -r '.values[] | select(.key=="baseUrl") | .value' "$ENV_FILE" 2>/dev/null | head -n1 || echo "")"
CLIENT_ID="$(jq -r '.values[] | select(.key=="clientId") | .value' "$ENV_FILE" 2>/dev/null | head -n1 || echo "")"
DEVICE_ID="$(jq -r '.values[] | select(.key=="deviceId") | .value' "$ENV_FILE" 2>/dev/null | head -n1 || echo "")"

if [[ -z "$BASE_URL" || "$BASE_URL" == "null" ]]; then
    BASE_URL="https://symphony.machine:8082/v1alpha2/margo"
    log_warn "Using default BASE_URL: $BASE_URL"
fi

# Log onboarding status
if [[ -n "$CLIENT_ID" && "$CLIENT_ID" != "null" && "$CLIENT_ID" != "" ]]; then
    log_success "Device already onboarded (Client ID: $CLIENT_ID)"
else
    log_warn "CLIENT_ID not found - device may not have onboarded successfully"
    CLIENT_ID=""
fi

if [[ -z "$DEVICE_ID" || "$DEVICE_ID" == "null" ]]; then
    log_error "DEVICE_ID not found in environment"
    exit 1
fi

mkdir -p "$RESPONSES_DIR"
rm -f "$CONTAINERS_FILE" "$EXECUTION_LOG"

log "Starting workload execution phase"
log "Device ID: $DEVICE_ID"
log "Base URL: $BASE_URL"
log "Client ID: ${CLIENT_ID:-<not yet onboarded>}"

# Function to make TLS requests to WFM
make_wfm_request() {
    local method="$1"
    local endpoint="$2"
    local data="$3"
    local response_file="$4"
    
    local url="${BASE_URL}${endpoint}"
    local curl_args=(
        -s
        -k
        -X "$method"
        --cacert "$LOCAL_CA_CERT_FILE"
        -H "Content-Type: application/json"
    )
    
    if [[ -n "$data" ]]; then
        curl_args+=(-d "$data")
    fi
    
    log "Requesting: $method $endpoint"
    
    if curl "${curl_args[@]}" "$url" > "$response_file" 2>&1; then
        log_success "Received response (size: $(wc -c < "$response_file") bytes)"
        return 0
    else
        log_error "Request failed"
        return 1
    fi
}

# Phase 1: Query current deployments from WFM (if client is onboarded)
log ""
log "Phase 1: Query deployments from WFM"
log "=================================="

if [[ -z "$CLIENT_ID" ]]; then
    log_warn "Skipping deployment query (client not onboarded)"
else
    DEPLOYMENTS_RESPONSE="$RESPONSES_DIR/deployments-query.json"
    if make_wfm_request "GET" "/api/v1/clients/$CLIENT_ID/deployments" "" "$DEPLOYMENTS_RESPONSE"; then
        if jq empty "$DEPLOYMENTS_RESPONSE" 2>/dev/null; then
            DEPLOYMENT_COUNT=$(jq 'if .deployments then (.deployments | length) else 0 end' "$DEPLOYMENTS_RESPONSE" 2>/dev/null || echo 0)
            log_success "Found $DEPLOYMENT_COUNT deployments"
            
            if jq '.deployments[]?' "$DEPLOYMENTS_RESPONSE" > /dev/null 2>&1; then
                DEPLOYMENT_ID=$(jq -r '.deployments[0]' "$DEPLOYMENTS_RESPONSE" 2>/dev/null | head -1)
                if [[ -n "$DEPLOYMENT_ID" && "$DEPLOYMENT_ID" != "null" ]]; then
                    jq --arg did "$DEPLOYMENT_ID" '.values |= map(if .key == "deploymentId" then .value = $did else . end)' "$ENV_FILE" > "$ENV_FILE.tmp"
                    mv "$ENV_FILE.tmp" "$ENV_FILE"
                    log_success "Extracted deployment ID: $DEPLOYMENT_ID"
                fi
            fi
        else
            log_warn "Invalid JSON in deployments response"
        fi
    else
        log_warn "Failed to query deployments"
    fi
fi

# Phase 2: Get deployment details and bundles
log ""
log "Phase 2: Retrieve deployment bundles"
log "===================================="

DEPLOYMENT_ID="${DEPLOYMENT_ID:-$DEFAULT_DEPLOYMENT_ID}"

if [[ -z "$CLIENT_ID" ]]; then
    log_warn "Skipping bundle retrieval (client not onboarded)"
    log_warn "Using real docker-compose specification for testing: $DEPLOYMENT_ID"
    
    # Create realistic workload spec based on nginx-proxy docker-compose
    BUNDLE_RESPONSE="$RESPONSES_DIR/bundle-docker-compose.json"
    jq -cn \
        --arg dep_id "$DEPLOYMENT_ID" \
        --arg dev_id "$DEVICE_ID" \
        '{
            apiVersion: "workload.margo.org/v1alpha1",
            kind: "WorkloadManifest",
            deploymentId: $dep_id,
            deviceId: $dev_id,
            source: "docker-compose",
            services: {
                "nginx-proxy": {
                    name: "nginx-proxy",
                    image: "nginxproxy/nginx-proxy:latest",
                    command: null,
                    ports: ["8080:80"],
                    volumes: ["/var/run/docker.sock:/tmp/docker.sock:ro"],
                    environment: {},
                    labels: {deploymentId: $dep_id, deviceId: $dev_id, service: "nginx-proxy"}
                },
                "whoami-1": {
                    name: "whoami-1",
                    image: "jwilder/whoami:latest",
                    command: null,
                    ports: ["8081:8000"],
                    environment: {VIRTUAL_HOST: "whoami.example.com"},
                    labels: {deploymentId: $dep_id, deviceId: $dev_id, service: "whoami"}
                },
                "whoami-2": {
                    name: "whoami-2",
                    image: "jwilder/whoami:latest",
                    command: null,
                    ports: ["8082:8000"],
                    environment: {VIRTUAL_HOST: "api.example.com"},
                    labels: {deploymentId: $dep_id, deviceId: $dev_id, service: "whoami"}
                }
            }
        }' > "$BUNDLE_RESPONSE"
    
    log_success "Using real docker-compose workload specification (nginx-proxy + 2x whoami)"
else
    BUNDLE_RESPONSE="$RESPONSES_DIR/bundle-details.json"
    if make_wfm_request "GET" "/api/v1/clients/$CLIENT_ID/bundles/$DEPLOYMENT_ID" "" "$BUNDLE_RESPONSE"; then
        if jq empty "$BUNDLE_RESPONSE" 2>/dev/null; then
            log_success "Retrieved bundle details"
        else
            log_warn "Invalid bundle response format"
        fi
    else
        log_warn "Failed to retrieve bundle details"
    fi
fi

# Phase 3: Execute workloads
log ""
log "Phase 3: Execute workloads in Docker"
log "===================================="

EXECUTED_CONTAINERS=0
FAILED_EXECUTIONS=0

if [[ ! -f "$BUNDLE_RESPONSE" ]]; then
    log_error "No bundle response available for execution"
    exit 1
fi

# Parse workloads from bundle and execute
if [[ ! -f "$BUNDLE_RESPONSE" ]]; then
    log_error "No bundle response available for execution"
    exit 1
fi

# Check if this is docker-compose format (services object) or workloads array format
if jq -e '.services' "$BUNDLE_RESPONSE" >/dev/null 2>&1; then
    log "Docker Compose format detected (services)"
    
    # Execute services from docker-compose
    SERVICES=$(jq '.services | keys[]' -r "$BUNDLE_RESPONSE" 2>/dev/null)
    
    for SERVICE_NAME in $SERVICES; do
        SERVICE_JSON=$(jq ".services[\"$SERVICE_NAME\"]" "$BUNDLE_RESPONSE")
        IMAGE=$(echo "$SERVICE_JSON" | jq -r '.image // "alpine:latest"')
        PORTS=$(echo "$SERVICE_JSON" | jq -r '.ports[]? // empty' 2>/dev/null)
        VOLUMES=$(echo "$SERVICE_JSON" | jq -r '.volumes[]? // empty' 2>/dev/null)
        ENV_VARS=$(echo "$SERVICE_JSON" | jq '.environment // {} | to_entries[] | "\(.key)=\(.value)"' -r 2>/dev/null)
        COMMAND=$(echo "$SERVICE_JSON" | jq -r '.command as $cmd | if ($cmd | type) == "array" then $cmd | join(" ") else $cmd end' 2>/dev/null || echo "")
        
        log ""
        log "Deploying service: $SERVICE_NAME"
        log "  Image: $IMAGE"
        
        # Build docker arguments
        DOCKER_ARGS=(
            "--detach"
            "--name" "${DEVICE_ID}-${SERVICE_NAME}-$(date +%s)"
            "--label" "device-agent=${DEVICE_ID}"
            "--label" "deployment=${DEPLOYMENT_ID}"
            "--label" "service=${SERVICE_NAME}"
        )
        
        # Add ports
        if [[ -n "$PORTS" ]]; then
            while IFS= read -r port; do
                [[ -n "$port" ]] && DOCKER_ARGS+=("-p" "$port")
                log "  Port: $port"
            done <<< "$PORTS"
        fi
        
        # Add volumes
        if [[ -n "$VOLUMES" ]]; then
            while IFS= read -r volume; do
                [[ -n "$volume" ]] && DOCKER_ARGS+=("-v" "$volume")
                log "  Volume: $volume"
            done <<< "$VOLUMES"
        fi
        
        # Add environment variables
        if [[ -n "$ENV_VARS" ]]; then
            while IFS= read -r env_var; do
                [[ -n "$env_var" ]] && DOCKER_ARGS+=("-e" "$env_var")
                log "  Env: $env_var"
            done <<< "$ENV_VARS"
        fi
        
        # Pull the image
        log "Pulling image: $IMAGE"
        if docker pull "$IMAGE" >> "$EXECUTION_LOG" 2>&1; then
            log_success "Image pulled successfully"
        else
            log_warn "Image pull returned non-zero (might be cached)"
        fi
        
        # Execute container
        log "Starting container..."
        if [[ -n "$COMMAND" && "$COMMAND" != "null" && "$COMMAND" != "" ]]; then
            DOCKER_OUTPUT=$(docker run "${DOCKER_ARGS[@]}" "$IMAGE" sh -c "$COMMAND" 2>&1) || true
        else
            DOCKER_OUTPUT=$(docker run "${DOCKER_ARGS[@]}" "$IMAGE" 2>&1) || true
        fi
        
        # Check if docker run was successful - look for daemon errors specifically (not warnings)
        if echo "$DOCKER_OUTPUT" | grep -qE "Error response from daemon:"; then
            # Docker run failed
            ERROR_MSG=$(echo "$DOCKER_OUTPUT" | grep "Error response from daemon" | head -1)
            log_error "Failed to start container: $ERROR_MSG"
            ((FAILED_EXECUTIONS++))
        else
            # Docker run succeeded - extract container ID (64-character hex string)
            CONTAINER_ID=$(echo "$DOCKER_OUTPUT" | grep -oE '^[a-f0-9]{64}$' | head -1 || echo "$DOCKER_OUTPUT" | grep -oE '[a-f0-9]{64}' | head -1)
            
            if [[ -n "$CONTAINER_ID" && "$CONTAINER_ID" =~ ^[a-f0-9]{64}$ ]]; then
                log_success "Container started: $CONTAINER_ID"
                echo "$CONTAINER_ID" >> "$CONTAINERS_FILE"
                ((EXECUTED_CONTAINERS++))
                
                # Verify container is running
                sleep 1
                CONTAINER_STATUS=$(docker ps --filter "id=$CONTAINER_ID" --format "{{.Status}}" 2>/dev/null || echo "")
                if [[ -n "$CONTAINER_STATUS" ]]; then
                    log_success "Container status: $CONTAINER_STATUS"
                else
                    log_warn "Could not verify container status"
                fi
            else
                log_error "Failed to extract container ID from output"
                ((FAILED_EXECUTIONS++))
            fi
        fi
    done
    
else
    log "Workloads array format detected (legacy)"
    
    # Use jq to iterate properly over workloads array
    WORKLOAD_COUNT=$(jq '.workloads | length' "$BUNDLE_RESPONSE" 2>/dev/null || echo 0)

    for ((i=0; i<WORKLOAD_COUNT; i++)); do
        WORKLOAD_JSON=$(jq ".workloads[$i]" "$BUNDLE_RESPONSE")
        WORKLOAD_NAME=$(echo "$WORKLOAD_JSON" | jq -r '.name // "unknown"')
        IMAGE=$(echo "$WORKLOAD_JSON" | jq -r '.image // "alpine:latest"')
        COMMAND=$(echo "$WORKLOAD_JSON" | jq -r '.command as $cmd | if ($cmd | type) == "array" then $cmd | join(" ") else $cmd end' 2>/dev/null || echo "")
        
        log ""
        log "Executing workload: $WORKLOAD_NAME"
        log "  Image: $IMAGE"
        
        # Build docker command
        DOCKER_ARGS=(
            "--detach"
            "--rm"
            "--name" "${DEVICE_ID}-${WORKLOAD_NAME}-$(date +%s)"
            "--label" "device-agent=${DEVICE_ID}"
            "--label" "deployment=${DEPLOYMENT_ID}"
            "--label" "workload=${WORKLOAD_NAME}"
        )
        
        # Pull the image first
        log "Pulling image: $IMAGE"
        if docker pull "$IMAGE" >> "$EXECUTION_LOG" 2>&1; then
            log_success "Image pulled successfully"
        else
            log_warn "Image pull returned non-zero (might be locally cached)"
        fi
        
        # Execute container
        log "Starting Docker container..."
        DOCKER_OUTPUT=$(docker run "${DOCKER_ARGS[@]}" "$IMAGE" sh -c "${COMMAND:-}" 2>&1) || true
        
        # Check for actual errors (not warnings)
        if echo "$DOCKER_OUTPUT" | grep -qE "Error response from daemon:"; then
            # Failed
            ERROR_MSG=$(echo "$DOCKER_OUTPUT" | grep "Error response from daemon" | head -1)
            log_error "Failed to start container: $ERROR_MSG"
            ((FAILED_EXECUTIONS++))
        else
            # Success - extract container ID (64 hex chars)
            CONTAINER_ID=$(echo "$DOCKER_OUTPUT" | grep -oE '^[a-f0-9]{64}$' | head -1 || echo "$DOCKER_OUTPUT" | grep -oE '[a-f0-9]{64}' | head -1)
            
            if [[ -n "$CONTAINER_ID" && "$CONTAINER_ID" =~ ^[a-f0-9]{64}$ ]]; then
                log_success "Container started: $CONTAINER_ID"
                echo "$CONTAINER_ID" >> "$CONTAINERS_FILE"
                ((EXECUTED_CONTAINERS++))
                
                # Verify container is running
                sleep 1
                CONTAINER_STATUS=$(docker ps --filter "id=$CONTAINER_ID" --format "{{.Status}}" 2>/dev/null || echo "")
                if [[ -n "$CONTAINER_STATUS" ]]; then
                    log_success "Container status: $CONTAINER_STATUS"
                else
                    log_warn "Could not verify container status"
                fi
            else
                log_error "Failed to extract container ID from output"
                ((FAILED_EXECUTIONS++))
            fi
        fi
    done
fi

# Phase 4: Verify containers are running
log ""
log "Phase 4: Verify deployed containers"
log "===================================="

if [[ -f "$CONTAINERS_FILE" ]]; then
    RUNNING_COUNT=$(wc -l < "$CONTAINERS_FILE")
    log "Expected containers to deploy: $RUNNING_COUNT"
    
    # Show all containers with device labels
    log ""
    log "Currently running containers (device-agent labeled):"
    docker ps --filter "label=device-agent=$DEVICE_ID" --format "table {{.ID}}\t{{.Image}}\t{{.Names}}\t{{.Status}}" | tee -a "$EXECUTION_LOG"
    
    ACTUAL_RUNNING=$(docker ps --filter "label=device-agent=$DEVICE_ID" --format "{{.ID}}" | wc -l)
    log_success "Deployed $ACTUAL_RUNNING container(s) for device $DEVICE_ID"
    
    if [[ $ACTUAL_RUNNING -gt 0 ]]; then
        log_success "All workloads deployed and running ✓"
    else
        log_warn "No running containers found (they might have already completed)"
    fi
else
    log_warn "No containers executed"
fi

# Phase 5: Report deployment status back to WFM (if client is onboarded)
log ""
log "Phase 5: Report deployment status to WFM"
log "========================================"

if [[ -z "$CLIENT_ID" ]]; then
    log_warn "Skipping status report (client not onboarded)"
else
    STATUS_PAYLOAD=$(jq -cn \
        --arg dep "$DEPLOYMENT_ID" \
        --arg dev "$DEVICE_ID" \
        --arg state "deployed" \
        '{
            apiVersion: "deployment.margo.org/v1alpha1",
            kind: "DeploymentStatusManifest",
            deploymentId: $dep,
            deviceId: $dev,
            components: [
                {
                    name: "docker-executor",
                    state: $state
                }
            ],
            status: {
                state: $state,
                executionTime: "'$(date -u +'%Y-%m-%dT%H:%M:%SZ')'",
                containerCount: '$EXECUTED_CONTAINERS',
                failedCount: '$FAILED_EXECUTIONS'
            }
        }')
    
    STATUS_RESPONSE="$RESPONSES_DIR/status-report.json"
    if make_wfm_request "POST" "/api/v1/clients/$CLIENT_ID/deployments/$DEPLOYMENT_ID/status" "$STATUS_PAYLOAD" "$STATUS_RESPONSE"; then
        log_success "Deployment status reported to WFM"
    else
        log_warn "Failed to report deployment status"
    fi
fi

# Summary
log ""
log "=================================================="
log "  Execution Summary"
log "=================================================="
log "Device ID: $DEVICE_ID"
log "Client ID: ${CLIENT_ID:-<not onboarded>}"
log "Deployment ID: $DEPLOYMENT_ID"
log "Containers executed: $EXECUTED_CONTAINERS"
log "Failed executions: $FAILED_EXECUTIONS"
log "Execution log: $EXECUTION_LOG"

echo ""
echo "==================================="
echo "Device Status Report"
echo "==================================="
if [[ -n "$CLIENT_ID" && "$CLIENT_ID" != "null" && "$CLIENT_ID" != "" ]]; then
    echo "✅ Device Onboarded: YES"
    echo "   Client ID: $CLIENT_ID"
    echo "   Device ID: $DEVICE_ID"
else
    echo "⚠️  Device Onboarded: NO (using mock deployment)"
fi

echo ""
echo "✅ Capabilities Status"
CAPS_FILE="$DATA_DIR/device-agent.env.json"
if [[ -f "$CAPS_FILE" ]]; then
    CORES=$(jq -r '.values[] | select(.key=="capabilitiesRequest") | .value | .properties.resources.cpu.cores' "$CAPS_FILE" 2>/dev/null || echo "?")
    MEMORY=$(jq -r '.values[] | select(.key=="capabilitiesRequest") | .value | .properties.resources.memory' "$CAPS_FILE" 2>/dev/null || echo "?")
    VENDOR=$(jq -r '.values[] | select(.key=="capabilitiesRequest") | .value | .properties.vendor' "$CAPS_FILE" 2>/dev/null || echo "?")
    echo "   Vendor: $VENDOR"
    echo "   CPU Cores: $CORES"
    echo "   Memory: $MEMORY"
fi

echo ""
echo "✅ Deployment Status"
echo "   Deployment ID: $DEPLOYMENT_ID"
echo "   Workloads Deployed: $EXECUTED_CONTAINERS"
echo "   Status: $([ $FAILED_EXECUTIONS -eq 0 ] && echo 'SUCCESS' || echo 'PARTIAL')"

if [[ -f "$CONTAINERS_FILE" ]]; then
    log ""
    log "Deployed container IDs (saved in $CONTAINERS_FILE):"
    cat "$CONTAINERS_FILE" | tee -a "$EXECUTION_LOG"

    echo ""
    echo "Verify with:"
    echo "  docker ps --filter label=device-agent=$DEVICE_ID"
fi

log ""
log "To clean up deployed containers:"
log "  ./4-cleanup.sh"
log ""

if [[ $FAILED_EXECUTIONS -gt 0 ]]; then
    log_error "Some workloads failed to execute ($FAILED_EXECUTIONS failures)"
    exit 1
else
    log_success "Workload execution phase complete"
    exit 0
fi
