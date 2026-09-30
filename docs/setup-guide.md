##### [Back To Main](../README.md)
# Setting Up the Code First Sandbox

## What You'll Need

**Three Virtual Machines:**
| VM Type | Processors(vCPU) | Memory | Storage | Purpose |
|---------|-----------|--------|---------|---------|
| **Main VM (WFM)** | 8 | 16GB | 100GB | Workload Fleet Manager as well as Margo Identity Service  |
| **Device VM 1 (Helm-capable device)** | 4 | 4-8GB | 50GB | Kubernetes-based device |
| **Device VM 2 (Compose-capable device)** | 4 | 4-8GB | 50GB | Docker-based device |

**Requirements:**
- Ubuntu operating system (**ubuntu-24.04.3-desktop-amd64 or server**) (you can check by doing ```cat /etc/os-release```)
   - Virtual Machine Manager (4.1.0 tested)
- Internet connection
- All VMs must be able to talk to each other (same network with static IP addresses)
- VM hostnames must be lowercase.

> Warning: If you are attempting to deploy this on corporate machines or within a corporate network, you will need to address any special networking requirements or access issues to enable internet communication (e.g, proxy configuration, certificates, firewall configuration, etc.). This falls outside the of the scope of this documentation. This warning applies to both the Main and the Device VMs when running the setup scripts('wfm.sh' & 'device-agent.sh').
---


## Step 1: Get the Setup Files

You need to download the setup files to all 3 VMs. Follow these steps on **each VM**:

1. **Open Terminal**
   - On your WFM VM, open the terminal/command line application

2. **Install Git (if not already installed)**
   ```bash
   sudo apt-get update
   sudo apt-get install git -y
   ```

3. **Create a workspace directory**
   ```bash
   mkdir -p $HOME/workspace
   cd $HOME/workspace
   ```

4. **Download the Setup Files**
   ```bash
   git clone --filter=blob:none --sparse https://github.com/margo/sandbox.git
   cd sandbox
   git sparse-checkout init --no-cone
   git sparse-checkout set \
      scripts/*
   git checkout main
   ```
---

## Step 2: Set Up Environment

On each VM, you need to configure environment variables (settings that tell the system where things are).

1. **Navigate to the scripts folder**
   ```bash
   cd $HOME/workspace/sandbox/scripts
   ```

2. **Set Environment Variables**

   Open and follow the [Environment Variables Setup Guide](../docs/env-setup.md)

   This will help you set up:
   - GitHub credentials (optional)
   - VM IP addresses
   - Network settings
   - Other required configurations

3. **Configure the domain/host(name) resolution locally**
   1. Open `/etc/hosts` file (create if it doesn't exist)
   2. Then append the following entries to the file:
      ```bash
      <ip-address-of-the-wfm-machine> symphony.machine
      <ip-address-of-the-harbor-machine> harbor.machine
      <ip-address-of-the-mis-machine> mis.margo.org #just an example, should be same as EXPOSED_MIS_HOST in mis.env
      ```
      your file would look something like this:
      ```bash
      127.0.0.1 localhost
      # The following lines are desirable for IPv6 capable hosts
      ::1 ip6-localhost ip6-loopback
      fe00::0 ip6-localnet
      ff00::0 ip6-mcastprefix
      ff02::1 ip6-allnodes

      192.11.11.11 symphony.machine # <---- newly appended line here with ip
      192.11.11.11 harbor.machine # <--- newly appended line with ip
      192.11.11.11 mis.margo.org # <--- newly appended line with ip
      ```

🔴 **Important:** Complete these steps on all 3 VMs before proceeding.

---

## Step 3: Build Everything

> **Note:** If during setup you see any error like the following: ```ERROR:  429 Too Many Requests
   toomanyrequests: You have reached your unauthenticated pull rate limit. https://www.docker.com/increase-rate-limit```. This is because docker allows certain number of anonymous image pulls in a day, and yours have exhausted. Please login using your dockerhub account. The command to do so is: `docker login -u <your-dockerhub-account-name>` , then it'll ask for the password once you execute this command.

### On the WFM VM:

> **Note:** Margo Identity Service(MIS) is installed on WFM VM. This can be installed on a separate VM.

#### Build and Run MIS

1. **Navigate to the scripts folder**
   ```bash
   cd $HOME/workspace/sandbox/scripts
   ```

2. **Install Basic Tools**
   ```bash
    sudo -E bash mis.sh
   ```
   - A menu will appear
   - Type `1` and press Enter
   - Choose: `Option 1: PreRequisites: Setup`

   This installs everything needed like Docker and other tools. This may take 1-5 minutes.

3. **Generate Root CAs for HTTPS server and for minting SVIDs for principals**
   ```bash
    sudo -E bash mis.sh
   ```
   - A menu will appear
   - Type `3` and press Enter
   - Choose: `Option 3: Factory Bootstrap: Generate Root CAs`

   It will place Root CAs in `$HOME/mis-deployment/certs`
   These are the files and their use cases:

   | File Path | Description |
   |-----------|-------------|
   | `$HOME/mis-deployment/certs/https-ca.key` | Private key of the self-signed HTTPS Root CA. Used to sign the HTTPS Normative Server certificate (`https-server.crt`). |
   | `$HOME/mis-deployment/certs/https-ca.crt` | Self-signed HTTPS Root CA certificate (valid for 10 years). Acts as the trust anchor for TLS clients and is used to validate the HTTPS Normative Server certificate (`https-server.crt`). |
   | `$HOME/mis-deployment/certs/https-server.key` | Private key corresponding to the HTTPS Normative Server certificate (`https-server.crt`). Used by the HTTPS Normative Server during TLS handshakes. |
   | `$HOME/mis-deployment/certs/https-server.crt` | Server certificate for the HTTPS Normative Server (valid for 1 year), signed by the HTTPS Root CA (`https-ca.crt` / `https-ca.key`). Presented to clients during TLS connections. |
   | `$HOME/mis-deployment/certs/ca.key` | Private key of the self-signed SVID Root CA. Used by the SVID generator to sign X.509 SVID certificates. |
   | `$HOME/mis-deployment/certs/ca.crt` | Self-signed SVID Root CA certificate (valid for 10 years). Serves as the trust anchor for X.509 SVIDs minted by the SVID generator using SPIFFE IDs. |

   The script also verifies the generated chain and prints a summary on completion.

   >Note: Docker image for Margo Identity Service has been already built and pushed to Margo GHCR registry from where the below script pull the image and starts MIS.

   > **Note on PKI Material — Self-Signed Root CA:**
   >
   > This step generates **self-signed Root Certificate Authorities (CAs)** for use as PKI material within the sandbox. Specifically, two self-signed Root CAs are created:
   >
   > - **Minter CA** (`ca.crt` / `ca.key`): Acts as the SPIFFE trust anchor for the
   >   configured Trust Domain. Used exclusively to sign X.509 SVIDs issued by MIS.
   > - **HTTPS CA** (`https-ca.crt` / `https-ca.key`): Used to sign the HTTPS server
   >   certificate (`https-server.crt`) that secures the normative Trust Bundle API.
   >   Clients connecting to MIS must trust this CA to establish the initial TLS connection.
   >
   > The self-signed approach is intentional for sandbox and proof-of-concept use. It keeps
   > the environment fully self-contained without requiring an external PKI infrastructure.
   >
   > **Supplying operator-provided PKI material is not currently supported in this
   > automated script-based deployment.** The `mis.sh` script does not accept externally
   > issued certificates as input to this step. If you require integration with your own
   > PKI infrastructure (e.g., an enterprise CA or HSM-backed CA), you must supply the
   > required certificate and key files manually after this step, replacing the generated
   > files at `$HOME/mis-deployment/certs/` with your own material, and ensuring their
   > correctness and trustworthiness independently.
   >
   > For a detailed explanation of the PKI trust model, the role of each certificate, and
   > guidance on supplying your own PKI infrastructure, see the [MIS PKI Setup and Trust Model — Note on PKI Trust Model](../mis/README.md#note-on-pki-trust-model) documentation.


   To replace the Root CA and perform a trust bundle reset after initial setup, see [Trust Bundle Revocation and Root CA Replacement](./identity-lifecycle.md#trust-bundle-revocation-and-root-ca-replacement-sandbox) in the Identity Lifecycle guide.

3. **Start the Margo Identity Service**
   ```bash
    sudo -E bash mis.sh
   ```
   - Type `4` and press Enter
   - Choose: `Option 4:  Margo Identity Service: Install`

   This starts the Margo Identity Service.

4. **Verify the Margo Identity Service Is Running Correctly**
   ```bash
   sudo docker logs -f margo-identity-service
   ```
   You should see log messages indicating the service is running. Press `Ctrl+C` to exit.


### Generate X.509-SVIDs for WFM and WFM client

1. **Navigate to the scripts folder**
   ```bash
   cd $HOME/workspace/sandbox/scripts
   ```

2. **Generate SVIDs interactively for both WFM and WFM client**
   ```bash
    sudo -E bash mis.sh
   ```
   - A menu will appear
   - Type `6` and press Enter
   - Choose: `Option 6: Generate SVID`

   This step produces below files at the path `$HOME/workspace/sandbox/scripts`

   Default trust domain is picked up from mis.env (refer to [Environment Variables Setup Guide](../docs/env-setup.md)) 

   **For WFM**
   ```
   STEP: Principal Selection: Select the principal for which to generate, Enter option 1)

   $HOME/workspace/sandbox/scripts/x509svid-wfm
   -r-------- 1 root root 227 Sep 11 07:20 payload-key.pem
   -rw------- 1 root root 607 Sep 11 07:20 payload-cert.pem
   ```
   >**Note:** `x509svid-wfm`, where `wfm` is WFM ID provided while running generator script interactively. Furthermore, `x509svid-wfm` is created in current working directory. Note down the SPIFFE IDs for later use in enabling communication in local authorization policy of WFM Client. 

   **For WFM Client**
   ```
    STEP: Principal Selection: Select the principal for which to generate, Enter option 2)

   🔴 This needs to be ran twice; for `Compose-capable device` and for `Helm-capable device`. Same steps can be used to generate as SVIDs for as many devices as required. 

   $HOME/workspace/sandbox/scripts/x509svid-wfm-docker-client
   -r-------- 1 root root 227 Sep 11 07:22 payload-key.pem
   -rw------- 1 root root 631 Sep 11 07:22 payload-cert.pem

   $HOME/workspace/sandbox/scripts/x509svid-wfm-helm-client
   -r-------- 1 root root 227 Sep 11 07:22 payload-key.pem
   -rw------- 1 root root 631 Sep 11 07:22 payload-cert.pem
   ```
   >**Note:** `x509svid-wfm-docker-client`, where `docker-client` is WFM client ID for compose-capable device and
   `x509svid-wfm-helm-client`, where `helm-client` is WFM client ID for helm-capable device, provided while running generator script interactively. Directories containing SVID & key are created in current working directory. Note down the SPIFFE IDs for later use in enabling communication in local authorization policy of WFM. 

   To renew or reissue these SVIDs after initial setup, see [Renewal and Reissuance](./identity-lifecycle.md#renewal-and-reissuance) in the Identity Lifecycle guide.


#### Build and Run WFM(Symphony)

1. **Navigate to the scripts folder**
   ```bash
   cd $HOME/workspace/sandbox/scripts
   ```

2. **Install Basic Tools**
   ```bash
    sudo -E bash wfm.sh
   ```
   - A menu will appear
   - Type `1` and press Enter
   - Choose: `Option 1: PreRequisites Setup`

   This installs everything needed like Redis, Docker, Helm, and other tools. This may take 10-15 minutes.

   > Note: Docker image for Workload Fleet Manager has been already built and pushed using CI pipeline to Margo GHCR registry from where the below script pull the image and starts WFM.

3. **Copy WFM SVIDs and MIS HTTPS server CA**
   ```bash
   cp $HOME/mis-deployment/certs/https-ca.crt $HOME/symphony/api/mis
   cp $HOME/workspace/sandbox/scripts/x509svid-wfm/payload-cert.pem $HOME/symphony/api/certificates
   cp $HOME/workspace/sandbox/scripts/x509svid-wfm/payload-key.pem $HOME/symphony/api/certificates
   ```
   > Note: Above commands need to be modified incase different wfm-id is used for generating WFM SVID. 

4. **Add WFM Client SPIFFE IDs as authorised clients interactively**

   This step acts as local authorization policy to allow/disallow wfm clients to connect with WFM(symphony). Add SpiffeIDs of WFM Client (Both Docker & Helm capable Device) to enable communication when device clients are started.
   ```bash
    sudo -E bash wfm.sh
   ```
   - A menu will appear
   - Type `7` and press Enter
   - Choose: `Option 7: Manage SPIFFE ID allowlist`

   Follow the steps interactively to add SPIFFE IDs of WFM Clients. These should be same as SPIFFE ID used to generate SVID for those WFM Clients. Use default path for file containing authorized clients, unless explicitly changed. 

   To revoke a device agent's access after initial setup, see [Device-Agent Revocation](./identity-lifecycle.md#device-agent-revocation-sandbox) in the Identity Lifecycle guide.

5. **Start the Workload Fleet Manager**
   ```bash
    sudo -E bash wfm.sh
   ```
   - Type `3` and press Enter
   - Choose: `Option 3: Symphony Start`

   This starts the Workload Fleet Manager service.


6. **Add Monitoring Tools**
   ```bash
    sudo -E bash wfm.sh
   ```
   - Type `5` and press Enter
   - Choose: `Option 5: ObservabilityStack Start`

   This adds tools to monitor workloads observability.

7. **Verify the Workload Fleet Manager Is Running Correctly**
   ```bash
   sudo docker logs -f symphony-api-container
   ```
   You should see log messages indicating the service is running. Press `Ctrl+C` to exit.

> Note: Services are configured to auto-start on VM reboot.
  However, if you encounter issues after reboot, you can manually restart them using the same menu options.


### On Each Device VM:

1. **Copy Security Files Between VMs (WFM Client SVIDs, HTTPS server CA and Harbor's CA to Device VM)**

   #### Step 1: Preparation on WFM VM

   | Step | Action | Command | Expected Result | Notes |
   |------|--------|---------|-----------------|-------|
   | 1 | Find WFM IP address | `hostname -I` | First IP address (e.g., 192.168.1.100) | Write down the IP address from Step 1 for use in the copy commands below. |
   | 2 | Locate Compose capable WFM client X.509 SVID | `$HOME/workspace/sandbox/scripts/`<br>`ls -la x509svid-wfm-docker-client` | Files: `payload-key.pem` and `payload-cert.pem`| `wfm-docker-client` in `x509svid-wfm-docker-client` is what is used in this guide. Use appropriate wfm client id if you changed it in SVID generation step |
   | 3 |  Locate Helm capable WFM client X.509 SVID | `$HOME/workspace/sandbox/scripts/`<br>`ls -la x509svid-wfm-helm-client` | Files: `payload-key.pem` and `payload-cert.pem`| `wfm-helm-client` in `x509svid-wfm-helm-client` is what is used in this guide. Use appropriate wfm client id if you changed it in SVID generation step |
   | 4 | Locate Harbor certificate | `cd $HOME/sandbox/scripts/harbor/certs`<br>`ls -la harbor.crt` | File: `harbor.crt` |  |
   | 5 | Locate MIS HTTPS CA certificate | `cd $HOME/mis-deployment/certs`<br>`ls -la https-ca.crt` | File: `https-ca.crt` | Acts as intial trust for connecting to MIS Normative APIs |

   **Note:** Write down the IP address from Step 1 for use in the copy commands below.

   #### Step 2: Prepare `$HOME/Certs` Directory on Device VM(s)
   In order to copy required certificates from WFM machine to Device VMs, create following directory(s) on Device VMs:
   ##### For Helm Capable Device VM
      ```bash
      mkdir -p $HOME/certs/helm-identity
      ```
   
   ##### For Compose Capable Device VM
      ```bash
      mkdir -p $HOME/certs/compose-identity
      ```
   
   Above commands will create a common `$HOME/certs` and based on requirement, it would create `helm-identity` or `compose-identity` sub directory for carrying identity certificates (SVID)

   #### Step 3: Copy Required Files from WFM VM to Device VM & Pre-requisite setup

   **Option A - Using SCP**
   🔴 **(Recommended - Run from Device VMs)**


   | Target VM | Run From | SCP Command | Example |
   |-----------|----------|-------------|---------|
   | **Docker Device** | Compose Capable Device VM | `scp username@WFM-VM-IP:~/workspace/sandbox/scripts/x509svid-wfm-docker-client/payload-key.pem $HOME/certs/compose-identity/` <br><br> `scp username@WFM-VM-IP:~/workspace/sandbox/scripts/x509svid-wfm-docker-client/payload-cert.pem $HOME/certs/compose-identity/` <br><br> `scp username@WFM-VM-IP:~/sandbox/scripts/harbor/certs/harbor.crt $HOME/certs/` <br><br> `scp username@WFM-VM-IP:~/mis-deployment/certs/https-ca.crt $HOME/certs/` | `scp azureuser@10.10.10.4:~/workspace/sandbox/scripts/x509svid-wfm-docker-client/payload-key.pem $HOME/certs/compose-identity/` <br><br> `scp azureuser@10.10.10.4:~/workspace/sandbox/scripts/x509svid-wfm-docker-client/payload-cert.pem $HOME/certs/compose-identity/` <br><br> `scp azureuser@10.10.10.4:~/sandbox/scripts/harbor/certs/harbor.crt $HOME/certs/` <br><br> `scp azureuser@10.10.10.4:~/mis-deployment/certs/https-ca.crt $HOME/certs/`|
   | **K3s Device** | Helm Capable Device VM | `scp username@WFM-VM-IP:~/workspace/sandbox/scripts/x509svid-wfm-helm-client/payload-key.pem $HOME/certs/helm-identity/` <br><br> `scp username@WFM-VM-IP:~/workspace/sandbox/scripts/x509svid-wfm-helm-client/payload-cert.pem $HOME/certs/helm-identity/` <br><br> `scp username@WFM-VM-IP:~/sandbox/scripts/harbor/certs/harbor.crt $HOME/certs/` <br><br> `scp username@WFM-VM-IP:~/mis-deployment/certs/https-ca.crt $HOME/certs/` | `scp azureuser@10.10.10.4:~/workspace/sandbox/scripts/x509svid-wfm-helm-client/payload-key.pem $HOME/certs/helm-identity/` <br><br> `scp azureuser@10.10.10.4:~/workspace/sandbox/scripts/x509svid-wfm-helm-client/payload-cert.pem $HOME/certs/helm-identity/` <br><br> `scp azureuser@10.10.10.4:~/sandbox/scripts/harbor/certs/harbor.crt $HOME/certs/` <br><br> `scp azureuser@10.10.10.4:~/mis-deployment/certs/https-ca.crt $HOME/certs/` |

   **Note:** Run with **sudo** if fails.

   **Replace:**
   - `username` with your WFM VM username
   - `WFM-VM-IP` with the IP address from Step 1


   **Option B - Manual Copy by creating respective files and copying contents**
   
   Manually create above files in Device VM(s) and copy content of those files from WFM VM to Device VM(s).

2. **Navigate to the scripts folder**
   ```bash
   cd $HOME/workspace/sandbox/scripts
   ```
3. **(Optional) Generate Labels for Device**

   If you want device to inherit user-defined labels, first create the required labels using the provided helper script.

   Run:

   ```bash
   bash create-device-labels.sh
   ```
   and follow on screen instructions to create user defined labels. Prefixing with an organization domain is RECOMMENDED for supplier-specific labels.

   Alternatively, if you have labels prepared & just want to use them without using above helper script, you can:
   - Create a labels.json file in current working directory
   - Paste your labels as a json object in labels.json, finally file should look like this: 
      ```json
      {
        "northstarida.com/hypervisor": "hyper-v",
        "northstarida.com/wasm.runtime": [
            "wamr"
        ],
        "northstarida.com/wasm.package.format": [
            ".wasm",
            ".aot"
        ],
        "northstarida.com/os": "zephyr"
      }
      ```

3. **Install Basic Tools**

   Based on the device type, select **k3s** or **docker** while sourcing the environment variables. For example:
   ```bash
   sudo -E bash device-agent.sh docker # for docker-compose device
   sudo -E bash device-agent.sh k3s    # for k3s device
   ```
   - Type `1` and press Enter
   - Choose: `Option 1: Install-prerequisites`

   This may take 10-15 minutes.


4. **Add WFM SPIFFE ID in local allow list policy for Device(s) interactively**

   Based on the device type, select **k3s** or **docker** while sourcing the environment variables. For example:
   ```bash
   sudo -E bash device-agent.sh docker # for docker-compose device
   sudo -E bash device-agent.sh k3s    # for k3s device
   ```
   - Type `11` and press Enter
   - Choose: `Option 11: Manage SPIFFE ID allowlist`

   Follow the steps interactively to add SPIFFE ID of WFM on devices.

   To revoke or update WFM authorization on the device agent after initial setup, see [WFM Revocation on Device Agent](./identity-lifecycle.md#wfm-revocation-on-device-agent-sandbox) in the Identity Lifecycle guide.

---

## Step 4: Deploy (Connect Everything)

### Start Device Services
> Note: Docker image for Workload Fleet Management client has been already built and pushed using CI pipeline to Margo GHCR registry from where the below script pull the image and starts WFM client.

**On Docker Device VM:**

1. **Navigate to the scripts folder**
   ```bash
   cd $HOME/workspace/sandbox/scripts
   ```

2. **Start the device's Workload Fleet Management Client**
   ```bash
    sudo -E bash device-agent.sh docker
   ```
   - Type `3` and press Enter
   - Choose: `Option 3: Device-agent-Start(docker-compose-device)`

3. **Check device status**
   ```bash
    sudo -E bash device-agent.sh docker
   ```
   - Type `7` and press Enter
   - Choose: `Option 7: Device-agent-Status`

4. **View device logs**
   ```bash
   # View the logs
   sudo docker logs -f workload-fleet-management-client
   ```
   You should see log messages indicating the service is running. Press `Ctrl+C` to exit the logs.

**On K3s Device VM:**

1. **Navigate to the scripts folder**
   ```bash
   cd $HOME/workspace/sandbox/scripts
   ```

2. **Start the device's Workload Fleet Management Client**
   ```bash
    sudo -E bash device-agent.sh k3s
   ```
   - Type `5` and press Enter
   - Choose: `Option 5: Device-agent-Start(k3s-device)`

3. **Check device status**
   ```bash
    sudo -E bash device-agent.sh k3s
   ```
   - Type `7` and press Enter
   - Choose: `Option 7: Device-agent-Status`

4. **View device logs**
   ```bash
   # View the logs (replace <pod-name> with actual pod name from above using #7)
   sudo kubectl logs -f <pod-name> -n default
   ```
   Example: `kubectl logs -f workload-fleet-management-client-deploy-5974667489-dw77w -n default`

   You should see log messages indicating the service is running. Press `Ctrl+C` to exit the logs.

> Note: Services are configured to auto-start on VM reboot.
  However, if you encounter issues after reboot, you can manually restart them using the same menu options.

### Add Monitoring to Devices
> Note : OTEL Collector: Pushes traces to Jaeger (port 30417) and metrics to Prometheus (port 30909). Promtail: Pushes logs to Loki (port 32100)
```
Device → Push Traces → WFM Jaeger (port 30417)
Device → Push Metrics → WFM Prometheus (port 30909)
Device → Push Logs → WFM Loki (port 32100)

Devices use a push-based architecture - they actively send data to WFM rather than being scraped. This works seamlessly across NAT/firewalls and requires no additional firewall configuration.
```

On each Device VM:
```bash
cd $HOME/workspace/sandbox/scripts
 sudo -E bash device-agent.sh docker # for docker-compose device
 sudo -E bash device-agent.sh k3s    # for k3s device
```
- Type `8` and press Enter
- Choose: `Option 8: otel-collector-promtail-installation`

> Note: Services are configured to auto-start on VM reboot.
  However, if you encounter issues after reboot, you can manually restart them using the same menu options.

## Step 4: Run and Use

Your sandbox environment is now fully set up and ready to use.

To manage application packages, deploy workloads to devices, and monitor your environment, refer to the **[Operations Guide](./operations-guide.md)**, which covers:

- **EasyCLI** — interactive interface for listing devices, uploading app packages, deploying and deleting instances
- **Monitoring dashboards** — Grafana, Jaeger, and Prometheus access and configuration
- **Pre-loaded sample applications** — Custom OTEL (Helm) and Nextcloud (Compose)

## Cleaning Up (Starting Fresh)

> If you are cleaning up due to a security event or identity compromise, review the [Identity Lifecycle and Operator Playbooks](./identity-lifecycle.md) guide before proceeding, as trust bundle revocation and identity reissuance may be required.

If you want to remove everything and start over:

### On WFM VM:

1. **Navigate to the scripts folder**
   ```bash
   cd $HOME/workspace/sandbox/scripts
   ```

2. **Stop and clean up wfm services**
   ```bash
   sudo -E bash ./wfm.sh  # Type 4 and press Enter - Option 4: Symphony Stop
   sudo -E bash ./wfm.sh  # Type 2 and press Enter - Option 2: PreRequisites Cleanup
   sudo -E bash ./wfm.sh  # Type 6 and press Enter - Option 6: ObservabilityStack Stop
   ```

3. **Stop and clean up mis services**
   ```bash
   sudo -E bash ./mis.sh  # Type 4 and press Enter - Option 5: Margo Identity Service: Uninstall
   sudo -E bash ./mis.sh  # Type 2 and press Enter - Option 2: PreRequisites Cleanup
   ```


### On Device VMs:

1. **Navigate to the scripts folder**
   ```bash
   cd $HOME/workspace/sandbox/scripts
   ```

2. **Stop and clean up services**
   ```bash
   sudo -E bash ./device-agent.sh  # Type 4 (Docker) or 6 (K3s) - Device-agent Stop
   sudo -E bash ./device-agent.sh  # Type 2 - Uninstall-prerequisites
   sudo -E bash ./device-agent.sh  # Type 9 - otel-collector-promtail-uninstallation
   sudo -E bash ./device-agent.sh  # Type 10 - cleanup-residual
   ```

---

**Sample Applications Included:**
- **Custom OTEL**: Monitoring application that demonstrates telemetry capabilities. It is pre-loaded helm application to run on k3s device.
- **Nextcloud**: File sharing and collaboration platform. It is pre-loaded docker-compose package to run on docker device.


These applications are pre-loaded and ready to deploy to your device VMs for testing.

---
