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

### Use the EasyCLI

On the WFM VM:

1. **Navigate to the scripts folder**
   ```bash
   cd $HOME/workspace/sandbox/scripts
   ```

2. **Run the Easy CLI script**
   ```bash
    sudo -E bash wfm-cli.sh
   ```

3. **Interactive Menu Interface**

   ```
   🎛️  WFM CLI Interactive Interface
   =================================
   Choose an option:
   1) 📦 list app-pkg
   2) 🖥️  List Devices
   3) 🚀 List Deployment
   4) 📋 List All
   5) 📤 Upload App-Package
   6) 🗑️  Delete App-Package
   7) 🚀 Deploy Instance
   8) 🗑️  Delete Instance
   9) 🚪 Exit

   Enter choice [1-9]:
   ```

#### Menu Options Reference

| Option | Function | What It Shows | When to Use |
|--------|----------|---------------|-------------|
| **1** | List app-pkg | All available application packages | Check what apps are available to deploy |
| **2** | List Devices | All connected devices | Verify devices are connected and onboarded |
| **3** | List Deployment | All active deployments | See what's currently deployed on devices |
| **4** | List All | Packages + Devices + Deployments | Get complete system overview |
| **5** | Upload App-Package | Upload menu (Custom OTEL/Nextcloud) | Add new applications to WFM |
| **6** | Delete App-Package | Prompts for package ID to delete | Remove unused packages |
| **7** | Deploy Instance | Prompts for package and device | Deploy app to a device |
| **8** | Delete Instance | Prompts for deployment ID to delete | Remove deployment from device |
| **9** | Exit | Closes the CLI | Exit the interface |

#### Sandbox WFM User Guide

**Option 1: List App Packages**

Select this option to display the app packages that were uploaded to the sandbox WFM. These are ready to deploy to an onboarded edge node.

> Note: Below is a example snippet showing the expected output of the selection. You will need to use option 5 (see below) to load the app packages first.
```
Enter choice [1-9]: 1
📦 Listing all app packages from WFM...
┌─────────────────────────────────────────┐
│              Server Config              │
├─────────────────────────────────────────┤
│ Host:      localhost                    │
│ Port:      8082                         │
│ Basepath:      v1alpha2/margo/nbi/v1        │
└─────────────────────────────────────────┘
+--------------------------------------+----------------------+---------+-----------+-----------+-------------+-------------------------------------+------------------+------------------+
| ID                                   | NAME                 | VERSION | OPERATION | STATE     | SOURCE TYPE | SOURCE                              | CREATED          | UPDATED          |
+--------------------------------------+----------------------+---------+-----------+-----------+-------------+-------------------------------------+------------------+------------------+
| af3af6b3-01c1-42bb-9168-347e99a174b8 | custom-otel-helm-app |         | ONBOARD   | ONBOARDED | OCI_REPO    | {"authentication":{"password":"Harb | 2025-12-02 10:00 | 2025-12-02 10:00 |
|                                      |                      |         |           |           |             | or12345","type":"basic","username": |                  |                  |
|                                      |                      |         |           |           |             | "admin"},"registryUrl":"172.19.59.1 |                  |                  |
|                                      |                      |         |           |           |             | 48:8443","repository":"library/cust |                  |                  |
|                                      |                      |         |           |           |             | om-otel-helm-app-package","tag":"la |                  |                  |
|                                      |                      |         |           |           |             | test","url":""}                     |                  |                  |
+--------------------------------------+----------------------+---------+-----------+-----------+-------------+-------------------------------------+------------------+------------------+
|                                      |                      |         |           |           |             |                                     | PAGE 1/1         | TOTAL: 1         |
+--------------------------------------+----------------------+---------+-----------+-----------+-------------+-------------------------------------+------------------+------------------+

Press Enter to continue...
```

**Option 2: List Devices**

Select this option to display the devices that have onboarded to the sandbox WFM.

> Note: Below is a example snippet showing the expected output of the selection. IDs displayed here are device client's SPIFFE ID from their respective SVID identity.
```
Enter choice [1-9]: 2
🖥️  Listing all devices from WFM...
┌─────────────────────────────────────────┐
│              Server Config              │
├─────────────────────────────────────────┤
│ Host:      symphony.machine             │
│ Port:      8082                         │
│ Basepath:      v1alpha2/margo/nbi/v1    │
└─────────────────────────────────────────┘
+---------------------------------------------------------------+------------------------------+-----------------+-----------+--------------+
| ID                                                            | CAPABILITIES                 | DEPLOYMENT TYPE | STATE     | CREATEDAT    |
+---------------------------------------------------------------+------------------------------+-----------------+-----------+--------------+
| spiffe://margo.org/margo/wfm/symphony-1/client/dockerdevice-1 | {"properties":{"cpus":[{"... | compose         | ONBOARDED | 2026-09-23 0 |
|                                                               |                              |                 |           | 9:26         |
| spiffe://margo.org/margo/wfm/symphony-1/client/k3sdevice-1    | {"properties":{"cpus":[{"... | helm            | ONBOARDED | 2026-09-23 0 |
|                                                               |                              |                 |           | 9:26         |
+---------------------------------------------------------------+------------------------------+-----------------+-----------+--------------+
|                                                               |                              |                 | PAGE 1/1  | TOTAL: 1     |
+---------------------------------------------------------------+------------------------------+-----------------+-----------+--------------+

Press Enter to continue...

```

**Option 3: List Deployments**

Select this option to display the current app deployments configured in the sandbox WFM.

> Note: Below is a example snippet showing the expected output of the selection.
```

Enter choice [1-9]: 3
🚀 Listing all deployments from WFM...
┌─────────────────────────────────────────┐
│              Server Config              │
├─────────────────────────────────────────┤
│ Host:      symphony.machine             │
│ Port:      8082                         │
│ Basepath:      v1alpha2/margo/nbi/v1        │
└─────────────────────────────────────────┘
+--------------------------------------+------------+------------+--------------------+--------+--------------+------------------+
| ID                                   | NAME       | PKG        | DEVICE             | OP     | RUNNINGSTATE | UPDATED          |
+--------------------------------------+------------+------------+--------------------+--------+--------------+------------------+
| 03ce43e9-287a-43c4-8c9c-16f1323b4ecc | nextclo... | bfe5032... | .../dockerdevice-1 | DEPLOY | INSTALLED      | 2026-09-23 10:20 |
+--------------------------------------+------------+------------+--------------------+--------+--------------+------------------+
|                                      |            |            |                    |        |              | TOTAL: 1         |
+--------------------------------------+------------+------------+--------------------+--------+--------------+------------------+

Press Enter to continue...

```

**Option 4: List All Resources**

Select this option to display the combined view of packages, devices, and deployments (see individual examples above for format).

**Option 5: Upload App-Package**

Select this option to upload an application package from the pre-configured harbor OCI registry to WFM for deployment. Also user can upload new application packges to local harbor OCI registry which can be discovered here and listed as an option to upload to WFM. Refer [upload instructions.](./upload-package.md)

> Note: Below is a example snippet showing the expected output of the selection.

```
Enter choice [1-9]: 5
📦 Upload App Package
====================
🔍 Discovering app packages from Harbor OCI Registry...
Select one of the packages:
1) nginx-helm-app-package
2) wordpress-compose-app-package
3) custom-otel-helm-app-package
4) nextcloud-compose-app-package
5) Exit


Enter choice [1-6]: 3
📤 Uploading custom-otel-helm-app-package to WFM...

✅ Custom OTEL Helm App uploaded successfully!

Press Enter to continue...
```

**Option 6: Delete App-Package**

Select this option to delete previously uploaded application packages from the sandbox WFM.

> Note: Below is a example snippet showing the expected output of the selection.
> Note: the id of the application package needs to be copied from the output shown below 'current packages'.
```
Enter choice [1-9]: 6
🗑️  Delete App Package
====================
📦 Current packages:
┌─────────────────────────────────────────┐
│              Server Config              │
├─────────────────────────────────────────┤
│ Host:      localhost                    │
│ Port:      8082                         │
│ Basepath:      v1alpha2/margo/nbi/v1        │
└─────────────────────────────────────────┘
+--------------------------------------+----------------------+---------+-----------+-----------+-------------+-------------------------------------+------------------+------------------+
| ID                                   | NAME                 | VERSION | OPERATION | STATE     | SOURCE TYPE | SOURCE                              | CREATED          | UPDATED          |
+--------------------------------------+----------------------+---------+-----------+-----------+-------------+-------------------------------------+------------------+------------------+
| ae011433-28ed-4f4e-a8af-474810810746 | custom-otel-helm-app |         | ONBOARD   | ONBOARDED | OCI_REPO    | {"authentication":{"password":"Harb | 2025-12-02 09:52 | 2025-12-02 09:52 |
|                                      |                      |         |           |           |             | or12345","type":"basic","username": |                  |                  |
|                                      |                      |         |           |           |             | "admin"},"registryUrl":"172.19.59.1 |                  |                  |
|                                      |                      |         |           |           |             | 48:8443","repository":"library/cust |                  |                  |
|                                      |                      |         |           |           |             | om-otel-helm-app-package","tag":"la |                  |                  |
|                                      |                      |         |           |           |             | test","url":""}                     |                  |                  |
+--------------------------------------+----------------------+---------+-----------+-----------+-------------+-------------------------------------+------------------+------------------+
|                                      |                      |         |           |           |             |                                     | PAGE 1/1         | TOTAL: 1         |
+--------------------------------------+----------------------+---------+-----------+-----------+-------------+-------------------------------------+------------------+------------------+

Enter the package name/ID to delete: ae011433-28ed-4f4e-a8af-474810810746
Are you sure you want to delete app-pkg 'ae011433-28ed-4f4e-a8af-474810810746'? (y/N): y
🗑️  Deleting package 'ae011433-28ed-4f4e-a8af-474810810746'...
┌─────────────────────────────────────────┐
│              Server Config              │
├─────────────────────────────────────────┤
│ Host:      localhost                    │
│ Port:      8082                         │
│ Basepath:      v1alpha2/margo/nbi/v1        │
└─────────────────────────────────────────┘
appPkgIdto be deleted ae011433-28ed-4f4e-a8af-474810810746
app Pkg deletion request has been accepted!

Application Pkg ae011433-28ed-4f4e-a8af-474810810746 deleted successfully

✅ Package 'ae011433-28ed-4f4e-a8af-474810810746' deleted successfully!
```

**Option 7: Deploy Instance**

Select this option to deploy an instance of an uploaded application package within the sandbox WFM.

Configuration Notes:
- Below is a example snippet showing the expected output of the selection.
- The id of the application package needs to be copied from the output shown below 'Available packages'.
- The id of the device needs to be copied from the output shown below 'Available devices'.
- While selecting device, a new column `ELIGIBLE` is now visible, indicating whether a particular device is eligible to run the application or not, based on [Device Eligibility Checks against a particular application](https://docs.margo.org/specification/applications/application-description#deviceconstraints-attributes) 

```
Enter choice [1-9]: 7
🚀 Deploy Instance
==================
📦 Available packages:
┌─────────────────────────────────────────┐
│              Server Config              │
├─────────────────────────────────────────┤
│ Host:      symphony.machine             │
│ Port:      8082                         │
│ Basepath:      v1alpha2/margo/nbi/v1        │
└─────────────────────────────────────────┘
+--------------------------------------+----------------------+---------+-----------+-----------+-------------+-------------------------------------+------------------+------------------+
| ID                                   | NAME                 | VERSION | OPERATION | STATE     | SOURCE TYPE | SOURCE                              | CREATED          | UPDATED          |
+--------------------------------------+----------------------+---------+-----------+-----------+-------------+-------------------------------------+------------------+------------------+
| bfe50327-c1ab-4264-aff3-c3a481f22a07 | nextcloud-compose... | latest  | ONBOARD   | ONBOARDED | OCI_REPO    | {"authentication":{"password":"Harb | 2026-09-23 09:36 | 2026-09-23 09:36 |
|                                      |                      |         |           |           |             | or12345","type":"basic","username": |                  |                  |
|                                      |                      |         |           |           |             | "admin"},"registryUrl":"https://har |                  |                  |
|                                      |                      |         |           |           |             | bor.machine:8443","repository":"lib |                  |                  |
|                                      |                      |         |           |           |             | rary/nextcloud-compose-app-package" |                  |                  |
|                                      |                      |         |           |           |             | ,"tag":"latest"}                    |                  |                  |
+--------------------------------------+----------------------+---------+-----------+-----------+-------------+-------------------------------------+------------------+------------------+
|                                      |                      |         |           |           |             |                                     | PAGE 1/1         | TOTAL: 1         |
+--------------------------------------+----------------------+---------+-----------+-----------+-------------+-------------------------------------+------------------+------------------+

Enter the package name/ID to deploy: bfe50327-c1ab-4264-aff3-c3a481f22a07

🖥️  Available devices:
┌─────────────────────────────────────────┐
│              Server Config              │
├─────────────────────────────────────────┤
│ Host:      symphony.machine             │
│ Port:      8082                         │
│ Basepath:      v1alpha2/margo/nbi/v1        │
└─────────────────────────────────────────┘
+---------------------------------------------------------------+------------------------------+-----------------+-----------+--------------+----------+
| ID                                                            | CAPABILITIES                 | DEPLOYMENT TYPE | STATE     | CREATEDAT    | ELIGIBLE |
+---------------------------------------------------------------+------------------------------+-----------------+-----------+--------------+----------+
| spiffe://margo.org/margo/wfm/symphony-1/client/dockerdevice-1 | {"properties":{"cpus":[{"... | compose         | ONBOARDED | 2026-09-23 0 | true     |
|                                                               |                              |                 |           | 9:26         |          |
| spiffe://margo.org/margo/wfm/symphony-1/client/k3sdevice-1    | {"properties":{"cpus":[{"... | helm            | ONBOARDED | 2026-09-23 0 | false    |
|                                                               |                              |                 |           | 9:26         |          |
+---------------------------------------------------------------+------------------------------+-----------------+-----------+--------------+----------+
|                                                               |                              |                 | PAGE 1/1  | TOTAL: 1     |          |
+---------------------------------------------------------------+------------------------------+-----------------+-----------+--------------+----------+

Enter the device ID for deployment: spiffe://margo.org/margo/wfm/symphony-1/client/dockerdevice-1

🚀 Deploying 'bfe50327-c1ab-4264-aff3-c3a481f22a07' to device 'spiffe://margo.org/margo/wfm/symphony-1/client/dockerdevice-1'...
┌─────────────────────────────────────────┐
│              Server Config              │
├─────────────────────────────────────────┤
│ Host:      symphony.machine             │
│ Port:      8082                         │
│ Basepath:      v1alpha2/margo/nbi/v1        │
└─────────────────────────────────────────┘
deploymentId 231282a3-b7c1-49d7-9c68-f0e0fb28c113 deploymentName nextcloud-stack-instance

Application configuration applied successfully

✅ Instance deployment request sent successfully!

📋 Updated deployments:
┌─────────────────────────────────────────┐
│              Server Config              │
├─────────────────────────────────────────┤
│ Host:      symphony.machine             │
│ Port:      8082                         │
│ Basepath:      v1alpha2/margo/nbi/v1        │
└─────────────────────────────────────────┘
+--------------------------------------+------------+------------+--------------------+--------+--------------+------------------+
| ID                                   | NAME       | PKG        | DEVICE             | OP     | RUNNINGSTATE | UPDATED          |
+--------------------------------------+------------+------------+--------------------+--------+--------------+------------------+
| 231282a3-b7c1-49d7-9c68-f0e0fb28c113 | nextclo... | bfe5032... | .../dockerdevice-1 | DEPLOY | PENDING      | 2026-09-23 09:37 |
+--------------------------------------+------------+------------+--------------------+--------+--------------+------------------+
|                                      |            |            |                    |        |              | TOTAL: 1         |
+--------------------------------------+------------+------------+--------------------+--------+--------------+------------------+

Press Enter to continue...

```

**Option 8: Delete Instance**

Select this option to delete an application instance within the sandbox WFM.

Configuration Notes:
- Below is a example snippet showing the expected output of the selection.
- The id of the application instance needs to be copied from the output shown below 'Current deployments'.

```

Enter choice [1-9]: 8
🗑️  Delete Instance
==================
🚀 Current deployments:
┌─────────────────────────────────────────┐
│              Server Config              │
├─────────────────────────────────────────┤
│ Host:      symphony.machine             │
│ Port:      8082                         │
│ Basepath:      v1alpha2/margo/nbi/v1        │
└─────────────────────────────────────────┘
+--------------------------------------+------------+------------+--------------------+--------+--------------+------------------+
| ID                                   | NAME       | PKG        | DEVICE             | OP     | RUNNINGSTATE | UPDATED          |
+--------------------------------------+------------+------------+--------------------+--------+--------------+------------------+
| 03ce43e9-287a-43c4-8c9c-16f1323b4ecc | nextclo... | bfe5032... | .../dockerdevice-1 | DEPLOY | INSTALLED    | 2026-09-23 10:21 |
+--------------------------------------+------------+------------+--------------------+--------+--------------+------------------+
|                                      |            |            |                    |        |              | TOTAL: 1         |
+--------------------------------------+------------+------------+--------------------+--------+--------------+------------------+

Enter the deployment/instance ID to delete: 03ce43e9-287a-43c4-8c9c-16f1323b4ecc
Are you sure you want to delete instance '03ce43e9-287a-43c4-8c9c-16f1323b4ecc'? (y/N): y
🗑️  Deleting instance '03ce43e9-287a-43c4-8c9c-16f1323b4ecc'...
┌─────────────────────────────────────────┐
│              Server Config              │
├─────────────────────────────────────────┤
│ Host:      symphony.machine             │
│ Port:      8082                         │
│ Basepath:      v1alpha2/margo/nbi/v1        │
└─────────────────────────────────────────┘
deploymentId to be deleted 03ce43e9-287a-43c4-8c9c-16f1323b4ecc
application deployment deletion request has been accepted!

Application Deployment 03ce43e9-287a-43c4-8c9c-16f1323b4ecc deleted successfully

✅ Instance '03ce43e9-287a-43c4-8c9c-16f1323b4ecc' deleted successfully!

📋 Updated deployments:
┌─────────────────────────────────────────┐
│              Server Config              │
├─────────────────────────────────────────┤
│ Host:      symphony.machine             │
│ Port:      8082                         │
│ Basepath:      v1alpha2/margo/nbi/v1        │
└─────────────────────────────────────────┘
+--------------------------------------+------------+------------+--------------------+--------+--------------+------------------+
| ID                                   | NAME       | PKG        | DEVICE             | OP     | RUNNINGSTATE | UPDATED          |
+--------------------------------------+------------+------------+--------------------+--------+--------------+------------------+
| 03ce43e9-287a-43c4-8c9c-16f1323b4ecc | nextclo... | bfe5032... | .../dockerdevice-1 | DEPLOY | REMOVING     | 2026-09-23 10:23 |
+--------------------------------------+------------+------------+--------------------+--------+--------------+------------------+
|                                      |            |            |                    |        |              | TOTAL: 1         |
+--------------------------------------+------------+------------+--------------------+--------+--------------+------------------+

Press Enter to continue...

```

### View Monitoring

To view the monitoring dashboards, you need your WFM VM's IP address.

1. **Find your WFM VM's IP address**
   ```bash
   hostname -I
   ```
   Write down the first IP address shown (for example: 192.168.1.100).

2. **Open your web browser and visit:**

   Replace `[WFM-VM-IP]` with your actual IP address from step 1.

   - **Grafana** (Charts and Graphs): `http://[WFM-VM-IP]:32000`
     - Username: `admin`
     - Password: `admin`

   - **Jaeger** (Performance Tracking): `http://[WFM-VM-IP]:32500`

   - **Prometheus** (Metrics): `http://[WFM-VM-IP]:30900`

   **Example:** If your WFM VM IP is 192.168.1.100, you would visit:
   - Grafana: `http://192.168.1.100:32000`
   - Jaeger: `http://192.168.1.100:32500`
   - Prometheus: `http://192.168.1.100:30900`

3. **Set up Data Sources in Grafana:**

   After logging into Grafana, configure Loki and Prometheus to view logs and metrics.

   **Step-by-Step Configuration:**

   | Step | Action | Details |
   |------|--------|---------|
   | 1 | Click on **Open Menu**(top left) | Navigate to **Connections** → **Data sources** |
   | 2 | Click **Add data source** | Search for the data source type |
   | 3 | Configure Prometheus | See Prometheus configuration table below |
   | 4 | Configure Loki | See Loki configuration table below |

   **Prometheus Data Source Configuration:**

   | Field | Value | Notes |
   |-------|-------|-------|
   | **Name** | `Prometheus` | Default name |
   | **URL** | `http://[WFM-VM-IP]:30900` | Replace `[WFM-VM-IP]` with your WFM IP<br>Example: `http://192.168.1.100:30900` |
   | **Save & Test** | Scroll at bottom and Click button | Should show "Successfully queried the Prometheus API" |

   **Loki Data Source Configuration:**

   | Field | Value | Notes |
   |-------|-------|-------|
   | **Name** | `Loki` | Default name |
   | **URL** | `http://[WFM-VM-IP]:32100` | Replace `[WFM-VM-IP]` with your WFM IP<br>Example: `http://192.168.1.100:32100` |
   | **Save & Test** | Scroll at bottom and Click button | Should show "Data source successfully connected." |

   **View Logs and Metrics:**

   | What to View | Steps |
   |--------------|-------|
   | **Metrics (Prometheus)** | 1. Click **Open Menu**(top left)  → **Explore**<br>2. Select **Prometheus** from data source dropdown<br>3. Enter a query (e.g., `up` to see all targets, select from metric dropdown, if you have installed pre-built  custom-otel-helm-app-package select **orders_processed_total** from metric dropdown)<br>4. Click **Run query**(top right)|
   | **Logs (Loki)** | 1. Click **Open Menu**(top left) → **Explore**<br>2. Select **Loki** from data source dropdown<br>3. On **Label filters** select a label (e.g., `job`)<br>4. Select a label value(e.g., dockerlogs or  `default/custom-otel-helm` if otel-app installed)<br>5. Click **Run query**(top right)


   Detailed documentation for  [Observability verification](../scripts/observability/README.md)
---

## Cleaning Up (Starting Fresh)

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
