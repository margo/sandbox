# WFM Supplier Conformance Flow

## What is this?

This is a simple conformance tester for WFM (Workload Fleet Management) servers.

**It simulates a device-agent making API calls to verify your WFM server works correctly.**

Think of it as a mock device that connects to your WFM and verifies all expected endpoints behave properly.

## Complete Workflow (Like device-agent.sh)

### Step 1: Start WFM Server
```bash
# WFM server generates ca-cert.pem at:
# /margo/home/symphony/api/certificates/ca-cert.pem
cd /path/to/wfm-server
./start.sh  # or your WFM startup command
```

### Step 2: Get WFM CA Certificate
Copy the CA certificate that WFM just generated:
```bash
cp /margo/home/symphony/api/certificates/ca-cert.pem \
   /home/margo/nitin/sandbox/conformance/wfm-supplier/certs/ca-cert.pem
```

This mirrors what `device-agent.sh` does when it copies the CA cert to verify the WFM server.

### Step 3: Run Conformance Tests
Now you can run setup + tests in one command:
```bash
cd /home/margo/nitin/sandbox/conformance/wfm-supplier
./run.sh all https://symphony.machine:8082/v1alpha2/margo
```

**What happens internally:**
- Setup script checks if `certs/ca-cert.pem` exists (exits if missing)
- Copies it to `newman-data/certs/ca-cert.pem` (runtime location)
- Generates fresh mock device ECDSA certificate
- Runs Newman with CA cert for TLS verification
- Mock device authenticates using RFC 9421 signatures

### Step 4: Review Results
Open the generated HTML report:
```bash
ls -lrt report_*.html
# Open in browser or view with less
```

## How to Use (Simple 3-Step Process)

### 1. Get the WFM CA Certificate (Manual Handoff)

**Important**: The CA certificate must come from YOUR WFM server. This is like device-agent getting the CA to verify the server.

**Where to get it:**
- When WFM server starts, it generates `ca-cert.pem` at: `/margo/home/symphony/api/certificates/ca-cert.pem`
- Or wherever your WFM instance stores its CA certificate

**What to do:**
1. Copy that CA certificate from your WFM server
2. Place it at: `conformance/wfm-supplier/certs/ca-cert.pem`
   
**Example:**
```bash
# From your WFM server machine:
cp /margo/home/symphony/api/certificates/ca-cert.pem \
   /home/margo/nitin/sandbox/conformance/wfm-supplier/certs/ca-cert.pem
```

**Important notes:**
- The certificate must exist BEFORE running setup or Newman
- Scripts check for it and exit with a clear error if missing
- This is a **manual verification step** — you're confirming you have the real WFM's certificate
- Each time you restart WFM with fresh certificates, copy the new CA cert here

**Why this design?**
- Mirrors real device-agent behavior (device gets WFM's CA to verify server)
- Prevents accidental wrong-server onboarding
- Ensures conformance testing against the actual server certificate

### 2. Run Setup
```bash
cd conformance/wfm-supplier
./1-setup_portman.sh https://your-wfm-server:8082/v1alpha2/margo
```
This creates:
- A Postman collection (the test script)
- A mock device certificate and key
- Environment data for the collection

### 3. Run the Tests
```bash
./2-run_newman.sh
```
This:
- Creates a fresh mock device each time
- Sends requests to your WFM (onboarding, capabilities, deployments, etc.)
- Verifies the responses are correct
- Generates an HTML report

**Or do both in one command:**
```bash
./run.sh all https://your-wfm-server:8082/v1alpha2/margo
```

## Certificate Lifecycle (Like device-agent.sh)

This section explains how certificates are handled to mirror real device-agent behavior:

### WFM Server Certificates (Provided by You)
| Component | Location | Lifecycle |
|-----------|----------|-----------|
| **CA Certificate** | `/margo/home/symphony/api/certificates/ca-cert.pem` (on WFM server) | Generated when WFM starts; same for all devices |
| **Server Certificate** | `/margo/home/symphony/api/certificates/server.pem` | Generated when WFM starts; signed by CA |
| **Validity** | -- | Valid until WFM restarts with new certs |

**What you do:**
- Copy CA cert to `conformance/wfm-supplier/certs/ca-cert.pem` before running tests
- This manually gives the conformance flow access to the WFM's CA

### Mock Device Certificates (Generated Automatically)
| Component | Location | Lifecycle |
|-----------|----------|-----------|
| **Device Key** | `newman-data/certs/device.key` | Generated fresh on each `2-run_newman.sh` run |
| **Device Cert** | `newman-data/certs/device-cert.pem` | Generated fresh on each `2-run_newman.sh` run |
| **Validity** | -- | Valid for 365 days (not checked in tests) |

**What scripts do:**
- `1-setup_portman.sh`: Generates initial mock device ECDSA certificate
- `2-run_newman.sh`: Generates **fresh** mock device certificate each run
- This ensures each run tests fresh onboarding (device-agent behavior)

### Full Certificate Flow

```
WFM Server (generates ca-cert.pem at /margo/home/symphony/api/certificates/)
    ↓
You copy: ca-cert.pem
    ↓
conformance/wfm-supplier/certs/ca-cert.pem (manual handoff)
    ↓
1-setup_portman.sh (checks it exists)
    ↓
2-run_newman.sh (before each run):
    - Verifies ca-cert.pem exists
    - Copies to newman-data/certs/ca-cert.pem (runtime location)
    - Generates fresh mock device ECDSA key + cert
    ↓
Newman execution:
    - Uses mock device cert for signing requests
    - Uses CA cert from WFM for TLS verification
    ↓
WFM Server:
    - Verifies mock device signature (with device cert)
    - Serves content (WFM cert signed by CA)
    ↓
Newman client:
    - Verifies WFM cert with copied CA cert
    - Validates responses
```

### Resetting for Fresh Onboarding

To stop WFM and reset for fresh onboarding:

```bash
# Stop WFM server
cd /path/to/wfm-server
./stop.sh  # or kill process

# WFM restarts fresh next time - generates new ca-cert.pem

# Before running conformance again:
# 1. Copy the new ca-cert.pem from WFM
cp /margo/home/symphony/api/certificates/ca-cert.pem \
   /home/margo/nitin/sandbox/conformance/wfm-supplier/certs/ca-cert.pem

# 2. Run conformance (will test fresh onboarding)
./run.sh all https://symphony.machine:8082/v1alpha2/margo
```

This ensures:
- Fresh WFM CA certificate is used for TLS verification
- Fresh mock device certificate is generated for each test run
- Each run tests complete fresh onboarding (matching device-agent flow)

## What Gets Tested?

When you run the tests, the mock device performs these steps:

1. **Get CA Certificate** — Downloads the root certificate
2. **Onboard Device** — Sends device certificate and gets a clientId
3. **Report Capabilities** — Sends what hardware/features the device has
4. **Update Capabilities** — Simulates capability change (upgrade)
5. **Get Deployments** — Retrieves workload assignments
6. **Get Deployment Details** — Retrieves specific workload YAML
7. **Report Status** — Reports deployment status back to WFM

All requests are properly signed using RFC 9421 signatures (like a real device).

## Files

| File | Purpose | Used? |
|------|---------|-------|
| `1-setup_portman.sh` | Downloads OpenAPI spec, generates collection, creates mock device | ✅ Yes |
| `2-run_newman.sh` | Runs the collection against your WFM, generates report | ✅ Yes |
| `run.sh` | Simple entrypoint (run both scripts or just one) | ✅ Yes (optional) |
| `certs/ca-cert.pem` | **You provide this** — WFM server certificate for trust verification | ✅ Yes |
| `postman_collection.json` | (Generated) The test collection | ✅ Yes |
| `newman-data/` | (Generated) Runtime data for the tests | ✅ Yes |
| `report_*.html` | (Generated) Test results report | ✅ Yes |
| `cmd/signreq/` | (Deprecated) Old HTTP request signing tool | ❌ No — Not used |

## What Do The Test Results Mean?

After running, you'll see output like:
```
→ Complete onboarding with client certificate
  POST https://your-wfm:8082/v1alpha2/margo/api/v1/onboarding [201 Created]
  ✓ Onboarding status check
```

- **201 Created** = Good! Response code correct
- **✓ Onboarding status check** = Assertion passed
- **11 assertions passed / 0 failed** = All tests passed

## Common Issues

**"❌ Missing WFM CA certificate"**
- Copy your CA certificate to `certs/ca-cert.pem` before running

**"Connection refused"**
- Check your WFM URL is correct
- Verify WFM is running and accessible

**"TLS certificate verification failed"**
- The CA cert doesn't match the WFM server
- Ensure you're using the correct CA cert

**Requests showing 400/401 errors but tests pass**
- This is normal. Some endpoints return 400/401 in test scenarios
- The "Scenario-aware status check" allows these as valid responses

## Requirements

These must be installed:
- `bash`, `curl`, `openssl`, `jq`
- `node` and `npm`

The scripts will auto-install:
- Portman (generates Postman collection from OpenAPI)
- Newman (runs Postman collection)
- Reporter (generates HTML report)

## Environment Variables

You can customize behavior with environment variables:

```bash
# Use a specific WFM server
WFM_BASE_URL=https://myserver:8082/v1alpha2/margo ./run.sh all

# Use a custom collection file
./2-run_newman.sh /path/to/custom-collection.json
```

## For Margo Developers

### Extending Tests (Customizing the Collection)

You can add your own test cases by modifying the Postman collection:

1. **Import the collection into Postman**
   - Open Postman
   - Click "Import" → Select `postman_collection.json`

2. **Add new test cases**
   - Create new requests or modify existing ones
   - Add test scripts, assertions, edge cases
   - Save changes

3. **Export and save**
   - Export as Collection 2.1 format
   - Replace the original `postman_collection.json`

4. **Run with Newman**
   - Your custom tests will execute: `./2-run_newman.sh`

**Important:** The script creates a runtime copy of your collection (`<collection>_runtime.json`) and applies patches only to that copy. Your original collection is **never modified**. This means:
- ✅ Your custom test scripts in existing endpoints will be preserved
- ✅ New endpoints you add will work as-is
- ✅ Environment variables like `{{onboardingRequest}}` will be injected by the runtime patches

**Example**: If you add a custom test to the onboarding endpoint, it will be preserved in your collection file. When Newman runs, the runtime patches will:
- Set the request body to use `{{onboardingRequest}}` 
- Add default test assertions
- Your custom tests will also run

Both your custom tests AND the default tests will execute on that endpoint.

### Example: Add a Custom Edge Case Test

1. In Postman, add a new request: `POST /api/v1/onboarding` (duplicate)
2. In the test tab, add: `pm.test("Custom: Onboarding timeout check", ...)`
3. Export and save to `postman_collection.json`
4. Run `./2-run_newman.sh` — both your test AND the default tests will run

### Understanding the Flow

1. **Setup phase** (`1-setup_portman.sh`):
   - Downloads WFM OpenAPI spec
   - Generates Postman collection from spec
   - Creates mock device identity (ECDSA certificate)
   - Prepares test payloads (onboarding request, capabilities, status report, etc.)

2. **Runtime phase** (`2-run_newman.sh`):
   - Generates fresh mock device (new certificate each run, so each run tests fresh onboarding)
   - Copies WFM CA cert to runtime location
   - Applies any runtime patches to collection
   - Executes collection using Newman
   - Verifies all responses using expected status codes and schema validation
   - Generates HTML report with full execution details

### Architecture

```
Device Agent (Newman)
    ↓ (makes API calls with RFC 9421 signatures)
    ↓
WFM Server (Under Test)
    ↓ (verifies signature, returns response)
    ↓
Collections Tests (verify response is correct)
    ↓
Test Report (HTML)
```

The mock device uses ECDSA keys (matching real device-agent) and signs requests before sending them.

## OpenAPI Source

The setup script fetches the spec from:
https://raw.githubusercontent.com/margo/specification/pre-draft/system-design/specification/margo-management-interface/workload-management-api-1.0.0.yaml

You can customize this by editing the `SPEC_URL` variable in `1-setup_portman.sh`.


## What Setup Generates
After running 1-setup_portman.sh:
- spec.yaml
- postman_collection.json
- newman-data/device-agent.env.json
- newman-data/device-agent.iteration.json
- newman-data/certs/ca-cert.pem
- newman-data/certs/device.key
- newman-data/certs/device-cert.pem

## Device-Agent Style Data Model
The scripts prepare request payloads to mimic current device-agent interactions:

- Onboarding request
  - apiVersion: onboarding.margo.org/v1alpha1
  - kind: OnboardingRequest
  - certificate: base64 encoded device certificate

- Capabilities request (POST)
  - apiVersion: device.margo.org/v1alpha1
  - kind: DeviceCapabilitiesManifest
  - properties: id, vendor, modelNumber, serialNumber, roles, resources

- Capabilities update request (PUT)
  - same structure with updated roles/resources

- Deployment status request (POST)
  - apiVersion: deployment.margo.org/v1alpha1
  - deploymentId
  - components
  - status

The collection is patched so matching endpoints use these variables.

## Collection Lifecycle
1. Run `./run.sh portman` to generate a fresh collection from spec.
2. Vendor customizes/imports collection and adds extra tests.
3. Run `./run.sh newman` to execute the current collection with the same runtime patching logic.

This keeps the execution logic stable even when the collection evolves over time.

## Collection Patching Details
During setup and execution, the collection is modified to make runtime execution practical:

- Request body injection
  - Onboarding POST uses `{{onboardingRequest}}`
  - Capabilities POST uses `{{capabilitiesRequest}}`
  - Capabilities PUT uses `{{capabilitiesUpdateRequest}}`
  - Deployment status POST uses `{{statusRequest}}`

- Path variable injection
  - `clientId` -> `{{clientId}}`
  - `deploymentId` -> `{{deploymentId}}`
  - `digest` -> `{{manifestEtag}}`

- Assertion policy rewrite
  - Default Portman success-path assertions are replaced with status-code allowlists tailored for this conformance scenario.
  - This enables negative/failure scenarios (for example `400`, `404`, `409`, `500` where applicable) to be counted as expected behavior.

## Runtime Behavior
At each run of `./run.sh newman` (or `./2-run_newman.sh`):
- A new device identity is generated (device-<timestamp>).
- A new ECDSA device certificate/key pair is generated.
- Environment and iteration data are refreshed.
- Newman runs the collection with:
  - environment file
  - iteration data file
  - trusted CA certificate from `certs/ca-cert.pem`
  - cli and htmlextra reporters

## Exit Code Semantics
`run.sh` and `2-run_newman.sh` exit with:
- `0` if all collection assertions and tests pass
- non-zero if any test fails

**What this means:**
- `0` = WFM responses matched all configured assertions
- non-zero = One or more test assertions failed

## Common Troubleshooting

1. Missing collection or data files
- Symptom: script says required files are missing
- Fix: run setup first
  ./run.sh portman

2. TLS certificate validation failures
- Symptom: HTTPS or x509 errors against WFM endpoint
- Fix options:
  - Copy the WFM CA certificate into `certs/ca-cert.pem`
  - Confirm the file is present before running `./run.sh portman` or `./run.sh newman`
  - Confirm BASE_URL points to reachable WFM endpoint

3. Portman or Newman command not found
- Symptom: tool not found
- Fix: rerun setup or install manually with npm global install

4. WFM rejects requests due to auth/signature
- Symptom: 401 or 403 from WFM
- Notes:
  - Mock device uses ECDSA keypair for RFC 9421 request signatures
  - WFM must trust the device certificate provided in onboarding
  - For custom/edge-case tests, verify WFM CA cert is correct

5. Endpoint mismatch due to BASE_URL
- Symptom: 404 for many endpoints
- Fix:
  - Ensure BASE_URL includes the expected prefix used by your WFM
  - Example format: https://host:port/v1alpha2/margo

6. You need strict success-only checks
- Symptom: you want failures such as `400`/`404` to fail the run
- Fix:
  - Customize assertions in `postman_collection.json` after setup, or adjust `1-setup_portman.sh` allowlists
  - Keep your vendor-specific strict tests in dedicated collection folders and run them separately or with folder filters

## Re-run Guidance
- Use `./run.sh portman` when:
  - OpenAPI source changes
  - WFM BASE_URL changes
  - You want a fresh generated collection
- Use `./run.sh newman` for repeated runs of current collection (including vendor customizations)
- Use `./run.sh all` for full regenerate-and-run cycle

## Notes
- This flow is a practical conformance harness for WFM Supplier persona testing using generated client-side tests.
- It is not a full production emulator of the complete device runtime lifecycle.

## FAQ: Customizing Tests

**Q: Can I add custom tests to the collection and have them preserved?**

A: Yes! Margo developers can:
1. Import `postman_collection.json` into Postman
2. Add custom test cases, requests, assertions
3. Export and save back to `postman_collection.json`
4. Run `./2-run_newman.sh` — your changes are preserved

The script uses a separate runtime copy for patching, so your original collection is never overwritten.

**Q: What if I want to modify an existing endpoint's test?**

A: You can add additional assertions to any endpoint. When the script runs:
- Your custom tests execute as-is
- Runtime patches apply default tests (they stack, not replace)
- Both sets of tests run on that endpoint

**Q: Can I use a completely custom collection?**

A: Yes, pass it as an argument:
```bash
./2-run_newman.sh /path/to/my-custom-collection.json
```

The script will still patch it to set request bodies and apply default assertions.

**Q: What gets patched at runtime?**

A: Only these behaviors are patched:
- Request body variables (e.g., `{{onboardingRequest}}`)
- Default test assertions for endpoints

Your custom tests, request headers, authentication, etc., are NOT modified.

## Deprecated / Not Used

**`cmd/signreq/` directory**
- **Status**: ❌ Deprecated — Not used in current implementation
- **What it was**: Old HTTP request signing tool using RFC 9421 (part of previous "precheck" phase)
- **Why removed**: Simplified flow uses Newman directly; request signing is handled by Postman environment/scripts
- **Action**: Can be safely deleted if desired
  ```bash
  rm -rf conformance/wfm-supplier/cmd/
  ```
- **Note**: The signing code is still available in `shared-lib/crypto/` if needed for other projects

**`run.sh` commands vs direct script calls**
- **Status**: ✅ Still works, optional convenience
- **Recommendation**: Use `run.sh` for:
  - Non-technical team members (simpler interface)
  - CI/CD automation (standardized entry point)
- **Can skip if**: You prefer calling scripts directly
  ```bash
  # Instead of: ./run.sh all <URL>
  # You can do: 
  ./1-setup_portman.sh <URL>
  ./2-run_newman.sh
  ```
- For hand-tailored vendor cases, keep placeholders as the contract and add custom requests/tests in the collection; Newman can execute both generated and custom folders together.