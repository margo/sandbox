#!/bin/bash
set -e

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load configuration first
source "${SCRIPT_DIR}/easy-cli/cli-config.sh"

# Load environment
load_wfm_env || true

# Load common utilities
source "${SCRIPT_DIR}/easy-cli/easy-cli-common.sh"

# Load all modules
source "${SCRIPT_DIR}/easy-cli/easy-cli-harbor-ops.sh"
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
    *)
      echo "Usage: $0 {list-packages|list-devices|list-deployments|list-all|upload|delete-package|deploy|delete-instance}"
      exit 1
      ;;
  esac
fi
