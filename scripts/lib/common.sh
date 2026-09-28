#!/bin/bash
# lib/common.sh - Shared utility functions

info() {
    echo "ℹ️  $1"
}

success() {
    echo "✅ $1"
}

pause() {
    echo
    read -rp "Press Enter to continue..." _
}

command_exists() {
    command -v "$1" >/dev/null 2>&1
}

package_installed() {
    dpkg -s "$1" >/dev/null 2>&1
}

get_ubuntu_codename() {
    lsb_release -cs 2>/dev/null || echo "noble"
}

validate_spiffe_json() {
    local json_file="$1"

    # ----------------------------------------------------------------
    # Argument check
    # ----------------------------------------------------------------
    if [[ -z "$json_file" ]]; then
        echo "[ERROR] No file path provided." >&2
        echo "Usage: validate_spiffe_json <path-to-json-file>" >&2
        return 1
    fi

    # ----------------------------------------------------------------
    # File existence check
    # ----------------------------------------------------------------
    info "Checking if file exists: $json_file"
    if [[ ! -f "$json_file" ]]; then
        echo "[ERROR] File not found: $json_file" >&2
        return 1
    fi
    success "Local Authorization Policy File exists."

    # ----------------------------------------------------------------
    # Valid JSON check
    # ----------------------------------------------------------------
    info "Validating JSON syntax..."
    if ! jq empty "$json_file" 2>/dev/null; then
        echo "[ERROR] File is not valid JSON: $json_file" >&2
        return 1
    fi
    success "Local Authorization Policy JSON syntax is valid."

    # ----------------------------------------------------------------
    # Root type must be an array
    # ----------------------------------------------------------------
    info "Checking that root element is an array..."
    local root_type
    root_type=$(jq -r 'type' "$json_file")
    if [[ "$root_type" != "array" ]]; then
        echo "[ERROR] Expected a JSON array at root, but got: $root_type" >&2
        return 1
    fi
    success "Root element is an array in Local Authorization Policy."

    # ----------------------------------------------------------------
    # Array must not be empty
    # ----------------------------------------------------------------
    info "Checking that the array is not empty..."
    local count
    count=$(jq 'length' "$json_file")
    if [[ "$count" -eq 0 ]]; then
        echo "[ERROR] Array is empty. At least one SPIFFE ID is required." >&2
        return 1
    fi
    success "Local Authorization Policy Array contains $count element(s)."

    # ----------------------------------------------------------------
    # Every string must be a valid SPIFFE ID (starts with spiffe://)
    # ----------------------------------------------------------------
    info "Validating SPIFFE ID format for each entry..."
    local invalid_ids
    invalid_ids=$(jq -r '.[] | select(startswith("spiffe://") | not)' "$json_file")
    if [[ -n "$invalid_ids" ]]; then
        echo "[ERROR] The following entries are not valid SPIFFE IDs (must start with 'spiffe://'):" >&2
        echo "$invalid_ids" | while IFS= read -r id; do
            echo "  - $id" >&2
        done
        return 1
    fi
    success "All entries are valid SPIFFE IDs in Local Authorization Policy."

    # ----------------------------------------------------------------
    # Success — print entries in pretty JSON format
    # ----------------------------------------------------------------
    echo ""
    success "All checks passed. Entries in '$json_file':"
    jq '.' "$json_file"
}