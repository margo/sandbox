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
# VENDOR QUICKSTART:
#   1. Customize environment: wfm-supplier/newman-data/device-agent.env.json
#      - Update deviceId, clientId with your actual device identifiers
#      - Update vendor, modelNumber, serialNumber in JSON payloads
#   2. Customize collection: Data-Generator/wfm-supplier/postman_collection.json
#      - Or use group-based collections for curated test subsets
#   3. Run tests: ./run-tests.sh and select persona, group, and WFM URL
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
WFM_GROUP_DIR="$DATA_GEN_DIR/wfm-supplier/groups"
DEVICE_GROUP_DIR="$DATA_GEN_DIR/device-supplier/groups"
APPLICATION_DIR="$CONFORMANCE_DIR/Application-Supplier"
APPLICATION_SERVICE_DIR="$CONFORMANCE_DIR/Application-Supplier-Service"

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

warn() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] ⚠️  $*" >&2
}

################################################################################
# Prerequisite Check / Install
################################################################################

# Maps a tool name to the package name for the detected package manager.
# Echoes "" if the tool/manager combo isn't recognized.
_prereq_pkg_name() {
    local mgr="$1" tool="$2"
    case "$mgr:$tool" in
        apt-get:go) echo "golang-go" ;;
        dnf:go|yum:go) echo "golang" ;;
        apk:go) echo "go" ;;
        brew:go) echo "go" ;;
        apt-get:jq|dnf:jq|yum:jq|apk:jq|brew:jq) echo "jq" ;;
        apt-get:openssl|dnf:openssl|yum:openssl|apk:openssl|brew:openssl) echo "openssl" ;;
        apt-get:node|dnf:node|yum:node|apk:node) echo "nodejs npm" ;;
        brew:node) echo "node" ;;
        *) echo "" ;;
    esac
}

# check_prerequisites detects the tools required by either persona:
#   go, jq, openssl, node/npm (WFM's scenario runner + Newman), newman itself.
# Reports what's missing and offers to install it via the detected system
# package manager (+ npm for newman). Safe to run multiple times — only
# touches packages that are actually missing, and never runs unprompted.
check_prerequisites() {
    echo ""
    info "Checking prerequisites for WFM Supplier + Device Supplier personas..."
    echo ""

    local pkg_manager=""
    if command -v apt-get >/dev/null 2>&1; then
        pkg_manager="apt-get"
    elif command -v dnf >/dev/null 2>&1; then
        pkg_manager="dnf"
    elif command -v yum >/dev/null 2>&1; then
        pkg_manager="yum"
    elif command -v apk >/dev/null 2>&1; then
        pkg_manager="apk"
    elif command -v brew >/dev/null 2>&1; then
        pkg_manager="brew"
    fi

    local -a missing_tools=()
    local -a missing_pkgs=()
    local tool pkg
    for tool in go jq openssl node; do
        if ! command -v "$tool" >/dev/null 2>&1; then
            missing_tools+=("$tool")
            if [[ -n "$pkg_manager" ]]; then
                pkg=$(_prereq_pkg_name "$pkg_manager" "$tool")
                # Intentionally unquoted: some entries (e.g. "nodejs npm") are
                # two package names and must split into separate array items.
                [[ -n "$pkg" ]] && missing_pkgs+=($pkg)
            fi
        fi
    done

    local newman_missing=false
    if ! command -v newman >/dev/null 2>&1; then
        newman_missing=true
        missing_tools+=("newman")
    fi

    if [[ ${#missing_tools[@]} -eq 0 ]]; then
        success "All prerequisites are already installed (go, jq, openssl, node, newman)."
        return 0
    fi

    warn "Missing prerequisites: ${missing_tools[*]}"

    echo ""
    read -p "Install missing prerequisites now? [y/N]: " confirm < /dev/tty
    if [[ ! "${confirm,,}" =~ ^y ]]; then
        warn "Skipped. Re-run this option any time, or install manually:"
        echo "    go       → https://golang.org/doc/install"
        echo "    jq       → install via your package manager (apt/dnf/yum/apk/brew install jq)"
        echo "    openssl  → install via your package manager (apt/dnf/yum/apk/brew install openssl)"
        echo "    node/npm → https://nodejs.org (or your package manager's nodejs/npm packages)"
        echo "    newman   → npm install -g newman newman-reporter-htmlextra"
        return 1
    fi

    local sudo_cmd=""
    [[ "$(id -u)" -ne 0 ]] && sudo_cmd="sudo"

    if [[ ${#missing_pkgs[@]} -gt 0 ]]; then
        if [[ -z "$pkg_manager" ]]; then
            warn "No supported package manager found (apt-get/dnf/yum/apk/brew) — install go/jq/openssl/node manually (see links above)."
        else
            info "Installing via $pkg_manager: ${missing_pkgs[*]}"
            case "$pkg_manager" in
                apt-get)
                    $sudo_cmd apt-get update && $sudo_cmd apt-get install -y "${missing_pkgs[@]}" || warn "apt-get install failed — see errors above."
                    ;;
                dnf)
                    $sudo_cmd dnf install -y "${missing_pkgs[@]}" || warn "dnf install failed — see errors above."
                    ;;
                yum)
                    $sudo_cmd yum install -y "${missing_pkgs[@]}" || warn "yum install failed — see errors above."
                    ;;
                apk)
                    $sudo_cmd apk add "${missing_pkgs[@]}" || warn "apk add failed — see errors above."
                    ;;
                brew)
                    brew install "${missing_pkgs[@]}" || warn "brew install failed — see errors above."
                    ;;
            esac
        fi
    fi

    if $newman_missing; then
        if command -v npm >/dev/null 2>&1; then
            info "Installing newman + newman-reporter-htmlextra via npm..."
            npm install -g newman newman-reporter-htmlextra || warn "npm install -g failed — try: sudo npm install -g newman newman-reporter-htmlextra"
        else
            warn "npm still not available — cannot install newman yet. Install Node.js first, then run: npm install -g newman newman-reporter-htmlextra"
        fi
    fi

    echo ""
    info "Re-checking..."
    local -a still_missing=()
    for tool in go jq openssl node newman; do
        command -v "$tool" >/dev/null 2>&1 || still_missing+=("$tool")
    done

    if [[ ${#still_missing[@]} -eq 0 ]]; then
        success "All prerequisites installed successfully."
        return 0
    else
        warn "Still missing: ${still_missing[*]}. You may need to open a new shell (PATH changes) or install these manually."
        return 1
    fi
}

################################################################################
# WFM Group Selection Functions
################################################################################

select_wfm_group() {
    local group_dir="$WFM_GROUP_DIR"
    
    if [[ ! -d "$group_dir" ]]; then
        warn "No groups directory found at: $group_dir" >&2
        return 1
    fi
    
    # Print to stderr so it displays to user (not captured by $() command substitution)
    {
        echo ""
        echo "📋 Available WFM Test Groups:"
        echo "================================================"
    } >&2
    
    # Collect available groups
    local groups=()
    local group_metadata=()
    
    for group_path in "$group_dir"/*; do
        if [[ -d "$group_path" && -f "$group_path/group.json" ]]; then
            local group_name=$(basename "$group_path")
            local group_json="$group_path/group.json"
            
            # Extract group info from group.json
            local version=$(jq -r '.version // "unknown"' "$group_json" 2>/dev/null || echo "unknown")
            local description=$(jq -r '.description // "No description"' "$group_json" 2>/dev/null || echo "No description")
            local test_count=$(jq '.testCases | length' "$group_json" 2>/dev/null || echo "0")
            
            groups+=("$group_path")
            group_metadata+=("$group_name|$version|$description|$test_count")
        fi
    done
    
    if [[ ${#groups[@]} -eq 0 ]]; then
        echo "❌ No test groups found. Please run conformance.sh to create groups." >&2
        return 1
    fi
    
    # Display groups to stderr
    {
        for i in "${!groups[@]}"; do
            local metadata="${group_metadata[$i]}"
            IFS='|' read -r name version desc count <<< "$metadata"
            printf "  %d) %-15s (v%s) - %d tests\n" "$((i+1))" "$name" "$version" "$count"
        done
        echo ""
    } >&2
    
    # Prompt for selection (read -p writes prompt to stderr by default)
    read -p "Select group (1-${#groups[@]}): " group_choice < /dev/tty
    
    if ! [[ "$group_choice" =~ ^[0-9]+$ ]] || [[ $group_choice -lt 1 || $group_choice -gt ${#groups[@]} ]]; then
        echo "❌ Invalid selection. Please enter a number between 1 and ${#groups[@]}" >&2
        return 1
    fi
    
    local selected_index=$((group_choice - 1))
    local selected_group="${groups[$selected_index]}"
    
    # Return group path to stdout (this will be captured by $())
    echo "$selected_group"
}

################################################################################
# Device Group Selection Function
################################################################################

select_device_group() {
    local group_dir="$DEVICE_GROUP_DIR"
    
    if [[ ! -d "$group_dir" ]]; then
        warn "No device groups directory found at: $group_dir" >&2
        return 1
    fi
    
    # Print to stderr so it displays to user (not captured by $() command substitution)
    {
        echo ""
        echo "📋 Available Device Test Groups:"
        echo "================================================"
    } >&2
    
    # Collect available groups
    local groups=()
    local group_metadata=()
    
    for group_path in "$group_dir"/*; do
        if [[ -d "$group_path" && -f "$group_path/group.json" ]]; then
            local group_name=$(basename "$group_path")
            local group_json="$group_path/group.json"
            
            # Extract group info from group.json
            local version=$(jq -r '.version // "unknown"' "$group_json" 2>/dev/null || echo "unknown")
            local description=$(jq -r '.description // "No description"' "$group_json" 2>/dev/null || echo "No description")
            local test_count=$(jq '.testCases | length' "$group_json" 2>/dev/null || echo "0")
            
            groups+=("$group_path")
            group_metadata+=("$group_name|$version|$description|$test_count")
        fi
    done
    
    if [[ ${#groups[@]} -eq 0 ]]; then
        echo "❌ No device test groups found. Please run conformance.sh to create groups." >&2
        return 1
    fi
    
    # Display groups to stderr
    {
        for i in "${!groups[@]}"; do
            local metadata="${group_metadata[$i]}"
            IFS='|' read -r name version desc count <<< "$metadata"
            printf "  %d) %-15s (v%s) - %d tests\n" "$((i+1))" "$name" "$version" "$count"
        done
        echo ""
    } >&2
    
    # Prompt for selection (read -p writes prompt to stderr by default)
    read -p "Select group (1-${#groups[@]}): " group_choice < /dev/tty
    
    if ! [[ "$group_choice" =~ ^[0-9]+$ ]] || [[ $group_choice -lt 1 || $group_choice -gt ${#groups[@]} ]]; then
        echo "❌ Invalid selection. Please enter a number between 1 and ${#groups[@]}" >&2
        return 1
    fi
    
    local selected_index=$((group_choice - 1))
    local selected_group="${groups[$selected_index]}"
    
    # Return group path to stdout (this will be captured by $())
    echo "$selected_group"
}

################################################################################
# WFM Supplier Test Execution (with Group Support)
################################################################################

execute_wfm_tests_with_url() {
    local wfm_url="${1:-}"
    
    
    # If WFM URL not provided, prompt user
    if [[ -z "$wfm_url" ]]; then
        echo ""
        read -p "Enter WFM Server Base URL [https://localhost:3001/v1alpha2/margo]: " wfm_url
        wfm_url="${wfm_url:-https://localhost:3001/v1alpha2/margo}"
    fi
    
    log "🚀 Starting WFM Supplier Test Execution"
    log "   WFM Server: $wfm_url"
    
    if run_wfm_newman "$wfm_url"; then
        success "WFM Tests Completed"
    else
        error "WFM test execution failed"
    fi
}

execute_wfm_tests() {
    local wfm_url="${1:-}"
    local group_path="${2:-}"
    
    # If group path is provided, run with group filtering
    if [[ -n "$group_path" && -d "$group_path" ]]; then
        execute_wfm_tests_with_group "$wfm_url" "$group_path"
        return $?
    fi
    
    # Otherwise run without group filtering (legacy behavior)
    execute_wfm_tests_with_url "$wfm_url"
}

resolve_group_path() {
    local group_dir="$1"
    local group_ref="$2"

    if [[ -d "$group_ref" ]]; then
        echo "$group_ref"
        return 0
    fi

    if [[ -d "$group_dir/$group_ref" ]]; then
        echo "$group_dir/$group_ref"
        return 0
    fi

    return 1
}


get_group_json_files() {
    local group_path="$1"

    find "$group_path" -maxdepth 1 -type f -name '*.json' \
        ! -name 'group.json' \
        ! -name '.*.json' \
        -print | sort
}

json_file_is_valid() {
    local json_file="$1"

    jq empty "$json_file" >/dev/null 2>&1
}


json_file_is_postman_collection() {
    local json_file="$1"

    # Accept any Postman-like collection: must have item[] and either:
    #   - standard info object (Postman v2.1, portman generated, etc.)
    #   - portman-style _ object with postman_id
    jq -e '
        (.item? | type == "array") and
        (
            (.info? | type == "object") or
            (._? | type == "object" and has("postman_id"))
        )
    ' "$json_file" >/dev/null 2>&1
}

postman_item_count() {
    local collection_file="$1"

    jq '[.. | objects | select(has("request"))] | length' "$collection_file" 2>/dev/null || echo "0"
}

filter_postman_collection_by_group() {
    local collection_file="$1"
    local group_json="$2"
    local output_file="$3"

    jq --slurpfile group "$group_json" '
        ($group[0].testCases // []) as $ids
        | def normalized_name:
            (.name? // "" | gsub(" "; "_") | ascii_downcase);
        def matches_group_id:
            ((.id? as $id | $ids | index($id)) != null)
            or ((.name? as $name | $ids | index($name)) != null)
            or ((normalized_name as $name | $ids | index($name)) != null);
        def prune_items:
            if type == "object" and (.item? | type == "array") then
                .item = [
                    .item[]
                    | prune_items
                    | select(matches_group_id or ((.item? // []) | length > 0))
                ]
            else
                .
            end;
        prune_items
    ' "$collection_file" > "$output_file"
}

discover_wfm_group_collections() {
    local group_path="$1"
    local group_json="$2"
    local collections=()

    echo "[DEBUG] Reading group.json: $group_json" >&2

    local testcase_paths=()
    mapfile -t testcase_paths < <(
        jq -r '.FolderPath[]? // empty' "$group_json" 2>/dev/null
    )

    if [[ ${#testcase_paths[@]} -eq 0 ]]; then
        echo "[WARN] FolderPath not defined in group.json" >&2
        return
    fi

    for testcases_path in "${testcase_paths[@]}"; do
         && testcases_p[[ "$testcases_path" != /* ]]ath="$CONFORMANCE_DIR/$testcases_path"

        echo "[DEBUG] Looking inside: $testcases_path" >&2

        if [[ ! -d "$testcases_path" ]]; then
            echo "[WARN] Testcases folder not found: $testcases_path" >&2
            continue
        fi

        shopt -s nullglob
        local files=("$testcases_path"/*.json)
        shopt -u nullglob

        if [[ ${#files[@]} -eq 0 ]]; then
            echo "[WARN] No JSON files found in: $testcases_path" >&2
            continue
        fi

        echo "[DEBUG] Found ${#files[@]} JSON files" >&2

        for json_file in "${files[@]}"; do
            echo "[DEBUG] Checking: $(basename "$json_file")" >&2

            if ! jq empty "$json_file" >/dev/null 2>&1; then
                echo "[WARN] Invalid JSON: $(basename "$json_file")" >&2
                continue
            fi

            echo "[DEBUG] Valid JSON" >&2

            if jq -e '
                (.item? | type == "array") and
                (
                    (.info? | type == "object") or
                    (._? | type == "object" and has("postman_id"))
                )
            ' "$json_file" >/dev/null 2>&1; then

                echo "[DEBUG] ✅ Selected as Postman collection" >&2
                collections+=("$json_file")
            else
                echo "[DEBUG] ❌ Not a Postman collection" >&2
            fi
        done
    done

    echo "[DEBUG] Total selected collections: ${#collections[@]}" >&2

    if [[ ${#collections[@]} -gt 0 ]]; then
        printf '%s\n' "${collections[@]}"
    fi
}
discover_group_scenario_files() {
    local group_path="$1"
    local scenario_files=()

    local group_json="$group_path/group.json"

    echo "[DEBUG] Reading group.json: $group_json" >&2

    local testcase_paths=()
    mapfile -t testcase_paths < <(
        jq -r '.FolderPath[]? // empty' "$group_json" 2>/dev/null
    )

    if [[ ${#testcase_paths[@]} -eq 0 ]]; then
        warn "FolderPath not defined in group.json"
        return
    fi

    for testcases_path in "${testcase_paths[@]}"; do
        [[ "$testcases_path" != /* ]] && testcases_path="$CONFORMANCE_DIR/$testcases_path"

        echo "[DEBUG] Looking for scenario files in: $testcases_path" >&2

        if [[ ! -d "$testcases_path" ]]; then
            warn "Testcases folder not found: $testcases_path"
            continue
        fi

        shopt -s nullglob
        local files=("$testcases_path"/*.json)
        shopt -u nullglob

        if [[ ${#files[@]} -eq 0 ]]; then
            warn "No JSON files found in: $testcases_path"
            continue
        fi

        echo "[DEBUG] Found ${#files[@]} JSON file(s)" >&2

        for json_file in "${files[@]}"; do
            echo "[DEBUG] Checking: $(basename "$json_file")" >&2

            if ! json_file_is_valid "$json_file"; then
                warn "Skipping invalid JSON file: $(basename "$json_file")"
                continue
            fi

            if jq -e '
                type == "array" and
                any(.[]?; type == "object" and (.steps? | type == "array"))
            ' "$json_file" >/dev/null 2>&1; then
                echo "[DEBUG] ✅ Scenario file selected: $(basename "$json_file")" >&2
                scenario_files+=("$json_file")
            else
                echo "[DEBUG] ❌ Not a scenario file: $(basename "$json_file")" >&2
            fi
        done
    done

    echo "[DEBUG] Total scenario files selected: ${#scenario_files[@]}" >&2

    if [[ ${#scenario_files[@]} -gt 0 ]]; then
        printf '%s\n' "${scenario_files[@]}"
    fi
}

build_device_group_scenarios() {
    local group_path="$1"
    local output_file="$2"
    local group_json="$group_path/group.json"
    local scenario_files=()

    mapfile -t scenario_files < <(discover_group_scenario_files "$group_path")

    if [[ ${#scenario_files[@]} -eq 0 ]]; then
        error "No scenario JSON files found in group: $group_path"
    fi

    jq -s --slurpfile group "$group_json" '
        ($group[0].testCases // []) as $ids
        | [ .[] | select(type == "array") | .[] | select(type == "object") ] as $all
        | (
            if ($ids | length) == 0 then
                # No filter: run every scenario with all its steps
                $all | map(.steps = (.steps // []))
            else
                [
                    $all[]
                    | select(
                        ((.id? as $id | $ids | index($id)) != null)
                        or (((.steps? // []) | map(.id? // empty)) as $stepIds
                            | any($stepIds[]?; . as $stepId | $ids | index($stepId)))
                    )
                    | .steps = [
                        .steps[]?
                        | select(.id? as $id | $ids | index($id) != null)
                      ]
                    | select((.steps | length) > 0)
                ]
            end
          ) as $filtered
        # If the ID-based filter matched nothing (testCases IDs are UUIDs from a
        # Postman collection, scenario IDs are string slugs), fall back to running
        # all scenarios with all their steps — preserves behaviour for groups like
        # diamond that carry both Postman and scenario files.
        | if ($filtered | length) > 0 then $filtered
          else $all | map(.steps = (.steps // []))
          end
        | unique_by(.id // .name // tostring)
    ' "${scenario_files[@]}" > "$output_file"

    local scenario_count
    scenario_count=$(jq 'length' "$output_file")

    if [[ "$scenario_count" -eq 0 ]]; then
        error "No scenarios in group files matched test IDs from: $group_json"
    fi

    info "Matched $scenario_count scenario(s) from ${#scenario_files[@]} group file(s)" >&2
}

create_temp_scenarios_file() {
    mktemp /tmp/margo-device-scenarios.XXXXXX.json
}

run_wfm_scenario_group() {
    local wfm_url="$1"
    local group_path="$2"
    local group_name="$3"
    local scenario_file
    local report_file
    local scenario_runner="$CONFORMANCE_DIR/wfm-supplier/run_wfm_scenarios.js"
    local cert_dir="$CONFORMANCE_DIR/wfm-supplier/newman-data/certs"

    command -v node >/dev/null 2>&1 || error "Node.js not found. Install Node.js before running WFM scenario tests."
    [[ -f "$scenario_runner" ]] || error "WFM scenario runner not found: $scenario_runner"
    
    # Generate fresh device certificate for each run to avoid 409 Conflict
    log "Generating fresh device certificate for test run..."
    local temp_device_id="device-$(date +%s)"
    mkdir -p "$cert_dir"
    openssl ecparam -name prime256v1 -genkey -noout -out "$cert_dir/device.key" >/dev/null 2>&1
    openssl req -new -x509 -days 365 \
        -key "$cert_dir/device.key" \
        -out "$cert_dir/device-cert.pem" \
        -subj "/C=IN/ST=GGN/L=Sector48/O=Margo/OU=Conformance/CN=$temp_device_id" >/dev/null 2>&1

    [[ -f "$cert_dir/device.key" ]] || error "Device private key not found: $cert_dir/device.key"
    [[ -f "$cert_dir/device-cert.pem" ]] || error "Device certificate not found: $cert_dir/device-cert.pem"

    scenario_file=$(create_temp_scenarios_file)
    build_device_group_scenarios "$group_path" "$scenario_file"

    report_file="$RUNNER_WFM/wfm-scenario-report-${group_name}_$(date +%Y%m%d_%H%M%S).html"

    log "▶️  Running WFM scenarios from group: $group_name"
    log "📊 Report: $report_file"

    set +e
    node "$scenario_runner" "$wfm_url" "$scenario_file" "$report_file" "$cert_dir"
    local result=$?
    set -e

    rm -f "$scenario_file"

    if [[ $result -eq 0 ]]; then
        success "WFM scenario tests completed for group: $group_name"
    else
        error "WFM scenario tests failed for group: $group_name"
    fi
}

run_wfm_newman() {
    local wfm_url="${1:-}"
    local collection_file="${2:-$CONFORMANCE_DIR/wfm-supplier/postman_collection.json}"
    local report_prefix="${3:-wfm-test-report}"
    local wfm_supplier_dir="$CONFORMANCE_DIR/wfm-supplier"
    local data_dir="$wfm_supplier_dir/newman-data"
    local env_file="$data_dir/device-agent.env.json"
    local iteration_file="$data_dir/device-agent.iteration.json"
    local cert_dir="$data_dir/certs"
    local local_ca_cert_file="$wfm_supplier_dir/certs/ca-cert.pem"
    local runtime_ca_cert_file="$cert_dir/ca-cert.pem"
    local runtime_collection="$wfm_supplier_dir/.collection.runtime.json"
    local report_file="${report_prefix}_$(date +%Y%m%d_%H%M%S).html"

    if [[ -z "$wfm_url" ]]; then
        if [[ -f "$env_file" ]]; then
            wfm_url=$(jq -r '.values[] | select(.key=="baseUrl") | .value' "$env_file" 2>/dev/null || echo "")
        fi
    fi

    [[ -z "$wfm_url" ]] && error "WFM URL not provided"
    [[ -d "$wfm_supplier_dir" ]] || error "WFM Supplier directory not found: $wfm_supplier_dir"
    [[ -f "$collection_file" ]] || error "Postman collection not found: $collection_file"
    [[ -f "$env_file" ]] || error "Newman environment not found: $env_file"

    command -v jq >/dev/null 2>&1 || error "jq not found. Install jq before running WFM tests."
    command -v newman >/dev/null 2>&1 || error "Newman not found. Install with: npm install -g newman newman-reporter-htmlextra"

    mkdir -p "$cert_dir"
    if [[ -f "$local_ca_cert_file" ]]; then
        cp "$local_ca_cert_file" "$runtime_ca_cert_file"
    elif [[ ! -f "$runtime_ca_cert_file" ]]; then
        error "Missing WFM CA certificate. Copy it to: $local_ca_cert_file"
    fi

    wfm_url="${wfm_url//v1aplha2/v1alpha2}"
    jq --arg baseUrl "$wfm_url" \
        '.values |= map(if .key == "baseUrl" then .value = $baseUrl else . end)' \
        "$env_file" > "$env_file.tmp"
    mv "$env_file.tmp" "$env_file"
    echo '[]' > "$iteration_file"

    cp "$collection_file" "$runtime_collection"
    
    # Use external jq filter file to avoid shell quoting issues
    local jq_filter_file="$wfm_supplier_dir/patch_postman_collection.jq"
    if [[ ! -f "$jq_filter_file" ]]; then
        error "JQ filter file not found: $jq_filter_file"
    fi
    
    jq -f "$jq_filter_file" "$runtime_collection" > "$runtime_collection.tmp"
    mv "$runtime_collection.tmp" "$runtime_collection"

    log "▶️  Running Newman against: $wfm_url"
    set +e
    (cd "$wfm_supplier_dir" && newman run "$runtime_collection" \
        --environment "$env_file" \
        --ssl-extra-ca-certs "$runtime_ca_cert_file" \
        --insecure \
        -r cli,htmlextra \
        --reporter-htmlextra-export "$report_file")
    local result=$?
    set -e

    rm -f "$runtime_collection" "$runtime_collection.tmp"

    if [[ -f "$wfm_supplier_dir/$report_file" ]]; then
        cp "$wfm_supplier_dir/$report_file" "$RUNNER_WFM/"
        success "Report: $RUNNER_WFM/$report_file"
    fi

    return $result
}

execute_wfm_tests_with_group() {
    local wfm_url="${1:-}"
    local group_path="${2:-}"
    
    if [[ ! -d "$group_path" ]]; then
        error "Group path not found: $group_path"
    fi
    
    local group_json="$group_path/group.json"
    local group_name=$(basename "$group_path")
    
    if [[ ! -f "$group_json" ]]; then
        error "group.json not found in: $group_path"
    fi
    
    # If WFM URL not provided, prompt user
    if [[ -z "$wfm_url" ]]; then
        echo ""
        read -p "Enter WFM Server Base URL [https://localhost:3001/v1alpha2/margo]: " wfm_url
        wfm_url="${wfm_url:-https://localhost:3001/v1alpha2/margo}"
    fi
    
    log "🚀 Starting WFM Supplier Test Execution (Group Mode)"
    log "   Group: $group_name"
    log "   WFM Server: $wfm_url"
    
    # Get group metadata
    local group_version=$(jq -r '.version // "unknown"' "$group_json")
    local group_desc=$(jq -r '.description // ""' "$group_json")
    local test_count=$(jq '.testCases | length' "$group_json")
    
    info "Group Details:"
    info "  Name: $group_name"
    info "  Version: $group_version"
    info "  Description: $group_desc"
    info "  Test cases: $test_count"

    # --- Scenario-format groups (primary path) -----------------------------------
    # Groups that store test data as a plain JSON array of scenario objects with
    # steps run through run_wfm_scenario_group.  These are identified by content,
    # not by filename.
    local group_scenario_files=()
    mapfile -t group_scenario_files < <(discover_group_scenario_files "$group_path")
    if [[ ${#group_scenario_files[@]} -gt 0 ]]; then
        log "📋 Found ${#group_scenario_files[@]} scenario file(s)"
        run_wfm_scenario_group "$wfm_url" "$group_path" "$group_name"
        return 0
    fi

    # --- Postman-collection groups (fallback path) ---------------------------
    # Discover every Postman collection in the group dir (plus any referenced via
    # collectionFiles in group.json) purely by content — file names are irrelevant.
    local group_collections=()
    mapfile -t group_collections < <(discover_wfm_group_collections "$group_path" "$group_json")

    if [[ ${#group_collections[@]} -eq 0 ]]; then
        error "No test data found for group: $group_name (place any *.json Postman collection in the group directory or list it under \"collectionFiles\" in group.json)"
    fi

    log "📋 Found ${#group_collections[@]} Postman collection file(s)"

    # Auto-sync: extract all UUIDs from every collection file and append any new
    # ones to group.json testCases so the user never has to maintain IDs manually.
    local merged_ids
    merged_ids=$(
        # Existing IDs from group.json (preserve order)
        jq -r '.testCases[]?' "$group_json" 2>/dev/null
        # UUIDs found in each collection file (in file order, deduped per-file)
        for cfile in "${group_collections[@]}"; do
            jq -r '.. | strings
                | select(test("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"))
            ' "$cfile" 2>/dev/null
        done
    )
    # Build deduplicated list preserving first-seen order
    local unique_ids
    unique_ids=$(echo "$merged_ids" | awk '!seen[$0]++')

    if [[ -n "$unique_ids" ]]; then
        local id_json
        id_json=$(echo "$unique_ids" | jq -R . | jq -s .)
        local current_count new_count
        current_count=$(jq '.testCases | length' "$group_json")
        jq --argjson ids "$id_json" '.testCases = $ids' "$group_json" > "$group_json.tmp" \
            && mv "$group_json.tmp" "$group_json"
        new_count=$(jq '.testCases | length' "$group_json")
        [[ "$new_count" -gt "$current_count" ]] && \
            info "Synced $new_count test IDs to group.json (was $current_count)"
    fi

    local scenario_runner="$CONFORMANCE_DIR/wfm-supplier/run_wfm_scenarios.js"
    command -v node >/dev/null 2>&1 || error "Node.js not found. Install Node.js before running WFM tests."
    [[ -f "$scenario_runner" ]] || error "WFM scenario runner not found: $scenario_runner"

    local cert_dir="$CONFORMANCE_DIR/wfm-supplier/newman-data/certs"
    local temp_device_id="device-$(date +%s)"
    mkdir -p "$cert_dir"
    openssl ecparam -name prime256v1 -genkey -noout -out "$cert_dir/device.key" >/dev/null 2>&1
    openssl req -new -x509 -days 365 \
        -key "$cert_dir/device.key" \
        -out "$cert_dir/device-cert.pem" \
        -subj "/C=IN/ST=GGN/L=Sector48/O=Margo/OU=Conformance/CN=$temp_device_id" >/dev/null 2>&1

    log "▶️  Running test cases from group: $group_name"
    echo ""

    local collection_index=0
    local executed_collections=0
    for group_collection in "${group_collections[@]}"; do
        collection_index=$((collection_index + 1))

        local filtered_collection="$group_path/.collection.${group_name}.${collection_index}.filtered.json"
        filter_postman_collection_by_group "$group_collection" "$group_json" "$filtered_collection"

        local matched_items
        matched_items=$(postman_item_count "$filtered_collection")
        if [[ "$matched_items" -eq 0 ]]; then
            warn "Skipping $(basename "$group_collection"): no runnable Postman items matched group.json"
            rm -f "$filtered_collection"
            continue
        fi

        log "   Collection: $(basename "$group_collection")"
        log "   Matched items: $matched_items"

        local report_suffix="${group_name}"
        [[ ${#group_collections[@]} -gt 1 ]] && report_suffix="${group_name}-${collection_index}"
        local report_file="$RUNNER_WFM/wfm-scenario-report-${report_suffix}_$(date +%Y%m%d_%H%M%S).html"
        log "📊 Report: $(basename "$report_file")"

        set +e
        node "$scenario_runner" "$wfm_url" "$filtered_collection" "$report_file" "$cert_dir"
        local result=$?
        set -e

        rm -f "$filtered_collection"

        if [[ $result -eq 0 ]]; then
            executed_collections=$((executed_collections + 1))
            success "WFM collection completed: $(basename "$group_collection")"
        else
            error "WFM test execution failed for group: $group_name"
        fi
    done

    if [[ "$executed_collections" -eq 0 ]]; then
        error "No runnable Postman items in $group_name matched test IDs from group.json"
    fi

    success "WFM Tests Completed for group: $group_name"
}

################################################################################
# Device Test Scenarios Selection
################################################################################

show_device_test_scenarios_menu() {
    echo "" >&2
    echo "Which test scenarios would you like to run?" >&2
    echo "1. Group-based test scenarios (select from available groups)" >&2
    echo "" >&2
    echo "Q) Quit" >&2
    echo "" >&2
}

select_device_test_scenarios() {
    while true; do
        show_device_test_scenarios_menu
        # Print prompt to stderr so it does not get captured in command substitution
        echo -n "Select option (1 or Q): " >&2
        read choice
        
        case "${choice,,}" in
            1|group)
                local device_group
                device_group=$(select_device_group)
                if [[ -z "$device_group" ]]; then
                    error "No device group selected"
                fi
                
                local group_scenarios
                group_scenarios=$(create_temp_scenarios_file)
                build_device_group_scenarios "$device_group" "$group_scenarios"

                echo "$group_scenarios"
                return 0
                ;;
            q|quit)
                info "Exiting..."
                exit 0
                ;;
            *)
                error "Invalid option. Please select 1 or Q"
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
    
    # Build mock server if not already built
    if [[ ! -f "bin/server" ]]; then
        log "📦 Building mock WFM server..."
        go build -o bin/server ./cmd/device-supplier || error "Failed to build mock server"
    fi
    
    # Build test runner if not already built
    if [[ ! -f "bin/run_tests" ]]; then
        log "📦 Building device test runner..."
        go build -o bin/run_tests run_tests.go || error "Failed to build test runner"
    fi
    
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
# Persona Selection Menu
################################################################################

show_persona_menu() {
    echo ""
    echo "Which Margo Persona do you want to test?"
    echo "1. WFM Supplier"
    echo "2. Device Supplier"
    echo "3. Application Supplier"
    echo ""
    echo "P) Check/Install Prerequisites (go, jq, openssl, node, newman)"
    echo "H) Help"
    echo "Q) Quit"
    echo ""
}

select_application() {

    local app_dir="$APPLICATION_DIR"
    local apps=()

    {
        echo ""
        echo "Available Applications"
        echo "======================"
    } >&2

    for dir in "$app_dir"/*; do
        [[ -d "$dir" ]] && apps+=("$dir")
    done

    if [[ ${#apps[@]} -eq 0 ]]; then
        echo "No applications found in $app_dir" >&2
        return 1
    fi

    {
        for i in "${!apps[@]}"; do
            printf "  %d) %s\n" \
                "$((i+1))" \
                "$(basename "${apps[$i]}")"
        done
        echo ""
    } >&2

    read -p "Select Application (1-${#apps[@]}): " choice < /dev/tty

    if ! [[ "$choice" =~ ^[0-9]+$ ]] || \
       [[ $choice -lt 1 || $choice -gt ${#apps[@]} ]]; then
        echo "Invalid selection" >&2
        return 1
    fi

    echo "${apps[$((choice-1))]}"
}

run_application_supplier() {
    local selected_app

    selected_app=$(select_application)

    local app_name
    app_name=$(basename "$selected_app")

    log "Selected Application: $app_name"

    cd "$APPLICATION_SERVICE_DIR" || \
        error "Unable to enter Application Supplier Service"

    log "Running Application Validation..."

    go run . "$selected_app"

    local result=$?

    # if [[ $result -eq 0 ]]; then
    #     success "Application Validation Passed"
    # else
    #     error "Application Validation Failed"
    # fi
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
  ./run-tests.sh                         # Interactive menu
  ./run-tests.sh wfm [GROUP] [WFM_URL]    # Run WFM tests
  ./run-tests.sh device [GROUP|SCENARIOS] # Run Device tests
  ./run-tests.sh help                    # Show this help

PERSONAS:

  WFM Supplier:
    • Tests Workload Fleet Manager compliance
    • Runs API contract tests from Postman collection
    • Supports test grouping for targeted testing
    • Uses Newman test executor
    • Generates HTML report with test results (group-specific if group is selected)

  Device Supplier:
    • Tests device conformance with Margo API
    • Runs functional test scenarios
    • Uses mock WFM server for validation
    • Generates conformance report with assertion results
  Application Supplier:
    • Validates application package structure
    • Runs Application Supplier Service
    • Executes application conformance checks

WORKFLOW:

  1. Run conformance.sh (CLI #1) to prepare test cases
     ./conformance.sh
     → Select persona and test type
     → Select or create test groups
     → Test cases prepared and grouped in Data-Generator/

  2. Run run-tests.sh (CLI #2) to execute tests
     ./run-tests.sh
     → Select persona
     → WFM Supplier: Select test group and provide WFM URL
     → Device Supplier: Select group-based scenarios
     → Reports generated in Runner/ (grouped by test group)

  3. Review conformance report
     • WFM report: Runner/wfm-supplier/ (organized by group)
     • Device report: Runner/device-supplier/

REQUIREMENTS:

  WFM Supplier:
    • npm (for Newman)
    • Install: npm install -g newman
    • Test data: Data-Generator/wfm-supplier/postman_collection_functional.json
    • Test groups: Data-Generator/wfm-supplier/groups/*/group.json

  Device Supplier:
    • Go 1.13+ (for test runner)
    • Test data: Data-Generator/device-supplier/test-scenarios.json
    • Mock server: device-supplier/run_tests.go

TEST GROUPS (WFM Supplier):

  Groups allow you to organize and run targeted test suites:
  
  • Create groups in CLI #1 (conformance.sh):
    - Select WFM Supplier → Functional Tests
    - Select/Create group with specific test cases
    - Tests are extracted from JSON files and stored in group.json
  
  • Run specific group in CLI #2 (run-tests.sh):
    - Select WFM Supplier
    - Choose which group to execute
    - Only tests in that group's group.json will run
    - Report will include group name and metadata
  
  Group Structure:
    groups/
    ├── diamond/
    │   ├── group.json (metadata + test case IDs)
    │   ├── postman_collection.json (group collection)
    │   └── ... (supporting files)
    ├── silver/
    └── rishabh/
  
  Example group.json:
    {
      "name": "diamond",
      "version": "1.0.4",
      "persona": "wfm-supplier",
      "description": "Diamond tier conformance tests",
      "testCases": ["id1", "id2", ...]
    }

EXAMPLE WORKFLOW:

  # Step 1: Generate tests with CLI #1
  cd conformance
  ./conformance.sh
  → Select: 1 (WFM Supplier)
  → Select: 1 (OpenAPI spec)
  → Enter: /path/to/openapi.yaml
  → Tests generated in Data-Generator/wfm-supplier/

  # Step 2: Run tests with CLI #2
  cd conformance
  ./run-tests.sh
  → Select: 1 (WFM Supplier)
  → Enter WFM URL
  → Tests execute
  → Report in Runner/wfm-supplier/

  # Step 3: Review results
  open Runner/wfm-supplier/wfm-test-report-*.html

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

run_wfm_flow() {

    while true; do
        echo ""
        echo "What type of test-cases do you want to run?"
        echo "1. OpenAPI spec based contract tests"
        echo "2. Functional tests (Group-based test management)"
        echo ""
        echo "B) Back"
        echo "Q) Quit"
        echo ""

        read -p "Select option (1-2, B, or Q): " test_choice

        case "${test_choice,,}" in

            1)
                show_wfm_cert_info

                echo ""
                read -p "Enter Postman Collection Path: " collection_path

                if [[ ! -f "$collection_path" ]]; then
                    error "Collection file not found: $collection_path"
                fi

                echo ""
                read -p "Enter WFM Server Base URL [https://localhost:3001/v1alpha2/margo]: " wfm_url
                wfm_url="${wfm_url:-https://localhost:3001/v1alpha2/margo}"

                run_wfm_newman "$wfm_url" "$collection_path"
                ;;

            2)
                show_wfm_cert_info

                echo ""
                info "Selecting test group..."

                local selected_group_path
                if selected_group_path=$(select_wfm_group); then
                    local group_name
                    group_name=$(basename "$selected_group_path")
                    success "Selected group: $group_name"

                    echo ""
                    read -p "Enter WFM Server Base URL [https://localhost:3001/v1alpha2/margo]: " wfm_url
                    wfm_url="${wfm_url:-https://localhost:3001/v1alpha2/margo}"

                    execute_wfm_tests_with_group "$wfm_url" "$selected_group_path"
                else
                    error "Failed to select group"
                fi
                ;;

            b)
                return
                ;;

            q)
                info "Exiting..."
                exit 0
                ;;

            *)
                warn "Invalid option"
                ;;
        esac
    done
}

device_generate_certs() {
    local device_dir="$CONFORMANCE_DIR/device-supplier"
    local cert_dir="$device_dir/certs"

    log "🔐 Generating TLS certificates for Mock WFM Server..."

    if [[ ! -f "$device_dir/generate-certs.sh" ]]; then
        error "generate-certs.sh not found in: $device_dir"
    fi

    # Detect host IP (same logic as generate-certs.sh default)
    local host_ip
    host_ip=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "")
    local server_host="${host_ip:-localhost}"

    cd "$device_dir"
    bash generate-certs.sh "$cert_dir" "$server_host" || error "Certificate generation failed"

    echo ""
    success "Certificates generated successfully!"
    echo ""
    echo "  Certificate directory          : $cert_dir"
    echo "  CA cert (give to device-agent) : $cert_dir/ca-cert.pem"
    echo "  Device certificate             : $cert_dir/device-cert.pem"
    echo ""
    echo "  ➜  Copy ca-cert.pem to your device-agent machine so it can trust the mock WFM."
    echo ""
}

device_start_server() {
    local device_dir="$CONFORMANCE_DIR/device-supplier"
    local cert_dir="$device_dir/certs"

    # Check certs exist
    if [[ ! -f "$cert_dir/ca-cert.pem" ]]; then
        warn "Certificates not found at $cert_dir. Please run 'Generate Certificates' first (option 1)."
        return 1
    fi

    cd "$device_dir"

    # Check Go is installed
    if ! command -v go &> /dev/null; then
        error "Go not found. Install from https://golang.org/doc/install"
    fi

    # Build mock server if needed
    if [[ ! -f "bin/server" ]]; then
        log "📦 Building mock WFM server..."
        go build -o bin/server ./cmd/device-supplier || error "Failed to build mock server"
    fi

    # Stop any stale server on port 3001
    if [[ -f /tmp/wfm-server.pid ]]; then
        local old_pid
        old_pid=$(cat /tmp/wfm-server.pid)
        if kill -0 "$old_pid" 2>/dev/null; then
            log "⛔ Stopping previous server instance (PID: $old_pid)..."
            kill -15 "$old_pid" 2>/dev/null
            sleep 1
        fi
        rm -f /tmp/wfm-server.pid
    fi

    if command -v lsof &> /dev/null; then
        local pid_on_port
        pid_on_port=$(lsof -ti :3001 2>/dev/null || true)
        if [[ -n "$pid_on_port" ]]; then
            log "⛔ Stopping process on port 3001 (PID: $pid_on_port)..."
            kill -15 "$pid_on_port" 2>/dev/null
            sleep 1
        fi
    fi

    # Start server in background (must run from device_dir so it finds ./data, ./manifests, ./certs)
    log "🚀 Starting Mock WFM Server..."
    (cd "$device_dir" && exec ./bin/server) > /tmp/wfm-server.log 2>&1 &
    local server_pid=$!
    echo $server_pid > /tmp/wfm-server.pid
    sleep 2

    if ! kill -0 "$server_pid" 2>/dev/null; then
        error "Failed to start mock server. Check /tmp/wfm-server.log"
    fi

    # Detect host IP for URL display
    local host_ip
    host_ip=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "")
    local server_host="${host_ip:-localhost}"
    local mock_url="https://${server_host}:3001/v1alpha2/margo"

    echo ""
    success "Mock WFM Server is running (PID: $server_pid)"
    echo ""
    echo "╔══════════════════════════════════════════════════════════════════════════════╗"
    echo "║  Mock WFM Server is ready. Share these details with your device-agent:      ║"
    echo "╠══════════════════════════════════════════════════════════════════════════════╣"
    printf "║  WFM URL  : %-63s║\n" "$mock_url"
    printf "║  CA Cert  : %-63s║\n" "$cert_dir/ca-cert.pem"
    echo "╚══════════════════════════════════════════════════════════════════════════════╝"
    echo ""
    echo "  ➜  Start your device-agent pointing at the WFM URL above."
    echo "  ➜  The device-agent must trust the CA certificate listed above."
    echo "  ➜  Onboarding must be the first API call; subsequent calls can be in any order."
    echo "  ➜  Once your device-agent is running, return here and select 'Run Tests' (option 3)."
    echo ""
}

device_run_tests() {
    local device_dir="$CONFORMANCE_DIR/device-supplier"

    # Check server is running
    if [[ ! -f /tmp/wfm-server.pid ]] || ! kill -0 "$(cat /tmp/wfm-server.pid)" 2>/dev/null; then
        warn "Mock WFM Server is not running. Please start it first (option 2)."
        return 1
    fi

    cd "$device_dir"

    # Check Go is installed
    if ! command -v go &> /dev/null; then
        error "Go not found. Install from https://golang.org/doc/install"
    fi

    # Build test runner if needed
    if [[ ! -f "bin/run_tests" ]]; then
        log "📦 Building device test runner..."
        go build -o bin/run_tests run_tests.go || error "Failed to build test runner"
    fi

    # Group selection
    echo ""
    info "Select the test group to validate..."
    local device_group
    if ! device_group=$(select_device_group); then
        warn "No group selected."
        return 1
    fi

    local group_name
    group_name=$(basename "$device_group")

    local group_scenarios
    group_scenarios=$(create_temp_scenarios_file)
    build_device_group_scenarios "$device_group" "$group_scenarios"

    # Stage scenarios for test runner
    log "📋 Staging test scenarios for group: $group_name..."
    mkdir -p ./device-scenarios
    cp "$group_scenarios" ./device-scenarios/test-scenarios.json
    rm -f "$group_scenarios"

    # Groups may opt into flexible-order mode (fixed_first onboarding, then the
    # rest of the scenarios in a random relative order) via a "flexibleOrder"
    # key in their group.json. Absent/false for every existing group, so this
    # is a no-op for them.
    local extra_flags=()
    if [[ "$(jq -r '.flexibleOrder // false' "$device_group/group.json" 2>/dev/null)" == "true" ]]; then
        extra_flags+=("-flexible-order")
        info "🔀 Flexible-order mode enabled for group '$group_name'."
    fi

    # Prompt user for the WFM URL — default to the detected host IP so it
    # matches the URL shown in step 2 (Start Mock WFM Server)
    local host_ip
    host_ip=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "")
    local default_url="https://${host_ip:-localhost}:3001/v1alpha2/margo"
    echo ""
    echo "  ➜  This simulates your device-agent connecting to the Mock WFM Server."
    read -p "Enter Mock WFM Server URL [$default_url]: " wfm_url < /dev/tty
    wfm_url="${wfm_url:-$default_url}"

    log "▶️  Running Device Conformance Tests against: $wfm_url"
    echo ""

    local test_result=0
    if ./bin/run_tests -url "$wfm_url" "${extra_flags[@]}" 2>&1 | tee "$RUNNER_DEVICE/test-execution.log"; then
        test_result=0
    else
        test_result=1
    fi

    # Copy report to Runner output directory
    local latest_report
    latest_report=$(find reports -name "conformance-report-*.html" -type f -printf '%T@ %p\n' 2>/dev/null | sort -rn | head -1 | cut -d' ' -f2- || true)

    if [[ -n "$latest_report" && -f "$latest_report" ]]; then
        cp "$latest_report" "$RUNNER_DEVICE/"
        success "Report: $RUNNER_DEVICE/$(basename "$latest_report")"
    else
        info "Reports location: $device_dir/reports/"
    fi

    echo ""
    if [[ $test_result -ne 0 ]]; then
        warn "Some tests failed. Check: $RUNNER_DEVICE/test-execution.log"
    else
        success "All device conformance tests passed!"
    fi
}

device_stop_server() {
    local stopped=0

    if [[ -f /tmp/wfm-server.pid ]]; then
        local pid
        pid=$(cat /tmp/wfm-server.pid)
        if kill -0 "$pid" 2>/dev/null; then
            log "⛔ Stopping Mock WFM Server (PID: $pid)..."
            kill -15 "$pid" 2>/dev/null
            sleep 1
            if ! kill -0 "$pid" 2>/dev/null; then
                success "Mock WFM Server stopped."
                stopped=1
            else
                warn "Server did not stop cleanly. Try: kill -9 $pid"
            fi
        else
            info "PID $pid is no longer active."
        fi
        rm -f /tmp/wfm-server.pid
    fi

    # Also clear anything still on port 3001
    if command -v lsof &> /dev/null; then
        local pid_on_port
        pid_on_port=$(lsof -ti :3001 2>/dev/null || true)
        if [[ -n "$pid_on_port" ]]; then
            log "⛔ Stopping remaining process on port 3001 (PID: $pid_on_port)..."
            kill -15 "$pid_on_port" 2>/dev/null
            sleep 1
            stopped=1
        fi
    fi

    if [[ $stopped -eq 0 ]]; then
        info "No Mock WFM Server was running."
    fi
}

run_device_flow() {
    while true; do
        # Show live server status in the menu header
        local server_status="⛔ Stopped"
        if [[ -f /tmp/wfm-server.pid ]] && kill -0 "$(cat /tmp/wfm-server.pid)" 2>/dev/null; then
            local _running_pid
            _running_pid=$(cat /tmp/wfm-server.pid)
            server_status="✅ Running (PID: $_running_pid)"
        fi

        echo ""
        echo "┌─────────────────────────────────────────────────────────────────────────┐"
        echo "│              Device Supplier - Conformance Testing                       │"
        echo "│  Mock WFM Server: $server_status"
        echo "├─────────────────────────────────────────────────────────────────────────┤"
        echo "│  Run steps in order:                                                     │"
        echo "│    1. Generate Certificates  (run once per setup)                        │"
        echo "│    2. Start Mock WFM Server  (prints URL for device-agent)               │"
        echo "│    3. Run Tests              (select group, validate conformance)         │"
        echo "│    4. Stop Mock WFM Server                                               │"
        echo "│                                                                          │"
        echo "│  B) Back to main menu                                                    │"
        echo "└─────────────────────────────────────────────────────────────────────────┘"
        echo ""

        read -p "Select option (1-4 or B): " device_choice < /dev/tty

        case "${device_choice,,}" in
            1) device_generate_certs || true ;;
            2) device_start_server || true ;;
            3) device_run_tests || true ;;
            4) device_stop_server || true ;;
            b|back)
                return 0
                ;;
            *)
                warn "Invalid option. Please select 1-4 or B."
                ;;
        esac

        echo ""
        read -p "Press Enter to continue..." _ < /dev/tty
    done
}

################################################################################
# Interactive Mode
################################################################################

interactive_mode() {
    while true; do
        show_persona_menu
        
        read -p "Select option (1-3, P, H, or Q): " choice

        case "${choice,,}" in
            1|wfm)
                echo ""
                info "You selected: WFM Supplier"
                run_wfm_flow
                ;;
            2|device)
                echo ""
                info "You selected: Device Supplier"
                run_device_flow
                ;;
            3|application)
                echo ""
                info "You selected: Application Supplier"
                run_application_supplier
                ;;
            p|prereq|prerequisites)
                check_prerequisites || true
                ;;
            h|help)
                show_help
                ;;
            q|quit)
                info "Exiting..."
                exit 0
                ;;
            *)
                error "Invalid option. Please select 1, 2,3, P, H, or Q"
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
            if [[ -n "${2:-}" ]]; then
                local group_path
                group_path=$(resolve_group_path "$WFM_GROUP_DIR" "$2") || \
                    error "WFM group not found: $2"
                execute_wfm_tests_with_group "${3:-}" "$group_path"
            else
                run_wfm_flow
            fi
            ;;
        device)
            if [[ -f "${2:-}" ]]; then
                execute_device_tests "$2"
            elif [[ -n "${2:-}" ]]; then
                local group_path
                group_path=$(resolve_group_path "$DEVICE_GROUP_DIR" "$2") || \
                    error "Device group or scenarios file not found: $2"
                local group_scenarios
                group_scenarios=$(create_temp_scenarios_file)
                build_device_group_scenarios "$group_path" "$group_scenarios"
                execute_device_tests "$group_scenarios"
                rm -f "$group_scenarios"
            else
                run_device_flow
            fi
            ;;
        3|application)
            run_application_supplier
            ;;
        help|-h|--help)
            show_help
            ;;
        *)
            error "Unknown command: $command

Usage: ./run-tests.sh [wfm|device|application|help]

Run './run-tests.sh help' for detailed instructions."
            ;;
    esac
}

# Run main function
main "$@"
