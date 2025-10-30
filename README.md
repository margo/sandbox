# Margo Project Documentation

## 📘 Introduction
Welcome to the Margo project! This document provides a comprehensive guide to setting up, deploying, and understanding the architecture and components of the Margo system. The Margo initiative defines mechanisms for interoperable orchestration at scale of edge applications/workloads and devices. It will deliver the interoperability promise through an open standard, a reference implementation and comprehensive compliance testing toolkit.  Margo unlocks barriers to innovation in complex multi-vendor environments and accelerates digital transformation for organizations of all sizes. More about MARGO [here](https://margo.org/)

---

## 🚀 Quick Start
This section provides an overview of the quick start steps for the project. More details are available in the respective sections below. It includes setting up your environment, building the project, deploying it, and running it.  

### 🛠️ Dev Toolset
Details of different [Dev Toolset](./docs/dev-toolsets.md) used in the development. 


#### 🔧 How to Build
- **Environment**: Setup prerequisites and configurations
- **Steps**: Step-by-step instructions to build the project

#### 🚚 How to Deploy
- **3 VM Architecture**: Margo envision 3 VM architecture for local setup where one VM is for WFM, one for stand alone cluster using k3s device and 1 more for standalone docker compose device.

    1. **WFM-VM**: WFM setup has been done using symphony, harbor and gogs. Also runs observability stack( Jaegar, Promtheus, Grafana and Loki)

    2. **K3s-Device-VM**: Using k3s as the standalone device. Runs device-agent, OTEL colletor, promtail and workloads deployed as k3s pods.

    3. **Docker-compose-Device-VM**: Using docker-compose as the standalone device. Runs device-agent, OTEL colletor, promtail and workloads deployed as docker containers.

   

- **VM Environment**: Configuration details for each VM. This size might vary based on number of workloads to be deployed on device and actual load post deployment of workloads. Below is for stable workload validation in devlopment environment.

    | VM Type                | OS            | VM Size                   |
    |------------------------|---------------|---------------------------| 
    | WFM                    | Ubuntu/Debian | (8 CPU, 16 GB RAM, 100 GB)|
    | K3s Device             | Ubuntu/Debian | (8 CPU, 16 GB RAM, 50 GB) |
    | Docker-Compose Device  | Ubuntu/Debian | (8 CPU, 16 GB RAM, 50 GB) |
    
- **Steps**: [WFM and Device-Agent Setup Guide](./pipeline/README.md)

#### ▶️ How to Run
- **Steps**: Execution instructions
- **Automation**: Scripts and tools for automated runs

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
