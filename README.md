# MARGO Project Documentation

## Table of Contents
- [Introduction](#introduction)
- [Quick Start Guide](#quick-start-guide)
  - [Development Toolset](#development-toolset)
  - [How to Build](#how-to-build)
  - [How to Deploy](#how-to-deploy)
  - [How to Run](#how-to-run)
- [Structure of the Repository](#structure-of-the-repository)
- [3rd Party Components](#3rd-party-components)
- [Design and Mapping to MARGO Architecture](#design-and-mapping-to-margo-architecture)
  - [Symphony WFM](#symphony-wfm)
  - [Repositories and Registry](#repositories-and-registry)
  - [Telemetry and Monitoring](#telemetry-and-monitoring)
  - [MVP Pattern](#mvp-pattern)
- [HTTP/1.1 and API Security](#http11-and-api-security)
- [Release Notes](#release-notes)
- [Comments and Feedback](#comments-and-feedback)

---

### Introduction
Welcome to the MARGO project's Code-first Sandbox ! 
The [MARGO](https://margo.org/) initiative defines mechanisms for interoperable orchestration at scale of edge applications/workloads and devices. It will deliver on the interoperability promise through an open standard, a reference implementation, and a comprehensive compliance testing toolkit. MARGO unlocks barriers to innovation in complex multi-vendor environments and accelerates digital transformation for organizations of all sizes. 

This project provides an open-source, sandbox with an implementation of the MARGO specified interfaces and workflows. The objective is to allow interested users to experiment with the interfaces and APIs and provide feedback to improve the MARGO specifications. This project is by no means intended for "commercial adoption".

Before you get started, please spend some time to understand the [Structure of the Repository](#structure-of-the-repository) first.

The project follows a Release schedule tied with the MARGO specification releases. Please look at the [Release Notes](#release-notes) sections for specific release specific content.
If you want to read more on the design aspects and understand how various components map to the MARGO Architecture, please read the [Design and Mapping to MARGO Architecture](#design-and-mapping-to-margo-architecture) section.

However, if you want to try out things first read the section [Quick Start Guide](#quick-start-guide) below to get your Sandbox Environment setup quickly.

Please leave a comment as we welcome feedback, a * is always appreciated !!

---

### Quick Start Guide
This section allows you to set up the 'Sandbox' environment for experimenting with the MARGO specifications and APIs. This includes instructions on the prerequisites for your setup, how to set up a build environment, creating a deployment on a set of virtual machines and running scenarios between the MARGO WFM and the Device-Agent using a simple CLI. 

Here is [Simplified Guide](./docs/simplified-setup-guide.md) to get you started quickly. Read the detailed steps below to explore further.

#### Development Toolset
- [Development Toolset](./docs/dev-toolsets.md)

#### How to Build
- [Build the Sandbox](./docs/build.md)

#### How to Deploy
- [Deploy the Sandbox](./docs/deploy.md)

#### How to Run
- [Run the Sandbox](./docs/run.md)

---

### Structure of the Repository
The repository is divided into three main parts. You can find more details here on [Repository Structure](./docs/repo-structure.md):

- `shared-lib`: Reusable libraries and utilities (Open Source Components)
- `standard`: Implementation of the components as per MARGO specification
- `non-standard`: Enabling components, which are not defined by MARGO, but required for an overall implementation

---

### 3rd Party Components
| Component Type | Component Name | Version |
|---|---|---|
| Container Registry | Harbor | v2.13.2 |
| OCI Client | ORAS | 1.1.0
| Observability Stack | Prometheus | prometheus-27.49.0 |
| Observability Stack | Grafana | grafana-10.3.0 |
| Observability Stack | Jaeger | jaeger-3.4.1 |
| Observability Stack | Loki | loki-6.46.0 |
| Observability Stack | OpenTelemetry Collector | 0.140.0 |
| Observability Stack | Promtail | 6.17.1 (helm chart for k3s device), grafana/promtail:2.9.10 (docker-image for docker device)  |
| Security & Authentication | OpenSSL | System default |
| Supporting Infrastructure | Helm | 3.15.1 |
| Supporting Infrastructure | Go | 1.23.2 / 1.24.4 |
| Supporting Infrastructure | Docker | Latest (from get.docker.com) |
| Supporting Infrastructure | Docker Compose | v2.24.6 |
| Supporting Infrastructure | K3s | Latest (from get.k3s.io) |
| Supporting Infrastructure | Node.js/NPM | System default |
| System Utilities | curl | System default |
| System Utilities | git | System default |
| System Utilities | wget | System default |
| System Utilities | build-essential | System default |
| System Utilities | gcc | System default |
| System Utilities | libc6-dev | System default |
| System Utilities | dos2unix | System default |

---

### Design and Mapping to MARGO Architecture
MARGO envisions a [Distributed system design](https://specification.margo.org/overview/envisioned-system-design/#overview) for Industry 4.0 applications, which chiefly includes Application Supplier infrastructure, Fleet Manager and Devices which run Applications.
The Fleet Manager responsible for deploying Applications as running Workloads is refered to as a 'Workload Fleet Manager' or WFM.

Other MARGO definitions are avialable in [MARGO Technical Lexicon](https://specification.margo.org/personas-and-definitions/technical-lexicon/)

This Code First Sandbox realises the MARGO system design using a set of open-source components, as well as an implementation of the 'standard' and 'non-standard' or enabling components.
You can see a view of the MARGO system design, with an overlay of the components available in the Code First Sandbox in [this diagram of the distributed system design](./docs/overlay-architecture.png).

This includes the following elements - 

#### Symphony WFM
- This Code First Sandbox uses [Eclipse Symphony](https://github.com/margo/symphony) as Workload Fleet Manager.
- As mentioned in MARGO architecture and overlay architecture WFM connects through MARGO envisioned communication mechanisms

#### Repositories and Registry
- Harbor provide application registry and images/helm-charts repository functionalities.
- Application supplier's packages , images/helm-charts are stored in Harbor.
 and docker images/helm artifacts related to these applications are stored in Harbor registry.
- Application packages are pulled/pushed/deleted from Harbor repository.
- WFM stores application packages in its database and are used during LCM (Life Cycle Management) operation.
- The WFM Client/Device-agent pulls docker images/helm artifacts from Harbor whenever workloads are getting deployed corresponding to the application packages during instance deployment.

#### Telemetry and Monitoring
- Sandbox deploys OpenTelemetry Collector at WFM client for instrumentation as per MARGO observability specification.
- OpenTelemetry Collector sends telemetry data to observability backends from WFM client. Promtail is also deployed on WFM client for logs aggregation. Promtail agent fetches and pushes logs to Loki on WFM.
- Observability backends should be external to WFM client. In Sandbox implementation, these backends are deployed on WFM. These include Prometheus, Jaeger, Loki and Grafana.
- Loki is deployed for log aggregation and Grafana dashboard for visualization.
- Jaeger is deployed for tracing.
- Prometheus is deployed for Metrics collection.

#### MVP Pattern
Eclipse Symphony is an open-source orchestration platform developed by the Eclipse Foundation to unify and manage complex workloads across diverse systems. In the context of Eclipse Symphony, MVP refers to a design pattern for building extensible systems, specifically a three-tiered architecture consisting of Managers, Vendors, and Providers.

This pattern is often referred to as HB-MVP (Host-Bound MVP):

- **Managers**: Implements business logic
- **Vendors**: Facilitate interaction with other systems
- **Providers**: Bridge the connection to external systems

##### Here's a breakdown of each component:
- **Vendors**: Vendors offer capabilities, typically exposed through an API surface. They act as the entry point for interacting with a specific service or functionality. Ideally, vendors are protocol-agnostic, allowing them to be bound to various communication protocols (e.g., HTTP, gRPC, MQTT) as needed.
- **Managers**: Managers implement the platform-agnostic business logic for a given capability. They receive requests from vendors and orchestrate the necessary actions, often by interacting with one or more providers. Managers are designed for reuse and encapsulate the core business logic.
- **Providers**: Providers are responsible for interacting with specific external systems or dependencies. They abstract away the details of platform-specific interactions, containing any platform-specific knowledge within their scope. Managers utilize providers to perform actions on external resources.

Code First Sandbox uses MVP pattern to implement MARGO specification.

---

### HTTP/1.1 and API Security
- Sandbox utilizes HTTP/1.1 to ensure maximum support for existing infrastructure.
- Server-side TLS is utilized instead of mTLS due to potential issues with TLS-terminating HTTPS load-balancer or HTTPS proxies doing lawful inspection.
- Use of X.509 certificates to represent both parties within the REST API construction. These certificates are utilized to prove each participant's identity, establish a secure TLS session, and securely transport information within secure envelopes. Supports client authentication using X.509 certificates conforming to RFC 5280.
- The device establishes a secure HTTPS connection using server-side TLS. It validates the server's identity using the public root CA certificate. By utilizing the certificates to create payload envelopes (HTTP request body), the device's management client can ensure secure transport between the device's management client and the Workload Fleet Management web service.
- For API security, server side TLS 1.3 (minimum) is used, where the keys are obtained from the Server's X.509 Certificate as defined in the standard HTTP over TLS.
- For API integrity, the device's management client is issued a client-specific X.509 certificate. The issuer of the client X.509 certificate is trusted under the assumption that the root CA download to the Workload Fleet Management server occurs as a precondition to onboarding the devices. This CA can be provided to the device in any offline mode.

---

### Release Notes
Details of version updates, bug fixes, and new features.

---

### Comments and Feedback
We welcome your thoughts! Please open an issue or submit a pull request for suggestions or improvements.

---
