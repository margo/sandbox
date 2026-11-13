#!/usr/bin/env bash

# =============================================================================
# Margo Conformance Identity Generator
# =============================================================================

set -euo pipefail

# =============================================================================
# CONFIGURABLE VARIABLES — edit these as needed
# =============================================================================
WFM_ID="wfm-1"
WFM_CLIENT_ID="wfm-client-1"
OUTPUT_DIRECTORY="${HOME}/conformance-identities"
TRUST_DOMAIN="ctt.margo.org"
SANDBOX_BRANCH="main"

# =============================================================================
# DERIVED VARIABLES
# =============================================================================
WFM_SPIFFE_ID="spiffe://${TRUST_DOMAIN}/margo/wfm/${WFM_ID}"
WFM_CLIENT_SPIFFE_ID="spiffe://${TRUST_DOMAIN}/margo/wfm/${WFM_ID}/client/${WFM_CLIENT_ID}"
MIS_HOST="mis.${TRUST_DOMAIN}"
SANDBOX_DIR="/tmp/sandbox"
MIS_DIR="${SANDBOX_DIR}/mis"
MIS_LOG="${OUTPUT_DIRECTORY}/mis.log"
MIS_PID_FILE="/tmp/mis.pid"
HOSTS_ENTRY="127.0.0.1 ${MIS_HOST}"

# =============================================================================
# COLORS & LOGGING
# =============================================================================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

log_info()    { echo -e "${GREEN}[INFO]${RESET}  $*"; }
log_warn()    { echo -e "${YELLOW}[WARN]${RESET}  $*"; }
log_error()   { echo -e "${RED}[ERROR]${RESET} $*" >&2; }
log_step()    { echo -e "\n${CYAN}${BOLD}>>> STEP: $*${RESET}"; }
log_section() {
  echo -e "\n${BOLD}========================================${RESET}"
  echo -e "${BOLD}  $*${RESET}"
  echo -e "${BOLD}========================================${RESET}"
}

# =============================================================================
# CLEANUP HANDLER
# =============================================================================
cleanup() {
  local exit_code=$?
  log_section "Cleanup"

  # Stop MIS process
  if [[ -f "$MIS_PID_FILE" ]]; then
    local mis_pid
    mis_pid=$(cat "$MIS_PID_FILE")
    if kill -0 "$mis_pid" 2>/dev/null; then
      log_info "Stopping MIS process (PID: ${mis_pid})..."
      kill "$mis_pid" 2>/dev/null || true
      wait "$mis_pid" 2>/dev/null || true
      log_info "MIS process stopped. ✓"
    else
      log_warn "MIS process (PID: ${mis_pid}) was not running."
    fi
    rm -f "$MIS_PID_FILE"
  fi

  # Remove sandbox clone
  if [[ -d "$SANDBOX_DIR" ]]; then
    log_info "Removing sandbox directory: ${SANDBOX_DIR}"
    rm -rf "$SANDBOX_DIR"
    log_info "Sandbox removed. ✓"
  fi

  # Remove /etc/hosts entry
  log_info "Removing '${MIS_HOST}' from /etc/hosts..."
  if grep -qF "$MIS_HOST" /etc/hosts; then
    sudo sed -i.bak "/[[:space:]]${MIS_HOST}\b/d" /etc/hosts
    log_info "/etc/hosts entry removed. ✓"
  else
    log_warn "No /etc/hosts entry found for '${MIS_HOST}'. Nothing to remove."
  fi

  if [[ $exit_code -eq 0 ]]; then
    log_info "Cleanup complete. Exiting successfully."
  else
    log_warn "Cleanup complete. Script exited with code ${exit_code}."
  fi
}

trap cleanup EXIT

# =============================================================================
# PREREQUISITE CHECKS
# =============================================================================
check_prerequisites() {
  log_step "Checking prerequisites"
  local missing=()
  for cmd in git make curl jq docker sudo; do
    if ! command -v "$cmd" &>/dev/null; then
      missing+=("$cmd")
    fi
  done
  if [[ ${#missing[@]} -gt 0 ]]; then
    log_error "Missing required tools: ${missing[*]}"
    exit 1
  fi
  log_info "All prerequisites satisfied. ✓"
}

# =============================================================================
# STEP 1 — Clone sandbox
# =============================================================================
clone_sandbox() {
  log_step "Cloning sandbox repository (branch: ${SANDBOX_BRANCH})"

  if [[ -d "$SANDBOX_DIR" ]]; then
    log_warn "Existing sandbox directory found at '${SANDBOX_DIR}'. Removing it first."
    rm -rf "$SANDBOX_DIR"
  fi

  git clone \
    --branch "$SANDBOX_BRANCH" \
    --single-branch \
    https://github.com/margo/sandbox.git \
    "$SANDBOX_DIR"

  log_info "Repository cloned to '${SANDBOX_DIR}'. ✓"
}

# =============================================================================
# STEP 2 — Build MIS
# =============================================================================
build_mis() {
  log_step "Building MIS (make build)"
  cd "$MIS_DIR"
  make build
  log_info "MIS built successfully. ✓"
}

# =============================================================================
# STEP 3 — Prepare configuration
# =============================================================================
prepare_config() {
  log_step "Preparing MIS configuration"
  cd "$MIS_DIR"

  cp pkg/conf/configuration.json .
  log_info "Copied configuration.json to '${MIS_DIR}'. ✓"

  log_info "Patching configuration.json — https.addr=':39443', trustDomain='${TRUST_DOMAIN}'"
  local tmp_conf
  tmp_conf=$(mktemp)
  jq \
    --arg addr ":39443" \
    --arg td "$TRUST_DOMAIN" \
    '.https.addr = $addr | .trustDomain = $td' \
    configuration.json > "$tmp_conf"
  mv "$tmp_conf" configuration.json
  log_info "configuration.json patched. ✓"
}

# =============================================================================
# STEP 4 — Update /etc/hosts
# =============================================================================
update_hosts() {
  log_step "Updating /etc/hosts for '${MIS_HOST}'"

  if grep -qF "$MIS_HOST" /etc/hosts; then
    log_warn "'${MIS_HOST}' already present in /etc/hosts. Updating entry."
    sudo sed -i.bak "s|.*[[:space:]]${MIS_HOST}\b.*|${HOSTS_ENTRY}|" /etc/hosts
  else
    log_info "Appending '${HOSTS_ENTRY}' to /etc/hosts."
    echo "$HOSTS_ENTRY" | sudo tee -a /etc/hosts > /dev/null
  fi

  log_info "/etc/hosts updated. ✓"
}

# =============================================================================
# STEP 5 — Generate PKI
# =============================================================================
generate_pki() {
  log_step "Generating PKI certificates (pki_gen.sh --automated --dns ${MIS_HOST})"
  cd "$MIS_DIR"

  bash ../scripts/lib/mis/pki_gen.sh --automated --dns "$MIS_HOST"
  log_info "PKI generation complete. ✓"
}

# =============================================================================
# STEP 6 — Start MIS process
# =============================================================================
start_mis() {
  log_step "Starting MIS process"
  cd "$MIS_DIR"

  mkdir -p "$OUTPUT_DIRECTORY"

  log_info "MIS logs will be written to: ${MIS_LOG}"
  ./mis start --config ./configuration.json > "$MIS_LOG" 2>&1 &
  local mis_pid=$!
  echo "$mis_pid" > "$MIS_PID_FILE"
  log_info "MIS started in background (PID: ${mis_pid}). ✓"

  # Wait briefly and confirm process is still alive
  sleep 3
  if ! kill -0 "$mis_pid" 2>/dev/null; then
    log_error "MIS process exited immediately. Check logs at: ${MIS_LOG}"
    exit 1
  fi
  log_info "MIS process is running. ✓"
}

# =============================================================================
# STEP 7 — Fetch trust details & trust bundle
# =============================================================================
fetch_trust_details() {
  log_step "Fetching trust details from MIS well-known endpoint"
  cd "$MIS_DIR"

  local cacert="./certs/https-ca.crt"
  local margo_url="https://${MIS_HOST}:39443/.well-known/margo"
  local max_retries=10
  local retry_interval=3
  local attempt=0
  local margo_response=""

  log_info "Waiting for MIS to become ready at ${margo_url}..."
  while [[ $attempt -lt $max_retries ]]; do
    attempt=$(( attempt + 1 ))
    log_info "Attempt ${attempt}/${max_retries}..."
    if margo_response=$(curl --silent --fail --cacert "$cacert" "$margo_url" 2>/dev/null); then
      log_info "MIS responded. ✓"
      break
    fi
    if [[ $attempt -eq $max_retries ]]; then
      log_error "MIS did not become ready after ${max_retries} attempts. Check logs at: ${MIS_LOG}"
      exit 1
    fi
    sleep "$retry_interval"
  done

  log_info "Raw response from /.well-known/margo:"
  echo "$margo_response" | jq .

  # Extract trustDomain
  local fetched_trust_domain
  fetched_trust_domain=$(echo "$margo_response" | jq -r '.trustDomain')
  log_info "Saving trustDomain ('${fetched_trust_domain}') to ${OUTPUT_DIRECTORY}/trustDetails.txt"
  echo "$fetched_trust_domain" > "${OUTPUT_DIRECTORY}/trustDetails.txt"
  log_info "trustDetails.txt saved. ✓"

  # Extract trustBundleUri and fetch bundle
  local trust_bundle_uri
  trust_bundle_uri=$(echo "$margo_response" | jq -r '.trustBundleUri')
  log_info "Fetching trust bundle from: ${trust_bundle_uri}"
  curl --silent --fail --cacert "$cacert" "$trust_bundle_uri" \
    -o "${OUTPUT_DIRECTORY}/trustbundle.json"
  log_info "trustbundle.json saved to '${OUTPUT_DIRECTORY}/trustbundle.json'. ✓"
}

# =============================================================================
# STEP 8 — Mint X.509 SVIDs
# =============================================================================
mint_svids() {
  log_step "Minting X.509 SVIDs"
  cd "$MIS_DIR"

  mkdir -p "$OUTPUT_DIRECTORY"/wfm
  mkdir -p "$OUTPUT_DIRECTORY"/wfmclient

  log_info "Minting SVID for WFM (SPIFFE ID: ${WFM_SPIFFE_ID})"
  ./mis mint x509 \
    --spiffeID "$WFM_SPIFFE_ID" \
    --ttl "86400" \
    --outputDir "$OUTPUT_DIRECTORY"/wfm
  log_info "WFM SVID minted. ✓"

  log_info "Minting SVID for WFM Client (SPIFFE ID: ${WFM_CLIENT_SPIFFE_ID})"
  ./mis mint x509 \
    --spiffeID "$WFM_CLIENT_SPIFFE_ID" \
    --ttl "86400" \
    --outputDir "$OUTPUT_DIRECTORY"/wfmclient
  log_info "WFM Client SVID minted. ✓"
}

# =============================================================================
# STEP 9 — Verify output artifacts
# =============================================================================
verify_artifacts() {
  log_step "Verifying output artifacts"

  local all_ok=true

  _check_file() {
    local path="$1"
    if [[ -f "$path" ]]; then
      log_info "  [✓] Found file : ${path}"
    else
      log_error "  [✗] Missing file: ${path}"
      all_ok=false
    fi
  }

  _check_dir() {
    local path="$1"
    if [[ -d "$path" ]]; then
      log_info "  [✓] Found dir  : ${path}"
    else
      log_error "  [✗] Missing dir : ${path}"
      all_ok=false
    fi
  }

  local wfm_svid_dir="${OUTPUT_DIRECTORY}/wfm"
  local wfm_client_svid_dir="${OUTPUT_DIRECTORY}/wfmclient"

  _check_file "${OUTPUT_DIRECTORY}/trustDetails.txt"
  _check_file "${OUTPUT_DIRECTORY}/trustbundle.json"
  _check_dir  "${wfm_svid_dir}"
  _check_file "${wfm_svid_dir}/payload-cert.pem"
  _check_file "${wfm_svid_dir}/payload-key.pem"
  _check_dir  "${wfm_client_svid_dir}"
  _check_file "${wfm_client_svid_dir}/payload-cert.pem"
  _check_file "${wfm_client_svid_dir}/payload-key.pem"

  if [[ "$all_ok" != "true" ]]; then
    log_error "One or more expected artifacts are missing. Review errors above."
    exit 1
  fi

  log_info "All expected artifacts are present. ✓"
}

# =============================================================================
# SUMMARY
# =============================================================================
print_summary() {
  log_section "Artifact Summary"
  echo ""
  echo -e "  ${BOLD}Output Directory   :${RESET} ${OUTPUT_DIRECTORY}"
  echo ""
  echo -e "  ${BOLD}Trust Details      :${RESET} ${OUTPUT_DIRECTORY}/trustDetails.txt"
  echo -e "  ${BOLD}Trust Bundle       :${RESET} ${OUTPUT_DIRECTORY}/trustbundle.json"
  echo -e "  ${BOLD}MIS Logs           :${RESET} ${MIS_LOG}"
  echo ""
  echo -e "  ${BOLD}WFM SVID Dir       :${RESET} ${OUTPUT_DIRECTORY}/x509svid-${WFM_ID}/"
  echo -e "    ├── payload-cert.pem"
  echo -e "    └── payload-key.pem"
  echo ""
  echo -e "  ${BOLD}WFM Client SVID Dir:${RESET} ${OUTPUT_DIRECTORY}/x509svid-${WFM_ID}-${WFM_CLIENT_ID}/"
  echo -e "    ├── payload-cert.pem"
  echo -e "    └── payload-key.pem"
  echo ""
  echo -e "  ${BOLD}SPIFFE IDs used    :${RESET}"
  echo -e "    WFM        → ${WFM_SPIFFE_ID}"
  echo -e "    WFM Client → ${WFM_CLIENT_SPIFFE_ID}"
  echo ""
}

# =============================================================================
# MAIN
# =============================================================================
main() {
  log_section "Margo Conformance Identity Generator"
  log_info "WFM ID            : ${WFM_ID}"
  log_info "WFM Client ID     : ${WFM_CLIENT_ID}"
  log_info "Trust Domain      : ${TRUST_DOMAIN}"
  log_info "Sandbox Branch    : ${SANDBOX_BRANCH}"
  log_info "Output Directory  : ${OUTPUT_DIRECTORY}"
  log_info "WFM SPIFFE ID     : ${WFM_SPIFFE_ID}"
  log_info "WFM Client SPIFFE : ${WFM_CLIENT_SPIFFE_ID}"
  log_info "MIS Host          : ${MIS_HOST}"

  mkdir -p "$OUTPUT_DIRECTORY"

  check_prerequisites
  clone_sandbox
  build_mis
  prepare_config
  update_hosts
  generate_pki
  start_mis
  fetch_trust_details
  mint_svids
  verify_artifacts
  print_summary

  log_info "Script completed successfully. Cleanup will now run."
}

main "$@"