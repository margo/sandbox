.PHONY: help build-server build-tests build clean test demo run-tests run-custom-tests run-server kill-server

help:
	@echo "Device Supplier Conformance Suite - Make Commands"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build-server          Build mock WFM server"
	@echo "  build-tests           Build test runner"
	@echo "  build                 Build both server and test runner"
	@echo "  run-server            Start mock WFM server (background)"
	@echo "  run-tests             Run default conformance tests (device-scenarios/test-scenarios.json)"
	@echo "  run-custom-tests      Run vendor scenarios: make run-custom-tests FILE=device-scenarios/my-file.json"
	@echo "  demo                  Full demo: start server, run tests, show report"
	@echo "  kill-server           Stop the running server"
	@echo "  clean                 Remove all build artifacts"
	@echo ""

# Build targets
build-server:
	@echo "📦 Preparing mock WFM server..."
	@mkdir -p bin
	@if [ ! -f bin/server ]; then \
		echo "Building from source..."; \
		go build -o bin/server ./cmd/device-supplier; \
	else \
		echo "✅ Server binary already exists: bin/server"; \
	fi

build-tests:
	@echo "📦 Preparing test runner..."
	@mkdir -p bin
	@if [ ! -f bin/run_tests ]; then \
		echo "Building from source..."; \
		go build -o bin/run_tests run_tests.go; \
	else \
		echo "✅ Test runner binary already exists: bin/run_tests"; \
	fi

build: build-server build-tests
	@echo "✅ All binaries built successfully"

# Run targets
run-server: build-server
	@echo "🚀 Starting Mock WFM Server..."
	@./bin/server > /tmp/wfm-server.log 2>&1 & \
	echo $$! > /tmp/wfm-server.pid; \
	sleep 1; \
	echo "✅ Server started (PID: $$(cat /tmp/wfm-server.pid))"

run-tests: build-tests
	@echo "🧪 Running conformance tests..."
	@./bin/run_tests

FILE ?= device-scenarios/test-scenarios.json
run-custom-tests: build-tests
	@echo "🧪 Running vendor scenarios from: $(FILE)"
	@./bin/run_tests -file $(FILE)

demo: build
	@echo ""
	@echo "╔════════════════════════════════════════════════════════════════════════════════╗"
	@echo "║                   DEMO: Device Supplier Conformance Suite                     ║"
	@echo "║                                                                              ║"
	@echo "║  This demonstrates a data-driven conformance testing framework for Margo     ║"
	@echo "║  No code changes needed to add test cases - just edit JSON files!            ║"
	@echo "╚════════════════════════════════════════════════════════════════════════════════╝"
	@echo ""
	@echo "📋 Step 1: Starting mock WFM server in background..."
	@make run-server
	@echo "📋 Step 2: Running conformance tests..."
	@make run-tests
	@echo ""
	@echo "✅ Demo complete! Check reports/ folder for HTML report"
	@echo "📌 Server is still running. Use 'make kill-server' to stop it"
	@echo ""

kill-server:
	@echo "⛔ Stopping server..."
	@if [ -f /tmp/wfm-server.pid ]; then \
		PID=$$(cat /tmp/wfm-server.pid); \
		if kill -0 $$PID 2>/dev/null; then \
			kill -15 $$PID; \
			sleep 1; \
			echo "✅ Server stopped (PID $$PID)"; \
		else \
			echo "⚠ Server process not running"; \
		fi; \
		rm -f /tmp/wfm-server.pid; \
	else \
		echo "⚠ No server process to stop"; \
	fi

clean:
	@echo "🧹 Cleaning up..."
	@rm -rf bin/
	@rm -f /tmp/wfm-server.pid /tmp/wfm-server.log
	@# Preserve go.sum and go.mod to avoid rebuild issues
	@echo "✅ Clean complete"

dev-setup:
	@echo "📋 Developer Setup"
	@echo "Installing Go dependencies..."
	@go mod tidy
	@echo "✅ Ready for development"
