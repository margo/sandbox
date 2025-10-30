# Device Agent

A Margo-compliant device agent for managing workloads on edge devices. The agent provides deployment, monitoring, and lifecycle management of applications across Kubernetes (Helm) and Docker Compose environments.
Note: This is a PoC and follows the standards and SUPs defined within Margo, please check the official docs.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Features](#features)
- [Development](#development)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)

## Overview

The Device Agent is an edge component of the Margo platform that:
- **Onboards devices** with the central orchestrator
- **Reports device capabilities** and hardware specifications
- **Manages workload deployments** using Helm or Docker Compose
- **Monitors workload health** and reports status back to orchestrator
- **Synchronizes state** between desired and actual deployment states

## Architecture

### Core Components

| Component | File | Purpose |
|-----------|------|---------|
| **Agent** | `main.go` | Main orchestrator managing all components and lifecycle |
| **Database** | `database/database.go` | In-memory database with disk persistence for deployment state |
| **State Syncer** | `stateSync.go` | Synchronizes desired vs actual state with orchestrator |
| **Deployment Manager** | `deployment.go` | Handles deployment, updates, and removal of workloads |
| **Monitor** | `monitor.go` | Monitors workload health and reports status changes |
| **Device Auth** | `onboarding.go` | Handles device registration and authentication |
| **Status Reporter** | `status.go` | Reports deployment status to orchestrator |
| **Capabilities Manager** | `device/capabilities.go` | Manages device capability discovery and reporting |

### Supported Runtimes

- **Kubernetes** - Via Helm v3 charts
- **Docker** - Via Docker Compose files

### Event-Driven Architecture

The agent uses an event-driven architecture where:
- Database changes trigger component actions
- Components communicate via database events
- Non-blocking operations with proper context handling
- Graceful shutdown with timeout management

## Quick Start

### Prerequisites

- Go 1.24.3 or later
- Docker and Docker daemon running
- Kubernetes cluster with kubectl configured (optional, for Helm deployments)
- Access to Margo orchestrator API

### Build and Run

1. **Clone and build:**
```bash
git clone https://github.com/margo/dev-repo
cd dev-repo/poc/device/agent
go build -o agent .
```

2. **Configure the agent:**
```bash
# Edit configuration files
cp config/config.yaml.example config/config.yaml
cp config/capabilities.json.example config/capabilities.json
# Update with your specific settings
```

3. **Run the agent:**
```bash
./agent
```

### Optimized Build

Use the provided build script for production deployment:

```bash
chmod +x build.sh
./build.sh
```

This creates an optimized, compressed binary with UPX compression.

### Docker Deployment to support Kubernetes runtime (for HELM apps)

```bash
# Build Docker image
docker build -f poc/device/agent/Dockerfile . -t margo.org/device-agent:latest

cd poc/device/agent

# Run container
docker run -d \
  -v $(pwd)/config:/config \
  -v /root/.kube/config:/root/.kube/config \
  -v ./data:/data \
  --name device-agent \
  margo.org/device-agent:latest
```

### Docker Deployment to manager Docker runtime (for compose apps)

```bash
# Build Docker image
docker build -f poc/device/agent/Dockerfile . -t margo.org/device-agent:latest

# Run container
docker run -d \
  -v $(pwd)/config:/config \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v ./data:/data \
  --name device-agent \
  margo.org/device-agent:latest
```

## Configuration

### Main Configuration (`config/config.yaml`)

```yaml
deviceId: device-101

wfm: 
  sbiUrl: http://localhost:8082/v1alpha2/margo/sbi/v1
  # Optional OAuth configuration
  # auth:
  #   clientId: your-client-id
  #   clientSecret: your-client-secret
  #   tokenUrl: https://auth.example.com/token

logging:
  level: DEBUG  # DEBUG, INFO, WARN, ERROR, FATAL

stateSeeking:
  # State synchronization interval in seconds
  interval: 15

# Multiple runtime support
runtimes:
  # Kubernetes runtime via Helm
  - type: KUBERNETES
    kubernetes:
      kubeconfigPath: /root/.kube/config
  
  # Docker runtime via Compose
  - type: DOCKER
    docker:
      url: unix:///var/run/docker.sock
      tlsSkipVerification: true
      # Optional TLS configuration
      # tls:
      #   cacertPath: /path/to/ca.pem
      #   certPath: /path/to/cert.pem
      #   keyPath: /path/to/key.pem

capabilities:
  readFromFile: /path/to/capabilities.json
```

### Device Capabilities (`config/capabilities.json`)

Define your device's hardware specifications:

```json
{
  "apiVersion": "device.margo/v1",
  "kind": "DeviceCapability",
  "properties": {
    "id": "northstarida.xtapro.k8s.edge",
    "vendor": "Northstar Industrial Applications",
    "modelNumber": "332ANZE1-N1",
    "serialNumber": "PF45343-AA",
    "roles": ["standalone cluster", "cluster lead"],
    "resources": {
      "memory": 64,
      "storage": 2000,
      "cpus": [{
        "architecture": "Intel x64",
        "cores": 24,
        "frequency": 6.2
      }]
    },
    "peripherals": [{
      "name": "NVIDIA GeForce RTX 4070 Ti SUPER OC Edition",
      "type": "GPU",
      "modelNumber": "TUF-RTX4070TIS-O16G",
      "properties": {
        "manufacturer": "NVIDIA",
        "ram": "16 GB",
        "clockSpeed": "2640 MHz"
      }
    }],
    "interfaces": [{
      "name": "RTL8125 NIC 2.5G Gigabit LAN",
      "type": "Ethernet",
      "modelNumber": "RTL8125",
      "properties": {
        "maxSpeed": "2.5 Gbps"
      }
    }]
  }
}
```

## Features

### State Synchronization
- **Periodic sync** with orchestrator (configurable interval)
- **Event-driven updates** for immediate state changes
- **Conflict resolution** between desired and actual states
- **Automatic reconciliation** when states diverge

### Workload Management

#### Helm Deployments
- Full lifecycle management of Helm charts
- Support for chart repositories and revisions
- Values injection from deployment parameters
- Release naming with deployment ID correlation

#### Docker Compose Deployments
- Container orchestration via Docker Compose
- Support for remote and local compose files
- Environment variable injection from parameters
- Project naming with deployment ID correlation

### Monitoring & Health Checks
- **Continuous monitoring** of deployed workloads
- **Status reporting** to orchestrator on state changes
- **Component-level health tracking**
- **Automatic failure detection and reporting**

### Persistence
- **In-memory database** with disk backup for performance
- **Atomic write operations** for data consistency
- **Automatic state restoration** on agent restart
- **Event-driven architecture** for real-time updates

### Error Handling
- **Structured error types** with component and operation context
- **Retryable error classification**
- **Comprehensive logging** with structured fields

## Development

### Project Structure

```
poc/device/agent/
├── main.go                    # Application entry point and Agent struct
├── onboarding.go             # Device authentication and onboarding
├── stateSync.go              # State synchronization with orchestrator
├── deployment.go             # Deployment lifecycle management
├── monitor.go                # Workload monitoring and health checks
├── status.go                 # Status reporting to orchestrator
├── build.sh                  # Optimized build script with UPX compression
├── Dockerfile                # Multi-stage Docker build
├── config/
│   ├── config.yaml           # Main agent configuration
│   └── capabilities.json     # Device hardware capabilities
├── database/
│   └── database.go           # In-memory database with persistence
├── device/
│   └── capabilities.go       # Capabilities management
└── types/
    ├── config.go             # Configuration structures and validation
    ├── error.go              # Structured error handling
    └── apiClient.go          # API client factory
```

### Key Interfaces

```go
// Core component interfaces
type StateSyncerIfc interface {
    Start()
    Stop()
}

type DeploymentManagerIfc interface {
    Start()
    Stop()
}

type DeploymentMonitorIfc interface {
    Start()
    Stop()
}

type StatusReporterIfc interface {
    Start()
    Stop()
}
```

### Adding New Runtime Support

1. **Implement workload interfaces** in `shared-lib/workloads/`
2. **Add runtime configuration** to `types/config.go`
3. **Update deployment manager** to handle new runtime type
4. **Add monitoring support** in `monitor.go`
5. **Register runtime** in agent initialization

#### Development Extension Example: Adding Podman Runtime Support

The following example tries to extending the device agent to support Podman as a new container runtime,
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

type PodmanConnectivityParams struct {
    RemoteURL         string
}

func NewPodmanClient(params PodmanConnectivityParams) (*PodmanClient, error) {
    // ... some code
    var conn Connection := PodmanConnectionObject()  
    return &PodmanClient{
        connection: conn,
        workingDir: "/tmp/podman-deployments",
    }, nil
}

func (p *PodmanClient) DeployCompose(ctx context.Context, projectName string, composeContent []byte, envVars map[string]string) error {
    // ... your code
    return nil
}

func (p *PodmanClient) RemoveCompose(ctx context.Context, projectName string) error {
    // ... your code
    return nil
}

func (p *PodmanClient) GetComposeStatus(ctx context.Context, projectName string) (*ComposeStatus, error) {
    // ... your code
    return &ComposeStatus{
      Name:     projectName,
      Status:   overallStatus,
      Services: services,
    }, nil
}
```

#### Step 3: Update Agent Initialization

Modify `main.go` to support Podman:

```go
func NewAgent(configPath string) (*Agent, error) {
    // ... existing code ...
    
    var helmClient *workloads.HelmClient
    var composeClient *workloads.DockerComposeClient
    var podmanClient *workloads.PodmanClient // New client
    
    for _, runtime := range cfg.Runtimes {
        switch runtime.Type {
        case "KUBERNETES":
            //... existing code
        case "DOCKER":
            //... existing code
        case "PODMAN": // New runtime support
            if runtime.Podman != nil {
                podmanClient, err = workloads.NewPodmanClient(workloads.PodmanConnectivityParams{
                    RemoteURL:         runtime.Podman.RemoteURL,
                })
                if err != nil {
                    return nil, err
                }
            }
        }
    }
    
    // Update deployer and monitor to include Podman client
    deployer := NewDeploymentManager(db, helmClient, composeClient, podmanClient, log)
    monitor := NewDeploymentMonitor(db, helmClient, composeClient, podmanClient, log)
    
    // ... rest of function
}
```

#### Step 4: Update Deployment Manager

Modify `deployment.go` to handle Podman deployments:

```go
type DeploymentManager struct {
  // ... existing code
  podmanClient  *workloads.PodmanClient // New client
}

func (dm *DeploymentManager) deployOrUpdate(ctx context.Context, deploymentId string, desiredState sbi.AppState) {
    // ... existing code ...
    
    // Determine deployment type and route accordingly
    profileType := appDeployment.Spec.DeploymentProfile.Type
    switch profileType {
    case sbi.HelmV3:
        err = dm.deployOrUpdateHelm(ctx, deploymentId, appDeployment)
    case sbi.Compose:
        err = dm.deployOrUpdateCompose(ctx, deploymentId, appDeployment)
    case sbi.PodmanCompose: // New deployment type
        err = dm.deployOrUpdatePodman(ctx, deploymentId, appDeployment)
    default:
        dm.database.SetPhase(deploymentId, "FAILED", fmt.Sprintf("Unsupported deployment type: %s", profileType))
    }
    
    // ... rest of function
}

func (dm *DeploymentManager) deployOrUpdatePodman(ctx context.Context, deploymentId string, appDeployment sbi.AppDeployment) error {
    // ... your code
    dm.log.Infow("Podman deployment successful", "appId", deploymentId, "projectName", projectName)
    return nil
}
```

#### Step 5: Update Configuration Example

Add Podman configuration to `config/config.yaml`:

` ` `yaml
deviceId: device-101

wfm: 
  sbiUrl: `http://localhost:8082/v1alpha2/margo/sbi/v1`

logging:
  level: DEBUG
  ```yaml
stateSeeking:
  interval: 15

runtimes:
  # New Podman runtime support
  - type: PODMAN
    podman:
      socketPath: /run/podman/podman.sock
      connectionTimeout: 30
      # Optional remote connection
      # remoteUrl: ssh://user@host/run/podman/podman.sock
      # tls:
      #   cacertPath: /path/to/ca.pem
      #   certPath: /path/to/cert.pem
      #   keyPath: /path/to/key.pem

capabilities:
  readFromFile: /root/margo/dev-repo/poc/device/agent/config/capabilities.json
```

#### Step 6: Update Monitor Component

Modify `monitor.go` to include Podman monitoring:

```go
type DeploymentMonitor struct {
  // ... existing code
  podmanClient  *workloads.PodmanClient // New client
}

func NewDeploymentMonitor(db database.DatabaseIfc, helmClient *workloads.HelmClient, composeClient *workloads.DockerComposeClient, podmanClient *workloads.PodmanClient, log *zap.SugaredLogger) *DeploymentMonitor {
    return &DeploymentMonitor{
        //... existing code
        podmanClient:  podmanClient, // Initialize new client
        log:           log,
        stopChan:      make(chan struct{}),
    }
}

func (hm *DeploymentMonitor) checkDeployment(appID string) {
    // ... exiting code
    // Route monitoring based on deployment type
    profileType := appDeployment.Spec.DeploymentProfile.Type
    switch profileType {
    case sbi.HelmV3:
        hm.checkHelmDeployment(appID, appDeployment)
    case sbi.Compose:
        hm.checkComposeDeployment(appID, appDeployment)
    case sbi.PodmanCompose: // New monitoring support
        hm.checkPodmanDeployment(appID, appDeployment)
    }
}

func (hm *DeploymentMonitor) checkPodmanDeployment(appID string, appDeployment sbi.AppDeployment) {
    //... existing code
}

func (hm *DeploymentMonitor) convertPodmanStatus(status string) sbi.ComponentStatusState {
    // ... exemplary code
    switch status {
    case "running":
        return sbi.ComponentStatusStateInstalled
    case "stopped", "exited":
        return sbi.ComponentStatusStateFailed
    case "starting":
        return sbi.ComponentStatusStateInstalling
    default:
        return sbi.ComponentStatusStateFailed
    }
}
```

#### Step 7: Add Unit Tests

Create `shared-lib/workloads/podman_test.go`:

```go
package workloads

import (
    "context"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNewPodmanClient(t *testing.T) {
    tests := []struct {
        name        string
        params      PodmanConnectivityParams
        expectError bool
    }{
        {
            name: "Valid remote URL",
            params: PodmanConnectivityParams{
                RemoteURL: "ssh://user@host/run/podman/podman.sock",
            },
            expectError: true, // Will fail in test environment
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            client, err := NewPodmanClient(tt.params)
            if tt.expectError {
                assert.Error(t, err)
                assert.Nil(t, client)
            } else {
                assert.NoError(t, err)
                assert.NotNil(t, client)
            }
        })
    }
}
```

#### Step 8: Update Dependencies

Add to `go.mod`:

```go
require (
    // ... existing dependencies ...
    github.com/containers/podman/v4 v4.9.0
)
```

#### Step 9: Documentation Update

Add to README.md:

```markdown
### Supported Runtimes

- **Kubernetes** - Via Helm v3 charts
- **Docker** - Via Docker Compose files  
- **Podman** - Via Podman Compose (rootless container support)

#### Podman Configuration Example

```yaml
runtimes:
  - type: PODMAN
    podman:
      socketPath: /run/podman/podman.sock
      connectionTimeout: 30
```

## Usage Example

With this extension, users can now configure Podman runtime:

```yaml
# config/config.yaml
deviceId: edge-device-podman-001

runtimes:
  - type: PODMAN
    podman:
      socketPath: /run/user/1000/podman/podman.sock  # Rootless Podman
      connectionTimeout: 30
```

This extension demonstrates:
- **Configuration expansion** for new runtime types
- **Client implementation** following existing patterns
- **Integration** with existing deployment and monitoring flows
- **Testing strategy** for new components
- **Documentation updates** for user guidance

The modular architecture makes it straightforward to add new runtime support while maintaining consistency with existing patterns.

### Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./database/
go test ./types/
```

## Troubleshooting

### Common Issues

1. **Connection refused to orchestrator**
   ```bash
   # Check orchestrator connectivity
   curl -f http://localhost:8082/health
   
   # Verify configuration
   grep sbiUrl config/config.yaml
   ```

2. **Docker socket permission denied**
   ```bash
   # Add user to docker group
   sudo usermod -aG docker $USER
   
   # Or run with sudo
   sudo ./agent
   ```

3. **Kubernetes deployment failures**
   ```bash
   # Test kubectl connectivity
   kubectl cluster-info
   
   # Verify kubeconfig path
   ls -la /root/.kube/config
   ```

4. **Capabilities file not found**
   ```bash
   # Verify file format
   cat config/capabilities.json | jq .
   ```

5. **State sync failures**
   ```bash
   # Check agent logs for sync errors
   grep "Failed to sync states" /var/log/agent.log
   
   # Verify reachability to orchestrator API endpoint
   curl -X POST http://localhost:8082/v1alpha2/margo/sbi/v1/state
   ```

### Logs

The agent uses structured logging with zap:

```bash
# View real-time logs
tail -f /var/log/agent.log

# Filter by log level
grep "ERROR" /var/log/agent.log
grep "WARN" /var/log/agent.log

# Filter by component
grep "deployment" /var/log/agent.log
grep "monitoring" /var/log/agent.log
grep "onboarding" /var/log/agent.log
```


### Debug Mode

Enable debug logging for detailed troubleshooting:

```yaml
# In config/config.yaml
logging:
  level: DEBUG
```

## API Integration

The agent communicates with the Margo orchestrator via REST APIs:

- **Onboarding**: `POST /onboarding/device`
- **Capabilities**: `POST /device/{deviceId}/capabilities`
- **State Sync**: `POST /state`
- **Status Reporting**: `POST /device/{deviceId}/deployment/{deploymentId}/status`

## Other Considerations

- **Memory Usage**: In-memory database with configurable persistence intervals
- **CPU Usage**: Configurable monitoring and sync intervals
- **Network**: Efficient state synchronization with delta updates
- **Storage**: Minimal disk usage for state persistence

## Security

- **Authentication**: OAuth 2.0 client credentials flow support
- **TLS**: Configurable TLS for Docker daemon connections

## Contributing

### Code Style

- Follow Go conventions and use `gofmt`
- Use structured logging with context fields
- Include comprehensive error handling with `AgentError` types
- Write table-driven tests for all components
- Document public interfaces and complex logic

### Adding New Features

1. **Component Interface**: Define clear interfaces for new components
2. **Error Handling**: Use `AgentError` for structured error reporting
3. **Configuration**: Add new config options to `types/config.go`
4. **Testing**: Include unit tests and integration tests
5. **Documentation**: Update README and inline documentation

## Agent Runtime management

### Single Runtime Management(docker)
```yaml
runtimes:
  - type: DOCKER
    docker:
      url: unix:///var/run/docker.sock
```

### Single Runtime Management(kubernetes)
```yaml
runtimes:
  - type: KUBERNETES
    kubernetes:
      kubeconfigPath: /root/.kube/config
```

### Multi-Runtime Deployment (not advised, it just adds up complexities)
```yaml
runtimes:
  - type: KUBERNETES
    kubernetes:
      kubeconfigPath: /root/.kube/config
  - type: DOCKER
    docker:
      url: unix:///var/run/docker.sock
```

## License

[License information - TBD]

---

**Note**: This agent is part of the Margo PoC. For complete documentation and API specifications, refer to the main Margo documentation and reach out to the stakeholders.