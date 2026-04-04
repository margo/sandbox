#!/bin/bash

set -e

# Mock WFM Server Setup Script
# This script generates certificates and sets up the mock WFM server
# similar to how Prism is set up in margo-conformance

echo "🚀 Setting up Mock WFM Server..."

# ========================================
# Configuration
# ========================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SCRIPT_DIR"

# Get the machine IP automatically, or use provided parameter
MACHINE_IP="${1:-$(hostname -I | awk '{print $1}')}"

# Fallback to localhost if hostname -I doesn't work
if [ -z "$MACHINE_IP" ]; then
  MACHINE_IP="127.0.0.1"
fi

CERT_DIR="$PROJECT_DIR/certs"
CERTS_HOME="$HOME/certs"

echo "Machine IP: $MACHINE_IP"
echo "Certificate directory: $CERT_DIR"
echo "Home certs directory: $CERTS_HOME"
echo ""

# ========================================
# Step 1: Generate Certificates
# ========================================
echo "[1/3] Generating certificates..."

mkdir -p "$CERT_DIR"

cd "$CERT_DIR"

# Generate CA private key
echo "  - Generating CA private key..."
openssl genrsa -out ca-key.pem 2048 2>/dev/null

# Generate CA certificate
echo "  - Generating CA certificate..."
openssl req -x509 -new -nodes \
  -key ca-key.pem \
  -sha256 -days 365 \
  -out ca-cert.pem \
  -subj "/CN=Mock-WFM-CA" 2>/dev/null

# Create server config with SANs
echo "  - Creating server configuration..."
cat > server.conf <<EOF
[ req ]
default_bits       = 2048
distinguished_name = req_distinguished_name
req_extensions     = req_ext
prompt = no

[ req_distinguished_name ]
CN = $MACHINE_IP

[ req_ext ]
subjectAltName = @alt_names

[ alt_names ]
DNS.1 = localhost
DNS.2 = $MACHINE_IP
IP.1  = 127.0.0.1
IP.2  = $MACHINE_IP
EOF

# Generate server private key
echo "  - Generating server private key..."
openssl genrsa -out server-key.pem 2048 2>/dev/null

# Generate server CSR
echo "  - Generating server certificate signing request..."
openssl req -new -key server-key.pem -out server.csr -config server.conf 2>/dev/null

# Sign server certificate with CA
echo "  - Signing server certificate..."
openssl x509 -req \
  -in server.csr \
  -CA ca-cert.pem \
  -CAkey ca-key.pem \
  -CAcreateserial \
  -out server-cert.pem \
  -days 365 \
  -sha256 \
  -extensions req_ext \
  -extfile server.conf 2>/dev/null

# Clean up CSR and config
rm -f server.csr server.conf

cd "$PROJECT_DIR"

echo "✅ Certificates generated successfully"
echo ""

# ========================================
# Step 2: Copy Certificates to ~/.certs
# ========================================
echo "[2/3] Copying certificates to home directory..."

mkdir -p "$CERTS_HOME"

echo "  - Copying CA certificate..."
cp "$CERT_DIR/ca-cert.pem" "$CERTS_HOME/ca-cert.pem"

echo "  - Copying server certificate..."
cp "$CERT_DIR/server-cert.pem" "$CERTS_HOME/server-cert.pem"

echo "  - Copying server key..."
cp "$CERT_DIR/server-key.pem" "$CERTS_HOME/server-key.pem"

echo "✅ Certificates copied to $CERTS_HOME"
echo ""

# ========================================
# Step 3: Update Configuration Files
# ========================================
echo "[3/3] Updating configuration files..."

# Update device agent config if it exists
DEVICE_CONFIG="/home/margo/nitin/sandbox/poc/device/agent/config/config.yaml"

if [ -f "$DEVICE_CONFIG" ]; then
  echo "  - Updating device agent configuration..."
  
  # Update WFM URL to use correct IP and port 9090 (HTTPS)
  sed -i "s|sbiUrl:.*|sbiUrl: https://$MACHINE_IP:9090/v1alpha2/margo|" "$DEVICE_CONFIG"
  
  # Update CA certificate path
  sed -i 's|path: "./config/ca-cert.pem"|path: "/root/certs/ca-cert.pem"|' "$DEVICE_CONFIG"
  
  echo "  - Device agent config updated"
else
  echo "  ⚠️  Device agent config not found at $DEVICE_CONFIG"
fi

echo "✅ Configuration updated"
echo ""

# ========================================
# Verification
# ========================================
echo "📋 Certificate Verification:"
echo ""
echo "Server Certificate Details:"
openssl x509 -in "$CERT_DIR/server-cert.pem" -noout -text 2>/dev/null | grep -E "Subject:|Issuer:|Not Before:|Not After:|DNS:|IP Address:" | head -20

echo ""
echo "✅ Setup complete!"
echo ""
echo "📝 Next steps:"
echo "  1. Navigate to: cd $PROJECT_DIR"
echo "  2. Build mock server: go build -o mock-wfm"
echo "  3. Run mock server: ./mock-wfm"
echo ""
echo "🌐 Access URLs:"
echo "  HTTP:  http://$MACHINE_IP:8090/v1alpha2/margo"
echo "  HTTPS: https://$MACHINE_IP:9090/v1alpha2/margo"
echo ""
echo "📂 Certificate locations:"
echo "  Server: $CERT_DIR/server-cert.pem"
echo "  Key:    $CERT_DIR/server-key.pem"
echo "  CA:     $CERT_DIR/ca-cert.pem"
echo "  Home:   $CERTS_HOME/"
echo ""
