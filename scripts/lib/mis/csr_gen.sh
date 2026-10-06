#!/usr/bin/env bash
# =============================================================================
# csr_gen.sh
# Generates an ECDSA P-256 private key and a Certificate Signing Request (CSR)
# suitable for minting SPIFFE SVIDs for Margo Principals.
# =============================================================================

set -euo pipefail

# -----------------------------------------------------------------------------
# CONFIGURATION — edit defaults below or override via flags
# -----------------------------------------------------------------------------
KEY_OUT="payload-key.pem"
CSR_OUT="payload-req.csr"

COUNTRY="IN"
STATE="Haryana"
LOCALITY="Gurugram"
ORGANIZATION="Margo"
ORG_UNIT="Margo Sandbox"
COMMON_NAME="mis.margo.org"
EMAIL="operator@margo.org"

# SPIFFE ID — must be provided via --spiffe-id flag
SPIFFE_ID=""

# -----------------------------------------------------------------------------
# Helpers
# -----------------------------------------------------------------------------
log() { echo "[INFO]  $*"; }
err() { echo "[ERROR] $*" >&2; exit 1; }

# -----------------------------------------------------------------------------
# Help
# -----------------------------------------------------------------------------
usage() {
    cat <<EOF
Usage: $(basename "$0") --spiffe-id <SPIFFE_ID> [OPTIONS]

Generates an ECDSA P-256 private key and a CSR with a SPIFFE ID embedded
as a URI Subject Alternative Name (URI SAN), suitable for minting SVIDs
for Margo Principals.

Required:
  --spiffe-id <id>       SPIFFE ID URI for the principal
                         e.g. spiffe://margo.org/margo/wfm/my-wfm

Optional:
  --cn <common_name>     Common Name for the certificate  (default: ${COMMON_NAME})
  --org <organization>   Organization name                (default: ${ORGANIZATION})
  --ou <org_unit>        Organizational Unit              (default: ${ORG_UNIT})
  --country <code>       2-letter country code            (default: ${COUNTRY})
  --state <state>        State or province                (default: ${STATE})
  --locality <city>      Locality / city                  (default: ${LOCALITY})
  --email <email>        Email address                    (default: ${EMAIL})
  --key-out <file>       Output path for private key      (default: ${KEY_OUT})
  --csr-out <file>       Output path for CSR              (default: ${CSR_OUT})
  --help                 Show this help message and exit

Examples:
  # Minimal — only SPIFFE ID required
  $(basename "$0") --spiffe-id spiffe://margo.org/margo/wfm/my-wfm

  # Custom output paths
  $(basename "$0") \
      --spiffe-id spiffe://margo.org/margo/wfm/my-wfm \
      --key-out /tmp/svid.key \
      --csr-out /tmp/svid.csr

Notes:
  • The SPIFFE ID must start with 'spiffe://'
  • The generated CSR can be submitted to a MIS for minting SVID against that CSR.
EOF
    exit 0
}

# -----------------------------------------------------------------------------
# Argument parsing
# -----------------------------------------------------------------------------
parse_args() {
    if [[ $# -eq 0 ]]; then
        err "No arguments provided. Run '$(basename "$0") --help' for usage."
    fi

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --help)
                usage
                ;;
            --spiffe-id)
                [[ $# -gt 1 ]] || err "--spiffe-id requires a value."
                SPIFFE_ID="$2"; shift 2
                ;;
            --cn)
                [[ $# -gt 1 ]] || err "--cn requires a value."
                COMMON_NAME="$2"; shift 2
                ;;
            --org)
                [[ $# -gt 1 ]] || err "--org requires a value."
                ORGANIZATION="$2"; shift 2
                ;;
            --ou)
                [[ $# -gt 1 ]] || err "--ou requires a value."
                ORG_UNIT="$2"; shift 2
                ;;
            --country)
                [[ $# -gt 1 ]] || err "--country requires a value."
                COUNTRY="$2"; shift 2
                ;;
            --state)
                [[ $# -gt 1 ]] || err "--state requires a value."
                STATE="$2"; shift 2
                ;;
            --locality)
                [[ $# -gt 1 ]] || err "--locality requires a value."
                LOCALITY="$2"; shift 2
                ;;
            --email)
                [[ $# -gt 1 ]] || err "--email requires a value."
                EMAIL="$2"; shift 2
                ;;
            --key-out)
                [[ $# -gt 1 ]] || err "--key-out requires a value."
                KEY_OUT="$2"; shift 2
                ;;
            --csr-out)
                [[ $# -gt 1 ]] || err "--csr-out requires a value."
                CSR_OUT="$2"; shift 2
                ;;
            *)
                err "Unknown argument: $1. Run '$(basename "$0") --help' for usage."
                ;;
        esac
    done

    # Validate required args
    [[ -n "${SPIFFE_ID}" ]] || err "--spiffe-id is required. Run '$(basename "$0") --help' for usage."

    # Validate SPIFFE ID format
    [[ "${SPIFFE_ID}" == spiffe://* ]] || err "SPIFFE ID must start with 'spiffe://'. Got: '${SPIFFE_ID}'"
}

# -----------------------------------------------------------------------------
# Main
# -----------------------------------------------------------------------------
main() {
    parse_args "$@"

    command -v openssl &>/dev/null || err "openssl not found. Install it with: sudo apt install openssl"

    # -------------------------------------------------------------------------
    # Step 1: Generate ECDSA P-256 private key
    # -------------------------------------------------------------------------
    log "Generating ECDSA P-256 private key → ${KEY_OUT}"
    openssl genpkey \
        -algorithm EC \
        -pkeyopt ec_paramgen_curve:P-256 \
        -out "${KEY_OUT}"

    log "Key generated successfully."
    openssl pkey -in "${KEY_OUT}" -noout -text 2>/dev/null | grep "NIST CURVE"

    # -------------------------------------------------------------------------
    # Step 2: Build SAN entries
    # URI SAN for SPIFFE ID is mandatory
    # -------------------------------------------------------------------------
    san_entries=("URI.1 = ${SPIFFE_ID}")
    SAN_STRING=$(printf "%s\n" "${san_entries[@]}")

    # -------------------------------------------------------------------------
    # Step 3: Write temporary OpenSSL config
    # -------------------------------------------------------------------------
    OPENSSL_CNF=$(mktemp /tmp/openssl_csr_XXXXXX.cnf)
    trap 'rm -f "${OPENSSL_CNF}"' EXIT

    cat > "${OPENSSL_CNF}" <<EOF
[ req ]
prompt              = no
default_md          = sha256
distinguished_name  = dn
req_extensions      = req_ext

[ dn ]
C                   = ${COUNTRY}
ST                  = ${STATE}
L                   = ${LOCALITY}
O                   = ${ORGANIZATION}
OU                  = ${ORG_UNIT}
CN                  = ${COMMON_NAME}
emailAddress        = ${EMAIL}

[ req_ext ]
subjectAltName      = @alt_names
basicConstraints    = CA:FALSE
keyUsage            = critical, digitalSignature
extendedKeyUsage    = clientAuth, serverAuth

[ alt_names ]
${SAN_STRING}
EOF

    # -------------------------------------------------------------------------
    # Step 4: Generate CSR
    # -------------------------------------------------------------------------
    log "Generating CSR → ${CSR_OUT}"
    openssl req \
        -new \
        -key "${KEY_OUT}" \
        -out "${CSR_OUT}" \
        -config "${OPENSSL_CNF}"

    # -------------------------------------------------------------------------
    # Step 5: Verify CSR
    # -------------------------------------------------------------------------
    log "Verifying CSR..."
    openssl req -in "${CSR_OUT}" -noout -verify -key "${KEY_OUT}"
    openssl req -in "${CSR_OUT}" -noout -text \
        | grep -E "Subject:|Public Key Algorithm:|NIST CURVE:|URI:"

    # -------------------------------------------------------------------------
    # Summary
    # -------------------------------------------------------------------------
    echo ""
    echo "============================================="
    echo "  SPIFFE ID   : ${SPIFFE_ID}"
    echo "  Private Key : ${KEY_OUT}"
    echo "  CSR         : ${CSR_OUT}"
    echo "============================================="
}

main "$@"