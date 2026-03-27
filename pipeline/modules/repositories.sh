#!/bin/bash
# modules/wfm/repositories.sh - Git repository cloning

source "$(dirname "${BASH_SOURCE[0]}")/../../lib/common.sh"

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
