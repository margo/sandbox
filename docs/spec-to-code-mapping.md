##### [Back To Main](../README.md)
Here's a comprehensive **Specification-to-Implementation Mapping**.

---

## 📋 Margo Specification to Code Mapping

This section maps Margo specification features to their implementation in the sandbox codebase, helping developers quickly locate relevant code without getting lost in Symphony internals.

### Quick Navigation by Margo Feature

| Margo Specification Feature | Implementation Location | Key Files & Functions |
|----------------------------|------------------------|----------------------|
| **Device Onboarding** | WFM Server + Device Agent | • WFM: `symphony-margo/api/pkg/apis/v1alpha1/managers/margo/device.go::OnboardDevice()`<br>• API: `symphony-margo/api/pkg/apis/v1alpha1/vendors/margo/device-agent-vendor.go::onboardDevice()`<br>• [Here is Spec](https://github.com/margo/specification-enhancements/blob/main/submitted/api-details-finalization-SUP-folder/api-details-finalization-SUP-WIP.md) |
| **Device Capabilities Reporting** | Device Agent → WFM | • Device: `poc/device/agent/main.go::reportCapabilities()`<br>• WFM: `symphony-margo/api/pkg/apis/v1alpha1/managers/margo/device.go::SaveDeviceCapabilities()`<br>• API: `symphony-margo/api/pkg/apis/v1alpha1/vendors/margo/device-agent-vendor.go::saveDeviceCapabilities()`<br>• [Here is Spec](https://github.com/margo/specification-enhancements/blob/main/submitted/api-details-finalization-SUP-folder/margo_workload_api_wip.yaml) |
| **Application Package Onboarding** | WFM Server | • Manager: `symphony-margo/api/pkg/apis/v1alpha1/managers/margo/appPkg.go::OnboardAppPkg()`<br>• State Machine: `symphony-margo/api/pkg/apis/v1alpha1/managers/margo/appPkgStateMachine.go`<br>• API: `symphony-margo/api/pkg/apis/v1alpha1/vendors/margo/workload-mgmt-vendor.go::onboardAppPkg()`<br>• [Here is Spec](https://github.com/margo/specification-enhancements/blob/main/submitted/sup_app_registry_as_oci.md) |
| **Application Package Processing** | WFM Server | • OCI: `symphony-margo/api/pkg/apis/v1alpha1/managers/margo/appPkg.go::processOciRepositoryWithStateTracking()`<br>• Package Manager: `non-standard/pkg/packageManager/` |
| **Deployment Creation** | WFM Server | • Manager: `symphony-margo/api/pkg/apis/v1alpha1/managers/margo/deployment.go::CreateDeployment()`<br>• State Machine: `symphony-margo/api/pkg/apis/v1alpha1/managers/margo/deploymentStateMachine.go`<br>• API: `symphony-margo/api/pkg/apis/v1alpha1/vendors/margo/workload-mgmt-vendor.go::createDeployment()` |
| **Desired State Distribution** | WFM Server → Device Agent | • WFM Bundle: `symphony-margo/api/pkg/apis/v1alpha1/managers/margo/deploymentBundle.go::rebuildTheBundleForDevice()`<br>• WFM API: `symphony-margo/api/pkg/apis/v1alpha1/vendors/margo/device-agent-vendor.go::getDesiredManifest()`<br>• Device Sync: `poc/device/agent/stateSync.go::performSync()`<br>•[Here is Spec](https://github.com/margo/specification-enhancements/blob/main/submitted/sup_desired_state_via_manifest_api.md) |
| **Bundle Download** | Device Agent ← WFM | • WFM: `symphony-margo/api/pkg/apis/v1alpha1/vendors/margo/device-agent-vendor.go::downloadBundle()`<br>• Device: `poc/device/agent/stateSync.go::downloadAndExtractBundle()`<br>• Archive Lib: `shared-lib/archive/` |
| **Individual Deployment Fetch** | Device Agent ← WFM | • WFM: `symphony-margo/api/pkg/apis/v1alpha1/vendors/margo/device-agent-vendor.go::downloadDeployment()`<br>• Device: `poc/device/agent/stateSync.go::fetchDeploymentYAML()` |
| **Deployment Execution (Helm)** | Device Agent | • Manager: `poc/device/agent/deployment.go::deployOrUpdateHelm()`<br>• Client: `shared-lib/workloads/helm.go`<br>• Monitor: `poc/device/agent/monitor.go::checkDeployment()` |
| **Deployment Execution (Compose)** | Device Agent | • Manager: `poc/device/agent/deployment.go::deployOrUpdateCompose()`<br>• Client: `shared-lib/workloads/compose.go` |
| **Deployment Status Reporting** | Device Agent → WFM | • Device: `poc/device/agent/statusReporter.go::reportStatus()`<br>• WFM: `symphony-margo/api/pkg/apis/v1alpha1/managers/margo/device.go::OnDeploymentStatus()`<br>• API: `symphony-margo/api/pkg/apis/v1alpha1/vendors/margo/device-agent-vendor.go::onDeploymentStatusUpdate()` |
| **Deployment Removal** | Device Agent | • Manager: `poc/device/agent/deployment.go::remove()`<br>• Helm: `poc/device/agent/deployment.go::removeHelm()`<br>• Compose: `poc/device/agent/deployment.go::removeCompose()` |
| **HTTP Signature Authentication** | WFM Server + Device Agent | • Signing: `shared-lib/crypto/signer.go`<br>• Verification: `symphony-margo/api/pkg/apis/v1alpha1/vendors/margo/device-agent-vendor.go::verifyRequestSignature()`<br>• [Here is Spec](https://github.com/margo/specification-enhancements/blob/main/submitted/api-details-finalization-SUP-folder/api-details-finalization-SUP-WIP.md) |
| **ETag-based Caching** | WFM Server + Device Agent | • WFM: `symphony-margo/api/pkg/apis/v1alpha1/vendors/margo/device-agent-vendor.go::getDesiredManifest()` (ETag generation)<br>• Device: `poc/device/agent/stateSync.go::getLastSyncedETag()` (If-None-Match)<br>• Spec: `[sup_desired_state_via_manifest_api.md](sup_desired_state_via_manifest_api.md#etag-header)` |
| **Manifest Version Rollback Protection** | Device Agent | • Validation: `poc/device/agent/stateSync.go::validateManifest()`<br>• Storage: `poc/device/agent/database/database.go::SetLastSyncedManifestVersion()` |
| **Observability (OTEL + Promtail)** | Device Agent | • Setup: `scripts/device-agent.sh::install_otel_collector_promtail_wrapper()`<br>• Docker: `scripts/device-agent.sh::install_otel_collector_promtail_docker()`<br>• K8s: `scripts/device-agent.sh::install_otel_collector_promtail()` |

---

### 🔍 Deep Dive: Key Implementation Patterns

#### 1. **Application Package Lifecycle**
```
User Request → workload-mgmt-vendor.go::onboardAppPkg()
           ↓
    appPkg.go::OnboardAppPkg() (creates DB record)
           ↓
    appPkg.go::processPackageAsync() (background processing)
           ↓
    └─ OCI: processOciRepositoryWithStateTracking()
           ↓
    transformer.go::AppPackageToSymphonyObjects() (converts to Symphony)
           ↓
    appPkg.go::storeSymphonyObjects() (persists to state provider)
```

**State Machine**: `appPkgStateMachine.go`
- States: `PENDING` → `PROCESSING` → `ONBOARDED` / `FAILED`
- Events: `START_PROCESSING`, `PROCESSING_COMPLETE`, `PROCESSING_FAILED`

---

#### 2. **Deployment Lifecycle**
```
User Request → workload-mgmt-vendor.go::createDeployment()
           ↓
    deployment.go::CreateDeployment() (creates deployment record)
           ↓
    deploymentBundle.go::rebuildTheBundleForDevice() (async bundle generation)
           ↓
    Device polls → device-agent-vendor.go::getDesiredManifest()
           ↓
    Device downloads → stateSync.go::performSync()
           ↓
    Device executes → deployment.go::deployOrUpdate()
           ↓
    Device reports → statusReporter.go::reportStatus()
           ↓
    WFM updates → device.go::OnDeploymentStatus()
```

**State Machine**: `deploymentStateMachine.go`
- States: `PENDING` → `INSTALLING` → `INSTALLED` → `REMOVING` → `REMOVED`
- Events: `START_INSTALLATION`, `INSTALLATION_SUCCESS`, `START_REMOVAL`, etc.

---

#### 3. **Desired State Synchronization (Pull-Based)**
```
Device Timer → stateSync.go::syncLoop() (every 15s)
           ↓
    stateSync.go::performSync()
           ↓
    apiClient.SyncStateWithResponse() (GET /clients/{id}/deployments)
           ↓
    ├─ 304 Not Modified → Skip (ETag match)
    └─ 200 OK → Process manifest
           ↓
    stateSync.go::validateManifest() (version + security checks)
           ↓
    ├─ Bundle download → downloadAndExtractBundle()
    └─ Individual fetch → processDeploymentsIndividually()
           ↓
    database.SetDesiredState() (triggers reconciliation)
           ↓
    deployment.go::reconcileDeployment() (executes workload)
```

---

### 📚 Specification Documents Reference

| Document | Purpose | Implementation |
|----------|---------|----------------|
| [Device-WFM API contract](https://github.com/margo/specification-enhancements/blob/main/submitted/api-details-finalization-SUP-folder/margo_workload_api_wip.yaml) | Device-WFM API contract | `device-agent-vendor.go` endpoints |
| [Authentication & security](https://github.com/margo/specification-enhancements/blob/main/submitted/api-details-finalization-SUP-folder/api-details-finalization-SUP-WIP.md) | Authentication & security | `crypto/signer.go`, `device-agent-vendor.go::verifyRequestSignature()` |
| [Desired state](https://github.com/margo/specification-enhancements/blob/main/submitted/sup_desired_state_via_manifest_api.md) | Desired state distribution | `deploymentBundle.go`, `stateSync.go` |
| [Application registry](https://github.com/margo/specification-enhancements/blob/main/submitted/sup_app_registry_as_oci.md) | Application registry | `appPkg.go::processOciRepositoryWithStateTracking()` |

---
