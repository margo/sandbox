#!/usr/bin/env bash
# =============================================================================
# csr_gen.sh
# Generates an ECDSA P-256 private key and a Certificate Signing Request (CSR)
# suitable for minting SPIFFE SVIDs for Margo Principals.
#
# Renewal behaviour:
#   If a valid ECDSA key already exists at KEY_OUT, it is reused and only a
#   fresh CSR is generated (key is NOT regenerated).
#
# SPIFFE ID:
#   Optional. Per MIAF2 §3, the SPIFFE ID in the CSR is advisory — the issuing
#   CA (MIS) is authoritative and may override it at SVID minting time.
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

# SPIFFE ID — optional per MIAF2 §3 (advisory; CA overrides at minting)
SPIFFE_ID=""

# -----------------------------------------------------------------------------
# Helpers
# -----------------------------------------------------------------------------
log() { echo "[INFO]  $*"; }
warn() { echo "[WARN]  $*"; }
err() { echo "[ERROR] $*" >&2; exit 1; }

# -----------------------------------------------------------------------------
# Help
# -----------------------------------------------------------------------------
usage() {
    cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Generates an ECDSA P-256 private key and a CSR suitable for minting SVIDs
for Margo Principals.

Renewal mode (automatic):
  If a valid ECDSA key already exists at --key-out path, the existing key is
  reused and only a fresh CSR is generated. The private key is NOT regenerated.
  This allows renewal of an SVID without changing the key pair.

SPIFFE ID (optional):
  Per MIAF2 §3, the SPIFFE ID embedded in the CSR is advisory. The issuing CA
  (MIS) is authoritative and may override it at SVID minting time. If omitted,
  no URI SAN is embedded in the CSR.

Optional:
  --spiffe-id <id>       SPIFFE ID URI for the principal (advisory)
                         e.g. spiffe://margo.org/margo/wfm/my-wfm/client/my-client
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
  # Fresh issuance — no SPIFFE ID (CA assigns identity at minting)
  $(basename "$0")

  # Fresh issuance — advisory SPIFFE ID
  $(basename "$0") --spiffe-id spiffe://margo.org/margo/wfm/my-wfm/client/my-client

  # Renewal — reuses existing key at default path, generates fresh CSR
  $(basename "$0") --key-out ./config/identity/payload-key.pem

  # Renewal — reuses existing key, carries current SPIFFE ID as advisory hint
  $(basename "$0") \
      --key-out ./config/identity/payload-key.pem \
      --spiffe-id spiffe://margo.org/margo/wfm/my-wfm/client/my-client

Notes:
  • SPIFFE ID, if provided, must match:
      spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<wfm-client-id>
  • The SPIFFE ID in the CSR is advisory — the MIS overrides it at minting.
  • Renewal does not change the key pair; only the CSR (and resulting SVID) is refreshed.
EOF
    exit 0
}

# -----------------------------------------------------------------------------
# Argument parsing
# -----------------------------------------------------------------------------
parse_args() {
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

    # Validate SPIFFE ID format only if provided (it is optional)
    if [[ -n "${SPIFFE_ID}" ]]; then
        [[ "${SPIFFE_ID}" == spiffe://* ]] \
            || err "SPIFFE ID must start with 'spiffe://'. Got: '${SPIFFE_ID}'"

        [[ "${SPIFFE_ID}" =~ ^spiffe://[^/]+/margo/wfm/[^/]+/client/[^/]+$ ]] \
            || err "SPIFFE ID does not match required format:
       spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<wfm-client-id>
       Example: spiffe://margo.org/margo/wfm/my-wfm/client/my-client
       Got: '${SPIFFE_ID}'"
    fi
}

# -----------------------------------------------------------------------------
# Key handling — generate new or reuse existing
# Returns 0 (reuse) or 1 (generated fresh) via RENEWAL flag
# -----------------------------------------------------------------------------
RENEWAL=false

handle_key() {
    if [[ -f "${KEY_OUT}" ]]; then
        # Validate the existing file is a recognisable ECDSA private key
        if openssl pkey -in "${KEY_OUT}" -noout 2>/dev/null \
           && openssl pkey -in "${KEY_OUT}" -noout -text 2>/dev/null | grep -q "NIST CURVE"; then
            RENEWAL=true
            warn "Existing valid ECDSA key found at: ${KEY_OUT}"
            warn "Reusing key for renewal — private key will NOT be regenerated."
            warn "A fresh CSR will be generated against the existing key."
        else
            err "File exists at '${KEY_OUT}' but is not a valid ECDSA key. Remove or rename it before proceeding."
        fi
    else
        log "No existing key found at '${KEY_OUT}'. Generating new ECDSA P-256 key..."
        openssl genpkey \
            -algorithm EC \
            -pkeyopt ec_paramgen_curve:P-256 \
            -out "${KEY_OUT}"
        log "Key generated successfully."
    fi

    # Always confirm the key in use is ECDSA P-256
    openssl pkey -in "${KEY_OUT}" -noout -text 2>/dev/null | grep "NIST CURVE"
}

# -----------------------------------------------------------------------------
# Main
# -----------------------------------------------------------------------------
main() {
    parse_args "$@"

    command -v openssl &>/dev/null || err "openssl not found. Install it with: sudo apt install openssl"

    # -------------------------------------------------------------------------
    # Step 1: Handle key — generate fresh or reuse for renewal
    # -------------------------------------------------------------------------
    handle_key

    # -------------------------------------------------------------------------
    # Step 2: Build SAN entries
    # URI SAN for SPIFFE ID is optional (advisory per MIAF2 §3)
    # -------------------------------------------------------------------------
    SAN_STRING=""
    if [[ -n "${SPIFFE_ID}" ]]; then
        SAN_STRING="URI.1 = ${SPIFFE_ID}"
        warn "SPIFFE ID embedded in CSR is advisory. The MIS (CA) is authoritative"
        warn "and may override this identity at SVID minting time."
    else
        log "No SPIFFE ID provided. CSR will carry no URI SAN."
        log "The MIS (CA) will assign the SPIFFE ID at SVID minting time."
    fi

    # -------------------------------------------------------------------------
    # Step 3: Write temporary OpenSSL config
    # -------------------------------------------------------------------------
    OPENSSL_CNF=$(mktemp /tmp/openssl_csr_XXXXXX.cnf)
    trap 'rm -f "${OPENSSL_CNF}"' EXIT

    # Build the req_extensions block conditionally
    if [[ -n "${SAN_STRING}" ]]; then
        REQ_EXT_BLOCK="req_extensions      = req_ext"
        ALT_NAMES_SECTION="
[ req_ext ]
subjectAltName      = @alt_names
basicConstraints    = CA:FALSE
keyUsage            = critical, digitalSignature
extendedKeyUsage    = clientAuth, serverAuth

[ alt_names ]
${SAN_STRING}"
    else
        REQ_EXT_BLOCK=""
        ALT_NAMES_SECTION="
[ req_ext ]
basicConstraints    = CA:FALSE
keyUsage            = critical, digitalSignature
extendedKeyUsage    = clientAuth, serverAuth"
    fi

    cat > "${OPENSSL_CNF}" <<EOF
[ req ]
prompt              = no
default_md          = sha256
distinguished_name  = dn
${REQ_EXT_BLOCK}

[ dn ]
C                   = ${COUNTRY}
ST                  = ${STATE}
L                   = ${LOCALITY}
O                   = ${ORGANIZATION}
OU                  = ${ORG_UNIT}
CN                  = ${COMMON_NAME}
emailAddress        = ${EMAIL}
${ALT_NAMES_SECTION}
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
        | grep -E "Subject:|Public Key Algorithm:|NIST CURVE:|URI:" || true

    # -------------------------------------------------------------------------
    # Summary
    # -------------------------------------------------------------------------
    echo ""
    echo "============================================="
    if [[ "${RENEWAL}" == true ]]; then
        echo "  Mode        : RENEWAL (existing key reused)"
    else
        echo "  Mode        : FRESH ISSUANCE (new key generated)"
    fi
    if [[ -n "${SPIFFE_ID}" ]]; then
        echo "  SPIFFE ID   : ${SPIFFE_ID} (advisory)"
    else
        echo "  SPIFFE ID   : not set (CA assigns at minting)"
    fi
    echo "  Private Key : ${KEY_OUT}"
    echo "  CSR         : ${CSR_OUT}"
    echo ""
    echo "  Next steps:"
    echo "    • Submit ${CSR_OUT} to your MIS to mint an SVID"
    if [[ "${RENEWAL}" == true ]]; then
        echo "    • Place the returned SVID at your certPath (replaces existing cert only)"
    else
        echo "    • Place the returned SVID and this key at your configured certPath/keyPath"
    fi
    echo "============================================="
}

main "$@"