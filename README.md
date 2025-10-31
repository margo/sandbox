# MARGO Project Documentation

## Table of Contents
- [📘 Introduction](#-introduction)
- [🚀 Sandbox Quick Start](#-sandbox-quick-start)
    - [🛠️ Development Toolset](#-development-toolset)
    - [🔧 How to Build](#-how-to-build)
    - [🚚 How to Deploy](#-how-to-deploy)
    - [▶️ How to Run](#-how-to-run)
- [🗂️ Structure of the Repository](#-structure-of-the-repository)
- [📦 3rd Party Components](#-3rd-party-components)
- [🧠 Design and Mapping to MARGO Architecture](#-design-and-mapping-to-margo-architecture)
  - [🎼 Symphony WFM](#-symphony-wfm)
  - [📁 Repositories and Registry](#-repositories-and-registry)
  - [📊 Telemetry and Monitoring](#-telemetry-and-monitoring)
  - [🧩 MVP Pattern](#-mvp-pattern)
- [🔐 HTTP/1.1 and API Security](#-http11-and-api-security)
- [📝 Release Notes](#-release-notes)
- [💬 Comments and Feedback](#-comments-and-feedback)

---

## 📘 Introduction
Welcome to the MARGO project! The MARGO initiative defines mechanisms for interoperable orchestration at scale of edge applications/workloads and devices. It will deliver on the interoperability promise through an open standard, a reference implementation, and a comprehensive compliance testing toolkit. MARGO unlocks barriers to innovation in complex multi-vendor environments and accelerates digital transformation for organizations of all sizes. More about MARGO [here](https://margo.org/).

---

## 🚀 Sandbox Quick Start
This section allows you to set up a 'Sandbox' environment for experimenting with the MARGO specifications and APIs. This includes instructions on the prerequisites, how to set up a build environment, creating a deployment on a set of virtual machines and running scenarios between the MARGO WFM and the Device-Agent using a simple CLI.

### 🛠️ Development Toolset
- [Development Toolset](./docs/dev-toolsets.md)

#### 🔧 How to Build
- [Build the Sandbox](./docs/build.md)

#### 🚚 How to Deploy
- [Deploy the Sandbox](./docs/deploy.md)

#### ▶️ How to Run
- [Run the Sandbox](./docs/run.md)

---

## 🗂️ Structure of the Repository
The repository is divided into three main parts. More details on [Repository Structure](./docs/repo-structure.md):

- `shared-lib`: Reusable libraries and utilities (Open Source Components)
- `standard`: Standard implementation components as per MARGO specification
- `non-standard`: Sandbox enabling components, these are not defined by MARGO but required for reference implementation

---

## 📦 3rd Party Components
| Component Type | Component Name | Version |
|---|---|---|
| Container Registry | Harbor | v2.13.2 |
| Repository Management | Gogs | Latest |
| Observability Stack | Prometheus | Latest |
| Observability Stack | Grafana | Latest |
| Observability Stack | Jaeger | Latest |
| Observability Stack | Loki | Latest |
| Observability Stack | OpenTelemetry Collector | Latest |
| Observability Stack | Promtail | Latest |
| Security & Authentication | OpenSSL | System default |
| Supporting Infrastructure | Helm | 3.15.1 |
| Supporting Infrastructure | Go | 1.23.2 / 1.24.4 |
| Supporting Infrastructure | Docker | Latest (from get.docker.com) |
| Supporting Infrastructure | Docker Compose | V2 (latest) |
| Supporting Infrastructure | K3s | Latest (from get.k3s.io) |
| Supporting Infrastructure | Rust | Latest (from rustup) |
| Supporting Infrastructure | Node.js/NPM | System default |
| System Utilities | curl | System default |
| System Utilities | git | System default |
| System Utilities | wget | System default |
| System Utilities | build-essential | System default |
| System Utilities | gcc | System default |
| System Utilities | libc6-dev | System default |
| System Utilities | dos2unix | System default |

---

## 🧠 Design and Mapping to MARGO Architecture
MARGO intends to create an open interoperability standard and ecosystem for the industrial edge, allowing edge compute devices, workloads, and fleet management software to be compatible and interoperable across manufacturers and software developers willing to adopt such standard.

- MARGO envision the [Architecture](./docs/margo-architecture.png)
- The reference implementation through this 'Sandbox' environment implements the key MARGO components as per [Overlay-Architecture](./docs/overlay-architecture.png)

### 🎼 Symphony WFM
- Sandbox uses Eclipse Symphony as Workload Fleet Manager
- As mentioned in MARGO architecture and overlay architecture WFM connects through MARGO envisioned communication mechanisms

### 📁 Repositories and Registry
- Gogs and Harbor provide application registry and repository functionalities
- Application supplier's packages are stored in Gogs and docker images/helm artifacts related to these applications are stored in Harbor registry
- Application packages are pulled/pushed/deleted from Gogs repository whenever WFM performs application package LCM operation
- The WFM Client/Device-agent pulls docker images/helm artifacts from Harbor whenever workloads are getting deployed corresponding to the application packages during instance deployment

### 📊 Telemetry and Monitoring
- Sandbox deploys OpenTelemetry Collector at WFM client for instrumentation as per MARGO observability specification
- OpenTelemetry Collector sends telemetry data to observability backends from WFM client. Promtail is also deployed on WFM client for logs aggregation. Promtail agent fetches and pushes logs to Loki on WFM
- Observability backends should be external to WFM client. In Sandbox implementation, these backends are deployed on WFM. These include Prometheus, Jaeger, Loki and Grafana
- Loki is deployed for log aggregation and Grafana dashboard for visualization
- Jaeger is deployed for tracing
- Prometheus is deployed for Metrics collection

### 🧩 MVP Pattern
Eclipse Symphony is an open-source orchestration platform developed by the Eclipse Foundation to unify and manage complex workloads across diverse systems. In the context of Eclipse Symphony, MVP refers to a design pattern for building extensible systems, specifically a three-tiered architecture consisting of Managers, Vendors, and Providers.

This pattern is often referred to as HB-MVP (Host-Bound MVP):

- **Managers**: Implement business logic
- **Vendors**: Facilitate interaction with other systems
- **Providers**: Bridge the connection to external systems

#### Here's a breakdown of each component:
- **Vendors**: Vendors offer capabilities, typically exposed through an API surface. They act as the entry point for interacting with a specific service or functionality. Ideally, vendors are protocol-agnostic, allowing them to be bound to various communication protocols (e.g., HTTP, gRPC, MQTT) as needed
- **Managers**: Managers implement the platform-agnostic business logic for a given capability. They receive requests from vendors and orchestrate the necessary actions, often by interacting with one or more providers. Managers are designed for reuse and encapsulate the core business logic
- **Providers**: Providers are responsible for interacting with specific external systems or dependencies. They abstract away the details of platform-specific interactions, containing any platform-specific knowledge within their scope. Managers utilize providers to perform actions on external resources

Sandbox uses MVP pattern to implement MARGO specification.

---

## 🔐 HTTP/1.1 and API Security
- Sandbox utilizes HTTP/1.1 to ensure maximum support for existing infrastructure
- Server-side TLS is utilized instead of mTLS due to potential issues with TLS-terminating HTTPS load-balancer or HTTPS proxies doing lawful inspection
- Use of X.509 certificates to represent both parties within the REST API construction. These certificates are utilized to prove each participant's identity, establish a secure TLS session, and securely transport information within secure envelopes. Supports client authentication using X.509 certificates conforming to RFC 5280
- The device establishes a secure HTTPS connection using server-side TLS. It validates the server's identity using the public root CA certificate. By utilizing the certificates to create payload envelopes (HTTP request body), the device's management client can ensure secure transport between the device's management client and the Workload Fleet Management web service
- For API security, server side TLS 1.3 (minimum) is used, where the keys are obtained from the Server's X.509 Certificate as defined in the standard HTTP over TLS
- For API integrity, the device's management client is issued a client-specific X.509 certificate. The issuer of the client X.509 certificate is trusted under the assumption that the root CA download to the Workload Fleet Management server occurs as a precondition to onboarding the devices

---

## 📝 Release Notes
Details of version updates, bug fixes, and new features.

---

## 💬 Comments and Feedback
We welcome your thoughts! Please open an issue or submit a pull request for suggestions or improvements.

---