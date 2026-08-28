# Stage 1: Build for correct OS Architecture
FROM --platform=$BUILDPLATFORM golang:1.25.10 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build a static binary
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -trimpath \
    -o mis ./mis/main.go

# Stage 3: Runtime
FROM alpine:3.18

# Install CA certificates and other essentials
RUN apk add --no-cache ca-certificates

WORKDIR /

COPY --from=builder /app/mis .

# Use ENTRYPOINT for better flexibility
ENTRYPOINT ["./mis"]
