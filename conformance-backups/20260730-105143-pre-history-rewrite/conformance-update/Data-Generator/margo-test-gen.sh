#!/usr/bin/env bash

################################################################################
# MARGO Test Case Generator - Data-Generator CLI
################################################################################
# Purpose: Generate and prepare test cases for MARGO personas
# Personas: WFM Supplier, Device Supplier
# 
# For WFM Supplier: Uses Portman to generate postman_collection.json from OpenAPI
# For Device Supplier: References existing test-scenarios.json with assertions
################################################################################

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFORMANCE_DIR="$(cd "$ROOT_DIR/.." && pwd)"
WFM_DIR="$CONFORMANCE_DIR/wfm-supplier"
DEVICE_DIR="$CONFORMANCE_DIR/device-supplier"
DATA_GEN_DIR="$ROOT_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# ============================================
# Utility Functions
# ============================================
log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $*"
}

success() {
    echo -e "${GREEN}✅ $*${NC}"
}

error() {
    echo -e "${RED}❌ $*${NC}"
    exit 1
}

warning() {
    echo -e "${YELLOW}⚠️  $*${NC}"
}

# ============================================
# WFM Supplier - Generate test cases via Portman
# ============================================
wfm_generate() {
    local base_url="${1:-}"
    
    if [[ -z "$base_url" ]]; then
        read -p "Enter WFM Base URL (e.g., https://symphony.machine:8082/v1alpha2/margo): " base_url
    fi
    
    if [[ -z "$base_url" ]]; then
        error "WFM Base URL is required"
    fi
    
    log "🔄 Generating test cases for WFM Supplier from OpenAPI spec..."
    
    # Ensure output directory exists
    mkdir -p "$DATA_GEN_DIR/wfm-supplier"
    
    # Run Portman to generate postman collection from OpenAPI
    (cd "$WFM_DIR" && bash 1-setup_portman.sh "$base_url") || error "Portman generation failed"
    
    # Copy generated postman_collection.json to Data-Generator directory
    if [[ -f "$WFM_DIR/postman_collection.json" ]]; then
        cp "$WFM_DIR/postman_collection.json" "$DATA_GEN_DIR/wfm-supplier/"
        success "WFM test cases generated: $DATA_GEN_DIR/wfm-supplier/postman_collection.json"
    else
        error "postman_collection.json not found after Portman generation"
    fi
    
    # Also copy iteration/data files if they exist
    if [[ -d "$WFM_DIR/newman-data" ]]; then
        cp -r "$WFM_DIR/newman-data" "$DATA_GEN_DIR/wfm-supplier/" 2>/dev/null || true
        success "Newman data files copied"
    fi
}

# ============================================
# Device Supplier - Reference existing test scenarios
# ============================================
device_generate() {
    log "📋 Preparing Device Supplier test scenarios..."
    
    # Ensure output directory exists
    mkdir -p "$DATA_GEN_DIR/device-supplier"
    
    # Device supplier uses pre-existing test scenarios
    if [[ -f "$DEVICE_DIR/device-scenarios/test-scenarios.json" ]]; then
        cp "$DEVICE_DIR/device-scenarios/test-scenarios.json" "$DATA_GEN_DIR/device-supplier/"
        success "Device test scenarios ready: $DATA_GEN_DIR/device-supplier/test-scenarios.json"
    else
        error "test-scenarios.json not found in device-supplier"
    fi
    
    # Copy data files
    if [[ -d "$DEVICE_DIR/data" ]]; then
        cp -r "$DEVICE_DIR/data" "$DATA_GEN_DIR/device-supplier/" 2>/dev/null || true
        success "Device data files copied"
    fi
}

# ============================================
# Show usage
# ============================================
usage() {
    cat <<'EOF'
MARGO Test Case Generator (Data-Generator CLI)

Usage:
  ./margo-test-gen.sh [PERSONA] [OPTIONS]

Personas:
  1, wfm              Generate WFM Supplier test cases via Portman
  2, device           Prepare Device Supplier test scenarios
  interactive         Interactive menu (default)

Examples:
  ./margo-test-gen.sh wfm https://symphony.machine:8082/v1alpha2/margo
  ./margo-test-gen.sh device
  ./margo-test-gen.sh interactive
  ./margo-test-gen.sh                (runs interactive mode)

Output Locations:
  WFM:    ./Data-Generator/wfm-supplier/postman_collection.json
  Device: ./Data-Generator/device-supplier/test-scenarios.json

EOF
}

# ============================================
# Interactive Menu
# ============================================
interactive_menu() {
    while true; do
        clear
        echo "======================================================================"
        echo "  MARGO Test Case Generator (Data-Generator)"
        echo "======================================================================"
        echo ""
        echo "Select Persona:"
        echo "  1. WFM Supplier    - Generate test cases from OpenAPI spec"
        echo "  2. Device Supplier - Prepare test scenarios"
        echo "  3. Exit"
        echo ""
        read -p "Enter choice (1-3): " choice
        
        case "$choice" in
            1|wfm)
                read -p "Enter WFM Base URL (default: https://symphony.machine:8082/v1alpha2/margo): " url
                wfm_generate "${url:-https://symphony.machine:8082/v1alpha2/margo}"
                read -p "Press Enter to continue..." _
                ;;
            2|device)
                device_generate
                read -p "Press Enter to continue..." _
                ;;
            3|exit)
                log "Exiting..."
                exit 0
                ;;
            *)
                error "Invalid choice"
                ;;
        esac
    done
}

# ============================================
# Main
# ============================================
main() {
    local persona="${1:-}"
    local arg1="${2:-}"
    
    # Interactive mode if no arguments
    if [[ -z "$persona" ]]; then
        interactive_menu
        return 0
    fi
    
    case "$persona" in
        1|wfm)
            wfm_generate "$arg1"
            ;;
        2|device)
            device_generate
            ;;
        interactive)
            interactive_menu
            ;;
        help|--help|-h)
            usage
            ;;
        *)
            error "Unknown persona: $persona"
            ;;
    esac
}

main "$@"
