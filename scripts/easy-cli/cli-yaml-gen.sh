#!/bin/bash
# YAML generation functions for WFM CLI

generate_instance_yaml_from_oci() {
  local package_name="$1"
  local package_id="$2"
  local device_id="$3"
  local output_file="$4"
  
  local harbor_url="${EXPOSED_HARBOR_HOST}:${EXPOSED_HARBOR_PORT}"
  local temp_dir=$(mktemp -d)
  
  cd "$temp_dir"
  
  if ! oras pull "${harbor_url}/${OCI_ORGANIZATION}/${package_name}:latest" \
      --plain-http \
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
  
  # Determine deployment type
  local deployment_type=$(awk '/deploymentProfile:/,/^[^ ]/ {if (/^\s+type:/) print}' margo.yaml | sed 's/.*type:\s*//' | tr -d '"' | tr -d "'" | xargs | head -1)
  
  if [ -z "$deployment_type" ]; then
    deployment_type=$(awk '/^spec:/,/^[^ ]/ {if (/^\s+type:/) print}' margo.yaml | sed 's/.*type:\s*//' | tr -d '"' | tr -d "'" | xargs | head -1)
  fi
  
  if [ -z "$deployment_type" ]; then
    if [[ "$package_name" =~ compose ]]; then
      deployment_type="compose"
    elif [[ "$package_name" =~ helm ]]; then
      deployment_type="helm.v3"
    fi
  fi
  
  local profile_type=""
  case "$deployment_type" in
    helm|helm.v3) profile_type="helm.v3" ;;
    compose|docker-compose) profile_type="compose" ;;
    *)
      if [[ "$package_name" =~ compose ]]; then
        profile_type="compose"
      else
        profile_type="helm.v3"
      fi
      ;;
  esac
  
  local repository=$(get_oci_repository_path "$package_name" "$temp_dir/margo.yaml")
  
  if [ "$profile_type" = "helm.v3" ]; then
    generate_helm_instance "$app_identifier" "$package_id" "$device_id" "$repository" "$output_file" "$temp_dir/margo.yaml"
  elif [ "$profile_type" = "compose" ]; then
    generate_compose_instance "$app_identifier" "$package_id" "$device_id" "$repository" "$output_file" "$temp_dir/margo.yaml"
  else
    echo "❌ Unsupported deployment type: $profile_type" >&2
    cd - >/dev/null
    rm -rf "$temp_dir"
    return 1
  fi
  
  cd - >/dev/null
  rm -rf "$temp_dir"
  return 0
}

generate_helm_instance() {
  local app_identifier="$1"
  local package_id="$2"
  local device_id="$3"
  local repository="$4"
  local output_file="$5"
  local margo_file="$6"
  
  local instance_name=$(echo "${app_identifier}-instance" | cut -c1-53)
  
  cat > "$output_file" <<EOF
# This is an input template allowing the WFM user to modify deployment instance specific parameters(currently read-only).
# This file is not MARGO specified, however these parameters will be used to create the MARGO ApplicationDeployment

apiVersion: non-margo.org
kind: ApplicationDeployment
metadata:
  name: ${instance_name}
spec:
  appPackageRef:
    id: ${package_id}
  deviceRef:
    id: ${device_id}
  deploymentProfile:
    type: helm.v3
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
        gsub(/^[ \t]+|[ \t]+$/, "", repo)
        gsub(/"/, "", repo)
        print "REPOSITORY:" repo
      }
      if (/revision:/) {
        rev = $0
        sub(/.*revision:/, "", rev)
        gsub(/^[ \t]+|[ \t]+$/, "", rev)
        gsub(/"/, "", rev)
        print "REVISION:" rev
      }
    }' "$margo_file" | {
      local current_name=""
      local current_repo=""
      local current_rev="0.1.0"
      
      while IFS=: read -r key value; do
        case "$key" in
          COMPONENT_NAME)
            current_name="$value"
            ;;
          REPOSITORY)
            current_repo="$value"
            ;;
          REVISION)
            current_rev="$value"
            if [ -n "$current_name" ] && [ -n "$current_repo" ]; then
              cat >> "$output_file" <<COMPONENT
      - name: ${current_name}
        properties:
          repository: ${current_repo}
          revision: ${current_rev}
          wait: true
          timeout: 5m
COMPONENT
              current_name=""
              current_repo=""
              current_rev="0.1.0"
            fi
            ;;
        esac
      done
    }
  else
    local component_name=$(echo "$app_identifier" | cut -c1-40)
    local chart_version=$(grep -E "^\s*version:" "$margo_file" | head -1 | sed 's/.*version:\s*//' | tr -d '"' | tr -d "'" | xargs)
    chart_version="${chart_version:-0.1.0}"
    
    cat >> "$output_file" <<COMPONENT
      - name: ${component_name}
        properties:
          repository: ${repository}
          revision: ${chart_version}
          wait: true
          timeout: 5m
COMPONENT
  fi
  
  if grep -q "parameters:" "$margo_file"; then
    echo "  parameters:" >> "$output_file"
    
    if grep -qi "otel\|otlp" "$margo_file"; then
      cat >> "$output_file" <<EOF
    otlpEndpoint:
      value: "http://otel-collector-opentelemetry-collector.observability:4318"
      targets:
      - pointer: env.OTEL_EXPORTER_OTLP_ENDPOINT
        components: ["${component_name}"]
EOF
    fi
  fi
}

generate_compose_instance() {
  local app_identifier="$1"
  local package_id="$2"
  local device_id="$3"
  local repository="$4"
  local output_file="$5"
  local margo_file="$6"
  
  local instance_name=$(echo "${app_identifier}-instance" | cut -c1-53)
  local stack_name=$(echo "${app_identifier}-stack" | cut -c1-40)
  
  cat > "$output_file" <<EOF
# This is an input template allowing the WFM user to modify deployment instance specific parameters(currently read-only).
# This file is not MARGO specified, however these parameters will be used to create the MARGO ApplicationDeployment
apiVersion: non-margo.org
kind: ApplicationDeployment
metadata:
  name: ${instance_name}
spec:
  appPackageRef:
    id: ${package_id}
  deviceRef:
    id: ${device_id}
  deploymentProfile:
    type: compose
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
      if (/packageLocation:/) {
        location = $0
        sub(/.*packageLocation:/, "", location)
        gsub(/^[ \t]+|[ \t]+$/, "", location)
        gsub(/"/, "", location)
        print "PACKAGE_LOCATION:" location
      }
    }' "$margo_file" | {
      local current_name=""
      while IFS=: read -r key value; do
        case "$key" in
          COMPONENT_NAME)
            current_name="$value"
            ;;
          PACKAGE_LOCATION)
            if [ -n "$current_name" ]; then
              cat >> "$output_file" <<COMPONENT
      - name: ${current_name}
        properties:
          packageLocation: ${value}
COMPONENT
              current_name=""
            fi
            ;;
        esac
      done
    }
  else
    cat >> "$output_file" <<COMPONENT
      - name: ${stack_name}
        properties:
          packageLocation: ${repository}
COMPONENT
  fi
  
  if grep -q "parameters:" "$margo_file"; then
    echo "  parameters:" >> "$output_file"
    
    local default_port=$(grep -E "^\s*port:" "$margo_file" | head -1 | sed 's/.*port:\s*//' | tr -d '"' | tr -d "'" | xargs)
    if [ -n "$default_port" ]; then
      cat >> "$output_file" <<EOF
    servicePort:
      value: ${default_port}
      targets:
        - pointer: PORTS.80
          components: ["${stack_name}"]
EOF
    fi
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
kind: ApplicationPackage
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
