##### [Back To Main](../README.md)
# MARGO Development Repository

A development repository for the MARGO project - an edge computing orchestration system that manages workloads across margo compliant devices and workload orchestrators.

## Overview

This repository contains the core components, shared libraries, proof-of-concepts, and tooling for the MARGO reference implementation.

## Repository Structure

```
dev-repo/
├── docker-compose  # Files related to running device-agent as docker container
├── docs            # Documentation related to MARGO project
├── go.mod          # Go module dependencies
├── go.sum          # Go module checksums
├── .github/        # GitHub workflows and templates
├── .vscode/        # VS Code configuration
├── helmchart       # Helmchart files to run device-agent as pod
├── LICENSE         # Project license
├── non-standard    # Sandbox enabling components (components that are not defined in MARGO, but needed for a complete PoC)
├── pipeline        # Automation scripts for build, deployment and run.
├── poc             # MARGO reference implementations and code for device-agent
├── README.md       # Main markdown file for whole project which links to other .md files
├── shared-lib      # Reusable libraries and utilities 
└── standard        # Standard MARGO components and APIs
```

## Core Components

### 🤖 Device Agent (`poc/device/agent/`)
Edge device agent that manages workload deployments on device and communicates with the workload orchestrator for state seeking,  status updates and other operations.

**Key Features:**
- Multi-runtime support (Kubernetes Distributions(for Helm workloads), Docker(for docker-compose workloads))
- Event-driven architecture with state synchronization
- Device onboarding and capability reporting
- Workload lifecycle management and monitoring
- In-memory database with persistence

**Quick Start:**
```bash
cd poc/device/agent
go build -o agent .
./agent
```

NOTE: Please check the [Agent Docs](../poc/device/agent/README.md). It has comprehensive literature on how it works, and how to extend its development.

### 📚 Shared Libraries (`shared-lib/`)
Reusable Go libraries providing common functionality across MARGO components.

**Libraries:**
- **HTTP utilities** (`http/`) - HTTP client with authentication utilities
- **Workload management** (`workloads/`) - Helm and Docker Compose clients
- **File operations** (`file/`) - File download and manipulation utilities

### 🛠️ Development Tools (`pipeline/`) 
Scripts and utilities for development, testing, and deployment automation.

**Tools:**
- **Setup script** (`wfm.sh`, `device-agent.sh`) - Automated environment setup (Gogs, Harbor, device-agent, Symphony etc.)
- **WFM CLI** (`wfm-cli.sh`) - Interactive menu with options to upload/apply/delete app packages, deploy/delete instances.


### 📋 Standard Components (`standard/`)
Official MARGO API specifications, generated code, and standard implementations.

**Contents:**
- Standard data models and schemas derived from the Official MARGO spec literature 
- Generated API clients and server stubs
- Protocol definitions and interfaces

### 🧪 Proof of Concepts (`poc/`)
Experimental implementations and prototypes for new features.

## Quick Start

### Prerequisites

- **Go 1.24.3+** for building components
- **Docker** for container-based deployments
- **Kubernetes cluster** (optional, for Helm deployments)
- **Git** for version control

### Environment Setup

1. **Clone the repository:**
```bash
git clone https://github.com/margo/dev-repo
cd dev-repo
```

2. **Install dependencies:**
```bash
go mod download
```


3. **Build and run device agent:**
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



## Development Workflow

### Adding New Features

1. **Shared functionality** → Add to `shared-lib/`
2. **API changes from Official MARGO Spec** → Update `standard/` specifications
2. **API changes needed for PoC but not defined in MARGO spec** → Update `non-standard/` specifications
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

