#!/bin/bash

# Start Mock WFM Server for Device Agent testing.
# This script starts the mock server with HTTPS support on port 9090.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Mock WFM Server Startup${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Check if binary exists
if [[ ! -f "./mock-wfm" ]]; then
    echo -e "${RED}❌ Error: mock-wfm binary not found${NC}"
    echo "Please build the server first:"
    echo "  go build -o mock-wfm ."
    exit 1
fi

# Check if certificates exist
if [[ ! -f "./server-cert.pem" || ! -f "./server-key.pem" ]]; then
    echo -e "${YELLOW}⚠️  Warning: TLS certificates not found${NC}"
    echo "Generating certificates..."
    bash generate-certs.sh . localhost
    echo ""
fi

# Check if CA certificate exists
if [[ -f "./ca-cert.pem" ]]; then
    echo -e "${GREEN}✅ CA Certificate found${NC}"
    echo "   File: $SCRIPT_DIR/ca-cert.pem"
    echo ""
    echo "📝 To use with device-agent on different VM:"
    echo "   1. Copy ca-cert.pem to device-agent config directory:"
    echo "      scp ca-cert.pem user@device-vm:~/sandbox/poc/device/agent/config/"
    echo ""
    echo "   2. Update device-agent config at ~/sandbox/poc/device/agent/config/config.yaml:"
    echo "      sbiUrl: https://<mock-wfm-ip>:9090/v1alpha2/margo"
    echo "      caKeyRef.path: ./config/ca-cert.pem"
    echo ""
fi

# Kill any existing process on port 9090
if lsof -Pi :9090 -sTCP:LISTEN -t >/dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  Port 9090 is already in use. Stopping existing process...${NC}"
    pkill -f "./mock-wfm" || true
    sleep 1
fi

# Start the server
echo -e "${GREEN}🚀 Starting Mock WFM Server...${NC}"
echo ""

./mock-wfm &
SERVER_PID=$!

# Wait for server to start
echo "Waiting for server to start..."
sleep 2

# Check if server is running
if kill -0 $SERVER_PID 2>/dev/null; then
    echo -e "${GREEN}✅ Server started successfully (PID: $SERVER_PID)${NC}"
    echo ""
    echo "📍 Server URLs:"
    echo "   HTTP:  http://localhost:8090/v1alpha2/margo"
    echo "   HTTPS: https://localhost:9090/v1alpha2/margo"
    echo ""
    echo "🔐 HTTPS Configuration:"
    echo "   Port: 9090"
    echo "   Certificate: $SCRIPT_DIR/server-cert.pem"
    echo "   Key: $SCRIPT_DIR/server-key.pem"
    echo "   CA Certificate: $SCRIPT_DIR/ca-cert.pem"
    echo ""
    echo "🧪 To run integration tests:"
    echo "   bash device-agent-integration-test.sh localhost 9090 ./ca-cert.pem"
    echo ""
    echo "Press Ctrl+C to stop the server..."
    echo ""
    
    # Wait for server process
    wait $SERVER_PID
else
    echo -e "${RED}❌ Failed to start server${NC}"
    exit 1
fi
