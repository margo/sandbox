##### [Back To Main](../README.md)
## 🛠 Development Toolsets used in Sandbox

Here are all the development toolsets used and their purposes:

---

### ⚙️ Core Development Tools

#### Go (Golang)
- **Version**: 1.23.2 (`device-agent.sh`), 1.24.4 (`wfm.sh`)
- **Uses**:
  - Building Symphony API server
  - Building Maestro CLI tool
  - Building device-agent applications
  - Go module management and dependencies

#### Docker & Docker Compose
- **Version**: Docker Compose V2
- **Uses**:
  - Container orchestration for services
  - Building and running device-agent containers
  - Running Symphony API as containerized service
  - Managing multi-container applications (Harbor, Keycloak, Gogs)

#### Rust
- **Uses**:
  - Building Rust components in Symphony API
  - Compiling providers in `/symphony/api/pkg/apis/v1alpha1/providers/target/rust`

---

### 🧭 Container Orchestration & Kubernetes

#### K3s (Lightweight Kubernetes)
- **Uses**:
  - Running device-agent pods with ServiceAccount authentication
  - Container orchestration for production workloads
  - RBAC and security management

#### Helm
- **Version**: 3.15.1
- **Uses**:
  - Package management for Kubernetes applications
  - Deploying device-agent and workload charts
  - Managing observability stack (Prometheus, Grafana, Jaeger, Loki)
  - Installing OTEL Collector and Promtail

#### kubectl
- **Uses**:
  - Kubernetes cluster management
  - Pod and service administration
  - ConfigMap and Secret management

---

### 📦 Container Registry & Repository Management

#### Harbor
- **Uses**:
  - Private container registry
  - Storing and managing Docker images and helm artefacts
  - Image security scanning and vulnerability management

#### Gogs
- **Uses**:
  - Git repository hosting
  - Management of workload packages as per margo defined specification
  - Nextcloud, Nginx and Custom OTEL workload packages are getting uploaded to Gogs while environment setup. These [Margo Packages](../poc/tests/artefacts) are based on margo application description specification 
  - API-based repository creation and management

---

### 📈 Observability & Monitoring Stack

#### Prometheus
- **Uses**:
  - Metrics collection and storage
  - Scraping OTEL Collector metrics
  - Time-series database for monitoring

#### Grafana
- **Uses**:
  - Metrics visualization and dashboards
  - Data source integration with Prometheus and Loki

#### Jaeger
- **Uses**:
  - Distributed tracing
  - OTLP (OpenTelemetry Protocol) support
  - Performance monitoring and debugging

#### Loki
- **Uses**:
  - Log aggregation and storage
  - Centralized logging solution
  - Receives logs via Promtail

#### OTEL Collector (OpenTelemetry)
- **Uses**:
  - Telemetry data collection (metrics, traces)
  - Data processing and forwarding
  - Integration with observability backends

#### Promtail
- **Uses**:
  - Log shipping agent
  - Pushing logs from Kubernetes pods to Loki

---

### 🔐 Security & Authentication

#### Keycloak
- **Uses**:
  - Identity and access management 
  - OAuth/OIDC authentication
  - User federation and SSO

  **Note : Keycloak is not currently being used. Server side TLS is used and client establishes initial trust using rootCA/public certificates then server assigns an unique client-id. This is as per approved margo SUP.** 

#### OpenSSL
- **Uses**:
  - TLS certificate generation (RSA and ECDSA)
  - CA certificate creation
  - Server certificate signing
  - Device certificate generation

---

### 🧰 Build & Development Utilities

#### Git
- **Uses**:
  - Source code version control
  - Repository cloning and branch management
  - Integration with GitHub private repositories

#### npm
- **Uses**:
  - Building Symphony UI components
  - JavaScript dependency management
  
  **Note : Symphony UI not used. Application and workload LCM operations are performed using the script /pipeline/wfm-cli.sh**

#### curl & wget
- **Uses**:
  - HTTP requests and API testing
  - Downloading installation packages
  - Service health checks

#### jq
- **Uses**:
  - JSON parsing and manipulation
  - API response processing

---

### 🖥 System Utilities

#### dos2unix
- **Uses**:
  - Converting line endings for cross-platform compatibility

#### build-essential, gcc, libc6-dev
- **Uses**:
  - Compilation toolchain for building native applications
  - C/C++ development dependencies

---

### 📦 Workload Packages

#### Nextcloud
- **Uses**:
  - File sharing and collaboration platform
  - Test application for deployment scenarios

#### Nginx
- **Uses**:
  - Web server and reverse proxy
  - Ingress controller for Kubernetes

---

### 📡 Custom OTEL Application
- **Uses**:
  - Custom telemetry application.
  - Test application for deploying a custom application with telemetry capabilities.
  - This is a sample online ordering application written in golang which sends metrics and traces to OpenTelemetry collector. This application is defined as margo specified application package and deployed as workload on device along with collector. The margo package and application source code is available at [Custom OTEL](../poc/tests/artefacts/custom-otel-helm-app)   

  - Demonstration of observability integration