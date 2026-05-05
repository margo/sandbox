#!/bin/bash

# Harbor-specific functions

source "$(dirname "${BASH_SOURCE[0]}")/../lib/common.sh"

create_harbor_systemd_service() {
  echo "🔧 Creating systemd service for Harbor auto-start..."
  local harbor_dir="$HOME/sandbox/scripts/harbor"

  sudo tee /etc/systemd/system/harbor.service > /dev/null <<EOF
[Unit]
Description=Harbor Container Registry
Requires=docker.service
After=docker.service network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=${harbor_dir}
ExecStartPre=/bin/sleep 10
ExecStart=/usr/bin/docker compose up -d
ExecStop=/usr/bin/docker compose down
TimeoutStartSec=0

[Install]
WantedBy=multi-user.target
EOF

  sudo systemctl daemon-reload
  sudo systemctl enable harbor.service

  echo "✅ Harbor systemd service created and enabled"
  echo "📋 Service will start Harbor automatically on boot"
  echo "📁 Working directory: ${harbor_dir}"
}

configure_harbor_restart_policy() {
  local compose_file="$HOME/sandbox/scripts/harbor/docker-compose.yml"

  if [ ! -f "$compose_file" ]; then
    echo "⚠️ docker-compose.yml not found, will be generated during install"
    return 0
  fi

  echo "🔧 Replacing restart policies in docker-compose.yml..."
  cp "$compose_file" "${compose_file}.backup.$(date +%s)"
  sed -i 's/^\s*restart:\s*always/    restart: unless-stopped/g' "$compose_file"

  echo "✅ Restart policies replaced with unless-stopped"
  echo "📋 Verifying restart policies in docker-compose.yml:"
  grep "restart:" "$compose_file"
}

setup_harbor() {
  if docker ps --format '{{.Names}}' | grep -q harbor; then
    echo 'Harbor is already running, stopping it first...'
    cd "$HOME/sandbox/scripts/harbor"
    sudo docker compose down --remove-orphans
    sleep 5
  fi

  cd "$HOME/sandbox/scripts/harbor"

  echo "🔐 Configuring Harbor for HTTPS-only on port ${EXPOSED_HARBOR_PORT}..."

  cp harbor.yml harbor.yml.backup.$(date +%s) 2>/dev/null || true

  # Disable HTTP completely
  sed -i "s|^hostname: .*|hostname: $EXPOSED_HARBOR_HOST|" harbor.yml
  sed -i 's|^http:|# http:|' harbor.yml
  sed -i 's|^  port: 80|#   port: 80|' harbor.yml
  sed -i 's|^  port: 8081|#   port: 8081|' harbor.yml

  # Enable HTTPS
  sed -i 's|^#https:|https:|' harbor.yml
  sed -i "s|^  #port: 443|  port: ${EXPOSED_HARBOR_PORT}|" harbor.yml
  sed -i 's|^  #certificate: /your/certificate/path|  certificate: /data/cert/harbor.crt|' harbor.yml
  sed -i 's|^  #private_key: /your/private/key/path|  private_key: /data/cert/harbor.key|' harbor.yml

  echo "📋 Verifying harbor.yml HTTPS configuration..."
  grep -A 5 "^https:" harbor.yml

  echo "📜 Generating self-signed certificates for Harbor..."
  mkdir -p ./certs

  cat > ./certs/harbor-cert.conf <<EOF
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
basicConstraints = CA:FALSE
keyUsage = keyEncipherment, digitalSignature
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = ${EXPOSED_HARBOR_HOST}
DNS.2 = localhost
IP.1 = 127.0.0.1
EOF

  openssl genrsa -out ./certs/harbor.key 4096
  openssl req -new -x509 -key ./certs/harbor.key -out ./certs/harbor.crt \
    -days 365 -config ./certs/harbor-cert.conf -extensions v3_req

  echo "🔍 Verifying generated certificate..."
  openssl x509 -in ./certs/harbor.crt -text -noout | grep -A 3 "Subject Alternative Name"

  sudo mkdir -p /data/cert
  sudo cp ./certs/harbor.crt /data/cert/
  sudo cp ./certs/harbor.key /data/cert/
  sudo chmod 644 /data/cert/harbor.crt
  sudo chmod 600 /data/cert/harbor.key

  echo "✅ Harbor certificates generated and installed"

  echo 'Preparing Harbor configuration with HTTPS...'
  chmod +x prepare
  sudo ./prepare


 echo "🔧 Removing port 80 binding from docker-compose.yml..."

if [ -f docker-compose.yml ]; then
  # Backup the generated docker-compose.yml
  cp docker-compose.yml docker-compose.yml.backup.$(date +%s)

  # Remove the port 80 binding from nginx/proxy service
  # Match with any amount of leading whitespace
  sed -i '/^\s*-\s*80:8080$/d' docker-compose.yml
  sed -i '/^\s*-\s*.*:80:8080$/d' docker-compose.yml

  echo "✅ Port 80 binding removed from docker-compose.yml"

  # Verify the change
  echo "📋 Verifying proxy/nginx ports in docker-compose.yml:"
  if grep -A 10 "proxy:" docker-compose.yml | grep -A 5 "ports:" | grep -v "^--$"; then
    echo "✅ Remaining ports configuration shown above"
  else
    echo "ℹ️ No ports section found or empty"
  fi

  # Double-check port 80 is not present
  if grep -q "80:8080" docker-compose.yml; then
    echo "⚠️ WARNING: Port 80:8080 still found in docker-compose.yml!"
    grep -n "80:8080" docker-compose.yml
  else
    echo "✅ Confirmed: Port 80:8080 successfully removed"
  fi
else
  echo "⚠️ docker-compose.yml not found after prepare"
fi


  echo "🔍 Verifying docker-compose.yml has port ${EXPOSED_HARBOR_PORT}..."
  if grep -q "${EXPOSED_HARBOR_PORT}" docker-compose.yml; then
    echo "✅ docker-compose.yml configured for HTTPS on port ${EXPOSED_HARBOR_PORT}"
  else
    echo "⚠️ Port ${EXPOSED_HARBOR_PORT} not found in docker-compose.yml"
    grep "ports:" docker-compose.yml | head -5
  fi

  configure_harbor_restart_policy

  echo 'Starting Harbor with HTTPS-only on port '${EXPOSED_HARBOR_PORT}'...'
  sudo docker compose up -d

  sleep 5
  for container in nginx registry registryctl redis harbor-jobservice harbor-core harbor-db harbor-portal harbor-log; do
    if docker ps -a --format "{{.Names}}" | grep -q "^${container}$"; then
      docker update --restart=unless-stopped "$container" 2>/dev/null && echo "✅ Updated: $container"
    fi
  done

  echo 'Waiting for Harbor to initialize...'
  sleep 15

  echo "🔍 Verifying Harbor nginx container ports..."
  docker ps --filter "name=nginx" --format "table {{.Names}}\t{{.Ports}}"

  echo ""
  echo "📊 Harbor container status:"
  docker ps --filter "name=harbor" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

  create_harbor_systemd_service

  echo "⏳ Waiting for all containers to be healthy..."
  sleep 45

  healthy_count=$(docker ps --filter "name=harbor" --filter "health=healthy" --format "{{.Names}}" | wc -l)
  total_count=$(docker ps --filter "name=harbor" --format "{{.Names}}" | wc -l)

  echo "✅ Harbor status: $healthy_count/$total_count containers healthy"

  echo "🔍 Testing Harbor HTTPS endpoint..."
  if curl -k -s https://${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}/v2/ | grep -q "errors"; then
    echo "✅ Harbor HTTPS endpoint responding correctly"
  else
    echo "⚠️ Harbor HTTPS endpoint may not be ready yet"
  fi

  # Verify port 80 is NOT in use
  echo "🔍 Verifying port 80 is not bound..."
  if docker ps --filter "name=nginx" --format "{{.Ports}}" | grep -q ":80->"; then
    echo "⚠️ WARNING: Port 80 is still bound!"
  else
    echo "✅ Port 80 is NOT bound - HTTPS-only mode confirmed"
  fi

  echo "🔐 Harbor is now running with HTTPS-only on port ${EXPOSED_HARBOR_PORT}"
  echo "📍 Access Harbor at: https://${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}"
}


trust_harbor_certificate() {
  echo "📜 Adding Harbor certificate to system trust store..."

  sudo cp /data/cert/harbor.crt /usr/local/share/ca-certificates/harbor.crt
  sudo update-ca-certificates

  sudo mkdir -p /etc/docker/certs.d/${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}
  sudo cp /data/cert/harbor.crt /etc/docker/certs.d/${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}/ca.crt

  echo "🔄 Restarting Docker daemon to apply certificate changes..."
  sudo systemctl restart docker

  for i in {1..30}; do
    if sudo systemctl is-active --quiet docker; then
      echo "✅ Docker daemon restarted"
      break
    fi
    sleep 1
  done

  echo "🔄 Restarting Harbor containers..."
  cd "$HOME/sandbox/scripts/harbor"
  sudo docker compose up -d

  echo "⏳ Waiting for Harbor to restart..."
  sleep 50

  if docker ps --filter "name=nginx" --format "{{.Ports}}" | grep -q "${EXPOSED_HARBOR_PORT}"; then
    echo "✅ Harbor restarted successfully on port ${EXPOSED_HARBOR_PORT}"
  else
    echo "⚠️ Harbor may still be starting..."
  fi

  echo "✅ Harbor certificate trusted by system and Docker"
}

stop_harbor_service() {
  echo "6. Stopping and removing Harbor service..."

  if docker ps -a --format '{{.Names}}' | grep harbor; then
    cd "$HOME/sandbox/scripts/harbor"
    sudo docker compose down --remove-orphans --volumes 2>/dev/null && echo "✅ Stopped Harbor containers"
    sleep 10
  fi

  [ -d "$HOME/sandbox/scripts/harbor" ] && sudo rm -rf "$HOME/sandbox/scripts/harbor" && echo "✅ Removed Harbor compose directory"
}


configure_harbor_trust_for_k3s() {
  echo "🔐 Configuring Harbor CA trust for k3s node..."

  HARBOR_CERT="$HOME/sandbox/scripts/harbor/certs/harbor.crt"
  HOST="${EXPOSED_HARBOR_HOST}"
  PORT="${EXPOSED_HARBOR_PORT}"
  CERT_DIR="/var/lib/rancher/k3s/agent/etc/containerd/certs.d/${HOST}:${PORT}"

  if [ -f "$CERT_DIR/ca.crt" ] && [ -f "$CERT_DIR/hosts.toml" ]; then
    echo "✅ Harbor trust already present — skipping k3s restart"
    return 0
  fi

  sudo mkdir -p "$CERT_DIR"
  sudo cp "$HARBOR_CERT" "$CERT_DIR/ca.crt"

  sudo tee "$CERT_DIR/hosts.toml" >/dev/null <<EOF
server = "https://${HOST}:${PORT}"

[host."https://${HOST}:${PORT}"]
  capabilities = ["pull", "resolve"]
  ca = "ca.crt"
EOF

  sudo cp "$HARBOR_CERT" /usr/local/share/ca-certificates/harbor.crt
  sudo update-ca-certificates

  echo "🔁 Restarting k3s (one‑time trust activation)..."
  sudo systemctl restart k3s
  sleep 10

  echo "✅ Harbor trust configured for k3s"
}
