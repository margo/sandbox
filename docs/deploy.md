##### [Back To Main](../README.md)
## 🚀 How to deploy Sandbox

 **1. 3-VM Architecture**: You can setup the Code First Sandbox using 3 VMs on a single host, where one VM is for WFM, one for a K3s cluster as a single node, and one more for a single node docker compose device. The Sandbox is tested with non cloud environment and all 3 VMs must have static IPs with connectivity between them.

   1. **WFM-VM**: WFM setup has been done using Symphony, Harbor and Gogs. Also runs observability stack(Jaeger, Prometheus, Grafana and Loki)
   2. **K3s-Device-VM**: Using k3s as the standalone device. Runs device-agent, OTEL colletor, promtail and workloads deployed as k3s pods.
   3. **Docker-compose-Device-VM**: Using docker-compose as the standalone device. Runs device-agent, OTEL colletor, promtail and workloads deployed as docker containers.
  

 **2. VM Environment**: Configuration details for each VM. This size might vary based on the number of workloads to be deployed on device and actual load post deployment of workloads. Below is for stable workload validation in development environment.

    | VM Type                | OS            | VM Size                    |
    |------------------------|---------------|----------------------------| 
    | WFM                    | Ubuntu/Debian | (8 vCPU, 16 GB RAM, 100 GB)|
    | K3s Device             | Ubuntu/Debian | (8 vCPU, 16 GB RAM, 50 GB) |
    | Docker-Compose Device  | Ubuntu/Debian | (8 vCPU, 16 GB RAM, 50 GB) |

  **Note:** Network configuration for the VMs should use the host-network, with static IPs assigned to the VMs.


 
**3. Deployment Configurations**:
In order to deploy the Sandbox, clone the [MARGO repository](https://github.com/margo/dev-repo) and run the following [Automation Scripts](https://github.com/margo/dev-repo/tree/main/pipeline).       

  **3.1** Deploy containerized instance of Symphony API on WFM VM.
   
     ```bash   
      # Choose option 3: Symphony: Start
      ./wfm.sh  
     ```  
     ```bash
     
  **3.2** Generate certs on K3s-Device-VM.   
    # This should be executed on device-agent VM after installing pre-req (option 1:Install-prerequisites)
    # Choose below options to generate RSA and ECDSA certificates
      Option 12) create_device_rsa_certs
      option 13) create_device_ecdsa_certs

  **3.3** Copy ca-crt.pem from WFM-VM to K3s-Device-VM.
    # ca-crt.pem is generated after the synphony-api is started on WFM-VM (option 3 - Symphony: Start)
       Locate the ca-cert.pem on the WFM-VM at : _cd $HOME/symphony/api/certificates_
    # Copy the ca-crt.pem file to the following destination paths on the Device VM:
       For Docker Compose–based Device-Agent: : cd $HOME/dev-repo/docker-compose/config
       For K3s-based Device-Agent: : $HOME/certs
      
  **3.4** Deploy containerized instance of Device Agent on Docker device.
    
    ```bash  
    # Choose option 3: Device-agent-Start(docker-compose-device)
      ./device-agent.sh  
    ```
  **3.5** Deploy containerized instance of Device Agent on k3s device.
   
    ```bash
    # Choose option 5: Device-agent-Start(k3s-device)
      ./device-agent.sh  
    ```
  
**4. Deployment Verification**:
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

**5. Clean Deployment**:
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
