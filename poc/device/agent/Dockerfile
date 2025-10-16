# Stage 1: Build Go binary
FROM golang:1.24.3 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-X main.version=tobedone" -o device-agent ./poc/device/agent

# Stage 2: Runtime with Docker CLI + Compose
FROM docker:28.3.3-cli AS runtime

WORKDIR /

# Copy the Go binary and config
COPY --from=builder /app/device-agent .
COPY --from=builder /app/poc/device/agent/config ./config

# Create volume for config override
VOLUME ["/config"]

# Add metadata labels
LABEL description="Margo Device Agent for edge workload management" \
      source="https://github.com/margo/dev-repo" \
      title="Device Agent"

# Verify docker CLI and Compose plugin (optional)
RUN docker --version && docker compose version || echo "Docker CLI or Compose plugin not found"

# Use ENTRYPOINT for better flexibility
ENTRYPOINT ["./device-agent"]
CMD ["-config", "config/config.yaml"]
