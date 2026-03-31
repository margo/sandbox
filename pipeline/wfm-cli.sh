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
 case "$1" in
    list-packages) list_app_packages_non_interactive ;;  
    list-devices) list_devices_non_interactive ;;        
    list-deployments) list_deployments_non_interactive ;; 
    list-all) list_all ;;
    upload) upload_app_package ;;
    upload-app-non-interactive) upload_app_package_non_interactive "$2" ;;
    delete-package) delete_app_package ;;
    delete-package-non-interactive) delete_app_package_non_interactive "$2" ;;
    deploy) deploy_instance ;;
    deploy-non-interactive) deploy_instance_non_interactive "$2" "$3" ;;
    delete-instance) delete_instance ;;
    delete-instance-non-interactive) delete_instance_non_interactive "$2" ;;
    get-package-id-by-name) get_package_id_by_name "$2" ;;  
    get-first-device-id) get_first_device_id ;;              
    *)
      echo "Usage: $0 {list-packages|list-devices|list-deployments|list-all|upload|upload-app-non-interactive|delete-package|delete-package-non-interactive|deploy|deploy-non-interactive|delete-instance|delete-instance-non-interactive|get-package-id-by-name|get-first-device-id}"
      exit 1
      ;;
  esac
fi
