#!/bin/bash

# Generate self-signed TLS certificates for mock WFM server
# Usage: bash generate-certs.sh [output-dir] [host-ip/hostname]

set -e

OUTPUT_DIR="${1:-.}"
SERVER_HOST="${2:-localhost}"
DAYS_VALID=365

is_ipv4_address() {
  [[ "$1" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]
}

echo "🔐 Generating TLS certificates for Mock WFM Server..."
echo "   Output directory: $OUTPUT_DIR"
echo "   Server host: $SERVER_HOST"
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
  -subj "/C=US/ST=State/L=City/O=Margo/OU=WFM/CN=Mock-WFM-CA" 2>/dev/null

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

echo ""
echo "✅ Certificate generation complete!"
echo ""
echo "Generated files:"
ls -lh "$OUTPUT_DIR"/ca-*.pem "$OUTPUT_DIR"/server-*.pem 2>/dev/null
echo ""
echo "Certificate details:"
echo "  CA Certificate:"
openssl x509 -in "$OUTPUT_DIR/ca-cert.pem" -noout -text 2>/dev/null | grep -A 1 "Issuer:\|Subject:\|Not"
echo ""
echo "  Server Certificate:"
openssl x509 -in "$OUTPUT_DIR/server-cert.pem" -noout -text 2>/dev/null | grep -A 1 "Issuer:\|Subject:\|Not\|DNS:\|IP:"
echo ""
echo "📝 Next steps:"
echo "  1. Copy 'ca-cert.pem' to: ~/sandbox/poc/device/agent/config/"
echo "  2. Copy 'server-cert.pem' and 'server-key.pem' to mock-wfm directory"
echo "  3. Update mock server to use HTTPS"
echo "  4. Configure device-agent with correct server URL"
