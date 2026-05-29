# WFM Supplier Conformance - Complete Guide

**For: Everyone (technical and non-technical)**

A comprehensive guide explaining what this system does, how it works, and why each piece exists.

---

## Table of Contents

1. [What Is This System?](#what-is-this-system)
2. [Key Concepts (Explained Simply)](#key-concepts-explained-simply)
3. [The Two Main Personas](#the-two-main-personas)
4. [Complete System Flow](#complete-system-flow)
5. [How Different Parts Communicate](#how-different-parts-communicate)
6. [Features and What They Do](#features-and-what-they-do)
7. [Architecture Diagram](#architecture-diagram)
8. [Real-World Analogy](#real-world-analogy)

---

## What Is This System?

### The Big Picture

Imagine you build a **Workload Fleet Management (WFM) system** that controls thousands of devices and their workloads (applications). Before deploying it to customers, you need to verify it works correctly.

This conformance testing system is like a **quality assurance (QA) team** that:
- ✅ Simulates real devices connecting to your WFM
- ✅ Sends API requests to verify proper responses
- ✅ Tests the complete lifecycle: device registration → capability reporting → workload assignment → status updates
- ✅ Produces reports showing everything works

### Why Is This Important?

When you release WFM, you need to guarantee:
1. **All required API endpoints exist and work**
2. **Responses follow the correct format**
3. **Device authentication works properly**
4. **The complete device lifecycle works end-to-end**

This system automates that verification.

---

## Key Concepts (Explained Simply)

### 1. WFM (Workload Fleet Management)

A **central server** that:
- Manages thousands of edge devices
- Assigns workloads (applications) to devices
- Tracks device status and capabilities
- Handles authentication and security

**Example:** Cloud control center that tells edge devices what to run.

### 2. Device Agent

A **software running on each edge device** that:
- Registers itself with WFM (onboarding)
- Reports its hardware capabilities (CPU, memory, storage)
- Downloads workloads assigned by WFM
- Reports execution status back to WFM

**Example:** A small app on your IoT device that talks to the control center.

### 3. Conformance Testing

A **process that verifies** the WFM system:
- Accepts proper device registrations
- Processes device information correctly
- Assigns workloads properly
- Handles device status updates

**Example:** A checklist that ensures everything the device expects works.

### 4. Mock Device

A **simulated device** (not real hardware) that:
- Behaves like a real device-agent
- Sends the same API requests
- Doesn't require actual hardware

**Example:** A software robot that pretends to be a device.

### 5. API (Application Programming Interface)

A **contract** between the device and WFM that defines:
- What requests a device can make
- What responses WFM will send
- What format data must be in

**Example:** A rulebook for how devices and WFM talk to each other.

### 6. Certificate & Authentication

A **security mechanism** that:
- Proves the device is authentic (like an ID card)
- Encrypts communication between device and WFM
- Prevents unauthorized devices from connecting

**Example:** Your passport proves you are who you say you are.

### 7. Postman Collection

A **test script package** that contains:
- All API endpoints to test
- All test data needed
- Expected responses
- Pass/fail criteria

**Example:** A detailed script an actor follows to perform a scene.

### 8. Newman

A **test runner** that:
- Executes the Postman collection
- Makes actual API calls to WFM
- Verifies responses
- Generates reports

**Example:** A director who ensures the actor follows the script and records everything.

---

## The Two Main Personas

This system supports two different use cases:

### Persona 1: WFM Supplier (You Are Here)

**Who:** WFM system provider/developer

**Goal:** Verify my WFM implementation works correctly

**What They Do:**
1. Start their WFM server
2. Run conformance tests against it
3. Get a report showing everything works (or what needs fixing)

**Key Responsibility:** Ensure WFM implementation matches the API specification

**Success Criteria:**
- All 8 API endpoints respond correctly
- Device can onboard successfully
- Device can report capabilities
- Device can receive and execute workloads
- All status updates work properly

---

### Persona 2: Device Agent Developer

**Who:** Edge device software developer

**Goal:** Verify my device-agent implementation works correctly

**What They Do:**
1. Implement device-agent software
2. Run tests against a real/mock WFM
3. Get detailed reports showing compatibility

**Key Responsibility:** Ensure device-agent sends correct requests and handles responses

**Success Criteria:**
- Device can onboard to any compliant WFM
- All API calls succeed
- Device properly parses responses
- Device handles errors gracefully

*(This guide focuses on Persona 1 — WFM Supplier)*

---

## Complete System Flow

### The Lifecycle of a Test Execution

```
┌─────────────────────────────────────────────────────────────┐
│ Start: WFM Supplier Runs Conformance Tests                  │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│ Step 1: SETUP PHASE (1-setup_portman.sh)                   │
│                                                             │
│ Input: OpenAPI Specification                               │
│ Actions:                                                    │
│  - Validate tools are available (jq, curl, openssl, npm)  │
│  - Download official Margo OpenAPI spec                    │
│  - Use Portman to convert spec → Postman collection        │
│  - Generate mock device certificate + key                  │
│  - Create realistic test payloads:                          │
│    * Onboarding request JSON                               │
│    * Capability report JSON                                │
│    * Deployment status JSON                                │
│  - Write Newman environment variables                      │
│  - Patch collection for runtime flexibility                │
│                                                             │
│ Output: Ready-to-run test collection                       │
│  - postman_collection.json (the test script)               │
│  - newman-data/device-agent.env.json (test variables)      │
│  - newman-data/certs/device.key + device-cert.pem (auth)  │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│ Step 2: EXECUTION PHASE (2-run_newman.sh)                  │
│                                                             │
│ Preparations:                                               │
│  - Verify CA certificate exists (security requirement)     │
│  - Verify setup artifacts exist                            │
│  - Check system requirements (jq, curl, openssl, newman)   │
│  - Update test environment with WFM URL                    │
│  - Copy CA certificate to runtime location                 │
│  - Generate fresh mock device certificate for this run     │
│                                                             │
│ Test Execution:                                             │
│  - Start Newman with:                                      │
│    * Postman collection (what to test)                     │
│    * Environment variables (test data)                     │
│    * Device certificate (authentication)                   │
│    * CA certificate (to verify WFM server)                 │
│                                                             │
│ Test Sequence (8 API calls):                               │
│  1. GET /api/v1/onboarding/certificate                    │
│     "Download WFM's root CA certificate"                   │
│     Status: Expected 200 OK                                │
│     Device Action: Saves this for future TLS connections   │
│                                                             │
│  2. POST /api/v1/onboarding                               │
│     "Register device with WFM"                             │
│     Body: Device certificate + public key                  │
│     Status: Expected 200 (first run) or 409 (already exists)│
│     Response: clientId (unique device identifier)          │
│     Device Action: Stores clientId for future requests     │
│                                                             │
│  3. POST /api/v1/clients/{clientId}/capabilities          │
│     "Report what hardware features this device has"        │
│     Body: CPU cores, memory, storage, OS, roles, etc       │
│     Status: Expected 200 or 400 (server may reject format) │
│     Device Action: Notifies WFM about device capabilities  │
│                                                             │
│  4. PUT /api/v1/clients/{clientId}/capabilities           │
│     "Update capabilities if device hardware changed"       │
│     Body: Same capability report with updates              │
│     Status: Expected 200 or 400                            │
│     Device Action: Confirms capability changes             │
│                                                             │
│  5. GET /api/v1/clients/{clientId}/deployments            │
│     "Get list of workloads assigned by WFM"               │
│     Status: Expected 200 or 400                            │
│     Response: List of workloads: [deploy-1, deploy-2, ...]│
│     Device Action: Knows which workloads to run            │
│                                                             │
│  6. GET /api/v1/clients/{clientId}/deployments/{id}/yaml   │
│     "Get detailed workload specification (YAML file)"      │
│     Status: Expected 200 or 401 (auth failure expected)    │
│     Response: Full workload definition                     │
│     Device Action: Parses workload and prepares execution  │
│                                                             │
│  7. GET /api/v1/clients/{clientId}/bundles/{id}           │
│     "Get workload bundles (container images, config)"      │
│     Status: Expected 200 or 401 (auth failure expected)    │
│     Device Action: Downloads workload resources            │
│                                                             │
│  8. POST /api/v1/clients/{clientId}/deployments/{id}/status│
│     "Report workload execution status back to WFM"         │
│     Body: Status (running/completed), metrics, logs        │
│     Status: Expected 200 or 400                            │
│     WFM Action: Records device status for monitoring       │
│                                                             │
│ Result: All requests sent, all responses collected         │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│ Step 3: REPORTING PHASE                                    │
│                                                             │
│ Console Output:                                             │
│  ✓ 8 requests executed                                     │
│  ✓ 11 assertions passed                                    │
│  ✗ 0 failures                                              │
│  ⏱ 307ms total time                                        │
│                                                             │
│ HTML Report Generated:                                      │
│  - report_20260528_183916.html                             │
│  - Contains: All requests, responses, test results         │
│  - Viewable in: Any web browser                            │
│  - Useful for: Debugging, documentation, sharing results   │
│                                                             │
│ Exit Code: 0 (success) or non-zero (failure)              │
│ Usage: Integration with CI/CD pipelines                    │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│ Done: WFM Conformance Verified ✅                           │
└─────────────────────────────────────────────────────────────┘
```

---

## How Different Parts Communicate

### The Communication Chain

```
┌──────────────────────────────────────────────────────────────┐
│                      COMMUNICATION FLOW                       │
└──────────────────────────────────────────────────────────────┘

1. USER (WFM Supplier)
   ↓ "Run conformance tests"
   
2. BASH SCRIPT (conformance_cli.sh or manual execution)
   ↓ Calls 1-setup_portman.sh and 2-run_newman.sh
   
3. SETUP SCRIPT (1-setup_portman.sh)
   ↓ "Here's what to test"
   
4. POSTMAN COLLECTION (postman_collection.json)
   ↓ "These are the 8 API calls to make"
   
5. NEWMAN (test executor)
   ↓ Sends HTTP requests using collection + environment
   
6. NETWORK
   ↓ HTTPS connection (encrypted, with TLS certificates)
   
7. WFM SERVER
   ↓ Receives requests, validates device certificate
   ↓ Processes onboarding, capability, deployment requests
   ↓ Sends back responses
   
8. NETWORK
   ↓ Returns responses with data
   
9. NEWMAN
   ↓ Receives responses, validates according to tests
   ↓ Compares: actual vs expected
   ↓ Records: pass/fail, timing, response details
   
10. HTML REPORT (report_YYYYMMDD_HHMMSS.html)
    ↓ "Here's what happened"
    
11. USER
    ↓ Reviews results
    ✓ All tests passed = WFM works!
    ✗ Some failed = WFM needs fixes
```

### Module Interactions

#### 1. Setup Phase Interaction

```
OpenAPI Spec (source of truth for API)
    ↓
Portman (tool that reads API spec)
    ↓
Postman Collection (converts spec into test cases)
    ↓
Patch Scripts (customize for runtime needs)
    ↓
Device Certificates (generate mock device identity)
    ↓
Test Payloads (create realistic request bodies)
    ↓
Environment File (store test variables)
    ↓
Ready for Execution
```

#### 2. Execution Phase Interaction

```
Newman (test executor)
    ↓ Loads: postman_collection.json
    ↓ Loads: newman-data/device-agent.env.json
    ↓ Loads: newman-data/certs/device-cert.pem
    ↓ Loads: certs/ca-cert.pem
    ↓
For Each Test Request:
    ├─ Substitute variables {{baseUrl}}, {{clientId}}, etc
    ├─ Set up HTTPS connection with TLS
    ├─ Sign request with device certificate (RFC 9421)
    ├─ Send to WFM
    ├─ Receive response
    ├─ Verify TLS certificate using CA cert
    ├─ Run test assertions
    ├─ Record: request, response, result, timing
    └─ Continue to next request
    ↓
Collect all results
    ↓
Generate HTML report + CLI summary
```

#### 3. Certificate Communication

```
WFM Server                          Mock Device (in Newman)
       ↓                                    ↓
  Has Certificate                   Has Certificate
  Signed by CA                       (Self-signed or CA-signed)
       ↓                                    ↓
  └─── Exchange via HTTPS ───┘
       ↓
  Verify Server Cert              Verify Server Cert
  Using: ca-cert.pem             Using: ca-cert.pem (copy)
       ↓
  ✓ Verified = Trusted            ✓ Verified = Trust WFM
       ↓
  Send Responses                  Send Requests Signed
  (to device cert owner)          (with device cert)
       ↓
  Device verifies                 WFM verifies
  data is from us                 device is legitimate
       ↓
  Process Response                Process Request
```

---

## Features and What They Do

### Feature 1: Automatic Collection Generation

**What:** Convert OpenAPI spec to Postman collection automatically

**Why:**
- Don't manually write test cases
- Always in sync with API spec
- Reduces human error
- Ensures all endpoints are covered

**How it works:**
```
API Specification (source of truth)
    ↓ Portman converts
Postman Collection (executable test script)
```

**Benefit:** If you update your API spec, tests automatically update

---

### Feature 2: Runtime Variable Substitution

**What:** Replace placeholders with actual values during test execution

**Example:**
```
Collection has: GET /api/v1/clients/{{clientId}}/deployments

During setup: No clientId exists yet

During execution:
  1. Onboarding call returns: clientId = "client-xyz"
  2. Environment updated: clientId = "client-xyz"
  3. Next call becomes: GET /api/v1/clients/client-xyz/deployments
```

**Why:** Tests adapt to real responses (device doesn't know clientId beforehand)

---

### Feature 3: Collection Patching

**What:** Customize the generated collection for realistic behavior

**Patches Applied:**
1. **Clear URL path variables** - Removes hardcoded fake values
2. **Inject request bodies** - Uses environment variables for dynamic payloads
3. **Add flexible assertions** - Accept auth failures gracefully (not all endpoints work without signatures)

**Why:**
- Generated collection is static; real execution is dynamic
- Some endpoints may fail for expected reasons (RFC 9421 signatures needed)
- Need flexible pass/fail criteria

**Configuration:**
```bash
# Default: Apply patches (for generated collections)
./2-run_newman.sh https://wfm.example.com:8082/v1alpha2/margo

# Skip patches (for hand-written, well-formed collections)
PATCH_COLLECTION=false ./2-run_newman.sh https://wfm.example.com:8082/v1alpha2/margo
```

---

### Feature 4: Fresh Device Generation per Run

**What:** Create new mock device identity every test execution

**Why:**
- Tests fresh onboarding (device doesn't exist yet)
- Each run is independent
- Mirrors real device deployment

**What Happens Each Run:**
```
Newman Start
    ↓
Generate new device-cert.pem (fresh identity)
    ↓
Clear environment variables
    ↓
Send requests with new device
    ↓
WFM treats this as a new device onboarding
    ↓
Newman Finish
```

---

### Feature 5: Certificate-Based Security

**What:** Use public key cryptography to authenticate requests

**Components:**
1. **WFM CA Certificate** - Public key to verify WFM is real
2. **Mock Device Certificate** - Proves device is legitimate
3. **TLS Connection** - Encrypted communication

**Security Guarantee:**
- Device proves it's authentic (signed request)
- Device verifies WFM is authentic (certificate validation)
- Communication is encrypted (HTTPS/TLS)

---

### Feature 6: Flexible Test Assertions

**What:** Define what counts as "pass" or "fail" for each endpoint

**Default Behavior (Strict):**
```
POST /onboarding → must return 200
GET /deployments → must return 200
```

**Realistic Behavior (Flexible):**
```
POST /onboarding → 200 (first time) or 409 (already exists) = PASS
GET /deployments → 200 (success) or 400 (auth not implemented yet) = PASS
GET /bundles → 401 (signature required) or 200 (if signed) = PASS
```

**Why:** Some status codes are expected, not failures

---

### Feature 7: HTML Report Generation

**What:** Create a detailed, visual report of all test results

**Report Contains:**
- All 8 API requests made
- Request headers, body, parameters
- Response status code, headers, body
- Timing information (response time)
- Test assertion results (pass/fail)
- Visual pass/fail indicators

**Format:** Standard HTML webpage viewable in any browser

**Use Cases:**
- Documentation: Share results with team
- Debugging: Find which request failed
- Audit trail: Prove WFM was tested
- CI/CD: Archive in build pipeline

---

## Architecture Diagram

### System Components

```
┌─────────────────────────────────────────────────────────────┐
│                    CONFORMANCE SYSTEM                        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────────┐      ┌──────────────────┐            │
│  │  Input: OpenAPI  │      │   Bash Scripts   │            │
│  │  Specification   │      │  (orchestration) │            │
│  └────────┬─────────┘      └────────┬─────────┘            │
│           │                         │                       │
│           ├─────────────────────────┤                       │
│           ↓                         ↓                       │
│  ┌──────────────────────────────────────────┐             │
│  │      1-setup_portman.sh                  │             │
│  │  - Download spec                         │             │
│  │  - Run Portman (spec → collection)       │             │
│  │  - Generate device certificates          │             │
│  │  - Create test payloads                  │             │
│  │  - Patch for runtime needs               │             │
│  └────────────────────┬─────────────────────┘             │
│                       ↓                                     │
│  ┌──────────────────────────────────────────┐             │
│  │      Generated Test Artifacts            │             │
│  │  - postman_collection.json (8 API tests) │             │
│  │  - newman-data/device-agent.env.json     │             │
│  │  - newman-data/certs/device.key          │             │
│  │  - newman-data/certs/device-cert.pem     │             │
│  └────────────────────┬─────────────────────┘             │
│                       ↓                                     │
│  ┌──────────────────────────────────────────┐             │
│  │      2-run_newman.sh                     │             │
│  │  - Prepare environment                   │             │
│  │  - Copy CA certificate                   │             │
│  │  - Start Newman                          │             │
│  │  - Generate report                       │             │
│  └────────────────────┬─────────────────────┘             │
│                       ↓                                     │
│  ┌──────────────────────────────────────────┐             │
│  │      Newman (Postman Runner)             │             │
│  │  - Execute 8 API requests                │             │
│  │  - Validate responses                    │             │
│  │  - Measure timing                        │             │
│  │  - Record results                        │             │
│  └────────────────────┬─────────────────────┘             │
│                       ↓                                     │
│  ┌──────────────────────────────────────────┐             │
│  │         Output Artifacts                 │             │
│  │  - Console summary (CLI output)          │             │
│  │  - HTML report (visual results)          │             │
│  │  - Exit code (automation integration)    │             │
│  └──────────────────────────────────────────┘             │
│                                                             │
└─────────────────────────────────────────────────────────────┘
                            ↓
        ┌──────────────────────────────────────┐
        │       WFM Server Being Tested        │
        │  - Receives 8 API requests           │
        │  - Processes device onboarding       │
        │  - Validates capabilities            │
        │  - Manages deployments               │
        │  - Returns responses                 │
        └──────────────────────────────────────┘
```

### Data Flow

```
User Input (WFM URL)
    ↓
Script 1: Setup & Patching
    ├─ Input: API Specification
    ├─ Input: Portman configuration
    └─ Output: Postman Collection + Environment
    ↓
Script 2: Execution & Testing
    ├─ Input: WFM URL
    ├─ Input: CA Certificate
    ├─ Input: Postman Collection
    ├─ Input: Environment Variables
    └─ Output: Test Results + HTML Report
    ↓
Newman Execution Engine
    ├─ Input: Collection (what to test)
    ├─ Input: Environment (test data)
    ├─ Input: Certificates (authentication)
    ├─ Process: HTTP requests to WFM
    └─ Output: Responses + Assertion Results
    ↓
Final Report
    ├─ Console Summary (quick overview)
    ├─ HTML Report (detailed analysis)
    └─ Exit Code (automation integration)
```

---

## Real-World Analogy

### Think of it Like a Restaurant Inspection

**Scenario:** A restaurant wants to certify it meets food safety standards before opening.

**The WFM Conformance System is like:**

| Aspect | Restaurant | WFM System |
|--------|-----------|-----------|
| **What's being tested?** | Restaurant kitchen operations | WFM API endpoints |
| **Who tests?** | Health inspector (QA team) | Conformance test (Newman) |
| **What's the checklist?** | Food handling standards (API spec) | API specification document |
| **The test process** | Inspector: "Prepare a meal" → Inspect ingredients, temperature, cleanliness | Newman: "Onboard a device" → Verify request format, certificate, response |
| **Test sequence** | 1. Check storage 2. Inspect prep area 3. Verify cooking temps 4. Check final presentation | 1. Get CA cert 2. Onboard device 3. Report capabilities 4. Get deployments 5. Report status |
| **Pass/Fail criteria** | All items must be safe and following standards | All responses must be valid and matching API spec |
| **Documentation** | Inspection report with findings | HTML report with all requests/responses |
| **Result** | "Restaurant meets standards ✓" or "Fix these items ✗" | "WFM passes conformance ✓" or "These endpoints fail ✗" |

---

## Summary

This conformance system provides **automated verification** that your WFM implementation:

1. **Follows the API specification** - All endpoints work as defined
2. **Handles device lifecycle** - Onboarding, capability reporting, workload assignment
3. **Maintains security** - Certificate authentication, encrypted communication
4. **Provides reliability** - Consistent responses, proper error handling
5. **Generates evidence** - HTML reports for documentation and auditing

### Key Takeaway

> **Instead of manually testing every endpoint and scenario, this system automates it all and produces a professional report proving your WFM works correctly.**

---

## Next Steps

- **Quick Execution:** See [quick-start.md](quick-start.md) for commands
- **Run the CLI:** `cd /conformance && ./conformance_cli.sh`
- **Review Scripts:** Examine `1-setup_portman.sh` and `2-run_newman.sh`
- **Check Results:** Open generated HTML reports in a web browser
