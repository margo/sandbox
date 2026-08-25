#!/bin/bash
# modules/packages.sh - OCI package management

source "$(dirname "${BASH_SOURCE[0]}")/../lib/common.sh"

push_nextcloud_to_oci() {
  echo "📦 Pushing Nextcloud application package to OCI Registry (HTTPS)..."

  local app_dir="$HOME/sandbox/poc/tests/artefacts/nextcloud-compose/margo-package"
  local repository="${OCI_ORGANIZATION}/nextcloud-compose-app-package"
  local tag="latest"

  cd "$app_dir" || { echo "❌ Nextcloud package dir missing"; return 1; }

  echo "$REGISTRY_PASS" | oras login "${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}" \
    -u "$REGISTRY_USER" --password-stdin 

  if [ ! -f "margo.yaml" ]; then
    echo "❌ margo.yaml not found in $app_dir"
    return 1
  fi

  NEXTCLOUD_COMPOSE_REPOSITORY="oci://${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}/library/nextcloud-compose-archive"
  NEXTCLOUD_COMPOSE_REVISION="1.0.0"
  compose_deploy_file="$HOME/sandbox/poc/tests/artefacts/nextcloud-compose/margo-package/margo.yaml"

  echo "Updating compose margo.yaml with repository and revision..."
  sed -i "s|{{NEXTCLOUD_COMPOSE_REPOSITORY}}|$NEXTCLOUD_COMPOSE_REPOSITORY|g" "$compose_deploy_file" 2>/dev/null || true
  sed -i "s|{{NEXTCLOUD_COMPOSE_REVISION}}|$NEXTCLOUD_COMPOSE_REVISION|g" "$compose_deploy_file" 2>/dev/null || true

  local files=("margo.yaml:application/vnd.margo.app.description.v1+yaml")

  if [ -d "resources" ] && [ "$(ls -A resources 2>/dev/null)" ]; then
    while IFS= read -r file; do
      if [ -f "$file" ]; then
        files+=("$file:application/octet-stream")
      fi
    done < <(find resources -type f 2>/dev/null)
  fi

  echo "Pushing files: ${files[@]}"
  oras push "${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}/${repository}:${tag}" \
    --artifact-type "application/vnd.margo.app.v1+json" \
    "${files[@]}"

  if [ $? -eq 0 ]; then
    echo "✅ Nextcloud package pushed to OCI Registry (HTTPS)"
    echo "📍 Location: https://${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}/${repository}:${tag}"
  else
    echo "❌ Failed to push Nextcloud package"
    return 1
  fi
}

push_custom_otel_to_oci() {
  echo "📦 Pushing Custom OTEL application package to OCI Registry (HTTPS)..."

  local app_dir="$HOME/sandbox/poc/tests/artefacts/custom-otel-helm-app/margo-package"
  local repository="${OCI_ORGANIZATION}/custom-otel-helm-app-package"
  local tag="latest"

  cd "$app_dir" || { echo "❌ Custom OTEL package dir missing"; return 1; }

  echo "$REGISTRY_PASS" | oras login "${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}" \
    -u "$REGISTRY_USER" --password-stdin 

  if [ ! -f "margo.yaml" ]; then
    echo "❌ margo.yaml not found in $app_dir"
    return 1
  fi

  local files=("margo.yaml:application/vnd.margo.app.description.v1+yaml")

  if [ -d "resources" ] && [ "$(ls -A resources 2>/dev/null)" ]; then
    while IFS= read -r file; do
      if [ -f "$file" ]; then
        files+=("$file:application/octet-stream")
      fi
    done < <(find resources -type f 2>/dev/null)
  fi

  echo "Pushing files: ${files[@]}"
  oras push "${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}/${repository}:${tag}" \
    --artifact-type "application/vnd.margo.app.v1+json" \
    "${files[@]}"

  if [ $? -eq 0 ]; then
    echo "✅ Custom OTEL package pushed to OCI Registry (HTTPS)"
    echo "📍 Location: https://${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}/${repository}:${tag}"
  else
    echo "❌ Failed to push Custom OTEL package"
    return 1
  fi
}

build_custom_otel_container_images() {
  echo "Building/Downloading Custom Otel images..."

  cd "$HOME/sandbox/poc/tests/artefacts/custom-otel-helm-app/code/app"
  docker build . -t "${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}/library/custom-otel-app:latest"

  echo "Ensuring Harbor registry login (HTTPS)..."
  docker login "${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}" -u admin -p Harbor12345
  echo "Pushing otel images to Harbor..."
  docker push "${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}/library/custom-otel-app:latest"

  OTEL_APP_CONTAINER_URL="${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}/library/custom-otel-app"
  deploy_file="$HOME/sandbox/poc/tests/artefacts/custom-otel-helm-app/code/helm/values.yaml"
  tag="latest"

  echo "Preparing Helm chart..."
  cd "$HOME/sandbox/poc/tests/artefacts/custom-otel-helm-app/code"
  CHART_FILE="$HOME/sandbox/poc/tests/artefacts/custom-otel-helm-app/code/helm/Chart.yaml"
  CHART_VERSION=$(grep "^version:" "$CHART_FILE" | awk '{print $2}')

  echo "Using existing chart version: $CHART_VERSION"

  sed -i "s|{{REPOSITORY}}|$OTEL_APP_CONTAINER_URL|g" "$deploy_file" 2>/dev/null || true
  sed -i "s|{{TAG}}|$tag|g" "$deploy_file" 2>/dev/null || true
  echo "Packaging Helm chart version $CHART_VERSION..."
  helm package helm/

  echo "Pushing chart to Harbor (HTTPS)..."
  helm push "custom-otel-helm-${CHART_VERSION}.tgz" "oci://${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}/library" 


  HELM_REPOSITORY="oci://${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}/library/custom-otel-helm"
  HELM_REVISION="$CHART_VERSION"
  helm_deploy_file="$HOME/sandbox/poc/tests/artefacts/custom-otel-helm-app/margo-package/margo.yaml"

  echo "Updating margo.yaml with chart version $CHART_VERSION..."
  sed -i "s|{{HELM_REPOSITORY}}|$HELM_REPOSITORY|g" "$helm_deploy_file" 2>/dev/null || true
  sed -i "s|{{HELM_REVISION}}|$HELM_REVISION|g" "$helm_deploy_file" 2>/dev/null || true

  echo "✅ Custom otel chart version $CHART_VERSION successfully pushed to Harbor (HTTPS)"
  echo "📦 Chart: oci://${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}/library/custom-otel-helm:$CHART_VERSION"
}

push_nextcloud_compose_to_oci() {
  echo "📦 Pushing Nextcloud Compose Archive to OCI Registry (HTTPS)..."

  local app_dir="$HOME/sandbox/poc/tests/artefacts/nextcloud-compose/code/compose"
  local repository="${OCI_ORGANIZATION}/nextcloud-compose-archive"

  NEXTCLOUD_COMPOSE_REPOSITORY="oci://${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}/library/nextcloud-compose-archive"
  NEXTCLOUD_COMPOSE_REVISION="1.0.0"

  local tag=${NEXTCLOUD_COMPOSE_REVISION:-1.0.0}
  local archive_name="nextcloud-compose-archive-${tag}.tar.gz"
  local archive_path="/tmp/${archive_name}"

  cd "$app_dir" || {
    echo "❌ Nextcloud package dir missing: $app_dir"
    return 1
  }

  echo "$REGISTRY_PASS" | oras login \
    "${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}" \
    -u "$REGISTRY_USER" \
    --password-stdin 

  if [ ! -f "compose.yaml" ]; then
    echo "❌ compose.yaml not found in $app_dir"
    return 1
  fi

  echo "📦 Creating Compose Archive..."

  # Stage files at archive root (no wrapping top-level directory)
  local staging_dir
  staging_dir=$(mktemp -d)

  # Ensure cleanup on function exit
  trap 'rm -rf "$staging_dir" "$archive_path"' RETURN

  # Copy compose.yaml
  cp "compose.yaml" "$staging_dir/compose.yaml"

  # Copy additional resources if present
  if [ -d "resources" ]; then
    cp -a resources "$staging_dir/"
  fi

  # Create archive in /tmp
  tar -czf "$archive_path" \
    -C "$staging_dir" \
    .

  if [ ! -f "$archive_path" ]; then
    echo "❌ Failed to create Compose Archive"
    return 1
  fi

  echo "📦 Compose Archive created: $archive_path"
  echo "📤 Pushing Compose Archive to OCI Registry..."

  # ORAS requires a relative filename, so push from /tmp
  (
    cd /tmp || exit 1

    oras push \
      "${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}/${repository}:${tag}" \
      --artifact-type "application/vnd.org.margo.component.compose+json" \
      "${archive_name}:application/vnd.org.margo.component.compose.tar+gzip"
  )

  if [ $? -eq 0 ]; then
    echo "✅ Nextcloud Compose Archive pushed successfully"
    echo "📍 OCI: oci://${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}/${repository}:${tag}"
  else
    echo "❌ Failed to push Nextcloud Compose Archive"
    return 1
  fi
}
