#!/bin/bash

################################################################################
# Margo Conformance Test Runner (CLI #2)
#
# Purpose: Execute test cases prepared by conformance.sh (CLI #1)
#          - WFM Supplier: Runs Newman with Postman collections
#          - Device Supplier: Runs mock server tests with test scenarios
#          - Generates signed conformance reports
#
# Story: #278 - "As a Margo adopter, I would like to have a tool to allow me
#        to select the suitable persona and run a set of conformance test-cases
#        to get a signed report of conformance"
#
################################################################################

set -euo pipefail

################################################################################
# Configuration
################################################################################

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CONFORMANCE_DIR="$SCRIPT_DIR"  # run-tests.sh is already IN the conformance directory
DATA_GEN_DIR="$CONFORMANCE_DIR/Data-Generator"
RUNNER_DIR="$CONFORMANCE_DIR/Runner"  # Output directory for test results

# Create output directories
RUNNER_WFM="$RUNNER_DIR/wfm-supplier"
RUNNER_DEVICE="$RUNNER_DIR/device-supplier"
mkdir -p "$RUNNER_WFM" "$RUNNER_DEVICE"

################################################################################
# Logging Functions
################################################################################

log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] 📝 $*"
}

info() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] ℹ️  $*"
}

success() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] ✅ $*"
}

error() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] ❌ $*" >&2
    exit 1
}

################################################################################
# Component Configuration Functions
################################################################################

# NOTE: Component info collection commented out for now
# Device Supplier: Mock server handles all configuration
# WFM Supplier: Can be added later if needed for external component testing
#
# get_component_info() {
#     echo ""
#     info "Component Information"
#     info "===================="
#     
#     read -p "Component Name: " component_name
#     read -p "Manufacturer/Organization: " manufacturer
#     read -p "Component Version: " component_version
#     read -p "Margo Spec Version (e.g., v1.0.0): " spec_version
#     read -p "Component IP/Hostname: " component_ip
#     read -p "Component Port: " component_port
#     read -p "Certificate Path (optional, press Enter to skip): " cert_path
#     
#     log "Component Information Collected:"
#     info "  Component: $component_name"
#     info "  Manufacturer: $manufacturer"
#     info "  Version: $component_version"
#     info "  Spec Version: $spec_version"
#     info "  Endpoint: $component_ip:$component_port"
#     info "  Certificate: ${cert_path:-none}"
# }

################################################################################
# WFM Supplier Test Execution
################################################################################

execute_wfm_tests() {
    local wfm_url="${1:-}"
    
    # If WFM URL not provided, prompt user
    if [[ -z "$wfm_url" ]]; then
        echo ""
        read -p "Enter WFM Server Base URL [https://localhost:3001/v1alpha2/margo]: " wfm_url
        wfm_url="${wfm_url:-https://localhost:3001/v1alpha2/margo}"
    fi
    
    log "🚀 Starting WFM Supplier Test Execution"
    log "   WFM Server: $wfm_url"
    
    # Call 2-run_newman.sh with WFM URL
    local wfm_supplier_dir="$CONFORMANCE_DIR/wfm-supplier"
    
    if [[ ! -d "$wfm_supplier_dir" ]]; then
        error "WFM Supplier directory not found: $wfm_supplier_dir"
    fi
    
    cd "$wfm_supplier_dir"
    
    if bash 2-run_newman.sh "$wfm_url"; then
        success "WFM Tests Completed"
    else
        error "WFM test execution failed"
    fi
}

################################################################################
# Device Test Scenarios Selection
################################################################################

show_device_test_scenarios_menu() {
    echo "" >&2
    echo "Which test scenarios would you like to run?" >&2
    echo "1. Custom test scenarios (~/sandbox/conformance/device-supplier/device-scenarios/test-scenarios.json)" >&2
    echo "2. User generated test scenarios (~/sandbox/conformance/Data-Generator/device-supplier/test-scenarios.json)" >&2
    echo "" >&2
    echo "Q) Quit" >&2
    echo "" >&2
}

select_device_test_scenarios() {
    local custom_scenarios="$CONFORMANCE_DIR/device-supplier/device-scenarios/test-scenarios.json"
    local generated_scenarios="$DATA_GEN_DIR/device-supplier/test-scenarios.json"
    
    while true; do
        show_device_test_scenarios_menu
        # Print prompt to stderr so it doesn't get captured in command substitution
        echo -n "Select option (1-2 or Q): " >&2
        read choice
        
        case "${choice,,}" in
            1|custom)
                if [[ ! -f "$custom_scenarios" ]]; then
                    error "Custom test scenarios not found: $custom_scenarios"
                fi
                echo "$custom_scenarios"
                return 0
                ;;
            2|generated)
                if [[ ! -f "$generated_scenarios" ]]; then
                    error "Generated test scenarios not found: $generated_scenarios"
                fi
                echo "$generated_scenarios"
                return 0
                ;;
            q|quit)
                info "Exiting..."
                exit 0
                ;;
            *)
                error "Invalid option. Please select 1, 2, or Q"
                ;;
        esac
    done
}

################################################################################
# Device Supplier Test Execution
################################################################################

execute_device_tests() {
    local test_scenarios="${1:-}"
    
    if [[ -z "$test_scenarios" ]]; then
        error "Test scenarios file not provided"
    fi
    
    log "🚀 Starting Device Supplier Test Execution"
    log "   (Mock server + test runner orchestration)"
    
    # Check if test scenarios file exists
    if [[ ! -f "$test_scenarios" ]]; then
        error "Test scenarios not found: $test_scenarios"
    fi
    
    log "📋 Test Scenarios: $(basename "$test_scenarios")"
    
    # Check if run_tests.go exists
    local run_tests_go="$CONFORMANCE_DIR/device-supplier/run_tests.go"
    if [[ ! -f "$run_tests_go" ]]; then
        error "Device test runner not found: $run_tests_go"
    fi
    
    cd "$CONFORMANCE_DIR/device-supplier"
    
    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        error "Go not found. Install from https://golang.org/doc/install"
    fi
    
    log "📦 Building mock WFM server..."
    mkdir -p bin
    go build -o bin/server ./cmd/device-supplier || error "Failed to build mock server"

    log "📦 Building device test runner..."
    go build -o bin/run_tests run_tests.go || error "Failed to build test runner"
    
    # Copy test scenarios from Data-Generator or use custom scenarios
    log "📋 Staging test scenarios..."
    mkdir -p ./device-scenarios
    
    # Check if source and destination are the same (for custom scenarios)
    local resolved_source=$(cd "$(dirname "$test_scenarios")" && pwd -P)/$(basename "$test_scenarios")
    local resolved_dest=$(cd "$(dirname ./device-scenarios)" && pwd -P)/$(basename ./device-scenarios)/test-scenarios.json
    
    if [[ "$resolved_source" != "$resolved_dest" ]]; then
        cp "$test_scenarios" ./device-scenarios/test-scenarios.json
    fi
    
    # Clean up any stale server process on port 3001
    if [[ -f /tmp/wfm-server.pid ]]; then
        local old_pid=$(cat /tmp/wfm-server.pid)
        if kill -0 $old_pid 2>/dev/null; then
            log "⛔ Stopping previous server instance (PID: $old_pid)..."
            kill -15 $old_pid 2>/dev/null
            sleep 1
        fi
        rm -f /tmp/wfm-server.pid
    fi
    
    # Also check if anything is listening on port 3001 and kill it
    if command -v lsof &> /dev/null; then
        local pid_on_port=$(lsof -ti :3001 2>/dev/null)
        if [[ -n "$pid_on_port" ]]; then
            log "⛔ Stopping process on port 3001 (PID: $pid_on_port)..."
            kill -15 $pid_on_port 2>/dev/null
            sleep 1
        fi
    fi
    
    # Start mock server in background
    log "🚀 Starting Mock WFM Server (background)..."
    ./bin/server > /tmp/wfm-server.log 2>&1 &
    local server_pid=$!
    echo $server_pid > /tmp/wfm-server.pid
    sleep 2  # Wait for server to initialize
    
    # Verify server started
    if ! kill -0 $server_pid 2>/dev/null; then
        error "Failed to start mock server. Check /tmp/wfm-server.log"
    fi
    success "Mock WFM Server started (PID: $server_pid)"
    
    # Run tests against mock server
    log "▶️  Running Device Conformance Tests (as device agent)..."
    echo ""
    
    local test_result=0
    if ./bin/run_tests 2>&1 | tee "$RUNNER_DEVICE/test-execution.log"; then
        test_result=0
    else
        test_result=1
    fi
    
    # Stop mock server
    log "⛔ Stopping Mock WFM Server..."
    if [[ -f /tmp/wfm-server.pid ]]; then
        local pid=$(cat /tmp/wfm-server.pid)
        if kill -0 $pid 2>/dev/null; then
            kill -15 $pid
            sleep 1
            success "Mock server stopped (PID: $pid)"
        fi
        rm -f /tmp/wfm-server.pid
    fi
    
    # Check test result
    if [[ $test_result -ne 0 ]]; then
        error "Device test execution failed. Check $RUNNER_DEVICE/test-execution.log"
    fi
    
    # Find and copy generated report
    local latest_report=$(find reports -name "conformance-report-*.html" -type f -printf '%T@ %p\n' 2>/dev/null | sort -rn | head -1 | cut -d' ' -f2-)
    
    if [[ -n "$latest_report" && -f "$latest_report" ]]; then
        cp "$latest_report" "$RUNNER_DEVICE/"
        success "Device Tests Completed"
        success "Report: $RUNNER_DEVICE/$(basename "$latest_report")"
        success "Execution log: $RUNNER_DEVICE/test-execution.log"
    else
        success "Device Tests Completed"
        info "Reports location: $CONFORMANCE_DIR/device-supplier/reports/"
    fi
}

################################################################################
# Test Suite Selection
################################################################################

show_test_suite_menu() {
    echo ""
    echo "Which test suite would you like to run?"
    echo "1. Contract Tests (OpenAPI specifications)"
    echo "2. Functional Tests (Full conformance scenarios)"
    echo "3. Both"
    echo ""
    echo "B) Back"
    echo "Q) Quit"
    echo ""
}

select_test_suites() {
    local persona="$1"
    
    while true; do
        show_test_suite_menu
        read -p "Select test suite (1-3, B, or Q): " suite_choice
        
        case "${suite_choice,,}" in
            1|contract)
                echo "Contract"
                break
                ;;
            2|functional)
                echo "Functional"
                break
                ;;
            3|both)
                echo "Both"
                break
                ;;
            b|back)
                return 1
                ;;
            q|quit)
                info "Exiting..."
                exit 0
                ;;
            *)
                error "Invalid option. Please select 1, 2, 3, B, or Q"
                ;;
        esac
    done
}

################################################################################
# Persona Selection Menu
################################################################################

show_persona_menu() {
    echo ""
    echo "Which Margo Persona do you want to test?"
    echo "1. WFM Supplier"
    echo "2. Device Supplier"
    echo ""
    echo "H) Help"
    echo "Q) Quit"
    echo ""
}

################################################################################
# Help Function
################################################################################

show_help() {
    cat << 'EOF'

╔═══════════════════════════════════════════════════════════════════════════╗
║            Conformance Test Runner - Help                                 ║
╚═══════════════════════════════════════════════════════════════════════════╝

DESCRIPTION:
  This CLI executes conformance tests prepared by conformance.sh (CLI #1).
  - WFM Supplier: Uses Newman to run Postman collections
  - Device Supplier: Uses mock server to run test scenarios
  - Generates signed conformance reports

USAGE:
  bash run-tests.sh                              # Interactive menu
  bash run-tests.sh wfm                          # Run WFM tests
  bash run-tests.sh device                       # Run Device tests
  bash run-tests.sh help                         # Show this help

PERSONAS:

  WFM Supplier:
    • Tests Workload Fleet Manager compliance
    • Runs API contract tests from Postman collection
    • Uses Newman test executor
    • Generates HTML report with test results

  Device Supplier:
    • Tests device conformance with Margo API
    • Runs functional test scenarios
    • Uses mock WFM server for validation
    • Generates conformance report with assertion results

WORKFLOW:

  1. Run conformance.sh (CLI #1) to prepare test cases
     bash conformance.sh
     → Select persona and test type
     → Test cases prepared in Data-Generator/

  2. Run run-tests.sh (CLI #2) to execute tests
     bash run-tests.sh
     → Select persona
     → WFM Supplier: Provide component information (optional)
     → Device Supplier: Tests run automatically
     → Reports generated in Runner/

  3. Review conformance report
     • WFM report: Runner/wfm-supplier/
     • Device report: Runner/device-supplier/

REQUIREMENTS:

  WFM Supplier:
    • npm (for Newman)
    • Install: npm install -g newman
    • Test data: Data-Generator/wfm-supplier/postman_collection.json

  Device Supplier:
    • Go 1.13+ (for test runner)
    • Test data: Data-Generator/device-supplier/test-scenarios.json
    • Mock server: device-supplier/run_tests.go

EXAMPLE WORKFLOW:

  # Step 1: Generate tests with CLI #1
  cd conformance
  bash conformance.sh
  → Select: 1 (WFM Supplier)
  → Select: 1 (OpenAPI spec)
  → Enter: /path/to/openapi.yaml
  → Tests generated in Data-Generator/wfm-supplier/

  # Step 2: Run tests with CLI #2
  cd conformance
  bash run-tests.sh
  → Select: 1 (WFM Supplier)
  → Provide component details (optional)
  → Tests execute
  → Report in Runner/wfm-supplier/

  # Step 3: Review results
  open Runner/wfm-supplier/wfm-test-report-*.html

COMPONENT INFORMATION REQUESTED:

  • Component Name: Your component's name
  • Manufacturer: Organization/Company
  • Component Version: Version number
  • Margo Spec Version: Which Margo spec version it follows
  • Component IP/Hostname: Where component is running
  • Component Port: Port number
  • Certificate Path: (Optional) TLS certificate if needed

REPORT CONTENTS:

  • Test execution summary (passed/failed/skipped)
  • Detailed test results for each scenario
  • Assertion validation results
  • Telemetry (execution time, etc.)
  • Digital signature (for conformance claim)

TROUBLESHOOTING:

  Error: "Postman collection not found"
  → Run conformance.sh first to generate test cases

  Error: "Newman not found"
  → Install: npm install -g newman

  Error: "Go not found"
  → Install from https://golang.org/doc/install

  Error: "Test execution failed"
  → Check test-execution.log in Runner/ directory
  → Verify component is running and accessible

EOF
}

################################################################################
# WFM Certificate Info
################################################################################

show_wfm_cert_info() {
    cat << 'EOF'

╔═══════════════════════════════════════════════════════════════════════════╗
║            WFM Certificate Setup Required                                 ║
╚═══════════════════════════════════════════════════════════════════════════╝

BEFORE RUNNING WFM SUPPLIER TESTS:

  1. Copy WFM CA Certificate to Device Agent VM
  
     Copy FROM (WFM Server):
       ~/symphony/api/certificates/ca-cert.pem
     
     Copy TO (Device Agent VM - this machine):
       ~/sandbox/conformance/wfm-supplier/certs/ca-cert.pem
  
  2. Command to copy (run on WFM Server):
     scp ~/symphony/api/certificates/ca-cert.pem \\
         <device-agent-user>@<device-agent-ip>:~/sandbox/conformance/wfm-supplier/certs/

WHAT IT DOES:
  • The ca-cert.pem is used to verify WFM Server identity
  • Tests use this certificate to establish secure connections
  • Required for RFC 9421 HTTP Message Signature verification

EOF

    echo ""
    read -p "Press Enter once you have copied the certificate, or Ctrl+C to cancel: " continue_input
}

################################################################################
# Interactive Mode
################################################################################

interactive_mode() {
    while true; do
        show_persona_menu
        
        read -p "Select option (1-2, H, or Q): " choice
        
        case "${choice,,}" in
            1|wfm)
                echo ""
                info "You selected: WFM Supplier"
                
                # Show certificate setup instructions
                show_wfm_cert_info
                
                # Prompt for WFM URL
                echo ""
                read -p "Enter WFM Server Base URL [https://localhost:3001/v1alpha2/margo]: " wfm_url
                wfm_url="${wfm_url:-https://localhost:3001/v1alpha2/margo}"
                
                execute_wfm_tests "$wfm_url"
                ;;
            2|device)
                echo ""
                info "You selected: Device Supplier"
                
                # Select device test scenarios
                local device_test_scenarios=$(select_device_test_scenarios)
                
                execute_device_tests "$device_test_scenarios"
                ;;
            h|help)
                show_help
                ;;
            q|quit)
                info "Exiting..."
                exit 0
                ;;
            *)
                error "Invalid option. Please select 1, 2, H, or Q"
                ;;
        esac
        
        echo ""
        read -p "Press Enter to continue or Q to quit: " continue_choice
        if [[ "${continue_choice,,}" == "q" ]]; then
            info "Exiting..."
            exit 0
        fi
        clear
    done
}

################################################################################
# Main Entry Point
################################################################################

main() {
    cat << 'EOF'
╔═══════════════════════════════════════════════════════════════════════════╗
║           Margo Conformance Test Runner                                   ║
║              Execute conformance tests and generate reports                ║
╚═══════════════════════════════════════════════════════════════════════════╝

EOF

    # No arguments - show interactive menu
    if [[ $# -eq 0 ]]; then
        interactive_mode
        return 0
    fi

    # Command line argument parsing
    local command="${1,,}"
    
    case "$command" in
        wfm)
            log "Executing WFM Supplier tests..."
            execute_wfm_tests
            ;;
        device)
            log "Executing Device Supplier tests..."
            execute_device_tests
            ;;
        help|-h|--help)
            show_help
            ;;
        *)
            error "Unknown command: $command

Usage: bash run-tests.sh [wfm|device|help]

Run 'bash run-tests.sh help' for detailed instructions."
            ;;
    esac
}

# Run main function
main "$@"
