#!/bin/bash

# Harbor-specific functions (production-ready)

source "$(dirname "${BASH_SOURCE[0]}")/../lib/common.sh"

HARBOR_DIR="$HOME/sandbox/scripts/harbor"
CERT_DIR="$HARBOR_DIR/certs"
HARBOR_CERT="$CERT_DIR/harbor.crt"

# ------------------------------------------------------------------------------
# Wait for Harbor readiness (NO GUESSING via sleep)
# ------------------------------------------------------------------------------
wait_for_harbor() {
  echo "⏳ Waiting for Harbor to be ready..."

  for i in {1..60}; do
    if curl -sk https://${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}/v2/ | grep -q "errors"; then
      echo "✅ Harbor is ready"
      return 0
    fi
    sleep 2
  done

  echo "❌ Harbor failed to become ready"
  return 1
}

# ------------------------------------------------------------------------------
# systemd service (FIXED)
# ------------------------------------------------------------------------------
create_harbor_systemd_service() {
  echo "🔧 Creating Harbor systemd service..."

  sudo tee /etc/systemd/system/harbor.service > /dev/null <<EOF
[Unit]
Description=Harbor Container Registry
Requires=docker.service
After=docker.service network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=${HARBOR_DIR}
ExecStart=/usr/bin/docker compose up -d
ExecStartPost=/bin/bash -c '${HARBOR_DIR}/wait_ready.sh'
ExecStop=/usr/bin/docker compose down

# Restart if something goes wrong
Restart=on-failure
RestartSec=10

TimeoutStartSec=0

[Install]
WantedBy=multi-user.target
EOF

  # Create helper readiness script
  cat > "${HARBOR_DIR}/wait_ready.sh" <<EOS
#!/bin/bash
for i in {1..60}; do
  if curl -sk https://${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}/v2/ | grep -q "errors"; then
    exit 0
  fi
  sleep 2
done
exit 1
EOS

  chmod +x "${HARBOR_DIR}/wait_ready.sh"

  sudo systemctl daemon-reload
  sudo systemctl enable harbor.service

  echo "✅ Harbor service created"
}

# ------------------------------------------------------------------------------
# Configure restart policy
# ------------------------------------------------------------------------------
configure_harbor_restart_policy() {
  local compose_file="$HARBOR_DIR/docker-compose.yml"

  if [ ! -f "$compose_file" ]; then
    return 0
  fi

  sed -i 's/^\s*restart:\s*always/    restart: unless-stopped/g' "$compose_file"
  echo "✅ Restart policy set to unless-stopped"
}

# ------------------------------------------------------------------------------
# Cert generation (FIXED: no regeneration)
# ------------------------------------------------------------------------------
generate_certs_if_needed() {
  mkdir -p "$CERT_DIR"

  if [ -f "$HARBOR_CERT" ]; then
    echo "✅ Existing Harbor cert found (reusing)"
    return
  fi

  echo "📜 Generating Harbor certificate..."

  cat > "$CERT_DIR/harbor-cert.conf" <<EOF
[req]
default_bits = 4096
prompt = no
default_md = sha256
distinguished_name = dn
req_extensions = v3_req

[dn]
C=IN
ST=GGN
L=Sector 48
O=Margo
CN=${EXPOSED_HARBOR_HOST}

[v3_req]
basicConstraints = CA:TRUE
keyUsage = keyEncipherment, digitalSignature
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = ${EXPOSED_HARBOR_HOST}
DNS.2 = localhost
IP.1 = 127.0.0.1
EOF

  openssl genrsa -out "$CERT_DIR/harbor.key" 4096
  openssl req -new -x509 -key "$CERT_DIR/harbor.key" \
    -out "$CERT_DIR/harbor.crt" \
    -days 365 \
    -config "$CERT_DIR/harbor-cert.conf" \
    -extensions v3_req

  sudo mkdir -p /data/cert
  sudo cp "$CERT_DIR/harbor.crt" /data/cert/
  sudo cp "$CERT_DIR/harbor.key" /data/cert/
}

# ------------------------------------------------------------------------------
# Setup Harbor
# ------------------------------------------------------------------------------
setup_harbor() {
  cd "$HARBOR_DIR" || return

  # Stop if already running
  if docker ps --format '{{.Names}}' | grep -q harbor; then
    echo "Stopping existing Harbor..."
    sudo docker compose down
  fi

  echo "🔐 Configuring Harbor HTTPS..."

  generate_certs_if_needed

  cp harbor.yml harbor.yml.backup.$(date +%s) 2>/dev/null || true

# Set hostname
sed -i "s|^hostname: .*|hostname: $EXPOSED_HARBOR_HOST|" harbor.yml

# Disable HTTP ONLY if it exists
if grep -q "^http:" harbor.yml; then
  sed -i 's|^http:|# http:|' harbor.yml
  sed -i 's|^  port: 80|#   port: 80|' harbor.yml
  sed -i 's|^  port: 8081|#   port: 8081|' harbor.yml
fi

# Enable HTTPS only if not already
if grep -q "^#https:" harbor.yml; then
  sed -i 's|^#https:|https:|' harbor.yml
fi

if grep -q "^  #port: 443" harbor.yml; then
  sed -i "s|^  #port: 443|  port: ${EXPOSED_HARBOR_PORT}|" harbor.yml
fi

if grep -q "^  #certificate:" harbor.yml; then
  sed -i 's|^  #certificate: .*|  certificate: /data/cert/harbor.crt|' harbor.yml
fi

if grep -q "^  #private_key:" harbor.yml; then
  sed -i 's|^  #private_key: .*|  private_key: /data/cert/harbor.key|' harbor.yml
fi

  # Prepare
  chmod +x prepare
  sudo ./prepare

  configure_harbor_restart_policy

  echo "🚀 Starting Harbor..."
  sudo docker compose up -d

  wait_for_harbor

  create_harbor_systemd_service

  echo "✅ Harbor setup complete"
}

# ------------------------------------------------------------------------------
# Trust Harbor (FIXED: no unnecessary Docker restart)
# ------------------------------------------------------------------------------
trust_harbor_certificate() {
  echo "📜 Trusting Harbor certificate..."

  sudo cp /data/cert/harbor.crt /usr/local/share/ca-certificates/harbor.crt
  sudo update-ca-certificates

  TARGET="/etc/docker/certs.d/${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}/ca.crt"
  sudo mkdir -p "$(dirname "$TARGET")"
  sudo cp /data/cert/harbor.crt "$TARGET"

  echo "✅ Harbor trusted (no Docker restart needed)"
}

# ------------------------------------------------------------------------------
# Cleanup
# ------------------------------------------------------------------------------
stop_harbor_service() {
  cd "$HARBOR_DIR" || return
  sudo docker compose down --remove-orphans --volumes || true
  echo "✅ Harbor stopped"
}
