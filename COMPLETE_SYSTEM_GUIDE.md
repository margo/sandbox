# Complete Conformance Testing System Guide

**Document Version**: 1.0  
**Last Updated**: June 14, 2026  
**Status**: ✅ Production Ready

---

## Table of Contents

1. [System Overview](#system-overview)
2. [Architecture](#architecture)
3. [WFM Supplier Persona](#wfm-supplier-persona)
4. [Device Supplier Persona](#device-supplier-persona)
5. [Group-Based Testing](#group-based-testing)
6. [Module Interactions](#module-interactions)
7. [Directory Structure](#directory-structure)
8. [Commands Reference](#commands-reference)
9. [Complete Workflows](#complete-workflows)
10. [Troubleshooting](#troubleshooting)

---

## System Overview

### Purpose

This conformance testing system allows Margo vendors to:
- **Generate** test cases for their persona (WFM Supplier or Device Supplier)
- **Execute** test cases against their implementations
- **Group** tests for organized, repeatable testing
- **Generate Reports** with detailed pass/fail results

### Two-Phase Architecture

| Phase | Component | Purpose |
|-------|-----------|---------|
| **Phase 1: Generation** | `conformance.sh` | Create/organize test cases into groups |
| **Phase 2: Execution** | `run-tests.sh` | Execute tests and generate reports |

### Supported Personas

1. **WFM Supplier** - Workflow Management Provider
2. **Device Supplier** - IoT Device Management Provider

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    User Interaction (CLI)                        │
└────────────────┬────────────────────────────────────────────────┘
                 │
        ┌────────┴────────┐
        │                 │
        ▼                 ▼
   ┌─────────────┐   ┌──────────────┐
   │ Phase 1:    │   │ Phase 2:     │
   │ conformance │   │ run-tests    │
   │ .sh         │   │ .sh          │
   └──────┬──────┘   └──────┬───────┘
          │                 │
          │ Generates       │ Uses
          ▼                 ▼
    ┌──────────────────────────────┐
    │  Data-Generator/             │
    │  ├── wfm-supplier/           │ ◄─── Test Collections & Groups
    │  │   ├── postman_collection  │
    │  │   ├── groups/             │
    │  │   │   ├── group1/         │
    │  │   │   └── group2/         │
    │  │   └── test-scenarios.json │
    │  │                           │
    │  └── device-supplier/        │
    │      ├── test-scenarios.json │
    │      ├── groups/             │
    │      │   ├── diamonddevice/  │
    │      │   └── silver/         │
    │      └── postman_collection  │
    └──────────────────────────────┘
          │         │
          │         └─────────────────┐
          │                           │
          ▼                           ▼
    ┌──────────────┐         ┌───────────────┐
    │ Executables  │         │ Test Runners  │
    ├──────────────┤         ├───────────────┤
    │ Newman       │         │ run_tests.go  │
    │ (WFM)        │         │ (Device)      │
    └──────────────┘         └───────────────┘
          │                           │
          └─────────────┬─────────────┘
                        │
                        ▼
                ┌──────────────────┐
                │  HTML Reports    │
                │  JSON Results    │
                │  Test Metrics    │
                └──────────────────┘
```

---

## WFM Supplier Persona

### Overview

WFM Supplier tests validate Workflow Management implementations using **Postman collections** and **Newman test runner**.

### Phase 1: Test Generation (conformance.sh)

#### Menu Flow

```
┌─ Select "1. WFM Supplier"
│
├─ Select test type:
│  ├─ 1) OpenAPI Spec (URL or local file)
│  └─ 2) Postman Collection
│
├─ Enter spec/collection path
│
└─ Tests copied to Data-Generator/wfm-supplier/
```

#### Generated Files

```
Data-Generator/wfm-supplier/
├── postman_collection.json      (Postman API definitions)
├── newman-data/                 (Test data)
│   ├── device-agent.env.json    (Environment variables)
│   ├── certs/                   (TLS certificates)
│   └── ...
└── groups/                      (Organized test groups)
    ├── group1/
    │   ├── group.json           (Metadata)
    │   ├── postman_collection.json
    │   └── test-results/
    └── group2/
        └── ...
```

#### Key Functions

| Function | Purpose | Location |
|----------|---------|----------|
| `show_wfm_options_menu()` | Display WFM test type options | line ~166 |
| `generate_wfm_tests()` | Create/validate WFM test cases | line ~383 |
| `select_wfm_group()` | Interactive WFM group picker | run-tests.sh:72 |
| `group_management_menu()` | Create/select/delete groups | line ~773 |

### Phase 2: Test Execution (run-tests.sh)

#### Menu Flow

```
┌─ Select "1. WFM Supplier"
│
├─ Select test type:
│  ├─ 1) OpenAPI Spec
│  └─ 2) Functional/Template
│
├─ Enter WFM endpoint URL (https://...)
│
├─ Select group (1, 2, etc.)
│
└─ Tests execute with Newman
    └─ HTML report generated
```

#### Execution Process

```
1. WFM endpoint URL provided
   ↓
2. Filter Postman collection by group
   ↓
3. Populate environment variables
   ├─ clientId
   ├─ deploymentId
   └─ digest values
   ↓
4. Newman runs tests
   ├─ Sends HTTP requests
   ├─ Validates status codes
   ├─ Checks response structure
   └─ Generates assertions
   ↓
5. Results saved
   ├─ JSON (test-results.json)
   ├─ HTML (test-results.html)
   └─ Summary displayed
```

#### Key Functions

| Function | Purpose | Location |
|----------|---------|----------|
| `execute_wfm_tests()` | Run Newman with collection | line ~252 |
| `prepare_environment()` | Set up env variables | line ~286 |
| `patch_url_variables()` | Replace {{variable}} in URLs | line ~298 |
| `filter_postman_collection()` | Extract group tests | run-tests.sh:207 |

#### Example Output

```
▶ Testing WFM Supplier
📋 Available WFM Test Groups:
================================================
  1) functional         (v1.0.0) - 45 tests
  2) template-advanced  (v1.5.0) - 32 tests

Select group (1-2): 1
[2026-06-14 15:00:01] 📝 Running Newman tests...
[2026-06-14 15:00:05] ✅ 45 tests passed, 0 failed
[2026-06-14 15:00:06] 📝 Report saved to: Runner/wfm-supplier/test-results.html
```

---

## Device Supplier Persona

### Overview

Device Supplier tests validate IoT device management using **custom test scenarios** (JSON-based) and **Go-based test runner**.

### Phase 1: Test Generation (conformance.sh)

#### Menu Flow

```
┌─ Select "2. Device Supplier"
│
├─ Go directly to group management
│
├─ Select group:
│  ├─ 0) Create new group
│  └─ 1-N) Select existing group
│
└─ Tests organized into group
    ├─ group.json (metadata)
    └─ test-scenarios.json (test definitions)
```

#### Generated Files

```
Data-Generator/device-supplier/
├── test-scenarios.json           (Current test scenarios)
├── groups/                       (Organized test groups)
│   ├── diamonddevice/
│   │   ├── group.json           (name, version, test count)
│   │   ├── test-scenarios.json  (102 test cases)
│   │   └── postman_collection.json
│   └── silver/
│       ├── group.json
│       ├── test-scenarios.json
│       └── postman_collection.json
└── device-scenarios/            (Default scenarios)
    └── test-scenarios.json
```

#### Test Scenario Structure

```json
{
  "scenarios": [
    {
      "id": "scenario-onboarding",
      "name": "Device Onboarding",
      "description": "...",
      "steps": [
        {
          "id": "step-1",
          "name": "Get Root CA Certificate",
          "method": "GET",
          "endpoint": "/api/v1/onboarding/certificate",
          "expected_status": 200,
          "validations": [
            {
              "field": "certificate",
              "operation": "is_string"
            }
          ]
        }
      ]
    }
  ]
}
```

#### Key Functions

| Function | Purpose | Location |
|----------|---------|----------|
| `create_test_group()` | Interactive group creation | line ~636 |
| `group_management_menu()` | Manage groups (create/select/delete) | line ~773 |
| `list_test_groups()` | List available groups | line ~838 |
| `delete_test_group()` | Remove group with confirmation | line ~847 |

### Phase 2: Test Execution (run-tests.sh)

#### Menu Flow

```
┌─ Select "2. Device Supplier"
│
├─ Show test scenarios menu
│  └─ "1. Group-based test scenarios"
│
├─ Select group:
│  ├─ Available groups displayed
│  └─ User picks group (1, 2, etc.)
│
└─ Go binary executes tests
    ├─ Reads test-scenarios.json
    ├─ Connects to mock WFM
    ├─ Executes HTTP requests
    ├─ Validates responses
    └─ Generates report
```

#### Execution Process

```
1. Group selected from available list
   ↓
2. Load test-scenarios.json from group
   ↓
3. Start mock WFM server (background)
   ├─ Port: 3001
   └─ URL: https://localhost:3001/v1alpha2/margo
   ↓
4. Run Go test binary
   ├─ Reads test scenarios
   ├─ Interpolates context variables
   ├─ Signs requests (RFC 9421)
   ├─ Injects certificates
   └─ Validates responses
   ↓
5. Results collected
   ├─ Status code validation
   ├─ Response field validation
   ├─ Assertion results
   └─ Context extraction for next steps
   ↓
6. Report generated
   ├─ JSON with full details
   ├─ Summary with pass/fail counts
   └─ HTML report (if configured)
```

#### Key Functions

| Function | Purpose | Location |
|----------|---------|----------|
| `execute_device_tests()` | Orchestrate device testing | run-tests.sh:524 |
| `select_device_group()` | Interactive group selector | run-tests.sh:140 |
| `executeStep()` | Run individual test step | run_tests.go:188 |
| `validateResponse()` | Check response against validations | run_tests.go:530 |

#### Example Output

```
Which test scenarios would you like to run?
1. Group-based test scenarios (select from available groups)

Q) Quit

Select option (1 or Q): 1

📋 Available Device Test Groups:
================================================
  1) diamonddevice   (v1.0.0) - 102 tests
  2) silver          (v1.1.1) - 102 tests

Select group (1-2): 1
[2026-06-14 14:05:32] 📝 🚀 Starting Device Supplier Test Execution
[2026-06-14 14:05:34] ✅ Mock WFM Server started (PID: 3806876)
[2026-06-14 14:05:34] 📝 ▶️  Running Device Conformance Tests...

▶ Running Scenario: Device Onboarding (scenario-onboarding)
  Description: Certificate retrieval plus successful and rejected onboarding flows.
  → Step: Get Root CA Certificate
    ✅ PASS - HTTP 200 (Expected: 200)
  → Step: Onboard Trusted Device
    ✅ PASS - HTTP 201 (Expected: 201)
  → Step: Reject Blocklisted Certificate
    ✅ PASS - HTTP 403 (Expected: 403)

║  Test Results: 102 PASSED, 0 FAILED (Total: 102)
```

---

## Group-Based Testing

### What are Groups?

Groups are collections of related test cases organized together for:
- **Focused Testing**: Run subset of tests relevant to your implementation
- **Repeatability**: Same group runs identically each time
- **Organization**: Logical grouping by feature/component
- **Versioning**: Track test suite versions

### Group Structure

```
groups/
├── {group-name}/
│   ├── group.json              ← Metadata
│   ├── test-scenarios.json     ← Test definitions (Device Supplier)
│   └── postman_collection.json ← API definitions (WFM Supplier)
```

### group.json Format

```json
{
  "name": "diamonddevice",
  "version": "1.0.0",
  "persona": "device-supplier",
  "description": "Diamond Device test suite",
  "testCases": [
    "uuid-1",
    "uuid-2",
    "uuid-3"
  ]
}
```

### Group Lifecycle

#### 1. Create Group (conformance.sh)

```bash
cd conformance
./conformance.sh
# Select persona (1 for WFM, 2 for Device)
# Select group option
# Select "0. Create new group"
# Answer prompts:
#   - Group name?
#   - Group version?
#   - Group description?
#   - Directory with test files?
```

**Result**: New group created in `Data-Generator/{persona}/groups/{group-name}/`

#### 2. List Groups

```bash
# Via script menu:
./conformance.sh -> Select persona -> See available groups

# Via directory:
ls Data-Generator/device-supplier/groups/
ls Data-Generator/wfm-supplier/groups/
```

#### 3. Select Group (run-tests.sh)

```bash
./run-tests.sh
# Select persona (1 for WFM, 2 for Device)
# Group selector shows available groups
# Select group number (1, 2, etc.)
```

**Result**: Tests from selected group execute

#### 4. Delete Group (conformance.sh)

```bash
./conformance.sh
# Select persona
# At group menu, select group then option to delete
# Confirm deletion
```

---

## Module Interactions

### Data Flow Diagram

```
Phase 1: Generation
──────────────────

User Input
    │
    ▼
conformance.sh
    ├─ show_persona_menu()
    ├─ parse_input()
    └─ route_to_persona_handler()
        │
        ├─ WFM Path:
        │  ├─ generate_wfm_tests()
        │  ├─ process_postman_collection()
        │  └─ setup_wfm_groups()
        │
        └─ Device Path:
           ├─ group_management_menu()
           ├─ create_test_group()
           └─ save_group_metadata()
    │
    ▼
Data-Generator/{persona}/
    ├─ postman_collection.json
    ├─ test-scenarios.json
    └─ groups/{group-name}/
        ├─ group.json
        └─ test-scenarios.json / postman_collection.json

──────────────────────────────────────────────────

Phase 2: Execution
──────────────────

User Input
    │
    ▼
run-tests.sh
    ├─ show_persona_menu()
    ├─ parse_input()
    └─ route_to_persona_handler()
        │
        ├─ WFM Path:
        │  ├─ select_wfm_group()
        │  ├─ read_environment_variables()
        │  ├─ filter_postman_collection()
        │  └─ execute_wfm_tests()
        │      │
        │      └─ Newman (HTTP test runner)
        │          ├─ Load collection
        │          ├─ Set environment
        │          ├─ Run requests
        │          └─ Validate responses
        │
        └─ Device Path:
           ├─ select_device_group()
           ├─ load_test_scenarios()
           ├─ start_mock_server()
           └─ execute_device_tests()
               │
               └─ run_tests.go (Go binary)
                   ├─ Parse test scenarios
                   ├─ Execute HTTP requests
                   ├─ Sign requests (RFC 9421)
                   ├─ Inject certificates
                   └─ Validate responses
    │
    ▼
Results
    ├─ HTML Report
    ├─ JSON Results
    └─ Summary (PASSED/FAILED)
```

---

## Directory Structure

### Complete Layout

```
conformance-persona-test/
│
├── conformance/                          # Phase 1 & 2 for standard HTTPS
│   ├── conformance.sh                    # ◄─ START HERE (Generation)
│   ├── run-tests.sh                      # ◄─ START HERE (Execution)
│   ├── Data-Generator/
│   │   ├── wfm-supplier/
│   │   │   ├── postman_collection.json
│   │   │   ├── test-scenarios.json
│   │   │   ├── newman-data/
│   │   │   │   ├── device-agent.env.json
│   │   │   │   └── certs/
│   │   │   │       ├── ca-cert.pem
│   │   │   │       └── device-key.pem
│   │   │   └── groups/
│   │   │       ├── functional/
│   │   │       │   ├── group.json
│   │   │       │   └── postman_collection.json
│   │   │       └── template-advanced/
│   │   │           └── ...
│   │   └── device-supplier/
│   │       ├── postman_collection.json
│   │       ├── test-scenarios.json
│   │       ├── groups/
│   │       │   ├── diamonddevice/
│   │       │   │   ├── group.json
│   │       │   │   ├── test-scenarios.json
│   │       │   │   └── postman_collection.json
│   │       │   └── silver/
│   │       │       └── ...
│   │       └── device-scenarios/
│   │           ├── test-scenarios.json
│   │           └── TEMPLATE_custom_test_scenario.json
│   ├── device-supplier/
│   │   ├── run_tests.go                  # ◄─ Device test executor (Go)
│   │   ├── bin/
│   │   │   ├── run_tests                 # Compiled binary
│   │   │   └── server                    # Mock WFM server
│   │   ├── certs/
│   │   │   ├── ca-cert.pem
│   │   │   └── device-key.pem
│   │   ├── cmd/
│   │   │   └── device-supplier/
│   │   │       └── main.go               # Mock WFM server source
│   │   └── device-scenarios/
│   │       └── test-scenarios.json       # Current test scenarios (working copy)
│   ├── wfm-supplier/
│   │   ├── newman-data/                  # Newman environment & certs
│   │   └── signing_proxy.go              # RFC 9421 signing proxy (unused for standard HTTPS)
│   └── Runner/                           # Test execution results
│       ├── wfm-supplier/
│       │   ├── test-results.json
│       │   └── test-results.html
│       └── device-supplier/
│           ├── test-results.json
│           └── test-results.html
│
├── conformance-2/                        # Phase 1 & 2 with RFC 9421 signing proxy
│   ├── conformance.sh                    # Same as conformance/
│   ├── run-tests.sh                      # Same as conformance/
│   ├── Data-Generator/                   # Same structure as conformance/
│   ├── device-supplier/                  # Same structure as conformance/
│   ├── wfm-supplier/
│   │   ├── signing_proxy.go              # ◄─ RFC 9421 signing proxy (active here)
│   │   └── signing_proxy (binary)        # Compiled proxy
│   └── Runner/                           # Test results
│
├── COMPLETE_SYSTEM_GUIDE.md              # ◄─ THIS FILE (Full documentation)
├── GROUP_TESTING_QUICK_REFERENCE.md      # Quick start for groups
└── README.md                             # General overview
```

### Key Differences: conformance vs conformance-2

| Aspect | conformance | conformance-2 |
|--------|-------------|---------------|
| **TLS** | Standard HTTPS (self-signed) | Standard HTTPS (self-signed) |
| **Signing** | Direct endpoint on port 8082 | RFC 9421 signing proxy on port 18082 |
| **Use Case** | Basic testing | RFC 9421 signed requests |
| **Data** | Identical in both | Identical in both |

---

## Commands Reference

### Generation Phase

#### WFM Supplier - OpenAPI Spec

```bash
cd conformance
./conformance.sh
# Select: 1 (WFM Supplier)
# Select: 1 (OpenAPI Spec)
# Enter: URL or path to OpenAPI spec
# Tests generated in Data-Generator/wfm-supplier/
```

#### WFM Supplier - Postman Collection

```bash
cd conformance
./conformance.sh
# Select: 1 (WFM Supplier)
# Select: 2 (Postman Collection)
# Enter: Path to postman_collection.json
# Tests generated in Data-Generator/wfm-supplier/
```

#### Device Supplier - Create Group

```bash
cd conformance
./conformance.sh
# Select: 2 (Device Supplier)
# At group menu, select: 0 (Create new group)
# Enter: Group name (e.g., "my-test-group")
# Enter: Version (e.g., "1.0.0")
# Enter: Description
# Enter: Directory with test-scenarios.json
# Group created in Data-Generator/device-supplier/groups/{name}/
```

#### Device Supplier - Select Existing Group

```bash
cd conformance
./conformance.sh
# Select: 2 (Device Supplier)
# At group menu, select: 1 or 2 (existing group)
# Tests from group loaded to Data-Generator/device-supplier/
```

### Execution Phase

#### WFM Supplier - Run Tests

```bash
cd conformance
./run-tests.sh
# Select: 1 (WFM Supplier)
# Select: Test type (1 for OpenAPI, 2 for Functional)
# Enter: WFM endpoint URL (e.g., https://my-wfm.example.com:8082)
# Select: Group number
# Tests run with Newman
# Report: Runner/wfm-supplier/test-results.html
```

#### Device Supplier - Run Tests

```bash
cd conformance
./run-tests.sh
# Select: 2 (Device Supplier)
# Menu shows: "1. Group-based test scenarios"
# Select: 1
# Select: Group number (1, 2, etc.)
# Tests execute
# Report: Runner/device-supplier/test-results.json
```

### Direct Binary Execution

#### Device Test Runner (Direct)

```bash
cd conformance/device-supplier
./bin/run_tests -scenario scenario-onboarding -step step-1.1
```

#### Mock WFM Server (Direct)

```bash
cd conformance/device-supplier
./bin/server &
# Server starts on https://localhost:3001/v1alpha2/margo
```

---

## Complete Workflows

### Workflow 1: WFM Supplier - Full Test Cycle

```bash
# Step 1: Generate tests
cd /home/margo/conformance-persona-test/conformance
./conformance.sh
# Input: 1 (WFM Supplier)
# Input: 1 (OpenAPI Spec)
# Input: /path/to/openapi.yaml
# Result: Tests in Data-Generator/wfm-supplier/

# Step 2: Create a test group (optional, for organization)
./conformance.sh
# Input: 1 (WFM Supplier)
# Input: 2 (Functional/Template)
# Input: /path/to/postman_collection.json
# At group menu: 0 (Create new)
# Input: my-wfm-tests, 1.0.0, My WFM tests
# Input: Data-Generator/wfm-supplier/
# Result: Group created

# Step 3: Run tests
./run-tests.sh
# Input: 1 (WFM Supplier)
# Input: Test type
# Input: https://my-wfm.example.com:8082 (your WFM endpoint)
# Input: Group number
# Result: HTML report in Runner/wfm-supplier/test-results.html
```

### Workflow 2: Device Supplier - Full Test Cycle

```bash
# Step 1: Generate tests from existing group
cd /home/margo/conformance-persona-test/conformance
./conformance.sh
# Input: 2 (Device Supplier)
# At group menu: 1 (Select diamonddevice group)
# Result: Tests loaded from diamonddevice group

# Step 2: Run tests
./run-tests.sh
# Input: 2 (Device Supplier)
# Input: 1 (Group-based test scenarios)
# Input: 1 (diamonddevice group)
# Result: JSON report in Runner/device-supplier/

# Step 3: Create custom group
./conformance.sh
# Input: 2 (Device Supplier)
# At group menu: 0 (Create new)
# Input: my-devices, 1.0.0, Custom device tests
# Input: /path/to/custom/test-scenarios/
# Result: New group created

# Step 4: Run custom group
./run-tests.sh
# Input: 2 (Device Supplier)
# Input: 1 (Group-based test scenarios)
# Input: 3 (my-devices group)
# Result: Tests from custom group execute
```

### Workflow 3: RFC 9421 Signed Requests (conformance-2)

```bash
# Use conformance-2 for RFC 9421 support
cd /home/margo/conformance-persona-test/conformance-2

# All steps same as conformance/
./conformance.sh
./run-tests.sh

# Key difference: Requests routed through signing proxy
# Port: 18082 (instead of 8082)
# Automatically signs with RFC 9421 headers
```

---

## Troubleshooting

### Issue: "No groups found"

**Cause**: No groups directory or no group.json files

**Solution**:
```bash
# Check groups directory exists
ls -la conformance/Data-Generator/device-supplier/groups/

# Should show:
# diamonddevice/
# silver/

# If missing, regenerate tests
cd conformance
./conformance.sh
# Select: 2 (Device Supplier)
# Select existing group to load tests
```

### Issue: "Failed to sign request"

**Cause**: Missing device private key

**Solution**:
```bash
# Check certificate files
ls -la conformance/device-supplier/certs/

# Should have:
# device-key.pem
# device-cert.pem

# If missing, regenerate
cd conformance/device-supplier
bash generate-certs.sh certs localhost
```

### Issue: "Invalid option" in menu

**Cause**: Input not recognized or case-sensitivity issue

**Solution**:
- Inputs are case-insensitive (1, Q, b all work)
- Use only provided options (1-N, B, Q, H)
- No spaces around input

### Issue: Tests showing "FAIL - Invalid status code"

**Cause**: Response status doesn't match expected_status in test definition

**Check**:
```bash
# View expected vs actual
# Check test-scenarios.json for expected_status
cat Data-Generator/device-supplier/test-scenarios.json | jq '.[].steps[].expected_status'

# Verify endpoint is responding correctly
curl -k https://localhost:3001/v1alpha2/margo/api/v1/onboarding/certificate
```

### Issue: Mock server won't start

**Cause**: Port 3001 already in use

**Solution**:
```bash
# Check what's using port 3001
lsof -i :3001

# Kill existing process
kill -9 <PID>

# Then retry run-tests.sh
```

### Issue: "Permission denied" running scripts

**Solution**:
```bash
chmod +x /home/margo/conformance-persona-test/conformance/*.sh
chmod +x /home/margo/conformance-persona-test/conformance-2/*.sh
chmod +x /home/margo/conformance-persona-test/conformance/device-supplier/bin/*
```

---

## Key Concepts Summary

### Test Scenarios (Device Supplier)

**Definition**: JSON-based test definitions specifying:
- Request method (GET, POST, etc.)
- Endpoint path
- Request body
- Expected status code
- Response validations
- Context extraction for subsequent steps

**Example**:
```json
{
  "id": "step-1",
  "name": "Get Certificate",
  "method": "GET",
  "endpoint": "/api/v1/onboarding/certificate",
  "expected_status": 200,
  "validations": [
    {"field": "certificate", "operation": "is_string"}
  ]
}
```

### Postman Collections (WFM Supplier)

**Definition**: OpenAPI-compatible collection with:
- Endpoint definitions
- HTTP methods
- Request/response schemas
- Environment variables
- Pre/post-request scripts

**Used by**: Newman test runner for automated API testing

### Group JSON

**Definition**: Metadata file describing test group:
```json
{
  "name": "group-name",
  "version": "1.0.0",
  "persona": "device-supplier",
  "description": "Group description",
  "testCases": ["uuid1", "uuid2"]
}
```

### Environment Variables

**WFM Supplier**:
- `clientId`: Client identifier
- `deploymentId`: Deployment identifier
- `deviceId`: Device identifier

**Device Supplier**:
- Context variables extracted from previous step responses
- Used for interpolation in subsequent steps

---

## Getting Help

### Documentation

- **This File**: Complete system architecture and workflows
- `GROUP_TESTING_QUICK_REFERENCE.md`: Quick start for group-based testing
- `README.md`: General overview

### Verify Setup

```bash
# Check all scripts have execute permission
ls -l conformance/*.sh conformance/device-supplier/bin/*

# Verify directory structure
find conformance/Data-Generator -type d | head -20

# Check Go binary exists
file conformance/device-supplier/bin/run_tests

# Test basic menu (no input, just show menu)
./conformance.sh < /dev/null
```

### Test Data Locations

| Component | Location |
|-----------|----------|
| WFM Groups | `conformance/Data-Generator/wfm-supplier/groups/` |
| Device Groups | `conformance/Data-Generator/device-supplier/groups/` |
| WFM Certs | `conformance/wfm-supplier/newman-data/certs/` |
| Device Certs | `conformance/device-supplier/certs/` |
| Test Results | `conformance/Runner/` |

---

## Quick Reference Commands

```bash
# Generate WFM tests
cd conformance && ./conformance.sh # Then: 1 → 2

# Generate Device tests
cd conformance && ./conformance.sh # Then: 2 → select group

# Run WFM tests
cd conformance && ./run-tests.sh # Then: 1 → select group

# Run Device tests
cd conformance && ./run-tests.sh # Then: 2 → 1 → select group

# Direct device test execution
cd conformance/device-supplier && ./bin/run_tests

# With specific scenario filter
cd conformance/device-supplier && ./bin/run_tests -scenario scenario-onboarding

# Check available groups
ls conformance/Data-Generator/device-supplier/groups/
ls conformance/Data-Generator/wfm-supplier/groups/

# View group metadata
cat conformance/Data-Generator/device-supplier/groups/diamonddevice/group.json
```

---

## Document History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026-06-14 | Initial complete system guide |

---

**Status**: ✅ Complete and Production Ready

For questions or updates, refer to this guide's architecture section for detailed module interactions.
