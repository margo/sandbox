#!/bin/bash

set -e


COLLECTION="postman_collection.json"

REPORT="report_$(date +%s).html"

echo "Generating dynamic CSR..."

DEVICE_ID="device-$(date +%s)"

openssl genrsa -out device.key 2048 >/dev/null 2>&1
openssl req -new -key device.key -out device.csr \
  -subj "/CN=$DEVICE_ID" >/dev/null 2>&1

CERT=$(awk 'NF {sub(/\r/, ""); printf "%s\\n",$0;}' device.csr)

# -------- Validate input --------
if [ ! -f "$COLLECTION" ]; then
    echo "Collection file not found: $COLLECTION"
    exit 1
fi

# -------- Install Newman --------
if ! command -v newman >/dev/null 2>&1; then
    echo "Installing Newman..."
    sudo npm install -g newman
else
    echo "Newman already installed"
fi

# -------- Install Reporter --------
if ! npm list -g newman-reporter-htmlextra >/dev/null 2>&1; then
    echo "Installing htmlextra reporter..."
    sudo npm install -g newman-reporter-htmlextra
else
    echo "htmlextra reporter already installed"
fi

# -------- Run Collection --------
echo "Running Postman collection..."

newman run "$COLLECTION" \
  --env-var "certificate=$CERT" \
  --env-var "apiVersion=onboarding.margo.org/v1alpha1" \
  --env-var "clientId=$DEVICE_ID" \
  --insecure \
  -r cli,htmlextra \
  --reporter-htmlextra-export "$REPORT"

echo "----------------------------------------"
echo "Execution Complete"
echo "Report generated: $REPORT"
echo "----------------------------------------"
