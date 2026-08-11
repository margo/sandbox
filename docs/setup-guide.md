##### [Back To Main](../README.md)
# Setting Up the Code First Sandbox

## What You'll Need

**Three Virtual Machines:**
| VM Type | Processors(vCPU) | Memory | Storage | Purpose |
|---------|-----------|--------|---------|---------|
| **Main VM (WFM)** | 8 | 16GB | 100GB | Workload Fleet Manager |
| **Device VM 1 (Helm-capable device)** | 4 | 4-8GB | 50GB | Kubernetes-based device |
| **Device VM 2 (Compose-capable device)** | 4 | 4-8GB | 50GB | Docker-based device |

**Requirements:**
- Ubuntu operating system (**ubuntu-24.04.3-desktop-amd64 or server**) (you can check by doing ```cat /etc/os-release```)
   - Virtual Machine Manager (4.1.0 tested)
- Internet connection
- All VMs must be able to talk to each other (same network with static IP addresses)
- VM hostnames must be lowercase.

> Warning: If you are attempting to deploy this on corporate machines or within a corporate network, you will need to address any special networking requirements or access issues to enable internet communication (e.g, proxy configuration, certificates, firewall configuration, etc.). This falls outside the of the scope of this documentation. This warning applies to both the WFM and the Device VMs when running the setup scripts('wfm.sh' & 'device-agent.sh').
---


## Step 1: Get the Setup Files

You need to download the setup files to all three VMs. Follow these steps on **each VM**:

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
      ```

🔴 **Important:** Complete this step on all three VMs before proceeding.

---

## Step 3: Build Everything

> **Note:** If during setup you see any error like the following: ```ERROR:  429 Too Many Requests
   toomanyrequests: You have reached your unauthenticated pull rate limit. https://www.docker.com/increase-rate-limit```. This is because docker allows certain number of anonymous image pulls in a day, and yours have exhausted. Please login using your dockerhub account. The command to do so is: `docker login -u <your-dockerhub-account-name>` , then it'll ask for the password once you execute this command.

### On the WFM VM:

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


3. **Start the Workload Fleet Manager**
   ```bash
    sudo -E bash wfm.sh
   ```
   - Type `3` and press Enter
   - Choose: `Option 3: Symphony Start`

   This starts the Workload Fleet Manager service.
> Note: Docker image for Workload Fleet Manager has been already built and pushed using CI pipeline to Margo GHCR registry from where the below script pull the image and starts WFM.

4. **Add Monitoring Tools**
   ```bash
    sudo -E bash wfm.sh
   ```
   - Type `5` and press Enter
   - Choose: `Option 5: ObservabilityStack Start`

   This adds tools to monitor workloads observability.

5. **Verify the Workload Fleet Manager Is Running Correctly**
   ```bash
   sudo docker logs -f symphony-api-container
   ```
   You should see log messages indicating the service is running. Press `Ctrl+C` to exit.

> Note: Services are configured to auto-start on VM reboot.
  However, if you encounter issues after reboot, you can manually restart them using the same menu options.


### On Each Device VM:

1. **Copy Security Files Between VMs ( Both WFM's and Harbor's to Device VM)**

   You need to copy a security file from the WFM VM to each Device VM.
   > Note: create the certs directory before copying the security files 
   > Use: `mkdir -p $HOME/certs`

   #### Step 1: Preparation on WFM VM

   | Step | Action | Command | Expected Result |
   |------|--------|---------|-----------------|
   | 1 | Find WFM IP address | `hostname -I` | First IP address (e.g., 192.168.1.100) |
   | 2 | Locate WFM certificate | `cd $HOME/symphony/api/certificates`<br>`ls -la ca-cert.pem` | File: `ca-cert.pem` |
   | 3 | Locate Harbor certificate | `cd $HOME/sandbox/scripts/harbor/certs`<br>`ls -la harbor.crt` | File: `harbor.crt` |


   **Note:** Write down the IP address from Step 1 for use in the copy commands below.


   #### Step 2: Copy Methods

   **Option A - Using SCP**
   🔴 **(Recommended - Run from Device VMs)**



   | Target VM | Run From | SCP Command | Example |
   |-----------|----------|-------------|---------|
   | **Docker Device** | Docker Device VM | `scp username@WFM-VM-IP:~/symphony/api/certificates/ca-cert.pem $HOME/certs/` <br><br> `scp username@WFM-VM-IP:~/sandbox/scripts/harbor/certs/harbor.crt $HOME/certs/` | `scp azureuser@10.10.10.4:~/symphony/api/certificates/ca-cert.pem $HOME/certs/` <br><br> `scp azureuser@10.10.10.4:~/sandbox/scripts/harbor/certs/harbor.crt $HOME/certs/` | `scp azureuser@10.10.10.4:~/symphony/api/certificates/ca-cert.pem $HOME/certs/` |
   | **K3s Device** | K3s Device VM | `scp username@WFM-VM-IP:~/symphony/api/certificates/ca-cert.pem $HOME/certs/` <br><br> `scp username@WFM-VM-IP:~/sandbox/scripts/harbor/certs/harbor.crt $HOME/certs/` | `scp azureuser@10.10.10.4:~/symphony/api/certificates/ca-cert.pem $HOME/certs/` <br><br> `scp azureuser@10.10.10.4:~/sandbox/scripts/harbor/certs/harbor.crt $HOME/certs/` | `scp azureuser@10.10.10.4:~/symphony/api/certificates/ca-cert.pem $HOME/certs/` |

   **Note:** Run with **sudo** if fails.

   **Replace:**
   - `username` with your WFM VM username
   - `WFM-VM-IP` with the IP address from Step 1

   **Option B - Manual Copy**

   | Step | Docker Device VM | K3s Device VM |
   |------|------------------|---------------|
   | 1 | Open `ca-cert.pem` on WFM VM and copy contents | Open `ca-cert.pem` on WFM VM and copy contents |
   | 2 | Create file `ca-cert.pem` in `$HOME/certs/` | Create file `ca-cert.pem` in `$HOME/certs/` |
   | 3 | Paste contents and save | Paste contents and save |

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

3. **Install Basic Tools**

   Based on the device type, select **k3s** or **docker** while sourcing the environment variables. For example:
   ```bash
   sudo -E bash device-agent.sh docker # for docker-compose device
   sudo -E bash device-agent.sh k3s    # for k3s device
   ```
   - Type `1` and press Enter
   - Choose: `Option 1: Install-prerequisites`

   This may take 10-15 minutes.

4. **Create Security Certificates**
   ```bash
    sudo -E bash device-agent.sh docker # for docker-compose device
    sudo -E bash device-agent.sh k3s    # for k3s device
   ```
   - First, type `11` and press Enter to choose: `Option 11: create_device_rsa_certs`
   - Then run the command again and type `12` and press Enter to choose: `Option 12: create_device_ecdsa_certs`

   These certificates allow secure communication between VMs and are automatically saved in `$HOME/certs` directory.

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

> Note: Below is a example snippet showing the expected output of the selection. For now, you'll need to look at the device agent's logs to identify the client ID if you have multiple devices provisioned.
```
Enter choice [1-9]: 2
🖥️  Listing all devices from WFM...
┌─────────────────────────────────────────┐
│              Server Config              │
├─────────────────────────────────────────┤
│ Host:      localhost                    │
│ Port:      8082                         │
│ Basepath:      v1alpha2/margo/nbi/v1        │
└─────────────────────────────────────────┘
+------------------------------------+------------------------------+------------------------------+-----------+------------------+
| ID                                 | SIGNATURE                    | CAPABILITIES                 | STATE     | CREATEDAT        |
+------------------------------------+------------------------------+------------------------------+-----------+------------------+
| client-56b77ecbfdc83e4a-1764667338 | LS0tLS1CRUdJTiBDRVJUSUZJQ... | {"apiVersion":"device.mar... | ONBOARDED | 2025-12-02 09:22 |
+------------------------------------+------------------------------+------------------------------+-----------+------------------+
|                                    |                              |                              | PAGE 1/1  | TOTAL: 1         |
+------------------------------------+------------------------------+------------------------------+-----------+------------------+

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
│ Host:      localhost                    │
│ Port:      8082                         │
│ Basepath:      v1alpha2/margo/nbi/v1        │
└─────────────────────────────────────────┘
+--------------------------------------+------------+------------+------------+--------+--------------+------------------+
| ID                                   | NAME       | PKG        | DEVICE     | OP     | RUNNINGSTATE | UPDATED          |
+--------------------------------------+------------+------------+------------+--------+--------------+------------------+
| e675eaa8-0acd-4df4-8187-ccddc2d72f91 | otel-de... | ae01143... | client-... | DEPLOY | INSTALLED    | 2025-12-02 09:55 |
+--------------------------------------+------------+------------+------------+--------+--------------+------------------+
|                                      |            |            |            |        |              | TOTAL: 1         |
+--------------------------------------+------------+------------+------------+--------+--------------+------------------+

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

```
Enter choice [1-9]: 7
🚀 Deploy Instance
==================
📦 Available packages:
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

Enter the package name/ID to deploy: ae011433-28ed-4f4e-a8af-474810810746

🖥️  Available devices:
┌─────────────────────────────────────────┐
│              Server Config              │
├─────────────────────────────────────────┤
│ Host:      localhost                    │
│ Port:      8082                         │
│ Basepath:      v1alpha2/margo/nbi/v1        │
└─────────────────────────────────────────┘
+------------------------------------+------------------------------+------------------------------+-----------+------------------+
| ID                                 | SIGNATURE                    | CAPABILITIES                 | STATE     | CREATEDAT        |
+------------------------------------+------------------------------+------------------------------+-----------+------------------+
| client-56b77ecbfdc83e4a-1764667338 | LS0tLS1CRUdJTiBDRVJUSUZJQ... | {"apiVersion":"device.mar... | ONBOARDED | 2025-12-02 09:22 |
+------------------------------------+------------------------------+------------------------------+-----------+------------------+
|                                    |                              |                              | PAGE 1/1  | TOTAL: 1         |
+------------------------------------+------------------------------+------------------------------+-----------+------------------+

Enter the device ID for deployment: client-56b77ecbfdc83e4a-1764667338
📋 Getting package details...
🔍 Searching for package: ae011433-28ed-4f4e-a8af-474810810746
📦 Package name: custom-otel-helm-app
📄 Using deployment file: /root/symphony/cli/templates/margo/custom-otel-helm/instance.yaml.copy
🚀 Deploying 'ae011433-28ed-4f4e-a8af-474810810746' to device 'client-56b77ecbfdc83e4a-1764667338'...
┌─────────────────────────────────────────┐
│              Server Config              │
├─────────────────────────────────────────┤
│ Host:      localhost                    │
│ Port:      8082                         │
│ Basepath:      v1alpha2/margo/nbi/v1        │
└─────────────────────────────────────────┘
deploymentId e675eaa8-0acd-4df4-8187-ccddc2d72f91 deploymentName otel-demo-instance

Application configuration applied successfully

✅ Instance deployment request sent successfully!

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
│ Host:      localhost                    │
│ Port:      8082                         │
│ Basepath:      v1alpha2/margo/nbi/v1        │
└─────────────────────────────────────────┘
+--------------------------------------+------------+------------+------------+--------+--------------+------------------+
| ID                                   | NAME       | PKG        | DEVICE     | OP     | RUNNINGSTATE | UPDATED          |
+--------------------------------------+------------+------------+------------+--------+--------------+------------------+
| e675eaa8-0acd-4df4-8187-ccddc2d72f91 | otel-de... | ae01143... | client-... | DEPLOY | INSTALLED    | 2025-12-02 09:55 |
+--------------------------------------+------------+------------+------------+--------+--------------+------------------+
|                                      |            |            |            |        |              | TOTAL: 1         |
+--------------------------------------+------------+------------+------------+--------+--------------+------------------+

Enter the deployment/instance ID to delete: e675eaa8-0acd-4df4-8187-ccddc2d72f91
Are you sure you want to delete instance 'e675eaa8-0acd-4df4-8187-ccddc2d72f91'? (y/N): y
🗑️  Deleting instance 'e675eaa8-0acd-4df4-8187-ccddc2d72f91'...
┌─────────────────────────────────────────┐
│              Server Config              │
├─────────────────────────────────────────┤
│ Host:      localhost                    │
│ Port:      8082                         │
│ Basepath:      v1alpha2/margo/nbi/v1        │
└─────────────────────────────────────────┘
deploymentId to be deleted e675eaa8-0acd-4df4-8187-ccddc2d72f91
application deployment deletion request has been accepted!

Application Deployment e675eaa8-0acd-4df4-8187-ccddc2d72f91 deleted successfully

✅ Instance 'e675eaa8-0acd-4df4-8187-ccddc2d72f91' deleted successfully!

📋 Updated deployments:
┌─────────────────────────────────────────┐
│              Server Config              │
├─────────────────────────────────────────┤
│ Host:      localhost                    │
│ Port:      8082                         │
│ Basepath:      v1alpha2/margo/nbi/v1        │
└─────────────────────────────────────────┘
+--------------------------------------+------------+------------+------------+--------+--------------+------------------+
| ID                                   | NAME       | PKG        | DEVICE     | OP     | RUNNINGSTATE | UPDATED          |
+--------------------------------------+------------+------------+------------+--------+--------------+------------------+
| e675eaa8-0acd-4df4-8187-ccddc2d72f91 | otel-de... | ae01143... | client-... | DEPLOY | REMOVING     | 2025-12-02 09:59 |
+--------------------------------------+------------+------------+------------+--------+--------------+------------------+
|                                      |            |            |            |        |              | TOTAL: 1         |
+--------------------------------------+------------+------------+------------+--------+--------------+------------------+

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

2. **Stop and clean up services**
   ```bash
   sudo -E bash ./wfm.sh  # Type 4 and press Enter - Option 4: Symphony Stop
   sudo -E bash ./wfm.sh  # Type 2 and press Enter - Option 2: PreRequisites Cleanup
   sudo -E bash ./wfm.sh  # Type 6 and press Enter - Option 6: ObservabilityStack Stop
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

## Quick Summary

**The setup process in simple terms:**

1. **Build**: Install tools and start services on all VMs
   - WFM VM: Installs management tools and starts the Workload Fleet Manager
   - Device VMs: Installs device software and creates security certificates

2. **Deploy**: Connect devices to the WFM VM using security certificates
   - Copy the security file from WFM VM to each Device VM
   - Start the device services

3. **Run**: Use the EasyCLI to manage applications on your devices
   - Use the menu-driven EasyCLI tool to deploy applications
   - Monitor everything through web dashboards

**Sample Applications Included:**
- **Custom OTEL**: Monitoring application that demonstrates telemetry capabilities. It is pre-loaded helm application to run on k3s device.
- **Nextcloud**: File sharing and collaboration platform. It is pre-loaded docker-compose package to run on docker device.


These applications are pre-loaded and ready to deploy to your device VMs for testing.

---

## Need Help?

If something doesn't work:
1. Check that all VMs can communicate with each other (ping test)
2. Verify environment variables are set correctly
3. Make sure the ca-cert.pem file was copied correctly
4. Check the logs using the commands in "Check Everything is Working" section