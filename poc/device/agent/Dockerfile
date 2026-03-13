# Stage 1: Build
FROM golang:1.24.4 AS builder

ARG VERSION
ARG COMMIT

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build a static binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-X main.build=${VERSION} -X main.commit=${COMMIT} -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o device-agent ./poc/device/agent

# Stage 2: Get Docker binary and compose plugin from official image
FROM docker:28.3.3-cli AS docker-source

# Stage 3: Runtime
FROM alpine:3.18

# Install CA certificates and other essentials
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /

COPY --from=builder /app/device-agent .
COPY --from=builder /app/poc/device/agent/config ./config
# REVIEW: we can install the docker and other binaries directly using apk
# this will remove the dependency from docker image 
COPY --from=docker-source /usr/local/bin/docker /usr/bin/docker

# Create the cli-plugins directory FIRST
RUN mkdir -p /usr/local/libexec/docker/cli-plugins

# Copy Docker Compose plugin (check if it exists in the source)
COPY --from=docker-source /usr/local/libexec/docker/cli-plugins/docker-compose /usr/local/libexec/docker/cli-plugins/docker-compose

# Make the plugin executable
RUN chmod +x /usr/local/libexec/docker/cli-plugins/docker-compose

# Create volume for config override
VOLUME ["/config"]

# Add metadata labels
LABEL org.opencontainers.image.title="Device Workload Fleet Management Client" \
      org.opencontainers.image.description="Margo sandbox workload fleet management client for edge workload management" \
      org.opencontainers.image.source="https://github.com/margo/sandbox" \
      org.opencontainers.image.version="${VERSION}"

# Use ENTRYPOINT for better flexibility
ENTRYPOINT ["./device-agent"]
CMD ["-config", "config/config.yaml"]
