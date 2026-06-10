#!/usr/bin/env bash

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFORMANCE_DIR="$SCRIPT_DIR"
DATA_GEN_DIR="$CONFORMANCE_DIR/Data-Generator"
GROUP_DIR="$DATA_GEN_DIR/wfm-supplier/groups"

TEMP_FILES=()
trap 'for file in "${TEMP_FILES[@]:-}"; do [[ -n "$file" ]] && rm -f "$file"; done' EXIT

log() { printf '[%s] %s\n' "$(date +'%Y-%m-%d %H:%M:%S')" "$*"; }
info() { log "INFO: $*"; }
success() { log "OK: $*"; }
warn() { log "WARN: $*" >&2; }
die() { log "ERROR: $*" >&2; exit 1; }

usage() {
    cat <<USAGE
Margo Conformance Test Case Generator

Usage:
  bash conformance.sh
  bash conformance.sh help

  bash conformance.sh wfm [openapi|contract] <openapi-file-or-url>
  bash conformance.sh wfm functional <postman-collection.json>
  bash conformance.sh wfm <openapi-file-or-url>

  bash conformance.sh device <test-scenarios.json>
  bash conformance.sh device scenarios [test-scenarios.json]
  bash conformance.sh device margo
  bash conformance.sh device openapi <api-url>

Examples:
  bash conformance.sh wfm ./wfm-supplier/spec.yaml
  bash conformance.sh wfm functional ./manual-test-cases/postman_collection_sample.json
  bash conformance.sh device margo
  bash conformance.sh device ./device-supplier/device-scenarios/test-scenarios.json
  bash conformance.sh device openapi https://symphony.machine:8082/v1alpha2/margo

Outputs:
  WFM files:    $DATA_GEN_DIR/wfm-supplier/
  Device files: $DATA_GEN_DIR/device-supplier/
USAGE
}

prompt_required() {
    local label="$1"
    local value=""
    read -r -p "$label: " value
    [[ -n "$value" ]] || die "$label cannot be empty"
    printf '%s\n' "$value"
}

expand_path() {
    local value="$1"
    case "$value" in
        "~") printf '%s\n' "$HOME" ;;
        "~/"*) printf '%s\n' "$HOME/${value#~/}" ;;
        *) printf '%s\n' "$value" ;;
    esac
}

is_url() {
    [[ "$1" == http://* || "$1" == https://* ]]
}

need_command() {
    local command="$1"
    local install_hint="${2:-}"
    if ! command -v "$command" >/dev/null 2>&1; then
        if [[ -n "$install_hint" ]]; then
            die "Missing required command '$command'. $install_hint"
        fi
        die "Missing required command '$command'"
    fi
}

ensure_file() {
    local path="$1"
    local label="$2"
    [[ -f "$path" ]] || die "$label not found: $path"
    [[ -s "$path" ]] || die "$label is empty: $path"
}

ensure_json() {
    local path="$1"
    local label="$2"
    need_command jq "Install jq and try again."
    jq empty "$path" >/dev/null 2>&1 || die "Invalid JSON in $label: $path"
}

download_to_temp() {
    local url="$1"
    local suffix="${2:-tmp}"
    local temp_file
    temp_file="$(mktemp "/tmp/margo-conformance.XXXXXX.$suffix")"
    TEMP_FILES+=("$temp_file")

    need_command curl "Install curl and try again."
    curl -fsSL "$url" -o "$temp_file" || die "Could not download: $url"
    ensure_file "$temp_file" "Downloaded file"
    printf '%s\n' "$temp_file"
}

copy_files_from_dir() {
    local source_dir="$1"
    local target_dir="$2"
    [[ -d "$source_dir" ]] || return 0

    local copied=false
    local entry
    shopt -s nullglob dotglob
    for entry in "$source_dir"/*; do
        cp -R "$entry" "$target_dir/"
        copied=true
    done
    shopt -u nullglob dotglob

    if [[ "$copied" == true ]]; then
        info "Copied supporting files from $source_dir"
    fi
}

show_generated_files() {
    local dir="$1"
    [[ -d "$dir" ]] || return 0
    info "Generated files:"
    find "$dir" -maxdepth 1 -type f -printf '  - %f (%s bytes)\n' | sort
}

run_portman() {
    local spec_file="$1"
    local output_file="$2"

    need_command portman "Install Portman, for example: npm install -g portman"
    portman -l "$spec_file" -o "$output_file" >/dev/null || die "Portman generation failed"
}

resolve_openapi_spec() {
    local spec_path="$1"
    if is_url "$spec_path"; then
        info "Downloading OpenAPI specification..." >&2
        download_to_temp "$spec_path" "yaml"
    else
        spec_path="$(expand_path "$spec_path")"
        ensure_file "$spec_path" "OpenAPI spec"
        local spec_dir spec_name
        spec_dir="$(cd "$(dirname "$spec_path")" && pwd -P)"
        spec_name="$(basename "$spec_path")"
        printf '%s\n' "$spec_dir/$spec_name"
    fi
}

generate_wfm_openapi_tests() {
    local openapi_spec_path="${1:-}"
    [[ -n "$openapi_spec_path" ]] || openapi_spec_path="$(prompt_required "OpenAPI Spec Path")"

    local output_dir="$DATA_GEN_DIR/wfm-supplier"
    local work_dir="$CONFORMANCE_DIR/wfm-supplier"
    local spec_file

    [[ -d "$work_dir" ]] || die "WFM supplier directory not found: $work_dir"
    mkdir -p "$output_dir"

    spec_file="$(resolve_openapi_spec "$openapi_spec_path")"

    info "Generating WFM OpenAPI contract tests..."
    (
        cd "$work_dir"
        run_portman "$spec_file" "postman_collection.json"
    )

    cp "$work_dir/postman_collection.json" "$output_dir/"
    copy_files_from_dir "$work_dir/newman-data" "$output_dir"

    success "WFM OpenAPI tests prepared: $output_dir"
    show_generated_files "$output_dir"
}

validate_postman_collection() {
    local collection_path="$1"
    ensure_file "$collection_path" "Postman collection"
    ensure_json "$collection_path" "Postman collection"

    jq -e '.info and .item and (.item | type == "array")' "$collection_path" >/dev/null \
        || die "Invalid Postman collection. Expected .info and array .item fields."
}

generate_wfm_functional_tests() {
    local collection_path="${1:-}"
    [[ -n "$collection_path" ]] || collection_path="$(prompt_required "Postman Collection JSON Path")"
    collection_path="$(expand_path "$collection_path")"

    local output_dir="$DATA_GEN_DIR/wfm-supplier"
    mkdir -p "$output_dir"

    validate_postman_collection "$collection_path"

    cp "$collection_path" "$output_dir/postman_collection_functional.json"
    touch "$output_dir/.functional-tests"

    success "WFM functional tests prepared: $output_dir/postman_collection_functional.json"
}

default_device_scenarios_path() {
    printf '%s\n' "$CONFORMANCE_DIR/device-supplier/device-scenarios/test-scenarios.json"
}

default_device_assertions_path() {
    printf '%s\n' "$CONFORMANCE_DIR/device-supplier/manifests/assertions.json"
}

assertions_for_scenarios() {
    local scenarios_path="$1"
    if [[ "$scenarios_path" == *"/device-scenarios/"* ]]; then
        printf '%s\n' "${scenarios_path%/*}/../manifests/assertions.json"
    else
        default_device_assertions_path
    fi
}

validate_device_scenarios() {
    local scenarios_path="$1"
    local assertions_path="$2"

    ensure_file "$scenarios_path" "Device test scenarios"
    ensure_json "$scenarios_path" "device test scenarios"

    jq -e 'type == "array" and length > 0' "$scenarios_path" >/dev/null \
        || die "Device scenarios must be a non-empty JSON array."

    jq -e 'all(.[]; has("id") and has("name") and has("steps"))' "$scenarios_path" >/dev/null \
        || die "Each device scenario must include id, name, and steps."

    if [[ -f "$assertions_path" ]]; then
        ensure_json "$assertions_path" "device assertions"
        jq -e 'has("endpoints")' "$assertions_path" >/dev/null \
            || warn "Assertions file has no endpoints section: $assertions_path"
    else
        warn "Assertions file not found, continuing without it: $assertions_path"
    fi
}

generate_device_scenario_tests() {
    local scenarios_path="${1:-}"
    [[ -n "$scenarios_path" ]] || scenarios_path="$(default_device_scenarios_path)"
    scenarios_path="$(expand_path "$scenarios_path")"

    local output_dir="$DATA_GEN_DIR/device-supplier"
    local assertions_path
    assertions_path="$(assertions_for_scenarios "$scenarios_path")"

    mkdir -p "$output_dir"
    validate_device_scenarios "$scenarios_path" "$assertions_path"

    cp "$scenarios_path" "$output_dir/test-scenarios.json"
    [[ -f "$assertions_path" ]] && cp "$assertions_path" "$output_dir/assertions.json"
    copy_files_from_dir "$CONFORMANCE_DIR/device-supplier/manifests" "$output_dir"
    touch "$output_dir/.device-scenarios"

    local scenario_count
    scenario_count="$(jq 'length' "$scenarios_path")"
    success "Device scenario tests prepared: $output_dir"
    info "Scenario count: $scenario_count"
    show_generated_files "$output_dir"
}

generate_device_openapi_tests() {
    local api_url="${1:-}"
    [[ -n "$api_url" ]] || api_url="$(prompt_required "Device Supplier API URL")"

    local output_dir="$DATA_GEN_DIR/device-supplier"
    local work_dir="$CONFORMANCE_DIR/device-supplier"
    local spec_file

    [[ -d "$work_dir" ]] || die "Device supplier directory not found: $work_dir"
    mkdir -p "$output_dir"

    api_url="${api_url%/}"
    spec_file="$(download_to_temp "$api_url/openapi" "yaml")"

    info "Generating Device OpenAPI contract tests..."
    (
        cd "$work_dir"
        run_portman "$spec_file" "device_postman_collection.json"
    )

    cp "$work_dir/device_postman_collection.json" "$output_dir/"
    copy_files_from_dir "$work_dir/newman-data" "$output_dir"

    success "Device OpenAPI tests prepared: $output_dir/device_postman_collection.json"
}

show_main_menu() {
    cat <<MENU

Which Margo persona do you want to create test cases for?
  1. WFM Supplier
  2. Device Supplier
  H. Help
  Q. Quit
MENU
}

show_wfm_menu() {
    cat <<MENU

WFM Supplier test type:
  1. OpenAPI contract tests
  2. Functional tests from Postman collection
  B. Back
  Q. Quit
MENU
}

show_device_menu() {
    cat <<MENU

Device Supplier test type:
  1. Existing MARGO test scenarios
  2. Custom test scenarios JSON
  3. OpenAPI contract tests from API URL
  B. Back
  Q. Quit
MENU
}

interactive_mode() {
    local choice
    while true; do
        show_main_menu
        read -r -p "Select option: " choice
        case "${choice,,}" in
            1|wfm) interactive_wfm ;;
            2|device) interactive_device ;;
            h|help) usage ;;
            q|quit|exit) exit 0 ;;
            *) warn "Invalid option: $choice" ;;
        esac
    done
}

interactive_wfm() {
    local choice value
    while true; do
        show_wfm_menu
        read -r -p "Select option: " choice
        case "${choice,,}" in
            1|openapi|contract)
                value="$(prompt_required "OpenAPI Spec Path or URL")"
                generate_wfm_openapi_tests "$value"
                return
                ;;
            2|functional|margo|template)
                group_management_menu
                return
                ;;
            b|back) return ;;
            q|quit|exit) exit 0 ;;
            *) warn "Invalid option: $choice" ;;
        esac
    done
}

interactive_device() {
    local choice value
    while true; do
        show_device_menu
        read -r -p "Select option: " choice
        case "${choice,,}" in
            1|existing|margo|functional|template|scenarios)
                generate_device_scenario_tests "$(default_device_scenarios_path)"
                return
                ;;
            2|custom)
                value="$(prompt_required "Custom test scenarios JSON Path")"
                generate_device_scenario_tests "$value"
                return
                ;;
            3|openapi|contract)
                value="$(prompt_required "Device Supplier API URL")"
                generate_device_openapi_tests "$value"
                return
                ;;
            b|back) return ;;
            q|quit|exit) exit 0 ;;
            *) warn "Invalid option: $choice" ;;
        esac
    done
}

handle_wfm_command() {
    local arg="${1:-}"
    case "${arg,,}" in
        ""|openapi|contract)
            generate_wfm_openapi_tests "${2:-}"
            ;;
        functional|margo|template)
            generate_wfm_functional_tests "${2:-}"
            ;;
        *)
            generate_wfm_openapi_tests "$arg"
            ;;
    esac
}

handle_device_command() {
    local arg="${1:-}"
    local expanded_arg
    expanded_arg="$(expand_path "$arg")"

    if [[ -n "$arg" && -f "$expanded_arg" ]]; then
        generate_device_scenario_tests "$expanded_arg"
        return
    fi

    case "${arg,,}" in
        ""|scenarios|existing)
            generate_device_scenario_tests "${2:-}"
            ;;
        margo|functional|template)
            generate_device_scenario_tests "$(default_device_scenarios_path)"
            ;;
        openapi|contract)
            generate_device_openapi_tests "${2:-}"
            ;;
        *)
            die "Unknown device option: $arg"
            ;;
    esac
}

################################################################################
# Test Group Management Functions
################################################################################

create_test_group() {
    mkdir -p "$GROUP_DIR"

    #Group name
    if [[ -z "${GROUP_NAME:-}" ]]; then
        echo ""
        read -p "Enter group name: " GROUP_NAME
        [[ -z "$GROUP_NAME" ]] && die "Group name cannot be empty"
    fi

    # Version
    echo ""
    read -p "Enter version of Margo Specification: " VERSION
    [[ -z "$VERSION" ]] && VERSION="1.0.0"

    # Description
    echo ""
    read -p "Enter a short description for group: " DESCRIPTION
    [[ -z "$DESCRIPTION" ]] && DESCRIPTION="User created group"

    GROUP_PATH="$GROUP_DIR/$GROUP_NAME"

    #Append mode
    APPEND_MODE=false
    if [[ -d "$GROUP_PATH" ]]; then
        echo ""
        echo "Using existing group: $GROUP_NAME"
        APPEND_MODE=true
    else
        mkdir -p "$GROUP_PATH"
    fi

    #Folder path instead of single file
    echo ""
    read -p "Enter folder path containing JSON files: " INPUT_PATH

    if [[ ! -d "$INPUT_PATH" ]]; then
        die "Provided path is not a folder!"
    fi

    log "Reading all JSON files from folder..."

    ALL_TESTS=()

    #Loop all JSON files
    for file in "$INPUT_PATH"/*.json; do
        [[ -e "$file" ]] || continue

        log "Processing: $(basename "$file")"

        #Copy file to group folder
        cp "$file" "$GROUP_PATH/"

        #Extract IDs
        mapfile -t IDS < <(jq -r '.. | .id? // empty' "$file")

        #fallback to names
        if [[ ${#IDS[@]} -eq 0 ]]; then
            mapfile -t IDS < <(
                jq -r '.. | .name? // empty' "$file" \
                | sed 's/ /_/g' \
                | tr '[:upper:]' '[:lower:]'
            )
        fi

        ALL_TESTS+=("${IDS[@]}")
    done

    if [[ ${#ALL_TESTS[@]} -eq 0 ]]; then
        die "No test cases found in folder!"
    fi

    log "Total extracted test cases: ${#ALL_TESTS[@]}"
    log "Reading JSON files, extracting test case IDs, and adding them to the group.json"

    # Convert to JSON
    TESTS_JSON=$(printf '%s\n' "${ALL_TESTS[@]}" | jq -R . | jq -s 'unique')

    # Merge logic
    if [[ "$APPEND_MODE" == true && -f "$GROUP_PATH/group.json" ]]; then

        OLD_TESTS=$(jq '.testCases' "$GROUP_PATH/group.json")

        jq -n \
            --arg name "$GROUP_NAME" \
            --arg version "$VERSION" \
            --arg persona "WfmSupplier" \
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

    else
        jq -n \
            --arg name "$GROUP_NAME" \
            --arg version "$VERSION" \
            --arg persona "WfmSupplier" \
            --arg desc "$DESCRIPTION" \
            --argjson tests "$TESTS_JSON" \
            '{
                name: $name,
                version: $version,
                persona: $persona,
                description: $desc,
                testCases: $tests
            }' > "$GROUP_PATH/group.json"
    fi
    info "Target Group Folder: $GROUP_PATH"
}

group_management_menu() {
    while true; do
        echo ""

        echo "Enter a number to select an existing group from the list below, or press 0 to create a new group "
        echo ""

        echo "Available Groups"
        echo "--------------------------------------"

        mkdir -p "$GROUP_DIR"
        mapfile -t groups < <(ls "$GROUP_DIR")

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
        if [[ "${choice,,}" == "b" ]]; then
            return
        fi

        # QUIT
        if [[ "${choice,,}" == "q" ]]; then
            info "Exiting..."
            exit 0
        fi

        #  CREATE NEW (0)
        if [[ "$choice" == "0" ]]; then
            unset GROUP_NAME
            create_test_group

        # EXISTING GROUP
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
            warn "Invalid input. Please enter a valid option."
            continue
        fi

        #  POST MENU (same as yours)
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

delete_test_group() {
    mkdir -p "$GROUP_DIR"

    echo ""
    echo "🗑️  Select group to delete:"
    echo "--------------------------"

    # FIXED: use find instead of ls
    mapfile -t GROUPS < <(find "$GROUP_DIR" -mindepth 1 -maxdepth 1 -type d)

    if [[ ${#GROUPS[@]} -eq 0 ]]; then
        warn "No groups found"
        return
    fi

    # Show list
    for i in "${!GROUPS[@]}"; do
        echo "$((i+1)). $(basename "${GROUPS[$i]}")"
    done

    echo ""
    read -p "Enter number to delete (or B to go back): " choice

    if [[ "${choice,,}" == "b" ]]; then
        return
    fi

    if ! [[ "$choice" =~ ^[0-9]+$ ]]; then
        die "Invalid input!"
    fi

    idx=$((choice-1))

    if [[ $idx -lt 0 || $idx -ge ${#GROUPS[@]} ]]; then
        die "Invalid selection!"
    fi

    GROUP_NAME=$(basename "${GROUPS[$idx]}")

    read -p "Are you sure you want to delete '$GROUP_NAME'? (y/n): " confirm

    if [[ "${confirm,,}" != "y" ]]; then
        info "Delete cancelled"
        return
    fi

    rm -rf "${GROUPS[$idx]}"

    success " Group '$GROUP_NAME' deleted successfully"
}

main() {
    if [[ $# -eq 0 ]]; then
        interactive_mode
        return
    fi

    local command="${1,,}"
    shift || true

    case "$command" in
        wfm) handle_wfm_command "${1:-}" "${2:-}" ;;
        device) handle_device_command "${1:-}" "${2:-}" ;;
        help|-h|--help) usage ;;
        *) die "Unknown command: $command. Run 'bash conformance.sh help' for usage." ;;
    esac
}

main "$@"
