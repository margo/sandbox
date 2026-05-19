#!/bin/bash

set -e

echo "========================================"
echo " Margo Portman Setup & Collection Script "
echo "========================================"

# -------- CONFIG --------
SPEC_URL="https://raw.githubusercontent.com/margo/specification/pre-draft/system-design/specification/margo-management-interface/workload-management-api-1.0.0.yaml"
SPEC_FILE="spec.yaml"
COLLECTION_FILE="postman_collection.json"
# DEFAULT_BASE_URL="https://localhost:8082/v1alpha2/margo"
# BASE_URL="${1:-$DEFAULT_BASE_URL}"
# echo "Using BASE_URL: $BASE_URL"

read -p "Enter Symphony Server BASE_URL: " BASE_URL

if [ -z "$BASE_URL" ]; then
    echo "❌ BASE_URL cannot be empty"
    exit 1
fi

# -------- Install Node js --------
if ! command -v node >/dev/null 2>&1; then
    echo "Installing Node.js..."

    curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
    sudo apt-get install -y nodejs

else
    echo "Node.js already installed: $(node -v)"
fi

# -------- Install npm --------
if ! command -v npm >/dev/null 2>&1; then
    echo "Installing npm..."
    sudo apt-get install -y npm
else
    echo "npm already installed: $(npm -v)"
fi

# -------- Install Portman --------
if ! command -v portman >/dev/null 2>&1; then
    echo "Installing Portman..."
    sudo npm install -g @apideck/portman
else
    echo "Portman already installed"
fi

# --------  Fetch OpenAPI Spec --------
echo "Downloading OpenAPI spec..."
curl -s "$SPEC_URL" -o "$SPEC_FILE"

if [ ! -f "$SPEC_FILE" ]; then
    echo "❌ Failed to download spec"
    exit 1
fi

echo "Spec downloaded: $SPEC_FILE"

# -------- Generate Postman Collection --------
echo "Generating Postman collection..."

portman \
  -l spec.yaml \
  -o "$COLLECTION_FILE" \
  -b "$BASE_URL"

echo "----------------------------------------"
echo "Done!"
echo "Postman Collection: $COLLECTION_FILE"
echo "----------------------------------------"
