

## 🔧 How to Build

- **Environment**: To build the project, ensure you have:
  - **GitHub Access**: Valid GitHub token and username for repository access
  - **System Requirements**: Ubuntu/Debian-based system with passwordless sudo access
  - **Network**: Internet connectivity for downloading dependencies
  - **Environment Variables**: 
    ```bash
    export GITHUB_USER="your-github-username"
    export GITHUB_TOKEN="your-github-token"
    export SYMPHONY_BRANCH="margo-dev-sprint-6"
    export DEV_REPO_BRANCH="dev-sprint-6"
    ```
    **Note:** Refer examples mentioned in [WFM and Device-Agent Setup Guide](../pipeline/README.md#step-1-environment-variables-setup) for exporting Environment variables.

- **Steps**: Step-by-step instructions to build the project:

  1. **Setup Prerequisites**
     ```bash
     # Install basic utilities, Go 1.24.4, Docker, Helm v3.15.1, and k3s
     ./wfm.sh  # Choose option 1: PreRequisites Setup
     ```

  2. **Configure Infrastructure Services**
     ```bash
     # Sets up Harbor registry, Gogs Git service, and clones repositories
     # Automatically configures container registry and Git repositories
     ```

  3. **Build and Start Symphony API**
     ```bash
     ./wfm.sh  # Choose option 3: Symphony Start
     # Builds containerized Symphony API with TLS enabled
     ```

  4. **Setup Device Agent** (Choose deployment method)
     ```bash
     # For Docker deployment:
     ./device-agent.sh  # Choose option 3: Device-agent-Start(docker-compose-device)
     
     # For Kubernetes deployment:
     ./device-agent.sh  # Choose option 5: Device-agent-Start(k3s-device)
     ```

  5. **Optional: Install Observability Stack**
     ```bash
     ./wfm.sh  # Choose option 5: ObservabilityStack Start
     # Installs Jaeger, Prometheus, Grafana, and Loki
     ```

This setup creates a complete sandbox environment with WFM (Workflow Manager) and Device-Agent components for experimenting with MARGO APIs and running CLI scenarios.