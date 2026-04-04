#!/bin/bash

# Generate test device certificate and CSR for mock server testing

mkdir -p certs
cd certs

echo "🔐 Generating device private key..."
openssl genrsa -out device.key 2048

echo "📝 Generating device CSR..."
openssl req -new -key device.key -out device.csr \
  -subj "/CN=device-001"

echo "✅ Device CSR generated"
echo ""
echo "CSR content (for X-Payload-Signature header):"
echo "================================================"
awk 'NF {sub(/\r/, ""); printf "%s\\n",$0;}' device.csr
echo ""

echo ""
echo "Generate device certificate (signed by CA):"
echo "================================================"
echo "openssl x509 -req -in device.csr -signkey device.key -out device.crt -days 365"
openssl x509 -req -in device.csr -signkey device.key -out device.crt -days 365

echo ""
echo "✅ Certificates generated in ./certs/"
echo "   - device.key (private key)"
echo "   - device.csr (certificate signing request)"
echo "   - device.crt (certificate)"
