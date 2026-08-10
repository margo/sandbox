# Margo Conformance Testing Framework - Complete Overview

**Understanding the entire Margo ecosystem, personas, test strategies, and how everything works together**

---

## Table of Contents

1. [What is Margo?](#what-is-margo)
2. [Personas & Their Roles](#personas--their-roles)
3. [Two-CLI Architecture](#two-cli-architecture)
4. [Complete End-to-End Workflow](#complete-end-to-end-workflow)
5. [WFM Supplier Workflow](#wfm-supplier-workflow)
6. [Device Supplier Workflow](#device-supplier-workflow)
7. [System Components & Communication](#system-components--communication)
8. [Key Concepts](#key-concepts)
9. [Test Data Flow](#test-data-flow)
10. [Environment Setup](#environment-setup)

---

## What is Margo?

**Margo** is an open specification for managing heterogeneous edge devices in industrial and enterprise environments.

### Core Purpose
- Allows organizations to manage multiple types of devices (Kubernetes clusters, standalone containers, Docker environments)
- Provides standardized APIs for device communication, workload deployment, and capability reporting
- Ensures consistent behavior across different device implementations

### Three Key Roles

| Role | Function | Example |
|------|----------|---------|
| **WFM (Workload Fleet Manager)** | Central control server that manages devices and workloads | Symphony API server |
| **Device** | Edge device that connects to WFM and executes workloads | Kubernetes cluster, Docker standalone device |
| **Tester** | Validates that implementations comply with Margo specification | You! Running these conformance tests |

---

## Personas & Their Roles

### 1. **WFM Supplier** (Workload Fleet Manager Provider)

**Who are they?**
- Organizations building WFM server implementations
- Want to prove their server correctly implements Margo specification

**What they test:**
- ✅ Correct API endpoints (OpenAPI contract)
- ✅ Proper HTTP status codes
- ✅ Valid request/response format
- ✅ Correct data handling

**Example:** "Siemens built a WFM called Symphony. We need to prove it works with Margo specification."

**Test Method:**
- Import OpenAPI specification
- Generate Postman collection (8 test requests)
- Execute against running WFM server
- Generate conformance report

**Key Communication Pattern:**
```
Test Client (Postman/Newman)
         ↓ POST /api/v1/onboarding
    Margo WFM Server
         ↓ Returns {clientId}
Test Client uses clientId in subsequent requests
         ↓ GET /api/v1/clients/{clientId}/capabilities
    Margo WFM Server
```

---

### 2. **Device Supplier** (Edge Device Provider)

**Who are they?**
- Organizations building device implementations (industrial IoT, edge computing)
- Want to prove their device correctly implements Margo specification

**What they test:**
- ✅ Proper device onboarding flow
- ✅ RFC 9421 HTTP message signatures (for authentication)
- ✅ Capability reporting to WFM
- ✅ Deployment acceptance and execution
- ✅ Status update reporting

**Example:** "We built an edge device that runs Kubernetes. We need to prove it correctly onboards and communicates with WFM."

**Test Method:**
- Use test scenarios (43 predefined test cases)
- Run against mock WFM server
- Validate RFC 9421 signature generation
- Generate conformance report

**Key Communication Pattern:**
```
Device Agent (Client)
         ↓ POST /api/v1/onboarding (with certificate)
    Mock WFM Server
         ↓ Returns {clientId}
Device stores clientId for future requests
         ↓ POST /api/v1/clients/{clientId}/capabilities (signed)
    Mock WFM Server verifies signature
         ↓ Returns confirmation
Device updates status periodically
```

---

## Two-CLI Architecture

The conformance testing framework uses **two specialized CLIs** with clear separation of concerns:

### CLI #1: `conformance.sh` (Preparation Phase)

**Purpose:** Generate test data based on what you want to test

**What it does:**
1. Takes input (OpenAPI spec for WFM, test scenarios for Device)
2. Generates/validates test cases
3. Prepares test data, certificates, environment files
4. Saves everything in `Data-Generator/`

**Flow:**
```
┌──────────────────────────┐
│ Input                    │
├──────────────────────────┤
│ OpenAPI Spec (WFM)       │
│ or Test Scenarios (Dev)  │
└──────────────┬───────────┘
               ↓
┌──────────────────────────────────────────┐
│ conformance.sh (CLI #1)                  │
├──────────────────────────────────────────┤
│ • Download/parse OpenAPI spec (WFM)      │
│ • Generate Postman collection            │
│ • Create test data & certificates        │
│ • Validate test scenarios (Device)       │
│ • Prepare environment files              │
└──────────────┬──────────────────────────┘
               ↓
┌──────────────────────────┐
│ Output: Data-Generator/  │
├──────────────────────────┤
│ postman_collection.json  │
│ test-scenarios.json      │
│ newman-data/             │
│ certificates/            │
└──────────────────────────┘
```

### CLI #2: `run-tests.sh` (Execution Phase)

**Purpose:** Execute prepared tests against actual systems

**What it does:**
1. Locates test data from `Data-Generator/`
2. Sets up runtime environment
3. Executes tests (Newman for WFM, Mock Server for Device)
4. Generates conformance reports
5. Saves everything in `Runner/`

**Flow:**
```
┌──────────────────────────────────┐
│ Input: Data-Generator/           │
├──────────────────────────────────┤
│ Test data & certificates         │
└──────────────┬────────────────────┘
               ↓
┌──────────────────────────────────────────┐
│ run-tests.sh (CLI #2)                    │
├──────────────────────────────────────────┤
│ • Validate setup files                   │
│ • Install system requirements            │
│ • Prepare certificates                   │
│ • Setup runtime environment              │
│ • Execute tests                          │
│ • Generate conformance reports           │
└──────────────┬──────────────────────────┘
               ↓
┌──────────────────────────┐
│ Output: Runner/          │
├──────────────────────────┤
│ HTML Reports             │
│ Test Execution Logs      │
│ Results Summary          │
└──────────────────────────┘
```

---

## Complete End-to-End Workflow

### Universal Pattern

```
PHASE 1: GENERATE TEST CASES
┌─────────────────────────────────────────────────────────┐
│ Persona Selection → Input Configuration → Test Generation│
│ conformance.sh (CLI #1)                                  │
│ Output: Data-Generator/                                  │
└──────────────────────┬──────────────────────────────────┘
                       ↓
PHASE 2: EXECUTE TESTS
┌─────────────────────────────────────────────────────────┐
│ Component Information → Run Tests → Generate Reports     │
│ run-tests.sh (CLI #2)                                   │
│ Output: Runner/                                         │
└──────────────────────┬──────────────────────────────────┘
                       ↓
PHASE 3: REVIEW CONFORMANCE
┌─────────────────────────────────────────────────────────┐
│ Open HTML Report → Verify Test Results → Claim Compliance│
│ Browser / HTML Report                                    │
│ Files: wfm-test-report-*.html or conformance-report-*.html
└─────────────────────────────────────────────────────────┘
```

### Step-by-Step Example

**Example: Testing a WFM Server**

```
Step 1: Generate Tests
$ bash conformance.sh
> Select: 1 (WFM Supplier)
> Enter: https://symphony.machine:8082/v1alpha2/margo/api/v1/swagger.json
Output: Data-Generator/wfm-supplier/postman_collection.json (8 tests)

Step 2: Execute Tests
$ bash run-tests.sh
> Select: 1 (WFM Supplier)
> Enter: Component info (name, version, IP, port)
Output: Runner/wfm-supplier/report_20260528_120000.html

Step 3: Review Results
> Open HTML report in browser
> Check: 8/8 tests passed ✅
> Generate conformance claim
```

---

## WFM Supplier Workflow

### Detailed Process

#### 1. **Generate Test Cases**

```bash
cd conformance
bash conformance.sh wfm openapi "https://your-wfm-server/api/spec.yaml"
```

**What happens internally:**
1. Portman tool downloads the OpenAPI spec
2. Analyzes endpoints: GET /onboarding/certificate, POST /onboarding, etc.
3. Generates Postman collection with 8 test requests
4. Creates Newman environment file with variable templates ({{baseUrl}}, {{clientId}}, etc.)
5. Generates device certificates for authentication
6. Saves to: `Data-Generator/wfm-supplier/`

**Generated Test Cases (8 requests):**
```
1. GET /api/v1/onboarding/certificate
   → Retrieves WFM's public certificate
   
2. POST /api/v1/onboarding
   → Device sends onboarding request
   ← WFM returns {clientId}
   
3. POST /api/v1/clients/{clientId}/capabilities
   → Device reports its capabilities
   
4. PUT /api/v1/clients/{clientId}/capabilities
   → Device updates capabilities
   
5. GET /api/v1/clients/{clientId}/bundles/{digest}
   → Device retrieves application bundles
   
6. GET /api/v1/clients/{clientId}/deployments
   → Device retrieves list of deployments
   
7. GET /api/v1/clients/{clientId}/deployments/{id}/{digest}
   → Device retrieves specific deployment YAML
   
8. POST /api/v1/clients/{clientId}/deployments/{id}/status
   → Device reports deployment status
```

#### 2. **Runtime Patching (Smart Collection Adaptation)**

Before executing tests, the framework patches the collection to:

**a) Inject Server URL at Runtime**
```
Before: hardcoded https://localhost:3001
After: {{baseUrl}} → filled from environment variable
User provides: https://symphony.machine:8082/v1alpha2/margo
```

**b) Enable Context Chaining**
```
Test 2 (Onboarding): Returns clientId
Test 3+ (Capabilities, Deployments): Use returned {{clientId}} in path
```

**c) Handle Authentication Gracefully**
```
Real WFM requires RFC 9421 HTTP message signatures
Postman can't generate RSA signatures
Solution: Use flexible assertions that record execution but don't fail on auth errors
```

#### 3. **Execute Tests Against Real WFM**

```bash
bash run-tests.sh wfm
```

**Execution flow:**
1. Locates test data from `Data-Generator/wfm-supplier/`
2. Asks for component information (name, version, etc.)
3. Runs Newman (Postman's command-line executor)
4. Newman injects baseUrl and other variables
5. Executes each of 8 test requests
6. Records request/response details
7. Validates assertions
8. Generates HTML report with results

**Example Output:**
```
newman

Margo Workload Management API

→ Download Root CA certificate
  GET https://symphony.machine:8082/v1alpha2/margo/api/v1/onboarding/certificate
  [200 OK, 2.07kB, 59ms]
  ✓ Status code is 2xx
  ✓ Content-Type is application/json
  ✓ Response has JSON Body
  ✓ Schema is valid

→ Complete onboarding with client certificate
  POST https://symphony.machine:8082/v1alpha2/margo/api/v1/onboarding
  [409 Conflict, 378B, 5ms]  ← Expected (device already onboarded)
  ✓ Request executed

... (6 more test requests)

Assertions: 11/11 passed ✅
Execution Time: 307ms
Report: Runner/wfm-supplier/report_20260528_120000.html
```

#### 4. **Interpret Results**

- **✅ Green (Pass):** Assertion succeeded, API working correctly
- **⚠️ Yellow (Warning):** Expected errors (409 Conflict, 400 Bad Request) but request still executed
- **❌ Red (Fail):** Assertion failed, API not responding as expected

**What each test validates:**

| Test | Validates |
|------|-----------|
| GET /certificate | WFM exposes its public certificate |
| POST /onboarding | WFM accepts device onboarding, returns valid clientId |
| POST /capabilities | WFM accepts device capability reports |
| PUT /capabilities | WFM allows updating device capabilities |
| GET /bundles | WFM provides application bundles to device |
| GET /deployments | WFM provides deployment list to device |
| GET /deployments/{id} | WFM provides deployment details |
| POST /status | WFM accepts device status updates |

---

## Device Supplier Workflow

### Detailed Process

#### 1. **Generate Test Scenarios**

```bash
cd conformance
bash conformance.sh device "./device-supplier/device-scenarios/test-scenarios.json"
```

**What happens internally:**
1. Reads test scenarios (43 predefined test cases)
2. Validates scenario format and structure
3. Parses expected inputs/outputs
4. Generates assertion rules
5. Prepares deployment templates
6. Creates device certificates (for RFC 9421 signing)
7. Saves to: `Data-Generator/device-supplier/`

#### 2. **Test Scenarios Explained (43 Tests)**

Device tests are organized into **7 major scenario groups**:

**Scenario 1: RFC 9421 Signatures**
- Validates that device correctly signs requests using HTTP message signatures
- Tests both successful and failed signature attempts

**Scenario 2: Device Onboarding**
- Device sends onboarding request with certificate
- WFM returns clientId
- Device stores clientId for future requests

**Scenario 3: Capability Reporting**
- Device reports capabilities (CPU, memory, storage, etc.)
- WFM stores and retrieves capabilities
- Tests both initial report and capability updates

**Scenario 4: Deployment Management**
- WFM sends deployment manifest to device
- Device receives and validates manifest
- Device reports deployment status

**Scenario 5: Bundle Management**
- Device retrieves application bundles from WFM
- Tests digest-based retrieval
- Validates bundle format and content

**Scenario 6: Error Handling**
- Missing authentication
- Invalid signatures
- Malformed requests
- Wrong device IDs

**Scenario 7: State Management**
- Device maintains state across requests
- Context chaining (clientId from onboarding used in next requests)
- Proper error recovery

#### 3. **Context Chaining Pattern**

**How device tests use returned data:**

```
Test 1.2: Onboarding Device
REQUEST: POST /api/v1/onboarding
         Body: device certificate
RESPONSE: {clientId: "device-123"}
EXTRACT: clientId → store in test context

Test 2.1: Report Capabilities
REQUEST: POST /api/v1/clients/{clientId}/capabilities  ← Uses extracted clientId
         Body: CPU, memory, storage info
RESPONSE: 200 OK
STATUS: ✅ Used extracted clientId correctly

Test 2.2: Query Capabilities
REQUEST: GET /api/v1/clients/{clientId}/capabilities   ← Same clientId
RESPONSE: Returns previously reported capabilities
VALIDATE: Matches what was sent in 2.1
```

#### 4. **Run Tests Against Mock WFM**

```bash
bash run-tests.sh device
```

**Execution flow:**
1. Locates test data from `Data-Generator/device-supplier/`
2. Starts a mock WFM server (listening on port 3001)
3. Runs device test runner (Go binary)
4. For each scenario:
   - Sends request from device
   - Mock WFM validates signature (RFC 9421)
   - Records response
   - Validates assertions
5. Stops mock server
6. Generates HTML conformance report

**Example Output:**
```
🚀 Running Device Supplier conformance tests...

Scenario 1: RFC 9421 Signatures
  Step 1.1: Onboard Device (Setup)
    ✅ PASS - HTTP 201, clientId returned
  Step 1.2: Send Signed Request
    ✅ PASS - RFC 9421 signature validated
  Step 1.3: Send Invalid Signature
    ✅ PASS - Correctly rejected (401 Unauthorized)

Scenario 2: Device Onboarding
  Step 2.1: Initial Onboarding
    ✅ PASS - Device onboarded successfully
  Step 2.2: Duplicate Onboarding
    ✅ PASS - Correctly returned 409 Conflict

... (41 more test steps)

Results: 43/43 tests PASSED ✅
Report: Runner/device-supplier/conformance-report-20260528_120000.html
```

#### 5. **Key Device Capabilities Reported**

When devices report capabilities, they include:

```json
{
  "vendor": "ACME Corp",
  "modelNumber": "Edge-Device-X1",
  "serialNumber": "SN-12345",
  "roles": ["worker", "controller"],
  "resources": {
    "cpu": {
      "cores": 8,
      "architecture": "x86_64"
    },
    "memory": {
      "capacity": "16GB"
    },
    "storage": [
      {
        "type": "SSD",
        "capacity": "500GB"
      }
    ],
    "peripherals": [
      {
        "type": "GPU",
        "manufacturer": "NVIDIA",
        "model": "RTX 3080"
      }
    ],
    "interfaces": [
      {
        "type": "ethernet",
        "speed": "10Gbps"
      }
    ]
  }
}
```

---

## System Components & Communication

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                     APPLICATION LAYER (Margo)                       │
├─────────────────────────────────────────────────────────────────────┤
│                                                                       │
│  ┌──────────────────────┐              ┌──────────────────────┐      │
│  │   WFM Supplier       │              │  Device Supplier     │      │
│  │ (Workload Manager)   │◄─────HTTP────►│  (Edge Device)       │      │
│  │                      │   RFC 9421    │                      │      │
│  │  ✓ Deployment Mgmt   │   Signatures  │  ✓ Capability Report │      │
│  │  ✓ Bundle Management │              │  ✓ Status Updates    │      │
│  │  ✓ Device Registry   │              │  ✓ Deployment Accept │      │
│  │  ✓ State Management  │              │  ✓ Work Execution    │      │
│  └──────────────────────┘              └──────────────────────┘      │
│           ▲                                      ▲                    │
│           │                                      │                   │
└───────────┼──────────────────────────────────────┼───────────────────┘
            │                                      │
            │ CONFORMANCE TESTING                 │
            │                                      │
            ▼                                      ▼
    ┌────────────────────┐              ┌────────────────────┐
    │  Test Generator    │              │  Test Generator    │
    │ (conformance.sh)   │              │ (conformance.sh)   │
    │                    │              │                    │
    │ • OpenAPI parsing  │              │ • Scenario parsing │
    │ • Collection gen   │              │ • Data validation  │
    │ • Test data prep   │              │ • Assertion gen    │
    └────────┬───────────┘              └────────┬───────────┘
             │                                   │
             ▼                                   ▼
    ┌─────────────────────────────────────────────────────┐
    │           Test Execution Layer                      │
    ├─────────────────────────────────────────────────────┤
    │                                                       │
    │  ┌──────────────────┐       ┌──────────────────┐    │
    │  │   Newman        │       │  Go Test Runner  │    │
    │  │ (Postman CLI)   │       │  (Mock WFM)      │    │
    │  │                  │       │                  │    │
    │  │ • HTTP Requests │       │ • Mock Server    │    │
    │  │ • Variable Subst│       │ • Signature Val. │    │
    │  │ • Assertions    │       │ • State Mgmt     │    │
    │  │ • Reports       │       │ • Reports        │    │
    │  └──────────────────┘       └──────────────────┘    │
    └─────────────────────────────────────────────────────┘
             │                                   │
             ▼                                   ▼
    ┌──────────────────────┐       ┌──────────────────────┐
    │  Real WFM Server     │       │  Mock WFM Server     │
    │ (External System)    │       │  (Local Testing)     │
    │                      │       │                      │
    │  • Processes requests│       │  • Validates requests│
    │  • Returns responses │       │  • Simulates WFM     │
    │  • Manages state     │       │  • Records results   │
    └──────────────────────┘       └──────────────────────┘
```

### Communication Patterns

#### 1. **Request/Response Pattern (WFM ↔ Device)**

```
Device                              WFM
  │                                  │
  ├─ POST /onboarding ───────────────>│
  │  Certificate + Device Info        │
  │                                   │
  │<─ 201 Created ────────────────────┤
  │  {clientId: "abc123"}              │
  │                                   │
  ├─ POST /clients/{clientId}/        │
  │  capabilities ─────────────────────>│
  │  Signed with RFC 9421              │
  │                                   │
  │<─ 200 OK ─────────────────────────┤
  │  Capability accepted               │
  │                                   │
  └─ GET /clients/{clientId}/         │
     deployments ─────────────────────>│
     {clientId}                        │
                                      │
  <─ 200 OK ────────────────────────┘
     [Deployment manifest list]
```

#### 2. **Authentication Pattern (RFC 9421 HTTP Message Signatures)**

```
Device wants to authenticate with WFM:

1. Device prepares request
   - HTTP Method: POST
   - URL: /api/v1/clients/abc123/capabilities
   - Body: device capabilities JSON
   - Headers: standard HTTP headers

2. Device signs the request (RFC 9421)
   - Uses device private key (RSA)
   - Creates signature over: method + path + body + headers
   - Adds to request: Signature header

3. Device sends to WFM:
   - WFM receives signed request
   
4. WFM verifies signature:
   - Extracts public certificate from request
   - Verifies certificate against known devices
   - Validates signature matches request content
   - If valid: Accept request (200 OK)
   - If invalid: Reject request (401 Unauthorized)

5. WFM processes request:
   - Updates device capabilities in database
   - Returns response

Note: Postman (browser-based) cannot generate RSA signatures
      Tests use flexible assertions to validate flow without signatures
```

#### 3. **State Management Pattern (Context Chaining)**

```
Test Execution Flow with State:

Iteration 1:
  Request: POST /onboarding
  Response: {clientId: "client-123"}
  Store: context.clientId = "client-123"

Iteration 2:
  Template: POST /clients/{clientId}/capabilities
  Resolved: POST /clients/client-123/capabilities  (uses stored clientId)
  Request sent with stored value
  Response: {accepted: true}

Iteration 3:
  Template: GET /clients/{clientId}/deployments
  Resolved: GET /clients/client-123/deployments  (uses same clientId)
  Request sent
  Response: [deployment1, deployment2, ...]

Flow: Earlier responses → Later requests use extracted data
      This simulates real device-to-WFM interaction
```

---

## Key Concepts

### 1. **OpenAPI Specification**

**What is it?**
- Standard way to describe REST APIs
- Machine-readable format (YAML/JSON)
- Contains: endpoints, methods, request/response schemas

**Used by WFM Supplier:**
- WFM provider publishes OpenAPI spec
- Conformance tool downloads and parses spec
- Generates 8 test cases from spec
- Ensures WFM implementation matches documented API

**Example:**
```yaml
/api/v1/onboarding:
  post:
    summary: Device onboarding
    parameters:
      - name: certificate
        in: body
        required: true
        schema: {$ref: '#/definitions/Certificate'}
    responses:
      201:
        description: Device onboarded
        schema:
          type: object
          properties:
            clientId:
              type: string
```

### 2. **Test Scenarios**

**What is it?**
- JSON file describing test cases to run
- Includes: request templates, assertions, error cases
- Device Supplier provides these

**Structure:**
```json
{
  "name": "RFC 9421 Signatures",
  "steps": [
    {
      "description": "Onboard Device",
      "method": "POST",
      "endpoint": "/api/v1/onboarding",
      "expected_status": 201,
      "assertions": [
        {"key": "clientId", "operation": "not_empty"}
      ]
    }
  ]
}
```

### 3. **Context Chaining**

**What is it?**
- Ability to use data from one request in subsequent requests
- Simulates real device behavior

**Example:**
- Request 1 returns `clientId`
- Request 2 uses that `clientId` in URL path
- Tests validate that chained requests work correctly

### 4. **RFC 9421 HTTP Message Signatures**

**What is it?**
- Standard for cryptographically signing HTTP requests
- Device signs requests using private key
- WFM verifies using device public certificate
- Ensures authentication and request integrity

**Why needed:**
- Device proves it is who it claims to be
- Request cannot be modified in transit
- WFM can trust the device

**Limitation:**
- Postman (UI tool) cannot generate RSA signatures natively
- Tests use flexible assertions to validate flow
- Real device implementations must generate signatures

### 5. **Postman Collection**

**What is it?**
- File format for storing API tests (JSON)
- Contains: requests, assertions, variables
- Executed by Newman (Postman's CLI tool)

**Margo use:**
- Generated from OpenAPI spec by Portman tool
- Contains 8 WFM test requests
- Uses `{{variables}}` for runtime flexibility

### 6. **Mock WFM Server**

**What is it?**
- Simplified WFM implementation for testing
- Accepts device requests
- Validates signatures
- Returns simulated responses

**Why needed:**
- Device Supplier may not have access to real WFM
- Tests need to work in isolation
- Mock server provides controlled environment

---

## Test Data Flow

### Data Movement Through the System

```
PHASE 1: DATA GENERATION
──────────────────────────

Input (WFM Supplier):
  • OpenAPI spec at https://symphony.machine:8082/v1alpha2/margo/swagger.json
         ↓
conformance.sh:
  • Downloads spec
  • Portman generates collection
  • Creates test data
         ↓
Output: Data-Generator/wfm-supplier/
  ├── postman_collection.json      (8 test requests)
  ├── newman-data/
  │   ├── device-agent.env.json    (variables: {{baseUrl}}, {{clientId}})
  │   └── certs/                   (certificates)
  └── ...

Input (Device Supplier):
  • test-scenarios.json (43 tests)
         ↓
conformance.sh:
  • Validates test format
  • Generates assertions
         ↓
Output: Data-Generator/device-supplier/
  ├── test-scenarios.json
  ├── assertions.json
  └── certs/


PHASE 2: TEST EXECUTION
───────────────────────

Input from Data-Generator:
  • postman_collection.json
  • test environment
  • certificates
         ↓
run-tests.sh:
  • Reads from Data-Generator
  • Patches collection (inject baseUrl, clientId, etc.)
  • Starts Newman / Mock Server
         ↓
Newman/Mock Server:
  • Executes requests
  • Records responses
  • Validates assertions
         ↓
Output: Runner/
  ├── wfm-supplier/
  │   └── report_20260528_120000.html
  └── device-supplier/
      └── conformance-report-20260528_120000.html


PHASE 3: RESULT REVIEW
──────────────────────

HTML Reports show:
  • Number of tests: 8 (WFM) or 43 (Device)
  • Passed/Failed counts
  • Detailed request/response for each test
  • Assertion results
  • Performance metrics
  • Conformance status: ✅ PASSED or ❌ FAILED
```

---

## Environment Setup

### Multi-Machine Deployment

**Required setup for testing against real systems:**

#### Machine 1: WFM Server
```bash
# Install and start Margo WFM
cd $HOME/workspace/sandbox/scripts
sudo -E bash wfm.sh
# Choose: Option 3 (Symphony Start)
# Result: WFM running at https://symphony.machine:8082
```

#### Machine 2: Device Agent (Kubernetes)
```bash
# Install and start device agent with K3s
cd $HOME/workspace/sandbox/scripts
sudo -E bash device-agent.sh
# Choose: Option 2 (Kubernetes - K3s)
# Result: Device connects to WFM
```

#### Machine 3: Conformance Testing (Test Runner)
```bash
# Run conformance tests
cd /home/margo/nitin/sandbox/conformance
bash conformance.sh wfm openapi "https://symphony.machine:8082/v1alpha2/margo/swagger.json"
bash run-tests.sh wfm
```

### Network Configuration

**Required for all machines to communicate:**

1. **Static IP addresses** (use DHCP reservation or manual config)
2. **Hostname resolution** (add to `/etc/hosts`):
   ```
   192.168.1.100 symphony.machine    # WFM server
   192.168.1.101 device.machine      # Device agent
   192.168.1.102 tester.machine      # Test runner
   ```

3. **Firewall rules** (allow traffic between machines):
   ```bash
   sudo ufw allow 8082/tcp  # WFM API
   sudo ufw allow 3001/tcp  # Mock WFM
   sudo ufw allow 6443/tcp  # K3s API
   ```

### Environment Variables

**Configuration for your setup:**

```bash
# On WFM machine: scripts/wfm.env
export EXPOSED_SYMPHONY_HOST=symphony.machine
export EXPOSED_SYMPHONY_PORT=8082
export SYMPHONY_BRANCH=main

# On Device machine: scripts/device-agent.env
export WFM_HOST=symphony.machine
export WFM_PORT=8082

# On Test machine: docs/env-setup.md
export CONFORMANCE_TESTS_ENABLED=true
```

---

## Summary

### What You're Testing

| Persona | Tests | Method | Output |
|---------|-------|--------|--------|
| **WFM Supplier** | 8 HTTP endpoints | Postman/Newman | HTML report with request/response details |
| **Device Supplier** | 43 scenarios with context chaining | Go mock server | HTML conformance report |

### How It Works

1. **Generate**: `conformance.sh` creates test data from OpenAPI spec or test scenarios
2. **Execute**: `run-tests.sh` runs tests against real systems or mock servers
3. **Report**: HTML reports show pass/fail, assertions, and conformance status

### Key Technologies

- **Portman**: OpenAPI → Postman conversion
- **Newman**: Postman CLI executor
- **Go**: Mock WFM server and test runner
- **RFC 9421**: HTTP message signatures for authentication
- **jq**: JSON manipulation for runtime patching
- **Docker**: Container orchestration
- **Kubernetes (K3s)**: Device platform

### Next Steps

1. **Quick Start**: Follow [quick-start.md](./quick-start.md) to run your first tests
2. **Deep Dive**: Read persona-specific guides in [docs/](./docs/)
3. **Troubleshooting**: Check [setup-guide.md](./setup-guide.md) for environment issues

---

**Questions?** Check the quick-start guide or conformance test outputs for detailed error messages and solutions.
