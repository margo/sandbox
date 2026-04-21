#!/bin/bash
# Configuration and environment management for WFM CLI

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"


load_wfm_env() {
  local env_file="$SCRIPT_DIR/wfm.env"
  
  if [[ ! -f "$env_file" ]]; then
    echo "[WARN] wfm.env not found at: $env_file"
    return 1
  fi
  
  echo "[INFO] Loading environment from: $env_file"
  set -a
  source "$env_file"
  set +a
}

# Harbor settings (can be overridden via env)
export EXPOSED_HARBOR_HOST="${EXPOSED_HARBOR_HOST:-harbor.machine}"
export EXPOSED_HARBOR_PORT="${EXPOSED_HARBOR_PORT:-8443}"

# Symphony settings (can be overridden via env)
export EXPOSED_SYMPHONY_HOST="${EXPOSED_SYMPHONY_HOST:-127.0.0.1}"
export EXPOSED_SYMPHONY_PORT="${EXPOSED_SYMPHONY_PORT:-8082}"

# OCI Registry settings (can be overridden via env)
# CHANGED: Use HTTPS for Harbor registry
export REGISTRY_URL="${REGISTRY_URL:-https://${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}}"
export REGISTRY_USER="${REGISTRY_USER:-admin}"
export REGISTRY_PASS="${REGISTRY_PASS:-Harbor12345}"
export OCI_ORGANIZATION="${OCI_ORGANIZATION:-library}"

# CLI path
export MAESTRO_CLI_PATH="${MAESTRO_CLI_PATH:-$HOME/symphony/cli}"
