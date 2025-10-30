# Margo Project Documentation

## 📘 Introduction
Welcome to the Margo project! The Margo initiative defines mechanisms for interoperable orchestration at scale of edge applications/workloads and devices. It will deliver on the interoperability promise through an open standard, a reference implementation and a comprehensive compliance testing toolkit.  Margo unlocks barriers to innovation in complex multi-vendor environments and accelerates digital transformation for organizations of all sizes. More about Margo [here](https://margo.org/).

---

## 🚀 Quick Start
This section allows you to set up a 'Sandbox' environment for experimenting with the MARGO specifications and APIs. This includes instructions on the prerequisites, how to set up a build environment, creating a deployment on a set of virtual machines and running scenarios between the MARGO WFM and the Device-Agent using a simple CLI.  

### 🛠️ Dev Toolset
[Dev Toolset](./docs/dev-toolsets.md)  
#### 🔧 How to Build
[Build Process](./docs/build.md)
#### 🚚 How to Deploy
[Deployment](./docs/deploy.md)
#### ▶️ How to Run
[Running the Setup](./docs/run.md)

---

## 🗂️ Structure of the Repository
The repository is divided into three main parts:
- `shared-lib`: Common libraries used across modules
- `standard`: Standard implementation components
- `non-standard`: Custom or experimental components

---

## 📦 3rd Party Components
List of integrated third-party tools and libraries used in the project.

---

## 🧠 Design and Mapping to MARGO Architecture

### 🎼 Symphony WFM
Workflow management integration details.

### 📁 Repositories
- **Gogs**: Git repository management
- **Harbor**: Container image registry

### 📊 Telemetry and Monitoring
- **OTEL Telemetry**: OpenTelemetry integration
- **Jaeger**: Distributed tracing
- **Prometheus**: Metrics collection
- **Grafana/Loki**: Visualization and log aggregation

### 🧩 Provider MVP Pattern
Explanation of the MVP (Model-View-Presenter) pattern used for providers.

### 🔐 HTTP1.1 and API Security
Security protocols and API communication standards.

---

## 📝 Release Notes
Details of version updates, bug fixes, and new features.

---

## 💬 Comments and Feedback
We welcome your thoughts! Please open an issue or submit a pull request for suggestions or improvements.

---
