export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

# Build with all optimizations
go build \
    -ldflags="-s -w -X main.version=$(git describe --tags)" \
    -trimpath \
    -tags production \
    -o agent-optimized

# Compress with UPX
upx --best --lzma agent-optimized

echo "Original size: $(du -h agent | cut -f1)"
echo "Optimized size: $(du -h agent-optimized | cut -f1)"
