#!/usr/bin/env bash

################################################################################
# MARGO Test Case Runner - Runner CLI
################################################################################
# Purpose: Execute conformance test cases for MARGO personas
# Personas: WFM Supplier, Device Supplier
# 
# For WFM Supplier: Runs Newman against postman_collection.json
# For Device Supplier: Runs device supplier conformance tests
################################################################################

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFORMANCE_DIR="$(cd "$ROOT_DIR/.." && pwd)"
WFM_DIR="$CONFORMANCE_DIR/wfm-supplier"
DEVICE_DIR="$CONFORMANCE_DIR/device-supplier"
DATA_GEN_DIR="$CONFORMANCE_DIR/Data-Generator"
RUNNER_DIR="$ROOT_DIR"

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
# WFM Supplier - Execute tests via Newman
# ============================================
wfm_run() {
    log "🚀 Running WFM Supplier conformance tests via Newman..."
    
    # Ensure output directories exist
    mkdir -p "$RUNNER_DIR/wfm-supplier"
    mkdir -p "$WFM_DIR"
    
    # Check if tests were generated (in Data-Generator folder)
    if [[ -f "$DATA_GEN_DIR/wfm-supplier/postman_collection.json" ]]; then
        log "📋 Found generated tests in Data-Generator, preparing for execution..."
        # Copy from Data-Generator to wfm-supplier for Newman to use
        cp "$DATA_GEN_DIR/wfm-supplier/postman_collection.json" "$WFM_DIR/" || error "Failed to copy postman_collection.json"
        
        # Copy newman data if it exists
        if [[ -d "$DATA_GEN_DIR/wfm-supplier/newman-data" ]]; then
            cp -r "$DATA_GEN_DIR/wfm-supplier/newman-data" "$WFM_DIR/" 2>/dev/null || true
        fi
        success "✅ Tests copied from Data-Generator"
    elif [[ ! -f "$WFM_DIR/postman_collection.json" ]]; then
        error "❌ postman_collection.json not found!

To fix this, first generate the tests:
  cd conformance/Data-Generator
  ./margo-test-gen.sh wfm https://symphony.machine:8082/v1alpha2/margo

Then run tests:
  cd ../Runner
  ./margo-test-run.sh wfm"
    fi
    
    # Run Newman
    (cd "$WFM_DIR" && bash 2-run_newman.sh) || error "Newman test execution failed"
    
    # Copy reports to Runner directory
    if ls "$WFM_DIR"/report_*.html 1>/dev/null 2>&1; then
        cp "$WFM_DIR"/report_*.html "$RUNNER_DIR/wfm-supplier/" 2>/dev/null || true
        log "📊 Reports saved to: $RUNNER_DIR/wfm-supplier/"
    fi
    
    success "WFM Supplier tests completed"
    
    # Display device status if available
    if [[ -f "$WFM_DIR/newman-data/device-agent.env.json" ]]; then
        log "📋 Device Status from API tests:"
        cat "$WFM_DIR/newman-data/device-agent.env.json" | grep -E "clientId|vendor|model|cpu|memory|deployment" || true
    fi
}

# ============================================
# Device Supplier - Execute tests
# ============================================
device_run() {
    log "🚀 Running Device Supplier conformance tests..."
    
    # Ensure output directory exists
    mkdir -p "$RUNNER_DIR/device-supplier"
    
    # Build and run tests
    (cd "$DEVICE_DIR" && make run-tests) || error "Device supplier tests failed"
    
    # Copy reports to Runner directory
    if [[ -d "$DEVICE_DIR/reports" ]]; then
        if ls "$DEVICE_DIR/reports"/*.html 1>/dev/null 2>&1; then
            cp "$DEVICE_DIR/reports"/*.html "$RUNNER_DIR/device-supplier/" 2>/dev/null || true
            log "📊 Reports saved to: $RUNNER_DIR/device-supplier/"
        fi
    fi
    
    success "Device Supplier tests completed"
}

# ============================================
# Device Supplier - Execute workloads (3-execute-workloads)
# ============================================
device_execute_workloads() {
    log "🚀 Executing Device Supplier workloads (deployment phase)..."
    
    # Ensure output directory exists
    mkdir -p "$RUNNER_DIR/device-supplier"
    
    # Execute workloads phase
    (cd "$WFM_DIR" && bash 3-execute-workloads.sh) || error "Workload execution failed"
    
    success "Device Supplier workload execution completed"
}

# ============================================
# Device Supplier - Full demo (build, start, run tests, cleanup)
# ============================================
device_demo() {
    log "🎬 Running Device Supplier full demo..."
    
    # Ensure output directory exists
    mkdir -p "$RUNNER_DIR/device-supplier"
    
    (cd "$DEVICE_DIR" && make demo) || error "Device supplier demo failed"
    
    success "Device Supplier demo completed"
}

# ============================================
# Full end-to-end workflow
# ============================================
full_workflow() {
    log "🔄 Starting full conformance workflow (WFM + Device)..."
    
    log "Step 1: Running WFM tests..."
    wfm_run
    
    log ""
    log "Step 2: Running Device tests..."
    device_run
    
    log ""
    log "Step 3: Executing workloads..."
    device_execute_workloads
    
    success "🎉 Full workflow completed!"
}

# ============================================
# Show usage
# ============================================
usage() {
    cat <<'EOF'
MARGO Test Case Runner (Runner CLI)

Usage:
  ./margo-test-run.sh [PERSONA] [COMMAND]

Personas:
  1, wfm              Run WFM Supplier conformance tests
  2, device           Run Device Supplier conformance tests
  all                 Run WFM + Device + Workload execution (full workflow)
  interactive         Interactive menu (default)

Device Commands (optional):
  tests               Run device tests only
  demo                Full demo (build, start server, run tests)
  workloads           Execute workload deployment phase
  cleanup             Stop device server and clean

Examples:
  ./margo-test-run.sh wfm
  ./margo-test-run.sh device tests
  ./margo-test-run.sh device demo
  ./margo-test-run.sh all
  ./margo-test-run.sh interactive
  ./margo-test-run.sh                (runs interactive mode)

Output Locations:
  WFM:    ./Runner/wfm-supplier/report_*.html
  Device: ./Runner/device-supplier/report_*.html

EOF
}

# ============================================
# Interactive Menu
# ============================================
interactive_menu() {
    while true; do
        clear
        echo "======================================================================"
        echo "  MARGO Test Case Runner (Runner CLI)"
        echo "======================================================================"
        echo ""
        echo "Select Persona:"
        echo "  1. WFM Supplier       - Run API contract tests"
        echo "  2. Device Supplier    - Run conformance tests"
        echo "  3. Device Demo        - Full demo workflow"
        echo "  4. Full Workflow      - WFM + Device + Workloads"
        echo "  5. Exit"
        echo ""
        read -p "Enter choice (1-5): " choice
        
        case "$choice" in
            1|wfm)
                wfm_run
                read -p "Press Enter to continue..." _
                ;;
            2|device)
                device_run
                read -p "Press Enter to continue..." _
                ;;
            3|demo)
                device_demo
                read -p "Press Enter to continue..." _
                ;;
            4|workflow|all)
                full_workflow
                read -p "Press Enter to continue..." _
                ;;
            5|exit)
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
    local cmd="${2:-}"
    
    # Interactive mode if no arguments
    if [[ -z "$persona" ]]; then
        interactive_menu
        return 0
    fi
    
    case "$persona" in
        1|wfm)
            wfm_run
            ;;
        2|device)
            case "$cmd" in
                tests)
                    device_run
                    ;;
                demo)
                    device_demo
                    ;;
                workloads)
                    device_execute_workloads
                    ;;
                cleanup)
                    (cd "$DEVICE_DIR" && make kill-server && make clean)
                    ;;
                *)
                    device_run  # default
                    ;;
            esac
            ;;
        3|demo)
            device_demo
            ;;
        all|workflow)
            full_workflow
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
