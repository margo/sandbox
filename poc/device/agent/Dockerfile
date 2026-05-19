# Stage 1: Build
FROM golang:1.25.10 AS builder

ARG VERSION
ARG COMMIT
ARG DATE

ARG TARGETOS
ARG TARGETARCH


WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build a static binary
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -trimpath \
    -ldflags="-X main.build=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
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

# Use ENTRYPOINT for better flexibility
ENTRYPOINT ["./device-agent"]
CMD ["-config", "config/config.yaml"]
