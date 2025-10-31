
##### [Back To Main](../README.md)
## 🔧 How to build Sandbox

- **Environment**: To build the Sandbox, ensure you have:
  - **GitHub Access**: Username for MARGO repository access and [Generate](../pipeline/README.md#-prerequisites#) valid GitHub token.
  - **System Requirements**:  [ Ubuntu/Debian-based VM requirements](./deploy.md#vm-requirements). 
  - **Network**: Internet connectivity for downloading dependencies.
  - **Environment Variables**: [Export Enviroment Varibales](../pipeline/README.md#step-1-environment-variables-setup).
    

- **Steps**: Post exporting environment variables refer below steps to build the Sandbox:

  1. **Setup Prerequisites**
     ```bash
     # Install basic utilities, Go, Docker, Helm, k3s etc
     ./wfm.sh  # Choose option 1: PreRequisites Setup
     ```

  2. **Configure Infrastructure Services**
     ```bash
     # Step-1 sets up Harbor registry, Gogs Git service, clones repositories and automatically configures container registry and Git repositories with predefined application packages like Nextcloud, Nginx and Custom OTEL.
     ```

  3. **Build and Start Symphony API**
     ```bash
     # Builds containerized Symphony API with TLS enabled
     # This needs to be ran on WFM VM
     ./wfm.sh  # Choose option 3: Symphony Start
     ```

  4. **Setup Device Agent** (Choose deployment method either Docker or K3s device)
     ```bash
     # This needs to be ran on device VM, below option(s) perform both building and running device-agent in respective device type (either docker-compose or k3s device)
     
     # For Docker deployment:
     ./device-agent.sh  # Choose option 3: Device-agent-Start(docker-compose-device)

     # For Kubernetes deployment:
     ./device-agent.sh  # Choose option 5: Device-agent-Start(k3s-device)
     ```

  5. **Optional: Install Observability Stack**
     ```bash
     # Installs Jaeger, Prometheus, Grafana, and Loki on WFM VM
     ./wfm.sh  # Choose option 5: ObservabilityStack Start
  
     # Installs OpenTelemetry Collecor and Promtail on Device VM
     ./device-agent.sh # Choose option 8: otel-collector-promtail-installation
     ```

This setup creates a complete sandbox environment with WFM (Workload Fleet Manager) and Device-Agent components for experimenting with MARGO APIs and running CLI scenarios.