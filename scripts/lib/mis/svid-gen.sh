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
  echo -e "  --spiffe-id <spiffeID>            Required (SpiffeID flow): full SPIFFE ID"
  echo -e "                                    e.g. spiffe://margo.org/margo/wfm/my-wfm"
  echo -e "                                         spiffe://margo.org/margo/wfm/my-wfm/client/my-client"
  echo -e "  --csr-path <path>                 Required (CSR flow): path to CSR file on host"
  echo -e "  --dns <name>                      Optional: DNS SAN (can be repeated)"
  echo -e ""
  echo -e "${BOLD}Examples:${RESET}"
  echo -e "  # Interactive mode"
  echo -e "  $(basename "$0") --interactive"
  echo -e ""
  echo -e "  # Automated mode - WFM (SpiffeID flow)"
  echo -e "  $(basename "$0") --automated --principal wfm --spiffe-id spiffe://margo.org/margo/wfm/my-wfm"
  echo -e ""
  echo -e "  # Automated mode - WFM Client (SpiffeID flow)"
  echo -e "  $(basename "$0") --automated --principal wfm-client --spiffe-id spiffe://margo.org/margo/wfm/my-wfm/client/my-client"
  echo -e ""
  echo -e "  # Automated mode - WFM (CSR flow)"
  echo -e "  $(basename "$0") --automated --principal wfm --csr-path /path/to/wfm.csr"
  echo -e ""
  echo -e "  # Automated mode - WFM Client (CSR flow)"
  echo -e "  $(basename "$0") --automated --principal wfm-client --csr-path /path/to/wfm-client.csr"
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

# --- Mint the SVID via SPIFFE ID ---
mint_svid() {
  local spiffe_id="$1"
  local ttl="$2"
  local dns_list="$3"
  local mode_label="$4"

  log_step "[${mode_label}] Minting X.509 SVID inside container"
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

# --- Mint the SVID via CSR ---
mint_svid_csr() {
  local host_csr_path="$1"
  local ttl="$2"
  local dns_list="$3"
  local mode_label="$4"

  log_step "[${mode_label}] Minting X.509 SVID via CSR inside container"

  # Validate CSR file exists on host
  if [[ ! -f "$host_csr_path" ]]; then
    log_error "CSR file not found on host: ${host_csr_path}"
    exit 1
  fi

  local csr_filename
  csr_filename="$(basename "$host_csr_path")"
  local container_csr_path="${CSR_CONTAINER_PATH}/${csr_filename}"

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

  # Build mint command with --csrPath
  local cmd=(docker exec "$CONTAINER_NAME" "$MIS_CLI" mint x509
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
  echo "    1) SPIFFE ID — provide a SPIFFE ID; key and certificate will be generated"
  echo "    2) CSR       — provide a CSR file; only certificate will be generated (key not created)"
  local GEN_METHOD=""
  while true; do
    read -rp "  Enter choice [1/2]: " method_choice
    case "$method_choice" in
      1) GEN_METHOD="spiffe-id"; break ;;
      2) GEN_METHOD="csr";       break ;;
      *) log_warn "Invalid choice. Please enter 1 or 2." ;;
    esac
  done
  log_info "Generation method: ${GEN_METHOD}"

  if [[ "$GEN_METHOD" == "spiffe-id" ]]; then
    _run_interactive_spiffe_id
  else
    _run_interactive_csr
  fi
}

# --- Interactive: SPIFFE ID flow ---
_run_interactive_spiffe_id() {
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
  log_section "Summary [${MODE_LABEL} — SPIFFE ID]"
  echo "  SPIFFE ID  : ${SPIFFE_ID}"
  echo "  TTL        : ${TTL} seconds"
  echo "  DNS SANs   : ${DNS_LIST:-(none)}"
  echo "  Output Dir : $(pwd)/${HOST_OUTPUT_DIR}"
  echo ""
  read -rp "  Proceed? [Y/n]: " confirm
  confirm="${confirm:-Y}"
  if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
    log_warn "Aborted by user."
    exit 0
  fi

  mint_svid "$SPIFFE_ID" "$TTL" "$DNS_LIST" "$MODE_LABEL"
  copy_certs_to_host "$MODE_LABEL"

  log_section "Done [${MODE_LABEL} — SPIFFE ID]"
  log_info "X.509 SVID certificates are available at: $(pwd)/${HOST_OUTPUT_DIR}"
}

# --- Interactive: CSR flow ---
_run_interactive_csr() {
  # CSR file path
  log_step "CSR File Path"
  echo "  Provide the path to the CSR file on this host."
  echo "  Note: Only the certificate will be generated; the private key will NOT be created."
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

  # Principal Selection (for output dir naming)
  log_step "Principal Selection"
  echo "  Select the principal this CSR belongs to:"
  echo "    1) WFM"
  echo "    2) WFM Client"
  while true; do
    read -rp "  Enter choice [1/2]: " mode_choice
    case "$mode_choice" in
      1) MODE="wfm";        MODE_LABEL="WFM";        break ;;
      2) MODE="wfm-client"; MODE_LABEL="WFM-Client";  break ;;
      *) log_warn "Invalid choice. Please enter 1 or 2." ;;
    esac
  done
  log_info "Principal selected: ${MODE_LABEL}"

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

  # Output dir: use CSR filename stem for uniqueness
  local csr_stem
  csr_stem="$(basename "${HOST_CSR_PATH%.*}")"
  HOST_OUTPUT_DIR="x509svid-${csr_stem}"

  # Summary
  log_section "Summary [${MODE_LABEL} — CSR]"
  echo "  CSR File   : ${HOST_CSR_PATH}"
  echo "  TTL        : ${TTL} seconds"
  echo "  DNS SANs   : ${DNS_LIST:-(none)}"
  echo "  Output Dir : $(pwd)/${HOST_OUTPUT_DIR}"
  echo "  Note       : Private key will NOT be generated."
  echo ""
  read -rp "  Proceed? [Y/n]: " confirm
  confirm="${confirm:-Y}"
  if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
    log_warn "Aborted by user."
    exit 0
  fi

  mint_svid_csr "$HOST_CSR_PATH" "$TTL" "$DNS_LIST" "$MODE_LABEL"
  copy_certs_to_host "$MODE_LABEL"

  log_section "Done [${MODE_LABEL} — CSR]"
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

  # Determine flow: CSR takes precedence if both are supplied; exactly one must be set
  if [[ -n "$auto_csr_path" && -n "$auto_spiffe_id" ]]; then
    log_error "Provide either --spiffe-id or --csr-path, not both."
    usage
  fi

  if [[ -z "$auto_csr_path" && -z "$auto_spiffe_id" ]]; then
    log_error "Automated mode requires either --spiffe-id or --csr-path."
    usage
  fi

  log_info "Principal  : ${MODE_LABEL}"
  log_info "TTL        : ${DEFAULT_TTL} seconds (default — 90 days)"
  log_info "DNS SANs   : ${auto_dns_list:-(none)}"

  check_container

  if [[ -n "$auto_spiffe_id" ]]; then
    # SPIFFE ID flow
    log_info "SPIFFE ID  : ${auto_spiffe_id}"
    parse_spiffe_id "$auto_spiffe_id" "$auto_mode"

    if [[ "$auto_mode" == "wfm" ]]; then
      HOST_OUTPUT_DIR="x509svid-wfm"
    else
      HOST_OUTPUT_DIR="x509svid-wfmclient"
    fi

    log_info "Output Dir : $(pwd)/${HOST_OUTPUT_DIR}"
    mint_svid "$auto_spiffe_id" "$DEFAULT_TTL" "$auto_dns_list" "$MODE_LABEL"

  else
    # CSR flow
    log_info "CSR Path   : ${auto_csr_path}"
    local csr_stem
    csr_stem="$(basename "${auto_csr_path%.*}")"
    HOST_OUTPUT_DIR="x509svid-${csr_stem}"

    log_info "Output Dir : $(pwd)/${HOST_OUTPUT_DIR}"
    mint_svid_csr "$auto_csr_path" "$DEFAULT_TTL" "$auto_dns_list" "$MODE_LABEL"
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