# Margo Conformance Testing — Architecture, Implementation & Troubleshooting Guide

**Last updated:** 2026-06-17  
**Spec reference:** [Margo Management Interface — workload-management-api-1.0.0](https://raw.githubusercontent.com/margo/specification/pre-draft/system-design/specification/margo-management-interface/workload-management-api-1.0.0.yaml)  
**Verified against:** Symphony WFM at `https://symphony.machine:8082/v1alpha2/margo`

---

## Table of Contents

1. [What Is This Conformance Suite?](#1-what-is-this-conformance-suite)
2. [System Architecture Overview](#2-system-architecture-overview)
3. [Two Personas Explained](#3-two-personas-explained)
4. [Directory Structure](#4-directory-structure)
5. [How to Run Tests](#5-how-to-run-tests)
6. [The Margo WFM API — Endpoint Reference](#6-the-margo-wfm-api--endpoint-reference)
7. [HTTP Message Signatures (RFC 9421) — Deep Dive](#7-http-message-signatures-rfc-9421--deep-dive)
8. [Real WFM Behavior vs Spec (Important Differences)](#8-real-wfm-behavior-vs-spec-important-differences)
9. [The Node.js Scenario Runner — How It Works](#9-the-nodejs-scenario-runner--how-it-works)
10. [Test Scenario JSON Format (Custom Adapter Format)](#10-test-scenario-json-format-custom-adapter-format)
11. [Group System — How Groups Work](#11-group-system--how-groups-work)
12. [Common Errors and How to Fix Them](#12-common-errors-and-how-to-fix-them)
13. [Certificate Lifecycle](#13-certificate-lifecycle)
14. [How the Real Device-Agent Works](#14-how-the-real-device-agent-works)
15. [Design Notes — Postman vs Custom Format](#15-design-notes--postman-vs-custom-format)
16. [Adding or Modifying Tests](#16-adding-or-modifying-tests)

---

## 1. What Is This Conformance Suite?

The Margo Conformance Suite is built for **Margo specification authors and members** to verify that a real WFM (Workload Fleet Manager) or a real Device Agent correctly implements the [Margo Management Interface specification](https://raw.githubusercontent.com/margo/specification/pre-draft/system-design/specification/margo-management-interface/workload-management-api-1.0.0.yaml).

**The use case:** A Margo member brings their WFM implementation (e.g., Symphony) or their device-agent implementation and runs the conformance suite against it. At the end, they get a signed test report showing which parts of the Margo spec their implementation conforms to.

The system has two CLIs:

```
conformance.sh   →  Prepare: create test groups, configure data, select test IDs
run-tests.sh     →  Execute: run tests against real WFM or device, generate reports
```

**Important:** The test scenarios (what to test and how) are created by the Margo user — typically exported from Postman as a collection. The conformance infrastructure handles running them, signing requests with RFC 9421, and generating a group-based report. See [Section 15](#15-design-notes--postman-vs-custom-format) for why there is also a custom JSON format.

---

## 2. System Architecture Overview

```
┌──────────────────────────────────────────────────────────────────────┐
│                     MARGO CONFORMANCE SYSTEM                          │
│                                                                       │
│  conformance.sh (CLI #1)          run-tests.sh (CLI #2)              │
│  ┌──────────────────────┐         ┌──────────────────────────────┐   │
│  │  Prepare              │         │  Execute                     │   │
│  │  - Create groups      │────────▶│                              │   │
│  │  - Collect test IDs   │         │  WFM Supplier persona        │   │
│  │    from scenario files│         │  ┌──────────────────────┐   │   │
│  │  - Generate group.json│         │  │ Scenario JSON files   │   │   │
│  │  - Generate certs     │         │  │ → run_wfm_scenarios.js│   │   │
│  └──────────────────────┘         │  │   (Node.js, RFC 9421  │   │   │
│                                   │  │    signing, hits WFM) │   │   │
│                                   │  └──────────────────────┘   │   │
│                                   │  OR                          │   │
│                                   │  ┌──────────────────────┐   │   │
│                                   │  │ Postman collections   │   │   │
│                                   │  │ → Newman runner       │   │   │
│                                   │  └──────────────────────┘   │   │
│                                   │                              │   │
│                                   │  Device Supplier persona     │   │
│                                   │  ┌──────────────────────┐   │   │
│                                   │  │ run_tests.go          │   │   │
│                                   │  │ (Go, mock WFM,        │   │   │
│                                   │  │  tests real device)   │   │   │
│                                   │  └──────────────────────┘   │   │
│                                   └──────────────────────────────┘   │
│                                              │                        │
│                                   HTML Reports per Group              │
│                                   Runner/wfm-supplier/*.html          │
└──────────────────────────────────────────────────────────────────────┘
```

### Component roles

| Component | Language | Role |
|-----------|----------|------|
| `conformance.sh` | Bash | Interactive setup: create groups, collect test IDs from scenario files, generate `group.json` |
| `run-tests.sh` | Bash | Orchestrator: selects persona/group, calls the right runner, generates reports |
| `run_wfm_scenarios.js` | Node.js | WFM Supplier runner when scenario files use the custom format (handles RFC 9421 signing) |
| Newman | npm | WFM Supplier runner when scenario files are Postman collections |
| `run_tests.go` | Go | Device Supplier runner: starts a mock WFM, tests a real device agent |

---

## 3. Two Personas Explained

### WFM Supplier

> "I am building a WFM. Test it against the Margo spec."

The WFM Supplier persona verifies that a **WFM implementation** correctly implements the Margo specification.

The conformance tool **acts as a conformant device** and sends all kinds of requests to the **real WFM under test** — valid requests, invalid requests, edge cases, negative tests. It checks that the WFM returns the correct responses, HTTP status codes, headers, and error formats.

```
Conformance Tool (acts as device)  ───────▶  Real WFM (under test)
  run_wfm_scenarios.js                        symphony.machine:8082
  ─ Signs requests with RFC 9421              /v1alpha2/margo
  ─ Sends onboarding, capabilities,
    deployment retrieval, status updates
  ─ Validates responses against spec
```

**Who uses this:** WFM vendors, Margo spec authors testing a WFM implementation.

### Device Supplier

> "I am building a device agent. Test it against the Margo spec."

The Device Supplier persona verifies that a **device agent implementation** correctly communicates with a WFM.

The conformance tool **acts as a mock WFM** and accepts connections from a real device agent. It checks that the device makes the right API calls, sends correctly-signed requests, and handles deployment instructions correctly.

```
Conformance Tool (acts as WFM)  ◀──────  Real Device Agent (under test)
  run_tests.go                            device-agent binary
  ─ Provides mock WFM endpoints
  ─ Validates what device sends
  ─ Returns test deployment instructions
```

**Who uses this:** Device vendors, Margo spec authors testing a device implementation.

---

## 4. Directory Structure

```
conformance/
├── conformance.sh                      # CLI #1: interactive group setup
├── run-tests.sh                        # CLI #2: test execution
│
├── wfm-supplier/                       # WFM Supplier persona
│   ├── run_wfm_scenarios.js            # ★ Custom-format scenario runner (RFC 9421 signing)
│   ├── run_wfm_scenarios.js.bak        # Backup copy
│   ├── spec.yaml                       # Local copy of the Margo WFM OpenAPI spec
│   ├── postman_collection.json         # Legacy Postman collection (Newman path)
│   └── newman-data/
│       ├── certs/                      # ★ Active certificate directory (used at runtime)
│       │   ├── device.key              # Fresh ECDSA P-256 private key (regenerated per scenario)
│       │   ├── device-cert.pem         # Fresh self-signed certificate
│       │   └── ca-cert.pem             # WFM's CA cert (copy from symphony after start)
│       └── device-agent.env.json       # Postman/Newman environment variables
│
├── Data-Generator/
│   └── wfm-supplier/
│       └── groups/
│           ├── diamond/                # Test group "diamond"
│           │   ├── group.json          # ★ Which test IDs to run (generated by conformance.sh)
│           │   └── test-scenarios.json # ★ Scenario definitions (user-provided)
│           └── silver/                 # Another test group (example)
│               ├── group.json
│               └── ...
│
└── Runner/
    └── wfm-supplier/                   # HTML reports (one per run)
        └── wfm-scenario-report-diamond_YYYYMMDD_HHMMSS.html
```

---

## 5. How to Run Tests

### Prerequisites

1. Real WFM (Symphony) must be running at `https://symphony.machine:8082`
2. CA certificate copied: `conformance/wfm-supplier/newman-data/certs/ca-cert.pem`
3. Node.js installed (v13.2+ for `dsaEncoding: 'ieee-p1363'` support)

### Getting the CA certificate

```bash
# Restart Symphony to clear previously-registered device certs (avoids 409 on re-onboarding)
# Use the wfm.sh menu: press 4 (stop) then 3 (start)

# Copy the CA cert
cp ~/symphony/api/certs/ca-cert.pem \
   ~/nitin/sandbox/conformance/wfm-supplier/newman-data/certs/ca-cert.pem
```

### Running the tests

```bash
cd ~/nitin/sandbox/conformance

# Interactive mode (prompts for persona, group, URL):
./run-tests.sh

# Direct mode:
./run-tests.sh wfm diamond https://symphony.machine:8082/v1alpha2/margo
```

**Arguments:**
- `wfm` — WFM Supplier persona
- `diamond` — test group name
- URL — full WFM base URL (must include `/v1alpha2/margo`)

### What happens during a run

1. `run-tests.sh` reads `group.json` for the selected group
2. It reads all `test-scenarios.json` files in the group directory
3. `jq` filters the scenarios: only steps whose IDs appear in `group.json` → `testCases` are included
4. A **fresh ECDSA P-256 certificate** is generated via `openssl`
5. `node run_wfm_scenarios.js` is called with the filtered scenario list
6. The runner executes each scenario (regenerating a fresh cert per scenario to avoid 409)
7. An HTML report is written to `Runner/wfm-supplier/`

### Example output

```
════════════════════════════════════════════════════════════════════════
 Margo WFM Conformance Test Runner
 WFM Scenario Test
 WFM:   https://symphony.machine:8082/v1alpha2/margo
════════════════════════════════════════════════════════════════════════

────────────────────────────────────────────────────────────────────────
 SCENARIO 5 of 7  ·  Device Onboarding
 Certificate retrieval plus successful and rejected onboarding flows.
────────────────────────────────────────────────────────────────────────

  [step-1.1]  Get Root CA Certificate
   ▶  GET    /api/v1/onboarding/certificate  [signed]
   ✓  PASS  200 OK

  [step-1.2]  Onboard Trusted Device
   ▶  POST   /api/v1/onboarding  [signed · content-digest]
   ✓  PASS  201 Created
        ↳ clientId = "client-f10ec980a73cbc76-1781705938"

  [step-1.3]  Reject Duplicate Certificate Registration
   ▶  POST   /api/v1/onboarding  [signed · content-digest]
   ✓  PASS  409 Conflict

  Scenario result: 3/3 steps  ✓ all passed
...
════════════════════════════════════════════════════════════════════════
 CONFORMANCE SUMMARY  ·  38 tests
════════════════════════════════════════════════════════════════════════

 Scenario                              Steps  Passed  Failed
 ───────────────────────────────────────────────────────────
 Capabilities Reporting                    3       3       0
 Capabilities Error Handling               8       8       0
 Deployment Retrieval And Status           3       3       0
 Capabilities With Missing ApiVersion      2       2       0
 Device Onboarding                         3       3       0
 Onboarding Error Handling                 6       6       0
 Status And Retrieval Errors              13      13       0
 ───────────────────────────────────────────────────────────
 TOTAL                                    38      38       0

 ✅  ALL 38 TESTS PASSED
════════════════════════════════════════════════════════════════════════
```

---

## 6. The Margo WFM API — Endpoint Reference

All paths are relative to the WFM base URL. The Symphony implementation uses the internal path prefix `/api/v1/` for all endpoints.

### GET `/api/v1/onboarding/certificate`

Returns the WFM's root CA certificate. **No signature required.**

**Response 200:**
```json
{ "certificate": "LS0tLS1CRUd..." }  // Base64-encoded PEM
```

---

### POST `/api/v1/onboarding`

Registers a new device. **Signature is optional** — the device certificate itself is the identity.

**Request body:**
```json
{
  "apiVersion": "onboarding.margo.org/v1alpha1",
  "kind": "OnboardingRequest",
  "certificate": "<base64-encoded PEM>"
}
```

The runner injects the base64-encoded `device-cert.pem` content when the scenario step contains `"certificate": "./certs/device-cert.pem"`.

**Response 201:**
```json
{ "clientId": "client-a74b314d0fb61bd5-1781702233" }
```

**CRITICAL:** Save `clientId`. It is used as the URL path parameter for every subsequent API call, AND must appear in `properties.id` in capability manifests.

**Response 409:** Same cert already registered:
```json
{ "Error": "Device signature already exists" }
```

**Response 400:** Schema validation failure:
```json
{ "Error": "invalid API version: v1" }
```

**What the WFM validates at onboarding:**
- `apiVersion` must be `"onboarding.margo.org/v1alpha1"` (returns 400: "API version")
- `kind` must be `"OnboardingRequest"` (returns 400: "kind")
- `certificate` must be present and non-empty (returns 400: "certificate")
- **Does NOT validate** that the certificate is signed by a trusted CA — any cert is accepted

---

### POST `/api/v1/clients/{clientId}/capabilities`
### PUT `/api/v1/clients/{clientId}/capabilities`

Reports device capabilities. **Signature required.**

**Request body:**
```json
{
  "apiVersion": "device.margo.org/v1alpha1",
  "kind": "DeviceCapabilitiesManifest",
  "properties": {
    "id": "client-a74b314d0fb61bd5-1781702233",
    "vendor": "Acme Corp",
    "modelNumber": "ACM-XYZ",
    "serialNumber": "SN-12345",
    "roles": ["Standalone Device"],
    "resources": {
      "cpu": { "cores": 4, "architecture": "arm64" },
      "memory": "16Gi",
      "storage": "256Gi",
      "interfaces": [{ "type": "ethernet" }],
      "peripherals": []
    }
  }
}
```

**CRITICAL RULE:** `properties.id` **MUST equal** the `clientId` from onboarding. If they differ:
```json
{ "Error": "device ID mismatch" }   ← HTTP 400
```

In scenario JSON always use `"id": "{clientId}"` so it gets substituted with the real value.

**Response 201:**
```json
{ "message": "Device capabilities reported successfully" }
```

**What the WFM validates:**
| Field | Validates? | On failure |
|-------|-----------|------------|
| `properties.id` vs URL `clientId` | ✅ Yes | 400 "device ID" |
| `apiVersion` | ✅ Yes | 400 |
| `kind` | ✅ Yes | 400 |
| `roles` values | ❌ No | 201 (accepted) |
| `interfaces[].type` values | ❌ No | 201 (accepted) |
| `cpu.architecture` values | ❌ No | 201 (accepted) |

---

### GET `/api/v1/clients/{clientId}/deployments`

Returns the current desired-state manifest. **Signature required.**

**Required header:** `Accept: application/vnd.margo.manifest.v1+json`

**Response 200:**
```json
{
  "manifestVersion": 1,
  "bundle": null,
  "deployments": []
}
```

**Response headers:**
```
ETag: "sha256:abc123..."
```

Use the ETag with `If-None-Match` on the next request to get 304 if unchanged.

**Response 304:** Manifest unchanged (no body).  
**Response 500:** Wrong `Accept` header (spec says 406, real WFM returns 500):
```json
{ "Error": "accept header not supported" }
```

---

### GET `/api/v1/clients/{clientId}/bundles/{digest}`

Downloads the deployment bundle archive. **Signature required.**

`digest` is content-addressable (e.g., `sha256:abcdef...`). If the digest does not match stored content → 404.

**Response 200 headers:**
```
Content-Type: application/vnd.margo.bundle.v1+tar+gzip
Cache-Control: public, max-age=31536000, immutable
ETag: "<digest>"
```

**Response 401:** No signature (GET resource endpoints return 401, not 400).

---

### GET `/api/v1/clients/{clientId}/deployments/{deploymentId}/{digest}`

Downloads a single deployment YAML file. **Signature required.**

**Response 200 headers:**
```
Content-Type: application/yaml
Cache-Control: public, max-age=31536000, immutable
Vary: Accept-Encoding
ETag: "<digest>"
```

**Response 401:** No signature.

---

### POST `/api/v1/clients/{clientId}/deployments/{deploymentId}/status`

Reports deployment status. **Signature required.**

**Request body:**
```json
{
  "apiVersion": "deployment.margo.org/v1alpha1",
  "kind": "DeploymentStatusManifest",
  "deploymentId": "dep-abc123",
  "status": { "state": "installed" },
  "components": [{ "name": "app-component-1", "state": "installed" }]
}
```

`deploymentId` in the body **must match** the `{deploymentId}` in the URL.

**Valid `state` values:** `installed`, `installing`, `uninstalling`, `failed`

**Response 200:**
```json
{ "acknowledgement": "received" }
```

---

## 7. HTTP Message Signatures (RFC 9421) — Deep Dive

Every WFM API request (except `GET /onboarding/certificate`) must be signed. This is the most complex and error-prone part. Read this section carefully if you encounter 400 errors.

### What gets signed

**Requests without a body (GET, DELETE):**
```
"@method": GET
"@target-uri": https://symphony.machine:8082/v1alpha2/margo/api/v1/clients/abc/deployments
"@authority": symphony.machine:8082
"@signature-params": ("@method" "@target-uri" "@authority");created=1718600000;keyid="a1b2c3..."
```

**Requests with a body (POST, PUT):**
```
"@method": POST
"@target-uri": https://symphony.machine:8082/v1alpha2/margo/api/v1/clients/abc/capabilities
"@authority": symphony.machine:8082
"content-digest": sha-256=:BASE64HASH:
"@signature-params": ("@method" "@target-uri" "@authority" "content-digest");created=1718600000;keyid="a1b2c3..."
```

**CRITICAL:** `@authority` is `host:port` (e.g., `symphony.machine:8082`). The WFM verifier (`shared-lib/crypto/verifier.go`) uses `htmsighttp.WithComponents(Method, TargetURI, Authority)` and **requires** all three. Requests missing `@authority` in their signature are silently rejected.

### Produced HTTP headers

```
Content-Digest: sha-256=:BASE64==:
Signature-Input: sig1=("@method" "@target-uri" "@authority" "content-digest");created=1718600000;keyid="abc123..."
Signature: sig1=:BASE64SIGNATURE:
```

### Key ID (`keyid`)

The `keyid` tells the WFM which registered public key to use for verification. It must match the algorithm used by `shared-lib/crypto/keyid.go::ComputeKeyIDFromPrivateKeyPEM`:

```
keyid = hex( SHA-256( PKIX-DER-encoded-public-key ) )
```

Step by step:
1. Load ECDSA private key from PEM
2. Derive the public key
3. Marshal to PKIX DER format (`x509.MarshalPKIXPublicKey`)
4. SHA-256 hash the DER bytes
5. Hex-encode the hash

**In Node.js:**
```javascript
function computeKeyId(privateKeyPem) {
  const privObj = crypto.createPrivateKey(privateKeyPem);
  const pubDer = crypto.createPublicKey(privObj).export({ format: 'der', type: 'spki' });
  return crypto.createHash('sha256').update(pubDer).digest('hex');
}
```

**Wrong approaches:** Hardcoding `"device-key"`, using the cert fingerprint in a different format, using the raw public key bytes before PKIX wrapping.

### ECDSA signature format — the most common bug

The Margo Go verifier (`lestrrat-go/dsig`) uses **IEEE P1363 format** (raw r||s):
- For P-256: exactly **64 bytes** (32 r + 32 s)

Node.js `createSign().sign()` produces **DER/ASN.1 format** by default:
- For P-256: approximately **70–72 bytes** (variable, ASN.1 wrapping)

The verifier does `if len(signature) != keySize*2 { reject }` — DER-encoded signatures are always rejected with no useful error.

**Wrong (DER, ~70 bytes — DO NOT USE):**
```javascript
const signer = crypto.createSign('sha256');
signer.update(signatureInput);
const sig = signer.sign(privateKey); // ← Wrong format
```

**Correct (IEEE P1363, exactly 64 bytes for P-256):**
```javascript
const signatureBytes = crypto.sign('sha256', Buffer.from(signatureInput), {
  key: privateKey,
  dsaEncoding: 'ieee-p1363',  // ← Required
});
```

`dsaEncoding: 'ieee-p1363'` requires Node.js v13.2+.

### Content-Digest (RFC 9530)

Format: `sha-256=:BASE64:` — colons wrap the base64 value.

```javascript
const digest = crypto.createHash('sha256').update(bodyText).digest('base64');
headers['Content-Digest'] = `sha-256=:${digest}:`;
```

**Important ordering:** Content-Digest must be computed and set in headers **before** signing, because the signature covers the `content-digest` header value. The runner calls `prepareContentDigest()` before `signRequest()`.

**Important:** WFM checks Content-Digest presence **before** checking the signature on POST/PUT endpoints. A request with a body but no Content-Digest (and no signature) returns 400, not 401.

### WFM signature verification flow

```
1. Receive request
2. Check for Signature-Input + Signature headers
   → Missing? → 400 "missing signature headers"
3. Check @authority is a covered component
   → Missing? → reject
4. Reconstruct signature base string from covered components
5. Look up key using keyid from registered certs
   → Not found? → reject
6. Verify ECDSA signature (IEEE P1363 format)
   → len check: must be keySize×2 (64 bytes for P-256)
   → Bad sig? → reject
7. (If body) verify Content-Digest matches actual body hash
8. Process request
```

---

## 8. Real WFM Behavior vs Spec (Important Differences)

These are behaviors observed on the real Symphony WFM that differ from the OpenAPI spec. Always check here when tests fail unexpectedly.

### Error field name: `Error` not `error`

Every real WFM error response uses `"Error"` (capital E). The spec examples show `"error"` (lowercase):

```json
// Spec examples show:        { "error": "Invalid certificate" }
// Real WFM always returns:   { "Error": "Invalid certificate" }
```

This affects every validation that checks error response fields. Always validate the `Error` field (capital E).

### Unsigned POST/PUT returns 400, not 401

The spec indicates 401 for auth failures. The real WFM returns:
- **400** `{"Error": "missing signature headers"}` — for POST/PUT with missing signature
- **401** — for GET endpoints (bundles, deployment manifests) with missing signature

Why? The WFM validates Content-Digest presence before checking the signature for body requests. A POST with neither Content-Digest nor Signature hits the Content-Digest validation first → 400.

### Wrong Accept header returns 500, not 406

```
GET /deployments  with  Accept: application/json

Spec says:   406 Not Acceptable
Real WFM:    500 Internal Server Error
             {"Error": "accept header not supported"}
```

### WFM does not validate capability content fields

The spec implies `roles`, `interfaces`, and CPU `architecture` should only accept defined enum values. The real WFM accepts anything:

| Field | Invalid Example | WFM Response |
|-------|----------------|--------------|
| `roles` | `["Unknown Role"]` | 201 Created |
| `interfaces[].type` | `"serial"` | 201 Created |
| `cpu.architecture` | `"sparc"` | 201 Created |

This is a compliance gap in the current WFM. Tests document this behavior explicitly.

### Onboarding: any cert is accepted (no 403)

The spec defines a 403 "Client rejected" response for untrusted certificates. The real WFM accepts **any** cert — there is no trust-store validation at the onboarding endpoint.

### Capabilities: apiVersion IS validated

Despite not validating field content, the WFM does validate `apiVersion` on capabilities:
- Missing `apiVersion` → 400 (WFM validates this strictly)
- Wrong `apiVersion` format → 400

### Duplicate cert registration → 409

Sending the same certificate twice to `/onboarding`:
```json
{ "Error": "Device signature already exists" }   ← HTTP 409
```

### Bundle/manifest endpoints require pre-configured deployments

`GET /bundles/{digest}` and `GET /deployments/{deploymentId}/{digest}` only return 200 when the WFM has actual deployments configured for that device. With no deployments:
- Bundle download with extracted digest (which is `null`) → 404
- Deployment manifest with empty `deploymentId` → 301 redirect

Steps 3.3–3.7 in the diamond group are excluded from the default run for this reason. Add them back to `group.json` testCases when deployments are configured in Symphony.

---

## 9. The Node.js Scenario Runner — How It Works

**File:** `conformance/wfm-supplier/run_wfm_scenarios.js`

This runner is used when the group contains scenario files in the custom JSON format (detected by `discover_group_scenario_files`). It handles RFC 9421 request signing — which Postman/Newman cannot do natively.

### Invocation

```bash
node run_wfm_scenarios.js \
  <wfm-base-url>      # e.g. https://symphony.machine:8082/v1alpha2/margo
  <scenarios.json>    # filtered scenario list (temp file, built by run-tests.sh)
  <report.html>       # output HTML report path
  <cert-dir>          # directory containing device.key + device-cert.pem + ca-cert.pem
  [group-name]        # optional: displayed in report header
  [group-version]     # optional: displayed in report header
```

### Execution flow

```
1. Print banner (group name, WFM URL)

For each scenario:
  2. Regenerate fresh ECDSA P-256 cert (new OpenSSL keygen per scenario)
  3. Reset context = {}

  For each step:
    4. Substitute {variables} in endpoint/headers/request_body using context
    5. Inject device cert PEM (base64) into onboarding bodies
    6. Compute Content-Digest (sha-256=:BASE64:) for body requests
    7. Sign request with RFC 9421 (unless skip_signing: true)
    8. Send HTTPS request
    9. Parse response JSON
   10. Check expected_status vs actual
   11. Run all validations
   12. If all pass: extract context values for later steps
   13. Print step result

  14. Print scenario summary

15. Write HTML report (group summary + step details)
16. Print final summary table (per-scenario pass/fail counts)
17. Exit 1 if any failures
```

### Variable substitution

Steps extract values from responses into a shared `context` object and reference them in later steps using `{variableName}`:

```json
// Setup step — extract clientId
"extract_context": { "clientId": "clientId" }

// Later step — use it
"endpoint": "/api/v1/clients/{clientId}/capabilities",
"request_body": { "properties": { "id": "{clientId}" } }
```

Context is **reset between scenarios**. All steps in a scenario share the same context.

### Certificate injection

`"certificate": "./certs/device-cert.pem"` is a magic marker in scenario bodies. The runner replaces it with the actual base64-encoded PEM content of the current cert file.

To send a literal string (for negative tests), set `"skip_certificate_injection": true` on the step.

### Header case-insensitivity

Node.js HTTP responses store all header names in lowercase (`etag`, not `ETag`). The runner's `getField()` function does case-insensitive lookup when traversing `_headers.*` paths, so you can write `"_headers.ETag"` in validations and it will correctly find `etag`.

---

## 10. Test Scenario JSON Format (Custom Adapter Format)

**File:** `Data-Generator/wfm-supplier/groups/<groupname>/test-scenarios.json`

This format was created as an adapter to enable RFC 9421 signing in scenarios. See [Section 15](#15-design-notes--postman-vs-custom-format) for the full context on why this format exists alongside Postman.

### File structure

The file must be a JSON **array** of scenario objects. The runner detects this format (vs Postman) by checking `type == "array" and any .[]? has .steps array`.

```json
[
  {
    "id": "scenario-onboarding",
    "name": "Device Onboarding",
    "description": "Human-readable description of what this scenario tests.",
    "steps": [
      {
        "id": "step-1.1",
        "name": "Get Root CA Certificate",
        "method": "GET",
        "endpoint": "/api/v1/onboarding/certificate",
        "headers": {},
        "expected_status": 200,
        "validations": [
          { "field": "certificate", "operation": "is_string" }
        ],
        "extract_context": {}
      },
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
        "headers": {},
        "expected_status": 201,
        "validations": [
          { "field": "clientId", "operation": "is_string" }
        ],
        "extract_context": {
          "clientId": "clientId"
        }
      }
    ]
  }
]
```

### Step flags

| Flag | Effect |
|------|--------|
| `skip_signing: true` | Send without `Signature`/`Signature-Input` headers. Content-Digest still computed and sent. Tests auth-failure paths. |
| `skip_certificate_injection: true` | Preserve the literal `certificate` value as-is instead of injecting the real base64 PEM. |

### Validation operations

| Operation | What it checks |
|-----------|---------------|
| `exists` | Field is present and not null |
| `is_string` | Field is a string |
| `is_number` | Field is a number |
| `is_array` | Field is an array |
| `not_empty` | Field exists and is not empty string |
| `equals` | Field exactly equals `value` (with variable substitution) |
| `contains` | String representation contains `value` |

### Field path syntax

| Path | Accesses |
|------|---------|
| `clientId` | Top-level response JSON field |
| `deployments.0.deploymentId` | Array element (numeric index) |
| `_headers.ETag` | Response header (case-insensitive) |
| `_body` | Raw response body string |
| `Error` | Top-level error field (capital E — WFM always uses this) |

---

## 11. Group System — How Groups Work

Groups are the core organizational unit of the conformance system. A Margo user organizes their test scenarios into groups, each group targeting a specific conformance level or area.

### How a group is created (conformance.sh)

1. User runs `conformance.sh` and selects "Create group"
2. The CLI prompts for:
   - Group name (e.g., `diamond`, `silver`)
   - Which scenario files to include in the group
   - Which specific test IDs (scenario IDs or step IDs) from those files to include
3. `group.json` is generated with the selected `testCases` list
4. Multiple scenario files can be in one group directory — the runner merges them

### group.json structure

```json
{
  "name": "diamond",
  "version": "1.2.1",
  "persona": "wfm-supplier",
  "description": "Full conformance test suite for WFM",
  "testCases": [
    "scenario-onboarding",
    "step-1.1",
    "step-1.2",
    "step-2.0",
    "step-2.1"
  ]
}
```

`testCases` can contain:
- **Scenario IDs** (e.g., `"scenario-onboarding"`) — includes the whole scenario
- **Step IDs** (e.g., `"step-1.1"`) — includes only that specific step (even if the rest of the scenario is not listed)
- **Legacy device IDs** — used by the Device Supplier persona

### Step-level filtering in the runner

When `run-tests.sh` builds the scenario file, `jq` applies two-level filtering:

1. **Scenario selection**: Include a scenario if its ID or any of its step IDs appears in `testCases`
2. **Step filtering**: Within selected scenarios, keep only steps whose IDs are in `testCases`

This means you can include a partial scenario — for example, include `step-3.0`, `step-3.1`, `step-3.2` from `scenario-deployments` without `step-3.3` through `step-3.7` (which need actual WFM deployments configured).

### Multiple scenario files

A group directory can contain multiple `test-scenarios.json` files (one per feature area, for example). The runner automatically discovers all files matching the format and merges them before filtering by `testCases`.

```
groups/diamond/
├── group.json
├── onboarding-scenarios.json
├── capabilities-scenarios.json
└── deployment-scenarios.json
```

All three files are merged and then filtered by `group.json` → `testCases`.

### Run reports are group-based

The HTML report name includes the group name and timestamp:
```
Runner/wfm-supplier/wfm-scenario-report-diamond_20260617_141857.html
```

The HTML report shows:
- Group metadata (name, version, WFM URL, run timestamp)
- Per-scenario summary table (steps / passed / failed)
- Full step-by-step details table

---

## 12. Common Errors and How to Fix Them

### All POST/PUT requests return 400 (ECDSA format bug)

**Symptom:** Every signed body request fails with 400, no useful error message.

**Root cause:** Node.js `createSign().sign()` produces DER/ASN.1 format (~70 bytes). The Go verifier checks `len(sig) == 64` for P-256 and immediately rejects.

**Fix:** Use `dsaEncoding: 'ieee-p1363'` in `crypto.sign()`:
```javascript
crypto.sign('sha256', data, { key: privateKey, dsaEncoding: 'ieee-p1363' })
```

**Diagnosis:** Log `signatureBytes.length` — must be exactly 64 for P-256.

---

### `expected HTTP 201, got 400` on capabilities

Most common causes in order:
1. `properties.id` ≠ `clientId` → Use `"id": "{clientId}"` in scenario JSON
2. Missing or wrong `apiVersion` → Must be `"device.margo.org/v1alpha1"`
3. Missing `kind` → Must be `"DeviceCapabilitiesManifest"`
4. Signature verification failure → Check ECDSA format (above)

---

### `field "Error" is missing` in validation

The WFM returns `"Error"` (capital E). All scenario validations must use `"field": "Error"` not `"field": "error"`.

---

### `expected HTTP 201, got 409` on onboarding

Certificate already registered from a previous run. Restart Symphony to clear registrations:
```bash
# wfm.sh menu: 4 = stop, 3 = start
cp ~/symphony/api/certs/ca-cert.pem \
   ~/nitin/sandbox/conformance/wfm-supplier/newman-data/certs/ca-cert.pem
```

---

### `expected HTTP 400, got 401` or vice versa on unsigned requests

Different endpoint types return different status for missing signatures:
- **POST/PUT** with missing signature → **400** `"missing signature headers"`
- **GET** bundle/manifest with missing signature → **401**

Match the `expected_status` to the endpoint type.

---

### `expected HTTP 406, got 500` on deployments

The real WFM returns 500 (not 406) for unsupported `Accept` headers. Set `expected_status: 500` and validate `Error` contains `"accept header"`.

---

### `expected HTTP 200, got 404` on bundle download

`bundle.digest` extracted from the deployments response is `null` — the WFM has no deployments configured. Remove steps 3.3–3.7 from `group.json` testCases, or configure a deployment in Symphony first.

---

### `{clientId}` substitutes to empty string in URLs

A setup (onboarding) step failed before extracting `clientId` into context. Look for the FAIL line on the setup step and fix that first. Ensure the setup step's ID is in `testCases`.

---

### Keyid mismatch (signature verification fails silently)

Ensure the `keyid` is computed as `hex(SHA-256(PKIX-DER-public-key))`. Any other format (raw bytes, cert fingerprint, hardcoded string) will cause the WFM to fail key lookup and reject the signature without a clear error.

---

## 13. Certificate Lifecycle

### During conformance testing

A fresh ECDSA P-256 cert is generated for **each scenario** (to avoid 409 from repeated onboarding):
```bash
openssl ecparam -name prime256v1 -genkey -noout -out device.key
openssl req -new -x509 -days 365 -key device.key -out device-cert.pem \
  -subj "/C=IN/ST=GGN/L=Sector48/O=Margo/OU=Conformance/CN=device-<timestamp>"
```

This self-signed cert is submitted to the WFM during the onboarding step. The WFM stores the cert's public key and associates it with the issued `clientId`. Subsequent requests are signed with the private key, and the WFM verifies using the stored public key.

### WFM CA certificate

The WFM serves its API over HTTPS using a cert signed by its own CA. The Node.js runner loads the CA cert (`ca-cert.pem`) to verify the WFM's TLS certificate.

Get it from: `~/symphony/api/certs/ca-cert.pem`  
Copy to: `conformance/wfm-supplier/newman-data/certs/ca-cert.pem`

### Real device-agent certificate

The real device-agent uses a static cert configured in `config/device-public.crt`. This is submitted once at onboarding and reused for all subsequent requests. The `clientId` received at onboarding is persisted by the agent.

---

## 14. How the Real Device-Agent Works

**Location:** `poc/device/agent/`  
**Config:** `poc/device/agent/config/config.yaml`

```yaml
wfm:
  sbiUrl: https://symphony.machine:8082/v1alpha2/margo
  clientPlugins:
    requestSigner:
      enabled: true
      hashAlgo: "sha256"
      signatureAlgo: "ecdsa"    # or "rsa"
      signatureFormat: "structured"
      keyRef:
        path: "./config/device-private.key"
    tlsHelper:
      enabled: true
      caKeyRef:
        path: "./config/ca-cert.pem"

capabilities:
  readFromFile: ./config/capabilities.json
```

### Capabilities file

```json
{
  "apiVersion": "device.margo.org/v1alpha1",
  "kind": "DeviceCapabilitiesManifest",
  "properties": {
    "id": "device-id-from-manufacturer",
    "vendor": "Northstar Industrial Applications",
    ...
  }
}
```

**Important:** The agent **overwrites** `properties.id` at runtime with the `clientId` received from the WFM during onboarding. This is the same reason conformance scenarios use `"id": "{clientId}"` — both must match the WFM-issued clientId.

### Agent startup sequence

```
1. Load private key, compute keyid (SHA-256 hex of PKIX DER public key)
2. GET /api/v1/onboarding/certificate → get WFM CA cert for TLS verification
3. POST /api/v1/onboarding with device cert → receive clientId
4. POST /api/v1/clients/{clientId}/capabilities
   with properties.id = clientId (overrides capabilities.json value)
5. Poll GET /api/v1/clients/{clientId}/deployments every 15 seconds
   → If ETag changed: process new/updated deployments
6. When deployments change: deploy/undeploy via Kubernetes/Docker runtime
7. After each deployment action: POST deployment status back to WFM
```

### Signing in Go (`shared-lib/crypto/signer.go`)

Uses `lestrrat-go/htmsig` library:
- Body requests: `@method` + `@target-uri` + `@authority` + `content-digest`
- No-body requests: `@method` + `@target-uri` + `@authority`
- The `lestrrat-go/dsig` library produces IEEE P1363 ECDSA signatures (raw r||s)

This is the reference implementation that the Node.js runner in `run_wfm_scenarios.js` must exactly match.

---

## 15. Design Notes — Postman vs Custom Format

This section explains a design tension that is important to understand when extending the conformance suite.

### The intended design

Margo users create their test scenarios in **Postman** (because Postman has a rich UI for designing API test cases) and export them as Postman collection JSON files. The conformance system runs these collections using **Newman** and generates reports.

The `conformance.sh` CLI reads Postman collection files, presents the test items to the user, and lets them select which items to include in a group. The `group.json` records those selections. The `run-tests.sh` Newman path executes the filtered collection.

### The signing problem

**Postman/Newman cannot natively implement RFC 9421 HTTP Message Signatures.** RFC 9421 requires:
- Computing a structured signature base string from specific HTTP components
- ECDSA signing with IEEE P1363 encoding (not DER)
- Setting `Signature-Input` and `Signature` headers before the request is sent

Postman pre-request scripts can run JavaScript, but they cannot call Node.js `crypto` APIs in the way needed, and they cannot reliably produce the correct IEEE P1363 ECDSA signature format.

### The custom format solution

The `run_wfm_scenarios.js` runner and the custom `test-scenarios.json` format were created to solve this. The format is:
- A JSON array of scenario objects (not Postman format)
- Each step specifies method, endpoint, body, headers, expected status, and validations
- The runner handles all signing, cert injection, and context propagation internally

### Coexistence in the current system

`run-tests.sh` uses both:

```bash
# Detected by: type == "array" and any .[]? has .steps array
if custom-format scenario files exist:
    use run_wfm_scenarios.js (RFC 9421 signing)
else:
    discover Postman collection files
    use Newman (no signing)
```

The detection happens in `discover_group_scenario_files()`. The Postman/Newman path is still available for test scenarios that don't need RFC 9421 signing (e.g., testing endpoints that don't require signatures).

### What this means for you

- If you are adding new WFM Supplier tests that require RFC 9421 signing → use the custom JSON format in `test-scenarios.json`
- If you are bringing a Postman collection that uses pre-request scripts for signing → the Newman path will run it, but signing correctness depends on your Postman scripts
- Future work: a converter from Postman collection format to the custom JSON format would allow users to author in Postman and run via the Node.js signed runner

---

## 16. Adding or Modifying Tests

### Add a step to an existing scenario

1. Open `Data-Generator/wfm-supplier/groups/<group>/test-scenarios.json`
2. Find the target scenario and add a new step object with a unique ID (e.g., `step-5.8`)
3. Add that step ID to `group.json` → `testCases`
4. Run tests to verify

### Add a new scenario

1. Add a new object to the top-level array in `test-scenarios.json` with `"id"`, `"name"`, `"description"`, `"steps"`
2. Give steps unique IDs (e.g., `step-8.0`, `step-8.1`)
3. Add step IDs to `group.json` → `testCases`

### Temporarily narrow the test run

Edit `group.json` to include only the step IDs you want to test:
```json
{ "testCases": ["step-5.1", "step-5.2"] }
```

Run tests, then restore the full list.

### Enable deployment-dependent steps (3.3–3.7)

These steps test bundle download, deployment manifest download, and status reporting. They require a deployment configured in Symphony:

1. Configure a deployment in Symphony for the test device
2. Add `"step-3.3"`, `"step-3.4"`, `"step-3.5"`, `"step-3.6"`, `"step-3.7"` to `group.json` → `testCases`

### Updating a step when WFM behavior changes

1. Find the step by ID in `test-scenarios.json`
2. Update `expected_status` to match the actual WFM response
3. Update `validations` fields/values to match actual response JSON
4. Always check `"field": "Error"` (capital E) — WFM always uses this

---

## Quick Reference Card

| Item | Value / Rule |
|------|-------------|
| **WFM base URL** | `https://symphony.machine:8082/v1alpha2/margo` |
| **API path prefix** | `/api/v1/` (all endpoints) |
| **Margo spec** | [workload-management-api-1.0.0.yaml](https://raw.githubusercontent.com/margo/specification/pre-draft/system-design/specification/margo-management-interface/workload-management-api-1.0.0.yaml) |
| **Error field name** | `"Error"` — capital E, always |
| **Capabilities success** | `{"message": "Device capabilities reported successfully"}` |
| **`properties.id` rule** | MUST equal `clientId` from onboarding response |
| **Signature components (GET)** | `@method`, `@target-uri`, `@authority` |
| **Signature components (POST/PUT)** | `@method`, `@target-uri`, `@authority`, `content-digest` |
| **ECDSA format** | IEEE P1363 (raw r\|\|s, **64 bytes** for P-256) — NOT DER |
| **Content-Digest format** | `sha-256=:BASE64:` (colons around base64) |
| **keyid formula** | `hex(SHA-256(PKIX-DER-public-key))` |
| **Cert type** | ECDSA P-256 (`prime256v1`), self-signed |
| **Cert regeneration** | Per scenario (avoids 409 on re-onboarding) |
| **Unsigned POST/PUT** | 400 "missing signature headers" |
| **Unsigned GET bundle/manifest** | 401 |
| **Wrong Accept on /deployments** | 500 (spec says 406) |
| **Duplicate cert** | 409 "Device signature already exists" |
| **CA cert path (runtime)** | `wfm-supplier/newman-data/certs/ca-cert.pem` |
| **CA cert source** | `~/symphony/api/certs/ca-cert.pem` |
| **Deployment steps (3.3–3.7)** | Excluded by default — need WFM deployments configured |
| **Scenario format detection** | `type == "array" and any .[]? has .steps array` |
