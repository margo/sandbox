##### [Back To Main](../README.md)

## Environment Variables Setup

Before running any script, make sure to update the environment variable files according to your system setup.
The environment files are located here **(wfm.env and device-agent.env)**: cd $HOME/workspace/sandbox/scripts
  

**For wfm.sh and wfm-cli.sh script**

Environment file path:- $HOME/workspace/sandbox/scripts/wfm.env

Update the following variables:
```bash
export EXPOSED_HARBOR_HOST=<harbor-machine-hostname-or-ip>
export EXPOSED_SYMPHONY_HOST=<symphony-machine-hostname-or-ip>
export EXPOSED_HARBOR_PORT=8443  # Use 8443 for HTTPS (recommended) or 8081 for HTTP
export EXPOSED_SYMPHONY_PORT=8082
export SYMPHONY_BRANCH=main      # a tag name is also accepted
export SANDBOX_REPO_BRANCH=main  # a tag name is also accepted
```

> **Tip – DNS resolution inside Docker:** If Docker containers cannot resolve `harbor.machine` or `symphony.machine`, replace the hostname with the machine's IP address (e.g., `export EXPOSED_HARBOR_HOST=10.139.9.90`). You can confirm the issue with `curl http://<hostname>:<port>/v2/` from the host machine.
>
> **HTTP vs HTTPS:** Port `8443` requires a valid TLS certificate configured in Harbor. If Harbor is running in HTTP mode, use port `8081` and add the registry to Docker's `insecure-registries` list in `/etc/docker/daemon.json`.

**For k3s/docker device-agent.sh script**

Environment file path:- $HOME/workspace/sandbox/scripts/device-agent.env

Update the following variables:
```bash
export SANDBOX_REPO_BRANCH=main #it can be a tag also
export WFM_HOST=<wfm-machine-hostname-or-ip>
export EXPOSED_HARBOR_HOST=<harbor-machine-hostname-or-ip>
```

