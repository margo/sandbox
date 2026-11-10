#!/bin/bash
# Instance/deployment management for WFM CLI

list_devices() {
  echo "🖥️  Listing all devices from WFM..."
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list devices || echo "❌ Failed to list devices"
  fi
  echo ""
  read -p "Press Enter to continue..."
}

list_devices_non_interactive() {
  echo "🖥️  Listing all devices from WFM..."
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list devices || echo "❌ Failed to list devices"
  fi
  echo ""
}

list_deployments() {
  echo "🚀 Listing all deployments from WFM..."
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list deployment || echo "❌ Failed to list deployment"
  fi
  echo ""
  read -p "Press Enter to continue..."
}

list_deployments_non_interactive() {
  echo "🚀 Listing all deployments from WFM..."
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list deployment || echo "❌ Failed to list deployment"
  fi
  echo ""
}

list_all() {
  echo "📋 Listing all resources from WFM..."
  echo "=================================="
  
  echo "📦 App Packages:"
  echo "----------------"
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list app-pkg || echo "❌ Failed to list app-pkg"
  fi
  
  echo ""
  echo "🖥️  Devices:"
  echo "----------"
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list devices || echo "❌ Failed to list devices"
  fi
  
  echo ""
  echo "🚀 Deployments:"
  echo "---------------"
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list deployment || echo "❌ Failed to list deployment"
  fi
  
  echo ""
  read -p "Press Enter to continue..."
}

list_all_non_interactive() {
  echo "📋 Listing all resources from WFM..."
  echo "=================================="
  
  echo "📦 App Packages:"
  echo "----------------"
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list app-pkg || echo "❌ Failed to list app-pkg"
  fi
  
  echo ""
  echo "🖥️  Devices:"
  echo "----------"
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list devices || echo "❌ Failed to list devices"
  fi
  
  echo ""
  echo "🚀 Deployments:"
  echo "---------------"
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list deployment || echo "❌ Failed to list deployment"
  fi
  
  echo ""
  
}

# Resolve supported deployment profile types for a device using supportedDeploymentTypes.
# Returns a comma-separated list such as: compose,helm
# Fails if supportedDeploymentTypes are missing or do not map to known deployment profile types.
get_supported_deployments_for_device() {
  local device_id="$1"

  if [ -z "$device_id" ]; then
    echo "❌ Error: Device ID is required" >&2
    return 1
  fi

  if ! check_maestro_cli; then
    echo "❌ Maestro CLI not available" >&2
    return 1
  fi

  local devices=$(${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list devices -o json 2>/dev/null)
  if [ $? -ne 0 ] || [ -z "$devices" ]; then
    echo "❌ Failed to get device list" >&2
    return 1
  fi

  if ! command -v jq >/dev/null 2>&1; then
    echo "❌ jq is required but not installed" >&2
    return 1
  fi

  local supported_types
  supported_types=$(echo "$devices" | jq -r --arg id "$device_id" '
    .Data[] | .items[] |
    select(.id == $id) |
    (.spec.capabilities.properties.supportedDeploymentTypes[]?)
  ')

  if [ -z "$supported_types" ]; then
    echo "❌ Device '$device_id' has no supportedDeploymentTypes in capabilities" >&2
    return 1
  fi

  local normalized=()
  local unknown_types=()

  while IFS= read -r value; do
    [ -z "$value" ] && continue
    case "${value,,}" in
      compose)
        normalized+=("compose")
        ;;
      helm)
        normalized+=("helm")
        ;;
      *)
        unknown_types+=("$value")
        ;;
    esac
  done <<< "$supported_types"

  if [ ${#normalized[@]} -eq 0 ]; then
    if [ ${#unknown_types[@]} -gt 0 ]; then
      echo "❌ Device '$device_id' has no usable supportedDeploymentTypes (unsupported: ${unknown_types[*]})" >&2
    else
      echo "❌ Device '$device_id' has no usable supportedDeploymentTypes in capabilities" >&2
    fi
    return 1
  fi

  local out=""
  for type in "${normalized[@]}"; do
    if [[ ",$out," != *",$type,"* ]]; then
      if [ -z "$out" ]; then
        out="$type"
      else
        out="$out,$type"
      fi
    fi
  done

  echo "$out"
}

deploy_instance() {
  echo "🚀 Deploy Instance"
  echo "=================="
  
  echo "📦 Available packages:"
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list app-pkg
  fi
  
  echo ""
  read -p "Enter the package name/ID to deploy: " package_id
  
  if [ -z "$package_id" ]; then
    echo "❌ Package name/ID is required"
    return 1
  fi
  
  echo ""
  echo "🖥️  Available devices:"
  ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list devices --appPkgId "$package_id"
  
  echo ""
  read -p "Enter the device ID for deployment: " device_id
  
  if [ -z "$device_id" ]; then
    echo "❌ Device ID is required"
    return 1
  fi
  
  local device_supported_deployments=""
  if ! device_supported_deployments=$(get_supported_deployments_for_device "$device_id"); then
    echo "❌ Unable to determine device supported deployment types for device '$device_id'"
    echo "   Deployment aborted. Ensure device supportedDeploymentTypes"
    return 1
  fi

  # Get app package details and extract metadata.name
  app_packages=$(${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list app-pkg -o json 2>/dev/null)
  
  if [ $? -ne 0 ] || [ -z "$app_packages" ]; then
    echo "❌ Failed to get package list"
    return 1
  fi
  
  # Parse JSON to find the package and extract metadata.name
  if command -v jq >/dev/null 2>&1; then
    package_name=$(echo "$app_packages" | jq -r --arg pkg_id "$package_id" '
      .Data[0].items[] |
      select(.id == $pkg_id or .metadata.name == $pkg_id) |
      .metadata.name
    ')
    
    if [ -z "$package_name" ] || [ "$package_name" = "null" ]; then
      echo "❌ Package '$package_id' not found in the package list"
      echo "Available packages:"
      echo "$app_packages" | jq -r '.Data[0].items[] | "  - Name: \(.metadata.name), ID: \(.id)"'
      return 1
    fi
  else
    echo "❌ jq command is required but not installed. Please install it and retry."
    return 1
  fi
  
  # Generate instance.yaml dynamically from OCI metadata
  local temp_instance_file=$(mktemp --suffix=.yaml)
  
  if ! generate_instance_yaml_from_oci "$package_name" "$package_id" "$device_id" "$temp_instance_file" "$device_supported_deployments"; then
    echo "⚠️  Dynamic instance generation failed. Falling back to legacy template lookup..."
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
  fi
  
  # SECURITY: Make file read-only and calculate checksum
  chmod 444 "$deploy_file"
  local file_checksum=$(calculate_checksum "$deploy_file")
  
  # SECURITY: Verify file integrity before deployment
  if ! verify_file_integrity "$deploy_file" "$file_checksum"; then
    rm -f "$temp_instance_file"
    return 1
  fi
  
  # SECURITY: Final integrity check before deployment
  if ! verify_file_integrity "$deploy_file" "$file_checksum"; then
    echo "❌ SECURITY ALERT: Configuration file was modified after confirmation!"
    echo "   Deployment aborted for security reasons."
    rm -f "$temp_instance_file"
    return 1
  fi
  
  echo ""
  echo "🚀 Deploying '$package_id' to device '$device_id'..."
  if check_maestro_cli; then
    if ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" apply -f "$deploy_file"; then
      echo "✅ Instance deployment request sent successfully!"
      
      echo ""
      echo "📋 Updated deployments:"
      ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list deployment
    else
      echo "❌ Failed to deploy instance"
    fi
  fi
  
  # Cleanup temporary file
  rm -f "$temp_instance_file"
  
  echo ""
  read -p "Press Enter to continue..."
}

deploy_instance_non_interactive() {
  local package_id="$1"
  local device_id="$2"
  local device_supported_deployments=""

  echo "🚀 Deploy Instance (Non-Interactive)"
  echo "===================================="
  
  if [ -z "$package_id" ]; then
    echo "❌ Error: Package name/ID is required"
    echo "Usage: deploy_instance_non_interactive <package_id> <device_id>"
    return 1
  fi
  
  if [ -z "$device_id" ]; then
    echo "❌ Error: Device ID is required"
    echo "Usage: deploy_instance_non_interactive <package_id> <device_id>"
    return 1
  fi
  
  if ! device_supported_deployments=$(get_supported_deployments_for_device "$device_id"); then
    echo "❌ Unable to determine device supported deployment types for device '$device_id'"
    echo "   Deployment aborted. Ensure device .capabilities.roles are set (Standalone Device or Standalone Cluster)."
    return 1
  fi

  echo "📦 Package: $package_id"
  echo "🖥️  Device: $device_id"
  
  # Get app package details and extract metadata.name
  app_packages=$(${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list app-pkg -o json 2>/dev/null)
  
  if [ $? -ne 0 ] || [ -z "$app_packages" ]; then
    echo "❌ Failed to get package list"
    return 1
  fi
  
  # Parse JSON to find the package and extract metadata.name
  if command -v jq >/dev/null 2>&1; then
    package_name=$(echo "$app_packages" | jq -r --arg pkg_id "$package_id" '
      .Data[0].items[] |
      select(.id == $pkg_id or .metadata.name == $pkg_id) |
      .metadata.name
    ')
    
    if [ -z "$package_name" ] || [ "$package_name" = "null" ]; then
      echo "❌ Package '$package_id' not found in the package list"
      echo "Available packages:"
      echo "$app_packages" | jq -r '.Data[0].items[] | "  - Name: \(.metadata.name), ID: \(.id)"'
      return 1
    fi
  else
    echo "❌ jq command is required but not installed. Please install it and retry."
    return 1
  fi
  
  # Generate instance.yaml dynamically from OCI metadata
  local temp_instance_file=$(mktemp --suffix=.yaml)
  
  if ! generate_instance_yaml_from_oci "$package_name" "$package_id" "$device_id" "$temp_instance_file" "$device_supported_deployments"; then
    echo "⚠️  Dynamic instance generation failed. Falling back to legacy template lookup..."
    # Fallback to template discovery
    deploy_file=$(get_instance_file_path "$package_name")
    
    if [ $? -ne 0 ] || [ -z "$deploy_file" ] || [ ! -f "$deploy_file" ]; then
      echo "❌ No template found and dynamic generation failed"
      rm -f "$temp_instance_file"
      return 1
    fi
    
    # Update template with values
    repository=$(get_oci_repository_path "$package_name")
    sed -i "s|{{DEVICE_ID}}|$device_id|g" "$deploy_file" 2>/dev/null || true
    sed -i "s|{{PACKAGE_ID}}|$package_id|g" "$deploy_file" 2>/dev/null || true
    sed -i "s|{{REPOSITORY}}|$repository|g" "$deploy_file" 2>/dev/null || true
  else
    deploy_file="$temp_instance_file"
  fi
  
  # SECURITY: Make file read-only and calculate checksum
  chmod 444 "$deploy_file"
  local file_checksum=$(calculate_checksum "$deploy_file")
  
  # SECURITY: Verify file integrity before deployment
  if ! verify_file_integrity "$deploy_file" "$file_checksum"; then
    rm -f "$temp_instance_file"
    return 1
  fi
  
  # SECURITY: Final integrity check before deployment
  if ! verify_file_integrity "$deploy_file" "$file_checksum"; then
    echo "❌ SECURITY ALERT: Configuration file was modified after confirmation!"
    echo "   Deployment aborted for security reasons."
    rm -f "$temp_instance_file"
    return 1
  fi
  
  echo ""
  echo "🚀 Deploying '$package_id' to device '$device_id'..."
  if check_maestro_cli; then
    if ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" apply -f "$deploy_file"; then
      echo "✅ Instance deployment request sent successfully!"
      
      echo ""
      echo "📋 Updated deployments:"
      ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list deployment
      
      # Cleanup temporary file
      rm -f "$temp_instance_file"
      return 0
    else
      echo "❌ Failed to deploy instance"
      rm -f "$temp_instance_file"
      return 1
    fi
  else
    echo "❌ Maestro CLI not available"
    rm -f "$temp_instance_file"
    return 1
  fi

  echo "will print deploy file for confirmation (non-interactive): "
  cat "$deploy_file"  # Display the contents of the deployment file for user confirmation
}


delete_instance() {
  echo "🗑️  Delete Instance"
  echo "=================="
  
  echo "🚀 Current deployments:"
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list deployment
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
      if ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" delete deployment "$instance_id"; then
        echo "✅ Instance '$instance_id' deleted successfully!"
        
        echo ""
        echo "📋 Updated deployments:"
        ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list deployment
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

delete_instance_non_interactive() {
  local instance_id="$1"
  
  echo "🗑️  Delete Instance (Non-Interactive)"
  echo "===================================="
  
  if [ -z "$instance_id" ]; then
    echo "❌ Error: Instance/deployment ID is required"
    echo "Usage: delete_instance_non_interactive <instance_id>"
    return 1
  fi
  
  echo "🗑️  Deleting instance '$instance_id'..."
  if check_maestro_cli; then
    if ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" delete deployment "$instance_id"; then
      echo "✅ Instance '$instance_id' deleted successfully!"
      
      echo ""
      echo "📋 Updated deployments:"
      ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list deployment
      return 0
    else
      echo "❌ Failed to delete instance '$instance_id'"
      return 1
    fi
  else
    echo "❌ Maestro CLI not available"
    return 1
  fi
}

