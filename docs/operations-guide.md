##### [Back To Main](../README.md) | [← Setup Guide](./setup-guide.md)

# Operations Guide

This guide covers day-to-day operations of the Margo sandbox environment once setup is complete. It assumes you have successfully completed all steps in the [Setup Guide](./setup-guide.md).

---

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
│ Host:      symphony.machine                    │
│ Port:      8082                         │
│ Basepath:      v1alpha2/margo/nbi/v1        │
└─────────────────────────────────────────┘
+--------------------------------------+----------------------+---------+-----------+-----------+-------------+-------------------------------------+------------------+------------------+
| ID                                   | NAME                 | VERSION | OPERATION | STATE     | SOURCE TYPE | SOURCE                              | CREATED          | UPDATED          |
+--------------------------------------+----------------------+---------+-----------+-----------+-------------+-------------------------------------+------------------+------------------+
| af3af6b3-01c1-42bb-9168-347e99a174b8 | custom-otel-helm-app |         | ONBOARD   | ONBOARDED | OCI_REPO    | {"authentication":{"password":"Harb | 2026-09-23 10:00 | 2026-09-23 10:00 |
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
│ Host:      symphony.machine                    │
│ Port:      8082                         │
│ Basepath:      v1alpha2/margo/nbi/v1        │
└─────────────────────────────────────────┘
+--------------------------------------+----------------------+---------+-----------+-----------+-------------+-------------------------------------+------------------+------------------+
| ID                                   | NAME                 | VERSION | OPERATION | STATE     | SOURCE TYPE | SOURCE                              | CREATED          | UPDATED          |
+--------------------------------------+----------------------+---------+-----------+-----------+-------------+-------------------------------------+------------------+------------------+
| ae011433-28ed-4f4e-a8af-474810810746 | custom-otel-helm-app |         | ONBOARD   | ONBOARDED | OCI_REPO    | {"authentication":{"password":"Harb | 2026-09-23 09:52 | 2026-09-23 09:52 |
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
│ Host:      symphony.machine                    │
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

## Pre-loaded Sample Applications

| Application | Type | Target Device | Description |
|-------------|------|---------------|-------------|
| **Custom OTEL** | Helm chart | K3s (Helm-capable) device | Monitoring app demonstrating telemetry capabilities |
| **Nextcloud** | Docker Compose | Docker (Compose-capable) device | File sharing and collaboration platform |

These applications are pre-loaded in Harbor and ready to upload to WFM and deploy to your device VMs for testing.