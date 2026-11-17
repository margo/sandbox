#!/usr/bin/env bash

# =============================================================================
# SVID Certificate Generator for Margo Identity Service
# =============================================================================

set -euo pipefail

# --- Constants & Defaults ---
CONTAINER_NAME="margo-identity-service"
MIS_CLI="./mis-cli"
_FALLBACK_TRUST_DOMAIN="margo.org"

# Derive default trust domain from EXPOSED_MIS_HOST (format: mis.<TrustDomain>)
if [[ -n "${EXPOSED_MIS_HOST:-}" && "${EXPOSED_MIS_HOST}" =~ ^mis\.(.+)$ ]]; then
  DEFAULT_TRUST_DOMAIN="${BASH_REMATCH[1]}"
else
  DEFAULT_TRUST_DOMAIN="${_FALLBACK_TRUST_DOMAIN}"
fi

DEFAULT_TTL=7776000  # 90 days in seconds
OUTPUT_DIR_IN_CONTAINER="svidCert"
CSR_CONTAINER_PATH="/tmp"

# --- Colors & Logging ---
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
log_section() { echo -e "\n${BOLD}========================================${RESET}"; echo -e "${BOLD}  $*${RESET}"; echo -e "${BOLD}========================================${RESET}"; }

# --- Usage ---
usage() {
  echo -e ""
  echo -e "${BOLD}Usage:${RESET}"
  echo -e "  $(basename "$0") [OPTIONS]"
  echo -e ""
  echo -e "${BOLD}Modes:${RESET}"
  echo -e "  --interactive                     Run in interactive mode (default if no mode flags given)"
  echo -e "  --automated                       Run in automated mode"
  echo -e ""
  echo -e "${BOLD}Automated Mode Flags:${RESET}"
  echo -e "  --principal <wfm|wfm-client>      Required: principal for which to generate"
  echo -e "  --spiffe-id <spiffeID>            Required: full SPIFFE ID for the principal"
  echo -e "                                    e.g. spiffe://margo.org/margo/wfm/my-wfm"
  echo -e "                                         spiffe://margo.org/margo/wfm/my-wfm/client/my-client"
  echo -e "  --csr-path <path>                 Optional (Enrolled mode): path to CSR file on host."
  echo -e "                                    When provided, only the certificate is generated (Enrolled mode)."
  echo -e "                                    When omitted, key + certificate are generated (Installed mode)."
  echo -e "  --dns <name>                      Optional: DNS SAN (can be repeated)"
  echo -e ""
  echo -e "${BOLD}Examples:${RESET}"
  echo -e "  # Interactive mode"
  echo -e "  $(basename "$0") --interactive"
  echo -e ""
  echo -e "  # Automated mode - WFM (Installed mode)"
  echo -e "  $(basename "$0") --automated --principal wfm --spiffe-id spiffe://margo.org/margo/wfm/my-wfm"
  echo -e ""
  echo -e "  # Automated mode - WFM Client (Installed mode)"
  echo -e "  $(basename "$0") --automated --principal wfm-client --spiffe-id spiffe://margo.org/margo/wfm/my-wfm/client/my-client"
  echo -e ""
  echo -e "  # Automated mode - WFM (Enrolled mode)"
  echo -e "  $(basename "$0") --automated --principal wfm --spiffe-id spiffe://margo.org/margo/wfm/my-wfm --csr-path /path/to/wfm.csr"
  echo -e ""
  echo -e "  # Automated mode - WFM Client (Enrolled mode)"
  echo -e "  $(basename "$0") --automated --principal wfm-client --spiffe-id spiffe://margo.org/margo/wfm/my-wfm/client/my-client --csr-path /path/to/wfm-client.csr"
  echo -e ""
  exit 0
}

# --- Check container is running ---
check_container() {
  log_step "Verifying container '${CONTAINER_NAME}' is running"
  if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    log_error "Container '${CONTAINER_NAME}' is not running. Aborting."
    exit 1
  fi
  log_info "Container '${CONTAINER_NAME}' is running. ✓"
}

# --- Extract WFM ID and Client ID from SPIFFE ID ---
parse_spiffe_id() {
  local spiffe_id="$1"
  local mode="$2"

  if [[ "$mode" == "wfm" ]]; then
    WFM_ID=$(echo "$spiffe_id" | sed -n 's|spiffe://[^/]*/margo/wfm/\([^/]*\)$|\1|p')
    if [[ -z "$WFM_ID" ]]; then
      log_error "Could not parse WFM ID from SPIFFE ID: ${spiffe_id}"
      log_error "Expected format: spiffe://<trust-domain>/margo/wfm/<wfm-id>"
      exit 1
    fi
    WFM_CLIENT_ID=""
  elif [[ "$mode" == "wfm-client" ]]; then
    WFM_ID=$(echo "$spiffe_id" | sed -n 's|spiffe://[^/]*/margo/wfm/\([^/]*\)/client/.*|\1|p')
    WFM_CLIENT_ID=$(echo "$spiffe_id" | sed -n 's|spiffe://[^/]*/margo/wfm/[^/]*/client/\(.*\)|\1|p')
    if [[ -z "$WFM_ID" || -z "$WFM_CLIENT_ID" ]]; then
      log_error "Could not parse WFM ID / Client ID from SPIFFE ID: ${spiffe_id}"
      log_error "Expected format: spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<wfm-client-id>"
      exit 1
    fi
  fi
}

# --- Determine host output directory name ---
resolve_host_output_dir() {
  local mode="$1"
  if [[ "$mode" == "wfm" ]]; then
    HOST_OUTPUT_DIR="x509svid-${WFM_ID}"
  else
    HOST_OUTPUT_DIR="x509svid-${WFM_ID}-${WFM_CLIENT_ID}"
  fi
}

# --- Mint the SVID (Installed mode: key + cert generated from SPIFFE ID) ---
mint_svid_installed() {
  local spiffe_id="$1"
  local ttl="$2"
  local dns_list="$3"
  local mode_label="$4"

  log_step "[${mode_label}] Minting X.509 SVID — Installed mode"
  log_info "SPIFFE ID : ${spiffe_id}"
  log_info "TTL       : ${ttl} seconds"
  [[ -n "$dns_list" ]] && log_info "DNS SANs  : ${dns_list}" || log_info "DNS SANs  : (none)"

  log_info "Creating output directory '${OUTPUT_DIR_IN_CONTAINER}' inside container..."
  docker exec "$CONTAINER_NAME" mkdir -p "$OUTPUT_DIR_IN_CONTAINER"

  local cmd=(docker exec "$CONTAINER_NAME" "$MIS_CLI" mint x509
    --spiffeID "$spiffe_id"
    --ttl "$ttl"
    --outputDir "$OUTPUT_DIR_IN_CONTAINER"
  )

  if [[ -n "$dns_list" ]]; then
    for dns_entry in $dns_list; do
      cmd+=(--dns "$dns_entry")
    done
  fi

  log_info "Executing: ${cmd[*]}"
  "${cmd[@]}"
  log_info "SVID minted successfully. ✓"
}

# --- Mint the SVID (Enrolled mode: cert generated from principal-supplied CSR) ---
mint_svid_enrolled() {
  local spiffe_id="$1"
  local host_csr_path="$2"
  local ttl="$3"
  local dns_list="$4"
  local mode_label="$5"

  log_step "[${mode_label}] Minting X.509 SVID — Enrolled mode"

  # Validate CSR file exists on host
  if [[ ! -f "$host_csr_path" ]]; then
    log_error "CSR file not found on host: ${host_csr_path}"
    exit 1
  fi

  local csr_filename
  csr_filename="$(basename "$host_csr_path")"
  local container_csr_path="${CSR_CONTAINER_PATH}/${csr_filename}"

  log_info "SPIFFE ID          : ${spiffe_id}"
  log_info "Host CSR path      : ${host_csr_path}"
  log_info "Container CSR path : ${container_csr_path}"
  log_info "TTL                : ${ttl} seconds"
  [[ -n "$dns_list" ]] && log_info "DNS SANs           : ${dns_list}" || log_info "DNS SANs           : (none)"

  # Copy CSR into container
  log_step "[${mode_label}] Copying CSR into container"
  docker cp "$host_csr_path" "${CONTAINER_NAME}:${container_csr_path}"
  log_info "CSR copied to container at '${container_csr_path}'. ✓"

  # Create output directory inside container
  log_info "Creating output directory '${OUTPUT_DIR_IN_CONTAINER}' inside container..."
  docker exec "$CONTAINER_NAME" mkdir -p "$OUTPUT_DIR_IN_CONTAINER"

  # Build mint command with --spiffeID and --csrPath
  local cmd=(docker exec "$CONTAINER_NAME" "$MIS_CLI" mint x509
    --spiffeID "$spiffe_id"
    --csrPath "$container_csr_path"
    --ttl "$ttl"
    --outputDir "$OUTPUT_DIR_IN_CONTAINER"
  )

  if [[ -n "$dns_list" ]]; then
    for dns_entry in $dns_list; do
      cmd+=(--dns "$dns_entry")
    done
  fi

  log_info "Executing: ${cmd[*]}"
  "${cmd[@]}"
  log_info "SVID minted successfully via CSR. ✓"

  # Remove CSR from container
  log_step "[${mode_label}] Removing CSR from container"
  docker exec "$CONTAINER_NAME" rm -f "$container_csr_path"
  log_info "CSR removed from container. ✓"
}

# --- Copy certs from container to host ---
copy_certs_to_host() {
  local mode_label="$1"

  log_step "[${mode_label}] Copying certificates from container to host"
  log_info "Container path : ${CONTAINER_NAME}:/${OUTPUT_DIR_IN_CONTAINER}"
  log_info "Host path      : $(pwd)/${HOST_OUTPUT_DIR}"

  if [[ -d "${HOST_OUTPUT_DIR}" ]]; then
    log_warn "Directory '${HOST_OUTPUT_DIR}' already exists locally. Removing and replacing."
    rm -rf "${HOST_OUTPUT_DIR}"
  fi

  docker cp "${CONTAINER_NAME}:/${OUTPUT_DIR_IN_CONTAINER}" "${HOST_OUTPUT_DIR}"
  log_info "Certificates copied to '$(pwd)/${HOST_OUTPUT_DIR}'. ✓"
}

# =============================================================================
# INTERACTIVE MODE
# =============================================================================
run_interactive() {
  log_section "SVID Generator — Interactive Mode"

  # Step 1: Container check
  check_container

  # Step 2: Generation method selection
  log_step "SVID Generation Method"
  echo "  Select how to generate the SVID:"
  echo "    1) Installed mode — provide a SPIFFE ID; key and certificate will be generated by MIS"
  echo "    2) Enrolled mode  — provide a SPIFFE ID and a CSR; only the certificate will be generated (principal holds the key)"
  local GEN_METHOD=""
  while true; do
    read -rp "  Enter choice [1/2]: " method_choice
    case "$method_choice" in
      1) GEN_METHOD="installed"; break ;;
      2) GEN_METHOD="enrolled";  break ;;
      *) log_warn "Invalid choice. Please enter 1 or 2." ;;
    esac
  done
  log_info "Generation method: ${GEN_METHOD}"

  if [[ "$GEN_METHOD" == "installed" ]]; then
    _run_interactive_installed
  else
    _run_interactive_enrolled
  fi
}

# --- Interactive: Installed mode ---
_run_interactive_installed() {
  # Trust Domain
  log_step "Trust Domain"
  read -rp "  Enter Trust Domain [default: ${DEFAULT_TRUST_DOMAIN}]: " TRUST_DOMAIN
  TRUST_DOMAIN="${TRUST_DOMAIN:-$DEFAULT_TRUST_DOMAIN}"
  log_info "Trust Domain set to: ${TRUST_DOMAIN}"

  # Principal Selection
  log_step "Principal Selection"
  echo "  Select the principal for which to generate:"
  echo "    1) WFM        — SPIFFE ID: spiffe://<trust-domain>/margo/wfm/<wfm-id>"
  echo "    2) WFM Client — SPIFFE ID: spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<wfm-client-id>"
  while true; do
    read -rp "  Enter choice [1/2]: " mode_choice
    case "$mode_choice" in
      1) MODE="wfm";        MODE_LABEL="WFM";        break ;;
      2) MODE="wfm-client"; MODE_LABEL="WFM-Client";  break ;;
      *) log_warn "Invalid choice. Please enter 1 or 2." ;;
    esac
  done
  log_info "Principal selected: ${MODE_LABEL}"

  if [[ "$MODE" == "wfm" ]]; then
    echo ""
    printf "  ${BOLD}WFM ID${RESET} uniquely identifies your Workflow Manager instance.\n"
    read -rp "  Enter WFM ID: " WFM_ID
    while [[ -z "$WFM_ID" ]]; do
      log_warn "WFM ID cannot be empty."
      read -rp "  Enter WFM ID: " WFM_ID
    done
    SPIFFE_ID="spiffe://${TRUST_DOMAIN}/margo/wfm/${WFM_ID}"
    WFM_CLIENT_ID=""
  else
    echo ""
    printf "  ${BOLD}WFM ID${RESET} identifies the Workflow Manager this client belongs to.\n"
    read -rp "  Enter WFM ID: " WFM_ID
    while [[ -z "$WFM_ID" ]]; do
      log_warn "WFM ID cannot be empty."
      read -rp "  Enter WFM ID: " WFM_ID
    done
    echo ""
    printf "  ${BOLD}WFM Client ID${RESET} uniquely identifies this specific WFM Client within the WFM.\n"
    read -rp "  Enter WFM Client ID: " WFM_CLIENT_ID
    while [[ -z "$WFM_CLIENT_ID" ]]; do
      log_warn "WFM Client ID cannot be empty."
      read -rp "  Enter WFM Client ID: " WFM_CLIENT_ID
    done
    SPIFFE_ID="spiffe://${TRUST_DOMAIN}/margo/wfm/${WFM_ID}/client/${WFM_CLIENT_ID}"
  fi
  log_info "SPIFFE ID will be: ${SPIFFE_ID}"

  # TTL
  log_step "SVID TTL (Time-To-Live)"
  echo "  Common values:"
  echo "    15 minutes  →  900"
  echo "    1 day       →  86400"
  echo "    1 month     →  2592000"
  echo "    1 quarter   →  7776000  (default)"
  echo "    1 year      →  31536000"
  read -rp "  Enter TTL in seconds [default: ${DEFAULT_TTL}]: " TTL
  TTL="${TTL:-$DEFAULT_TTL}"
  if ! [[ "$TTL" =~ ^[0-9]+$ ]]; then
    log_error "TTL must be a positive integer. Got: '${TTL}'"
    exit 1
  fi
  log_info "TTL set to: ${TTL} seconds"

  # DNS SANs
  log_step "DNS Subject Alternative Names (optional)"
  echo "  Enter space-separated DNS names to include in the SVID (press Enter to skip)."
  echo "  Example: myservice.example.com api.example.com"
  read -rp "  DNS SANs: " DNS_LIST
  if [[ -n "$DNS_LIST" ]]; then
    log_info "DNS SANs: ${DNS_LIST}"
  else
    log_info "No DNS SANs specified."
  fi

  resolve_host_output_dir "$MODE"

  # Summary
  log_section "Summary [${MODE_LABEL} — Installed mode]"
  echo "  SPIFFE ID  : ${SPIFFE_ID}"
  echo "  TTL        : ${TTL} seconds"
  echo "  DNS SANs   : ${DNS_LIST:-(none)}"
  echo "  Output Dir : $(pwd)/${HOST_OUTPUT_DIR}"
  echo ""
  read -rp "  Proceed? [Y/n]: " confirm
  confirm="${confirm:-Y}"
  if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
    log_warn "Aborted by user. No SVID was generated"
    exit 2
  fi

  mint_svid_installed "$SPIFFE_ID" "$TTL" "$DNS_LIST" "$MODE_LABEL"
  copy_certs_to_host "$MODE_LABEL"

  log_section "Done [${MODE_LABEL} — Installed mode]"
  log_info "X.509 SVID certificates are available at: $(pwd)/${HOST_OUTPUT_DIR}"
}

# --- Interactive: Enrolled mode ---
_run_interactive_enrolled() {
  # Trust Domain
  log_step "Trust Domain"
  read -rp "  Enter Trust Domain [default: ${DEFAULT_TRUST_DOMAIN}]: " TRUST_DOMAIN
  TRUST_DOMAIN="${TRUST_DOMAIN:-$DEFAULT_TRUST_DOMAIN}"
  log_info "Trust Domain set to: ${TRUST_DOMAIN}"

  # Principal Selection
  log_step "Principal Selection"
  echo "  Select the principal this CSR belongs to:"
  echo "    1) WFM        — SPIFFE ID: spiffe://<trust-domain>/margo/wfm/<wfm-id>"
  echo "    2) WFM Client — SPIFFE ID: spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<wfm-client-id>"
  while true; do
    read -rp "  Enter choice [1/2]: " mode_choice
    case "$mode_choice" in
      1) MODE="wfm";        MODE_LABEL="WFM";        break ;;
      2) MODE="wfm-client"; MODE_LABEL="WFM-Client";  break ;;
      *) log_warn "Invalid choice. Please enter 1 or 2." ;;
    esac
  done
  log_info "Principal selected: ${MODE_LABEL}"

  if [[ "$MODE" == "wfm" ]]; then
    echo ""
    printf "  ${BOLD}WFM ID${RESET} uniquely identifies your Workflow Manager instance.\n"
    read -rp "  Enter WFM ID: " WFM_ID
    while [[ -z "$WFM_ID" ]]; do
      log_warn "WFM ID cannot be empty."
      read -rp "  Enter WFM ID: " WFM_ID
    done
    SPIFFE_ID="spiffe://${TRUST_DOMAIN}/margo/wfm/${WFM_ID}"
    WFM_CLIENT_ID=""
  else
    echo ""
    printf "  ${BOLD}WFM ID${RESET} identifies the Workflow Manager this client belongs to.\n"
    read -rp "  Enter WFM ID: " WFM_ID
    while [[ -z "$WFM_ID" ]]; do
      log_warn "WFM ID cannot be empty."
      read -rp "  Enter WFM ID: " WFM_ID
    done
    echo ""
    printf "  ${BOLD}WFM Client ID${RESET} uniquely identifies this specific WFM Client within the WFM.\n"
    read -rp "  Enter WFM Client ID: " WFM_CLIENT_ID
    while [[ -z "$WFM_CLIENT_ID" ]]; do
      log_warn "WFM Client ID cannot be empty."
      read -rp "  Enter WFM Client ID: " WFM_CLIENT_ID
    done
    SPIFFE_ID="spiffe://${TRUST_DOMAIN}/margo/wfm/${WFM_ID}/client/${WFM_CLIENT_ID}"
  fi
  log_info "SPIFFE ID will be: ${SPIFFE_ID}"

  # CSR file path
  log_step "CSR File Path"
  echo "  Provide the path to the CSR (PKCS#10) file on this host."
  echo "  Note: The principal generated its own key and exported this CSR. Only the certificate will be signed."
  local HOST_CSR_PATH=""
  while true; do
    read -rp "  Enter CSR file path: " HOST_CSR_PATH
    if [[ -z "$HOST_CSR_PATH" ]]; then
      log_warn "CSR file path cannot be empty."
    elif [[ ! -f "$HOST_CSR_PATH" ]]; then
      log_warn "File not found: '${HOST_CSR_PATH}'. Please enter a valid path."
    else
      break
    fi
  done
  log_info "CSR file: ${HOST_CSR_PATH}"

  # TTL
  log_step "SVID TTL (Time-To-Live)"
  echo "  Common values:"
  echo "    15 minutes  →  900"
  echo "    1 day       →  86400"
  echo "    1 month     →  2592000"
  echo "    1 quarter   →  7776000  (default)"
  echo "    1 year      →  31536000"
  read -rp "  Enter TTL in seconds [default: ${DEFAULT_TTL}]: " TTL
  TTL="${TTL:-$DEFAULT_TTL}"
  if ! [[ "$TTL" =~ ^[0-9]+$ ]]; then
    log_error "TTL must be a positive integer. Got: '${TTL}'"
    exit 1
  fi
  log_info "TTL set to: ${TTL} seconds"

  # DNS SANs
  log_step "DNS Subject Alternative Names (optional)"
  echo "  Enter space-separated DNS names to include in the SVID (press Enter to skip)."
  echo "  Example: myservice.example.com api.example.com"
  read -rp "  DNS SANs: " DNS_LIST
  if [[ -n "$DNS_LIST" ]]; then
    log_info "DNS SANs: ${DNS_LIST}"
  else
    log_info "No DNS SANs specified."
  fi

  resolve_host_output_dir "$MODE"

  # Summary
  log_section "Summary [${MODE_LABEL} — Enrolled mode]"
  echo "  SPIFFE ID  : ${SPIFFE_ID}"
  echo "  CSR File   : ${HOST_CSR_PATH}"
  echo "  TTL        : ${TTL} seconds"
  echo "  DNS SANs   : ${DNS_LIST:-(none)}"
  echo "  Output Dir : $(pwd)/${HOST_OUTPUT_DIR}"
  echo "  Note       : Private key will NOT be generated (principal retains its own key)."
  echo ""
  read -rp "  Proceed? [Y/n]: " confirm
  confirm="${confirm:-Y}"
  if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
    log_warn "Aborted by user."
    exit 0
  fi

  mint_svid_enrolled "$SPIFFE_ID" "$HOST_CSR_PATH" "$TTL" "$DNS_LIST" "$MODE_LABEL"
  copy_certs_to_host "$MODE_LABEL"

  log_section "Done [${MODE_LABEL} — Enrolled mode]"
  log_info "X.509 SVID certificates are available at: $(pwd)/${HOST_OUTPUT_DIR}"
}

# =============================================================================
# AUTOMATED MODE
# =============================================================================
run_automated() {
  local auto_mode="$1"
  local auto_spiffe_id="$2"
  local auto_csr_path="$3"
  local auto_dns_list="${AUTO_DNS_LIST[*]:-}"

  log_section "SVID Generator — Automated Mode"

  case "$auto_mode" in
    wfm)        MODE_LABEL="WFM" ;;
    wfm-client) MODE_LABEL="WFM-Client" ;;
    *)
      log_error "Invalid --principal value: '${auto_mode}'. Must be 'wfm' or 'wfm-client'."
      exit 1
      ;;
  esac

  # --spiffe-id is always required
  if [[ -z "$auto_spiffe_id" ]]; then
    log_error "Automated mode requires --spiffe-id for both Installed and Enrolled modes."
    usage
  fi

  log_info "Principal  : ${MODE_LABEL}"
  log_info "SPIFFE ID  : ${auto_spiffe_id}"
  log_info "TTL        : ${DEFAULT_TTL} seconds (default — 90 days)"
  log_info "DNS SANs   : ${auto_dns_list:-(none)}"

  check_container
  parse_spiffe_id "$auto_spiffe_id" "$auto_mode"

  if [[ "$auto_mode" == "wfm" ]]; then
    HOST_OUTPUT_DIR="x509svid-wfm"
  else
    HOST_OUTPUT_DIR="x509svid-wfmclient"
  fi

  log_info "Output Dir : $(pwd)/${HOST_OUTPUT_DIR}"

  if [[ -n "$auto_csr_path" ]]; then
    # Enrolled mode: principal supplies its own CSR; MIS signs and returns certificate only
    log_info "Mode       : Enrolled (CSR provided — certificate only, principal retains key)"
    mint_svid_enrolled "$auto_spiffe_id" "$auto_csr_path" "$DEFAULT_TTL" "$auto_dns_list" "$MODE_LABEL"
  else
    # Installed mode: MIS generates key + certificate from SPIFFE ID
    log_info "Mode       : Installed (no CSR — key and certificate generated by MIS)"
    mint_svid_installed "$auto_spiffe_id" "$DEFAULT_TTL" "$auto_dns_list" "$MODE_LABEL"
  fi

  copy_certs_to_host "$MODE_LABEL"

  log_section "Done [${MODE_LABEL}]"
  log_info "X.509 SVID certificates are available at: $(pwd)/${HOST_OUTPUT_DIR}"
}

# =============================================================================
# ARGUMENT PARSING & ENTRYPOINT
# =============================================================================
SCRIPT_MODE=""
AUTO_MODE=""
AUTO_SPIFFE_ID=""
AUTO_CSR_PATH=""
AUTO_DNS_LIST=()

if [[ $# -eq 0 ]]; then
  SCRIPT_MODE="interactive"
fi

while [[ $# -gt 0 ]]; do
  case "$1" in
    --interactive)   SCRIPT_MODE="interactive"; shift ;;
    --automated)     SCRIPT_MODE="automated";   shift ;;
    --principal)     AUTO_MODE="$2";            shift 2 ;;
    --spiffe-id)     AUTO_SPIFFE_ID="$2";       shift 2 ;;
    --csr-path)      AUTO_CSR_PATH="$2";        shift 2 ;;
    --dns)           AUTO_DNS_LIST+=("$2");      shift 2 ;;
    --help|-h)       usage ;;
    *)
      log_error "Unknown argument: $1"
      usage
      ;;
  esac
done

case "$SCRIPT_MODE" in
  interactive)
    run_interactive
    ;;
  automated)
    if [[ -z "$AUTO_MODE" ]]; then
      log_error "Automated mode requires --principal flag."
      usage
    fi
    run_automated "$AUTO_MODE" "$AUTO_SPIFFE_ID" "$AUTO_CSR_PATH"
    ;;
  *)
    log_error "No mode specified. Use --interactive or --automated."
    usage
    ;;
esac