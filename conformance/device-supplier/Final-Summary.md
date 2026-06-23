# Margo Device Supplier Conformance Suite — Complete Guide

**Version:** 1.0 (June 2026)
**API Spec:** Margo Management Interface v1.0.0

---

## What Is This Suite?

This is a **conformance testing framework** for the **Device Supplier** persona in the Margo ecosystem. Its job is to verify that a real device-agent correctly implements the Margo Management Interface API by testing it against a mock WFM (Workload Fleet Manager) server.

**Who uses it:** A Margo vendor who has built a device-agent and wants to verify it behaves correctly before connecting it to a real WFM.

**What it does:**
- Runs a mock WFM server that enforces the full Margo Management Interface spec
- Accepts connections from any device-agent implementation
- Validates every API call the device-agent makes against a rulebook (`assertions.json`)
- Reports pass/fail per test step

**Important: You do NOT need to write any Go code.** The test-scenarios are plain JSON files you create following a template. The mock server reads validation rules from `assertions.json` — all data-driven, no compilation needed to add or change tests.

---

## Architecture

```
                         ┌──────────────────────────────────┐
                         │     MOCK WFM SERVER (bin/server)  │
                         │     https://localhost:3001         │
                         │                                    │
  YOUR DEVICE-AGENT ────►│  Validates requests against        │
  (real device, the      │  assertions.json rulebook          │
   thing being tested)   │                                    │
                         │  ✓ RFC 9421 signature check        │
                         │  ✓ Content-Digest check            │
                         │  ✓ Schema validation               │
                         └──────────────────────────────────┘
                                        │
                                        ▼
                               Reports / Logs

───────── Additionally, for scripted testing: ─────────

  ┌──────────────────────────────────────┐
  │  MOCK DEVICE-AGENT (bin/run_tests)   │
  │  Reads test-scenarios.json           │
  │  Sends scripted API calls            │
  │  Validates responses                 │
  └──────────────────────────────────────┘
       ↑
       Your test-scenarios.json
       (custom format, explained below)
```

**Two ways to run conformance tests:**

| Method | When to use | How |
|--------|-------------|-----|
| **Scripted (run_tests.go)** | Verify mock server works; automate specific API call sequences | Write test-scenarios.json → `make demo` |
| **Real device-agent** | Test your actual device-agent implementation | Start mock server → connect your device-agent |

The **recommended real-world workflow** is: use the Data-Generator CLI to register your test scenarios, then use the Runner CLI to start the mock server and run tests. Your real device-agent connects to the mock server for actual conformance testing.

---

## Directory Layout

```
conformance/
├── conformance.sh                  CLI #1 — Data-Generator
├── run-tests.sh                    CLI #2 — Runner
│
├── Data-Generator/
│   └── device-supplier/
│       ├── assertions.json         Validation rulebook (server copy)
│       ├── test-scenarios.json     Active test scenarios
│       └── groups/                 Test groups
│           ├── gold/
│           │   ├── group.json      Which test IDs belong to this group
│           │   └── test-scenarios.json
│           └── silver/
│               └── ...
│
└── device-supplier/                Mock WFM server source
    ├── Makefile                    Build + run commands
    ├── run_tests.go                Mock device-agent (internal use)
    ├── manifests/
    │   └── assertions.json         Server validation rulebook (authoritative)
    ├── device-scenarios/
    │   └── test-scenarios.json     Active test scenarios for run_tests
    ├── certs/                      TLS certificates
    │   ├── ca-cert.pem             Root CA — give this to your real device-agent
    │   ├── server-cert.pem         Mock WFM server TLS cert
    │   ├── device-cert.pem         Demo device cert (used by run_tests.go)
    │   └── device-key.pem          Demo device key (used by run_tests.go)
    ├── bin/
    │   ├── server                  Built mock WFM server binary
    │   └── run_tests               Built mock device-agent binary
    └── reports/
        └── conformance-report-*.html
```

---

## Prerequisites

| Requirement | Check |
|-------------|-------|
| Go 1.20+ | `go version` |
| `jq` | `jq --version` |
| `openssl` | `openssl version` |
| `make` | `make --version` |

---

## Part 1 — Create Your Test Scenarios

### The Custom Format

Margo vendors create their test scenarios as a **custom JSON array** following this format. This is NOT Postman, NOT pytest, NOT any external framework — it is a purpose-built format parsed directly by the mock device-agent runner.

**Start from this template — copy it and fill in your own values:**
```
conformance/device-supplier/device-scenarios/TEMPLATE_custom_test_scenario.json
```

This file contains four ready-to-use scenario skeletons covering every Margo endpoint (onboarding, capabilities, deployments, bundle download, status reporting, and negative tests). Copy it, rename it to something like `my-device-test-scenarios.json`, and replace the placeholder values with your device's details.

> `test-scenarios.json` in the same folder is the full reference implementation — it is the complete working test suite used by the conformance runner. Use the **TEMPLATE** file, not `test-scenarios.json`, as your starting point.

### Top-Level Structure

Your file must be a **JSON array of scenario objects**:

```json
[
  {
    "id": "scenario-my-device-onboarding",
    "name": "Device Onboarding",
    "description": "Tests the onboarding flow for my device",
    "steps": [ ... ]
  },
  {
    "id": "scenario-my-device-capabilities",
    "name": "Capability Reporting",
    "description": "Tests capability POST/PUT",
    "steps": [ ... ]
  }
]
```

Scenarios run **top to bottom**. Each scenario is independent — variables do not leak between scenarios.

---

### The 8 Margo Endpoints You Can Test

Every step's `endpoint` must be one of these. Anything else returns 404.

| # | Method | Endpoint | Purpose | Auth required? |
|---|--------|----------|---------|----------------|
| 1 | GET | `/api/v1/onboarding/certificate` | Fetch the WFM root CA certificate | No |
| 2 | POST | `/api/v1/onboarding` | Register device — receive `clientId` | No (bootstrap) |
| 3 | POST | `/api/v1/clients/{clientId}/capabilities` | Report device hardware + roles | Yes (sign) |
| 4 | PUT | `/api/v1/clients/{clientId}/capabilities` | Update device hardware + roles | Yes (sign) |
| 5 | GET | `/api/v1/clients/{clientId}/deployments` | Get deployment manifest | Yes (sign) |
| 6 | GET | `/api/v1/clients/{clientId}/bundles/{digest}` | Download deployment bundle tarball | Yes (sign) |
| 7 | GET | `/api/v1/clients/{clientId}/deployments/{deploymentId}/{digest}` | Get individual deployment manifest | Yes (sign) |
| 8 | POST | `/api/v1/clients/{clientId}/deployments/{deploymentId}/status` | Report deployment status | Yes (sign) |

> `{clientId}`, `{digest}`, `{deploymentId}` are placeholder tokens — the runner substitutes values you save with `extract_context` from earlier steps.

"Auth required" means the request must carry a valid RFC 9421 HTTP signature. The runner signs automatically unless you set `"skip_signing": true`.

---

### Valid Field Values

Use these exact values in your request bodies to pass validation. Any other value returns 422.

**Onboarding request:**
```json
"apiVersion": "onboarding.margo.org/v1alpha1"
"kind":       "OnboardingRequest"
```

**Capabilities manifest:**
```json
"apiVersion": "device.margo.org/v1alpha1"
"kind":       "DeviceCapabilitiesManifest"
```

Valid `properties.roles` values (must have at least one):
```
"Standalone Cluster"   "Cluster Leader"   "Standalone Device"
```

Valid `resources.cpu.architecture` values:
```
"amd64"   "x86_64"   "arm64"   "arm"
```

Valid `resources.interfaces[*].type` values:
```
"ethernet"   "wifi"   "cellular"   "bluetooth"   "usb"   "canbus"   "rs232"
```

**Deployment status report:**
```json
"apiVersion": "deployment.margo.org/v1alpha1"
"kind":       "DeploymentStatusManifest"
```

Valid `status.state` and `components[*].state` values:
```
"pending"   "installing"   "installed"   "failed"   "removing"   "removed"
```

---

### Step Object — Full Schema

```json
{
  "id": "step-1.1",
  "name": "Onboard My Device",
  "method": "POST",
  "endpoint": "/api/v1/onboarding",
  "request_body": {
    "apiVersion": "onboarding.margo.org/v1alpha1",
    "kind": "OnboardingRequest",
    "certificate": "./certs/device-cert.pem"
  },
  "headers": {},
  "expected_status": 201,
  "validations": [
    { "field": "clientId", "operation": "exists" }
  ],
  "extract_context": {
    "clientId": "clientId"
  },
  "skip_signing": false,
  "skip_certificate_injection": false
}
```

#### All Step Fields

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `id` | Yes | string | Unique step ID. Use format `step-X.Y` (e.g., `step-1.1`, `step-2.3`). Referenced in group.json. |
| `name` | Yes | string | Human-readable name shown in reports |
| `method` | Yes | string | HTTP verb: `"GET"`, `"POST"`, or `"PUT"` |
| `endpoint` | Yes | string | URL path, without base URL. Supports `{placeholder}` substitution |
| `request_body` | No | object | JSON body for POST/PUT. Omit for GET requests |
| `headers` | No | object | Extra HTTP headers. Values support `{placeholder}` substitution |
| `expected_status` | Yes | number | Expected HTTP status code. Any other code → FAIL |
| `validations` | No | array | Checks to run on the response body/headers |
| `extract_context` | No | object | Save response values as named variables for later steps |
| `skip_signing` | No | bool | Default `false`. Set `true` to skip RFC 9421 signature (for unsigned-request rejection tests) |
| `skip_certificate_injection` | No | bool | Default `false`. Set `true` to send the `certificate` string as-is without loading from a file path |

### Validations — Checking the Response

```json
"validations": [
  { "field": "clientId",        "operation": "exists" },
  { "field": "status",          "operation": "equals",   "value": "capabilities_received" },
  { "field": "error",           "operation": "contains", "value": "certificate" },
  { "field": "certificate",     "operation": "not_empty" },
  { "field": "deployments",     "operation": "is_array" },
  { "field": "_headers.ETag",   "operation": "not_empty" }
]
```

#### Validation Operations

| Operation | `value` field needed? | Passes when |
|-----------|----------------------|-------------|
| `exists` | No | Field is present (any value, even empty) |
| `not_empty` | No | Field is present and not `""`, `null`, or `[]` |
| `equals` | Yes | Field value matches `value` exactly |
| `contains` | Yes | Field value (string) contains `value` as a substring |
| `is_string` | No | Field value is a JSON string |
| `is_number` | No | Field value is a JSON number |
| `is_array` | No | Field value is a JSON array |
| `is_object` | No | Field value is a JSON object |

#### Field Path Syntax

| Path | Accesses |
|------|---------|
| `"clientId"` | Top-level JSON field |
| `"status.state"` | Nested field |
| `"deployments.0.deploymentId"` | First array element's field |
| `"_headers.ETag"` | HTTP response header (`_headers.` prefix) |

### Context — Passing Data Between Steps

`extract_context` saves a value from one step's response. Saved values can be injected as `{placeholders}` in later steps within the same scenario.

**Save a value:**
```json
"extract_context": {
  "clientId":     "clientId",
  "manifestEtag": "_headers.ETag",
  "deploymentId": "deployments.0.deploymentId"
}
```

Left side = your variable name. Right side = JSON path in the response.

**Use a saved value:**
```json
"endpoint": "/api/v1/clients/{clientId}/capabilities",

"headers": {
  "If-None-Match": "{manifestEtag}"
},

"request_body": {
  "deploymentId": "{deploymentId}"
}
```

Placeholders work in: `endpoint`, `headers` values, and string values inside `request_body`.

### Certificate Handling

The `certificate` field in `request_body` has special handling:

**Load from file (positive tests):**
```json
"request_body": {
  "certificate": "./certs/device-cert.pem"
}
```
The runner detects the `./certs/` prefix and reads the PEM file. Actual cert content is sent.

**Use a literal string (rejection tests):**
```json
"request_body": {
  "certificate": "rnd-key-7f3a91b2c4d8e6"
},
"skip_certificate_injection": true
```
The literal string is sent as-is. The server's rejection list blocks it with 403.

---

### Common Test Patterns

Every scenario that needs a `clientId` must onboard first. Use a setup step at the start:

```json
{
  "id": "step-1.0",
  "name": "Onboard Device (Setup)",
  "method": "POST",
  "endpoint": "/api/v1/onboarding",
  "request_body": {
    "apiVersion": "onboarding.margo.org/v1alpha1",
    "kind": "OnboardingRequest",
    "certificate": "./certs/device-cert.pem"
  },
  "headers": {},
  "expected_status": 201,
  "validations": [],
  "extract_context": { "clientId": "clientId" },
  "skip_signing": false
}
```

**Pattern A — Positive test (expect the call to succeed):**
```json
{
  "id": "step-1.1",
  "name": "Report Capabilities",
  "method": "POST",
  "endpoint": "/api/v1/clients/{clientId}/capabilities",
  "request_body": {
    "apiVersion": "device.margo.org/v1alpha1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {
      "id": "my-device",
      "vendor": "My Vendor",
      "modelNumber": "MODEL-001",
      "serialNumber": "SN-12345",
      "roles": ["Standalone Device"],
      "resources": {
        "cpu": { "cores": 4, "architecture": "amd64" },
        "memory": "8Gi",
        "storage": "128Gi",
        "interfaces": [{ "type": "ethernet" }],
        "peripherals": []
      }
    }
  },
  "headers": {},
  "expected_status": 201,
  "validations": [],
  "extract_context": {},
  "skip_signing": false
}
```

**Pattern B — Validation error test (expect 422 for bad field value):**

Change any value to something invalid (wrong role, wrong architecture, empty array) and expect 422:
```json
{
  "id": "step-2.1",
  "name": "Reject Invalid Role",
  "method": "POST",
  "endpoint": "/api/v1/clients/{clientId}/capabilities",
  "request_body": {
    "apiVersion": "device.margo.org/v1alpha1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {
      "id": "my-device", "vendor": "My Vendor",
      "modelNumber": "MODEL-001", "serialNumber": "SN-12345",
      "roles": ["NotAValidRole"],
      "resources": {
        "cpu": { "cores": 4, "architecture": "amd64" },
        "memory": "8Gi", "storage": "128Gi",
        "interfaces": [{ "type": "ethernet" }],
        "peripherals": []
      }
    }
  },
  "headers": {},
  "expected_status": 422,
  "validations": [{ "field": "errors", "operation": "is_array" }],
  "extract_context": {},
  "skip_signing": false
}
```

Other things that trigger 422: empty `roles` array, unknown `architecture`, unknown `interface.type`, missing required field, invalid `status.state` in a status report.

**Pattern C — Signature rejection test (expect 401):**

Set `"skip_signing": true` to send the request without a signature:
```json
{
  "id": "step-2.2",
  "name": "Reject Unsigned Capabilities",
  "method": "POST",
  "endpoint": "/api/v1/clients/{clientId}/capabilities",
  "request_body": { ... },
  "headers": {},
  "expected_status": 401,
  "validations": [{ "field": "error", "operation": "exists" }],
  "extract_context": {},
  "skip_signing": true
}
```

**Pattern D — Schema error test (expect 400):**

Send a request with missing required fields or wrong `apiVersion`/`kind`:
```json
{
  "id": "step-2.3",
  "name": "Reject Missing Certificate",
  "method": "POST",
  "endpoint": "/api/v1/onboarding",
  "request_body": {
    "apiVersion": "onboarding.margo.org/v1alpha1",
    "kind": "OnboardingRequest"
  },
  "headers": {},
  "expected_status": 400,
  "validations": [{ "field": "error", "operation": "exists" }],
  "extract_context": {},
  "skip_signing": false
}
```

Other things that trigger 400: missing `Content-Digest` header on a POST/PUT (add header `"Content-Digest": ""` and omit the body hash), wrong or missing `kind`.

**Pattern E — ETag caching test (expect 304):**

Save the `ETag` from a GET response, then send it back with `If-None-Match`:
```json
{
  "id": "step-3.1",
  "name": "Get Deployments",
  "method": "GET",
  "endpoint": "/api/v1/clients/{clientId}/deployments",
  "headers": { "Accept": "application/vnd.margo.manifest.v1+json" },
  "expected_status": 200,
  "validations": [],
  "extract_context": { "manifestEtag": "_headers.ETag" },
  "skip_signing": false
},
{
  "id": "step-3.2",
  "name": "Get Deployments — Cached (should return 304)",
  "method": "GET",
  "endpoint": "/api/v1/clients/{clientId}/deployments",
  "headers": {
    "Accept": "application/vnd.margo.manifest.v1+json",
    "If-None-Match": "{manifestEtag}"
  },
  "expected_status": 304,
  "validations": [],
  "extract_context": {},
  "skip_signing": false
}
```

---

### Complete Example: Onboarding → Capabilities → Deployment

```json
[
  {
    "id": "scenario-vendor-happy-path",
    "name": "Vendor Happy Path",
    "description": "Full onboarding, capabilities, and deployment status flow",
    "steps": [
      {
        "id": "step-1.0",
        "name": "Onboard Device",
        "method": "POST",
        "endpoint": "/api/v1/onboarding",
        "request_body": {
          "apiVersion": "onboarding.margo.org/v1alpha1",
          "kind": "OnboardingRequest",
          "certificate": "./certs/device-cert.pem"
        },
        "headers": {},
        "expected_status": 201,
        "validations": [
          { "field": "clientId", "operation": "exists" }
        ],
        "extract_context": {
          "clientId": "clientId"
        }
      },
      {
        "id": "step-1.1",
        "name": "Report Capabilities",
        "method": "POST",
        "endpoint": "/api/v1/clients/{clientId}/capabilities",
        "request_body": {
          "apiVersion": "device.margo.org/v1alpha1",
          "kind": "DeviceCapabilitiesManifest",
          "properties": {
            "id": "vendor-device-001",
            "vendor": "Vendor Corp",
            "modelNumber": "VC-1000",
            "serialNumber": "SN-ABC123",
            "roles": ["Standalone Device"],
            "resources": {
              "cpu": { "cores": 4, "architecture": "arm64" },
              "memory": "8Gi",
              "storage": "64Gi",
              "interfaces": [{ "type": "ethernet" }],
              "peripherals": []
            }
          }
        },
        "headers": {},
        "expected_status": 201,
        "validations": [
          { "field": "status", "operation": "equals", "value": "capabilities_received" }
        ],
        "extract_context": {}
      },
      {
        "id": "step-1.2",
        "name": "Get Deployments",
        "method": "GET",
        "endpoint": "/api/v1/clients/{clientId}/deployments",
        "headers": {
          "Accept": "application/vnd.margo.manifest.v1+json"
        },
        "expected_status": 200,
        "validations": [
          { "field": "deployments", "operation": "is_array" },
          { "field": "_headers.ETag", "operation": "exists" }
        ],
        "extract_context": {
          "deploymentId": "deployments.0.deploymentId"
        }
      },
      {
        "id": "step-1.3",
        "name": "Post Status",
        "method": "POST",
        "endpoint": "/api/v1/clients/{clientId}/deployments/{deploymentId}/status",
        "request_body": {
          "apiVersion": "deployment.margo.org/v1alpha1",
          "kind": "DeploymentStatusManifest",
          "deploymentId": "{deploymentId}",
          "status": { "state": "installed" },
          "components": [
            { "name": "my-app", "state": "installed" }
          ]
        },
        "headers": {},
        "expected_status": 200,
        "validations": [
          { "field": "acknowledgement", "operation": "equals", "value": "received" }
        ],
        "extract_context": {}
      }
    ]
  }
]
```

### Quick Reference: Which Status Code to Expect?

| Endpoint | Success | Invalid body | No signature | Cert blocked | Wrong digest / client |
|----------|---------|-------------|-------------|-------------|----------------------|
| GET /onboarding/certificate | 200 | — | — | — | — |
| POST /onboarding | 201 | 400 | — (not required) | 403 | — |
| POST /capabilities | 201 | 422 | 401 | — | 404 (unknown clientId) |
| PUT /capabilities | 201 | 422 | 401 | — | 404 (unknown clientId) |
| GET /deployments | 200 / 304 | 406 (wrong Accept header) | 401 | — | 404 (unknown clientId) |
| GET /bundles/{digest} | 200 / 304 | — | 401 | — | 404 (wrong digest) |
| GET /deployments/{id}/{digest} | 200 / 304 | — | 401 | — | 404 (wrong digest) |
| POST /status | 200 | 422 | 401 | — | 422 (deploymentId mismatch) |

---

## Part 2 — Register Scenarios with Data-Generator

The **Data-Generator** (`conformance.sh`) organises your test-scenarios.json files into named groups. Groups let you run targeted subsets of tests.

### What It Does

1. You place your `test-scenarios.json` file(s) in a folder
2. Run `conformance.sh` (interactive CLI)
3. It reads all JSON files, extracts all step/scenario IDs, copies files to `Data-Generator/device-supplier/groups/<name>/`
4. Creates a `group.json` listing every test ID in that group

The Runner CLI later reads from these groups to decide which scenarios to run.

### Run the Data-Generator

```bash
cd /home/margo/test/sandbox/conformance
bash conformance.sh
```

Follow the interactive prompts:
1. Select **Device Supplier** persona
2. Select or create a group (e.g., `myvendor`)
3. Point to your `test-scenarios.json` file

The CLI creates:
```
Data-Generator/device-supplier/groups/myvendor/
  ├── group.json            # lists all test IDs extracted from your files
  └── test-scenarios.json   # copy of your file
```

### group.json Format

```json
{
  "name": "myvendor",
  "version": "1.0.0",
  "persona": "device-supplier",
  "description": "My vendor conformance scenarios",
  "testCases": [
    "step-1.0",
    "step-1.1",
    "step-1.2",
    "scenario-vendor-happy-path"
  ]
}
```

The `testCases` array contains IDs that will be used to filter which steps/scenarios run. If no IDs match, ALL scenarios run (safe fallback).

---

## Part 3 — Run Conformance Tests

### Option A: Via Runner CLI (Recommended)

The **Runner** (`run-tests.sh`) starts the mock WFM server, runs your test scenarios through the mock device-agent, then generates a report.

```bash
cd /home/margo/test/sandbox/conformance
bash run-tests.sh
```

Follow the interactive prompts:
1. Select **Device Supplier** (option 2)
2. Select **Group-based test scenarios** (option 1)
3. Select your group (e.g., `myvendor`)

The runner:
- Builds `bin/server` and `bin/run_tests` if not already built
- Starts `bin/server` (mock WFM) in background on `https://localhost:3001`
- Copies your scenarios from the group into `device-scenarios/test-scenarios.json`
- Runs `bin/run_tests` (mock device-agent) which reads the scenarios and fires API calls
- Stops the server and generates an HTML report in `reports/`

### Option B: Via Makefile (Quick Demo)

From the `device-supplier/` directory you can build and run directly:

```bash
cd /home/margo/test/sandbox/conformance/device-supplier

# Build both binaries
make build

# Run full demo (starts server + runs tests + generates report)
make demo

# Or step by step:
make run-server        # Terminal 1 — start mock WFM server
make run-tests         # Terminal 2 — run mock device-agent

# Stop server
make kill-server
```

#### All Makefile Targets

| Command | What it does |
|---------|-------------|
| `make build` | Build mock server (`bin/server`) and test runner (`bin/run_tests`) |
| `make build-server` | Build mock server only |
| `make build-tests` | Build test runner only |
| `make run-server` | Start mock WFM server in background on port 3001 |
| `make run-tests` | Run the mock device-agent against the server |
| `make demo` | Build → start server → run tests → show report path |
| `make kill-server` | Stop the running server |
| `make clean` | Remove `bin/` directory |

---

## Part 4 — Test with Your Real Device-Agent

This is the primary conformance test path. Your actual device-agent implementation connects to the mock WFM server. The mock server validates every request it receives.

### Step 1: Build and Start the Mock WFM Server

```bash
cd /home/margo/test/sandbox/conformance/device-supplier
make build
make run-server
```

The server starts on `https://localhost:3001`. Check it is running:
```bash
curl -k https://localhost:3001/health
```

### Step 2: Give Your Device-Agent the CA Certificate

Your device-agent must trust the mock server's TLS certificate. Copy the mock server's CA cert to wherever your device-agent expects its trusted CA:

```bash
# The mock server CA cert is at:
ls /home/margo/test/sandbox/conformance/device-supplier/certs/ca-cert.pem

# Copy it to the device-agent's expected CA location
# (adjust destination path to match your device-agent config)
cp /home/margo/test/sandbox/conformance/device-supplier/certs/ca-cert.pem \
   ~/certs/ca-cert.pem
```

### Step 3: Generate Device Certificates (if needed)

Use the `device-agent.sh` script to generate ECDSA device certificates (the format required by the Margo spec — P-256 curve):

```bash
cd /home/margo/test/sandbox/scripts
sudo -E bash device-agent.sh
```

In the interactive menu:
1. Enter `1` to select **Docker** device type
2. Choose option `12` → **create_device_ecdsa_certs**

This generates:
```
~/certs/
  ├── device-ecdsa.key    # ECDSA P-256 private key
  └── device-ecdsa.crt    # Self-signed device certificate
```

Copy these into your device-agent's certificate configuration directory and configure your device-agent to:
- Use `device-ecdsa.crt` as its device certificate (sent during onboarding)
- Use `device-ecdsa.key` to sign RFC 9421 HTTP requests

### Step 4: Clear Device State (Required When Switching WFMs)

> **Critical:** If your device-agent has previously connected to a real WFM, it stores a `clientId` locally. The mock WFM starts fresh with no clients registered — so the device will get a 404 on its first capabilities call and the test will fail.
>
> Always clear the device-agent's persisted data before switching to the mock WFM:

```bash
# Stop the running device-agent container first
cd ~/sandbox/docker-compose
docker compose down

# Delete the persisted onboarding state (forces fresh onboarding with mock WFM)
rm -rf ~/sandbox/docker-compose/data/
```

The device-agent will now re-register from scratch with the mock WFM and get a new `clientId`.

### Step 5: Start Your Device-Agent Against the Mock WFM

The `device-agent.env` file sets `WFM_HOST=symphony.machine` (or your configured host). Override only the port so the agent connects to the mock WFM instead of the real one:

```bash
# Start device-agent pointing at mock WFM port 3001
WFM_PORT=3001 sudo -E bash /home/margo/test/sandbox/scripts/device-agent.sh docker start-docker
```

> **Note:** `WFM_HOST` comes from `device-agent.env` (already points to the correct host). Only `WFM_PORT` needs to be overridden from the default. Setting `WFM_PORT=3001` before the `sudo -E` call preserves it through the environment.

Monitor device-agent logs:
```bash
sudo docker logs workload-fleet-management-client -f
```

### Step 6: Observe Results

Watch the mock server logs to see what the device-agent sends and how the server responds:

```bash
tail -f /tmp/wfm-server.log
```

A successful full flow looks like this in the device-agent logs:

```
INFO  Starting device onboarding
INFO  Preflight-http-request POST https://symphony.machine:3001/v1alpha2/margo/api/v1/onboarding
INFO  Device onboarding successful  {"deviceClientId": "ebf0e8d1-710b-4d92-a0ce-cca4d4c72576"}
INFO  Device onboarded              {"deviceId": "ebf0e8d1-..."}
INFO  Starting capabilities reporting
INFO  Preflight-http-request POST .../clients/ebf0e8d1-.../capabilities
INFO  Capabilities reported successfully
INFO  Workload Fleet Management Client started successfully
INFO  Preflight-http-request GET  .../clients/ebf0e8d1-.../deployments
INFO  Bundle downloaded successfully
INFO  Performing sync....
INFO  No change in desired and current states (304 Not Modified)   ← polling working
```

And in the mock WFM server logs:

```
[Router] POST /v1alpha2/margo/api/v1/onboarding
[Router] POST /v1alpha2/margo/api/v1/clients/{clientId}/capabilities
[Router] GET  /v1alpha2/margo/api/v1/clients/{clientId}/deployments
[Router] GET  /v1alpha2/margo/api/v1/clients/{clientId}/bundles/{digest}
[Router] POST /v1alpha2/margo/api/v1/clients/{clientId}/deployments/{appId}/status
```

The mock server validates every request against `assertions.json`. Validation failures appear in the log as 400/401/422 responses with JSON error details.

### Stop When Done

```bash
cd /home/margo/test/sandbox/conformance/device-supplier
make kill-server
```

---

## API Reference — All 8 Endpoints

Base URL: `https://localhost:3001/v1alpha2/margo`

### 1. GET /api/v1/onboarding/certificate

Retrieve the Root CA certificate. No authentication required.

```
→ Response 200:
{ "certificate": "-----BEGIN CERTIFICATE-----\n..." }
```

### 2. POST /api/v1/onboarding

Register the device with the WFM. No signature required (bootstrap step — device has no key yet).

```json
→ Request body:
{
  "apiVersion": "onboarding.margo.org/v1alpha1",
  "kind": "OnboardingRequest",
  "certificate": "<device PEM certificate>"
}

→ Response 201:
{ "clientId": "<uuid>" }

→ Response 400: invalid fields
{ "error": "field description" }

→ Response 403: blocklisted certificate
{ "error": "Client rejected: ..." }
```

### 3. POST /api/v1/clients/{clientId}/capabilities

Report device hardware capabilities. **RFC 9421 signature + Content-Digest required.**

```json
→ Request body:
{
  "apiVersion": "device.margo.org/v1alpha1",
  "kind": "DeviceCapabilitiesManifest",
  "properties": {
    "id": "device-001",
    "vendor": "Vendor Corp",
    "modelNumber": "MDL-100",
    "serialNumber": "SN-12345",
    "roles": ["Standalone Device"],
    "resources": {
      "cpu": { "cores": 4, "architecture": "arm64" },
      "memory": "8Gi",
      "storage": "64Gi",
      "interfaces": [{ "type": "ethernet" }],
      "peripherals": []
    }
  }
}

→ Response 201:
{ "status": "capabilities_received" }

→ Response 401: missing or invalid signature
→ Response 422: schema validation failure
{ "status": "validation_failed", "errors": [...] }
```

**Valid `roles` values:** `"Standalone Cluster"`, `"Cluster Leader"`, `"Standalone Device"`  
**Valid `cpu.architecture` values:** `"amd64"`, `"x86_64"`, `"arm64"`, `"arm"`  
**Valid `interfaces[*].type` values:** `"ethernet"`, `"wifi"`, `"cellular"`, `"bluetooth"`, `"usb"`, `"canbus"`, `"rs232"`  
**Valid `peripherals[*].type` values:** `"gpu"`, `"display"`, `"camera"`, `"microphone"`, `"speaker"`

### 4. PUT /api/v1/clients/{clientId}/capabilities

Update device capabilities. Same body and rules as POST. **RFC 9421 signature required.**

### 5. GET /api/v1/clients/{clientId}/deployments

Retrieve the deployment manifest. **RFC 9421 signature required.** Must set `Accept: application/vnd.margo.manifest.v1+json`.

```
→ Response 200: manifest with ETags
→ Response 304: not modified (ETag match via If-None-Match)
→ Response 406: unsupported Accept header
```

### 6. GET /api/v1/clients/{clientId}/bundles/{digest}

Download the deployment bundle. **RFC 9421 signature required.** Content-addressed: `{digest}` must match the server's bundle.

```
→ Response 200: tar.gz archive (Cache-Control: immutable)
→ Response 304: not modified
→ Response 404: digest not found
```

### 7. GET /api/v1/clients/{clientId}/deployments/{deploymentId}/{digest}

Download a single deployment YAML. **RFC 9421 signature required.**

```
→ Response 200: YAML file (Cache-Control: immutable)
→ Response 304: not modified
→ Response 404: not found
```

### 8. POST /api/v1/clients/{clientId}/deployments/{deploymentId}/status

Report deployment status. **RFC 9421 signature + Content-Digest required.**

```json
→ Request body:
{
  "apiVersion": "deployment.margo.org/v1alpha1",
  "kind": "DeploymentStatusManifest",
  "deploymentId": "<uuid>",
  "status": { "state": "installed" },
  "components": [
    { "name": "app-component", "state": "installed" }
  ]
}

→ Response 200:
{ "acknowledgement": "received" }

→ Response 422: invalid state or missing fields
```

**Valid `status.state` values:** `"pending"`, `"installing"`, `"installed"`, `"failed"`, `"removing"`, `"removed"`

---

## RFC 9421 HTTP Signatures

All write endpoints (POST/PUT capabilities, POST status) and all authenticated GET endpoints require HTTP Message Signatures per [RFC 9421](https://www.rfc-editor.org/rfc/rfc9421).

**How the mock server verifies:**
1. Checks `Content-Digest` header matches SHA-256 of the request body → 400 if mismatch
2. Verifies `Signature` header using the device's public key from the onboarding certificate → 401 if invalid

**What your device-agent must sign:** `@method`, `@target-uri`, and `content-digest` (for body requests).

**Key format:** ECDSA P-256 in IEEE P1363 format (not DER).

---

## Validation Rules Reference (assertions.json)

The mock server loads `manifests/assertions.json` at startup. All validation is data-driven — change the file and restart the server to change what gets validated. No Go code changes needed.

### Rule Structure

```json
{
  "rule_id": "capabilities-008",
  "field": "properties.roles",
  "type": "array",
  "required": true,
  "minItems": 1,
  "itemsType": "string",
  "itemsEnum": ["Standalone Cluster", "Cluster Leader", "Standalone Device"],
  "description": "properties.roles must contain at least one valid Margo device role"
}
```

| Field | Meaning |
|-------|---------|
| `rule_id` | Unique ID. Returned in error responses so you know which rule failed |
| `field` | JSON path to the field (dot notation; `*` wildcard for array items) |
| `type` | `"string"`, `"number"`, `"object"`, or `"array"` |
| `required` | If `true`, request is rejected when field is absent |
| `value` | Field must exactly equal this value |
| `enum` | Field value must be one of these strings |
| `minLength` | For strings: minimum character count |
| `minItems` | For arrays: minimum item count |
| `itemsType` | For arrays: each item must be this type |
| `itemsEnum` | For arrays: each item must be one of these values |

### Error Response Formats

| Scenario | Status | Body |
|----------|--------|------|
| Missing/invalid field (onboarding) | 400 | `{ "error": "field is required" }` |
| Schema validation failure (capabilities, status) | 422 | `{ "status": "validation_failed", "errors": [{ "rule_id": "...", "error": "..." }] }` |
| Invalid/missing signature | 401 | `{ "error": "Signature verification failed" }` |
| Body tampered (digest mismatch) | 400 | `{ "error": "content-digest header missing or invalid" }` |
| Blocklisted certificate | 403 | `{ "error": "Client rejected: ..." }` |
| Unknown client ID | 404 | `{ "error": "Client not found" }` |

### How to Add a Validation Rule

1. Open `manifests/assertions.json`
2. Find the endpoint block (e.g., `POST_capabilities`)
3. Add your rule to the `validations` array:

```json
{
  "rule_id": "capabilities-019",
  "field": "properties.firmwareVersion",
  "type": "string",
  "required": true,
  "minLength": 1,
  "description": "properties.firmwareVersion is required"
}
```

4. Restart the server (rules are loaded at startup):
```bash
make kill-server
make run-server
```

---

## Environment Variables

### Mock Server (`bin/server`)

| Variable | Default | Purpose |
|----------|---------|---------|
| `KEEP_DATA` | not set | Set to `true` to preserve `data/clients.json` and `data/deployments.json` between restarts |

```bash
KEEP_DATA=true ./bin/server
```

### Mock Device-Agent (`bin/run_tests`)

| Variable | Default | Purpose |
|----------|---------|---------|
| `DEVICE_PRIVATE_KEY_PATH` | `./certs/device-key.pem` | Path to device ECDSA private key |
| `DEVICE_CERTIFICATE_PATH` | `./certs/device-cert.pem` | Path to device certificate |

```bash
DEVICE_PRIVATE_KEY_PATH=~/certs/device-ecdsa.key \
DEVICE_CERTIFICATE_PATH=~/certs/device-ecdsa.crt \
./bin/run_tests
```

---

## Validate Your JSON File

Before submitting your `test-scenarios.json` to the Data-Generator, validate it:

```bash
# Check syntax
jq empty my-test-scenarios.json && echo "Valid JSON"

# Check structure — should print an array of scenario names
jq '.[].name' my-test-scenarios.json

# Count scenarios and total steps
jq '[.[] | .steps | length] | add' my-test-scenarios.json

# List all step IDs
jq '[.[].steps[].id]' my-test-scenarios.json
```

---

## Troubleshooting

**"Server not responding" or connection refused**
```bash
# Check if server is running
ps aux | grep bin/server
# Or restart:
make kill-server && make run-server
# Check server log:
tail -50 /tmp/wfm-server.log
```

**404 on POST /capabilities — "Client not found"**
- Your device-agent stored a `clientId` from a previous WFM session and skipped onboarding
- The mock WFM starts fresh on every run and has no record of that `clientId`
- Fix: stop the device-agent, clear its persisted state, then restart:
  ```bash
  cd ~/sandbox/docker-compose
  docker compose down
  rm -rf data/
  WFM_PORT=3001 sudo -E bash /path/to/device-agent.sh docker start-docker
  ```

**401 Unauthorized on POST /capabilities or POST /status**
- Your device-agent's signing key does not match the certificate submitted at onboarding
- The mock server stores the public key from `POST /onboarding` and uses it to verify signatures
- Ensure the same key pair is used for both

**400 Bad Request: "content-digest header missing or invalid"**
- Your device-agent must compute `SHA-256` of the request body and set the `Content-Digest` header:  
  `Content-Digest: sha-256=:<base64-encoded-hash>:`

**422 Validation errors**
- The response body contains `"errors": [{ "rule_id": "...", "error": "..." }]`
- Cross-reference the `rule_id` against `manifests/assertions.json` to see which field failed

**403 Forbidden: "Client rejected"**
- The certificate submitted at onboarding is in the `rejected_certificates` blocklist in `assertions.json`
- Use a different certificate

**Reports directory not found**
- Run from the `device-supplier/` directory: `cd conformance/device-supplier`
- The `reports/` directory is created automatically on first test run

**Go build permission errors on module cache**
```bash
# Use a custom cache directory:
GOPATH=/tmp/go-build GOCACHE=/tmp/go-cache make build
```

**jq: invalid JSON**
```bash
# Find exactly where the error is:
python3 -m json.tool my-test-scenarios.json
```

---

## End-to-End Workflow Summary

```
1. CREATE TEST SCENARIOS
   └── Draft my-test-scenarios.json following the template above
   └── Validate: jq empty my-test-scenarios.json

2. REGISTER WITH DATA-GENERATOR
   └── cd conformance/
   └── bash conformance.sh
   └── Select Device Supplier → create/select group → point to your file
   └── Result: Data-Generator/device-supplier/groups/<name>/

3a. RUN VIA RUNNER CLI (uses mock device-agent)
   └── bash run-tests.sh
   └── Select Device Supplier → Group-based → your group
   └── Mock server starts → mock device-agent runs your scenarios → report generated

3b. RUN WITH YOUR REAL DEVICE-AGENT
   └── cd conformance/device-supplier/
   └── make run-server
   └── Copy certs/ca-cert.pem to your device-agent's trusted CA location
   └── Generate device certs: sudo -E bash ~/test/sandbox/scripts/device-agent.sh → option 12
   └── Configure device-agent WFM_HOST=localhost, WFM_PORT=3001
   └── Start device-agent → watch /tmp/wfm-server.log for results
   └── make kill-server when done
```

---

*Last updated: June 2026 | Margo Management Interface API v1.0.0 | Device Supplier Conformance Suite*
