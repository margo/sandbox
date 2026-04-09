#!/bin/bash
set -e

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load configuration first (contains load_wfm_env function)
source "${SCRIPT_DIR}/easy-cli/cli-config.sh"

# Load environment
load_wfm_env || true

# Load common utilities
source "${SCRIPT_DIR}/easy-cli/cli-common.sh"

# Load all modules
source "${SCRIPT_DIR}/easy-cli/cli-harbor-ops.sh"
source "${SCRIPT_DIR}/easy-cli/cli-yaml-gen.sh"
source "${SCRIPT_DIR}/easy-cli/cli-packages.sh"
source "${SCRIPT_DIR}/easy-cli/cli-instances.sh"
source "${SCRIPT_DIR}/easy-cli/cli-menu.sh"

# Main loop for interactive mode
main_loop() {
  install_basic_utilities
  while true; do
    show_menu
  done
}

# Main script execution
if [[ -z "$1" ]]; then
  # Interactive mode
  main_loop
else
  # Command-line mode
  echo "[DEBUG] Script started with argument: '$1'" >&2
  echo "[DEBUG] Total arguments: $#" >&2
  echo "[DEBUG] All arguments: $@" >&2
  
  case "$1" in
    list-packages) 
      echo "[DEBUG] Matched list-packages case" >&2
      list_app_packages_non_interactive 
      ;;
    list-devices) 
      echo "[DEBUG] Matched list-devices case" >&2
      list_devices_non_interactive 
      ;;
    list-deployments) 
      echo "[DEBUG] Matched list-deployments case" >&2
      list_deployments_non_interactive 
      ;; 
    list-all-non-interactive) list_all_non_interactive ;;
    upload) upload_app_package ;;
    upload-app-non-interactive) upload_app_package_non_interactive "$2" ;;
    delete-package) delete_app_package ;;
    delete-package-non-interactive) delete_app_package_non_interactive "$2" ;;
    deploy) deploy_instance ;;
    deploy-non-interactive) deploy_instance_non_interactive "$2" "$3" ;;
    delete-instance) delete_instance ;;
    delete-instance-non-interactive) delete_instance_non_interactive "$2" ;;
    get-package-id-by-name) get_package_id_by_name "$2" ;;  
    get-device-id-by-role) get_device_id_by_role "$2" ;;              
    *)
      echo "[DEBUG] No case matched, showing usage" >&2
      echo "Usage: $0 {list-packages|list-devices|list-deployments|list-all-non-interactive|upload|upload-app-non-interactive|delete-package|delete-package-non-interactive|deploy|deploy-non-interactive|delete-instance|delete-instance-non-interactive|get-package-id-by-name|get-device-id-by-role}"
      exit 1
      ;;
  esac
fi

