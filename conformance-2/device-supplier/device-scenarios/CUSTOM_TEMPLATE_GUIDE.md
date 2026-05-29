# Device Supplier Custom Test Scenarios - Template Guide

## Overview

The Device Supplier persona now supports **two options** when generating test cases:

1. **Option 1: Use Existing** - Uses the pre-built `test-scenarios.json` (7 conformance scenarios)
2. **Option 2: Provide Custom** - Bring your own test scenarios file

This guide explains how to create your own custom test scenarios.

---

## Quick Start

### Step 1: Copy the Template

```bash
cp TEMPLATE_custom_test_scenario.json my_test_scenarios.json
```

### Step 2: Edit Your Custom Scenarios

Open `my_test_scenarios.json` and modify:
- Scenario names and IDs
- API endpoints and methods
- Request bodies and headers
- Expected status codes
- Validation rules

### Step 3: Use with conformance.sh

```bash
# Interactive mode
bash conformance.sh
# Select: 2 (Device Supplier)
# Select: 2 (Provide your own test scenarios)
# Enter: my_test_scenarios.json

# Or direct command
bash conformance.sh device "/path/to/my_test_scenarios.json"
```

---

## Template Structure

The template file contains:

```json
[
  {
    "id": "scenario-unique-id",
    "name": "Human Readable Scenario Name",
    "description": "What this scenario validates",
    "steps": [
      {
        "id": "step-1.1",
        "name": "Step Description",
        "method": "GET|POST|PUT",
        "endpoint": "/api/v1/path",
        "request_body": { ... },  // Optional for GET
        "headers": { ... },        // Optional
        "expected_status": 200,
        "validations": [ ... ],    // Optional
        "extract_context": { ... }, // Optional
        "skip_signing": false,     // Optional
        "skip_certificate_injection": false  // Optional
      }
    ]
  }
]
```

---

## Field Reference

### Scenario Fields

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `id` | Yes | string | Unique identifier (e.g., `scenario-user-flow`) |
| `name` | Yes | string | Display name shown in reports |
| `description` | Yes | string | One-line explanation of what scenario tests |
| `steps` | Yes | array | Ordered list of API calls to make |

### Step Fields

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `id` | Yes | string | Unique step ID (e.g., `step-1.1` for scenario 1, step 1) |
| `name` | Yes | string | Display name for the step |
| `method` | Yes | string | HTTP method: `GET`, `POST`, or `PUT` |
| `endpoint` | Yes | string | URL path (relative to API base) |
| `request_body` | No | object | JSON body for POST/PUT requests |
| `headers` | No | object | Custom HTTP headers |
| `expected_status` | Yes | number | Expected HTTP response code |
| `validations` | No | array | Field checks on the response |
| `extract_context` | No | object | Save response values for later steps |
| `skip_signing` | No | boolean | Omit RFC 9421 signature (default: false) |
| `skip_certificate_injection` | No | boolean | Don't load cert from file (default: false) |

---

## Validations

Check response fields with validation rules:

```json
"validations": [
  {
    "field": "fieldName",
    "operation": "equals|contains|not_empty|exists|is_string|is_number|is_array",
    "value": "optional-value-for-equals-and-contains"
  }
]
```

### Validation Operations

| Operation | Value Required? | Passes When |
|-----------|----------------|-------------|
| `equals` | Yes | Field exactly matches value |
| `contains` | Yes | Field contains value as substring |
| `not_empty` | No | Field is present and not empty |
| `exists` | No | Field is present (any value) |
| `is_string` | No | Field is a JSON string |
| `is_number` | No | Field is a JSON number |
| `is_array` | No | Field is a JSON array |

### Field Path Examples

```json
"field": "clientId"           // Top-level field
"field": "data.userId"         // Nested field
"field": "items.0.id"          // First array element's id
"field": "_headers.ETag"       // HTTP response header
```

---

## Context Extraction

Pass data from one step to the next:

```json
// Step 1: Save clientId
"extract_context": {
  "clientId": "clientId",
  "manifestEtag": "_headers.ETag"
}

// Step 2: Use {placeholder}
"endpoint": "/api/v1/clients/{clientId}/deployments",
"headers": {
  "If-None-Match": "{manifestEtag}"
}
```

---

## Complete Example

```json
[
  {
    "id": "scenario-full-workflow",
    "name": "Complete Device Workflow",
    "description": "Full device onboarding and deployment flow",
    "steps": [
      {
        "id": "step-1.1",
        "name": "Onboard Device",
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
          { "field": "clientId", "operation": "is_string" },
          { "field": "status", "operation": "exists" }
        ],
        "extract_context": {
          "clientId": "clientId",
          "token": "token"
        }
      },
      {
        "id": "step-1.2",
        "name": "Get Capabilities",
        "method": "GET",
        "endpoint": "/api/v1/capabilities",
        "expected_status": 200,
        "validations": [
          { "field": "capabilities", "operation": "is_array" }
        ]
      },
      {
        "id": "step-1.3",
        "name": "Request Deployment",
        "method": "POST",
        "endpoint": "/api/v1/clients/{clientId}/deployments",
        "request_body": {
          "workloads": ["wf-scheduler", "wf-executor"]
        },
        "expected_status": 200,
        "validations": [
          { "field": "deploymentId", "operation": "is_string" }
        ]
      }
    ]
  },
  {
    "id": "scenario-error-handling",
    "name": "Error Handling",
    "description": "Test error responses",
    "steps": [
      {
        "id": "step-2.1",
        "name": "Invalid Request",
        "method": "POST",
        "endpoint": "/api/v1/onboarding",
        "request_body": {
          "invalid": "payload"
        },
        "expected_status": 400,
        "validations": [
          { "field": "error", "operation": "exists" }
        ]
      }
    ]
  }
]
```

---

## Tips & Best Practices

### Naming Conventions

- **Scenario IDs**: Use hyphens: `scenario-user-onboarding`
- **Step IDs**: Use format `step-X.Y` where X is scenario number, Y is step number
- **Variable names**: Use camelCase: `clientId`, `deploymentToken`

### Request Bodies

```json
// Option 1: Inline JSON object
"request_body": {
  "name": "John",
  "age": 30
}

// Option 2: With file reference
"request_body": {
  "certificate": "./certs/device-cert.pem"
}
```

### Headers with Variables

```json
"headers": {
  "Authorization": "Bearer {token}",
  "X-Client-Id": "{clientId}",
  "If-Match": "{etag}"
}
```

### Negative Tests

```json
{
  "id": "step-error",
  "name": "Expected Failure",
  "method": "POST",
  "endpoint": "/api/v1/invalid",
  "expected_status": 404,  // Expect failure
  "validations": [
    { "field": "error", "operation": "contains", "value": "Not Found" }
  ]
}
```

---

## Validation Example

```bash
# Check your JSON syntax
jq empty my_test_scenarios.json

# View structure
jq '.[].name' my_test_scenarios.json

# Count scenarios
jq 'length' my_test_scenarios.json

# See all step names
jq '.[] | {scenario: .id, steps: [.steps[].name]}' my_test_scenarios.json
```

---

## Common Errors

### Error: "Invalid JSON"
**Solution:** Validate with jq:
```bash
jq empty my_test_scenarios.json
```

### Error: "Field not found in response"
**Solution:** Check response first by looking at test output, then adjust field path

### Error: "Expected status 200, got 500"
**Solution:** Check `expected_status` matches what server returns. For error tests, set the status you expect (e.g., 400, 404)

---

## File Location

Place custom test scenarios in any directory, then use full path when running:

```bash
# Examples
bash conformance.sh device ~/my_tests/custom_scenarios.json
bash conformance.sh device ./test-data/scenarios.json
bash conformance.sh device /tmp/new_scenarios.json
```

Or copy to device-supplier directory for easy reference:
```bash
cp my_test_scenarios.json device-supplier/device-scenarios/
bash conformance.sh device device-supplier/device-scenarios/my_test_scenarios.json
```

---

## Next Steps

1. **Create your scenarios** - Copy template and customize
2. **Validate JSON** - Run `jq empty` on your file
3. **Generate test cases** - Use conformance.sh option 2
4. **View template example** - See [TEMPLATE_custom_test_scenario.json](TEMPLATE_custom_test_scenario.json)
5. **Full schema docs** - See [Final-Summary.md](../Final-Summary.md) for complete reference

---

**Last Updated:** May 26, 2026  
**Version:** 1.0  
**Status:** ✅ Ready for use
