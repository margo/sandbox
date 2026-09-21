#!/bin/bash
# modules/wfm/repositories.sh - Git repository cloning

source "$(dirname "${BASH_SOURCE[0]}")/../lib/common.sh" 

clone_symphony_repo() {
  cd "$HOME"
  if ! test -d "$HOME/symphony/.git" ; then
    rm -rf "$HOME/symphony"     
    echo "🔄 Cloning symphony branch: $SYMPHONY_BRANCH"
    if [[ -n "$GITHUB_USER" && -n "$GITHUB_TOKEN" ]]; then 
      git clone --branch "$SYMPHONY_BRANCH" --single-branch --depth 1 \
                "https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/margo/symphony.git" \
                "$HOME/symphony"
    else
      git clone --branch "$SYMPHONY_BRANCH" --single-branch --depth 1 \
            "https://github.com/margo/symphony.git" "$HOME/symphony"
    fi
  fi
  cd "$HOME/symphony"
  echo "✅ symphony repo checkout to branch ${SYMPHONY_BRANCH} done"
}

clone_dev_repo() {
  cd "$HOME"
  if ! test -d "$HOME/sandbox/.git" ; then
    rm -rf "$HOME/sandbox"
    echo "🔄 Cloning sandbox branch: $SANDBOX_REPO_BRANCH"
    if [[ -n "$GITHUB_USER" && -n "$GITHUB_TOKEN" ]]; then 
      git clone --branch "$SANDBOX_REPO_BRANCH" --single-branch --depth 1 \
                "https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/margo/sandbox.git" \
                "$HOME/sandbox"     
    else
      git clone --branch "$SANDBOX_REPO_BRANCH" --single-branch --depth 1 \
            "https://github.com/margo/sandbox.git" "$HOME/sandbox"
    fi
 fi
  cd "$HOME/sandbox"
  echo "✅ sandbox repo checkout to branch ${SANDBOX_REPO_BRANCH} done"
}

update_capabilities_labels() {
    local labels_file="${SCRIPT_DIR}/labels.json"
    local capabilities_file="$HOME/sandbox/poc/device/agent/config/capabilities.json"

    # Check labels.json exists, if not, clean up the labels from configuration.json as well
    if [[ ! -f "$labels_file" ]]; then
        echo "File '$labels_file' not found, skipping label copying to capabilties"
        remove_json_key $capabilities_file
        return 0
    fi

    # Check labels.json is not empty, if not, clean up the labels from configuration.json as well
    if [[ ! -s "$labels_file" ]]; then
        echo "File '$labels_file' is empty. skipping label copying to capabilties"
        remove_json_key $capabilities_file
        return 0
    fi

    # Validate JSON files
    jq empty "$labels_file" >/dev/null 2>&1 || {
        echo "'$labels_file' contains invalid JSON."
        return 1
    }

    jq empty "$capabilities_file" >/dev/null 2>&1 || {
        echo "'$capabilities_file' contains invalid JSON."
        return 1
    }

    # Replace .labels completely with contents of labels.json
    tmp_file=$(mktemp)

    jq --slurpfile labels "$labels_file" \
       '.labels = $labels[0]' \
       "$capabilities_file" > "$tmp_file" \
    && mv "$tmp_file" "$capabilities_file"

    echo "Successfully updated .labels in $capabilities_file"
}

# This will simply remove labels key from configuration.json
remove_json_key() {
    local file="$1"
    local key="labels"

    cp "$file" "$file.bak" || {
        echo "ERROR: Failed to create backup of '$file'"
        return 1
    }

    if jq "del(.${key})" "$file" > "$file.tmp" &&
       mv "$file.tmp" "$file"; then
        rm -f "$file.bak"
        return 0
    fi

    echo "ERROR: Failed to remove key '${key}' from '$file'. Restoring original file."
    mv "$file.bak" "$file" 2>/dev/null
    rm -f "$file.tmp"
    return 1
}

setup_mis_deployment() {
    local MIS_DEPLOYMENT_DIR="$HOME/mis-deployment"
    local SANDBOX_DIR="/tmp/sandbox"
    local MIS_COMPOSE_FILE="$SANDBOX_DIR/mis/docker-compose.yaml"
    local MIS_CONFIG_FILE="$SANDBOX_DIR/mis/pkg/conf/configuration.json"

    # Check MIS deployment directory
    if [[ ! -d "$MIS_DEPLOYMENT_DIR" ]]; then
        echo "❌ MIS deployment folder missing: $MIS_DEPLOYMENT_DIR"
        echo "Please generate Factory Root CAs first..."
        return 1
    fi

    # Clean existing MIS deployment files
    echo "🗑️  Removing existing MIS deployment files..."
    rm -f "$MIS_DEPLOYMENT_DIR/docker-compose.yaml"
    rm -f "$MIS_DEPLOYMENT_DIR/configuration.json"

    # Clean previous clone
    rm -rf "$SANDBOX_DIR"

    # Clone sandbox repository
    echo "🔄 Cloning sandbox branch: $SANDBOX_REPO_BRANCH into $SANDBOX_DIR"

    if [[ -n "$GITHUB_USER" && -n "$GITHUB_TOKEN" ]]; then
        git clone \
            --branch "$SANDBOX_REPO_BRANCH" \
            --single-branch \
            --depth 1 \
            "https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/margo/sandbox.git" \
            "$SANDBOX_DIR"
    else
        git clone \
            --branch "$SANDBOX_REPO_BRANCH" \
            --single-branch \
            --depth 1 \
            "https://github.com/margo/sandbox.git" \
            "$SANDBOX_DIR"
    fi

    # Check clone result
    if [[ $? -ne 0 ]]; then
        echo "❌ Failed to clone sandbox repository."
        rm -rf "$SANDBOX_DIR"
        return 1
    fi

    # Show cloned revision for troubleshooting
    echo "🔖 Cloned sandbox commit:"
    git -C "$SANDBOX_DIR" rev-parse --short HEAD

    # Verify required MIS files
    echo "🔍 Verifying MIS deployment files..."

    if [[ ! -f "$MIS_COMPOSE_FILE" ]]; then
        echo "❌ MIS docker-compose.yaml not found:"
        echo "   $MIS_COMPOSE_FILE"
        echo
        echo "📂 Available files under $SANDBOX_DIR/mis:"
        find "$SANDBOX_DIR/mis" -maxdepth 4 -type f -print 2>/dev/null || true

        rm -rf "$SANDBOX_DIR"
        return 1
    fi

    if [[ ! -f "$MIS_CONFIG_FILE" ]]; then
        echo "❌ MIS configuration.json not found:"
        echo "   $MIS_CONFIG_FILE"
        echo
        echo "📂 Available configuration files:"
        find "$SANDBOX_DIR/mis" -maxdepth 5 -type f -name "*.json" -print 2>/dev/null || true

        rm -rf "$SANDBOX_DIR"
        return 1
    fi

    # Copy required files
    echo "📂 Copying MIS deployment files..."

    if ! cp "$MIS_COMPOSE_FILE" "$MIS_DEPLOYMENT_DIR/docker-compose.yaml"; then
        echo "❌ Failed to copy docker-compose.yaml"
        rm -rf "$SANDBOX_DIR"
        return 1
    fi

    if ! cp "$MIS_CONFIG_FILE" "$MIS_DEPLOYMENT_DIR/configuration.json"; then
        echo "❌ Failed to copy configuration.json"
        rm -rf "$SANDBOX_DIR"
        return 1
    fi

    # Verify copied files
    if [[ ! -f "$MIS_DEPLOYMENT_DIR/docker-compose.yaml" ||
          ! -f "$MIS_DEPLOYMENT_DIR/configuration.json" ]]; then
        echo "❌ MIS deployment files were not copied correctly."
        rm -rf "$SANDBOX_DIR"
        return 1
    fi

    # Cleanup cloned repository
    rm -rf "$SANDBOX_DIR"

    echo "✅ MIS deployment setup complete."
    echo "   📄 $MIS_DEPLOYMENT_DIR/docker-compose.yaml"
    echo "   📄 $MIS_DEPLOYMENT_DIR/configuration.json"

    return 0
}