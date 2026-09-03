#!/usr/bin/env bash
# =============================================================================
# PKI Generator Script
# Generates: https CA, minter CA, and a server certificate signed by https CA
# =============================================================================

set -euo pipefail

# --- Constants ----------------------------------------------------------------
OUTPUT_DIR="./certs"
KEY_SIZE=4096
CA_DAYS=3650   # 10 years
CERT_DAYS=365  # 1 year

# Default values (automated mode)
DEFAULT_CN="Capgemini"
DEFAULT_C="IN"
DEFAULT_ST="Haryana"
DEFAULT_L="Gurugram"
DEFAULT_O="Capgemini"
DEFAULT_OU="Margo Sandbox Team"
DEFAULT_EMAIL="admin@capgemini.com"
DEFAULT_DNS_SAN="mis.northstarida.com"

# Output file names
HTTPS_CA_KEY="$OUTPUT_DIR/https-ca.key"
HTTPS_CA_CRT="$OUTPUT_DIR/https-ca.crt"
MINTER_CA_KEY="$OUTPUT_DIR/ca.key"
MINTER_CA_CRT="$OUTPUT_DIR/ca.crt"
SERVER_KEY="$OUTPUT_DIR/https-server.key"
SERVER_CRT="$OUTPUT_DIR/https-server.crt"
SERVER_CSR="$OUTPUT_DIR/https-server.csr"
SERVER_EXT="$OUTPUT_DIR/https-server_ext.cnf"

# --- Helpers ------------------------------------------------------------------
log()  { echo "[INFO]  $(date '+%Y-%m-%d %H:%M:%S') $*"; }
warn() { echo "[WARN]  $(date '+%Y-%m-%d %H:%M:%S') $*"; }
die()  { echo "[ERROR] $(date '+%Y-%m-%d %H:%M:%S') $*" >&2; exit 1; }

prompt() {
    local var_name="$1"
    local prompt_text="$2"
    local default_val="$3"
    local input

    read -rp "  ${prompt_text} [${default_val}]: " input
    # If user pressed Enter without input, use default
    printf -v "$var_name" '%s' "${input:-$default_val}"
}

# --- Preflight ----------------------------------------------------------------
check_dependencies() {
    log "Checking dependencies..."
    if ! command -v openssl &>/dev/null; then
        die "openssl is not installed or not in PATH. Please install it and retry."
    fi
    log "openssl found: $(openssl version)"
}

prepare_output_dir() {
    log "Preparing output directory: $OUTPUT_DIR"
    mkdir -p "$OUTPUT_DIR"
    chmod 700 "$OUTPUT_DIR"
}

# --- Input collection ---------------------------------------------------------
collect_interactive_inputs() {
    echo ""
    echo "============================================================"
    echo "  Interactive PKI Configuration"
    echo "============================================================"
    echo "  Press Enter to accept the default value shown in brackets."
    echo ""

    prompt CN          "Common Name (CN)"                  "$DEFAULT_CN"
    prompt C           "Country (C, 2-letter ISO code)"    "$DEFAULT_C"
    prompt ST          "State / Province (ST)"             "$DEFAULT_ST"
    prompt L           "Locality / City (L)"               "$DEFAULT_L"
    prompt O           "Organization (O)"                  "$DEFAULT_O"
    prompt OU          "Organizational Unit (OU)"          "$DEFAULT_OU"
    prompt EMAIL       "Email Address"                     "$DEFAULT_EMAIL"
    prompt CA_VALIDITY "CA validity in days (10yr=3650)"   "$CA_DAYS"
    prompt SRV_VALIDITY "Server cert validity in days"     "$CERT_DAYS"
    prompt DNS_SAN     "Server DNS SAN"                    "$DEFAULT_DNS_SAN"

    echo ""
    log "Configuration collected."
}

collect_automated_inputs() {
    log "Using automated (default) configuration."
    CN="$DEFAULT_CN"
    C="$DEFAULT_C"
    ST="$DEFAULT_ST"
    L="$DEFAULT_L"
    O="$DEFAULT_O"
    OU="$DEFAULT_OU"
    EMAIL="$DEFAULT_EMAIL"
    CA_VALIDITY="$CA_DAYS"
    SRV_VALIDITY="$CERT_DAYS"
    DNS_SAN="${dns_override:-$DEFAULT_DNS_SAN}"   
}

# --- Certificate generation ---------------------------------------------------
build_subject() {
    # $1 = CN override (optional)
    local cn="${1:-$CN}"
    echo "/C=${C}/ST=${ST}/L=${L}/O=${O}/OU=${OU}/CN=${cn}/emailAddress=${EMAIL}"
}

generate_https_ca() {
    log "--- Generating HTTPS CA key (RSA-${KEY_SIZE}) ---"
    openssl genrsa -out "$HTTPS_CA_KEY" "$KEY_SIZE"
    chmod 600 "$HTTPS_CA_KEY"
    log "HTTPS CA key written to: $HTTPS_CA_KEY"

    log "--- Generating HTTPS CA self-signed certificate (${CA_VALIDITY} days) ---"
    openssl req -new -x509 \
        -key    "$HTTPS_CA_KEY" \
        -out    "$HTTPS_CA_CRT" \
        -days   "$CA_VALIDITY" \
        -subj   "$(build_subject "Capgemini HTTPS CA")" \
        -extensions v3_ca \
        -addext "basicConstraints=critical,CA:TRUE" \
        -addext "keyUsage=critical,keyCertSign,cRLSign" \
        -addext "subjectKeyIdentifier=hash"
    log "HTTPS CA certificate written to: $HTTPS_CA_CRT"
}

generate_minter_ca() {
    log "--- Generating Minter CA key (RSA-${KEY_SIZE}) ---"
    openssl genrsa -out "$MINTER_CA_KEY" "$KEY_SIZE"
    chmod 600 "$MINTER_CA_KEY"
    log "Minter CA key written to: $MINTER_CA_KEY"

    log "--- Generating Minter CA self-signed certificate (${CA_VALIDITY} days) ---"
    openssl req -new -x509 \
        -key    "$MINTER_CA_KEY" \
        -out    "$MINTER_CA_CRT" \
        -days   "$CA_VALIDITY" \
        -subj   "$(build_subject "Capgemini Minter CA")" \
        -extensions v3_ca \
        -addext "basicConstraints=critical,CA:TRUE" \
        -addext "keyUsage=critical,keyCertSign,cRLSign" \
        -addext "subjectKeyIdentifier=hash"
    log "Minter CA certificate written to: $MINTER_CA_CRT"
}

generate_server_cert() {
    log "--- Generating Server key (RSA-${KEY_SIZE}) ---"
    openssl genrsa -out "$SERVER_KEY" "$KEY_SIZE"
    chmod 600 "$SERVER_KEY"
    log "Server key written to: $SERVER_KEY"

    log "--- Generating Server CSR ---"
    openssl req -new \
        -key  "$SERVER_KEY" \
        -out  "$SERVER_CSR" \
        -subj "$(build_subject "$DNS_SAN")"
    log "Server CSR written to: $SERVER_CSR"

    log "--- Writing SAN extension config ---"
    cat > "$SERVER_EXT" <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions     = v3_req
prompt             = no

[req_distinguished_name]
C  = ${C}
ST = ${ST}
L  = ${L}
O  = ${O}
OU = ${OU}
CN = ${DNS_SAN}

[v3_req]
basicConstraints     = CA:FALSE
keyUsage             = critical, digitalSignature, keyEncipherment
extendedKeyUsage     = serverAuth
subjectAltName       = @alt_names

[alt_names]
DNS.1 = ${DNS_SAN}
EOF

    log "--- Signing Server certificate with HTTPS CA (${SRV_VALIDITY} days) ---"
    openssl x509 -req \
        -in         "$SERVER_CSR" \
        -CA         "$HTTPS_CA_CRT" \
        -CAkey      "$HTTPS_CA_KEY" \
        -CAcreateserial \
        -out        "$SERVER_CRT" \
        -days       "$SRV_VALIDITY" \
        -extfile    "$SERVER_EXT" \
        -extensions v3_req
    log "Server certificate written to: $SERVER_CRT"

    # Clean up temporary files
    rm -f "$SERVER_CSR" "$SERVER_EXT" "$OUTPUT_DIR/https-ca.srl"
    log "Temporary CSR and extension files removed."
}

# --- Verification -------------------------------------------------------------
verify_certificates() {
    log "============================================================"
    log "Verifying generated certificates..."
    log "============================================================"

    log ">> HTTPS CA certificate details:"
    openssl x509 -in "$HTTPS_CA_CRT" -noout -subject -issuer -dates -fingerprint -sha256

    echo ""
    log ">> Minter CA certificate details:"
    openssl x509 -in "$MINTER_CA_CRT" -noout -subject -issuer -dates -fingerprint -sha256

    echo ""
    log ">> Server certificate details:"
    openssl x509 -in "$SERVER_CRT" -noout -subject -issuer -dates -ext subjectAltName

    echo ""
    log ">> Verifying server certificate chain against HTTPS CA:"
    openssl verify -CAfile "$HTTPS_CA_CRT" "$SERVER_CRT" \
        && log "Chain verification: OK" \
        || warn "Chain verification FAILED"
}

# --- Summary ------------------------------------------------------------------
print_summary() {
    echo ""
    echo "============================================================"
    echo "  Generated Files Summary"
    echo "============================================================"
    printf "  %-30s %s\n" "HTTPS CA Key:"        "$HTTPS_CA_KEY"
    printf "  %-30s %s\n" "HTTPS CA Certificate:" "$HTTPS_CA_CRT"
    printf "  %-30s %s\n" "Minter CA Key:"       "$MINTER_CA_KEY"
    printf "  %-30s %s\n" "Minter CA Certificate:" "$MINTER_CA_CRT"
    printf "  %-30s %s\n" "Server Key:"          "$SERVER_KEY"
    printf "  %-30s %s\n" "Server Certificate:"  "$SERVER_CRT"
    echo "============================================================"
    echo ""
    log "All done!"
}

# --- Usage --------------------------------------------------------------------
usage() {
    cat <<EOF
Usage: $(basename "$0") [--interactive | --automated] [--dns <DNS_SAN>]

Modes:
  --interactive   Prompt for all certificate fields interactively.
  --automated     Use built-in defaults (no prompts).

Options:
  --dns <value>   Override the server DNS SAN (used in automated mode).
                  Falls back to default: $DEFAULT_DNS_SAN

If no mode is specified, the script will ask you to choose.
EOF
    exit 0
}

# --- Main ---------------------------------------------------------------------
main() {
    local mode=""
    local dns_override=""

    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --interactive) mode="interactive" ;;
            --automated)   mode="automated"   ;;
            --dns)
                shift
                [[ -z "${1:-}" ]] && die "--dns requires a value."
                dns_override="$1"
                ;;
            --help|-h) usage ;;
            *) die "Unknown argument: $1. Use --interactive, --automated, or --dns <value>." ;;
        esac
        shift
    done
    echo ""
    echo "============================================================"
    echo "  PKI Generator — HTTPS CA, Minter CA & Server Certificate"
    echo "============================================================"
    echo ""

    check_dependencies
    prepare_output_dir

    # If mode not set via flag, ask the user
    if [[ -z "$mode" ]]; then
        echo "Select mode:"
        echo "  1) interactive"
        echo "  2) automated"
        read -rp "  Choice [1/2]: " choice
        case "$choice" in
            1|interactive) mode="interactive" ;;
            2|automated)   mode="automated"   ;;
            *) die "Invalid choice. Exiting." ;;
        esac
    fi

    if [[ "$mode" == "interactive" ]]; then
        collect_interactive_inputs
    else
        collect_automated_inputs
    fi

    # Override CA_VALIDITY / SRV_VALIDITY with collected values
    CA_DAYS="$CA_VALIDITY"
    CERT_DAYS="$SRV_VALIDITY"

    generate_https_ca
    echo ""
    generate_minter_ca
    echo ""
    generate_server_cert
    echo ""
    verify_certificates
    print_summary
}

main "$@"





