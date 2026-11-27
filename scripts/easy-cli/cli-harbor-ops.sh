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

discover_package_tags_from_harbor() {
  local package_repo="$1"
  local harbor_url="${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}"
  local org="${OCI_ORGANIZATION}"

  local tags=$(curl -s -k -u "${REGISTRY_USER}:${REGISTRY_PASS}"     "https://${harbor_url}/api/v2.0/projects/${org}/repositories/${package_repo}/artifacts" 2>/dev/null |     jq -r '.[].tags[]?.name' 2>/dev/null)

  if [ -z "$tags" ]; then
    tags=$(curl -s -k -u "${REGISTRY_USER}:${REGISTRY_PASS}"       "https://${harbor_url}/v2/${org}/${package_repo}/tags/list" 2>/dev/null |       jq -r '.tags[]?' 2>/dev/null)
  fi

  if [ -z "$tags" ] && command -v oras >/dev/null 2>&1; then
    tags=$(oras repo tags --insecure "${harbor_url}/${org}/${package_repo}" 2>/dev/null)
  fi

  if [ -n "$tags" ]; then
    echo "$tags" | grep -v '^null$' | grep -v '^$' | sort -V
  else
    echo "latest"
  fi
}


get_package_metadata_from_oci() {
  local package_repo="$1"
  local tag="${2:-latest}"
  local harbor_url="${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}"
  local full_repo="${OCI_ORGANIZATION}/${package_repo}"

  local temp_dir=$(mktemp -d)
  cd "$temp_dir"

  oras pull "${harbor_url}/${full_repo}:${tag}" \
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
  local org="${OCI_ORGANIZATION:-library}"

  case $package_name in
    *custom-otel*|*otel-demo*)
      echo "oci://${harbor_url}/${org}/custom-otel-helm"
      return 0
      ;;
    *nextcloud*)
      echo "https://raw.githubusercontent.com/docker/awesome-compose/refs/heads/master/nextcloud-redis-mariadb/compose.yaml"
      return 0
      ;;
    *)
      local base="${package_name%-app-package}"
      base="${base%-v[0-9]*}"
      base="${base%:[0-9]*}"
      echo "oci://${harbor_url}/${org}/${base}"
      return 0
      ;;
  esac
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
