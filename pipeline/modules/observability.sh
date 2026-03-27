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
