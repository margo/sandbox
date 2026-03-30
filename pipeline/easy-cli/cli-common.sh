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
