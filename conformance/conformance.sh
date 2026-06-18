#!/bin/bash

################################################################################
# Conformance Test Case Generator CLI
# Purpose: Create and prepare test cases for Margo Personas
# Usage: ./conformance.sh
################################################################################

set -euo pipefail

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFORMANCE_DIR="$SCRIPT_DIR"
DATA_GEN_DIR="$CONFORMANCE_DIR/Data-Generator"

################################################################################
# Logging Functions
################################################################################

log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] 📝 $*"
}

success() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] ✅ $*"
}

error() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] ❌ $*" >&2
    exit 1
}

info() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] ℹ️  $*"
}

warn() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] ⚠️  $*"
}

################################################################################
# WFM Supplier Test Case Generation - with OpenAPI Spec Path
################################################################################

generate_wfm_tests() {
    local openapi_spec_path="${1:-}"
    
    if [[ -z "$openapi_spec_path" ]]; then
        info "Provide the OpenAPI Specification path"
        info "Can be: URL (https://...) or local file path"
        info "Example: https://raw.githubusercontent.com/margo/specification/.../openapi.yaml"
        info "Or: /path/to/local/openapi.yaml"
        read -p "OpenAPI Spec Path: " openapi_spec_path
    fi
    
    if [[ -z "$openapi_spec_path" ]]; then
        error "OpenAPI Spec path cannot be empty"
    fi
    
    log "🚀 Generating WFM Supplier test cases..."
    log "OpenAPI Spec: $openapi_spec_path"
    
    # Create output directory
    mkdir -p "$DATA_GEN_DIR/wfm-supplier"
    
    # Use Portman to generate Postman collection from OpenAPI
    cd "$CONFORMANCE_DIR/wfm-supplier"
    
    if ! command -v portman &> /dev/null; then
        warn "Portman not installed. Installing from npm..."
        npm install -g portman
    fi
    
    log "🔧 Running Portman to generate test cases..."
    
    # Determine if spec is a URL or local file
    local spec_file=""
    
    if [[ "$openapi_spec_path" == https://* || "$openapi_spec_path" == http://* ]]; then
        # Download from URL
        log "📥 Downloading OpenAPI specification from URL..."
        spec_file="/tmp/openapi_spec_$$.yaml"
        if ! curl -sSL "$openapi_spec_path" -o "$spec_file" 2>/dev/null; then
            error "Could not download OpenAPI spec from URL: $openapi_spec_path"
        fi
        log "✅ Downloaded OpenAPI spec"
    else
        # Local file path
        if [[ ! -f "$openapi_spec_path" ]]; then
            error "OpenAPI Spec file not found: $openapi_spec_path"
        fi
        spec_file="$openapi_spec_path"
        log "✅ Using local OpenAPI spec: $spec_file"
    fi
    
    # Validate spec file
    if [[ ! -f "$spec_file" ]]; then
        error "OpenAPI Spec file does not exist: $spec_file"
    fi
    
    # Validate spec file is not empty
    if [[ ! -s "$spec_file" ]]; then
        error "OpenAPI Spec file is empty: $spec_file"
    fi
    
    # Run Portman
    log "📋 Generating Postman collection..."
    if ! portman -l "$spec_file" -o postman_collection.json 2>/dev/null; then
        error "Portman generation failed"
    fi
    
    log "✅ Test cases generated successfully"
    log "📋 Copying postman_collection.json to Data-Generator..."
    
    # Copy to Data-Generator
    cp postman_collection.json "$DATA_GEN_DIR/wfm-supplier/"
    
    # Copy newman data if exists
    if [[ -d "newman-data" ]]; then
        log "📦 Copying newman data files..."
        cp -r newman-data "$DATA_GEN_DIR/wfm-supplier/" 2>/dev/null || true
    fi
    
    # Clean up temp file if it was downloaded
    if [[ "$openapi_spec_path" == https://* || "$openapi_spec_path" == http://* ]]; then
        rm -f "$spec_file"
    fi
    
    success "WFM test cases generated and prepared"
    success "Output: $DATA_GEN_DIR/wfm-supplier/"
    
    # Show summary
    info "Generated files:"
    ls -lh "$DATA_GEN_DIR/wfm-supplier/" | tail -n +2 | awk '{print "  - " $9 " (" $5 ")"}'
}

################################################################################
# WFM Supplier Functional Tests - Manual Test Cases (MARGO Template)
################################################################################

generate_wfm_functional_tests() {
    local postman_collection_path="${1:-}"
    
    if [[ -z "$postman_collection_path" ]]; then
        info "Provide the path to Postman collection JSON file"
        info "Path to manual/hand-crafted Postman collection"
        info "Example: /path/to/postman_collection.json"
        info "Or place files in: $CONFORMANCE_DIR/manual-test-cases/"
        read -p "Postman Collection JSON Path: " postman_collection_path
    fi
    
    if [[ -z "$postman_collection_path" ]]; then
        error "Postman collection path cannot be empty"
    fi
    
    # Expand tilde if used
    postman_collection_path="${postman_collection_path/\~/$HOME}"
    
    log "🚀 Setting up WFM Supplier Functional Tests (MARGO Template)..."
    log "Collection: $postman_collection_path"
    
    # Create output directory
    mkdir -p "$DATA_GEN_DIR/wfm-supplier"
    
    # Validate file exists
    if [[ ! -f "$postman_collection_path" ]]; then
        error "Postman collection file not found: $postman_collection_path"
    fi
    
    # Validate file is not empty
    if [[ ! -s "$postman_collection_path" ]]; then
        error "Postman collection file is empty: $postman_collection_path"
    fi
    
    # Validate JSON format
    log "📋 Validating JSON format..."
    if ! jq empty "$postman_collection_path" 2>/dev/null; then
        error "Invalid JSON format in: $postman_collection_path"
    fi
    
    # Validate Postman collection structure (basic validation)
    log "🔍 Validating Postman collection format..."
    local has_info
    has_info=$(jq '.info' "$postman_collection_path" 2>/dev/null)
    
    if [[ -z "$has_info" ]] || [[ "$has_info" == "null" ]]; then
        error "Invalid Postman collection: missing 'info' field"
    fi
    
    local has_items
    has_items=$(jq '.item' "$postman_collection_path" 2>/dev/null)
    
    if [[ -z "$has_items" ]] || [[ "$has_items" == "null" ]]; then
        error "Invalid Postman collection: missing 'item' field"
    fi
    
    log "✅ Collection validation passed"
    
    # Get collection info
    local collection_name
    local item_count
    collection_name=$(jq -r '.info.name' "$postman_collection_path")
    item_count=$(jq '.item | length' "$postman_collection_path")
    
    log "📊 Collection Details:"
    log "   Name: $collection_name"
    log "   Test items: $item_count"
    
    # Copy collection to Data-Generator
    log "📋 Copying Postman collection to Data-Generator..."
    
    cp "$postman_collection_path" "$DATA_GEN_DIR/wfm-supplier/postman_collection_functional.json"
    
    # Also create a marker file indicating this is functional tests
    touch "$DATA_GEN_DIR/wfm-supplier/.functional-tests"
    
    success "WFM Functional Tests prepared successfully"
    success "Output: $DATA_GEN_DIR/wfm-supplier/postman_collection_functional.json"
    
    info "Ready for test execution with Newman"
}

################################################################################
# Device Test Type Selection Menu
################################################################################

show_test_type_menu() {
    echo ""
    echo "What type of test-cases do you want to add?"
    echo "1. OpenAPI spec based contract tests"
    echo "2. Functional tests (Group-based test management)"
    echo ""
    echo "B) Back"
    echo "Q) Quit"
    echo ""
}

################################################################################
# Device Supplier Test Case Validation
################################################################################

validate_device_test_scenarios() {
    local scenarios_path="${1:-}"
    local assertions_path="${2:-}"
    
    # Validate test scenarios file
    if [[ ! -f "$scenarios_path" ]]; then
        error "Test scenarios file not found: $scenarios_path"
    fi
    
    if [[ ! -s "$scenarios_path" ]]; then
        error "Test scenarios file is empty: $scenarios_path"
    fi
    
    # Validate JSON format
    log "📋 Validating JSON format for test scenarios..."
    if ! jq empty "$scenarios_path" 2>/dev/null; then
        error "Invalid JSON format in test scenarios: $scenarios_path"
    fi
    
    # Validate test scenarios structure
    log "🔍 Validating test scenarios structure..."
    local scenario_count
    scenario_count=$(jq 'length' "$scenarios_path" 2>/dev/null)
    
    if [[ -z "$scenario_count" ]] || [[ "$scenario_count" == "0" ]]; then
        error "Test scenarios file must contain at least one scenario"
    fi
    
    # Check for required fields in each scenario
    local has_required_fields
    has_required_fields=$(jq '[.[] | has("id") and has("name") and has("steps")] | all' "$scenarios_path" 2>/dev/null)
    
    if [[ "$has_required_fields" != "true" ]]; then
        error "Invalid test scenarios: each scenario must have 'id', 'name', and 'steps' fields"
    fi
    
    # Check if assertions file exists
    if [[ -f "$assertions_path" ]]; then
        log "📋 Validating JSON format for assertions..."
        if ! jq empty "$assertions_path" 2>/dev/null; then
            error "Invalid JSON format in assertions file: $assertions_path"
        fi
        
        log "🔍 Validating assertions structure..."
        local has_endpoints
        has_endpoints=$(jq 'has("endpoints")' "$assertions_path" 2>/dev/null)
        
        if [[ "$has_endpoints" != "true" ]]; then
            warn "Assertions file does not have 'endpoints' section (non-critical)"
        fi
    else
        warn "Assertions file not found at: $assertions_path (optional)"
    fi
    
    log "✅ Validations passed"
    
    # Show scenarios summary
    info "Test Scenarios Summary:"
    jq -r '.[] | "  - \(.id): \(.name)"' "$scenarios_path" | head -10
    
    local total=$(jq 'length' "$scenarios_path")
    if [[ $total -gt 10 ]]; then
        info "  ... and $((total - 10)) more scenarios"
    fi
}

################################################################################
# Device Supplier Test Case Setup
################################################################################

generate_device_tests() {
    local test_scenarios_path="${1:-}"
    
    if [[ -z "$test_scenarios_path" ]]; then
        info "Provide the path to Device test scenarios JSON file"
        info "Expected location: device-supplier/device-scenarios/test-scenarios.json"
        info "Or use: $CONFORMANCE_DIR/device-supplier/device-scenarios/test-scenarios.json"
        read -p "Test Scenarios JSON Path: " test_scenarios_path
    fi
    
    if [[ -z "$test_scenarios_path" ]]; then
        error "Test scenarios path cannot be empty"
    fi
    
    # Expand tilde if used
    test_scenarios_path="${test_scenarios_path/\~/$HOME}"
    
    # Check for assertions file
    local assertions_path="$CONFORMANCE_DIR/device-supplier/manifests/assertions.json"
    
    # If test scenarios path is provided, try to find assertions nearby
    if [[ "$test_scenarios_path" == *"device-scenarios"* ]]; then
        assertions_path="${test_scenarios_path%/*}/../manifests/assertions.json"
    fi
    
    log "🚀 Setting up Device Supplier Test Cases..."
    log "Test scenarios: $test_scenarios_path"
    log "Assertions file: $assertions_path"
    
    # Create output directory
    mkdir -p "$DATA_GEN_DIR/device-supplier"
    
    # Validate test scenarios and assertions
    validate_device_test_scenarios "$test_scenarios_path" "$assertions_path"
    
    # Copy test scenarios to Data-Generator
    log "📋 Copying test scenarios to Data-Generator..."
    cp "$test_scenarios_path" "$DATA_GEN_DIR/device-supplier/test-scenarios.json"
    
    # Copy assertions file if it exists
    if [[ -f "$assertions_path" ]]; then
        log "📋 Copying assertions file..."
        cp "$assertions_path" "$DATA_GEN_DIR/device-supplier/assertions.json"
    fi
    
    # Copy supporting files from device-supplier if they exist
    if [[ -d "$CONFORMANCE_DIR/device-supplier/manifests" ]]; then
        log "📦 Copying supporting manifest files..."
        cp -r "$CONFORMANCE_DIR/device-supplier/manifests"/* "$DATA_GEN_DIR/device-supplier/" 2>/dev/null || true
    fi
    
    # Create marker file
    touch "$DATA_GEN_DIR/device-supplier/.device-scenarios"
    
    success "Device Supplier Test Cases prepared successfully"
    success "Output: $DATA_GEN_DIR/device-supplier/"
    
    info "Generated files:"
    ls -lh "$DATA_GEN_DIR/device-supplier/" | grep -E "\.(json|yaml)$" | awk '{print "  - " $9 " (" $5 ")"}'
}

################################################################################
# Device Supplier Test Case Generation - OpenAPI Contract Tests
################################################################################

generate_device_openapi_tests() {
    log "🚀 Generating Device Supplier - OpenAPI Contract Tests..."
    
    local api_url="${1:-}"
    
    if [[ -z "$api_url" ]]; then
        info "Enter the Margo API URL for Device Supplier"
        info "Example: https://symphony.machine:8082/v1alpha2/margo"
        read -p "API URL: " api_url
    fi
    
    if [[ -z "$api_url" ]]; then
        error "API URL cannot be empty"
    fi
    
    log "📥 Downloading OpenAPI specification..."
    
    # Create output directory
    mkdir -p "$DATA_GEN_DIR/device-supplier"
    
    # Use Portman to generate Postman collection from OpenAPI
    cd "$CONFORMANCE_DIR/device-supplier" 2>/dev/null || \
        error "device-supplier directory not found"
    
    if ! command -v portman &> /dev/null; then
        warn "Portman not installed. Installing from npm..."
        npm install -g portman
    fi
    
    log "🔧 Running Portman to generate OpenAPI contract tests..."
    
    # Create a temporary OpenAPI spec file
    local spec_file="/tmp/device_openapi_spec_$$.yaml"
    if curl -sSL "$api_url/openapi" -o "$spec_file" 2>/dev/null; then
        log "✅ Downloaded OpenAPI spec"
    else
        error "Could not download OpenAPI spec from $api_url"
    fi
    
    # Generate Postman collection for device endpoints
    portman --spec "$spec_file" --output device_postman_collection.json 2>/dev/null || \
        error "Portman generation failed"
    
    log "✅ Contract tests generated"
    log "📋 Copying device_postman_collection.json to Data-Generator..."
    
    # Copy to Data-Generator
    cp device_postman_collection.json "$DATA_GEN_DIR/device-supplier/"
    
    # Copy newman data if exists
    if [[ -d "newman-data" ]]; then
        log "📦 Copying newman data files..."
        cp -r newman-data "$DATA_GEN_DIR/device-supplier/" 2>/dev/null || true
    fi
    
    # Clean up temp file
    rm -f "$spec_file"
    
    success "Device OpenAPI contract tests generated"
    success "Output: $DATA_GEN_DIR/device-supplier/"
    success "Test file: device_postman_collection.json"
    
    # Show summary
    info "Generated files:"
    ls -lh "$DATA_GEN_DIR/device-supplier/" | tail -n +2 | awk '{print "  - " $9 " (" $5 ")"}'
}

################################################################################
# Device Supplier Test Case Generation - MARGO Template Functional Tests
################################################################################

generate_device_margo_template_tests() {
    log "🚀 Generating Device Supplier - MARGO Template Functional Tests..."
    
    # Create output directory
    mkdir -p "$DATA_GEN_DIR/device-supplier"
    
    log "📥 Preparing test scenarios from MARGO template..."
    
    # Copy test scenarios from device-supplier
    local source_file="$CONFORMANCE_DIR/device-supplier/device-scenarios/test-scenarios.json"
    
    if [[ ! -f "$source_file" ]]; then
        error "Test scenarios not found at $source_file"
    fi
    
    log "📋 Copying test-scenarios.json (MARGO template)..."
    cp "$source_file" "$DATA_GEN_DIR/device-supplier/"
    
    # Copy additional test data files if they exist
    if [[ -d "$CONFORMANCE_DIR/device-supplier/device-scenarios" ]]; then
        log "📦 Copying additional test data files..."
        cp -r "$CONFORMANCE_DIR/device-supplier/device-scenarios"/* \
            "$DATA_GEN_DIR/device-supplier/" 2>/dev/null || true
    fi
    
    success "Device MARGO template functional tests generated"
    success "Output: $DATA_GEN_DIR/device-supplier/"
    success "Test file: test-scenarios.json"
    
    # Show summary
    info "Generated files:"
    ls -lh "$DATA_GEN_DIR/device-supplier/" | tail -n +2 | awk '{print "  - " $9 " (" $5 ")"}'
    
    # Count scenarios
    if [[ -f "$DATA_GEN_DIR/device-supplier/test-scenarios.json" ]]; then
        local test_count=$(jq '.[] | .id' "$DATA_GEN_DIR/device-supplier/test-scenarios.json" 2>/dev/null | wc -l)
        info "Total test scenarios: $test_count"
    fi
    
    info "Test Type: MARGO Template with RFC 9421 Signature Validation"
}

################################################################################
# Persona Selection Menu
################################################################################

show_persona_menu() {
    echo ""
    echo "Which Margo Persona do you want to manage?"
    echo "1. WFM Supplier "
    echo "2. Device Supplier (Group-based test management)"
    echo ""
    echo "H) Help"
    echo "Q) Quit"
    echo ""
}

show_help() {
    cat << 'EOF'

╔═══════════════════════════════════════════════════════════════════════════╗
║            Conformance Test Case Generator - Help                          ║
╚═══════════════════════════════════════════════════════════════════════════╝

DESCRIPTION:
  This CLI generates test cases for Margo conformance testing.
  It prepares test data for both WFM Supplier and Device Supplier personas.
  
  When you select WFM Supplier, you can prepare either OpenAPI-based tests or a functional Postman collection.
  
  When you select Device Supplier, you manage group-based device scenarios.

USAGE:
  ./conformance.sh                         # Interactive menu
  ./conformance.sh wfm openapi [SPEC_PATH] # Generate WFM OpenAPI tests
  ./conformance.sh wfm functional [PATH]   # Prepare WFM Postman collection
  ./conformance.sh device                  # Create/select Device groups
  ./conformance.sh device [SCENARIOS]      # Prepare Device scenarios

PERSONA OPTIONS:
  
  WFM Supplier:
    • Generates Postman collection from OpenAPI spec
    • Tests API contracts via Newman
    • Option 1: OpenAPI spec based (asks for spec path: URL or local file)
    • Option 2: MARGO template (not yet implemented)
  
  Device Supplier:
    • Interactive mode opens group management
    • Command mode can prepare a scenario file or template data
    • Template file: device-supplier/device-scenarios/TEMPLATE_custom_test_scenario.json
    • See device-supplier/Final-Summary.md for complete schema documentation
    
INTERACTIVE MENU:
  1       - Select WFM Supplier
  2       - Select Device Supplier group management
  H/help  - Show this help message
  Q/quit  - Exit the program

EXAMPLES:

  1. Interactive mode (menu-driven):
     ./conformance.sh

  2. Generate WFM tests with OpenAPI spec URL:
     ./conformance.sh wfm https://raw.githubusercontent.com/margo/specification/.../openapi.yaml

  3. Generate WFM tests with local OpenAPI spec file:
     ./conformance.sh wfm /path/to/local/openapi.yaml

  4. Generate Device tests with existing test-scenarios.json:
     ./conformance.sh device "/home/margo/nitin/sandbox/conformance/device-supplier/device-scenarios/test-scenarios.json"

  5. Generate Device tests with custom test scenarios:
     ./conformance.sh device "/path/to/your/custom_test_scenarios.json"

  6. View custom test scenario template:
     cat device-supplier/device-scenarios/TEMPLATE_custom_test_scenario.json

WHAT GETS GENERATED:

  WFM Supplier:
    • postman_collection.json (92KB) - 8 API test cases
    • newman-data/ - Test environment and certificates
    • Location: Data-Generator/wfm-supplier/

  Device Supplier (Option 1 - Existing):
    • test-scenarios.json (34KB) - 7 pre-built conformance test scenarios
    • assertions.json - Validation rules and assertions
    • deployment-template.yaml - Supporting manifest files
    • Location: Data-Generator/device-supplier/

  Device Supplier (Option 2 - Custom):
    • test-scenarios.json - Your custom test scenarios file
    • assertions.json - Your custom assertions (if provided)
    • Location: Data-Generator/device-supplier/
    • Template: device-supplier/device-scenarios/TEMPLATE_custom_test_scenario.json

NEXT STEPS:

  After generating tests, use the execution CLI to run them:
    ./run-tests.sh
    ./run-tests.sh wfm <group-name> <wfm-url>
    ./run-tests.sh device <group-name>

REQUIREMENTS:

  WFM:
    • npm (for Portman)
    • curl (to download OpenAPI spec from URL)
    • OpenAPI specification file (URL or local path)

  Device:
    • npm (for Portman) - if using OpenAPI contract tests
    • jq (to parse test scenarios) - if using MARGO template
    • Margo API endpoint or device-supplier/device-scenarios/test-scenarios.json

EOF
}

################################################################################
# Group Management Functions
################################################################################

set_supplier_context() {
    SUPPLIER="$1"
    GROUP_DIR="$DATA_GEN_DIR/$SUPPLIER/groups"
}

create_test_group() {
    mkdir -p "$GROUP_DIR"

    # Group name
    if [[ -z "${GROUP_NAME:-}" ]]; then
        echo ""
        read -p "Enter group name: " GROUP_NAME
        [[ -z "$GROUP_NAME" ]] && error "Group name cannot be empty"
    fi

    GROUP_PATH="$GROUP_DIR/$GROUP_NAME"

    # Version
    echo ""
    read -p "Enter version of Margo Specification: " VERSION
    [[ -z "$VERSION" ]] && VERSION="1.0.0"

    # Description
    echo ""
    read -p "Enter a short description for group: " DESCRIPTION
    [[ -z "$DESCRIPTION" ]] && DESCRIPTION="User created group"

    # Append mode
    APPEND_MODE=false
    if [[ -d "$GROUP_PATH" ]]; then
        info "Using existing group: $GROUP_NAME"
        APPEND_MODE=true
    else
        mkdir -p "$GROUP_PATH"
    fi

    # Input folder
    echo ""
    read -p "Enter folder path containing JSON files: " INPUT_PATH
    [[ ! -d "$INPUT_PATH" ]] && error "Provided path is not a folder!"

    log " Reading all JSON files from folder..."

    ALL_TESTS=()

    # SAFE LOOP (IMPORTANT FIX)
    shopt -s nullglob
    files=("$INPUT_PATH"/*.json)

    # check empty
    if [[ ${#files[@]} -eq 0 ]]; then
        error "No JSON files found in folder!"
    fi

    for file in "${files[@]}"; do
        filename=$(basename "$file")
        log " Processing: $filename"

        # copy file
        cp "$file" "$GROUP_PATH/"

        # SAFE jq (NO CRASH)
        IDS=$(jq -r '.. | .id? // empty' "$file" 2>/dev/null || true)

        if [[ -z "$IDS" ]]; then
            IDS=$(jq -r '.. | .name? // empty' "$file" 2>/dev/null \
                | sed 's/ /_/g' \
                | tr '[:upper:]' '[:lower:]')
        fi

        # append safely
        while IFS= read -r id; do
            [[ -n "$id" ]] && ALL_TESTS+=("$id")
        done <<< "$IDS"
    done

    # final validation
    if [[ ${#ALL_TESTS[@]} -eq 0 ]]; then
        error "No test cases found in files"
    fi

    log " Total extracted test cases: ${#ALL_TESTS[@]}"

    TESTS_JSON=$(printf '%s\n' "${ALL_TESTS[@]}" | jq -R . | jq -s 'unique')

    PERSONA="$SUPPLIER"

    # merge / create
    if [[ "$APPEND_MODE" == true && -f "$GROUP_PATH/group.json" ]]; then
        OLD_TESTS=$(jq '.testCases' "$GROUP_PATH/group.json")

        jq -n \
            --arg name "$GROUP_NAME" \
            --arg version "$VERSION" \
            --arg persona "$PERSONA" \
            --arg desc "$DESCRIPTION" \
            --argjson old "$OLD_TESTS" \
            --argjson new "$TESTS_JSON" \
            '{
                name: $name,
                version: $version,
                persona: $persona,
                description: $desc,
                testCases: ($old + $new | unique)
            }' > "$GROUP_PATH/group.json"

        success " Reading JSON files, extracting test case IDs, and adding them to the group.json"

    else
        jq -n \
            --arg name "$GROUP_NAME" \
            --arg version "$VERSION" \
            --arg persona "$PERSONA" \
            --arg desc "$DESCRIPTION" \
            --argjson tests "$TESTS_JSON" \
            '{
                name: $name,
                version: $version,
                persona: $persona,
                description: $desc,
                testCases: $tests
            }' > "$GROUP_PATH/group.json"

        success " Created new group.json"
    fi

    info "Target Group Folder: $GROUP_PATH"
}

group_management_menu() {
    # safety check
    if [[ -z "${GROUP_DIR:-}" ]]; then
        error "GROUP_DIR not set. Please select supplier first."
    fi

    while true; do
        echo ""
        echo "Enter a number to select an existing group or press 0 to create a new group"
        echo ""

        echo "Available Groups"
        echo "--------------------------------------"

        mkdir -p "$GROUP_DIR"

        mapfile -t groups < <(
            find "$GROUP_DIR" -mindepth 1 -maxdepth 1 -type d -exec basename {} \;
        )

        if [ ${#groups[@]} -eq 0 ]; then
            echo "  No groups available"
        else
            for i in "${!groups[@]}"; do
                echo "  $((i+1))) ${groups[i]}"
            done
        fi

        echo ""
        echo "B → Back"
        echo "Q → Quit"
        echo ""

        read -p "Enter your choice: " choice

        # BACK
        [[ "${choice,,}" == "b" ]] && return

        # QUIT
        [[ "${choice,,}" == "q" ]] && { info "Exiting..."; exit 0; }

        # CREATE
        if [[ "$choice" == "0" ]]; then
            unset GROUP_NAME
            create_test_group

        # EXISTING
        elif [[ "$choice" =~ ^[0-9]+$ ]]; then
            index=$((choice-1))

            if [[ -n "${groups[index]}" ]]; then
                GROUP_NAME="${groups[index]}"
                info "Selected existing group: $GROUP_NAME"
                create_test_group
            else
                warn "Invalid group number"
                continue
            fi

        else
            warn "Invalid choice"
            continue
        fi

        echo ""
        echo "Options:"
        echo "  B → Back"
        echo "  Q → Quit"
        echo ""

        read -p "Select option: " post_choice

        case "${post_choice,,}" in
            b) continue ;;
            q) info "Exiting..."; exit 0 ;;
            *) warn "Invalid option" ;;
        esac
    done
}

list_test_groups() {
    mkdir -p "$GROUP_DIR"

    if ls -d "$GROUP_DIR"/*/ > /dev/null 2>&1; then
        for dir in "$GROUP_DIR"/*/; do
            echo " - $(basename "$dir")"
        done
    else
        echo "No groups found"
    fi
}

################################################################################
# Interactive Menu Loop
################################################################################

interactive_mode() {
    while true; do
        show_persona_menu
        
        read -p "Select option (1-2, H, or Q): " choice
        
        case "${choice,,}" in
        1|wfm)
    echo ""
    info "You selected: WFM Supplier"

    while true; do
        show_test_type_menu
        read -p "Select test type (1-2, B, or Q): " test_choice

        case "${test_choice,,}" in

            1|openapi|contract)
                echo ""
                read -p "Enter OpenAPI Spec Path: " spec_path
                generate_wfm_tests "$spec_path"
                ;;

            2|margo|functional|template)
                echo ""
                info "Functional Test Mode Selected"

                set_supplier_context "wfm-supplier"
                group_management_menu
                ;;

            b|back)
                break
                ;;

            q|quit)
                info "Exiting..."
                exit 0
                ;;

            *)
                warn "Invalid option"
                ;;
        esac
    done
    ;;
            2|device)
                echo ""
                info "You selected: Device Supplier"
                echo ""

                TEMPLATE_PATH="$CONFORMANCE_DIR/device-supplier/device-scenarios/../docs/Template.md"
                TEMPLATE_PATH="$(realpath "$TEMPLATE_PATH" 2>/dev/null || echo "$TEMPLATE_PATH")"

                echo "⚠️  WARNING:"
                echo "================================================"
                echo "Please ensure TCs are built on the Margo template before proceeding with Device Supplier tests."
                echo ""
                echo "Template reference: $TEMPLATE_PATH"

                # Go directly to group-based selection
                set_supplier_context "device-supplier"
                group_management_menu
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
# Command Line Argument Parsing
################################################################################

main() {
    # Show welcome message
    cat << 'EOF'
╔═══════════════════════════════════════════════════════════════════════════╗
║           Margo Conformance Test Case Generator                           ║
║       Create test cases for API and conformance testing                   ║
╚═══════════════════════════════════════════════════════════════════════════╝

EOF

    # No arguments - show interactive menu
    if [[ $# -eq 0 ]]; then
        interactive_mode
        exit 0
    fi
    
    # Parse command line arguments
    local command="${1,,}"
    
    case "$command" in
        wfm)
            local wfm_type="${2,,}"
            case "$wfm_type" in
                openapi|contract)
                    generate_wfm_tests "${3:-}"
                    ;;
                margo|functional|template)
                    generate_wfm_functional_tests "${3:-}"
                    ;;
                "")
                    # No subcommand - use as spec path for backward compatibility
                    generate_wfm_tests "${2:-}"
                    ;;
                *)
                    error "Unknown WFM test type: $wfm_type
Usage: ./conformance.sh wfm [openapi|functional] [path]"
                    ;;
            esac
            ;;
        device)
            local device_arg="${2:-}"
            
            # If second argument is a file path, use it directly
            if [[ -f "$device_arg" ]]; then
                generate_device_tests "$device_arg"
                exit 0
            fi
            
            local device_type="${device_arg,,}"
            case "$device_type" in
                openapi|contract)
                    generate_device_openapi_tests "${3:-}"
                    ;;
                margo|functional|template|scenarios)
                    generate_device_margo_template_tests
                    ;;
                "")
                    # No device type specified - use generate_device_tests for interactive path prompt
                    generate_device_tests ""
                    ;;
                *)
                    # Try to expand and check if it's a file path
                    local expanded_path="${device_arg/\~/$HOME}"
                    if [[ -f "$expanded_path" ]]; then
                        generate_device_tests "$expanded_path"
                    else
                        error "Unknown device option: $device_type
Usage: ./conformance.sh device [/path/to/scenarios.json|openapi|margo]"
                    fi
                    ;;
            esac
            ;;
        help|-h|--help)
            show_help
            ;;
        *)
            error "Unknown command: $command

Usage: ./conformance.sh [wfm|device|help]

Run './conformance.sh help' for detailed instructions."
            ;;
    esac
}

# Run main function
main "$@"
