##### [Back To Main](../README.md)

## Environment Variables Setup

Before running any script, make sure to update the environment variable files according to your system setup.
The environment files are located here **(wfm.env and device-agent.env)**: cd $HOME/workspace/sandbox/pipeline
  

**For wfm.sh and wfm-cli.sh script**

Environment file path:- $HOME/workspace/sandbox/pipeline/wfm.env

Update the following variables:
```bash
export EXPOSED_HARBOR_HOST=<harbor-machine-hostname-or-ip>
export EXPOSED_SYMPHONY_HOST=<symphony-machine-hostname-or-ip>
export EXPOSED_HARBOR_PORT=8081
export EXPOSED_SYMPHONY_PORT=8082
export SYMPHONY_BRANCH=main #it can be a tag also
export SANDBOX_REPO_BRANCH=main #it can be a tag also
```

**For k3s/docker device-agent.sh script**

Environment file path:- $HOME/workspace/sandbox/pipeline/device-agent.env

Update the following variables:
```bash
export SANDBOX_REPO_BRANCH=main #it can be a tag also
export WFM_HOST=<wfm-machine-hostname-or-ip>
export EXPOSED_HARBOR_HOST=<harbor-machine-hostname-or-ip>
```

