# Stage 1: Build for correct OS Architecture
FROM --platform=$BUILDPLATFORM golang:1.25.10 AS builder

ARG VERSION
ARG COMMIT
ARG DATE

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -trimpath \
    -ldflags="-X main.build=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o device-agent ./poc/device/agent

# Stage 2: Minimal Runtime
FROM alpine:3.18

# Install CA certificates (for TLS/HTTPS) and timezone data
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /

# Copy only your application binary and configs
COPY --from=builder /app/device-agent .
COPY --from=builder /app/poc/device/agent/config ./config

VOLUME ["/config"]

ENTRYPOINT ["./device-agent"]
CMD ["-config", "config/config.yaml"]