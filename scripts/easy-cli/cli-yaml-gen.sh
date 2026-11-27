#!/bin/bash
# YAML generation functions for WFM CLI

# Deployment profile technology keys (per standard/snapshot.spec.yaml appDeploymentProfile)
DEPLOYMENT_KEY_COMPOSE="${DEPLOYMENT_KEY_COMPOSE:-margo.org/deployment/compose}"
DEPLOYMENT_KEY_HELM="${DEPLOYMENT_KEY_HELM:-margo.org/deployment/helm}"

# Default component properties
DEFAULT_TIMEOUT="${DEFAULT_TIMEOUT:-5m0s}"
DEFAULT_HELM_REVISION="${DEFAULT_HELM_REVISION:-0.1.0}"
DEFAULT_COMPOSE_REVISION="${DEFAULT_COMPOSE_REVISION:-1.0.0}"

generate_instance_yaml_from_oci() {
  local package_name="$1"
  local package_id="$2"
  local device_id="$3"
  local output_file="$4"

  local harbor_url="${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}"
  local temp_dir=$(mktemp -d)

  cd "$temp_dir"

  if ! oras pull "${harbor_url}/${OCI_ORGANIZATION}/${package_name}:latest" \
      -u "${REGISTRY_USER}:${REGISTRY_PASS}" >/dev/null 2>&1; then
    echo "❌ Failed to pull package from OCI" >&2
    cd - >/dev/null
    rm -rf "$temp_dir"
    return 1
  fi

  if [ ! -f "margo.yaml" ]; then
    echo "❌ margo.yaml not found in package" >&2
    cd - >/dev/null
    rm -rf "$temp_dir"
    return 1
  fi

  # Extract metadata
  local app_id=$(grep -E "^\s*id:" margo.yaml | head -1 | sed 's/.*id:\s*//' | tr -d '"' | tr -d "'" | xargs)
  local app_name=$(grep -E "^\s*name:" margo.yaml | head -1 | sed 's/.*name:\s*//' | tr -d '"' | tr -d "'" | xargs)

  local app_identifier="${app_id:-$(echo "${app_name}" | tr '[:upper:]' '[:lower:]' | tr ' ' '-' | tr -d '_.,' | sed 's/--*/-/g' | sed 's/^-//;s/-$//')}"
  app_identifier=$(echo "$app_identifier" | cut -c1-40)

  # Determine deployment technology key
  local raw_key=$(awk '/deploymentProfiles:/,0 {if (/^[[:space:]]*-[[:space:]]*key:/) print}' margo.yaml | head -1 | sed 's/.*key:[[:space:]]*//' | tr -d '"' | tr -d "'" | xargs)

  local profile_key=""
  if [ "$raw_key" = "$DEPLOYMENT_KEY_COMPOSE" ]; then
    profile_key="$DEPLOYMENT_KEY_COMPOSE"
  elif [ "$raw_key" = "$DEPLOYMENT_KEY_HELM" ]; then
    profile_key="$DEPLOYMENT_KEY_HELM"
  else
    echo "❌ Unsupported or missing deployment key in margo.yaml: '$raw_key'" >&2
    cd - >/dev/null
    rm -rf "$temp_dir"
    return 1
  fi

  local repository=$(get_oci_repository_path "$package_name" "$temp_dir/margo.yaml")

  generate_instance "$app_identifier" "$package_id" "$device_id" "$repository" "$output_file" "$temp_dir/margo.yaml" "$profile_key"
  local gen_status=$?

  cd - >/dev/null
  rm -rf "$temp_dir"
  return $gen_status
}

generate_instance() {
  local app_identifier="$1"
  local package_id="$2"
  local device_id="$3"
  local repository="$4"
  local output_file="$5"
  local margo_file="$6"
  local profile_key="$7"  # "margo.org/deployment/helm" or "margo.org/deployment/compose"

  local instance_name=$(echo "${app_identifier}-instance" | cut -c1-53)
  local component_name=$(echo "${app_identifier}" | cut -c1-40)
  local default_rev
  if [[ "$profile_key" == *"helm"* ]]; then
    default_rev="$DEFAULT_HELM_REVISION"
  else
    default_rev="$DEFAULT_COMPOSE_REVISION"
  fi

  cat > "$output_file" <<EOF
# This is an input template allowing the WFM user to modify deployment instance specific parameters(currently read-only).
# This file is not MARGO specified, however these parameters will be used to create the MARGO ApplicationDeployment

apiVersion: non-margo.org
metadata:
  name: ${instance_name}
spec:
  appPackageRef:
    id: ${package_id}
  deviceRef:
    id: ${device_id}
  deploymentProfile:
    key: ${profile_key}
    components:
EOF

  if grep -q "components:" "$margo_file"; then
    awk '/components:/,/^[^ ]/ {
      if (/- name:/) {
        name = $0
        sub(/.*name:/, "", name)
        gsub(/^[ \t]+|[ \t]+$/, "", name)
        gsub(/"/, "", name)
        print "COMPONENT_NAME:" name
      }
      if (/repository:/ && !/registryUrl/) {
        repo = $0
        sub(/.*repository:/, "", repo)
        sub(/#.*$/, "", repo)
        gsub(/^[ \t]+|[ \t]+$/, "", repo)
        gsub(/"/, "", repo)
        print "REPOSITORY:" repo
      }
      if (/revision:/) {
        rev = $0
        sub(/.*revision:/, "", rev)
        sub(/#.*$/, "", rev)
        gsub(/^[ \t]+|[ \t]+$/, "", rev)
        gsub(/"/, "", rev)
        print "REVISION:" rev
      }
      if (/timeout:/) {
        tout = $0
        sub(/.*timeout:/, "", tout)
        sub(/#.*$/, "", tout)
        gsub(/^[ \t]+|[ \t]+$/, "", tout)
        gsub(/"/, "", tout)
        print "TIMEOUT:" tout
      }
      if (/wait:/) {
        w = $0
        sub(/.*wait:/, "", w)
        sub(/#.*$/, "", w)
        gsub(/^[ \t]+|[ \t]+$/, "", w)
        gsub(/"/, "", w)
        print "WAIT:" w
      }
    }' "$margo_file" | {
      local current_name=""
      local current_repo=""
      local current_rev="$default_rev"
      local current_wait="true"
      local current_timeout="$DEFAULT_TIMEOUT"

      flush_component() {
        if [ -n "$current_name" ]; then
          if [[ "$current_repo" == *"{{"* ]] || [ -z "$current_repo" ]; then
            current_repo="$repository"
          fi
          if [[ "$current_rev" == *"{{"* ]] || [ -z "$current_rev" ]; then
            current_rev="$default_rev"
          fi
          if [[ "$current_timeout" =~ ^[0-9]+m$ ]]; then
            current_timeout="${current_timeout}0s"
          elif [[ ! "$current_timeout" =~ ^[0-9]+m[0-9]+s$ ]]; then
            current_timeout="$DEFAULT_TIMEOUT"
          fi
          cat >> "$output_file" <<COMPONENT
      - name: ${current_name}
        properties:
          repository: ${current_repo}
          revision: ${current_rev}
          wait: ${current_wait}
          timeout: ${current_timeout}
COMPONENT
          current_name=""
          current_repo=""
          current_rev="$default_rev"
          current_wait="true"
          current_timeout="$DEFAULT_TIMEOUT"
        fi
      }

      while IFS=: read -r key value; do
        case "$key" in
          COMPONENT_NAME)
            flush_component
            current_name="$value"
            ;;
          REPOSITORY)
            current_repo="$value"
            ;;
          REVISION)
            current_rev="$value"
            ;;
          TIMEOUT)
            current_timeout="$value"
            ;;
          WAIT)
            current_wait="$value"
            ;;
        esac
      done
      flush_component
    }
  else
    local fallback_rev="$default_rev"
    if [[ "$profile_key" == *"helm"* ]]; then
      local version_field
      version_field=$(grep -E "^\s*version:" "$margo_file" | head -1 \
        | sed 's/.*version:\s*//' | tr -d '"' | tr -d "'" | xargs)
      [ -n "$version_field" ] && fallback_rev="$version_field"
    fi

    cat >> "$output_file" <<COMPONENT
      - name: ${component_name}
        properties:
          repository: ${repository}
          revision: ${fallback_rev}
          wait: true
          timeout: ${DEFAULT_TIMEOUT}
COMPONENT
  fi
}

generate_wfm_package_yaml() {
  local package_repo="$1"
  local output_file="$2"
  local harbor_url="${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}"

  cat > "$output_file" <<EOF
# This is an input template allowing the WFM user to modify deployment instance specific parameters.
# This file is not MARGO specified, however these parameters will be used to create the MARGO ApplicationDeployment
apiVersion: non-margo.org
metadata:
  name: ${package_repo}
  labels:
    env: dev
  annotations:
    description: "Application package from Harbor OCI Registry"
spec:
  sourceType: OCI_REPO
  source:
    registryUrl: "https://${harbor_url}"
    repository: "${OCI_ORGANIZATION}/${package_repo}"
    tag: "latest"
    authentication:
      type: "basic"
      username: "${REGISTRY_USER}"
      password: "${REGISTRY_PASS}"
EOF
}


# Legacy template-based functions (for backward compatibility)
get_instance_file_path() {
  local package_name="$1"

  if [ -z "$HOME" ]; then
    echo "❌ HOME environment variable not set" >&2
    return 1
  fi

  local template_base="$HOME/symphony/cli/templates/margo"

  case $package_name in
    "custom-otel-helm-app-package"|"custom-otel-helm-app"|"custom-otel"|"otel-demo-pkg")
      original_file_path="$template_base/custom-otel-helm/instance.yaml"
      file_path="$template_base/custom-otel-helm/instance.yaml.copy"
      ;;
    "nextcloud-compose-app-package"|"nextcloud-compose-app"|"nextcloud"|"nextcloud-pkg")
      original_file_path="$template_base/nextcloud-compose/instance.yaml"
      file_path="$template_base/nextcloud-compose/instance.yaml.copy"
      ;;
    *)
      local search_name="${package_name%-app-package}"
      local template_dir=$(find "$template_base" -maxdepth 1 -type d -iname "*${search_name}*" 2>/dev/null | head -1)

      if [ -n "$template_dir" ] && [ -f "$template_dir/instance.yaml" ]; then
        original_file_path="$template_dir/instance.yaml"
        file_path="$template_dir/instance.yaml.copy"
      else
        echo "❌ No instance template found for package '$package_name'" >&2
        echo "ℹ️  Searched in: $template_base" >&2
        return 1
      fi
      ;;
  esac

  if [ -f "$original_file_path" ]; then
    cp -f "$original_file_path" "$file_path"
    echo "$file_path"
  else
    echo "❌ Deployment file not found: $original_file_path" >&2
    return 1
  fi
}
