#!/bin/bash

set -euo pipefail  # Exit on error, undefined vars, and pipe failures

# =============================================================================
# Configuration Variables
# =============================================================================
INSTALL_GOLANG=${INSTALL_GOLANG:-true}
GOLANG_VERSION=${GOLANG_VERSION:-"1.24"}
GOPATH=${GOPATH:-"/usr/local/go"}

INSTALL_MAGE_TOOL=${INSTALL_MAGE_TOOL:-true}
INSTALL_DOCKERBUILDX=${INSTALL_DOCKERBUILDX:-false}

# ./buildx build --no-cache --platform linux/amd64 -t mjlatest . -f Dockerfile

# Repository Configuration
PULL_REPO_FROM_GIT=${PULL_REPO_FROM_GIT:-true}
GIT_REPO_URL=${GIT_REPO_URL:-"https://github.com/eclipse-symphony/symphony.git"}
GIT_BRANCH=${GIT_BRANCH:-"0.48.35"}
PATH_TO_REPO_CODE_ON_DISK=${PATH_TO_REPO_CODE_ON_DISK:-"symphony-codebase"}

# Build Configuration
BUILD_CLI=${BUILD_CLI:-true}
BUILD_API=${BUILD_API:-true}
BUILD_AGENT=${BUILD_AGENT:-false}
BUILD_CONTAINERS=${BUILD_CONTAINERS:-false}

# Paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CUSTOM_API_DIR="${SCRIPT_DIR}/api"
CUSTOM_CLI_DIR="${SCRIPT_DIR}/cli"
SDK_PATH="../../../../../sdk"

# =============================================================================
# Utility Functions
# =============================================================================
log_info() {
    echo "[INFO] $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

log_error() {
    echo "[ERROR] $(date '+%Y-%m-%d %H:%M:%S') - $1" >&2
}

log_warn() {
    echo "[WARN] $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

cleanup_on_exit() {
    local exit_code=$?
    if [ $exit_code -ne 0 ]; then
        log_error "Build failed with exit code $exit_code"
        # Add any cleanup logic here if needed
    fi
    exit $exit_code
}

check_command() {
    if ! command -v "$1" &> /dev/null; then
        log_error "Required command '$1' not found"
        return 1
    fi
}

# =============================================================================
# Setup Functions
# =============================================================================
install_golang() {
    if [ "$INSTALL_GOLANG" != "true" ]; then
        log_info "Skipping Go installation"
        return 0
    fi

    log_info "Installing Go $GOLANG_VERSION..."
    
    # Check if Go is already installed with correct version
    if command -v go &> /dev/null; then
        local current_version=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
        if [ "$current_version" = "$GOLANG_VERSION" ]; then
            log_info "Go $GOLANG_VERSION already installed"
            return 0
        fi
    fi

    # Install Go (implementation depends on your system)
    log_warn "Go installation not implemented - please install Go $GOLANG_VERSION manually"
}

install_mage() {
    if [ "$INSTALL_MAGE_TOOL" != "true" ]; then
        log_info "Skipping Mage installation"
        return 0
    fi

    log_info "Installing Mage build tool..."
    
    if command -v mage &> /dev/null; then
        log_info "Mage already installed"
        return 0
    fi

    go install github.com/magefile/mage@latest
    export PATH=$PATH:$(go env GOPATH)/bin
    log_info "Mage installation completed"
}

install_dockerbuildx() {
		if [ "$INSTALL_DOCKERBUILDX" != "true" ]; then
				log_info "Skipping docker installation"
				return 0
		fi

		log_info "Installing dockerbuildx..."

    # Download buildx binary
    BUILDX_VERSION=$(curl -s https://api.github.com/repos/docker/buildx/releases/latest | grep tag_name | cut -d '"' -f 4)
    curl -L https://github.com/docker/buildx/releases/download/${BUILDX_VERSION}/buildx-${BUILDX_VERSION}.linux-amd64 -o ~/.docker/cli-plugins/docker-buildx

    # Make it executable
    chmod +x ~/.docker/cli-plugins/docker-buildx

    # Create the plugins directory if it doesn't exist
    mkdir -p ~/.docker/cli-plugins
}

# =============================================================================
# Repository Management Functions
# =============================================================================
setup_repository() {
    if [ "$PULL_REPO_FROM_GIT" = "true" ]; then
        clone_repository
    else
        setup_local_repository
    fi
}

clone_repository() {
    log_info "Setting up repository from Git"
    
    # Validate Git URL
    if [ -z "$GIT_REPO_URL" ]; then
        log_error "GIT_REPO_URL cannot be empty when PULL_REPO_FROM_GIT=true"
        exit 1
    fi

    # Clean up existing directory
    if [ -d "$PATH_TO_REPO_CODE_ON_DISK" ]; then
        log_info "Removing existing repository code from: $PATH_TO_REPO_CODE_ON_DISK"
        rm -rf "$PATH_TO_REPO_CODE_ON_DISK"
    fi

    log_info "Cloning repository from: $GIT_REPO_URL (branch: $GIT_BRANCH)"
    git clone --branch "$GIT_BRANCH" --depth 1 "$GIT_REPO_URL" "$PATH_TO_REPO_CODE_ON_DISK"
    
    log_info "Repository cloned successfully"
}

setup_local_repository() {
    log_info "Using local repository code"
    
    # Create directory if it doesn't exist
    mkdir -p "$PATH_TO_REPO_CODE_ON_DISK"
    
    # Validate directory exists and is accessible
    if [ ! -d "$PATH_TO_REPO_CODE_ON_DISK" ]; then
        log_error "Failed to create or access local repository path: $PATH_TO_REPO_CODE_ON_DISK"
        exit 1
    fi
    
    log_info "Using local repository at: $PATH_TO_REPO_CODE_ON_DISK"
}

copy_custom_code() {
    log_info "Copying custom code to repository"
    
    # Validate source directories exist
    if [ ! -d "$CUSTOM_API_DIR" ]; then
        log_error "Custom API directory not found: $CUSTOM_API_DIR"
        exit 1
    fi
    
    if [ ! -d "$CUSTOM_CLI_DIR" ]; then
        log_error "Custom CLI directory not found: $CUSTOM_CLI_DIR"
        exit 1
    fi

    # Copy custom code
    log_info "Copying API customizations..."
    cp -r "$CUSTOM_API_DIR"/* "$PATH_TO_REPO_CODE_ON_DISK/api/"
    
    log_info "Copying CLI customizations..."
    cp -r "$CUSTOM_CLI_DIR"/* "$PATH_TO_REPO_CODE_ON_DISK/cli/"
    
    log_info "Custom code copied successfully"
}

# =============================================================================
# Build Functions
# =============================================================================
setup_go_modules() {
    log_info "Setting up Go modules"
    
    ###########################################################################
    # change the go.mod file in the cli directory
    cd "$PATH_TO_REPO_CODE_ON_DISK/cli"

    # Add replace directive for SDK
    if ! grep -q "github.com/margo/dev-repo/sdk" go.mod; then
        log_info "Adding SDK replace directive to go.mod"
        echo "replace github.com/margo/dev-repo/sdk => $SDK_PATH" >> go.mod
    else
        log_info "SDK replace directive already exists in go.mod"
    fi
    
    # Tidy and vendor modules
    log_info "Running go mod tidy..."
    go mod tidy
    
    log_info "Running go mod vendor..."
    go mod vendor
    cd -

    ###########################################################################
    # change the go.mod file in the api directory as well
    cd "$PATH_TO_REPO_CODE_ON_DISK/api"
    
    # Add replace directive for SDK
    if ! grep -q "github.com/margo/dev-repo/sdk" go.mod; then
        log_info "Adding SDK replace directive to go.mod"
        echo "replace github.com/margo/dev-repo/sdk => $SDK_PATH" >> go.mod
    else
        log_info "SDK replace directive already exists in go.mod"
    fi
    
    # Tidy and vendor modules
    log_info "Running go mod tidy..."
    go mod tidy
    
    log_info "Running go mod vendor..."
    go mod vendor
    cd - > /dev/null
}

build_components() {
    log_info "Starting build process"
    
    cd "$PATH_TO_REPO_CODE_ON_DISK"
    
    if [ "$BUILD_CLI" = "true" ]; then
        log_info "Building Maestro CLI..."
        mage buildCli || {
            log_error "CLI build failed"
            exit 1
        }
        log_info "CLI build completed successfully"
    fi
    
    if [ "$BUILD_API" = "true" ]; then
        log_info "Building Symphony API server..."
        mage buildApi || {
            log_error "API build failed"
            exit 1
        }
        log_info "API build completed successfully"
    fi
    
    if [ "$BUILD_AGENT" = "true" ]; then
        log_info "Building Symphony Agent..."
        mage buildAgent || {
            log_error "Agent build failed"
            exit 1
        }
        log_info "Agent build completed successfully"
    fi
    
    if [ "$BUILD_CONTAINERS" = "true" ]; then
        log_info "Building container images..."
        mage buildContainers || {
            log_error "Container build failed"
            exit 1
        }
        log_info "Container images built successfully"
    fi
    
    cd - > /dev/null
}

# =============================================================================
# Validation Functions
# =============================================================================
validate_prerequisites() {
    log_info "Validating prerequisites"
    
    # Check required commands
    local required_commands=("git")
    
    if [ "$INSTALL_GOLANG" != "true" ]; then
        required_commands+=("go")
    fi
    
    for cmd in "${required_commands[@]}"; do
        check_command "$cmd" || exit 1
    done
    
    log_info "Prerequisites validation completed"
}

validate_environment() {
    log_info "Validating environment"
    
    # Validate Go version if not installing
    if [ "$INSTALL_GOLANG" != "true" ] && command -v go &> /dev/null; then
        local go_version=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
        log_info "Found Go version: $go_version"
    fi
    
    log_info "Environment validation completed"
}

# =============================================================================
# Main Execution
# =============================================================================
main() {
    log_info "Starting Symphony build process"
    log_info "Configuration:"
    log_info "  - PULL_REPO_FROM_GIT: $PULL_REPO_FROM_GIT"
    log_info "  - GIT_REPO_URL: $GIT_REPO_URL"
    log_info "  - PATH_TO_REPO_CODE_ON_DISK: $PATH_TO_REPO_CODE_ON_DISK"
    log_info "  - BUILD_CLI: $BUILD_CLI"
    log_info "  - BUILD_API: $BUILD_API"
    
    # Set up error handling
    trap cleanup_on_exit EXIT
    
		# uninstall_docker
		
		# Setup
		install_dockerbuildx
    install_golang
    install_mage
		
    # Validation
    validate_prerequisites
    validate_environment

    # Repository setup
    setup_repository
    copy_custom_code
    
    # Build process
    setup_go_modules
    build_components

		# ./buildx build --no-cache --platform linux/amd64 -t mjlatest . -f api/Dockerfile
		# docker run --rm -it -v ./api:/configs -e CONFIG=symphony-api-no-k8s.json mjlatest:latest
    
    log_info "Build process completed successfully!"
}

# Execute main function
main "$@"
