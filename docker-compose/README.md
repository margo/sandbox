##### [Back To Main](../README.md)
# Pre-requisites:
- Docker and Docker Compose installed
- Ensure that you have the container image for the device's Workload Fleet management Client with you, if not you can build it using the following command (assuming that you cloned the entire sandbox at one place). To build, please run the following command:
```bash
cd sandbox
docker build -f poc/device/agent/Dockerfile . -t margo.org/workload-fleet-management-client:latest
cd docker-compose
```

# Main steps:

1. Copy the config.yaml and capabilities.yaml files to the config directory:
```bash
mkdir -p config
cp -r ../poc/device/agent/config/* ./config/
```

2. Create a data directory for persistent storage:
```bash
mkdir -p data
```

3. (Optional) If you need Kubernetes runtime management, uncomment the kubeconfig volume mount in docker-compose.yaml and ensure your kubeconfig is available at `/root/.kube/config`

4. Change the parameters as per your needs in `config/config.yaml` and `config/capabilities.json`

5. Start the device's Workload Fleet Management Client using Docker Compose:
```bash
docker compose up -d
```

6. To view logs:
```bash
docker compose logs -f workload-fleet-management-client
```

7. To stop the service:
```bash
docker compose down
```

# Notes:
- The Workload Fleet Management Client runs with Docker socket access to manage Docker runtimes
- Configuration files are mounted from the local `config/` directory
- Data persistence is handled through the `data/` directory mount
- The container will restart automatically unless stopped manually
