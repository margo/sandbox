#!/bin/bash
# lib/common.sh - Shared utility functions

info() {
    echo "ℹ️  $1"
}

success() {
    echo "✅ $1"
}

pause() {
    echo
    read -rp "Press Enter to continue..." _
}

command_exists() {
    command -v "$1" >/dev/null 2>&1
}

package_installed() {
    dpkg -s "$1" >/dev/null 2>&1
}

get_ubuntu_codename() {
    lsb_release -cs 2>/dev/null || echo "noble"
}

resolve_target_arch() {
    local host_arch="$(uname -m)"
    case "${host_arch}" in
        x86_64|amd64)
            echo "amd64"
            ;;
        aarch64|arm64)
            echo "arm64"
            ;;
        *)
            echo "Unsupported architecture" >&2
            return 1
            ;;
    esac
}
