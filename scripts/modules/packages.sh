#!/bin/bash
# modules/packages.sh - OCI package management

source "$(dirname "${BASH_SOURCE[0]}")/../lib/common.sh"

get_margo_media_type() {
  local file="$1"
  local filename
  filename=$(basename "$file")
  local ext="${filename##*.}"
  ext=$(echo "$ext" | tr '[:upper:]' '[:lower:]')

  local format="$ext"
  case "$ext" in
    md)       format="markdown" ;;
    txt)      format="plain" ;;
    jpg|jpeg) format="jpeg" ;;
    png)      format="png" ;;
    svg)      format="svg" ;;
    pdf)      format="pdf" ;;
    yaml|yml) format="yaml" ;;
    json)     format="json" ;;
  esac

  # Match margo.yaml catalog references if yq is available
  if command -v yq &>/dev/null && [ -f "margo.yaml" ]; then
    local icon desc notes license
    icon=$(yq eval '.metadata.catalog.application.icon // ""' margo.yaml 2>/dev/null)
    desc=$(yq eval '.metadata.catalog.application.descriptionFile // ""' margo.yaml 2>/dev/null)
    notes=$(yq eval '.metadata.catalog.application.releaseNotes // ""' margo.yaml 2>/dev/null)
    license=$(yq eval '.metadata.catalog.application.licenseFile // ""' margo.yaml 2>/dev/null)

    if [ "$file" = "$icon" ] || [ "./$file" = "$icon" ] || [ "$file" = "${icon#./}" ]; then
      echo "application/vnd.margo.app.icon.v1+${format}"; return
    elif [ "$file" = "$desc" ] || [ "./$file" = "$desc" ] || [ "$file" = "${desc#./}" ]; then
      echo "application/vnd.margo.app.descriptionFile.v1+${format}"; return
    elif [ "$file" = "$notes" ] || [ "./$file" = "$notes" ] || [ "$file" = "${notes#./}" ]; then
      echo "application/vnd.margo.app.releaseNotes.v1+${format}"; return
    elif [ "$file" = "$license" ] || [ "./$file" = "$license" ] || [ "$file" = "${license#./}" ]; then
      echo "application/vnd.margo.app.licenseFile.v1+${format}"; return
    fi
  fi

  # Fallback based on filename patterns
  local lower_name
  lower_name=$(echo "$filename" | tr '[:upper:]' '[:lower:]')
  if [[ "$lower_name" =~ (icon|logo) ]]; then
    echo "application/vnd.margo.app.icon.v1+${format}"
  elif [[ "$lower_name" =~ (readme|description) ]]; then
    echo "application/vnd.margo.app.descriptionFile.v1+${format}"
  elif [[ "$lower_name" =~ (release_notes|releasenotes|changelog|release-notes) ]]; then
    echo "application/vnd.margo.app.releaseNotes.v1+${format}"
  elif [[ "$lower_name" =~ (license|licence) ]]; then
    echo "application/vnd.margo.app.licenseFile.v1+${format}"
  else
    echo "application/octet-stream"
  fi
}

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
        local media_type
        media_type=$(get_margo_media_type "$file")
        files+=("$file:${media_type}")
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
        local media_type
        media_type=$(get_margo_media_type "$file")
        files+=("$file:${media_type}")
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

push_nextcloud_compose_to_oci() (
  set -euo pipefail

  local_app_dir="${NEXTCLOUD_COMPOSE_DIR:-$HOME/sandbox/poc/tests/artefacts/nextcloud-compose/code/compose}"
  registry="${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}"
  repository="${OCI_ORGANIZATION}/nextcloud-compose-archive"
  compose_revision="${NEXTCLOUD_COMPOSE_REVISION:-1.0.0}"
  component_name="${NEXTCLOUD_COMPONENT_NAME:-nextcloud-stack}"
  description="${NEXTCLOUD_DESCRIPTION:-Nextcloud Compose Component}"
  author="${MAINTAINER_EMAIL:-Margo Community (info@margo.org)}"

  for tool in oras jq tar; do
    command -v "$tool" >/dev/null || { echo "❌ Required tool missing: $tool"; exit 1; }
  done

  # --- Input Validation (SemVer 2.0 without leading 'v') ---------------------
  semver_re='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$'
  if [[ ! "$compose_revision" =~ $semver_re ]]; then
    echo "❌ Revision '$compose_revision' is not valid SemVer 2.0 (no leading 'v', no '+build')"
    exit 1
  fi

  [ -d "$local_app_dir" ] || { echo "❌ App directory missing: $local_app_dir"; exit 1; }
  [ -f "$local_app_dir/compose.yaml" ] || { echo "❌ compose.yaml not found in $local_app_dir"; exit 1; }

  # --- Staging & Archive Creation --------------------------------------------
  workdir=$(mktemp -d)
  trap 'rm -rf "$workdir"' EXIT

  echo "📦 Packaging Compose component..."
  archive_name="${component_name}-${compose_revision}-compose.tar.gz"
  archive_path="$workdir/$archive_name"

  # Spec: exactly one top-level directory named after the component
  mkdir -p "$workdir/stage/$component_name"
  cp -a "$local_app_dir"/. "$workdir/stage/$component_name"/

  # Create deterministic tarball, excluding version control files
  tar --exclude='.git' --owner=0 --group=0 --numeric-owner \
      -czf "$archive_path" -C "$workdir/stage" "$component_name"

   # --- Annotations: MUST be on the layer descriptor --------------------------
  annotations_file="$workdir/annotations.json"
  jq -n \
    --arg layer "$archive_name" \
    --arg ver "$compose_revision" \
    --arg title "$component_name" \
    --arg authors "$author" \
    --arg desc "$description" \
    '{
      ($layer): {
        "org.margo.component.type": "compose",
        "org.margo.component.version": $ver,
        "org.opencontainers.image.title": $title,
        "org.opencontainers.image.authors": $authors,
        "org.opencontainers.image.description": $desc,
        "org.opencontainers.image.version": $ver
      },
      "$manifest": {
        "org.margo.component.type": "compose",
        "org.margo.component.version": $ver,
        "org.opencontainers.image.version": $ver
      }
    }' > "$annotations_file"

  # --- Login & push ----------------------------------------------------------
  echo "$REGISTRY_PASS" | oras login "$registry" -u "$REGISTRY_USER" --password-stdin \
    || { echo "❌ Failed to authenticate with OCI Registry"; exit 1; }

  echo "📤 Pushing Compose Archive to OCI Registry..."
  # oras needs a relative file name, so push from the work dir
  (
    cd "$workdir" || exit 1
    oras push "${registry}/${repository}:${compose_revision}" \
      --artifact-type "application/vnd.org.margo.component.compose+json" \
      --annotation-file "$annotations_file" \
      "${archive_name}:application/vnd.org.margo.component.compose.tar+gzip"
  ) || { echo "❌ Failed to push Compose Archive"; exit 1; }

  echo "✅ Compose Archive published to oci://${registry}/${repository}:${compose_revision}"

)
