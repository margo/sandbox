# Stage 1: Build
FROM golang:1.24.3 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build a static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o device-agent ./poc/device/agent

# Stage 2: Runtime
FROM scratch

WORKDIR /app

COPY --from=builder /app/device-agent .
COPY --from=builder /app/poc/device/agent/config ./config

EXPOSE 8080

CMD ["./device-agent"]