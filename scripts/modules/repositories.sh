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

setup_mis_deployment() {
  # Clean MIS deployment files
  if [[ -d "$HOME/mis-deployment" ]]; then
    echo "🗑️  Removing existing MIS deployment files..."
    rm -rf "$HOME/mis-deployment/docker-compose.yaml"
    rm -rf "$HOME/mis-deployment/configuration.json"
  else
    echo "MIS deployment folder missing, certificates not generated. Please generate Factory Root CAs first..."
    return 1
  fi


  # Clone sandbox repo into /tmp
  rm -rf /tmp/sandbox
  echo "🔄 Cloning sandbox branch: $SANDBOX_REPO_BRANCH into /tmp/sandbox"
  if [[ -n "$GITHUB_USER" && -n "$GITHUB_TOKEN" ]]; then
    git clone --branch "$SANDBOX_REPO_BRANCH" --single-branch --depth 1 \
              "https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/margo/sandbox.git" \
              /tmp/sandbox
  else
    git clone --branch "$SANDBOX_REPO_BRANCH" --single-branch --depth 1 \
              "https://github.com/margo/sandbox.git" /tmp/sandbox
  fi

  # Copy required files to $HOME/mis-deployment
  echo "📂 Copying MIS deployment files..."
  cp /tmp/sandbox/mis/docker-compose.yaml "$HOME/mis-deployment/"
  cp /tmp/sandbox/mis/configuration.json "$HOME/mis-deployment/"
  
  # Cleanup
  rm -rf /tmp/sandbox

  echo "✅ MIS deployment setup complete in $HOME/mis-deployment"
}
