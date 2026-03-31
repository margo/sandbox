#!/bin/bash
# Package management operations for WFM CLI

list_app_packages() {
  echo "📦 Listing all app packages from WFM..."
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list app-pkg || echo "❌ Failed to list app-pkg"
  fi
  echo ""
  read -p "Press Enter to continue..."
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
    if ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" apply -f "$temp_pkg_file"; then
      echo "✅ $package_name uploaded successfully!"
    else
      echo "❌ Failed to upload $package_name"
    fi
  fi
  
  rm -f "$temp_pkg_file"
  echo ""
  read -p "Press Enter to continue..."
}

# Add this new function to cli-packages.sh

upload_app_package_non_interactive() {
  local package_name="$1"
  
  if [ -z "$package_name" ]; then
    echo "❌ Error: Package name is required"
    echo "Usage: upload_app_package_non_interactive <package_name>"
    return 1
  fi
  
  
  echo "📦 Uploading App Package: $package_name"
  echo "=========================================="
  
  # Discover packages from Harbor
  local packages=$(discover_app_packages_from_harbor)
  
  if [ -z "$packages" ]; then
    echo "❌ No app packages available in Harbor"
    return 1
  fi
  
  # Search for matching package
  local selected_pkg=""
  while IFS= read -r pkg; do
    local display_name=$(get_package_metadata_from_oci "$pkg")
    if [[ "$display_name" == "$package_name" ]] || [[ "$pkg" == *"$package_name"* ]]; then
      selected_pkg="$pkg"
      break
    fi
  done <<< "$packages"
  
  if [ -z "$selected_pkg" ]; then
    echo "❌ Package '$package_name' not found in Harbor"
    echo "Available packages:"
    while IFS= read -r pkg; do
      echo "  - $(get_package_metadata_from_oci "$pkg")"
    done <<< "$packages"
    return 1
  fi
  
  local found_package_name=$(get_package_metadata_from_oci "$selected_pkg")
  echo "✅ Found package: $found_package_name"
  
  # Generate package.yaml for WFM
  local temp_pkg_file=$(mktemp)
  generate_wfm_package_yaml "$selected_pkg" "$temp_pkg_file"
  
  if [ ! -f "$temp_pkg_file" ]; then
    echo "❌ Failed to generate package file"
    return 1
  fi
  
  echo "📤 Uploading $found_package_name to WFM..."
  if check_maestro_cli; then
    if ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" apply -f "$temp_pkg_file"; then
      echo "✅ $found_package_name uploaded successfully!"
      rm -f "$temp_pkg_file"
      return 0
    else
      echo "❌ Failed to upload $found_package_name"
      rm -f "$temp_pkg_file"
      return 1
    fi
  else
    echo "❌ Maestro CLI not available"
    rm -f "$temp_pkg_file"
    return 1
  fi
}


delete_app_package() {
  echo "🗑️  Delete App Package"
  echo "===================="
  
  echo "📦 Current packages:"
  if check_maestro_cli; then
    ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list app-pkg
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
      if ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" delete app-pkg "$package_id"; then
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

delete_app_package_non_interactive() {
  local package_id="$1"
  
  echo "🗑️  Delete App Package (Non-Interactive)"
  echo "===================="
  
  if [ -z "$package_id" ]; then
    echo "❌ Error: Package name/ID is required"
    echo "Usage: delete_app_package_non_interactive <package_id>"
    return 1
  fi
  
  echo "🗑️  Deleting package '$package_id'..."
  if check_maestro_cli; then
    if ${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" delete app-pkg "$package_id"; then
      echo "✅ Package '$package_id' deleted successfully!"
      return 0
    else
      echo "❌ Failed to delete app-pkg '$package_id'"
      return 1
    fi
  else
    echo "❌ Maestro CLI not available"
    return 1
  fi
}

