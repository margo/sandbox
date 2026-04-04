#!/bin/bash

set -e

# Mock WFM Server Cleanup Script
# Removes certificates and stops the mock server

echo "🧹 Starting cleanup..."

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CERT_DIR="$PROJECT_DIR/certs"
CERTS_HOME="$HOME/certs"

# ========================================
# Step 1: Stop Mock WFM Server
# ========================================
echo "Stopping mock WFM server..."

# Find and kill mock-wfm processes
MOCK_WFM_PIDS=$(ps aux | grep "mock-wfm" | grep -v grep | awk '{print $2}' || true)

if [ -n "$MOCK_WFM_PIDS" ]; then
  echo "$MOCK_WFM_PIDS" | xargs kill -9 2>/dev/null || true
  echo "✅ Mock WFM server stopped"
else
  echo "ℹ️  Mock WFM server not running"
fi

# ========================================
# Step 2: Remove Local Certificates
# ========================================
echo "Removing local certificates..."

if [ -d "$CERT_DIR" ]; then
  rm -rf "$CERT_DIR"
  echo "✅ Local certificates removed"
else
  echo "ℹ️  No local certs folder found"
fi

# ========================================
# Step 3: Remove Certificates from Home
# ========================================
echo "Cleaning home certificates..."

if [ -f "$CERTS_HOME/ca-cert.pem" ]; then
  rm -f "$CERTS_HOME/ca-cert.pem"
  echo "✅ CA certificate removed"
fi

if [ -f "$CERTS_HOME/server-cert.pem" ]; then
  rm -f "$CERTS_HOME/server-cert.pem"
  echo "✅ Server certificate removed"
fi

if [ -f "$CERTS_HOME/server-key.pem" ]; then
  rm -f "$CERTS_HOME/server-key.pem"
  echo "✅ Server key removed"
fi

# ========================================
# Step 4: Remove Binary
# ========================================
echo "Removing binary..."

if [ -f "$PROJECT_DIR/mock-wfm" ]; then
  rm -f "$PROJECT_DIR/mock-wfm"
  echo "✅ Binary removed"
else
  echo "ℹ️  No binary found"
fi

# ========================================
# Step 5: Remove Logs (Optional)
# ========================================
echo "Removing logs..."

if [ -f "$PROJECT_DIR/server.log" ]; then
  rm -f "$PROJECT_DIR/server.log"
  echo "✅ server.log removed"
fi

# ========================================
# Done
# ========================================
echo ""
echo "✅ Cleanup complete!"
echo ""
echo "📝 To set up again, run: bash setup.sh [MACHINE_IP]"
