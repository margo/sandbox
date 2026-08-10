# WFM Supplier Demo Guide for Margo Clients

## Goal of This Demo
Use two scripts to show a complete, repeatable conformance run against a live WFM endpoint:
- Script 1 prepares everything (spec, collection, payloads, variables, assertions).
- Script 2 executes the run and generates a report.

This guide is written as a presenter script so you can explain every part clearly.

## Demo Story in One Minute
1. We take the official Margo OpenAPI spec.
2. We generate a Postman collection automatically using Portman.
3. We patch the generated collection to behave like a device-agent flow.
4. We execute the collection with Newman.
5. We produce a CLI summary and an HTML report for clients.

## Files You Will Explain
- [1-setup_portman.sh](1-setup_portman.sh)
- [2-run_newman.sh](2-run_newman.sh)
- [README.md](README.md)
- [postman_collection.json](postman_collection.json)
- [newman-data/device-agent.env.json](newman-data/device-agent.env.json)
- [newman-data/device-agent.iteration.json](newman-data/device-agent.iteration.json)
- [report_20260520_101531.html](report_20260520_101531.html)

## Pre-Demo Checklist
1. Confirm WFM is running and reachable.
2. Confirm endpoint format is correct:
   - https://<host>:<port>/v1alpha2/margo
3. Confirm tools are available (scripts also auto-install missing npm tools):
   - curl, openssl, jq, node, npm
4. Make scripts executable:
   - chmod +x 1-setup_portman.sh 2-run_newman.sh

## Interactive CLI Option (Recommended for Live Demo)
Use the persona-based shell CLI to drive the demo in guided steps:

1. Go to conformance root:
   - cd /home/margo/nitin/sandbox/conformance
2. Start CLI:
   - ./conformance_cli.sh
3. Choose persona:
   - WFM Supplier or Device Supplier
4. Follow next-step prompts:
   - Setup, run, setup+run, report (WFM)
   - Build, server, tests, demo, report (Device)

CLI file:
- [../conformance_cli.sh](../conformance_cli.sh)

## Script 1 Walkthrough: Setup and Collection Preparation
File: [1-setup_portman.sh](1-setup_portman.sh)

### What it does
1. Validates required tools.
2. Downloads OpenAPI spec from Margo source.
3. Generates Postman collection using Portman.
4. Creates a fresh device identity and certificate.
5. Builds realistic request payloads for onboarding/capabilities/status.
6. Writes Newman environment and iteration files.
7. Patches generated collection for runtime variables.
8. Rewrites strict default assertions into scenario-aware status checks.

### What to say in demo
- We are not hand-writing all API tests from scratch.
- We start from the official API contract, then adapt execution behavior for realistic supplier testing.
- We preserve generated structure, but patch runtime details to match how device-side workflows behave.

### Key output artifacts
- spec.yaml
- postman_collection.json
- newman-data/certs/device.key
- newman-data/certs/device-cert.pem
- newman-data/device-agent.env.json
- newman-data/device-agent.iteration.json

## Script 2 Walkthrough: Runtime Execution and Reporting
File: [2-run_newman.sh](2-run_newman.sh)

### What it does
1. Verifies required setup artifacts exist.
2. Regenerates dynamic device identity per execution.
3. Refreshes environment and iteration payloads.
4. Runs Newman with:
   - collection
   - environment file
   - iteration file
   - insecure TLS mode
   - CLI + HTML extra report
5. Returns Newman exit code and report path.

### What to say in demo
- Every run is independent because identity and payload data are refreshed.
- We keep traceability by exporting a timestamped HTML report.
- Exit code is automation-friendly for CI/CD integration.

## Why Scenario-Aware Assertions Were Added
Default Portman tests are success-path oriented. In real supplier testing, some failure-path responses are expected for certain sequences.

So the setup script rewrites request-level tests to allow endpoint-specific status ranges, for example:
- onboarding can allow conflict/error states when identity already exists.
- retrieval endpoints can allow not found/unauthorized paths based on data preconditions.

### Important explanation to clients
A passing run means responses matched expected scenario allowlists for this demo profile.
It does not always mean every endpoint returned 2xx.

## End-to-End Demo Commands
Run from this folder: [sandbox/conformance/wfm-supplier](sandbox/conformance/wfm-supplier).

1. Setup:
   ./1-setup_portman.sh https://20.64.178.117:8082/v1alpha2/margo

2. Execute:
   ./2-run_newman.sh

3. Check latest report:
   ls -1t report_*.html | head -1

## How to Explain the Report to Clients
1. Start with total requests, assertions, and failed count.
2. Show each endpoint row and returned status.
3. Explain whether each status is expected in this scenario profile.
4. Open HTML report for a visual walkthrough and traceability.

## Suggested Demo Talk Track (Short)
- We generate from official spec to avoid drift from contract.
- We inject device-agent style payloads and runtime variables.
- We run repeatable conformance checks through Newman.
- We publish a report that can be reviewed by engineering and business teams.
- We can tighten or relax status allowlists based on agreed test policy.

## Questions Clients Usually Ask
1. Can we add custom vendor tests?
   - Yes. Add requests/tests to the generated collection and run through the same Newman pipeline.

2. Can we enforce strict success-only policy?
   - Yes. Update allowlists in setup patching logic to require only 2xx where desired.

3. Can this be used in CI?
   - Yes. Use script exit code and archive generated HTML reports.

4. Can we test different WFM instances quickly?
   - Yes. Pass a different BASE URL to setup and rerun execution.

## One-Line Summary for Closing
This demo shows a contract-driven, automation-ready conformance workflow for WFM supplier testing, with clear reporting and configurable assertion policy.
