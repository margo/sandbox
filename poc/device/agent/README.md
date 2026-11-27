##### [Back To Main](../../../README.md)
# Device's Workload Fleet Management Client

An edge device's Workload Fleet Management Client used by the Margo platform to manage application workloads, report workload target profile characteristics, and synchronize desired vs actual state with the WFM.

This repository contains a code-first-sandbox implementation that supports multiple runtimes (Helm for Kubernetes and Docker Compose) and is designed for easy extension (for example, adding other runtimes).

## Contents

- Overview
- Build and Run
- Configuration
- Runtimes & Features
- Development & Tests
- Troubleshooting
- Security


## Overview

The Device Workload Fleet Management Client runs on edge devices and provides these core responsibilities:

- Establishing MIAF identity and authorization with the Margo compliant WFM
- Reporting workload target profile characteristics (resources, peripherals, interfaces, and supported deployment technologies)
- Managing application workloads (Helm charts for Kubernetes runtime, Docker Compose for Docker runtime)
- Monitoring deployed workloads and reporting status changes in the deployments
- Periodic desired-state synchronization and event-driven deployment reconciliation
- Checking application placement constraints before a workload is deployed

Design goals: small footprint, modular runtime adapters, event-driven reconciliation, and clear observability.

## Architecture (high level)

Key components (see source):

- `main.go` — application bootstrap, component wiring and lifecycle management
- `database/database.go` — in-memory state store with on-disk persistence, subscriptions, MIAF state and sync metadata
- `onboarding.go` — workload target profile reporting and device-client settings
- `stateSync.go` — polls desired state, validates manifest versions, and fetches deployment bundles or individual deployments
- `deployment.go` — evaluates placement constraints and deploys, updates or removes workloads through runtime clients
- `monitor.go` — polls Helm release state and emits component status updates
- `status.go` — deployment status reporting API calls to the WFM
- `trustbundle.go` — retrieves, validates, caches and refreshes the MIAF SPIFFE trust bundle
- `watcher.go` — watches and validates changes to the authorized-WFM SPIFFE ID list
- `types/config.go` — YAML configuration and workload target profile loading/validation

Runtimes supported out of the box:

- Kubernetes (Helm v3)
- Docker (Compose)

The system is event-driven: database writes emit events consumed by other components to perform actions (deploy, monitor, report). Components use contexts for cancellation and support graceful shutdown.

## Build and Run

Minimal steps to get the agent running locally (developer flow):

Prerequisites

- Go 1.20+ (1.24.x recommended)
- Docker (for compose runtime and containerized runs)
- (Optional) kubectl and access to a Kubernetes cluster for Helm-based deployments

Build and run locally

1. Build

```bash
cd poc/device/agent
go build -o agent .
```

2. Provide configuration

Copy and edit the configuration files in `poc/device/agent/config/`:

```bash
# Edit config/config.yaml as needed, for example you can use vim editor to edit the file as given below:
vim config/config.yaml
```

3. Run

```bash
./agent
```

## Configuration

`config/config.yaml` is the source of truth for the supported configuration. It currently defines:

- `logging.level` — zap log level.
- `database.dataDir` — directory used for persisted device, trust-bundle, desired-state and deployment data.
- `miaf` — the device client's MIAF identity and authorization configuration. `x509.certPath` and `x509.keyPath` point to the device client's X.509-SVID certificate and private key. `mis.endpoint` and `mis.caPath` configure HTTPS access to the Margo Identity Service. Alternatively, `mis.trustBundle.path` and `mis.trustDomain` can be used with an operator-provided SPIFFE trust bundle. `mis.cacheInterval` is the fallback refresh interval in seconds. `authzPath` points to the authorized-WFM list.
- `wfm.sbiUrl` — the WFM SBI endpoint.
- `stateSeeking.interval` — desired-state polling interval in seconds.
- `runtimes` — enabled runtime clients. The checked-in example enables Kubernetes/Helm; Docker Compose can be enabled by adding a `DOCKER` entry with its Docker socket URL.
- `deviceProfile.readFromFile` — path to the workload target profiles JSON document.

Paths are resolved by the process, so use paths valid from the directory where the agent is started. The MIAF certificate, private key, CA certificate and trust bundle are sensitive trust material and must be readable only by the agent account.

The checked-in configuration is intentionally a working example rather than a complete template:

```yaml
logging:
  level: DEBUG
database:
  dataDir: "/var/lib/margo/device-agent/data"
miaf:
  x509:
    certPath: "./config/identity/payload-cert.pem"
    keyPath: "./config/identity/payload-key.pem"
  mis:
    endpoint: "https://mis.margo.org:9443"
    caPath: "./config/mis/https-ca.crt"
    cacheInterval: 60
  authzPath: "./config/authorized.json"
wfm:
  sbiUrl: "https://symphony.machine:8084/v1alpha2/margo"
stateSeeking:
  interval: 15
runtimes:
  - type: KUBERNETES
    kubernetes:
      kubeconfigPath: /root/.kube/config
deviceProfile:
  readFromFile: ./config/profiles.json
```

### Authorized WFM identities

`config/authorized.json` is a JSON array of SPIFFE IDs for WFMs that are allowed to communicate with this device client. For example:

```json
["spiffe://margo.org/margo/wfm/wfm"]
```

An operator can minimally update this file by adding or removing an authorized WFM SPIFFE ID. The agent watches the file, validates every ID as a Margo WFM identity, and keeps the previous list if an update is invalid. Each ID must belong to the configured MIAF trust domain.

### Workload profile characteristics

`config/profiles.json` contains the workload target profile document reported to the WFM via the `PUT /api/v1/profile/{workloadTargetId}` endpoint. It contains a unique `workloadTargetId` and an array of `characteristics` describing delegated compute resources, peripherals, network interfaces, and supported deployment technologies.

Only update values that describe resources actually delegated to Margo workloads. The minimal operator-editable fields are:
- `workloadTargetId` — unique identifier for the workload target
- `margo.org/resource/cpu` — CPU cores and architecture
- `margo.org/resource/memory` — available memory in GiB
- `margo.org/resource/storage` — available storage in GiB
- `margo.org/peripherals` — available hardware peripherals (e.g., GPU)
- `margo.org/interfaces` — network interfaces (e.g., ethernet, wifi, bluetooth)
- `margo.org/deployment/<type>` — supported deployment technologies (e.g., `margo.org/deployment/compose` or `margo.org/deployment/helm`) with an empty properties object `{}`

See the [Margo Profiles specification](https://github.com/margo/specification/blob/4137e6a6cd0b34f0eb4e7bb3382c1248cf71b3d5/system-design/specification/margo-management-interface/profiles.md) for the complete schema and allowed values.

## Runtimes & features

Supported workloads and notable features:

- Helm (Kubernetes) — full lifecycle management for Helm v3 charts, values injection, repo handling, release naming linked to deployment IDs
- Docker Compose — deploy/remove Compose projects, environment injection, project names correlated to deployment IDs
- (Extensible) — the runtime adapter pattern makes adding other runtimes straightforward

Operational features:

- State synchronization: periodic and event-driven modes, with reconciliation and conflict handling
- Monitoring & health-checks: continuous monitoring and status reporting back to the WFM
- Persistence: in-memory DB with optional on-disk persistence for state
- Error handling: structured errors and retry classification

### Profile placement constraint matching

Before deploying an application, the agent evaluates the deployment profile's optional `placementConstraints` against its reported profile characteristics in `config/profiles.json`. In the schema-agnostic matching model introduced in PR #233, `placementConstraints` unifies both resource capacities (CPU, memory, storage) and hardware eligibility rules into a list of generic `matchExpressions`.

Each match expression targets a specific characteristic and specifies how its properties must be matched:
- `key` (required): Identifies the profile characteristic to evaluate (e.g., `margo.org/resource/cpu`, `margo.org/resource/memory`, `margo.org/peripherals`, `margo.org/interfaces`). If `property` is omitted, the expression tests solely for the existence of the characteristic (e.g., verifying support for `margo.org/deployment/compose` or `margo.org/deployment/helm`).
- `property` (optional): An RFC 6901 JSON Pointer evaluated relative to the characteristic's `properties` object (e.g., `/memory`, `/storage`, `/cpus`, `/peripherals`, `/interfaces`).
- `operator` (required): Evaluation operator:
  - Scalar comparisons: `GtEq`, `LtEq`, `Gt`, `Lt`, `In`, `NotIn` evaluated against `values`.
  - Array containment: `ContainsAll` or `ContainsAny` evaluated using an `itemSelector` with nested match expressions against array elements (e.g., matching peripheral or interface types).
- `values` / `itemSelector`: Comparison operands used by the operator.

Example constraint structure from an `ApplicationDescription`:

```yaml
placementConstraints:
  - matchExpressions:
      - key: margo.org/resource/cpu
        property: /cpus
        operator: ContainsAny
        itemSelector:
          matchExpressions:
            - property: /cores
              operator: GtEq
              values: [1.5]
            - property: /architecture
              operator: In
              values: ["amd64"]
      - key: margo.org/resource/memory
        property: /memory
        operator: GtEq
        values: [1]
      - key: margo.org/peripherals
        property: /peripherals
        operator: ContainsAll
        itemSelector:
          matchExpressions:
            - property: /type
              operator: In
              values: ["gpu"]
```

All expressions within `placementConstraints` must evaluate to `true` (logical AND). If the workload target does not satisfy the placement constraints, the deployment fails locally with the matching failure reason, reports the failure status back to the WFM, and skips runtime deployment.

This check complements WFM placement. While the WFM uses placement constraints to schedule workloads to eligible workload targets across the fleet, the device agent retains local responsibility for validating that the workload can safely run on its currently available characteristics.

## Development & tests

Project structure (top-level of the agent):

```
poc/device/agent/
├─ main.go
├─ onboarding.go
├─ stateSync.go
├─ deployment.go
├─ monitor.go
├─ status.go
├─ trustbundle.go
├─ watcher.go
├─ config/
│  ├─ authorized.json
│  ├─ config.yaml
│  └─ profiles.json
├─ database/
│  └─ database.go
└─ types/
  ├─ config.go
  ├─ config_test.go
  └─ error.go
```

Key developer notes:

- Interfaces exist for the state syncer, deployment manager, monitor and status reporter — follow existing patterns when adding new components
- Add unit tests under the same package (table-driven tests for logic, small integration tests for runtime clients)

Running tests

```bash
cd poc/device/agent
go test ./...
```

When you add runtime clients, also add small integration or smoke tests where feasible. Keep dependencies pinned in `go.mod`.

### Adding New Runtime Support

1. **Implement workload interfaces** in `shared-lib/workloads/`
2. **Add runtime configuration** to `types/config.go`
3. **Update deployment manager** to handle new runtime type
4. **Add monitoring support** in `monitor.go`
5. **Register runtime** in workload-fleet-management-client initialization

#### Development Extension Example: Adding Podman Runtime Support

The following example tries to extend the device workload-fleet-management-client to support Podman as a new container runtime,
this is just exemplary code. : )

#### Step 1: Add Configuration Support

Update `types/config.go`:

```go
type PodmanConfig struct {
  RemoteURL               string     `yaml:"remoteUrl,omitempty"`
  // ... other fields
}

type RuntimeInfo struct {
  Type       string            `yaml:"type"`
  Kubernetes *KubernetesConfig `yaml:"kubernetes,omitempty"`
  Docker     *DockerConfig     `yaml:"docker,omitempty"`
  Podman     *PodmanConfig     `yaml:"podman,omitempty"` // New runtime
}
```

#### Step 2: Create Podman Workload Client

Create `shared-lib/workloads/podman.go`:

```go
package workloads

import (
    "context"
    "fmt"
    "strings"

    "github.com/containers/podman/v4/pkg/bindings"
    "github.com/containers/podman/v4/pkg/bindings/containers"
    "github.com/containers/podman/v4/pkg/bindings/pods"
)

type PodmanClient struct {
    connection context.Context
    workingDir string
}
```
(If you plan to add Podman support, follow the same pattern used for Docker and Helm: add config types, create a runtime client in `shared-lib/workloads`, wire it in `main.go`, and extend deployment and monitoring flows.)

## Troubleshooting

Quick checks

- WFM unreachable: verify `wfm.sbiUrl` and that the network path is accessible. Try telnet to the WFM IP and Port. If it works, then try `curl` to the endpoints.
- Docker socket permissions: ensure the container or user has access to `/var/run/docker.sock` or run the workload-fleet-management-client as a user in the `docker` group
- Kubernetes issues: verify `kubeconfig` and that `kubectl` can access the cluster
- Profiles file: validate `config/profiles.json` with `jq` before use

Logging

- The agent uses zap for structured logs. Configure log level in `config.yaml`.
- For container runs mount a log directory or direct logs to stdout/stderr for your runtime to capture.

Debugging tips

- Increase `logging.level` to `DEBUG` to get richer traces for state sync and deployment flows
- Inspect the in-memory DB persistence file under `data/` to see the last known state


## Security

The agent uses the [Margo Identity and Authorization Framework (MIAF)](https://docs.margo.org/specification/identity/identity-framework) for WFM communication. MIAF uses a SPIFFE ID carried in an X.509-SVID, mutual TLS for peer authentication, a SPIFFE trust bundle for certificate validation, and local policy based on the peer's verified SPIFFE ID for authorization.

- The device client's X.509-SVID and private key are configured under `miaf.x509`.
- The trust bundle is retrieved from the MIS using `miaf.mis.endpoint` and `miaf.mis.caPath`, or loaded from the configured operator-provided bundle. The agent validates and caches the bundle and refreshes it periodically; a positive `spiffe_refresh_hint` in the bundle can change the effective refresh interval.
- `miaf.authzPath` is the local allow-list of WFM SPIFFE IDs. It is validated at startup and revalidated when the file changes.
- The WFM client is configured with the MIAF mTLS transport. The old request-signing, OAuth helper and standalone `tlsHelper` configuration are no longer part of this agent's configuration and must not be added to `config.yaml`.
- Use an `https://` WFM endpoint and keep the SVID key, MIS CA certificate, trust bundle and authorization file protected. Plain HTTP is not a supported MIAF deployment mode.
