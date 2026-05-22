#!/usr/bin/env bash

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WFM_DIR="$ROOT_DIR/wfm-supplier"
DEVICE_DIR="$ROOT_DIR/device-supplier"

# ============================================
# Utility Functions (Like wfm.sh pattern)
# ============================================
run_cmd() {
    local dir="$1"
    local msg="$2"
    shift 2
    echo "$msg"
    (cd "$dir" && "$@")
    local rc=$?
    if [[ $rc -eq 0 ]]; then
        echo "Status: SUCCESS"
    else
        echo "Status: FAILED (exit code: $rc)"
    fi
    # Only prompt for input if running interactively
    if [[ -t 0 ]]; then
        read -p "Press Enter to continue..." _
    fi
    return 0
}

show_menu() {
    local title="$1"
    shift
    local options=("$@")
    
    while true; do
        clear
        echo "=============================="
        echo "  $title"
        echo "=============================="
        echo
        for i in "${!options[@]}"; do
            echo "  $((i+1)). ${options[$i]}"
        done
        echo
        read -p "Enter choice: " choice
        echo "$choice"
        return 0
    done
}

# ============================================
# WFM Actions
# ============================================
ensure_wfm_portman() {
    local url="${1:-}"
    if [[ -z "$url" ]]; then
        read -p "Enter WFM URL: " url
    fi
    [[ -n "$url" ]] && run_cmd "$WFM_DIR" "Generating Postman-based collection using Portman..." bash run.sh portman "$url"
}

wfm_1() { ensure_wfm_portman "${1:-}"; }
wfm_2() {
    if [[ ! -f "$WFM_DIR/postman_collection.json" || ! -f "$WFM_DIR/newman-data/device-agent.env.json" || ! -f "$WFM_DIR/newman-data/device-agent.iteration.json" ]]; then
        echo "Portman-generated collection is missing, generating it first..."
        ensure_wfm_portman
    fi
    run_cmd "$WFM_DIR" "Running Newman against the current Postman-based collection..." bash run.sh newman
}
wfm_3() { read -p "Enter WFM URL: " url; [[ -n "$url" ]] && run_cmd "$WFM_DIR" "Generating Postman-based collection and running Newman..." bash run.sh all "$url"; }
wfm_4() { echo "Latest report:"; (cd "$WFM_DIR" && ls -lrt report_*.html 2>/dev/null | tail -1 || echo "No reports"); read -p "Press Enter..." _; }

# ============================================
# Device Actions
# ============================================
dev_1() { run_cmd "$DEVICE_DIR" "Building..." make build; }
dev_2() { run_cmd "$DEVICE_DIR" "Starting server..." make run-server; }
dev_3() { run_cmd "$DEVICE_DIR" "Running tests..." make run-tests; }
dev_4() { run_cmd "$DEVICE_DIR" "Running demo..." make demo; }
dev_5() { run_cmd "$DEVICE_DIR" "Stopping server..." make kill-server; }
dev_6() { run_cmd "$DEVICE_DIR" "Cleaning..." make clean; }
dev_7() { echo "Latest report:"; (cd "$DEVICE_DIR/reports" && ls -lrt conformance-report-*.html 2>/dev/null | tail -1 || echo "No reports"); read -p "Press Enter..." _; }

# ============================================
# Persona Menus (Simplified)
# ============================================
wfm_menu() {
    while true; do
        clear
        echo "=============================="
        echo "  WFM Supplier"
        echo "=============================="
        echo "  1. Setup Portman"
        echo "  2. Run Newman"
        echo "  3. Setup + Run"
        echo "  4. View Report"
        echo "  5. Back"
        echo
        read -p "Enter choice: " choice
        case "$choice" in
            1) wfm_1 ;;
            2) wfm_2 ;;
            3) wfm_3 ;;
            4) wfm_4 ;;
            5) return ;;
        esac
    done
}

device_menu() {
    while true; do
        clear
        echo "=============================="
        echo "  Device Supplier"
        echo "=============================="
        echo "  1. Build"
        echo "  2. Start Server"
        echo "  3. Run Tests"
        echo "  4. Demo"
        echo "  5. Stop Server"
        echo "  6. Clean"
        echo "  7. View Report"
        echo "  8. Back"
        echo
        read -p "Enter choice: " choice
        case "$choice" in
            1) dev_1 ;;
            2) dev_2 ;;
            3) dev_3 ;;
            4) dev_4 ;;
            5) dev_5 ;;
            6) dev_6 ;;
            7) dev_7 ;;
            8) return ;;
        esac
    done
}

# ============================================
# Main Menu
# ============================================
main_menu() {
    while true; do
        clear
        echo "=============================="
        echo "  Margo Conformance CLI"
        echo "=============================="
        echo "  1. WFM Supplier"
        echo "  2. Device Supplier"
        echo "  3. Exit"
        echo
        read -p "Enter choice: " choice

        case "$choice" in
            1) wfm_menu ;;
            2) device_menu ;;
            3) echo "Goodbye!"; exit 0 ;;
        esac
    done
}

# ============================================
# Start Here
# ============================================
main_menu
