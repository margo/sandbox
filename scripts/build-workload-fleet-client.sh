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
ORG="${GITHUB_ORG:-margo}"   # from workflow or default
IMAGE_NAME="margo.org/workload-fleet-management-client"
IMAGE_REPO="${REGISTRY}/${ORG}/${IMAGE_NAME}"
DOCKERFILE_PATH="poc/device/agent/Dockerfile"

GIT_SHA="$(git rev-parse --short HEAD 2>/dev/null || echo local)"
TAGS=(
  "${IMAGE_REPO}:latest"
  "${IMAGE_REPO}:${GIT_SHA}"
)

echo "---------------------------------------------"
echo "Image repo : ${IMAGE_REPO}"
echo "Tags       : ${TAGS[*]}"
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
  if ! docker system info 2>/dev/null | grep -q ghcr.io; then
    echo "❌ Not logged in to GHCR"
    echo "👉 Run once:"
    echo "   docker login ghcr.io"
    exit 1
  fi
  echo "✅ Existing GHCR login detected"
fi

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
  --push \
  .

  
echo "✅ Image pushed successfully"
echo "docker pull ${IMAGE_REPO}:latest"
