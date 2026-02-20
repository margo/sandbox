#!/bin/bash
set -e


# ----------------------------
# Load environment file
# ----------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
load_wfm_env() {
  local env_file="$SCRIPT_DIR/wfm.env"

  if [[ ! -f "$env_file" ]]; then
    echo "[WARN] wfm.env not found at: $env_file"
    return 1
  fi

  echo "[INFO] Loading environment from: $env_file"
  set -a
  source "$env_file"
  set +a

}

load_wfm_env || true

# ----------------------------
# Environment & Validation
# ----------------------------

#--- harbor settings (can be overridden via env)
EXPOSED_HARBOR_HOST="${EXPOSED_HARBOR_HOST:-127.0.0.1}"
EXPOSED_HARBOR_PORT="${EXPOSED_HARBOR_PORT:-8081}"

#--- symphony settings (can be overridden via env)
EXPOSED_SYMPHONY_HOST="${EXPOSED_SYMPHONY_HOST:-127.0.0.1}"
EXPOSED_SYMPHONY_PORT="${EXPOSED_SYMPHONY_PORT:-8082}"


#--- OCI Registry settings (can be overridden via env)
REGISTRY_URL="${REGISTRY_URL:-http://${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}}"
REGISTRY_USER="${REGISTRY_USER:-admin}"
REGISTRY_PASS="${REGISTRY_PASS:-Harbor12345}"
OCI_ORGANIZATION="${OCI_ORGANIZATION:-library}"


# ----------------------------
# Utility Functions
# ----------------------------
MAESTRO_CLI_PATH="$HOME/symphony/cli"

install_basic_utilities() {
  apt install jq -y
}

check_maestro_cli() {
  if [ ! -f "${MAESTRO_CLI_PATH}/maestro" ]; then
    echo "❌ maestro CLI not found in ${MAESTRO_CLI_PATH} directory"
    echo "Please ensure maestro CLI is built and available there"
    return 1
  fi
  return 0
}

validate_choice() {
  local choice="$1"
  local max_choice="$2"
  if [[ ! "$choice" =~ ^[0-9]+$ ]] || [ "$choice" -lt 1 ] || [ "$choice" -gt "$max_choice" ]; then
    echo "❌ Invalid choice. Please enter a number between 1 and $max_choice"
    return 1
  fi
  return 0
}

# ----------------------------
# List Functions
# ----------------------------
list_app_packages() {
  echo "📦 Listing all app packages from WFM..."
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm list app-pkg || echo "❌ Failed to list app-pkg"
  fi
  echo ""
  read -p "Press Enter to continue..."
}

list_devices() {
  echo "🖥️  Listing all devices from WFM..."
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm list devices || echo "❌ Failed to list devices"
  fi
  echo ""
  read -p "Press Enter to continue..."
}

list_deployments() {
  echo "🚀 Listing all deployments from WFM..."
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm list deployment || echo "❌ Failed to list deployment"
  fi
  echo ""
  read -p "Press Enter to continue..."
}

list_all() {
  echo "📋 Listing all resources from WFM..."
  echo "=================================="

  echo "📦 App Packages:"
  echo "----------------"
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm list app-pkg || echo "❌ Failed to list app-pkg"
  fi

  echo ""
  echo "🖥️  Devices:"
  echo "----------"
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm list devices || echo "❌ Failed to list devices"
  fi

  echo ""
  echo "🚀 Deployments:"
  echo "---------------"
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm list deployment || echo "❌ Failed to list deployment"
  fi

  echo ""
  read -p "Press Enter to continue..."
}

# ----------------------------
# Dynamic Instance File Generation
# ----------------------------
generate_instance_yaml_from_oci() {
  local package_name="$1"
  local package_id="$2"
  local device_id="$3"
  local output_file="$4"

  local harbor_url="${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}"

  # Pull margo.yaml from OCI to extract metadata
  local temp_dir=$(mktemp -d)
  cd "$temp_dir"

  # Pull entire package (suppress output)
  if ! oras pull "${harbor_url}/${OCI_ORGANIZATION}/${package_name}:latest" \
      --plain-http \
      -u "${REGISTRY_USER}:${REGISTRY_PASS}" >/dev/null 2>&1; then
    echo "❌ Failed to pull package from OCI" >&2
    cd - >/dev/null
    rm -rf "$temp_dir"
    return 1
  fi

  if [ ! -f "margo.yaml" ]; then
    echo "❌ margo.yaml not found in package" >&2
    cd - >/dev/null
    rm -rf "$temp_dir"
    return 1
  fi

  # Extract BOTH id and name from margo.yaml
  local app_id=$(grep -E "^\s*id:" margo.yaml | head -1 | sed 's/.*id:\s*//' | tr -d '"' | tr -d "'" | xargs)
  local app_name=$(grep -E "^\s*name:" margo.yaml | head -1 | sed 's/.*name:\s*//' | tr -d '"' | tr -d "'" | xargs)

  # Prefer id over name for identifiers (id is already properly formatted per Margo spec)
  local app_identifier=""
  if [ -n "$app_id" ]; then
    # Use id directly (already lowercase, numbers, dashes only)
    app_identifier="$app_id"
  else
    # Fallback: sanitize name if id doesn't exist
    app_identifier=$(echo "${app_name}" | tr '[:upper:]' '[:lower:]' | tr ' ' '-' | tr -d '_.,' | sed 's/--*/-/g' | sed 's/^-//;s/-$//')
  fi

  # Truncate to safe length (max 40 chars to leave room for suffixes)
  app_identifier=$(echo "$app_identifier" | cut -c1-40)

  # Try multiple patterns to find deployment type in nested structure
  local deployment_type=""

  # Pattern 1: Look for "type:" under deploymentProfile section
  deployment_type=$(awk '/deploymentProfile:/,/^[^ ]/ {if (/^\s+type:/) print}' margo.yaml | sed 's/.*type:\s*//' | tr -d '"' | tr -d "'" | xargs | head -1)

  # Pattern 2: If not found, try looking for spec.type
  if [ -z "$deployment_type" ]; then
    deployment_type=$(awk '/^spec:/,/^[^ ]/ {if (/^\s+type:/) print}' margo.yaml | sed 's/.*type:\s*//' | tr -d '"' | tr -d "'" | xargs | head -1)
  fi

  # Pattern 3: Check if package name contains hints
  if [ -z "$deployment_type" ]; then
    if [[ "$package_name" =~ compose ]]; then
      deployment_type="compose"
    elif [[ "$package_name" =~ helm ]]; then
      deployment_type="helm.v3"
    fi
  fi

  # Determine deployment profile type
  local profile_type=""
  case "$deployment_type" in
    helm|helm.v3)
      profile_type="helm.v3"
      ;;
    compose|docker-compose)
      profile_type="compose"
      ;;
    *)
      # Infer from package name as last resort
      if [[ "$package_name" =~ compose ]]; then
        profile_type="compose"
      else
        profile_type="helm.v3"
      fi
      ;;
  esac

  # Get repository path
  local repository=$(get_oci_repository_path "$package_name" "$temp_dir/margo.yaml")

  # Generate instance.yaml based on deployment type
  if [ "$profile_type" = "helm.v3" ]; then
    generate_helm_instance "$app_identifier" "$package_id" "$device_id" "$repository" "$output_file" "$temp_dir/margo.yaml"
  elif [ "$profile_type" = "compose" ]; then
    generate_compose_instance "$app_identifier" "$package_id" "$device_id" "$repository" "$output_file" "$temp_dir/margo.yaml"
  else
    echo "❌ Unsupported deployment type: $profile_type" >&2
    cd - >/dev/null
    rm -rf "$temp_dir"
    return 1
  fi

  cd - >/dev/null
  rm -rf "$temp_dir"

  return 0
}


# Generate Helm-based instance.yaml
generate_helm_instance() {
  local app_identifier="$1"  # Receives id (pre-formatted) or sanitized name
  local package_id="$2"
  local device_id="$3"
  local repository="$4"
  local output_file="$5"
  local margo_file="$6"

  # Extract Helm-specific metadata
  local chart_version=$(grep -E "^\s*version:" "$margo_file" | head -1 | sed 's/.*version:\s*//' | tr -d '"' | tr -d "'" | xargs)
  chart_version="${chart_version:-0.1.0}"

  # Create instance name from identifier (already properly formatted)
  # Truncate to 53 chars for Helm release name limit
  local instance_name=$(echo "${app_identifier}-instance" | cut -c1-53)

  # Component name: truncate to 40 chars to leave room for UUID suffix
  local component_name=$(echo "$app_identifier" | cut -c1-40)

  cat > "$output_file" <<EOF
# This is an input template allowing the WFM user to modify deployment instance specific parameters(currently read-only).
# This file is not MARGO specified, however these parameters will be used to create the MARGO ApplicationDeployment

apiVersion: non-margo.org
kind: ApplicationDeployment
metadata:
  name: ${instance_name}
spec:
  appPackageRef:
    id: ${package_id}
  deviceRef:
    id: ${device_id}
  deploymentProfile:
    type: helm.v3
    components:
      - name: ${component_name}
        properties:
          repository: ${repository}
          revision: ${chart_version}
          wait: true
          timeout: 5m
EOF

  # Add parameters if they exist in margo.yaml
  if grep -q "parameters:" "$margo_file"; then
    echo "  parameters:" >> "$output_file"

    # Extract OTEL endpoint if present (common pattern)
    if grep -qi "otel\|otlp" "$margo_file"; then
      cat >> "$output_file" <<EOF
    otlpEndpoint:
      value: "http://otel-collector-opentelemetry-collector.observability:4318"
      targets:
      - pointer: env.OTEL_EXPORTER_OTLP_ENDPOINT
        components: ["${component_name}"]
EOF
    fi
  fi
}


# Generate Compose-based instance.yaml
generate_compose_instance() {
  local app_identifier="$1"  # Receives id (pre-formatted) or sanitized name
  local package_id="$2"
  local device_id="$3"
  local repository="$4"
  local output_file="$5"
  local margo_file="$6"

  # Create names from identifier (already properly formatted)
  local instance_name=$(echo "${app_identifier}-instance" | cut -c1-53)
  local stack_name=$(echo "${app_identifier}-stack" | cut -c1-40)

  cat > "$output_file" <<EOF
# This is an input template allowing the WFM user to modify deployment instance specific parameters(currently read-only).
# This file is not MARGO specified, however these parameters will be used to create the MARGO ApplicationDeployment
apiVersion: non-margo.org
kind: ApplicationDeployment
metadata:
  name: ${instance_name}
spec:
  appPackageRef:
    id: ${package_id}
  deviceRef:
    id: ${device_id}
  deploymentProfile:
    type: compose
    components:
      - name: ${stack_name}
        properties:
          packageLocation: ${repository}
EOF

  # Add common Compose parameters
  if grep -q "parameters:" "$margo_file"; then
    echo "  parameters:" >> "$output_file"

    # Add port mapping if specified
    local default_port=$(grep -E "^\s*port:" "$margo_file" | head -1 | sed 's/.*port:\s*//' | tr -d '"' | tr -d "'" | xargs)
    if [ -n "$default_port" ]; then
      cat >> "$output_file" <<EOF
    servicePort:
      value: ${default_port}
      targets:
        - pointer: PORTS.80
          components: ["${stack_name}"]
EOF
    fi
  fi
}




# ----------------------------
# Harbor OCI Discovery Functions
# ----------------------------
discover_app_packages_from_harbor() {
  # Send discovery message to stderr so it doesn't get captured
  echo "🔍 Discovering app packages from Harbor OCI Registry..." >&2

  local harbor_url="${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}"
  local org="${OCI_ORGANIZATION}"

  # Get list of repositories from Harbor API
  local repos=$(curl -s -u "${REGISTRY_USER}:${REGISTRY_PASS}" \
    "http://${harbor_url}/api/v2.0/projects/${org}/repositories" | \
    jq -r '.[].name' 2>/dev/null)

  if [ -z "$repos" ]; then
    echo "❌ No repositories found in Harbor" >&2
    return 1
  fi

  # Filter for app-package repositories (must end with -app-package)
  local app_packages=$(echo "$repos" | grep -E "app-package$" | sed "s|${org}/||")

  if [ -z "$app_packages" ]; then
    echo "❌ No app packages found" >&2
    echo "ℹ️  App packages must end with '-app-package' suffix" >&2
    echo "ℹ️  Example: nginx-helm-app-package, wordpress-compose-app-package" >&2
    return 1
  fi

  # Output package names to stdout
  echo "$app_packages"
}




get_package_metadata_from_oci() {
  local package_repo="$1"
  local harbor_url="${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}"
  local full_repo="${OCI_ORGANIZATION}/${package_repo}"

  # Pull margo.yaml from OCI to get metadata
  local temp_dir=$(mktemp -d)
  cd "$temp_dir"

  oras pull "${harbor_url}/${full_repo}:latest" \
    --plain-http \
    -u "${REGISTRY_USER}:${REGISTRY_PASS}" \
    margo.yaml 2>/dev/null

  if [ -f "margo.yaml" ]; then
    # Extract display name from margo.yaml
    local display_name=$(grep -E "^\s*name:" margo.yaml | head -1 | sed 's/.*name:\s*//' | tr -d '"')
    echo "${display_name:-${package_repo}}"
  else
    echo "${package_repo}"
  fi

  cd - >/dev/null
  rm -rf "$temp_dir"
}


upload_app_package() {
  echo "📦 Upload App Package"
  echo "===================="

  # Discover packages from Harbor
  local packages=$(discover_app_packages_from_harbor)

  if [ -z "$packages" ]; then
    echo "❌ No app packages available"
    read -p "Press Enter to continue..."
    return 1
  fi

  # Build menu dynamically
  echo "Select one of the packages:"
  local -a package_array
  local index=1

  while IFS= read -r pkg; do
    package_array+=("$pkg")
    local display_name=$(get_package_metadata_from_oci "$pkg")
    echo "$index) $display_name"
    ((index++))
  done <<< "$packages"

  echo "$index) Exit"
  echo ""

  read -p "Enter choice [1-$index]: " app_package_choice

  if [ "$app_package_choice" = "$index" ]; then
    echo "Returning to main menu..."
    return 0
  fi

  local max_choice=$((index - 1))
  if ! validate_choice "$app_package_choice" "$max_choice"; then
    return 1
  fi

  # Get selected package
  local selected_pkg="${package_array[$((app_package_choice - 1))]}"
  local package_name=$(get_package_metadata_from_oci "$selected_pkg")

  # Generate package.yaml for WFM
  local temp_pkg_file=$(mktemp)
  generate_wfm_package_yaml "$selected_pkg" "$temp_pkg_file"

  if [ ! -f "$temp_pkg_file" ]; then
    echo "❌ Failed to generate package file"
    return 1
  fi

  echo "📤 Uploading $package_name to WFM..."
  if check_maestro_cli; then
    if ${MAESTRO_CLI_PATH}/maestro wfm apply -f "$temp_pkg_file"; then
      echo "✅ $package_name uploaded successfully!"
    else
      echo "❌ Failed to upload $package_name"
    fi
  fi

  rm -f "$temp_pkg_file"
  echo ""
  read -p "Press Enter to continue..."
}

generate_wfm_package_yaml() {
  local package_repo="$1"
  local output_file="$2"
  local harbor_url="${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}"

  cat > "$output_file" <<EOF
# This is an input template allowing the WFM user to modify deployment instance specific parameters.
# This file is not MARGO specified, however these parameters will be used to create the MARGO ApplicationDeployment
apiVersion: non-margo.org
kind: ApplicationPackage
metadata:
  name: ${package_repo}
  labels:
    env: dev
  annotations:
    description: "Application package from Harbor OCI Registry"
spec:
  sourceType: OCI_REPO
  source:
    registryUrl: "http://${harbor_url}"
    repository: "${OCI_ORGANIZATION}/${package_repo}"
    tag: "latest"
    authentication:
      type: "basic"
      username: "${REGISTRY_USER}"
      password: "${REGISTRY_PASS}"
EOF
}

# Keep these for backward compatibility with existing deployments
get_package_name() {
  local choice="$1"
  case $choice in
    1) echo "Custom OTEL Helm App" ;;
    2) echo "Nextcloud Compose App" ;;
    *)
      # Fallback to dynamic discovery
      local packages=$(discover_app_packages_from_harbor)
      local package_array=($packages)
      local idx=$((choice - 1))
      if [ $idx -lt ${#package_array[@]} ]; then
        get_package_metadata_from_oci "${package_array[$idx]}"
      else
        echo "Unknown Package"
      fi
      ;;
  esac
}




delete_app_package() {
  echo "🗑️  Delete App Package"
  echo "===================="

  echo "📦 Current packages:"
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm list app-pkg
  fi

  echo ""
  read -p "Enter the package name/ID to delete: " package_id

  if [ -z "$package_id" ]; then
    echo "❌ Package name/ID is required"
    return 1
  fi

  read -p "Are you sure you want to delete app-pkg '$package_id'? (y/N): " confirm
  if [[ "$confirm" =~ ^[Yy]$ ]]; then
    echo "🗑️  Deleting package '$package_id'..."
    if check_maestro_cli; then
      if ${MAESTRO_CLI_PATH}/maestro wfm delete app-pkg "$package_id"; then
        echo "✅ Package '$package_id' deleted successfully!"
      else
        echo "❌ Failed to delete app-pkg '$package_id'"
      fi
    fi
  else
    echo "Deletion cancelled"
  fi

  echo ""
  read -p "Press Enter to continue..."
}

# ----------------------------
# Instance Management Functions (ENHANCED)
# ----------------------------

# Dynamic instance file path discovery
get_instance_file_path() {
  local package_name="$1"
  local file_path=""

  # Validate HOME directory
  if [ -z "$HOME" ]; then
    echo "❌ HOME environment variable not set" >&2
    return 1
  fi

  # Try to find matching template directory dynamically
  local template_base="$HOME/symphony/cli/templates/margo"

  # First try exact match (backward compatibility)
  case $package_name in
    "custom-otel-helm-app-package"|"custom-otel-helm-app"|"custom-otel"|"otel-demo-pkg")
      original_file_path="$template_base/custom-otel-helm/instance.yaml"
      file_path="$template_base/custom-otel-helm/instance.yaml.copy"
      ;;
    "nextcloud-compose-app-package"|"nextcloud-compose-app"|"nextcloud"|"nextcloud-pkg")
      original_file_path="$template_base/nextcloud-compose/instance.yaml"
      file_path="$template_base/nextcloud-compose/instance.yaml.copy"
      ;;
    *)
      # Dynamic discovery: search for matching directory
      # Remove "-app-package" suffix if present for matching
      local search_name="${package_name%-app-package}"

      # Search for directory containing the package name (case-insensitive)
      local template_dir=$(find "$template_base" -maxdepth 1 -type d -iname "*${search_name}*" 2>/dev/null | head -1)

      if [ -n "$template_dir" ] && [ -f "$template_dir/instance.yaml" ]; then
        original_file_path="$template_dir/instance.yaml"
        file_path="$template_dir/instance.yaml.copy"
      else
        echo "❌ No instance template found for package '$package_name'" >&2
        echo "ℹ️  Searched in: $template_base" >&2
        return 1
      fi
      ;;
  esac

  # Copy and verify file exists
  if [ -f "$original_file_path" ]; then
    cp -f "$original_file_path" "$file_path"
    echo "$file_path"
  else
    echo "❌ Deployment file not found: $original_file_path" >&2
    return 1
  fi
}


# Dynamic OCI repository path discovery
get_oci_repository_path() {
  local package_name="$1"
  local margo_file="$2"  # Accept margo.yaml file path
  local harbor_url="${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}"
  local container_url=""

  # First try hardcoded mappings (backward compatibility)
  case $package_name in
    "custom-otel-helm-app-package"|"custom-otel-helm-app"|"custom-otel"|"otel-demo-pkg")
      container_url="oci://${harbor_url}/library/custom-otel-helm"
      ;;
    "nextcloud-compose-app-package"|"nextcloud-compose-app"|"nextcloud"|"nextcloud-pkg")
      container_url="https://raw.githubusercontent.com/docker/awesome-compose/refs/heads/master/nextcloud-redis-mariadb/compose.yaml"
      ;;
    *)
      # Use the already-pulled margo.yaml file
      if [ -f "$margo_file" ]; then
        # Simple grep-based extraction for packageLocation (Compose)
        local compose_location=$(grep "packageLocation:" "$margo_file" | \
                                head -1 | \
                                sed 's/.*packageLocation:\s*//' | \
                                tr -d '"' | tr -d "'" | xargs)

        # Simple grep-based extraction for repository (Helm)
        local helm_repo=$(grep "repository:" "$margo_file" | \
                          grep -v "registryUrl" | \
                          head -1 | \
                          sed 's/.*repository:\s*//' | \
                          tr -d '"' | tr -d "'" | xargs)

        # Use whichever is found
        if [ -n "$compose_location" ]; then
          container_url="$compose_location"
        elif [ -n "$helm_repo" ]; then
          container_url="$helm_repo"
        else
          # Fallback: construct OCI path from package name
          local chart_name="${package_name%-app-package}"
          container_url="oci://${harbor_url}/library/${chart_name}"
        fi
      else
        # If file doesn't exist, construct default OCI path
        local chart_name="${package_name%-app-package}"
        container_url="oci://${harbor_url}/library/${chart_name}"
      fi
      ;;
  esac

  echo "$container_url"
}






deploy_instance() {
  echo "🚀 Deploy Instance"
  echo "=================="

  echo "📦 Available packages:"
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm list app-pkg
  fi

  echo ""
  read -p "Enter the package name/ID to deploy: " package_id

  if [ -z "$package_id" ]; then
    echo "❌ Package name/ID is required"
    return 1
  fi

  echo ""
  echo "🖥️  Available devices:"
  ${MAESTRO_CLI_PATH}/maestro wfm list devices

  echo ""
  read -p "Enter the device ID for deployment: " device_id

  if [ -z "$device_id" ]; then
    echo "❌ Device ID is required"
    return 1
  fi

  # Get app package details and extract metadata.name
  # echo "📋 Getting package details..."
  app_packages=$(${MAESTRO_CLI_PATH}/maestro wfm list app-pkg -o json 2>/dev/null)

  if [ $? -ne 0 ] || [ -z "$app_packages" ]; then
    echo "❌ Failed to get package list"
    return 1
  fi

  # Parse JSON to find the package and extract metadata.name
  if command -v jq >/dev/null 2>&1; then
    # echo "🔍 Searching for package: $package_id"

    package_name=$(echo "$app_packages" | jq -r --arg pkg_id "$package_id" '
      .Data[0].items[] |
      select(.metadata.id == $pkg_id or .metadata.name == $pkg_id) |
      .metadata.name
    ')

    if [ -z "$package_name" ] || [ "$package_name" = "null" ]; then
      echo "❌ Package '$package_id' not found in the package list"
      echo "Available packages:"
      echo "$app_packages" | jq -r '.Data[0].items[] | "  - Name: \(.metadata.name), ID: \(.metadata.id)"'
      return 1
    fi
  else
    echo "❌ jq command is required but not installed. Please install it and retry."
    return 1
  fi

  # echo "📦 Package name: $package_name"

  # Generate instance.yaml dynamically from OCI metadata
  # echo "🔧 Generating instance deployment configuration..."
  local temp_instance_file=$(mktemp --suffix=.yaml)

  if ! generate_instance_yaml_from_oci "$package_name" "$package_id" "$device_id" "$temp_instance_file" 2>/dev/null; then
    # echo "❌ Failed to generate instance configuration"
    # echo "ℹ️  Falling back to template-based approach..."

    # Fallback to template discovery
    deploy_file=$(get_instance_file_path "$package_name")

    if [ $? -ne 0 ] || [ -z "$deploy_file" ] || [ ! -f "$deploy_file" ]; then
      echo "❌ No template found and dynamic generation failed"
      return 1
    fi

    # Update template with values
    repository=$(get_oci_repository_path "$package_name")
    sed -i "s|{{DEVICE_ID}}|$device_id|g" "$deploy_file" 2>/dev/null || true
    sed -i "s|{{PACKAGE_ID}}|$package_id|g" "$deploy_file" 2>/dev/null || true
    sed -i "s|{{REPOSITORY}}|$repository|g" "$deploy_file" 2>/dev/null || true
  else
    deploy_file="$temp_instance_file"
    # echo "✅ Instance configuration generated successfully"
  fi

  # ============================================
  # SECURITY: Make file read-only and calculate checksum
  # ============================================
  chmod 444 "$deploy_file"  # Read-only for everyone
  local file_checksum=$(sha256sum "$deploy_file" | awk '{print $1}')

  # echo "📄 Using deployment file: $deploy_file"
  # echo "🔒 File protection: Read-only mode enabled"
  # echo "🔐 Integrity hash: ${file_checksum:0:16}..."

  # Show generated configuration for review
  #echo ""
  #echo "📋 Generated Instance Configuration:"
  #echo "===================================="
  #cat "$deploy_file"
  #echo "===================================="
  #echo ""

  # ============================================
  # SECURITY: Verify file integrity before deployment
  # ============================================
  local current_checksum=$(sha256sum "$deploy_file" | awk '{print $1}')
  if [ "$file_checksum" != "$current_checksum" ]; then
    echo "❌ SECURITY ALERT: Configuration file was modified!"
    echo "   Expected: ${file_checksum:0:16}..."
    echo "   Current:  ${current_checksum:0:16}..."
    rm -f "$temp_instance_file"
    return 1
  fi


  #read -p "Proceed with deployment? (y/N): " confirm
  #if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
  #  echo "Deployment cancelled"
  #  rm -f "$temp_instance_file"
  #  return 0
  #fi

  # ============================================
  # SECURITY: Final integrity check before deployment
  # ============================================
  current_checksum=$(sha256sum "$deploy_file" | awk '{print $1}')
  if [ "$file_checksum" != "$current_checksum" ]; then
    echo "❌ SECURITY ALERT: Configuration file was modified after confirmation!"
    echo "   Deployment aborted for security reasons."
    rm -f "$temp_instance_file"
    return 1
  fi

  echo ""
  echo "🚀 Deploying '$package_id' to device '$device_id'..."
  if check_maestro_cli; then
    if ${MAESTRO_CLI_PATH}/maestro wfm apply -f "$deploy_file"; then
      echo "✅ Instance deployment request sent successfully!"

      echo ""
      echo "📋 Updated deployments:"
      ${MAESTRO_CLI_PATH}/maestro wfm list deployment
    else
      echo "❌ Failed to deploy instance"
    fi
  fi

  # Cleanup temporary file
  rm -f "$temp_instance_file"

  echo ""
  read -p "Press Enter to continue..."
}



delete_instance() {
  echo "🗑️  Delete Instance"
  echo "=================="

  echo "🚀 Current deployments:"
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm list deployment
  fi

  echo ""
  read -p "Enter the deployment/instance ID to delete: " instance_id

  if [ -z "$instance_id" ]; then
    echo "❌ Instance ID is required"
    return 1
  fi

  read -p "Are you sure you want to delete instance '$instance_id'? (y/N): " confirm
  if [[ "$confirm" =~ ^[Yy]$ ]]; then
    echo "🗑️  Deleting instance '$instance_id'..."
    if check_maestro_cli; then
      if ${MAESTRO_CLI_PATH}/maestro wfm delete deployment "$instance_id"; then
        echo "✅ Instance '$instance_id' deleted successfully!"

        echo ""
        echo "📋 Updated deployments:"
        ${MAESTRO_CLI_PATH}/maestro wfm list deployment
      else
        echo "❌ Failed to delete instance '$instance_id'"
      fi
    fi
  else
    echo "Deletion cancelled"
  fi

  echo ""
  read -p "Press Enter to continue..."
}

# ----------------------------
# Menu Functions
# ----------------------------
show_menu() {
  clear
  load_wfm_env || true
  echo "🎛️  WFM CLI Interactive Interface(EasyCLI)"
  echo "================================="
  echo "Choose an option:"
  echo "1) 📦 List Application Package"
  echo "2) 🖥️ List Devices"
  echo "3) 🚀 List Deployment"
  echo "4) 📋 List All"
  echo "5) 📤 Upload Application Package"
  echo "6) 🗑️ Delete Application Package"
  echo "7) 🚀 Deploy Instance"
  echo "8) 🗑️ Delete Instance"
  echo "9) 🚪 Exit"
  echo ""

  read -p "Enter choice [1-9]: " choice
  case $choice in
    1) list_app_packages ;;
    2) list_devices ;;
    3) list_deployments ;;
    4) list_all ;;
    5) upload_app_package ;;
    6) delete_app_package ;;
    7) deploy_instance ;;
    8) delete_instance ;;
    9) echo "👋 Goodbye!"; exit 0 ;;
    *) echo "⚠️ Invalid choice"; sleep 2 ;;
  esac
}

# ----------------------------
# Main Script Execution
# ----------------------------
main_loop() {
  install_basic_utilities
  while true; do
    show_menu
  done
}

if [[ -z "$1" ]]; then
  main_loop
else
  load_wfm_env || true
  case "$1" in
    list-packages) list_app_packages ;;
    list-devices) list_devices ;;
    list-deployments) list_deployments ;;
    list-all) list_all ;;
    upload) upload_app_package ;;
    delete-package) delete_app_package ;;
    deploy) deploy_instance ;;
    delete-instance) delete_instance ;;
    *) echo "Usage: $0 {list-packages|list-devices|list-deployments|list-all|upload|delete-package|deploy|delete-instance}"; exit 1 ;;
  esac
fi
