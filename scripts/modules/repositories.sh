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
    local labels_file="./labels.json"
    local capabilities_file="$HOME/sandbox/poc/device/agent/config/capabilities.json"

    # Check labels.json exists
    if [[ ! -f "$labels_file" ]]; then
        echo "File '$labels_file' not found, skipping label copying to capabilties"
        return 0
    fi

    # Check labels.json is not empty
    if [[ ! -s "$labels_file" ]]; then
        echo "File '$labels_file' is empty."
        return 1
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
