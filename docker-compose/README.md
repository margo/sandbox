##### [Back To Main](../README.md)
# Pre-requisites:
- Docker and Docker Compose installed
- Ensure that you have the container image for the device's Workload Fleet Management Client locally. The Compose file uses `pull_policy: never` by default, so build it if it is not already available:
```bash
cd sandbox
docker build -f poc/device/agent/Dockerfile . -t margo.org/workload-fleet-management-client:latest
cd docker-compose
```

The image reference can also be changed with the `workload_Fleet_Management_Client_IMAGE_REF` environment variable.

# Main steps:

1. Create the configuration directories and copy the example configuration files:
```bash
mkdir -p config/identity config/mis data
cp -r ../poc/device/agent/config/* ./config/
```

2. Ensure that the following files and directories exist on the host before starting the container. They are mounted at the paths shown inside the container:

```text
docker-compose/
|-- config/
|   |-- config.yaml
|   |-- capabilities.json
|   |-- identity/
|   |   |-- payload-cert.pem
|   |   `-- payload-key.pem
|   `-- mis/
|       |-- https-ca.crt
|       `-- trust-bundle.json
`-- data/
```

The MIAF-related entries in `config/config.yaml` require the following:
- `config/identity/payload-cert.pem` is the SVID certificate.
- `config/identity/payload-key.pem` is the private key for the SVID certificate. Protect this file appropriately.
- `config/mis/https-ca.crt` is the CA certificate used to verify the MIS endpoint.
- `config/mis/trust-bundle.json` contains the static JWK set SPIFFE trust bundle, used as a fallback for Operator authentication when MIS trust-bundle discovery is unavailable.
- `config/capabilities.json` contains the device capabilities referenced by `capabilities.readFromFile`.
- `data/` is the persistent database directory and is mounted at `/var/lib/margo/device-agent/data`.

> **Authorization:** The device agent is authorized to connect to the WFM whose ID is present in its own SPIFFE ID, which is contained in the SVID certificate (`payload-cert.pem`).

3. Set the runtime and service parameters in `config/config.yaml`. The supplied configuration selects Kubernetes; to manage Kubernetes, uncomment the kubeconfig volume mount in `docker-compose.yaml` and ensure the host kubeconfig exists at `/root/.kube/config`. To manage Docker instead, select the `DOCKER` runtime in `config.yaml`; the Compose file already mounts `/var/run/docker.sock`.

4. Update `config/config.yaml` and `config/capabilities.json` for your environment, including the MIS endpoint, WFM SBI URL, runtime, and state-seeking interval.

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
- The Workload Fleet Management Client has Docker socket access to manage Docker runtimes when the Docker runtime is selected
- Configuration, MIAF identity, MIS trust, and capability files are mounted from the local `config/` directory
- The device agent is authorized to connect to the WFM whose ID is present in its own SPIFFE ID, contained in the SVID certificate
- `config/mis/trust-bundle.json` provides a static JWK set SPIFFE trust bundle for Operator fallback authentication when dynamic trust-bundle discovery is unavailable
- Data persistence is handled through the `data/` directory mount
- Container logs use the `json-file` driver with 10 MB files and three rotated files
- The container will restart automatically unless stopped manually
