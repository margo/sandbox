# Margo Development Repository

A development repository for the Margo platform - an edge computing orchestration system that manages workloads across margo compliant devices and workload orchestrators.

## Overview

This repository contains the core components, shared libraries, proof-of-concepts, and tooling for the Margo PoCs.

## Repository Structure

```
dev-repo/
├── README.md                 # This file
├── LICENSE                   # Project license
├── go.mod                    # Go module dependencies
├── go.sum                    # Go module checksums
├── .github/                  # GitHub workflows and templates
├── .vscode/                  # VS Code configuration
├── config/                   # Global configuration files
├── tools/                    # Development and testing tools
├── shared-lib/               # Reusable libraries and utilities
├── standard/                 # Standard Margo components and APIs
├── non-standard/             # Experimental and non-standard components that are not defined in Margo, but needed for a complete PoC
└── poc/                      # Proof-of-concept implementations
```

## Core Components

### 🤖 Device Agent (`poc/device/agent/`)
Edge device agent that manages workload deployments on device and talks to the workload orchestrator for state seeking,  status updates and some other operations.

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

NOTE: Please check the README.md given in `poc/device/agent` directory. It has comprehensive literature on how it works, and how to extend its development.

### 📚 Shared Libraries (`shared-lib/`)
Reusable Go libraries providing common functionality across Margo components.

**Libraries:**
- **HTTP utilities** (`http/`) - HTTP client with authentication utilties
- **Workload management** (`workloads/`) - Helm and Docker Compose clients
- **File operations** (`file/`) - File download and manipulation utilities

### 🛠️ Development Tools (`tools/`) [TBD -- Incomplete]
Scripts and utilities for development, testing, and deployment automation.

**Tools:**
- **Setup script** (`setup.sh`) - Automated environment setup (Gogs, Harbor, Keycloak, Symphony)
- **Test suite** (`tests.sh`) - Comprehensive testing framework
- **Test cases** (`helm-app-pkg-testcases.yaml`) - YAML-based test definitions

### 📋 Standard Components (`standard/`)
Official Margo API specifications, generated code, and standard implementations.

**Contents:**
- Standard data models and schemas derived from the Official Margo spec literature 
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

3. **Setup development environment:**
```bash
cd tools
chmod +x setup.sh
./setup.sh
```

4. **Build and run device agent:**
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

# Run integration tests
cd tools
./test_runner.sh
```

## Development Workflow

### Adding New Features

1. **Shared functionality** → Add to `shared-lib/`
2. **API changes from Official Margo Spec** → Update `standard/` specifications
2. **API changes needed for PoC but not defined in Margo spec** → Update `non-standard/` specifications
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

# Integration tests
cd tools
# End-to-end tests
./test_runner.sh -f test-cases/e2e-tests.yaml
```

## Deployment Options

### Development
```bash
# Local development
go run ./poc/device/agent

# With custom config
go run ./poc/device/agent -config custom-config.yaml
```

### Production
[TBD]

## Contributing

### Development Setup

1. **Fork the repository**
2. **Create feature branch**: `git checkout -b feature/new-feature`
3. **Follow code standards**: Run `gofmt` and `golangci-lint`
4. **Write tests**: [Not a priority]
5. **Update documentation**: Include README updates
6. **Submit PR**: Include tests and documentation

### Code Standards

- **Go conventions** - Follow effective Go practices
- **Error handling** - Use structured errors with context
- **Testing** - Table-driven tests with proper mocking [Not a priority]
- **Documentation** - Godoc comments for public APIs
- **Logging** - Structured logging with appropriate levels

## Troubleshooting

### Common Issues

1. **Build failures** - Check Go version and dependencies
2. **Agent connectivity** - Verify orchestrator URL and network access
3. **Runtime errors** - Check Docker