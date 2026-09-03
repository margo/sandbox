# ============================================================
# Variables
# ============================================================
SERVICE_NAME    := mis
VERSION         := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.0.1")
COMMIT          := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Directories
ROOT_DIR        := $(shell dirname $(CURDIR))
BUILD_DIR       := $(CURDIR)/build

# Go build settings
GOOS            ?= linux
GOARCH          ?= amd64
CGO_ENABLED     := 0
BINARY          := $(SERVICE_NAME)

# Docker settings
IMAGE_NAME      ?= margo-identity-service
IMAGE_TAG       ?= $(VERSION)
# ============================================================
# Phony Targets
# ============================================================
.PHONY: all build container clean help

all: build

# ============================================================
# Build binary
# ============================================================
build:
	@echo ">>> Building $(SERVICE_NAME) binary..."
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build \
		-trimpath \
		-o $(BINARY) \
		$(CURDIR)/main.go
	@echo ">>> Binary available at: $(BINARY)"

# ============================================================
# Run (This requires certificates to be pre-placed)
# ============================================================
run:
	cp ./pkg/conf/configuration.json .
	go run main.go start --config ./configuration.json
	rm ./configuration.json

# ============================================================
# Docker container
# ============================================================
container:
	@echo ">>> Building Docker image $(IMAGE_NAME):$(IMAGE_TAG)..."
	docker build \
		-f $(CURDIR)/Dockerfile \
		--build-arg TARGETOS=$(GOOS) \
		--build-arg TARGETARCH=$(GOARCH) \
		-t $(IMAGE_NAME):$(IMAGE_TAG) \
		-t $(IMAGE_NAME):latest \
		$(ROOT_DIR)
	@echo ">>> Image built: $(IMAGE_NAME):$(IMAGE_TAG)"

# ============================================================
# Clean
# ============================================================
clean:
	@echo ">>> Cleaning build artifacts..."
	@rm -f $(BINARY)
	@rm -rf $(BUILD_DIR)
	@echo ">>> Done."

# ============================================================
# Help
# ============================================================
help:
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@echo "  build      Build the $(SERVICE_NAME) binary (output: ./$(SERVICE_NAME))"
	@echo "  container  Build Docker image ($(IMAGE_NAME):$(IMAGE_TAG))"
	@echo "  clean      Remove all build artifacts"
	@echo ""
	@echo "Overridable vars: GOOS, GOARCH, IMAGE_NAME, IMAGE_TAG"
	@echo "  Example: make build GOARCH=arm64"
	@echo ""