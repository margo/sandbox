# Stage 1: Build
FROM golang:1.24.3 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download


COPY . .

# Build a static binary

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-X main.version=tobedone" -o device-agent ./poc/device/agent

# Stage 2: Get Docker binary from official image
FROM docker:28.3.3-cli AS docker-source

# Install Docker Compose CLI plugin
RUN apk add --no-cache curl && \
    mkdir -p /usr/local/lib/docker/cli-plugins && \
    DOCKER_COMPOSE_VERSION=$(curl -s https://api.github.com/repos/docker/compose/releases/latest | grep 'tag_name' | cut -d\" -f4) && \
    curl -L "https://github.com/docker/compose/releases/download/${DOCKER_COMPOSE_VERSION}/docker-compose-linux-x86_64" -o /usr/local/lib/docker/cli-plugins/docker-compose && \
    chmod +x /usr/local/lib/docker/cli-plugins/docker-compose

# Stage 3: Runtime
FROM alpine:3.18

# Install CA certificates and other essentials
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /

COPY --from=builder /app/device-agent .
COPY --from=builder /app/poc/device/agent/config ./config
COPY --from=docker-source /usr/local/bin/docker /usr/bin/docker
COPY --from=docker-source /usr/local/lib/docker/cli-plugins/docker-compose /usr/lib/docker/cli-plugins/docker-compose
# Create volume for config override
VOLUME ["/config"]

# Add metadata labels
LABEL description="Margo Device Agent for edge workload management" \
      source="https://github.com/margo/dev-repo" \
      title="Device Agent"

# Use ENTRYPOINT for better flexibility
ENTRYPOINT ["./device-agent"]
CMD ["-config", "config/config.yaml"]
