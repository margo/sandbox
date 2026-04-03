#!/bin/bash
# modules/observability.sh - Observability stack configuration

source "$(dirname "${BASH_SOURCE[0]}")/../lib/common.sh" 

# Observability release names
PROM_RELEASE="${PROM_RELEASE:-prometheus}"
JAEGER_RELEASE="${JAEGER_RELEASE:-jaeger}"
GRAFANA_RELEASE="${GRAFANA_RELEASE:-grafana}"
LOKI_RELEASE="${LOKI_RELEASE:-loki}"

create_observability_namespace() {
  echo "🔧 Checking observability namespace..."

  if sudo kubectl get namespace $NAMESPACE_OBSERVABILITY >/dev/null 2>&1; then
    echo "✅ Namespace '$NAMESPACE_OBSERVABILITY' already exists"
  else
    echo "🔧 Creating namespace '$NAMESPACE_OBSERVABILITY'..."
    sudo kubectl create namespace $NAMESPACE_OBSERVABILITY
    echo "✅ Namespace '$NAMESPACE_OBSERVABILITY' created successfully"
  fi
}

install_jaeger() {
  if helm status $JAEGER_RELEASE -n "$NAMESPACE_OBSERVABILITY" >/dev/null 2>&1; then
    echo "⚠️ Jaeger Helm release found, checking pod health..."
  fi

  echo "🔄 Refreshing Jaeger Helm repo..."
  helm repo remove jaegertracing || true
  helm repo add jaegertracing https://jaegertracing.github.io/helm-charts
  helm repo update

  echo "🚀 Installing Jaeger v3.4.1 with OTLP and NodePort configuration..."
  helm install $JAEGER_RELEASE jaegertracing/jaeger \
    --version 3.4.1 \
    --namespace $NAMESPACE_OBSERVABILITY \
    --set agent.enabled=false \
    --set collector.enabled=true \
    --set collector.otlp.enabled=true \
    --set collector.service.type=NodePort \
    --set collector.service.nodePort=30417 \
    --set collector.service.additionalPorts[0].name=otlp-grpc \
    --set collector.service.additionalPorts[0].port=4317 \
    --set collector.service.additionalPorts[0].protocol=TCP \
    --set query.enabled=true \
    --set query.service.type=NodePort \
    --set query.service.nodePort=32500

  echo "⏳ Waiting for Jaeger pods to initialize..."
  sleep 10

  echo "🛠 Patching Jaeger Collector Service for OTLP gRPC..."
  sudo kubectl patch svc ${JAEGER_RELEASE}-collector \
    -n $NAMESPACE_OBSERVABILITY \
    --type='json' \
    -p='[
      {
        "op": "add",
        "path": "/spec/ports/-",
        "value": {
          "name": "otlp-grpc",
          "port": 4317,
          "protocol": "TCP",
          "targetPort": 4317,
          "nodePort": 30417
        }
      }
    ]'

  echo "🛠 Patching Jaeger Collector Service for OTLP HTTP..."
  sudo kubectl patch svc ${JAEGER_RELEASE}-collector \
    -n $NAMESPACE_OBSERVABILITY \
    --type='json' \
    -p='[
      {
        "op": "add",
        "path": "/spec/ports/-",
        "value": {
          "name": "otlp-http",
          "port": 4318,
          "protocol": "TCP",
          "targetPort": 4318,
          "nodePort": 30418
        }
      }
    ]'

  echo "✅ Jaeger setup complete!"
  echo "🌐 Query UI: NodePort 32500"
  echo "📡 OTLP gRPC: Port 4317"
  echo "📡 OTLP HTTP: Port 4318"
}

install_prometheus() {
  mkdir -p "$HOME/sandbox/pipeline/observability"
  cd "$HOME/sandbox/pipeline/observability"
  echo "📡 Setting up Prometheus with Remote Write receiver..."

  cat <<EOF > prometheus-values.yaml
server:
  image:
    repository: prom/prometheus
    tag: latest
  service:
    type: NodePort
    nodePort: 30900
  persistentVolume:
    enabled: false
  
  # Enable Remote Write receiver
  extraArgs:
    web.enable-remote-write-receiver: ""
    enable-feature: "remote-write-receiver"
  
  serverFiles:
    prometheus.yml:
      global:
        scrape_interval: 15s
        evaluation_interval: 15s
      
      # Keep self-scraping for Prometheus metrics
      scrape_configs:
        - job_name: 'prometheus'
          static_configs:
            - targets: ['localhost:9090']
EOF

  helm repo remove prometheus-community || true
  helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
  helm repo update

  helm install $PROM_RELEASE prometheus-community/prometheus \
    --version 27.49.0 \
    --namespace $NAMESPACE_OBSERVABILITY \
    -f prometheus-values.yaml

  echo "🔧 Patching Prometheus service to expose Remote Write endpoint..."
  sudo kubectl patch svc prometheus-server -n $NAMESPACE_OBSERVABILITY \
    --type='json' \
    -p='[
      {
        "op": "add",
        "path": "/spec/ports/-",
        "value": {
          "name": "remote-write",
          "port": 9090,
          "protocol": "TCP",
          "targetPort": 9090,
          "nodePort": 30909
        }
      }
    ]'

  echo "✅ Prometheus setup complete!"
  echo "📊 Prometheus UI: NodePort 30900"
  echo "📡 Remote Write endpoint: NodePort 30909"
  echo "ℹ️  Devices should push metrics to: http://WFM_HOST:30909/api/v1/write"
}

install_loki() {
  mkdir -p "$HOME/sandbox/pipeline/observability"
  cd "$HOME/sandbox/pipeline/observability"
  echo "📦 Installing Loki for log aggregation..."

  cat <<EOF > loki-values.yaml
deploymentMode: SingleBinary
chunksCache:
  enabled: false
loki:
  auth_enabled: false
  commonConfig:
    replication_factor: 1
  limits_config:
    allow_structured_metadata: false
  storage:
    type: filesystem
  schemaConfig:
    configs:
      - from: 2020-10-24
        store: boltdb-shipper
        object_store: filesystem
        schema: v11
        index:
          prefix: index_
          period: 24h
  storage_config:
    boltdb_shipper:
      active_index_directory: /tmp/loki/index
      cache_location: /tmp/loki/cache
    filesystem:
      directory: /tmp/loki/chunks
singleBinary:
  replicas: 1
read:
  replicas: 0
write:
  replicas: 0
backend:
  replicas: 0
EOF

  helm install $LOKI_RELEASE grafana/loki --version 6.46.0 -f loki-values.yaml --namespace $NAMESPACE_OBSERVABILITY

  echo "🔧 Patching Loki service to expose via NodePort 32100..."
  sudo kubectl patch svc loki -n $NAMESPACE_OBSERVABILITY \
    --type='json' \
    -p='[
      {
        "op": "replace",
        "path": "/spec/type",
        "value": "NodePort"
      },
      {
        "op": "add",
        "path": "/spec/ports/0/nodePort",
        "value": 32100
      }
    ]'

  echo "✅ Loki installed and exposed at NodePort 32100"
}

install_grafana() {
  echo "📊 Installing Grafana..."
  helm repo remove grafana || true
  helm repo add grafana https://grafana.github.io/helm-charts
  helm repo update

  helm install $GRAFANA_RELEASE grafana/grafana \
    --version 10.3.0 \
    --namespace $NAMESPACE_OBSERVABILITY \
    --set service.type=NodePort \
    --set service.nodePort=32000 \
    --set adminPassword='admin' \
    --set persistence.enabled=false

  echo "✅ Grafana installed!"
  echo "🌐 Grafana UI available at NodePort 32000"
  echo "🔐 Login with username: admin and password: admin"
}



install_otel_collector_promtail() {
  echo "Installing OTEL Collector and Promtail..."
  cd "$HOME/sandbox/pipeline/observability" || { echo '❌ observability dir missing'; exit 1; }
  create_observability_namespace
  install_promtail
  install_otel_collector
  echo "✅ OTEL Collector and Promtail installation completed."
}

function install_promtail() {
  echo "📦 Installing Promtail to push logs to Loki at $WFM_HOST..."

  cat <<EOF > promtail-values.yaml
config:
  server:
    http_listen_port: 9080
    grpc_listen_port: 0

  positions:
    filename: /tmp/positions.yaml

  clients:
    - url: http://${WFM_HOST}:32100/loki/api/v1/push

  scrape_configs:
    - job_name: pod-logs
      static_configs:
        - targets:
            - localhost
          labels:
            job: podlogs
            __path__: /var/log/pods/*/*/*.log
EOF

  helm repo add grafana https://grafana.github.io/helm-charts
  helm repo update

  helm install $PROMTAIL_RELEASE grafana/promtail --version 6.17.1 -f promtail-values.yaml --namespace $NAMESPACE_OBSERVABILITY

  echo "✅ Promtail installed and configured to push logs to Loki"
}

function install_otel_collector() {
  echo "📡 Installing OTEL Collector to send metrics and traces to WFM node..."

  cat <<EOF > otel-values.yaml
mode: deployment
image:
  repository: otel/opentelemetry-collector-contrib

extraEnvs:
  - name: KUBE_NODE_NAME
    valueFrom:
      fieldRef:
        fieldPath: spec.nodeName

config:
  receivers:
    otlp:
      protocols:
        http:
          endpoint: 0.0.0.0:4318
        grpc:
          endpoint: 0.0.0.0:4317

    hostmetrics:
      collection_interval: 30s
      scrapers:
        cpu:
        memory:
        disk:
        filesystem:
        load:
        network:
        processes:
        paging:

    kubeletstats:
      collection_interval: 30s
      auth_type: "serviceAccount"
      endpoint: "https://\${KUBE_NODE_NAME}:10250"
      insecure_skip_verify: true
      metric_groups:
        - container
        - pod
        - node

  exporters:
    # Send traces to Jaeger
    otlp:
      endpoint: ${WFM_HOST}:30417
      tls:
        insecure: true

    # CHANGED: Push metrics to Prometheus Remote Write
    prometheusremotewrite:
      endpoint: http://${WFM_HOST}:30909/api/v1/write
      tls:
        insecure: true
      resource_to_telemetry_conversion:
        enabled: true

    debug:
      verbosity: detailed

  processors:
    batch: {}

  service:
    pipelines:
      traces:
        receivers: [otlp]
        processors: [batch]
        exporters: [otlp, debug]

      # CHANGED: Push metrics instead of exposing endpoint
      metrics:
        receivers: [otlp, hostmetrics, kubeletstats]
        processors: [batch]
        exporters: [prometheusremotewrite, debug]
EOF

  helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
  helm repo update

  helm install $OTEL_RELEASE open-telemetry/opentelemetry-collector --version 0.140.0 -f otel-values.yaml --namespace $NAMESPACE_OBSERVABILITY

  echo "✅ OTEL Collector setup complete!"
  echo "🔍 Traces sent to: ${WFM_HOST}:30417"
  echo "📊 Metrics pushed to: ${WFM_HOST}:30909/api/v1/write"
}


install_otel_collector_promtail_docker() {
  echo "Installing OTEL Collector v0.140.0 and Promtail v2.9.10 as Docker containers..."
  cd "$HOME/sandbox/pipeline/observability" || { echo '❌ observability dir missing'; exit 1; }

  # Get docker group GID for proper permissions
  DOCKER_GID=$(getent group docker | cut -d: -f3)
  echo "Docker group GID: $DOCKER_GID"

  # Create docker-compose.yml for observability stack
  cat <<EOF > docker-compose-observability.yml
version: '3.8'

services:
  promtail:
    image: grafana/promtail:2.9.10
    container_name: promtail
    volumes:
      - /var/log:/var/log:ro
      - /var/lib/docker/containers:/var/lib/docker/containers:ro
      - ./promtail-config.yml:/etc/promtail/config.yml
    command: -config.file=/etc/promtail/config.yml
    restart: unless-stopped
    network_mode: host

  otel-collector:
    image: otel/opentelemetry-collector-contrib:0.140.0
    container_name: otel-collector
    user: "0:${DOCKER_GID}"
    volumes:
      - ./otel-collector-config.yml:/etc/otel/config.yml
      - /var/run/docker.sock:/var/run/docker.sock
    command: --config=/etc/otel/config.yml
    ports:
      - "4317:4317"   # OTLP gRPC
      - "4318:4318"   # OTLP HTTP
    restart: unless-stopped
    network_mode: host
    environment:
      - HOST_IP=\${HOST_IP:-127.0.0.1}
EOF

  # Create Promtail config
  cat <<EOF > promtail-config.yml
server:
  http_listen_port: 9080
  grpc_listen_port: 0

positions:
  filename: /tmp/positions.yaml

clients:
  - url: http://${WFM_HOST}:32100/loki/api/v1/push

scrape_configs:
  - job_name: docker-logs
    static_configs:
      - targets:
          - localhost
        labels:
          job: dockerlogs
          __path__: /var/lib/docker/containers/*/*.log
EOF

  # Create OTEL Collector config with Prometheus Remote Write
  cat <<EOF > otel-collector-config.yml
receivers:
  # OTLP receiver for application traces/metrics
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4318
      grpc:
        endpoint: 0.0.0.0:4317

  # Host-level metrics
  hostmetrics:
    collection_interval: 30s
    scrapers:
      cpu:
      memory:
      disk:
      filesystem:
      load:
      network:
      processes:
      paging:

  # Docker container metrics
  docker_stats:
    endpoint: unix:///var/run/docker.sock
    collection_interval: 10s
    timeout: 5s
    api_version: "1.44"

exporters:
  # Send traces to Jaeger on WFM server
  otlp/jaeger:
    endpoint: ${WFM_HOST}:30417
    tls:
      insecure: true

  # CHANGED: Push metrics to Prometheus Remote Write
  prometheusremotewrite:
    endpoint: http://${WFM_HOST}:30909/api/v1/write
    tls:
      insecure: true
    resource_to_telemetry_conversion:
      enabled: true

  # Debug output
  debug:
    verbosity: detailed

processors:
  batch:
    timeout: 10s
    send_batch_size: 1024

  # Add resource attributes
  resource:
    attributes:
      - key: device.type
        value: docker
        action: insert
      - key: device.ip
        value: \${HOST_IP}
        action: insert

service:
  pipelines:
    # Traces pipeline - send to Jaeger
    traces:
      receivers: [otlp]
      processors: [batch, resource]
      exporters: [otlp/jaeger, debug]

    # CHANGED: Metrics pipeline - push to Prometheus Remote Write
    metrics:
      receivers: [otlp, hostmetrics, docker_stats]
      processors: [batch, resource]
      exporters: [prometheusremotewrite, debug]
EOF

  # Get host IP for resource attributes
  HOST_IP=$(hostname -I | awk '{print $1}')
  export HOST_IP

  # Start the observability stack
  docker compose -f docker-compose-observability.yml up -d

  # Create & enable systemd unit to start this stack on reboot
  create_observability_systemd_service

  echo "✅ OTEL Collector v0.140.0 and Promtail v2.9.10 installed"
  echo "📡 OTLP gRPC: localhost:4317"
  echo "📡 OTLP HTTP: localhost:4318"
  echo "🔍 Traces sent to Jaeger at: ${WFM_HOST}:30417"
  echo "📊 Metrics pushed to Prometheus at: ${WFM_HOST}:30909/api/v1/write"
  echo "📝 Logs sent to Loki at: ${WFM_HOST}:32100"
}

install_otel_collector_promtail_wrapper() {
  if [ "$DEVICE_TYPE" = "k3s" ]; then
    install_otel_collector_promtail  # Existing k8s-based installation
  else
    install_otel_collector_promtail_docker  # New Docker-based installation
  fi
}

uninstall_otel_collector_promtail_wrapper() {
  if [ "$DEVICE_TYPE" = "k3s" ]; then
    uninstall_otel_collector_promtail  # Existing k8s-based uninstallation
  else
    uninstall_otel_collector_promtail_docker  # New Docker-based uninstallation
  fi
}

uninstall_otel_collector_promtail_docker() {
  echo "🧹 Uninstalling Promtail and OTEL Collector containers..."
  cd "$HOME/sandbox/pipeline/observability" || { echo '❌ observability dir missing'; exit 1; }

  if [ -f "docker-compose-observability.yml" ]; then
    docker compose -f docker-compose-observability.yml down
    rm -f docker-compose-observability.yml promtail-config.yml otel-collector-config.yml
  fi

  echo "✅ Cleanup complete."
}


uninstall_otel_collector_promtail() {
  echo "🧹 Uninstalling Promtail and OTEL Collector..."
  cd "$HOME/sandbox/pipeline/observability" || { echo '❌ observability dir missing'; exit 1; }

    # Uninstall helm releases only if they exist
    for release in $PROMTAIL_RELEASE $OTEL_RELEASE; do
        if helm status $release -n "$NAMESPACE_OBSERVABILITY" >/dev/null 2>&1; then
            echo "🗑️ Uninstalling $release..."
            helm uninstall $release --namespace "$NAMESPACE_OBSERVABILITY"
        else
            echo "⏭️ $release not found, skipping..."
        fi
    done
  rm -f promtail-values.yaml otel-values.yaml
  echo "✅ Cleanup complete."

}