#!/bin/bash

set -e

# Configuration
TMP_SPEC="snapshot.spec.yaml"
MIS_SPEC_FILE=("spec.yaml")

SPEC_URL="https://raw.githubusercontent.com/margo/specification/refs/heads/pre-draft/system-design/specification/identity/trust-bundle-api-1.0.0-rc.2.yaml"
curl -sSL -o "$TMP_SPEC" \
  "$SPEC_URL"

OUTPUT_DIR="./generatedCode"
MIS_PACKAGE_NAME="github.com/margo/sandbox/mis/standard/generatedCode"


# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

command_exists() { command -v "$1" >/dev/null 2>&1; }

check_prerequisites() {
    log_info "Checking prerequisites..."

    if ! command_exists go; then
        log_error "Go is not installed. Please install Go."
        exit 1
    fi

    log_success "Go is available: $(go version)"

    if [ ! -f "$MIS_SPEC_FILE" ]; then
        log_error "OpenAPI spec file '$MIS_SPEC_FILE' not found!"
        exit 1
    fi

    log_success "OpenAPI spec file found: $MIS_SPEC_FILE"

    # if [ ! -f "$WFM_SBI_SPEC_FILE" ]; then
    #     log_error "OpenAPI spec file '$WFM_SBI_SPEC_FILE' not found!"
    #     exit 1
    # fi

    # log_success "OpenAPI spec file found: $WFM_SBI_SPEC_FILE"
}

install_tools() {
    log_info "Installing oapi-codegen..."
    go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
    export PATH="$PATH:$(go env GOPATH)/bin"
    log_success "oapi-codegen installed"
}


generate_code() {
    log_info "Generating Go code..."

    # Clean and create output directory
    rm -rf "$OUTPUT_DIR"
    mkdir -p "$OUTPUT_DIR"

    # Generate models first
    log_info "Generating models..."
    oapi-codegen -generate types,skip-prune -package generatedCode "$MIS_SPEC_FILE" > "$OUTPUT_DIR/models.go"

    # Generate client
    log_info "Generating client..."
    oapi-codegen -generate client -package generatedCode "$MIS_SPEC_FILE" > "$OUTPUT_DIR/client.go"

    # Generate server
    # log_info "Generating server..."
    # oapi-codegen -generate server -package generatedCode "$MIS_SPEC_FILE" > "$OUTPUT_DIR/server.go"

    # Generate models first
    # log_info "Generating models..."
    # oapi-codegen -generate types,skip-prune -package sbi "$WFM_SBI_SPEC_FILE" > "$OUTPUT_DIR/wfm/sbi/models.go"

    # # Generate client
    # log_info "Generating client..."
    # oapi-codegen -generate client -package sbi "$WFM_SBI_SPEC_FILE" > "$OUTPUT_DIR/wfm/sbi/client.go"

    # Generate server (optional)
    # log_info "Generating server..."
    # oapi-codegen -generate server -package server "$MIS_SPEC_FILE" > "$OUTPUT_DIR/server/server.go"

    # Fix imports after generation
    # fix_imports_simple

    # Initialize modules
    # (cd "$OUTPUT_DIR" && go mod init "$MIS_PACKAGE_NAME" && go mod tidy)

    log_success "Code generation completed!"
}

# Alternative simpler approach for fixing imports
# fix_imports_simple() {
#     log_info "Fixing imports (simple approach)..."

#     # For client
#     if [ -f "$OUTPUT_DIR/wfm/sbi/wfmNbiClient.go" ]; then
#         # Check if import is missing
#         if ! grep -q "\"$MIS_PACKAGE_NAME/models\"" "$OUTPUT_DIR/wfm/sbi/wfmNbiClient.go"; then
#             # Add import after package line
#             sed -i '/^package client$/a\\nimport . "'"$MIS_PACKAGE_NAME"'/models"' "$OUTPUT_DIR/wfm/sbi/wfmNbiClient.go"
#             log_success "Added import to client"
#         fi
#     fi

#     # For client
#     if [ -f "$OUTPUT_DIR/client/wfmNbiClient.go" ]; then
#         # Check if import is missing
#         if ! grep -q "\"$MIS_PACKAGE_NAME/models\"" "$OUTPUT_DIR/client/wfmNbiClient.go"; then
#             # Add import after package line
#             sed -i '/^package client$/a\\nimport . "'"$MIS_PACKAGE_NAME"'/models"' "$OUTPUT_DIR/client/wfmNbiClient.go"
#             log_success "Added import to client"
#         fi
#     fi

#     # For client
#     if [ -f "$OUTPUT_DIR/client/wfmNbiClient.go" ]; then
#         # Check if import is missing
#         if ! grep -q "\"$MIS_PACKAGE_NAME/models\"" "$OUTPUT_DIR/client/wfmNbiClient.go"; then
#             # Add import after package line
#             sed -i '/^package client$/a\\nimport . "'"$MIS_PACKAGE_NAME"'/models"' "$OUTPUT_DIR/client/wfmNbiClient.go"
#             log_success "Added import to client"
#         fi
#     fi

#     # For server
#     # if [ -f "$OUTPUT_DIR/server/server.go" ]; then
#     #     if ! grep -q "\"$MIS_PACKAGE_NAME/models\"" "$OUTPUT_DIR/server/server.go"; then
#     #         sed -i '/^package server$/a\\nimport . "'"$MIS_PACKAGE_NAME"'/models"' "$OUTPUT_DIR/server/server.go"
#     #         log_success "Added import to server"
#     #     fi
#     # fi
# }

main() {
    check_prerequisites
    install_tools
    generate_code

    echo "Generated files:"
    echo "- Models: $OUTPUT_DIR/models/"
    echo "- Client: $OUTPUT_DIR/client/"
    # echo "- Server: $OUTPUT_DIR/server/"
    # echo "- Server: $OUTPUT_DIR/server/"

    # Verify the imports work
    log_info "Verifying generated code..."
    for dir in models client; do
        if [ -d "$OUTPUT_DIR/$dir" ]; then
            (cd "$OUTPUT_DIR/$dir" && go build . && log_success "$dir builds successfully") || log_error "$dir failed to build"
        fi
    done
}

main "$@"
