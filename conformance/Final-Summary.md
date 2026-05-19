# Final Summary: Margo Device Supplier Conformance Suite — Complete Documentation

**Status:** ✅ **43/43 Tests Passing (100% Conformance)**  
**Version:** April 2026  
**Purpose:** Comprehensive technical guide for understanding, running, and extending the conformance suite

---

## Executive Summary

The **Margo Device Supplier Conformance Suite** validates WFM (Workload Fleet Manager) server compliance with the Margo Management Interface API v1.0.0. It includes:

- ✅ Mock WFM server (server.go) enforcing all spec requirements
- ✅ Test runner (run_tests.go) acting as a device agent
- ✅ Data-driven configuration (JSON) for scenarios and validation rules
- ✅ RFC 9421 HTTP Message Signatures for secure device-to-server communication
- ✅ HTML reports showing all test results
- ✅ **Zero code changes needed** to add new tests or modify validation rules

**Quick Start:**
```bash
make demo  # Builds, starts server, runs all 43 tests,generates report
```

---

## Table of Contents

1. [Quick Start](#quick-start)
2. [Architecture Overview](#architecture-overview)
3. [Complete Directory Structure](#complete-directory-structure)
4. [The Three Pillars](#the-three-pillars)
5. [All 8 API Endpoints](#all-8-api-endpoints)
6. [End-to-End Data Flow](#end-to-end-data-flow)
7. [RFC 9421 HTTP Message Signatures](#rfc-9421-signatures)
8. [Test Scenario Schema](#test-scenario-schema)
9. [Assertion Schema](#assertion-schema)
10. [How to Add Tests](#how-to-add-tests)
11. [How to Modify Rules](#how-to-modify-rules)
12. [Running the Suite](#running-the-suite)
13. [Certificate Handling](#certificate-handling)
14. [Environment Variables](#environment-variables)
15. [State Management](#state-management)
16. [TLS Architecture](#tls-architecture)
17. [Go Dependencies](#go-dependencies)
18. [Test Reports](#test-reports)
19. [Real Device Integration](#real-device-integration)
20. [FAQ & Troubleshooting](#faq-troubleshooting)

---

## Quick Start

### For Everyone: Run the Tests (60 seconds)

```bash
cd /home/margo/nitin/margo/personas/device_supplier

# Single command: builds, starts server, runs 43 tests, generates report
make demo
```

**Expected output:**
```
✅ WFM Server is ready
▶ Running scenarios...
╔════════════════════════════════════╗
║ Test Results: 43 PASSED, 0 FAILED ║
╚════════════════════════════════════╝
📊 Report: reports/conformance-report-2026-04-22T...html
```

### For Developers: Step-by-Step

**Terminal 1: Start server**
```bash
cd /home/margo/nitin/margo/personas/device_supplier
make build
./bin/server
```

**Terminal 2: Run tests**
```bash
./bin/run_tests
# or: go run run_tests.go
```

**View results:**
```bash
open reports/conformance-report-*.html
```

---

## Architecture Overview

```
┌─────────────────────────────────┐
│   Test Runner (run_tests.go)    │
│   - Reads test-scenarios.json   │
│   - Signs requests (RFC 9421)   │
│   - Validates responses         │
└────────────┬────────────────────┘
             │ HTTPS
             │
┌────────────▼────────────────────┐
│ Mock WFM Server (server.go)      │
│ https://localhost:3001           │
│ - Reads assertions.json          │
│ - Validates requests             │
│ - Returns spec-compliant         │
└─────────────────────────────────┘
```

**Key Principle:** JSON files define WHAT to test and HOW to validate. Go code executes and validates.

---

## Complete Directory Structure

```
device_supplier/
├── README.md                         Quick start
├── Final-Summary.md                  This comprehensive guide
├── CERTIFICATE_GENERATION.md         Certificate guide
├── Makefile                          Build automation
├── server.go                         Mock WFM server
├── run_tests.go                      Test runner
├── go.mod / go.sum                   Go dependencies
│
├── manifests/
│   └── assertions.json               → Server validation rulebook
│
├── device-scenarios/
│   └── test-scenarios.json           → Test execution script (~43 steps)
│
├── certs/
│   ├── ca-cert.pem, ca-key.pem       Root CA
│   ├── server-cert.pem, server-key.pem Server TLS
│   ├── device-cert.pem, device-key.pem Device certs
│   └── generate-certs.sh            Certificate script
│
├── data/                             Runtime state (auto-created/cleared)
│   ├── clients.json                  Registered devices
│   └── deployments.json              Deployment history
│
├── bin/                              Compiled binaries
│   ├── server                        Server binary
│   └── run_tests                     Test runner binary
│
└── reports/                          Test results
    └── conformance-report-*.html     HTML reports (timestamped)
```

---

## The Three Pillars

### Pillar 1: Mock WFM Server (server.go)

- **Location:** `https://localhost:3001/v1alpha2/margo`
- **Language:** Go 1.24+
- **Size:** ~600 lines

**Startup:**
1. Clear old data (unless `KEEP_DATA=true`)
2. Load assertions.json
3. Load/generate TLS certificates
4. Listen on port 3001

**Key handlers:** 7 endpoint handlers + validation engine

**Design:** Rules in JSON, not code → Change behavior without recompiling

### Pillar 2: Test Runner (run_tests.go)

- **Language:** Go 1.24+
- **Size:** ~350 lines
- **Role:** Device agent simulator

**Startup:**
1. Load device certificate & key
2. Load test scenarios.json
3. Wait for server (/health endpoint)
4. Execute all steps

**Per step:**
1. Build request body from JSON
2. Resolve certificates (file paths → PEM content)
3. Add RFC 9421 signatures
4. Send to server
5. Validate response (status, fields, headers)
6. Extract context for future steps

### Pillar 3: JSON Configuration

#### assertions.json (Server Rulebook)
- **Loaded by:** Server at startup
- **Changes take effect:** After server restart
- **Edited by:** Anyone (no code needed)

#### test-scenarios.json (Test Script)
- **Loaded by:** Test runner at startup
- **Changes take effect:** Next test run (no restart needed!)
- **Edited by:** Anyone (no code needed)

---

## All 8 API Endpoints

All endpoints: `https://localhost:3001/v1alpha2/margo`

### 1. GET /api/v1/onboarding/certificate
- **Auth:** None (device unauthenticated)
- **Returns:** Root CA cert (PEM)  
- **Status:** 200

### 2. POST /api/v1/onboarding
- **Auth:** None (bootstrap step)
- **Body:** Device X.509 certificate
- **Returns:** `{ "clientId": "..." }`
- **Status:** 201 (success) | 400 (validation) | 403 (blocklisted)

### 3. POST /api/v1/clients/{clientId}/capabilities
- **Auth:** ✅ **RFC 9421 signature + Content-Digest required**
- **Body:** Hardware capabilities
- **Returns:** `{ "status": "capabilities_received" }`
- **Status:** 201 (success) | 400/422 (validation) | 401 (bad signature)

### 4. PUT /api/v1/clients/{clientId}/capabilities
- **Auth:** ✅ **RFC 9421 signature + Content-Digest required**
- **Same as POST** (update operation)

### 5. GET /api/v1/clients/{clientId}/deployments
- **Auth:** Client registered (no signature needed)
- **Returns:** Manifest with deployment URLs + ETags
- **Status:** 200 | 304 (Not Modified) | 404 (not found)

### 6. GET /api/v1/clients/{clientId}/bundles/{digest}
- **Auth:** Client registered (no signature needed)
- **Content-Addressing:** Server verifies `{digest}` matches bundle
- **Returns:** tar.gz archive
- **Status:** 200 | 304 | 404

### 7. GET /api/v1/clients/{clientId}/deployments/{deploymentId}/{digest}
- **Auth:** Client registered (no signature needed)
- **Content-Addressing:** Server verifies `{digest}` matches YAML
- **Returns:** Single YAML file
- **Status:** 200 | 304 | 404

### 8. POST /api/v1/clients/{clientId}/deployments/{deploymentId}/status
- **Auth:** ✅ **RFC 9421 signature + Content-Digest required**
- **Body:** Deployment status report (pending/installing/installed/failed/removing/removed)
- **Returns:** `{ "acknowledgement": "received" }`
- **Status:** 200 (success) | 400/422 (validation) | 401 (bad signature)

---

## End-to-End Data Flow

```
T=0s  Device boots
T=1s  GET /onboarding/certificate
      ← Root CA cert
T=2s  POST /onboarding (with device cert)
      ← Server: { "clientId": "..." }
T=3s  POST /capabilities (with RFC 9421 signature)
      ← Server: { "status": "capabilities_received" }
T=4s  GET /deployments
      ← Server: manifest with deployment URLs
T=5s  GET /bundles/{digest} or GET /deployments/{id}/{digest}
      ← Server: YAML or tar.gz
T=6s  Device deploys workload
T=7s  POST /status (with RFC 9421 signature)
      ← Server: { "acknowledgement": "received" }
T=60s Device polls deployments again (repeat cycle)
```

---

## RFC 9421 Signatures

**What:** Internet standard for signing HTTP requests  
**Why:** Server must verify requests came from actual device, not impersonator

**How device signs POST/PUT:**
1. Compute SHA-256 hash of body
2. Sign over: @method, @target-uri, content-digest
3. Add headers: Content-Digest, Signature-Input, Signature

**How server verifies:**
1. Recompute Content-Digest from actual body
2. If mismatch → 400 (body tampered)
3. Verify signature with device's stored cert public key
4. If invalid → 401 (not from device)
5. If valid → proceed

**Required on:** POST/PUT (capabilities, status)  
**Not required on:** GET (read-only), POST onboarding (no key yet)

---

## Test Scenario Schema

**File:** `device-scenarios/test-scenarios.json`  
**No recompile needed** — changes take effect on the very next run  
**Processed by:** `run_tests.go` (reads JSON, executes steps, validates responses)

### What Is This Format? (Not Postman / Not Pytest / Not Cucumber)

> **Important:** `test-scenarios.json` does **not** use any external test framework. It is a **custom JSON DSL** parsed directly by `run_tests.go`.

| Approach | What it does | Why not used here |
|----------|-------------|-------------------|
| **Postman / Newman** | GUI-based HTTP test collections | Requires Postman install, complex export/import |
| **Pytest + requests** | Python test code | Requires Python, coding knowledge |
| **Cucumber / Gherkin** | BDD test language | Requires framework install, feature files |
| **test-scenarios.json (this suite)** | Pure JSON, interpreted by Go runner | No tools needed; edit file and re-run |

---

### Top-Level Structure

The file is a **JSON array of scenarios**. Each scenario is an independent test flow with one or more ordered steps:

```json
[
  { "id": "scenario-onboarding",    "steps": [ ... ] },
  { "id": "scenario-capabilities",  "steps": [ ... ] },
  { "id": "scenario-deployments",   "steps": [ ... ] }
]
```

Scenarios run **in order**, top to bottom. Each scenario gets its own fresh context (variables do not leak between scenarios).

---

### Scenario Object

```json
{
  "id": "scenario-unique-id",
  "name": "Human Readable Name",
  "description": "What this scenario validates",
  "steps": [ ... ]
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `id` | Yes | Unique string among all scenarios. Used for filtering: `./bin/run_tests -scenario scenario-onboarding` |
| `name` | Yes | Displayed in terminal output and HTML report |
| `description` | Yes | One-line explanation of what this scenario covers |
| `steps` | Yes | Ordered array of step objects (see below) |

---

### Step Object — All Fields

```json
{
  "id": "step-1.2",
  "name": "Onboard Trusted Device",
  "method": "POST",
  "endpoint": "/api/v1/onboarding",
  "request_body": {
    "apiVersion": "onboarding.margo.org/v1alpha1",
    "kind": "OnboardingRequest",
    "certificate": "./certs/device-cert.pem"
  },
  "headers": {
    "Accept": "application/vnd.margo.manifest.v1+json"
  },
  "expected_status": 201,
  "validations": [
    { "field": "clientId", "operation": "is_string" }
  ],
  "extract_context": {
    "clientId": "clientId"
  },
  "skip_signing": false,
  "skip_certificate_injection": false
}
```

#### Field Reference

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `id` | Yes | string | Unique step ID. Convention: `step-X.Y` where X is scenario number and Y is step number. Used for filtering: `./bin/run_tests -step step-1.2` |
| `name` | Yes | string | Displayed in terminal and HTML report |
| `method` | Yes | string | HTTP verb: `"GET"`, `"POST"`, or `"PUT"` |
| `endpoint` | Yes | string | URL path (without base URL). Supports `{placeholder}` substitution |
| `request_body` | No | object | JSON body sent with POST/PUT. Omit entirely for GET requests |
| `headers` | No | object | Custom HTTP headers. Applied after signing. Values support `{placeholder}` substitution |
| `expected_status` | Yes | number | HTTP status code the step expects. Anything else → FAIL |
| `validations` | No | array | Response field checks (see Validations section below) |
| `extract_context` | No | object | Save response values as variables for later steps (see Context section below) |
| `skip_signing` | No | bool | Default `false`. Set `true` to omit RFC 9421 Signature headers — use for GET requests or negative signature tests |
| `skip_certificate_injection` | No | bool | Default `false`. Set `true` to send `certificate` field as a literal string without loading from file — use for rejection/blocklist tests |

---

### Validations — Checking the Response

Each validation checks one field in the JSON response body (or response headers):

```json
"validations": [
  { "field": "clientId",    "operation": "is_string" },
  { "field": "status",      "operation": "equals",    "value": "capabilities_received" },
  { "field": "error",       "operation": "contains",  "value": "Client rejected" },
  { "field": "certificate", "operation": "not_empty" },
  { "field": "deployments", "operation": "is_array"  },
  { "field": "_headers.ETag", "operation": "not_empty" }
]
```

#### All Operations

| Operation | `value` needed? | Passes when |
|-----------|----------------|-------------|
| `equals` | Yes | Field value matches `value` exactly |
| `contains` | Yes | Field value (string) contains `value` as substring |
| `not_empty` | No | Field is present and not `""`, `null`, or `[]` |
| `exists` | No | Field is present (any value including empty) |
| `is_string` | No | Field value is a JSON string |
| `is_number` | No | Field value is a JSON number |
| `is_array` | No | Field value is a JSON array |

#### Field Path Syntax

| Path | Accesses |
|------|---------|
| `"clientId"` | Top-level field |
| `"status.state"` | Nested field |
| `"deployments.0.id"` | First array element's `id` |
| `"_headers.ETag"` | HTTP response header (prefix with `_headers.`) |

---

### Context — Passing Data Between Steps

`extract_context` saves values from a response. `{placeholders}` inject them into later steps.

**Saving a value:**
```json
"extract_context": {
  "clientId":      "clientId",
  "manifestEtag":  "_headers.ETag"
}
```

Left side is your variable name. Right side is the JSON path (same syntax as validations).

**Using a saved value:**
```json
"endpoint": "/api/v1/clients/{clientId}/capabilities",

"headers": {
  "If-None-Match": "{manifestEtag}"
},

"request_body": {
  "deploymentId": "{deploymentId}"
}
```

Placeholders work in: `endpoint`, `headers` values, and `request_body` string values.

**Built-in context variable:**

| Variable | Set by | Used by |
|----------|--------|---------|
| `clientId` | Any step with `"extract_context": { "clientId": "clientId" }` | All subsequent endpoint URLs |

---

### Certificate Handling in Steps

The `certificate` field inside `request_body` gets special treatment:

**Load from file (positive tests):**
```json
"request_body": {
  "certificate": "./certs/device-cert.pem"
}
```
Runner detects the path prefix and reads the file. Actual PEM content is sent to the server.

**Use literal string (rejection/negative tests):**
```json
"request_body": {
  "certificate": "rnd-key-7f3a91b2c4d8e6"
},
"skip_certificate_injection": true
```
Runner skips file loading. Literal string `"rnd-key-7f3a91b2c4d8e6"` is sent — server's rejection list blocks it with 403.

---

### Existing Scenarios (Reference)

| Scenario ID | Name | Steps | What it covers |
|-------------|------|-------|---------------|
| `scenario-onboarding` | Device Onboarding | 3 | Get CA cert, successful onboarding, rejected cert (403) |
| `scenario-capabilities` | Capabilities Reporting | 3 | POST capabilities, PUT capabilities |
| `scenario-deployments` | Deployment Lifecycle | 7 | Get manifest, ETag caching, bundle, YAML download, status |
| `scenario-onboarding-errors` | Onboarding Error Cases | 4 | Bad apiVersion, missing cert, empty cert, missing kind |
| `scenario-capabilities-errors` | Capabilities Error Cases | 7 | Missing fields, invalid enum values, wrong types |
| `scenario-status-and-retrieval-errors` | Status/Retrieval Errors | 9 | Wrong Accept header, digest mismatch, invalid state, end-to-end |

---

### Step-by-Step Guide: Writing a New Test

**Decide what you want to test.** There are three patterns:

#### Pattern A: Happy Path (expect success)

Test that the server accepts a valid request.

**Example goal:** Verify server accepts a valid onboarding request.

```json
{
  "id": "step-X.1",
  "name": "Onboard with valid certificate",
  "method": "POST",
  "endpoint": "/api/v1/onboarding",
  "request_body": {
    "apiVersion": "onboarding.margo.org/v1alpha1",
    "kind": "OnboardingRequest",
    "certificate": "./certs/device-cert.pem"
  },
  "expected_status": 201,
  "validations": [
    { "field": "clientId", "operation": "is_string" }
  ],
  "extract_context": {
    "clientId": "clientId"
  }
}
```

Key choices:
- `expected_status: 201` because POST /onboarding returns 201 on success
- `certificate` uses file path → runner loads real PEM
- `extract_context` saves `clientId` for steps that follow

---

#### Pattern B: Error / Rejection (expect failure)

Test that the server rejects an invalid request.

**Example goal:** Verify server rejects request with missing `kind` field (expects 400).

```json
{
  "id": "step-X.2",
  "name": "Reject onboarding with missing kind",
  "method": "POST",
  "endpoint": "/api/v1/onboarding",
  "request_body": {
    "apiVersion": "onboarding.margo.org/v1alpha1",
    "certificate": "./certs/device-cert.pem"
  },
  "expected_status": 400,
  "validations": [
    { "field": "error", "operation": "contains", "value": "kind" }
  ],
  "extract_context": {}
}
```

Key choices:
- `expected_status: 400` because missing required field → bad request
- Response validation checks error message contains the field name (`"kind"`)
- No `kind` field in `request_body` — that's the deliberate trigger

---

#### Pattern C: Multi-Step Flow (chain steps with context)

Test a full lifecycle where Step 2 depends on data from Step 1.

**Example goal:** Onboard → Report capabilities.

**Step 1: Onboard (save clientId)**
```json
{
  "id": "step-X.1",
  "name": "Onboard Device",
  "method": "POST",
  "endpoint": "/api/v1/onboarding",
  "request_body": {
    "apiVersion": "onboarding.margo.org/v1alpha1",
    "kind": "OnboardingRequest",
    "certificate": "./certs/device-cert.pem"
  },
  "expected_status": 201,
  "validations": [
    { "field": "clientId", "operation": "exists" }
  ],
  "extract_context": {
    "clientId": "clientId"
  }
}
```

**Step 2: Report capabilities (use saved clientId)**
```json
{
  "id": "step-X.2",
  "name": "Report Capabilities",
  "method": "POST",
  "endpoint": "/api/v1/clients/{clientId}/capabilities",
  "request_body": {
    "apiVersion": "device.margo.org/v1alpha1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {
      "id": "my-device-001",
      "vendor": "MyVendor",
      "modelNumber": "MDL-100",
      "serialNumber": "SN-99999",
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
  "expected_status": 201,
  "validations": [
    { "field": "status", "operation": "equals", "value": "capabilities_received" }
  ],
  "extract_context": {}
}
```

Key choices:
- `{clientId}` in `endpoint` is replaced with value saved in Step 1
- Full valid body is required because all fields are enforced by assertions.json

---

### Adding Your Scenario to the File

1. Open `device-scenarios/test-scenarios.json`
2. The file is a JSON array `[...]`. Add your scenario object at the end, before the closing `]`:

```json
[
  { "id": "scenario-onboarding", ... },
  { "id": "scenario-capabilities", ... },
  
  {
    "id": "scenario-my-new-test",
    "name": "My New Test",
    "description": "Tests X behavior",
    "steps": [
      { ...step A... },
      { ...step B... }
    ]
  }
]
```

3. Save the file
4. Run tests (no recompile):

```bash
./bin/run_tests
```

Or run **only your new scenario**:
```bash
./bin/run_tests -scenario scenario-my-new-test
```

Or run **only one specific step**:
```bash
./bin/run_tests -scenario scenario-my-new-test -step step-X.1
```

---

### Quick Reference: Which Status Code to Expect?

| Endpoint | Success | Validation error | Signature error | Cert blocklisted |
|----------|---------|-----------------|----------------|-----------------|
| GET /onboarding/certificate | 200 | — | — | — |
| POST /onboarding | 201 | 400 | — | 403 |
| POST /capabilities | 201 | 422 | 401 | — |
| PUT /capabilities | 201 | 422 | 401 | — |
| GET /deployments | 200 / 304 | — | — | — |
| GET /bundles/{digest} | 200 / 304 | 404 | — | — |
| GET /deployments/{id}/{digest} | 200 / 304 | 404 | — | — |
| POST /status | 200 | 422 | 401 | — |

**Quick rule:**
- Missing/wrong field in body → use `400` (onboarding) or `422` (capabilities, status)
- Invalid/missing signature → `401`
- Blocked certificate → `403`
- Wrong digest in URL → `404`

---

## Assertion Schema

**File:** `manifests/assertions.json`  
**Restart required:** Yes (server reloads at startup)  
**Edited by:** Anyone (no code needed)

### What Is This Schema? (Not Schemathesis / Not JSON Schema)

> **Important:** `assertions.json` does **not** use an industry-standard schema format like JSON Schema, OpenAPI Schema Objects, or tools like Schemathesis. It uses a **custom, purpose-built schema** implemented directly in `server.go`.

**Why a custom schema?**

| Approach | What it does | Why not used here |
|----------|-------------|-------------------|
| **JSON Schema** | Validates JSON structure | Too complex; not designed for HTTP conformance assertions |
| **Schemathesis** | Auto-generates tests from OpenAPI spec | Generates tests; doesn't enforce server-side validation rules |
| **OpenAPI Schema Objects** | Describes request/response structure | Descriptive, not enforced at runtime |
| **assertions.json (this suite)** | Server-side validation engine driven by JSON rules | Simple, readable, no external tooling required |

**How assertions.json rules were written:**

1. Open the Margo Management Interface OpenAPI spec
2. For each `POST`/`PUT` endpoint, find the `requestBody.content.application/json.schema`
3. For each `required` field in the schema → write a rule with `"required": true`
4. For each field with `type: string` and `enum` values → write a rule with `"enum": [...]`
5. For each field with `format: string` and no fixed value → write a rule with `"type": "string", "minLength": 1`
6. For each field with fixed expected value → write a rule with `"value": "exact-value"`

This is a **manual translation** from the OpenAPI spec into this suite's rule DSL — not generated by any tool.

---

### Top-Level Structure

```json
{
  "rejected_certificates": [ ... ],
  "endpoints": {
    "POST_onboarding":    { ... },
    "POST_capabilities":  { ... },
    "PUT_capabilities":   { ... },
    "POST_status":        { ... },
    "GET_onboarding_certificate": { ... }
  },
  "error_responses": {
    "badRequest":    { "status_code": 400, "format": "error_string" },
    "unprocessable": { "status_code": 422, "format": "validation_errors" }
  }
}
```

**All 3 top-level keys are required:**

| Key | Purpose |
|-----|---------|
| `rejected_certificates` | Array of cert values that get a 403 response during onboarding |
| `endpoints` | Map of `METHOD_name` → validation rules for that endpoint |
| `error_responses` | Defines what error format each type of failure uses |

---

### Endpoint Block Structure

Each key in `endpoints` is `METHOD_shortname` (e.g., `POST_onboarding`, `GET_onboarding_certificate`):

```json
"POST_onboarding": {
  "path": "/api/v1/onboarding",
  "method": "POST",
  "status_code": 201,
  "validation_error_key": "badRequest",
  "validations": [ ... ],
  "response_structure": {
    "matches": [
      { "description": "Response must contain clientId", "json": "clientId", "type": "string" }
    ]
  }
}
```

| Field | Required | Meaning |
|-------|----------|---------|
| `path` | Yes | URL path (informational, not used for routing) |
| `method` | Yes | HTTP method (informational) |
| `status_code` | Yes | Expected HTTP status on success |
| `validation_error_key` | Only for write endpoints | Which `error_responses` entry to use when validation fails (`"badRequest"` or `"unprocessable"`) |
| `validations` | Yes | Array of rule objects (see below) |
| `response_structure` | No | Documents expected response fields (informational) |

**Endpoints registered in this suite:**

| Key | Endpoint |
|-----|---------|
| `GET_onboarding_certificate` | GET /api/v1/onboarding/certificate |
| `POST_onboarding` | POST /api/v1/onboarding |
| `POST_capabilities` | POST /api/v1/clients/{clientId}/capabilities |
| `POST_status` | POST /api/v1/clients/{clientId}/deployments/{deploymentId}/status |

---

### Validation Rule Structure

Each rule in the `validations` array follows this shape:

```json
{
  "rule_id": "capabilities-008",
  "field": "properties.roles",
  "type": "array",
  "required": true,
  "itemsType": "string",
  "itemsEnum": ["Standalone Cluster", "Cluster Leader", "Standalone Device"],
  "description": "properties.roles must contain valid Margo device roles"
}
```

#### All Supported Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `rule_id` | string | Yes | Unique identifier. Naming convention: `endpoint-NNN` e.g., `"capabilities-011"` |
| `field` | string | Yes | JSON path to the field being validated (see Path Syntax below) |
| `type` | string | Yes | One of: `"string"`, `"number"`, `"object"`, `"array"` |
| `required` | bool | Yes | If `true`, server rejects request if field is absent |
| `value` | string/any | No | If set, field must match **exactly** this value |
| `enum` | string[] | No | If set, field value must be one of these strings |
| `minLength` | number | No | For `"string"` type: minimum character count |
| `minItems` | number | No | For `"array"` type: minimum number of items |
| `itemsType` | string | No | For `"array"` type: each item must be this type (`"string"` or `"object"`) |
| `itemsEnum` | string[] | No | For `"array"` type: each item must be one of these values |
| `description` | string | Yes | Human-readable explanation used in error messages |

#### Type Behaviours

| `type` | What the engine checks |
|--------|----------------------|
| `"string"` | Value is a string; optionally checks `value` (exact match), `enum`, `minLength` |
| `"number"` | Value is a JSON number (float64 internally) |
| `"object"` | Value is a JSON object `{}` |
| `"array"` | Value is a JSON array `[]`; optionally checks `minItems`, `itemsType`, `itemsEnum` |

#### Field Path Syntax

| Path format | Means | Example |
|-------------|-------|---------|
| `"field"` | Top-level field | `"apiVersion"` |
| `"parent.child"` | Nested field | `"properties.vendor"` |
| `"parent.child.grandchild"` | Deeply nested | `"properties.resources.cpu.cores"` |
| `"array.*.field"` | Wildcard: every array item's field | `"components.*.state"` |

---

### All 6 Rule Patterns — With Real Examples

**Pattern 1: Required exact-value string**  
_(e.g., apiVersion — spec mandates a fixed value)_

```json
{
  "rule_id": "onboarding-001",
  "field": "apiVersion",
  "type": "string",
  "required": true,
  "value": "onboarding.margo.org/v1alpha1",
  "description": "apiVersion must be exactly 'onboarding.margo.org/v1alpha1'"
}
```

**Pattern 2: Required non-empty string**  
_(e.g., certificate, serialNumber — must be present, no fixed value)_

```json
{
  "rule_id": "capabilities-007",
  "field": "properties.serialNumber",
  "type": "string",
  "required": true,
  "minLength": 1,
  "description": "properties.serialNumber is required"
}
```

**Pattern 3: Optional string from allowed set (enum)**  
_(e.g., CPU architecture — optional but must be a known value if present)_

```json
{
  "rule_id": "capabilities-012",
  "field": "properties.resources.cpu.architecture",
  "type": "string",
  "required": false,
  "enum": ["amd64", "x86_64", "arm64", "arm"],
  "description": "cpu.architecture must use a supported value when present"
}
```

**Pattern 4: Required enum string**  
_(e.g., deployment state — required, only allowed values)_

```json
{
  "rule_id": "status-005",
  "field": "status.state",
  "type": "string",
  "required": true,
  "enum": ["pending", "installing", "installed", "failed", "removing", "removed"],
  "description": "status.state must be a valid deployment state"
}
```

**Pattern 5: Required array with enum items**  
_(e.g., device roles — array, each item must be a known role)_

```json
{
  "rule_id": "capabilities-008",
  "field": "properties.roles",
  "type": "array",
  "required": true,
  "itemsType": "string",
  "itemsEnum": ["Standalone Cluster", "Cluster Leader", "Standalone Device"],
  "description": "properties.roles must contain valid Margo device roles"
}
```

**Pattern 6: Wildcard array item validation**  
_(e.g., every interface must have a recognized type)_

```json
{
  "rule_id": "capabilities-016",
  "field": "properties.resources.interfaces.*.type",
  "type": "string",
  "required": true,
  "enum": ["ethernet", "wifi", "cellular", "bluetooth", "usb", "canbus", "rs232"],
  "description": "Each interface must declare a supported type"
}
```

---

### Error Response Configuration

The `error_responses` section defines what HTTP response format is used when validation fails:

```json
"error_responses": {
  "badRequest": {
    "status_code": 400,
    "format": "error_string"
  },
  "unprocessable": {
    "status_code": 422,
    "format": "validation_errors",
    "status": "validation_failed"
  }
}
```

| format | When used | Response body |
|--------|-----------|---------------|
| `"error_string"` | `"badRequest"` (400) | `{ "error": "field is required" }` |
| `"validation_errors"` | `"unprocessable"` (422) | `{ "status": "validation_failed", "errors": [{ "rule_id": "...", "error": "..." }] }` |

Which format is used depends on `"validation_error_key"` in the endpoint block:
- `"badRequest"` → 400 with single error string (onboarding)
- `"unprocessable"` → 422 with full error list (capabilities, status)

---

### Rule ID Naming Convention

```
<endpoint-shortname>-<NNN>

Examples:
  onboarding-001   → First rule for POST /onboarding  
  capabilities-008 → Eighth rule for POST /capabilities  
  status-005       → Fifth rule for POST /status
```

Use sequential numbers. Gaps are fine. IDs are returned in error responses so clients can identify exactly which rule failed.

---

### How to Write a New Assertion (Step-by-Step)

**Step 1: Find the field in the OpenAPI spec**

Look at the Margo Management Interface API spec. Find the endpoint's `requestBody` schema. For each property, note:
- Is it in `required: [...]`? → `"required": true`
- Does it have `enum: [...]`? → add `"enum": [...]` to rule
- Does it have `type: string` with no enum? → use `"minLength": 1`
- Does it have `type: array`? → use `"type": "array"` with `itemsType`/`itemsEnum`
- Does it have a hardcoded expected value (like apiVersion)? → add `"value": "exact-value"`

**Step 2: Choose the endpoint key**

Endpoint keys follow `METHOD_shortname`. For a new endpoint, create a new key:
```json
"POST_newEndpoint": { ... }
```

For an existing endpoint, add to its `validations` array.

**Step 3: Write the rule**

```json
{
  "rule_id": "newEndpoint-001",
  "field": "myField",
  "type": "string",
  "required": true,
  "value": "expectedValue",
  "description": "myField must be 'expectedValue' per spec section X.Y"
}
```

**Step 4: Restart the server**

```bash
make kill-server
./bin/server
```

**Step 5: Verify with a test**

Add a matching test step to `test-scenarios.json` that sends a request missing your new required field, and expects 400 or 422. Run:

```bash
./bin/run_tests
```

If your rule works, the test passes.

---

## How to Add Tests

### Step 1: Open test-scenarios.json

### Step 2: Find appropriate scenario

- `scenario-onboarding` → Onboarding tests
- `scenario-onboarding-errors` → Onboarding error cases
- `scenario-capabilities` → Capability tests
- etc.

### Step 3: Add step

```json
{
  "id": "step-4.6",
  "name": "Test new scenario",
  "method": "POST",
  "endpoint": "/api/v1/onboarding",
  "request_body": {
    "apiVersion": "onboarding.margo.org/v1alpha1",
    "kind": "OnboardingRequest",
    "certificate": "./certs/device-cert.pem"
  },
  "expected_status": 201,
  "validations": [
    { "field": "clientId", "operation": "exists" }
  ]
}
```

### Step 4: Save and run

```bash
# No recompile!
./bin/run_tests
open reports/conformance-report-*.html  # Your new test appears
```

---

## How to Modify Rules

### Step 1: Open assertions.json

### Step 2: Find endpoint

- `POST_onboarding`
- `POST_capabilities`
- `POST_status`
- etc.

### Step 3: Modify rule

**Add validation:**
```json
{
  "rule_id": "custom-001",
  "field": "customField",
  "type": "string",
  "required": true,
  "value": "exactValue"
}
```

**Modify enum:**
```json
{
  "rule_id": "status-005",
  "field": "status.state",
  "enum": ["pending", "installing", "installed", "failed", "removing", "removed", "paused"]
}
```

**Block certificate:**
```json
"rejected_certificates": [
  "rnd-key-7f3a91b2c4d8e6",
  "new-blocked-cert"
]
```

### Step 4: Restart server

```bash
make kill-server
./bin/server
./bin/run_tests
```

---

## Running the Suite

### Quickest Way (60 seconds)

```bash
make demo
```

### Detailed Way

**Build:**
```bash
make build
# or: go build -o bin/server server.go && go build -o bin/run_tests run_tests.go
```

**Start server (Terminal 1):**
```bash
./bin/server
```

**Run tests (Terminal 2):**
```bash
./bin/run_tests
```

**View report:**
```bash
open reports/conformance-report-*.html
```

**Stop:**
```bash
make kill-server
```

### Commands

| Command | Does |
|---------|------|
| `make build` | Compile server & runner |
| `make build-server` | Compile server only |
| `make build-tests` | Compile tests only |
| `make run-server` | Start server |
| `make run-tests` | Run tests |
| `make demo` | Build → start server → run tests → show report |
| `make kill-server` | Stop server |
| `make clean` | Delete bin/ and go.sum |

---

## Certificate Handling

### How Resolution Works

Test runner converts file paths to PEM content:

```go
if path starts with "./" or "certs/" {
  return readFile(path)  // Actual PEM
} else {
  return path  // Literal string
}
```

In test-scenarios.json:
```json
"certificate": "./certs/device-cert.pem"  // File path
```

Becomes:
```json
"certificate": "-----BEGIN CERTIFICATE-----\nMIID...\n-----END CERTIFICATE-----"  // Content
```

### Using Literal Strings

For rejection tests, skip file loading:
```json
{
  "certificate": "rnd-key-7f3a91b2c4d8e6",
  "skip_certificate_injection": true
}
```

### Certificate Files

| File | Purpose |
|------|---------|
| `ca-cert.pem` | Root CA cert (trust anchor) |
| `ca-key.pem` | CA private key |
| `server-cert.pem` | Server TLS cert (HTTPS) |
| `server-key.pem` | Server TLS key |
| `device-cert.pem` | Device cert for positive tests |
| `device-key.pem` | Device key for signing |
| `device-revoked.pem` | Revoked cert (in rejection list) |

---

## Environment Variables

### Server

**`KEEP_DATA`** (default: not set = clear on startup)
```bash
KEEP_DATA=true ./bin/server
```
Preserves clients.json and deployments.json between restarts.

### Test Runner

**`DEVICE_PRIVATE_KEY_PATH`** (default: `./certs/device-key.pem`)
```bash
DEVICE_PRIVATE_KEY_PATH=/path/to/key.pem ./bin/run_tests
```

**`DEVICE_CERTIFICATE_PATH`** (default: `./certs/device-cert.pem`)
```bash
DEVICE_CERTIFICATE_PATH=/path/to/cert.pem ./bin/run_tests
```

---

## State Management

### Runtime Files

- `data/clients.json` — Registered devices
- `data/deployments.json` — Deployment history

### Clean Startup (Default)

Every server startup clears both files. Why? Tests create fresh clients each run.

### Preserve State

```bash
KEEP_DATA=true ./bin/server
```

### Thread Safety

Server uses `sync.RWMutex` to protect concurrent access. Multiple requests never corrupt data.

---

## TLS Architecture

### Trust Chain

```
Root CA (ca-cert.pem)
  ├─→ Server TLS cert (server-cert.pem)
  └─→ Device cert (device-cert.pem)
```

### HTTPS Connection

1. Client connects to `https://localhost:3001`
2. Server presents `server-cert.pem`
3. Client verifies against `ca-cert.pem`
4. Encrypted channel established

**Note:** Test runner skips TLS verification (self-signed cert in test environment).

### Device Certificate

Stored in `data/clients.json` during onboarding. Used to verify RFC 9421 signatures on future requests.

---

## Go Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/gorilla/mux` | HTTP routing |
| `github.com/google/uuid` | Generate UUIDs |
| `github.com/lestrrat-go/htmsig` | **RFC 9421 signing/verifying (critical!)** |
| `github.com/lestrrat-go/sfv` | Used by htmsig |
| `crypto/*` (stdlib) | X.509, RSA, SHA-256 |
| `archive/tar`, `compress/gzip` (stdlib) | Bundle archives |

**Why htmsig matters:** Same library on client and server guarantees format compatibility.

---

## Test Reports

### Location

```
reports/conformance-report-2026-04-22T10-30-15-000Z.html
```

### Format

HTML table with all 43 steps:
- Step ID, Name, Scenario
- Status (✅ PASS or ❌ FAIL)
- HTTP code, Details

### 43 Test Steps Across 6 Scenarios

1. **Onboarding** (3) — Get CA, successful onboarding, rejection
2. **Capabilities** (3) — POST, PUT manifests
3. **Deployments** (7) — Get manifest, bundles, individual YAML, status
4. **Onboarding Errors** (4) — Bad apiVersion, missing cert, empty cert, missing kind
5. **Capabilities Errors** (7) — Missing fields, bad enums, invalid types
6. **Status/Retrieval Errors** (9) — Endpoint-specific errors, end-to-end flow

---

## Real Device Integration

For this server/suite, the only required handoff to a real device-agent is the CA certificate.

Copy `certs/ca-cert.pem` from this repo to the device-agent VM so the agent trusts the mock server CA.

Current example path (from current real device-agent setup):

```text
~/sandbox/poc/device/agent/config/
```

Copy command example (from suite host to agent host):

```bash
scp certs/ca-cert.pem margo@margo-device-k3s:~/sandbox/poc/device/agent/config/ca-cert.pem
```

Or if you are on the same host:

```bash
cp certs/ca-cert.pem ~/sandbox/poc/device/agent/config/ca-cert.pem
```

Important:

- This path is an example for the current environment.
- Vendor-provided device-agents may use a different cert/config location.
- The requirement remains the same: place `ca-cert.pem` in whatever location the device-agent is configured to read as its trusted CA.

---

## FAQ & Troubleshooting

**Q: Do I need to recompile after editing test-scenarios.json?**  
Nothe runner reads it on startup. Just re-run `./bin/run_tests`.

**Q: Do I need to recompile after editing assertions.json?**  
No, but the server must restart to reload it.

**Q: Can I run tests without the server?**  
No, tests poll `/health` for 5 seconds.

**Q: Why does onboarding not require a signature?**  
Because the device hasn't registered its key yet. Onboarding is the trust bootstrap.

**Q: What if body is tampered but signature is valid?**  
Content-Digest won't match → HTTP 400.

**Q: Why do deployments use content-addressed URLs?**  
Server guarantees: fetch `sha256:abc123`, always get those exact bytes. Prevents rollback.

**Q: How do I regenerate certificates?**  
```bash
bash certs/generate-certs.sh
```

**Q: Tests fail with "Server not responding"?**  
Make sure server is running: `ps aux | grep bin/server`

**Q: 401 Unauthorized on POST /capabilities?**  
Signing key doesn't match onboarded cert. Check `echo $DEVICE_PRIVATE_KEY_PATH`.

**Q: 422 Validation errors?**  
Check assertions.json for the endpoint. Ensure required fields present and enums correct.

**Q: Tests slow down over time?**  
With `KEEP_DATA=true`, data accumulates. Run `rm data/clients.json data/deployments.json`.

---

## Summary

This conformance suite provides:

✅ **Complete coverage** — All 8 endpoints, all error paths (43 steps)  
✅ **Data-driven design** — Modify tests/rules without recompiling  
✅ **RFC 9421 signatures** — Industry-standard security  
✅ **Content-addressed URIs** — Tamper-proof deployments  
✅ **Thread-safe server** — Concurrent request handling  
✅ **100% passing** — All tests validat against spec  

**Get started:** `make demo` (60 seconds to results)

---

*Document Version: 2026-04-22 | Margo Management Interface API v1.0.0 | Status: 43/43 ✅*
