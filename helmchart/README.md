##### [Back To Main](../README.md)
# Pre-requisites:
- Kubernetes runtime (k3s/k8s)
- Ensure that you have the container image for the device workload-fleet-management-client with you, if not you can build it using the following command(assuming that you cloned the entire sandbox at one place). To build, please run the following command (you can use nordctl or other tools as per your preference):
```bash
cd sandbox
docker build -f poc/device/agent/Dockerfile . -t margo.org/workload-fleet-management-client:latest
docker save -o workload-fleet-management-client.tar margo.org/workload-fleet-management-client:latest
# use this command if on k8s cluster
ctr -n k8s.io image import workload-fleet-management-client.tar
# use this command if on k3s cluster
k3s ctr -n k8s.io image import workload-fleet-management-client.tar
cd helmchart
```

# Files required by the chart
The chart reads non-sensitive configuration from `config/config.yaml` and `config/capabilities.json`. It reads the following sensitive or certificate files from the chart root when `secrets.create` is enabled:

```text
helmchart/
|-- harbor.crt
|-- https-ca.crt
|-- payload-cert.pem
|-- payload-key.pem
|-- authorized.json
`-- config/
	|-- config.yaml
	`-- capabilities.json
```

The files must exist before running `helm install` or `helm template`; Helm embeds them in the generated Kubernetes Secret and ConfigMap. Do not commit `payload-key.pem` or other private credentials to source control.

The MIAF-related entries in `config/config.yaml` require:
- `payload-cert.pem`: the SVID certificate, mounted in the pod as `/config/identity/payload-cert.pem`.
- `payload-key.pem`: the private key for the SVID certificate, mounted as `/config/identity/payload-key.pem`.
- `https-ca.crt`: the CA certificate used to verify the MIS endpoint, mounted as `/config/mis/https-ca.crt`.
- `authorized.json`: the authorized clients file, mounted as `/config/authorized.json`.
- `config/capabilities.json`: the device capabilities file referenced by `capabilities.readFromFile` and mounted as `/config/capabilities.json`.

The chart also requires `harbor.crt` for the Harbor CA mount at `/usr/local/share/ca-certificates/harbor.crt`. The configured MIS endpoint must be reachable from the pod. If MIS trust-bundle discovery is unavailable and the corresponding settings are enabled in `config/config.yaml`, `trust-bundle.json` must also be made available at `/config/mis/trust-bundle.json`; the current chart templates do not package or mount this optional file, so the ConfigMap/Secret templates must be extended before using that fallback.

The supplied configuration selects the Kubernetes runtime and uses in-cluster ServiceAccount authentication. To manage Docker instead, select the `DOCKER` runtime in `config/config.yaml` and provide the required Docker socket/runtime configuration; this chart does not currently mount `/var/run/docker.sock`.

# Installation
1. Copy the configuration files into the chart's `config/` directory and the certificate/authorization files into the chart root:
```bash
cp ../poc/device/agent/config/config.yaml config/config.yaml
cp ../poc/device/agent/config/capabilities.json config/capabilities.json
cp /path/to/harbor.crt harbor.crt
cp /path/to/https-ca.crt https-ca.crt
cp /path/to/payload-cert.pem payload-cert.pem
cp /path/to/payload-key.pem payload-key.pem
cp /path/to/authorized.json authorized.json
```

2. Update `config/config.yaml` and `config/capabilities.json` for the target environment, including the MIS endpoint, WFM SBI URL, runtime, and state-seeking interval. If persistence is enabled, align `database.dataDir` with the deployment mount at `/data` (or update the deployment mount to `/var/lib/margo/device-agent/data`) so data is written to the PVC.

3. Review `values.yaml` and install the chart in the desired namespace. Persistence is enabled by default and creates a 1 GiB PVC unless `persistence.existingClaim` is set:
```bash
helm install workload-fleet-management-client . --namespace default --create-namespace
```

# Authentication and permissions
With `rbac.create: true` (the default), the chart creates:

- A ServiceAccount for the client pod
- A ClusterRole with permissions for the configured workload management operations
- A ClusterRoleBinding connecting the ServiceAccount to the ClusterRole

The client authenticates with the Kubernetes API using the ServiceAccount token mounted automatically by Kubernetes at `/var/run/secrets/kubernetes.io/serviceaccount/token`. Set `rbac.create: false` only when equivalent permissions and a ServiceAccount are managed separately.

# Verification

```bash
# Check the release and pod
helm status workload-fleet-management-client --namespace default
kubectl get pods -n default

# Check generated configuration resources
kubectl get configmap,secret,pvc -n default | grep workload-fleet-management-client

# Check ServiceAccount and RBAC resources
kubectl get serviceaccount,clusterrole,clusterrolebinding -n default | grep workload-fleet-management-client

# Check logs
kubectl logs -n default deployment/workload-fleet-management-client-deploy
```

# Render locally before installation
```bash
helm template workload-fleet-management-client . --namespace default
```

# Cleanup
```bash
helm uninstall workload-fleet-management-client --namespace default
```

The generated PVC is retained by the chart's resource policy. Delete it separately only when its stored device-agent state is no longer needed.
