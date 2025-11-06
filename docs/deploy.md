##### [Back To Main](../README.md)
## 🚀 How to deploy Sandbox

 **3 VM Architecture**: You can setup the Code First Sandbox using 3 VMs on a single host, where one VM is for WFM, one for a K3s cluster as a single node and one more for a single node docker compose device.

   1. **WFM-VM**: WFM setup has been done using Symphony, Harbor and Gogs. Also runs observability stack(Jaegar, Promtheus, Grafana and Loki)
   2. **K3s-Device-VM**: Using k3s as the standalone device. Runs device-agent, OTEL colletor, promtail and workloads deployed as k3s pods.
   3. **Docker-compose-Device-VM**: Using docker-compose as the standalone device. Runs device-agent, OTEL colletor, promtail and workloads deployed as docker containers.
  

 **VM Environment**: Configuration details for each VM. This size might vary based on number of workloads to be deployed on device and actual load post deployment of workloads. Below is for stable workload validation in devlopment environment.

    | VM Type                | OS            | VM Size                   |
    |------------------------|---------------|---------------------------| 
    | WFM                    | Ubuntu/Debian | (8 vCPU, 16 GB RAM, 100 GB)|
    | K3s Device             | Ubuntu/Debian | (8 vCPU, 16 GB RAM, 50 GB) |
    | Docker-Compose Device  | Ubuntu/Debian | (8 vCPU, 16 GB RAM, 50 GB) |

  Note : Network configuration for the VMs should use the host-network, with static IPs assigned to the VMs.

-->> This section below needs to re-written
 
**Deployment Configurations**:
  
  Use the WFM VM for building and deploying a containerized instance of Symphony API
  ```bash   
    ./wfm.sh  # Choose option 3: Symphony Start
  ```  
  # Device Agent as docker container
  # start_device_agent_docker_service()  
  # For Docker deployment:
  ```bash   
    ./device-agent.sh  # Choose option 3: Device-agent-Start(docker-compose-device)
  ```
  # Device Agent as pod   
  # build_start_device_agent_k3s_service()
  # For Kubernetes deployment:
  ```bash
    ./device-agent.sh  # Choose option 5: Device-agent-Start(k3s-device)
  ```
  
**Deployment Verification**:
  ```bash
  # Check WFM status
  # Check container logs
  docker logs -f symphony-container-name
  
  # Check Device Agent status
  ./device-agent.sh  # Choose option 7: Device-agent-Status
  OR
  docker logs -f device-agent-container-name (For docker-compose device) 
  OR
  kubectl logs -f device-agent-pod-name (For k3s device)
  
  # Verify observability (if installed)
  Grafana: http://${WFM_IP}:32000 (admin/admin)
  Jaeger: http://${WFM_IP}:32500
  Prometheus: http://${WFM_IP}:30900
  ```

**Clean Deployment**:
  ```bash
  # On WFM
  ./wfm.sh  # Option 4: Symphony Stop
  ./wfm.sh  # Option 2: PreRequisites Cleanup
  ./wfm.sh  # Option 6: ObservabilityStack Stop
  
  
  # On Device
  ./device-agent.sh  # Option 4 or 6: Device-agent Stop
  ./device-agent.sh  # Option 2: Uninstall-prerequisites
  ./device-agent.sh  # Option 9: otel-collector-promtail-uninstallation
  ./device-agent.sh  # Option 11: cleanup-residual
  ```

This deployment setup supports both development and production-like environments with TLS-enabled communications and comprehensive observability stack.
