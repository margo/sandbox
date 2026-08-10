##### [Back To Main](../README.md)
# MARGO Development Repository

A development repository for the MARGO project.

## Overview

This repository contains the proof-of-concept components, shared libraries, standard spec, and tooling for the MARGO reference implementation.

## Repository Structure

```
sandbox/
├── .github/        # GitHub workflows and templates
├── .vscode/        # VS Code configuration
├── LICENSE         # Project license
├── README.md       # Main markdown file for whole project which links to other .md files
├── docs            # Documentation related to the code development that are done for the MARGO project
├── go.mod          # Go module dependencies
├── go.sum          # Go module checksums
└── standard        # MARGO API definitions copied from the official sources and the auto-generated schema code is placed here. It is a Go-package and is imported in the main codebase.
├── non-standard    # Sandbox enabling APIs/components (these are not defined in MARGO, but needed for a complete workflow for Code-first Sandbox). It is a Go-package and is imported in the main codebase.
├── shared-lib      # Go-package. Reusable libraries, and utilities. It is a Go-package and is imported in the main codebase.
├── poc             # The runnable implementations and code for code-first-sandbox.
├── scripts        # Automation scripts for build, deployment and run.
├── docker-compose  # Files related to running Workload Fleet Management Client using docker-compose
├── helmchart       # Helmchart files to run Workload Fleet Management Client in kubernetes environment
```

## Core Components

### 🧪 Proof of Concepts (`poc/`)
Experimental implementations and prototypes for Margo.

#### Subdirectories:
- `poc/device/agent` -- The device's Workload Fleet Management Client codebase
- `poc/wfm/cli` -- The client wrapper that can be used to talk to Margo compliant APIs on wfm
- `poc/tests` -- Test artefacts like Margo Application Descriptions etc.

## Quick Start

### Prerequisites

- **Go 1.24.3+** for building components
- **Docker** for container-based deployment
- **Kubernetes cluster** (for Helm based deployment)
- **Git** for version control

### Environment Setup

1. **Clone the repository:**
```bash
git clone https://github.com/margo/sandbox
cd sandbox
```

2. **Install dependencies:**
```bash
go mod download
```

3. **Build and run device's Workload Fleet Management Client:**
```bash
cd poc/device/agent
go build -o agent .
./agent
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...


### 🤖 Device's Workload Fleet Management Client (`poc/device/agent/`)
Edge device's workload fleet management client that manages workload deployments on device and communicates with the workload-orchestrator/fleet-manager for state seeking, deployment status updates and other operations.

**Key Features:**
- Multi-runtime support (Kubernetes Distributions(for Helm workloads), Docker(for docker-compose workloads))
- Event-driven architecture with state synchronization with workfleet-orchestrator/fleet-manager
- Device onboarding and capability reporting
- Workload lifecycle management and monitoring
- In-memory database with persistence on disk

NOTE: Please check the [Workload Fleet Management Client Docs](../poc/device/agent/README.md). It has comprehensive literature on how it works, and how to extend its development.

### 📚 Shared Libraries (`shared-lib/`)
Reusable Go libraries providing common functionality across MARGO components.

**Libraries:**
- **Git based operations** (`git/`) - Pull repos from Git etc...
- **File operations** (`file/`) - File download and manipulation utilities
- **Cryptography based helpers** (`crypto/`) - TLS, payload signing using certificates etc.
- **HTTP utilities** (`http/`) - HTTP client with authentication utilities
- **Container image operations** (`oci/`) - Pull image from container image repos etc...
- **Workload management** (`workloads/`) - Helm and Docker Compose clients
- **Archive** (`archive/`) - Unpacking or packing archives(tar.gz) etc..
- **Pointer operations** (`pointers/`) - Some helper functions to deep clone, safely get pointer to temp variables etc...

### 🛠️ Development Tools (`scripts/`)
Scripts and utilities for development, testing, and deployment automation.

**Tools:**
- **Setup script** (`wfm.sh`, `device-agent.sh`) - Automated environment setup (Harbor, device's Workload Fleet Management Client, Symphony etc.)
- **EasyCLI** (`wfm-cli.sh`) - EasyCLI is an interactive menu with options to upload/apply/delete app packages, deploy/delete instances.


### 📋 Standard Components (`standard/`)
Official MARGO API specifications, and auto-generated code.

**Contents:**
- Standard data models and schemas derived from the Official MARGO spec literature
- Generated API clients and server stubs
- Protocol definitions and interfaces

### 📋 Non-Standard Components (`non-standard/`)

This directory contains API definitions that fall outside the Margo specification but are necessary to complete end-to-end workflows. It serves two purposes:

1. **Workflow Completion**: Implements APIs needed for end-to-end implementation that are not part of Margo
2. **Reference Implementation**: Demonstrates complete workflow patterns for WFM developers

**Example**: While Margo specifies that Application Descriptions are hosted in Git repositories, it doesn't define how WFM discovers repository locations or manages credentials. We created a custom WFM API for passing repository metadata. This non-standard API enables uploading Application Description metadata to WFM. Similarly, other APIs in this directory facilitate the other workflows.

**Note**: These specifications are reference implementations only and are not part of official Margo. WFM developers may use these as guidance or implement their own solutions. No official support is provided for these non-standard components.

## Development Workflow

### Adding New Features

1. **Shared functionality** → Add to `shared-lib/`
2. **API changes from Official MARGO Spec** → Update `standard/` specifications
2. **API changes needed for Code-first Sandbox but not defined in MARGO spec** → Update `non-standard/` specifications
3. **Implementation of the standard and non-standard features** → Implement in `poc/`
4. **Testing utilities** → Add to `tools/`

### Code Organization

- **Error handling** - Use structured errors with context
- **Logging** - Structured logging with zap
- **Testing** - Table-driven tests with mocks

### Testing Strategy

```bash
# Unit tests
go test ./shared-lib/...
go test ./poc/device/agent/...
```

## Deployment Options

### Development
```bash
# Local development
go run ./poc/device/agent

# With custom config
go run ./poc/device/agent -config custom-config.yaml
```
