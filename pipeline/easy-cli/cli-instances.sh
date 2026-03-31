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

list_deployments() {
  echo "🚀 Listing all deployments from WFM..."
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list deployment || echo "❌ Failed to list deployment"
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
  ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list devices
  
  echo ""
  read -p "Enter the device ID for deployment: " device_id
  
  if [ -z "$device_id" ]; then
    echo "❌ Device ID is required"
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
  
  # Generate instance.yaml dynamically from OCI metadata
  local temp_instance_file=$(mktemp --suffix=.yaml)
  
  if ! generate_instance_yaml_from_oci "$package_name" "$package_id" "$device_id" "$temp_instance_file" 2>/dev/null; then
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
  
  # Generate instance.yaml dynamically from OCI metadata
  local temp_instance_file=$(mktemp --suffix=.yaml)
  
  if ! generate_instance_yaml_from_oci "$package_name" "$package_id" "$device_id" "$temp_instance_file" 2>/dev/null; then
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

