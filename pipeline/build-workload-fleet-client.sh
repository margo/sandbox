#!/usr/bin/env bash
set -Eeuo pipefail
# --------------------------------------------------
# Resolve repo root
# --------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"
# --------------------------------------------------
# Required environment variables
# --------------------------------------------------
: "${GITHUB_ORG:?Set GITHUB_ORG (GitHub org or username)}"
: "${GITHUB_TOKEN:?Set GITHUB_TOKEN (GitHub token)}"
# --------------------------------------------------
# Image configuration
# --------------------------------------------------
IMAGE_NAME="workload-fleet-management-client"
REGISTRY="ghcr.io"
IMAGE_REPO="${REGISTRY}/${GITHUB_ORG}/${IMAGE_NAME}"
DOCKERFILE_PATH="poc/device/agent/Dockerfile"
# Tags
GIT_SHA="$(git rev-parse --short HEAD)"
TAGS=(
 "${IMAGE_REPO}:latest"
 "${IMAGE_REPO}:${GIT_SHA}"
)
echo "---------------------------------------------"
echo "Building and pushing image to GHCR"
echo "Image repo : ${IMAGE_REPO}"
echo "Tags       : ${TAGS[*]}"
echo "---------------------------------------------"
# --------------------------------------------------
# Login to GHCR
# --------------------------------------------------
echo "${GITHUB_TOKEN}" | docker login ghcr.io -u "${GITHUB_ORG}" --password-stdin
# --------------------------------------------------
# Build image
# --------------------------------------------------
echo "Building Docker image..."
docker build \
 -f "${DOCKERFILE_PATH}" \
 $(printf -- "-t %s " "${TAGS[@]}") \
 .
# --------------------------------------------------
# Push image
# --------------------------------------------------
echo "Pushing Docker image to GHCR..."
for tag in "${TAGS[@]}"; do
 docker push "${tag}"
done
echo "✅ Image successfully pushed to GHCR"
