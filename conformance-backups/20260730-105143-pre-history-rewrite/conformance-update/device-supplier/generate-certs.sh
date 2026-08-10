#!/bin/bash

# Generate TLS certificates for Margo Mock WFM Server
# This creates:
#   - CA certificate (ca-cert.pem) - for device-agent to validate server
#   - CA private key (ca-private.key) - for signing server certificate
#   - Server certificate (server.crt) - TLS certificate for HTTPS
#   - Server private key (server.key) - TLS private key for HTTPS
#
# Usage: bash generate-certs.sh [output-dir] [host-ip/hostname]
# Default output: ./certs
# Example: bash generate-certs.sh ./certs localhost
# Example: bash generate-certs.sh ./certs 192.168.1.100

set -e

OUTPUT_DIR="${1:-./certs}"
# Auto-detect host IP if not provided; fallback to localhost
if [[ -z "$2" ]]; then
  # Try to detect the host IP address
  HOST_IP=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "")
  SERVER_HOST="${HOST_IP:-localhost}"
else
  SERVER_HOST="$2"
fi
DAYS_VALID=365

is_ipv4_address() {
  [[ "$1" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]
}

echo "🔐 Generating TLS Certificates for Margo Mock WFM Server"
echo "════════════════════════════════════════════════════════"
echo "   Output directory: $OUTPUT_DIR"
echo "   Server host: $SERVER_HOST"
echo "   Validity: $DAYS_VALID days"
echo ""

# Create output directory if it doesn't exist
mkdir -p "$OUTPUT_DIR"

# 1. Generate CA private key
echo "[1/5] Generating CA private key..."
openssl genrsa -out "$OUTPUT_DIR/ca-key.pem" 2048 2>/dev/null

# 2. Generate CA certificate
echo "[2/5] Generating CA certificate..."
openssl req -new -x509 -days $DAYS_VALID -key "$OUTPUT_DIR/ca-key.pem" \
  -out "$OUTPUT_DIR/ca-cert.pem" \
  -subj "/C=IN/ST=GGN/L=Sector48/O=Margo/OU=WFM/CN=Mock-WFM-CA" 2>/dev/null

# 3. Generate server private key
echo "[3/5] Generating server private key..."
openssl genrsa -out "$OUTPUT_DIR/server-key.pem" 2048 2>/dev/null

# 4. Generate server CSR (Certificate Signing Request)
echo "[4/5] Generating server CSR..."
if is_ipv4_address "$SERVER_HOST"; then
  HOST_SAN_ENTRY="IP.2 = $SERVER_HOST"
else
  HOST_SAN_ENTRY="DNS.3 = $SERVER_HOST"
fi

cat > "$OUTPUT_DIR/san.conf" << EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
C = US
ST = State
L = City
O = Margo
OU = WFM
CN = $SERVER_HOST

[v3_req]
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = 127.0.0.1
IP.1 = 127.0.0.1
$HOST_SAN_ENTRY
EOF

openssl req -new -key "$OUTPUT_DIR/server-key.pem" \
  -out "$OUTPUT_DIR/server.csr" \
  -config "$OUTPUT_DIR/san.conf" 2>/dev/null

# 5. Generate server certificate signed by CA
echo "[5/5] Generating server certificate..."
openssl x509 -req -in "$OUTPUT_DIR/server.csr" \
  -CA "$OUTPUT_DIR/ca-cert.pem" -CAkey "$OUTPUT_DIR/ca-key.pem" \
  -CAcreateserial -out "$OUTPUT_DIR/server-cert.pem" \
  -days $DAYS_VALID \
  -extensions v3_req -extfile "$OUTPUT_DIR/san.conf" 2>/dev/null

# Clean up temporary files
rm -f "$OUTPUT_DIR/server.csr" "$OUTPUT_DIR/san.conf"

# 6. Generate device certificate for conformance testing
echo "[6/6] Generating device test certificates..."

# Device valid certificate - used for positive test scenarios
openssl genrsa -out "$OUTPUT_DIR/device-key.pem" 2048 2>/dev/null
openssl req -new -x509 -days $DAYS_VALID \
  -key "$OUTPUT_DIR/device-key.pem" \
  -out "$OUTPUT_DIR/device-cert.pem" \
  -subj "/C=IN/ST=GGN/L=Sector48/O=AcmeCorp/OU=Devices/CN=device-001" 2>/dev/null

echo ""
echo "✅ Certificate generation complete!"
echo ""
echo "📋 Generated files:"
ls -lh "$OUTPUT_DIR"/ca-cert.pem "$OUTPUT_DIR"/ca-key.pem "$OUTPUT_DIR"/server-cert.pem "$OUTPUT_DIR"/server-key.pem 2>/dev/null
ls -lh "$OUTPUT_DIR"/device-cert.pem "$OUTPUT_DIR"/device-key.pem 2>/dev/null
echo ""
echo "🔍 Certificate details:"
echo "  CA Certificate:"
openssl x509 -in "$OUTPUT_DIR/ca-cert.pem" -noout -text 2>/dev/null | grep -A 1 "Issuer:\|Subject:\|Not"
echo ""
echo "  Server Certificate:"
openssl x509 -in "$OUTPUT_DIR/server-cert.pem" -noout -text 2>/dev/null | grep -A 1 "Issuer:\|Subject:\|Not\|DNS:\|IP:"
echo ""
echo "════════════════════════════════════════════════════════"
echo "📝 NEXT STEPS FOR DEPLOYMENT"
echo "════════════════════════════════════════════════════════"
echo ""
echo "1️⃣  Mock WFM Server Setup (on this machine):"
echo "    ✓ Certificates ready in: $OUTPUT_DIR"
echo "    ✓ Start mock server: cd $OUTPUT_DIR/.. && go run server.go"
echo "    ✓ Server will use: server-cert.pem + server-key.pem for HTTPS"
echo ""
echo "2️⃣  Device-Agent VM Setup (copy ca-cert.pem to device VM):"
echo ""
echo "    Option A - Direct copy (if on same network):"
echo "      mkdir -p /root/certs"
echo "      scp $OUTPUT_DIR/ca-cert.pem root@DEVICE_VM:/root/certs/"
echo ""
echo "    Option B - Manual copy:"
echo "      1. Copy file from: $OUTPUT_DIR/ca-cert.pem"
echo "      2. Paste to device VM at: /root/certs/ca-cert.pem"
echo ""
echo "    Option C - Copy to home directory for auto-discovery:"
echo "      mkdir -p ~./certs"
echo "      cp $OUTPUT_DIR/ca-cert.pem ~./certs/"
echo ""
echo "3️⃣  Verify setup:"
echo "    On Device VM: ls -la /root/certs/ca-cert.pem"
echo "    Should show: ca-cert.pem (1294 bytes)"
echo ""
echo "════════════════════════════════════════════════════════"
echo "✅ All certificates generated successfully!"
echo "════════════════════════════════════════════════════════"
