##### [Back To Main](../README.md)

## Environment Variables Setup

Before running any script, make sure to update the environment variable files according to your system setup.
The environment files are located here **(wfm.env and device-agent.env)**: cd $HOME/workspace/sandbox/pipeline
[Environment vairable(.env) files](../pipeline/)  

**For wfm.sh and wfm-cli.sh script**

Environment file path:-
[WFM Env file](../pipeline/wfm.env)

Update the following variables:
```bash
export EXPOSED_HARBOR_IP=<wfm-machine-ip>
export EXPOSED_SYMPHONY_IP=<wfm-machine-ip>
export EXPOSED_HARBOR_PORT=8081
export EXPOSED_SYMPHONY_PORT=8082
export DEVICE_NODE_IPS="<k3-device-machine-ip:port>,<docker-device-machine-ip:port>" # "172.19.59.148:30999,172.19.59.150:8899"  port:30999 is for k3s device & port:8899 is for docker device
export SYMPHONY_BRANCH=main #it can be a tag also
export SANDBOX_REPO_BRANCH=main #it can be a tag also
```

**For k3s/docker device-agent.sh script**

Environment file path:-
[For k3s/docker Device's Workload Fleet Management Client-Env file](../pipeline/device-agent.env)

Update the following variables:
```bash
export SANDBOX_REPO_BRANCH=main #it can be a tag also
export WFM_IP=<wfm-machine-ip>
export EXPOSED_HARBOR_IP=<wfm-machine-ip>
```

