# Device Supplier — How to Draft Your Own Test Cases

Use this guide alongside `TEMPLATE_custom_test_scenario.json` to write and run your own conformance tests.

---

## Step 1: Copy the Template

```bash
cp device-scenarios/TEMPLATE_custom_test_scenario.json device-scenarios/my-device-scenarios.json
```

Open `my-device-scenarios.json` and replace the placeholder values with your device's real details.

---

## Step 2: Understand the File Structure

Your file must be a **JSON array** of scenario objects. Each scenario is an independent test flow with one or more ordered steps:

```json
[
  {
    "id": "scenario-unique-id",
    "name": "Human Readable Name",
    "description": "What this scenario validates",
    "steps": [ ... ]
  }
]
```

Each step is one HTTP call to the mock WFM server:

```json
{
  "id": "step-1.1",
  "name": "Step Description",
  "method": "POST",
  "endpoint": "/api/v1/onboarding",
  "request_body": { ... },
  "headers": {},
  "expected_status": 201,
  "validations": [ ... ],
  "extract_context": { ... },
  "skip_signing": false,
  "skip_certificate_injection": false
}
```

---

## Step 3: Know the Real Margo Endpoints

Every step's `endpoint` must be one of the 8 real Margo API endpoints:

| # | Method | Endpoint | Purpose |
|---|--------|----------|---------|
| 1 | GET | `/api/v1/onboarding/certificate` | Fetch root CA cert |
| 2 | POST | `/api/v1/onboarding` | Register device, receive `clientId` |
| 3 | POST | `/api/v1/clients/{clientId}/capabilities` | Report device capabilities |
| 4 | PUT | `/api/v1/clients/{clientId}/capabilities` | Update device capabilities |
| 5 | GET | `/api/v1/clients/{clientId}/deployments` | Get deployment manifest |
| 6 | GET | `/api/v1/clients/{clientId}/bundles/{digest}` | Download deployment bundle |
| 7 | GET | `/api/v1/clients/{clientId}/deployments/{deploymentId}/{digest}` | Get individual deployment |
| 8 | POST | `/api/v1/clients/{clientId}/deployments/{deploymentId}/status` | Report deployment status |

> `{clientId}`, `{digest}`, `{deploymentId}` are placeholders — the runner substitutes values saved from earlier steps using `extract_context`.

---

## Step 4: Know What Values Are Valid

Use these values in your `request_body` to pass validation:

**`properties.roles`** — must be a non-empty array, each item must be one of:
- `"Standalone Cluster"`
- `"Cluster Leader"`
- `"Standalone Device"`

**`properties.resources.cpu.architecture`** — one of:
- `"amd64"`, `"x86_64"`, `"arm64"`, `"arm"`

**`properties.resources.interfaces[*].type`** — one of:
- `"ethernet"`, `"wifi"`, `"cellular"`, `"bluetooth"`, `"usb"`, `"canbus"`, `"rs232"`

**`apiVersion` values** (must match exactly):
- Onboarding: `"onboarding.margo.org/v1alpha1"`
- Capabilities: `"device.margo.org/v1alpha1"`
- Status: `"orchestration.margo.org/v1alpha1"`

**`kind` values** (must match exactly):
- Onboarding: `"OnboardingRequest"`
- Capabilities: `"DeviceCapabilitiesManifest"`
- Status: `"AppDeploymentStatus"`

---

## Step 5: Understand All Step Fields

| Field | Required | Description |
|-------|----------|-------------|
| `id` | Yes | Unique step ID. Recommended format: `step-X.Y` |
| `name` | Yes | Display name shown in test reports |
| `method` | Yes | `"GET"`, `"POST"`, or `"PUT"` |
| `endpoint` | Yes | URL path from the table above. Supports `{placeholder}` |
| `request_body` | No | JSON object to send. Omit for GET |
| `headers` | No | Extra headers. Values support `{placeholder}` |
| `expected_status` | Yes | The HTTP status you expect. Any other code = FAIL |
| `validations` | No | Checks to run on the response body or headers |
| `extract_context` | No | Save response values as variables for later steps |
| `skip_signing` | No | `true` = send without RFC 9421 signature. Use to test 401 rejection |
| `skip_certificate_injection` | No | `true` = send `certificate` field as-is without reading from file |

---

## Step 6: Write Validations

Validations check the response body (or headers) after the HTTP call:

```json
"validations": [
  { "field": "clientId",      "operation": "exists" },
  { "field": "clientId",      "operation": "is_string" },
  { "field": "status",        "operation": "equals",   "value": "ok" },
  { "field": "error",         "operation": "contains", "value": "certificate" },
  { "field": "errors",        "operation": "is_array" },
  { "field": "_headers.ETag", "operation": "not_empty" }
]
```

**All operations:**

| Operation | Needs `value`? | Passes when |
|-----------|---------------|-------------|
| `exists` | No | Field is present (any value) |
| `not_empty` | No | Field is present and not `""`, `null`, or `[]` |
| `equals` | Yes | Field value matches `value` exactly |
| `contains` | Yes | Field string contains `value` as a substring |
| `is_string` | No | Field value is a JSON string |
| `is_number` | No | Field value is a JSON number |
| `is_array` | No | Field value is a JSON array |
| `is_object` | No | Field value is a JSON object |

**Field path syntax:**

| Path | Accesses |
|------|---------|
| `"clientId"` | Top-level response field |
| `"status.state"` | Nested field |
| `"deployments.0.deploymentId"` | First array element |
| `"_headers.ETag"` | HTTP response header |

---

## Step 7: Pass Data Between Steps

Use `extract_context` to save response values, then inject them as `{placeholders}` in later steps.

**Save a value (in step N):**
```json
"extract_context": {
  "clientId":    "clientId",
  "bundleDigest": "bundleRef.digest",
  "manifestEtag": "_headers.ETag"
}
```

**Use a saved value (in step N+1):**
```json
"endpoint": "/api/v1/clients/{clientId}/deployments",
"headers":  { "If-None-Match": "{manifestEtag}" }
```

Placeholders work in: `endpoint`, `headers` values, and string values inside `request_body`.

> Variables only live within the same scenario — they do not carry over between scenarios.

---

## Step 8: Common Test Patterns

### Positive test (expect success)
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
      "vendor": "Acme",
      "modelNumber": "ACM-001",
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

### Negative test — invalid field value (expect 422)
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
      "id": "my-device",
      "vendor": "Acme",
      "modelNumber": "ACM-001",
      "serialNumber": "SN-12345",
      "roles": ["InvalidRoleName"],
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
  "expected_status": 422,
  "validations": [{ "field": "errors", "operation": "is_array" }],
  "extract_context": {},
  "skip_signing": false
}
```

### Negative test — missing signature (expect 401)
```json
{
  "id": "step-3.1",
  "name": "Reject Unsigned Request",
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

### ETag caching test (expect 304)
```json
{
  "id": "step-4.1",
  "name": "Cached Deployment Request",
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

## Step 9: Validate Your JSON

Before running, check the file is valid JSON:

```bash
jq empty my-device-scenarios.json && echo "valid"
```

List all scenario and step names:
```bash
jq '.[] | {scenario: .id, steps: [.steps[].id]}' my-device-scenarios.json
```

---

## Step 10: Register and Run

```bash
cd /path/to/conformance/

# Register with Data-Generator
bash conformance.sh
# → Select: Device Supplier
# → Select: Create new group
# → Point to your file when prompted

# Start mock WFM server
cd device-supplier/
make run-server

# Run your tests
make run-tests

# Open report
open reports/conformance-report-*.html
```

---

## What Causes Each Error Code

| Code | Trigger |
|------|---------|
| 400 | Missing required field, wrong `apiVersion`, wrong `kind`, missing `Content-Digest` header |
| 401 | Missing RFC 9421 signature (`skip_signing: true`) |
| 403 | Certificate in the rejected-certificates blocklist |
| 404 | `clientId` not registered, wrong bundle digest |
| 406 | Wrong `Accept` header on GET deployments |
| 422 | Invalid field value (`roles`, `architecture`, `interface.type`), empty required array, field type mismatch |

---

## Reference

- **Template to copy:** `device-scenarios/TEMPLATE_custom_test_scenario.json`
- **Full working examples:** `device-scenarios/test-scenarios.json`
- **Complete field and API reference:** `Final-Summary.md`
- **Validation rules (what causes 422):** `manifests/assertions.json`
