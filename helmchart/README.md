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

# Main steps:
1. Copy the config.yaml and capabilities.json files in this directory.
```bash
cp -r ../poc/device/agent/config/* .
```

2. Change the params as per your need in these config.yaml and capabilities.json files.

3. Install the chart in default namespace:
```bash
helm install workload-fleet-management-client
```

4. Authentication Method:
This Helm chart uses ServiceAccount-based authentication to connect with the Kubernetes API server. The chart automatically creates:

ServiceAccount for the workload-fleet-management-client pod
ClusterRole with necessary permissions
ClusterRoleBinding to link ServiceAccount with permissions
The workload-fleet-management-client will authenticate using the ServiceAccount token automatically mounted by Kubernetes at /var/run/secrets/kubernetes.io/serviceaccount/token.

```bash
Note: Refer build_start_device_agent_k3s_service() in /sandbox/pipeline/device-agent.sh for details of the method used for creation of ServiceAccount , ClusterRole and ClusterRoleBinding. Also code ensures that the workload-fleet-management-client's ServiceAccount has the necessary permissions to interact with Kubernetes resources, particularly secrets and configmaps.
```

5. Verification:

```bash
# Check if pods are running
kubectl get pods -n default

# Check ServiceAccount and RBAC resources
kubectl get serviceaccount,clusterrole,clusterrolebinding -n default | grep workload-fleet-management-client

# Check logs
kubectl logs -n default deployment/workload-fleet-management-client-deploy

```
6. Cleanup:
```bash
# Uninstall Helm release
helm uninstall workload-fleet-management-client --namespace default

# Clean up RBAC resources (if needed)
kubectl delete clusterrole workload-fleet-management-client-role
kubectl delete clusterrolebinding workload-fleet-management-client-binding



```
