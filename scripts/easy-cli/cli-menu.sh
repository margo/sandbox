#!/bin/bash
# Menu and UI functions for WFM CLI

show_menu() {
  clear
  load_wfm_env || true
  echo "🎛️  WFM CLI Interactive Interface (EasyCLI)"
  echo "=========================================="
  echo "Choose an option:"
  echo "1) 📦 List Application Package"
  echo "2) 🖥️  List Devices"
  echo "3) 🚀 List Deployment"
  echo "4) 📋 List All"
  echo "5) 📤 Upload Application Package"
  echo "6) 🗑️  Delete Application Package"
  echo "7) 🚀 Deploy Instance"
  echo "8) 🗑️  Delete Instance"
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
    *) echo "⚠️  Invalid choice"; sleep 2 ;;
  esac
}
