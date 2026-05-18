#!/bin/bash
# Harbor OCI registry operations for WFM CLI

discover_app_packages_from_harbor() {
  echo "🔍 Discovering app packages from Harbor OCI Registry..." >&2

  local harbor_url="${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}"
  local org="${OCI_ORGANIZATION}"

  # CHANGED: Use HTTPS for Harbor API
  local repos=$(curl -s -k -u "${REGISTRY_USER}:${REGISTRY_PASS}" \
    "https://${harbor_url}/api/v2.0/projects/${org}/repositories" | \
    jq -r '.[].name' 2>/dev/null)

  if [ -z "$repos" ]; then
    echo "❌ No repositories found in Harbor" >&2
    return 1
  fi

  local app_packages=$(echo "$repos" | grep -E "app-package$" | sed "s|${org}/||")

  if [ -z "$app_packages" ]; then
    echo "❌ No app packages found" >&2
    echo "ℹ️  App packages must end with '-app-package' suffix" >&2
    echo "ℹ️  Example: nginx-helm-app-package, wordpress-compose-app-package" >&2
    return 1
  fi

  echo "$app_packages"
}

get_package_metadata_from_oci() {
  local package_repo="$1"
  local harbor_url="${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}"
  local full_repo="${OCI_ORGANIZATION}/${package_repo}"

  local temp_dir=$(mktemp -d)
  cd "$temp_dir"

  oras pull "${harbor_url}/${full_repo}:latest" \
    --insecure \
    -u "${REGISTRY_USER}:${REGISTRY_PASS}" \
    margo.yaml 2>/dev/null

  if [ -f "margo.yaml" ]; then
    local display_name=$(grep -E "^\s*name:" margo.yaml | head -1 | sed 's/.*name:\s*//' | tr -d '"')
    echo "${display_name:-${package_repo}}"
  else
    echo "${package_repo}"
  fi

  cd - >/dev/null
  rm -rf "$temp_dir"
}

get_oci_repository_path() {
  local package_name="$1"
  local margo_file="$2"
  local harbor_url="${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}"

  # Hardcoded mappings for backward compatibility
  case $package_name in
    "custom-otel-helm-app-package"|"custom-otel-helm-app"|"custom-otel"|"otel-demo-pkg")
      echo "oci://${harbor_url}/library/custom-otel-helm"
      return 0
      ;;
    "nextcloud-compose-app-package"|"nextcloud-compose-app"|"nextcloud"|"nextcloud-pkg")
      echo "https://raw.githubusercontent.com/docker/awesome-compose/refs/heads/master/nextcloud-redis-mariadb/compose.yaml"
      return 0
      ;;
  esac

  # Dynamic discovery from margo.yaml
  if [ -f "$margo_file" ]; then
    local compose_location=$(grep "packageLocation:" "$margo_file" | head -1 | sed 's/.*packageLocation:\s*//' | tr -d '"' | tr -d "'" | xargs)
    local helm_repo=$(grep "repository:" "$margo_file" | grep -v "registryUrl" | head -1 | sed 's/.*repository:\s*//' | tr -d '"' | tr -d "'" | xargs)

    if [ -n "$compose_location" ]; then
      echo "$compose_location"
    elif [ -n "$helm_repo" ]; then
      echo "$helm_repo"
    else
      local chart_name="${package_name%-app-package}"
      echo "oci://${harbor_url}/library/${chart_name}"
    fi
  else
    local chart_name="${package_name%-app-package}"
    echo "oci://${harbor_url}/library/${chart_name}"
  fi
}

# Backward compatibility function
get_package_name() {
  local choice="$1"
  case $choice in
    1) echo "Custom OTEL Helm App" ;;
    2) echo "Nextcloud Compose App" ;;
    *)
      local packages=$(discover_app_packages_from_harbor)
      local package_array=($packages)
      local idx=$((choice - 1))
      if [ $idx -lt ${#package_array[@]} ]; then
        get_package_metadata_from_oci "${package_array[$idx]}"
      else
        echo "Unknown Package"
      fi
      ;;
  esac
}
