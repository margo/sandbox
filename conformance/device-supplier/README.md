# Margo Device Supplier Conformance Suite

High-level entry point for running the suite and finding detailed documentation.

## Current Status

- 43/43 tests passing
- RFC 9421 request signing and verification enabled
- Data-driven test and validation model

## What Goes Where

- Use this README for quick setup and daily commands.
- Use [Final-Summary.md](Final-Summary.md) for full architecture, schemas, and authoring guides.

## Quick Start

From this directory (`margo/personas/device_supplier`):

1. Build binaries:

```bash
make build
```

2. Start server (Terminal 1):

```bash
make run-server
```

3. Run tests (Terminal 2):

```bash
make run-tests
```

4. Open report:

```bash
open reports/conformance-report-*.html
```

Expected summary: `Test Results: 43 PASSED, 0 FAILED`.

## Frequently Used Commands

| Command | Purpose |
|---|---|
| `make build` | Build server and test runner |
| `make run-server` | Start mock WFM server |
| `make run-tests` | Execute all conformance tests |
| `make demo` | Demo flow (starts server, runs tests) |
| `make kill-server` | Stop running server process |
| `make clean` | Remove build artifacts |

## Detailed Documentation

Deep documentation lives in [Final-Summary.md](Final-Summary.md).

- Full architecture: [Final-Summary.md](Final-Summary.md#architecture-overview)
- API endpoint behavior: [Final-Summary.md](Final-Summary.md#all-8-api-endpoints)
- Test scenario schema: [Final-Summary.md](Final-Summary.md#test-scenario-schema)
- Assertion schema: [Final-Summary.md](Final-Summary.md#assertion-schema)
- How to write new test scenarios: [Final-Summary.md](Final-Summary.md#step-by-step-guide-writing-a-new-test)
- How to write new assertions: [Final-Summary.md](Final-Summary.md#how-to-write-a-new-assertion-step-by-step)
- Certificate handling: [Final-Summary.md](Final-Summary.md#certificate-handling)
- Environment variables: [Final-Summary.md](Final-Summary.md#environment-variables)
- Troubleshooting: [Final-Summary.md](Final-Summary.md#faq--troubleshooting)

Additional certificate docs:

- [docs/CERTIFICATE_ARCHITECTURE.md](docs/CERTIFICATE_ARCHITECTURE.md)
- [docs/CERTIFICATE_GENERATION.md](docs/CERTIFICATE_GENERATION.md)

## Certificate Distribution for Real Device-Agent

For this server/suite, the only required handoff to a real device-agent is the CA certificate.

Copy `certs/ca-cert.pem` from this repo to the device-agent VM so the agent trusts the mock server CA.

Example real device-agent path:

```bash
cd ~/sandbox/poc/device/agent/config/
ls -lrt
```

Copy command example (from suite host to agent host):

```bash
scp certs/ca-cert.pem margo@margo-device-k3s:~/sandbox/poc/device/agent/config/ca-cert.pem
```

Or if you are on the same host:

```bash
cp certs/ca-cert.pem ~/sandbox/poc/device/agent/config/ca-cert.pem
```

Current example path (from current real device-agent setup):

```text
~/sandbox/poc/device/agent/config/
```

Important:

- This path is an example for the current environment.
- Vendor-provided device-agents may use a different cert/config location.
- The requirement remains the same: place `ca-cert.pem` in whatever location the device-agent is configured to read as its trusted CA.

## Minimal Troubleshooting

1. Server not reachable:

```bash
make run-server
```

2. Signature failures (401):

- Verify device cert/key pairing used by runner.
- See [Final-Summary.md](Final-Summary.md#certificate-handling).

3. Validation failures (400/422):

- Check request body against assertion rules in [Final-Summary.md](Final-Summary.md#assertion-schema).
