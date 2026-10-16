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

# ----------------------------
# Validate EXPOSED_MIS_HOST format
# Must be: mis.<trustdomain>
# where <trustdomain> is a valid DNS authority name
# e.g., mis.margo.org, mis.example.com
# ----------------------------
validate_mis_host() {
  local host="${EXPOSED_MIS_HOST:-}"

  # Check if variable is set and non-empty
  if [[ -z "$host" ]]; then
    echo "[ERROR] EXPOSED_MIS_HOST is not set or empty in mis.env"
    echo "[ERROR] Expected format: mis.<trustdomain>  (e.g., mis.margo.org)"
    exit 1
  fi

  # Check prefix is exactly "mis."
  if [[ "$host" != mis.* ]]; then
    echo "[ERROR] EXPOSED_MIS_HOST must start with 'mis.' — got: '$host'"
    echo "[ERROR] Expected format: mis.<trustdomain>  (e.g., mis.margo.org)"
    exit 1
  fi

  # Extract trust domain (everything after "mis.")
  local trust_domain="${host#mis.}"

  # Trust domain must not be empty
  if [[ -z "$trust_domain" ]]; then
    echo "[ERROR] EXPOSED_MIS_HOST has no trust domain after 'mis.' — got: '$host'"
    exit 1
  fi

  # Validate trust domain is DNS-compatible:
  #   - Only alphanumeric, hyphens, dots
  #   - Each label: starts/ends with alphanumeric, max 63 chars
  #   - No consecutive dots, no leading/trailing dots
  #   - Total length <= 253 chars
  if [[ ${#trust_domain} -gt 253 ]]; then
    echo "[ERROR] Trust domain exceeds 253 characters: '$trust_domain'"
    exit 1
  fi

  # Check overall character set
  if [[ ! "$trust_domain" =~ ^[a-zA-Z0-9]([a-zA-Z0-9.\-]*[a-zA-Z0-9])?$ ]]; then
    echo "[ERROR] Trust domain '$trust_domain' contains invalid characters."
    echo "[ERROR] Only alphanumeric characters, hyphens, and dots are allowed."
    exit 1
  fi

  # Check each DNS label individually
  IFS='.' read -ra labels <<< "$trust_domain"
  if [[ ${#labels[@]} -lt 1 ]]; then
    echo "[ERROR] Trust domain must have at least one label: '$trust_domain'"
    exit 1
  fi

  for label in "${labels[@]}"; do
    if [[ -z "$label" ]]; then
      echo "[ERROR] Trust domain '$trust_domain' contains consecutive or trailing dots."
      exit 1
    fi
    if [[ ${#label} -gt 63 ]]; then
      echo "[ERROR] DNS label '$label' exceeds 63 characters in '$trust_domain'."
      exit 1
    fi
    if [[ ! "$label" =~ ^[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?$ && ! "$label" =~ ^[a-zA-Z0-9]$ ]]; then
      echo "[ERROR] DNS label '$label' is invalid — labels must start and end with alphanumeric characters."
      exit 1
    fi
  done

  echo "[INFO] EXPOSED_MIS_HOST validated: '$host' (trust domain: '$trust_domain')"
}