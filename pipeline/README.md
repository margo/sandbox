# How to setup wfm?
export GITHUB_USER=<your-github-username>
export GITHUB_TOKEN=<your-github-token>
export EXPOSED_HARBOR_IP=<machine-ip>
export EXPOSED_GOGS_IP=<machine-ip>
export EXPOSED_KEYCLOAK_IP=<machine-ip>
export EXPOSED_SYMPHONY_IP=<machine-ip>
export DEVICE_NODE_IP=<device-ip>
export SYMPHONY_BRANCH=margo-dev-sprint-6
export DEV_REPO_BRANCH=dev-sprint-6

sudo -E bash wfm.sh

# How to setup device agent?
export GITHUB_USER=<your-github-username>
export GITHUB_TOKEN=<your-github-token>
export DEV_REPO_BRANCH=margo-dev-sprint-6
export WFM_IP=127.0.0.1
export WFM_PORT=8082
sudo -E bash device-agent.sh


# FAQ:
Q1. Seeing the following errors while installing/uninstalling
`Waiting for cache lock: Could not get lock /var/lib/dpkg/lock-frontend. It is held by process 7096 (unattended-upgr)`
Ans: There is some other process on your OS that is using the package manager, and most likely it could be the upgrade happening for your OS. Wait for some time, and then retry it. Else please check the OS docs to debug the issue.