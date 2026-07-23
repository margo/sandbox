#!/bin/bash
# Common utility functions for WFM CLI

install_basic_utilities() {
  local PACKAGES="jq"

  echo "🔄 Installing CLI utilities..."
  INSTALLATION_NEEDED="false"

  for pkg in $PACKAGES; do
    if ! dpkg -l | grep -q "^ii  $pkg "; then
      INSTALLATION_NEEDED="true"
      break
    fi
  done

  if [[ "${INSTALLATION_NEEDED}" == "true" ]]; then
    sudo apt update && sudo apt install -y $PACKAGES
    echo "✅ CLI utilities installed"
  else
    echo "⚡️ CLI utilities already installed"
  fi
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

# Checksum verification functions
calculate_checksum() {
  local file="$1"
  sha256sum "$file" | awk '{print $1}'
}

verify_file_integrity() {
  local file="$1"
  local expected_checksum="$2"
  local current_checksum=$(calculate_checksum "$file")

  if [ "$expected_checksum" != "$current_checksum" ]; then
    echo "❌ SECURITY ALERT: File was modified!"
    echo "   Expected: ${expected_checksum:0:16}..."
    echo "   Current:  ${current_checksum:0:16}..."
    return 1
  fi
  return 0
}

# Pause function for interactive mode
pause() {
  echo ""
  read -p "Press Enter to continue..." _
}

# Get package ID by name
get_package_id_by_name() {
  local package_name="$1"

  if [ -z "$package_name" ]; then
    echo "❌ Error: Package name is required"
    return 1
  fi

  if ! check_maestro_cli; then
    echo "❌ Maestro CLI not available"
    return 1
  fi

  local app_packages=$(${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list app-pkg -o json 2>/dev/null)

  if [ $? -ne 0 ] || [ -z "$app_packages" ]; then
    echo "❌ Failed to get package list"
    return 1
  fi

  if command -v jq >/dev/null 2>&1; then
    local package_id=$(echo "$app_packages" | jq -r --arg name "$package_name" '
      .Data[0].items[] |
      select(.metadata.name == $name) |
      .id
    ')

    if [ -z "$package_id" ] || [ "$package_id" = "null" ]; then
      echo "❌ Package '$package_name' not found"
      return 1
    fi

    echo "$package_id"
    return 0
  else
    echo "❌ jq is required but not installed"
    return 1
  fi
}

# Get device ID by role
get_device_id_by_role() {
  local role="$1"

  if [ -z "$role" ]; then
    echo "❌ Error: Role parameter is required"
    return 1
  fi

  if ! check_maestro_cli; then
    echo "❌ Maestro CLI not available"
    return 1
  fi

  local devices=$(${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list devices -o json 2>/dev/null)

  if [ $? -ne 0 ] || [ -z "$devices" ]; then
    echo "❌ Failed to get device list"
    return 1
  fi

  if command -v jq >/dev/null 2>&1; then

    # 1. Correct path: .spec.capabilities.properties.roles[]
    # 2. Iterate through all Data[] items, not just Data[0]
    local device_id=$(echo "$devices" | jq -r --arg role "$role" '
      .Data[] |
      .items[] |
      select(.spec.capabilities.properties.roles[]? == $role) |
      .id
    ' | head -1)

    if [ -z "$device_id" ] || [ "$device_id" = "null" ]; then
      echo "❌ No device found with role: $role"
      return 1
    fi

    echo "$device_id"
    return 0
  else
    echo "❌ jq is required but not installed"
    return 1
  fi
}

# Get device ID by supportedDeploymentTypes
get_device_id_by_deployment_type() {
  local deployment_type="$1"

  if [ -z "$deployment_type" ]; then
    echo "❌ Error: Deployment type parameter is required"
    return 1
  fi

  if ! check_maestro_cli; then
    echo "❌ Maestro CLI not available"
    return 1
  fi

  local devices=$(${MAESTRO_CLI_PATH}/maestro wfm --host "$EXPOSED_SYMPHONY_HOST" --port "$EXPOSED_SYMPHONY_PORT" list devices -o json 2>/dev/null)

  if [ $? -ne 0 ] || [ -z "$devices" ]; then
    echo "❌ Failed to get device list"
    return 1
  fi

  
  echo "raw data__"
  echo "$devices" 

  if command -v jq >/dev/null 2>&1; then

    # 1. Correct path: .spec.capabilities.properties.supportedDeploymentTypes[]
    # 2. Iterate through all Data[] items, not just Data[0]
    local device_id=$(echo "$devices" | jq -r --arg deployment_type "$deployment_type" '
      .Data[] |
      .items[] |
      select(.spec.capabilities.properties.supportedDeploymentTypes[]? == $deployment_type) |
      .id
    ' | head -1)

    if [ -z "$device_id" ] || [ "$device_id" = "null" ]; then
      echo "raw data: $devices" 
      echo "❌ No device found with deployment type(s): $deployment_type, $devices"
      return 1
    fi

    echo "$device_id"
    return 0
  else
    echo "❌ jq is required but not installed"
    return 1
  fi
}
