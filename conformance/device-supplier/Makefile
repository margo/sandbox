.PHONY: help build-server build-tests build clean test demo run-tests run-server kill-server

help:
	@echo "Device Supplier Conformance Suite - Make Commands"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build-server     Build mock WFM server"
	@echo "  build-tests      Build test runner"
	@echo "  build            Build both server and test runner"
	@echo "  run-server       Start mock WFM server (background)"
	@echo "  run-tests        Run conformance tests"
	@echo "  demo             Full demo: start server, run tests, show report"
	@echo "  kill-server      Stop the running server"
	@echo "  clean            Remove all build artifacts"
	@echo ""

# Build targets
build-server:
	@echo "📦 Building mock WFM server..."
	@mkdir -p bin
	@go build -o bin/server server.go
	@echo "✅ Server built: bin/server"

build-tests:
	@echo "📦 Building test runner..."
	@mkdir -p bin
	@go build -o bin/run_tests run_tests.go
	@echo "✅ Test runner built: bin/run_tests"

build: build-server build-tests
	@echo "✅ All binaries built successfully"

# Run targets
run-server: build-server
	@echo "🚀 Starting Mock WFM Server..."
	@./bin/server

run-tests: build-tests
	@echo "🧪 Running conformance tests..."
	@./bin/run_tests

demo: build
	@echo ""
	@echo "╔════════════════════════════════════════════════════════════════════════════════╗"
	@echo "║                   DEMO: Device Supplier Conformance Suite                     ║"
	@echo "║                                                                              ║"
	@echo "║  This demonstrates a data-driven conformance testing framework for Margo     ║"
	@echo "║  No code changes needed to add test cases - just edit JSON files!            ║"
	@echo "╚════════════════════════════════════════════════════════════════════════════════╝"
	@echo ""
	@echo "📋 Step 1: Check server is ready..."
	@sleep 2
	@echo "📋 Step 2: Starting mock WFM server in background..."
	@./bin/server &
	@sleep 2
	@echo "📋 Step 3: Running conformance tests..."
	@./bin/run_tests
	@echo ""
	@echo "✅ Demo complete! Check reports/ folder for HTML report"
	@echo ""

kill-server:
	@echo "⛔ Stopping server..."
	@pkill -f "bin/server" || echo "Server not running"
	@echo "✅ Server stopped"

clean:
	@echo "🧹 Cleaning up..."
	@rm -rf bin/
	@rm -f go.sum
	@echo "✅ Clean complete"

dev-setup:
	@echo "📋 Developer Setup"
	@echo "Installing Go dependencies..."
	@go mod tidy
	@echo "✅ Ready for development"
