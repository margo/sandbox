# GitHub Actions Workflow: deploy-symphony.yml

## Overview
This workflow automates the deployment of the Symphony Application to a remote VM using SSH. It is triggered on pushes to the `main` and `githib-actions` branches, on pull requests targeting `main`, and can also be triggered manually.

## Triggers
- **push**: On branches `main` and `githib-actions`
- **pull_request**: On branch `main`
- **workflow_dispatch**: Manual trigger

## Workflow Creation
The workflow file `.github/workflows/deploy-symphony.yml` should be created in your repository. It defines the automation for deployment, environment setup, and test scenario execution.

## Environment Variables
- `VM_HOST`: The IP address or hostname of the target VM
- `VM_USER`: The username for SSH
- `VM_PASS`: The password for SSH

> **Note:** For security, it is recommended to store these as GitHub Secrets instead of plain text.

## Job: install_symphony
Runs on `ubuntu-latest` and performs the following steps on the remote VM:

### Steps
1. **Check and install Go and Rust**
   - Installs Go and Rust only if they are not already present on the VM.

2. **Clone or update Symphony repository**
   - Clones the Symphony repository if not present, otherwise updates it with `git pull`.

3. **Build native Rust library**
   - Builds the Rust library only if the output is missing or outdated compared to the source files.

4. **Build symphony-api Go binary**
   - Builds the Go binary for the Symphony API, setting the required library path.

5. **Run symphony-api in background**
   - Starts the Symphony API as a background process using `nohup` and `env` to set environment variables.

6. **Test Manager: Triggering Scenarios**
   - After deployment, the workflow can trigger test scenarios. This can be done using a test manager utility, such as:
     - **Maestro CLI**: For mobile or scenario-based testing, integrate Maestro CLI commands to trigger scenarios.
     - **Golang Integration Test Suite**: If your project includes Go integration tests, add a step to run `go test ./...` or a custom Go test runner after deployment.
   - Example step (Maestro CLI):
     ```yaml
     - name: Run Maestro Scenarios
       run: maestro test path/to/maestro/tests
     ```
   - Example step (Go integration tests):
     ```yaml
     - name: Run Go Integration Tests
       run: go test -v ./integration/...
     ```
   - Ensure the test manager or CLI is installed on the runner or VM as needed.

## Best Practices
- **Idempotency:** Each step checks for existing installations or builds to avoid unnecessary work.
- **Security:** Use GitHub Secrets for sensitive data.
- **Maintainability:** Each logical operation is a separate step for clarity and easier troubleshooting.
- **Test Automation:** Integrate scenario-based or integration tests to validate deployments automatically.

## Example Usage
To use this workflow, place it in `.github/workflows/deploy-symphony.yml` in your repository. Update the environment variables or secrets as needed for your VM. Add or modify test scenario steps as required for your project.

---

For further customization or troubleshooting, refer to the comments in the workflow file or contact your DevOps team.
