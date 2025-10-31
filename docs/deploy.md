##### [Back To Main](../README.md)
## 🚀 How to deploy Sandbox

 **3 VM Architecture**: Margo envision 3 VM architecture for local setup where one VM is for WFM, one for stand alone cluster using k3s device and one more for standalone docker compose device.

   1. **WFM-VM**: WFM setup has been done using Symphony, Harbor and Gogs. Also runs observability stack(Jaegar, Promtheus, Grafana and Loki)
   2. **K3s-Device-VM**: Using k3s as the standalone device. Runs device-agent, OTEL colletor, promtail and workloads deployed as k3s pods.
   3. **Docker-compose-Device-VM**: Using docker-compose as the standalone device. Runs device-agent, OTEL colletor, promtail and workloads deployed as docker containers.
  

 **VM Environment**: Configuration details for each VM. This size might vary based on number of workloads to be deployed on device and actual load post deployment of workloads. Below is for stable workload validation in devlopment environment.

    | VM Type                | OS            | VM Size                   |
    |------------------------|---------------|---------------------------| 
    | WFM                    | Ubuntu/Debian | (8 CPU, 16 GB RAM, 100 GB)|
    | K3s Device             | Ubuntu/Debian | (8 CPU, 16 GB RAM, 50 GB) |
    | Docker-Compose Device  | Ubuntu/Debian | (8 CPU, 16 GB RAM, 50 GB) |
  
 
**Deployment Configurations**:
  ```bash
  # Builds and deploys containerized Symphony API 
  # start_symphony_api_container() 
     
    ./wfm.sh  # Choose option 3: Symphony Start
    
  # Device Agent as docker container
  # start_device_agent_docker_service()  
  # For Docker deployment:
     
    ./device-agent.sh  # Choose option 3: Device-agent-Start(docker-compose-device)

  # Device Agent as pod   
  # build_start_device_agent_k3s_service()
  # For Kubernetes deployment:

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