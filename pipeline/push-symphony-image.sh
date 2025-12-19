
#!/usr/bin/env bash
set -Eeuo pipefail

# --------------------------------------------------
# Resolve repo root safely
# --------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

# --------------------------------------------------
# Required environment variables
# --------------------------------------------------
: "${GITHUB_USER:?Set GITHUB_USER}"
: "${GITHUB_TOKEN:?Set GITHUB_TOKEN}"

REGISTRY="ghcr.io"
OWNER="$GITHUB_USER"
IMAGE="symphony-api"
IMAGE_BASE="$REGISTRY/$OWNER/$IMAGE"

DOCKERFILE="$REPO_ROOT/api/Dockerfile"
GIT_SHA="$(git rev-parse --short HEAD)"

info() { echo ":information_source:  $1"; }
ok()   { echo ":white_check_mark: $1"; }

info "Repository root: $REPO_ROOT"
info "Image: $IMAGE_BASE"
info "Commit SHA: $GIT_SHA"

# --------------------------------------------------
# Ensure buildx builder exists
# --------------------------------------------------
docker buildx inspect symphony-builder >/dev/null 2>&1 || \
docker buildx create --name symphony-builder --use

# --------------------------------------------------
# Build & Push (Multi-Arch + Cache)
# --------------------------------------------------
info "Building and pushing multi-arch image with cache..."

docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --push \
  --cache-from type=gha \
  --cache-to type=gha,mode=max \
  --secret id=github_username,env=GITHUB_USER \
  --secret id=github_token,env=GITHUB_TOKEN \
  --tag "$IMAGE_BASE:latest" \
  --tag "$IMAGE_BASE:$GIT_SHA" \
  -f "$DOCKERFILE" \
  "$REPO_ROOT"

ok "Image pushed successfully:"
echo " $IMAGE_BASE:latest"
echo " $IMAGE_BASE:$GIT_SHA"

