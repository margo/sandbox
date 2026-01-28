default:
	@just --list

setup-devcontainer:
	@echo "Building devcontainer..."
	@devcontainer build --workspace-folder . 2>&1 || (echo "Build failed. Checking for common issues..." && echo "Try updating devcontainer CLI: npm install -g @devcontainers/cli" && exit 1)

_ensure-devcontainer:
	@devcontainer up --workspace-folder . > /dev/null 2>&1 || true

go-format-fix:
	@just _ensure-devcontainer
	@devcontainer exec --workspace-folder . go fmt ./...

go-vet:
	@just _ensure-devcontainer
	@devcontainer exec --workspace-folder . go vet ./...
