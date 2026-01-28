##### [Back To Main](../../../README.md)
# Device's Workload Fleet Management Client

An edge device's Workload Fleet Management Client used by the Margo platform to manage application workloads, report device capabilities, and synchronize desired vs actual state with the WFM.

This repository contains a code-first-sandbox implementation that supports multiple runtimes (Helm for Kubernetes and Docker Compose) and is designed for easy extension (for example, adding other runtimes).

## Contents

- Overview
- Build and Run
- Configuration
- Runtimes & Examples
- Development & Tests
- Troubleshooting
- Contributing


## Overview

The Device Workload Fleet Management Client runs on edge devices and provides these core responsibilities:

- Onboarding and authentication with the Margo compliant WFM
- Reporting device capabilities (hardware, interfaces)
- Managing application workloads (Helm charts for Kubernetes runtime, Docker Compose for Docker runtime)
- Monitoring deployed workloads and reporting status changes in the deployments
- Periodic and event-driven state synchronization and reconciliation for synchronized state seeking

Design goals: small footprint, modular runtime adapters, event-driven reconciliation, and clear observability.

## Architecture (high level)

Key components (see source):

- `main.go` — application bootstrap, component wiring and lifecycle management
- `database/database.go` — lightweight in-memory DB with persistence and event hooks
- `onboarding.go` — device client registration, credentials and token management
- `device/capabilities.go` — capability discovery and reporting
- `stateSync.go` — reconciles desired vs actual state with the WFM
- `deployment.go` — deploy/update/remove workloads through runtime clients
- `monitor.go` — polls or subscribes to runtime state and emits status updates
- `status.go` — status reporting API calls to the WFM

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

Example minimal `config.yaml` (illustrative):

```yaml
logging:
  # Log level (e.g., DEBUG, INFO, WARN, ERROR, FATAL)
  level: DEBUG

# The device's root identity/attestation used for onboarding/registration of this device client with WFM (for auto-onboarding).
deviceRootIdentity:
  # Supported values: RANDOM, PKI, later on you can use it to add support for something like TPM, FIDO etc.
  identityType: "PKI"
  # Type-specific attestation or credential data. Keep only the keys that you need.
  attestation:
    # if you have set identity type as Random, then set these details
    # random:
    #   value: "this-is-a-unique-random-identity-attached-with-this-device"

    # if you have set identity type as PKI, then set these details
    pki:
      # certificate that the device client will present to the wfm during registration
      pubCertPath: ./config/device-public.crt

wfm:
  sbiUrl: https://10.139.2.248:8082/v1alpha2/margo #http://172.19.59.148:8082/v1alpha2/margo/sbi/v1
  clientPlugins:
    requestSigner:
      enabled: true
      hashAlgo: "sha256" # supported: sha256, used to create a hash of the payload
      signatureAlgo: "rsa" # supported: rsa, or ecdsa, these are used to sign the hash
      signatureFormat: "structured" # supported: structured, http-signature, how the signature will be added to the http request
      keyRef:
        path: "./config/device-private.key" # this should be the path to the private key pem file

    # NOTE: The oauth workflow is not yet defined in Margo spec, hence keep this disabled
    # the auth info is auto-fetched by the workload-fleet-management-client when it gets onboarded
    # but if you, for any reason, want to specify the oauth info, then you can pass that info over here
    authHelper:
      # if the wfm doesn't have oauth enabled on its endpoints, then set enable: false
      # and the workload-fleet-management-client will not add any authorization header in the request
      enabled: false

    # this plugin will be used to verify server tls certificates
    # note: we do not support mTLS hence client side certificates are not part of this configuration
    tlsHelper:
      enabled: true
      # path to the ca certificate that will be used to verify server certificates
      caKeyRef:
        path: "./config/ca-cert.pem"

stateSeeking:
  # How frequently the workload-fleet-management-client attempts to seek for the desired state from wfm
  # in seconds
  interval: 15

# the workload-fleet-management-client architecture is kept in a way that it is capable of managing more than one runtimes
# but one client with multiple devices is not defined by Margo yet, as it comes with its own complexities,
# for example: how would the client know which device should the application be deployed to? etc...
# If this is needed please reach out to the Margo group and follow the formal approach of Margo for SUPs.
# For now, always keep one runtime in this section, and comment all others
runtimes:
  - type: KUBERNETES
    kubernetes:
      kubeconfigPath: /root/.kube/config
  - type: DOCKER
    docker:
      url: unix:///var/run/docker.sock

# Note: Auto-discovery of device capabilities is not defined and hence not implemented yet,
# as a result you are supposed to provide the details in the file.
# Path to the capabilities file (required).
capabilities:
  readFromFile: ./config/capabilities.json

```

Device capabilities are JSON documents describing hardware, and resources. Place a file at the configured path or if you want then you can implement your own provider that posts capabilities to the WFM.

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
├─ config/
│  ├─ config.yaml
│  └─ capabilities.json
├─ database/
│  └─ database.go
├─ device/
│  └─ capabilities.go
└─ types/
   └─ *.go (config, types, errors)
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
- Capabilities file: validate JSON with `jq` before use

Logging

- The agent uses zap for structured logs. Configure log level in `config.yaml`.
- For container runs mount a log directory or direct logs to stdout/stderr for your runtime to capture.

Debugging tips

- Increase `logging.level` to `DEBUG` to get richer traces for state sync and deployment flows
- Inspect the in-memory DB persistence file under `data/` to see the last known state


## Security

- **TLS Verification**: The client can verify server TLS certificates when connecting to WFM. Configure this using the `tlsHelper` settings in the configuration file.
- **Plain HTTP (Not Recommended)**: For development or testing, the client supports unencrypted HTTP. Set `wfm.sbiUrl` to `http://` and `tlsHelper.enabled` to `false`. **Warning**: Only use HTTP in trusted networks.
- **Request Signing**: The client signs requests by default for enhanced security. To disable this feature, set `requestSigner.enabled` to `false`. **Warning**: Note that request signing is defined in Official Margo spec. Disable this when in development or debugging phase.
