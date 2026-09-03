#!/usr/bin/env bash

set -euo pipefail

OUTPUT_FILE="$(pwd)/configuration.json"

# ── Defaults ────────────────────────────────────────────────────────────────
DEFAULT_TRUST_DOMAIN="margo.org"
DEFAULT_TRUST_BUNDLE_URI=".well-known/spiffe/bundle.json"
DEFAULT_LOG_LEVEL="info"
DEFAULT_CA_CERT="./ca.crt"
DEFAULT_CA_KEY="./ca.key"
DEFAULT_HTTPS_ADDR=":8443"
DEFAULT_HTTPS_CA="./https-ca.crt"
DEFAULT_HTTPS_CERT="./https-server.crt"
DEFAULT_HTTPS_KEY="./https-server.key"

# ── Helpers ──────────────────────────────────────────────────────────────────
prompt() {
  local var_name="$1"
  local prompt_text="$2"
  local default="$3"
  local example="$4"

  echo ""
  echo "  Example: ${example}"
  read -rp "  ${prompt_text} [default: ${default}]: " input
  if [[ -z "${input}" ]]; then
    eval "${var_name}='${default}'"
  else
    eval "${var_name}='${input}'"
  fi
}

write_config() {
  cat > "${OUTPUT_FILE}" <<EOF
{
  "trustDomain": "${TRUST_DOMAIN}",
  "trustBundleURI": "${TRUST_BUNDLE_URI}",
  "log": {
    "level": "${LOG_LEVEL}"
  },
  "ca": {
    "cert": "${CA_CERT}",
    "key": "${CA_KEY}"
  },
  "https": {
    "addr": "${HTTPS_ADDR}",
    "ca": "${HTTPS_CA}",
    "cert": "${HTTPS_CERT}",
    "key": "${HTTPS_KEY}"
  }
}
EOF
}

# ── Mode selection ───────────────────────────────────────────────────────────
usage() {
  echo "Usage: $0 [--interactive | --automated]"
  echo ""
  echo "  --interactive   Prompt for each configuration value (with defaults)"
  echo "  --automated     Write configuration.json using built-in defaults"
  exit 1
}

if [[ $# -ne 1 ]]; then
  usage
fi

MODE="$1"

case "${MODE}" in

  # ── Automated ──────────────────────────────────────────────────────────────
  --automated)
    echo "Running in automated mode — using defaults..."
    TRUST_DOMAIN="${DEFAULT_TRUST_DOMAIN}"
    TRUST_BUNDLE_URI="${DEFAULT_TRUST_BUNDLE_URI}"
    LOG_LEVEL="${DEFAULT_LOG_LEVEL}"
    CA_CERT="${DEFAULT_CA_CERT}"
    CA_KEY="${DEFAULT_CA_KEY}"
    HTTPS_ADDR="${DEFAULT_HTTPS_ADDR}"
    HTTPS_CA="${DEFAULT_HTTPS_CA}"
    HTTPS_CERT="${DEFAULT_HTTPS_CERT}"
    HTTPS_KEY="${DEFAULT_HTTPS_KEY}"
    ;;

  # ── Interactive ────────────────────────────────────────────────────────────
  --interactive)
    echo "Running in interactive mode — press Enter to accept the default."
    echo "============================================================"

    prompt TRUST_DOMAIN \
      "Trust Domain" \
      "${DEFAULT_TRUST_DOMAIN}" \
      "margo.org, example.org, mycompany.io"

    prompt TRUST_BUNDLE_URI \
      "Trust Bundle URI" \
      "${DEFAULT_TRUST_BUNDLE_URI}" \
      ".well-known/spiffe/bundle.json"

    prompt LOG_LEVEL \
      "Log Level" \
      "${DEFAULT_LOG_LEVEL}" \
      "debug, info, warn, error"

    prompt CA_CERT \
      "CA Certificate path" \
      "${DEFAULT_CA_CERT}" \
      "./ca.crt, /etc/pki/ca/ca.crt, ./data/ca/current.crt"

    prompt CA_KEY \
      "CA Private Key path" \
      "${DEFAULT_CA_KEY}" \
      "./ca.key, /etc/pki/ca/ca.key, ./data/ca/current.key"

    prompt HTTPS_ADDR \
      "HTTPS Listen Address" \
      "${DEFAULT_HTTPS_ADDR}" \
      ":8443, :443, 0.0.0.0:8443"

    prompt HTTPS_CA \
      "HTTPS CA Certificate path" \
      "${DEFAULT_HTTPS_CA}" \
      "./https-ca.crt, ./ca.crt"

    prompt HTTPS_CERT \
      "HTTPS Server Certificate path" \
      "${DEFAULT_HTTPS_CERT}" \
      "./https-server.crt, ./server.crt"

    prompt HTTPS_KEY \
      "HTTPS Server Private Key path" \
      "${DEFAULT_HTTPS_KEY}" \
      "./https-server.key, ./server.key"
    ;;

  *)
    usage
    ;;
esac

# ── Write & report ───────────────────────────────────────────────────────────
write_config

echo ""
echo "✅ configuration.json written successfully."
echo "📄 Full path: ${OUTPUT_FILE}"