##### [Back To Main](../README.md)
## Running the EasyCLI script

This guide helps you get started with the WFM CLI script to interact with the Workload Fleet Manager (WFM). This is implemented on top of Eclipse Symphony's Maestro CLI.

### Interactive Mode (Recommended)
Needs to be ran at WFM VM
```bash
chmod +x wfm-cli.sh
./wfm-cli.sh
```

This launches an interactive menu with options:
- List app packages, devices, deployments
- Upload/delete app packages
- Deploy/delete instances

### Command Line Mode
```bash
./wfm-cli.sh <command>
```
Available commands:
- `list-packages` - List all app packages
- `list-devices` - List all devices  
- `list-deployments` - List all deployments
- `list-all` - List all resources
- `upload` - Upload app package
- `delete-package` - Delete app package
- `deploy` - Deploy instance
- `delete-instance` - Delete instance

