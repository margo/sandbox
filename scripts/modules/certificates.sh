#!/bin/bash

# Certificate generation functions
source "$(dirname "${BASH_SOURCE[0]}")/../lib/common.sh"  # Changed from ../../ to ../


collect_certs_info() {
    echo "Collecting certificate information..."
    CN="${EXPOSED_SYMPHONY_HOST:-localhost}"
    C="IN"
    ST="GGN"
    L="Some ABC Location"
    O="Margo"
    EMAIL="admin@example.com"
    DAYS="365"
    if [[ $CN =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
      SAN_IPS="${EXPOSED_SYMPHONY_HOST:-127.0.0.1}"
    else
      SAN_DOMAINS="${EXPOSED_SYMPHONY_HOST:-localhost}"
    fi

    echo "Using certificate defaults with CN: $CN"
}

generate_config_for_certs() {
    local config_file="$1"
    local cert_type="$2"

    cat > "$config_file" << EOF
[req]
default_bits = 2048
prompt = no
distinguished_name = dn
$([ "$cert_type" = "server" ] && echo "req_extensions = v3_req")

[dn]
C=$C
ST=$ST
L=$L
O=$O
CN=$CN
emailAddress=$EMAIL

[v3_req]
basicConstraints = CA:TRUE
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
EOF

    local dns_count=1
    local ip_count=1

    if [ -n "$SAN_DOMAINS" ]; then
        echo "Adding SAN domains..."
        IFS=',' read -ra DOMAINS <<< "$SAN_DOMAINS"
        for domain in "${DOMAINS[@]}"; do
            echo "DNS.$dns_count = ${domain// /}" >> "$config_file"
            ((dns_count++))
        done
    fi

    if [ -n "$SAN_IPS" ]; then
        echo "Adding SAN IPs..."
        IFS=',' read -ra IPS <<< "$SAN_IPS"
        for ip in "${IPS[@]}"; do
            echo "IP.$ip_count = ${ip// /}" >> "$config_file"
            ((ip_count++))
        done
    fi

    echo "Generated OpenSSL config at $config_file:"
    cat "$config_file"
}

generate_ca() {
    info "Generating CA certificate..."
    local ca_key="$CERT_DIR/ca-key.pem"
    local ca_cert="$CERT_DIR/ca-cert.pem"
    local ca_config="$CERT_DIR/ca.conf"

    generate_config_for_certs "$ca_config" "ca"

    openssl genrsa -out "$ca_key" 2048
    openssl req -new -x509 -key "$ca_key" -out "$ca_cert" -days "$DAYS" -config "$ca_config"
    chmod 600 "$ca_key"

    success "CA generated: $ca_cert"
}

generate_server_certs() {
    echo "Generating server certificate..."
    if ! mkdir -p "$CERT_DIR"; then
        echo "Error: Failed to create directory $CERT_DIR"
        return 1
    fi

    if [[ ! -w "$CERT_DIR" ]]; then
        echo "Error: Cannot write to $CERT_DIR"
        return 1
    fi
    
    local server_key="$CERT_DIR/server-key.pem"
    local server_csr="$CERT_DIR/server.csr"
    local server_cert="$CERT_DIR/server-cert.pem"
    local server_config="$CERT_DIR/server.conf"
    
    generate_ca
    generate_config_for_certs "$server_config" "server"

    openssl genrsa -out "$server_key" 2048
    openssl req -new -key "$server_key" -out "$server_csr" -config "$server_config"

    if [[ -f "$CERT_DIR/ca-cert.pem" ]]; then
        openssl x509 -req -in "$server_csr" -CA "$CERT_DIR/ca-cert.pem" -CAkey "$CERT_DIR/ca-key.pem" \
            -CAcreateserial -out "$server_cert" -days "$DAYS" -extensions v3_req -extfile "$server_config"
        success "Server certificate signed by CA: $server_cert"
    else
        openssl x509 -req -in "$server_csr" -signkey "$server_key" -out "$server_cert" -days "$DAYS" \
            -extensions v3_req -extfile "$server_config"
        success "Self-signed server certificate: $server_cert"
    fi

    rm -f "$server_csr"
    chmod 600 "$server_key"
}
