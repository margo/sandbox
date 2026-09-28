#!/usr/bin/env bash
set -Eeuo pipefail

# --------------------------------------------------
# Resolve repo root
# --------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

# --------------------------------------------------
# Image configuration
# --------------------------------------------------
REGISTRY="ghcr.io"
ORG="${GITHUB_ORG:-margo}"
IMAGE_NAME="margo-identity-service"
IMAGE_REPO="${REGISTRY}/${ORG}/${IMAGE_NAME}"
DOCKERFILE_PATH="mis/Dockerfile"

GIT_SHA="$(git rev-parse --short HEAD 2>/dev/null || echo local)"
TAGS=(
  "${IMAGE_REPO}:latest"
  "${IMAGE_REPO}:${GIT_SHA}"
)

echo "---------------------------------------------"
echo "Image repo : ${IMAGE_REPO}"
echo "Tags       : ${TAGS[*]}"
echo "Dockerfile : ${DOCKERFILE_PATH}"
echo "---------------------------------------------"

# --------------------------------------------------
# Authentication (smart & minimal)
# --------------------------------------------------
if [[ -n "${GITHUB_ACTIONS:-}" ]]; then
  echo "🔐 Running inside GitHub Actions — logging into GHCR"
  echo "${GITHUB_TOKEN}" | docker login ghcr.io \
    -u "${GITHUB_ACTOR}" \
    --password-stdin
else
  echo "🔎 Running locally — checking GHCR login"
  GHCR_LOGGED_IN=false
  for config in "$HOME/.docker/config.json" "/root/.docker/config.json"; do
    if grep -q "ghcr.io" "$config" 2>/dev/null; then
      GHCR_LOGGED_IN=true
      break
    fi
  done

  if [[ "$GHCR_LOGGED_IN" == "false" ]]; then
    echo "❌ Not logged in to GHCR"
    echo "👉 Run once:"
    echo "   docker login ghcr.io"
    exit 1
  fi
  echo "✅ Existing GHCR login detected"
fi

# --------------------------------------------------
# Setup QEMU for multi-platform builds
# --------------------------------------------------
echo "🔧 Setting up QEMU for multi-platform builds..."
docker run --privileged --rm tonistiigi/binfmt --install all
echo "✅ QEMU setup complete"

# --------------------------------------------------
# Ensure buildx builder with multi-platform support
# --------------------------------------------------
if ! docker buildx inspect mis-builder &>/dev/null; then
  echo "🔧 Creating buildx builder: mis-builder"
  docker buildx create --name mis-builder --driver docker-container --use
else
  echo "⚡️ Using existing buildx builder: mis-builder"
  docker buildx use mis-builder
fi
docker buildx inspect --bootstrap

# --------------------------------------------------
# Build image
# --------------------------------------------------
echo "Platforms: linux/amd64, linux/arm64"

docker buildx build \
  -f "${DOCKERFILE_PATH}" \
  --platform linux/amd64,linux/arm64 \
  $(printf -- "-t %s " "${TAGS[@]}") \
  --cache-from=type=gha \
  --cache-to=type=gha,mode=max \
  --provenance=false \
  --push \
  .

echo "✅ MIS image pushed successfully"
echo "docker pull ${IMAGE_REPO}:latest"
