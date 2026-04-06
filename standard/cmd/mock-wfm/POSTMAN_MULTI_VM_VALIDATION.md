# Postman Collection: Multi-VM Deployment Validation Guide

**Status:** Quick reference for using Postman to validate cross-VM deployments  
**Created:** April 6, 2026

## Overview

This guide shows how to use the **mock-wfm-comprehensive.postman_collection.json** to verify that your mock-WFM and device-agent setup work correctly across different VMs.

## Key Concepts

When testing device-agents on different VMs, you need to validate:
1. ✅ Mock-WFM returns deployments with correct `packageLocation`
2. ✅ Device-agent can resolve and download the `packageLocation` resource
3. ✅ The downloaded compose file is valid
4. ✅ Device-agent can process and execute the deployment

This guide focuses on **#1 and #2** — validating the deployment configuration before device-agent integration.

---

## Quick Workflow

### Step 1: Start Mock-WFM Server

```bash
cd /home/margo/nitin/sandbox/standard/cmd/mock-wfm

# Start the service
./start-mock-wfm.sh

# Verify it's running (should see this output)
# ✅ Server started successfully
# HTTP:  http://localhost:8090/v1alpha2/margo
# HTTPS: https://localhost:8443/v1alpha2/margo
```

### Step 2: Import Postman Collection

1. Open **Postman**
2. Click **Import** → **Upload Files**
3. Select: `/home/margo/nitin/sandbox/mock-wfm-comprehensive.postman_collection.json`
4. Create or select environment
5. Click **Import**

---

### Step 3: Configure Environment Variables

In Postman, set these variables for your environment:

| Variable | Example Value | Purpose |
|----------|---------------|---------|
| `wfm_base_url` | `http://localhost:8090/v1alpha2/margo` | Mock-WFM API endpoint |
| `device_id` | `device-001` | Device identifier for deployment retrieval |
| `client_id` | `client-001` | Onboarded client ID |

**Set in Postman:**
1. Tab: **Environments**
2. Create new or edit existing
3. Add variables above
4. Click **Save**

---

## Validation Scenarios

### Scenario 1: Verify Deployments Have Correct packageLocation

**Goal:** Confirm mock-WFM returns deployments with appropriate `packageLocation` values

**Steps:**

1. **Onboard a device** (if not already done):
   - Run: `POST /api/v1/onboarding` (see Postman collection)
   - Capture `clientId` from response

2. **Fetch deployments**:
   ```
   GET /api/v1/clients/{clientId}/deployments
   ```
   
   **Postman:**
   - Find request: **"GET Deployments - Headers Validation"**
   - Set `{{client_id}}` in path
   - Click **Send**

3. **Verify response**:
   ```json
   {
     "deployments": [
       {
         "metadata": {
           "name": "compose-sample"
         },
         "spec": {
           "deploymentProfile": {
             "components": [
               {
                 "properties": {
                   "packageLocation": "./docker-compose-sample.yaml"
                 }
               }
             ]
           }
         }
       }
     ]
   }
   ```

   ✅ **Expected:** `packageLocation` should be one of:
   - Relative path: `./docker-compose-sample.yaml`
   - HTTP URL: `http://mock-wfm-ip:8080/compose/docker-compose-sample.yaml`
   - GitHub URL: `https://raw.githubusercontent.com/...`

---

### Scenario 2: Test Different packageLocation Strategies

**Goal:** Validate that device-agents can fetch compose files from different locations

#### Option A: Relative Path

**Setup:**
- Files are mounted on same path on both VMs

**Postman validation:**
```
GET /api/v1/clients/{clientId}/deployments

Response should show:
packageLocation: ./docker-compose-sample.yaml
```

**Then on device-agent VM:**
```bash
# Verify the file is accessible
ls -l ./docker-compose-sample.yaml

# Validate compose format
docker-compose -f ./docker-compose-sample.yaml config
```

---

#### Option B: HTTP Server URL

**Setup in mock-WFM:**
```yaml
# Update packageLocation to:
packageLocation: http://192.168.1.100:8080/docker-compose-sample.yaml
```

**Postman validation:**
```
GET /api/v1/clients/{clientId}/deployments

Response should show:
packageLocation: http://192.168.1.100:8080/docker-compose-sample.yaml
```

**Then verify:**
```bash
# From device-agent VM
curl -O http://192.168.1.100:8080/docker-compose-sample.yaml
docker-compose -f docker-compose-sample.yaml config
```

---

#### Option C: GitHub URL

**Setup in mock-WFM:**
```yaml
packageLocation: https://raw.githubusercontent.com/margo/conformance/main/docker-compose-sample.yaml
```

**Postman validation:**
```
GET /api/v1/clients/{clientId}/deployments

Response should show:
packageLocation: https://raw.githubusercontent.com/margo/conformance/main/docker-compose-sample.yaml
```

**Then verify:**
```bash
# From device-agent VM (or any VM with internet)
curl -O https://raw.githubusercontent.com/margo/conformance/main/docker-compose-sample.yaml
docker-compose -f docker-compose-sample.yaml config
```

---

### Scenario 3: Full Conformance Test Suite

**Goal:** Run all 39 tests to ensure mock-WFM is conformant with the OpenAPI spec

**Steps:**

1. **In Postman**, open the **Runner**:
   - Click **Runner** in top left
   - Select collection: **mock-wfm-comprehensive**
   - Select environment: Your configured environment
   - Click **Run**

2. **Expected output:**
   ```
   39/39 tests passed ✅
   ```

3. **Check "GET Deployments" test results**:
   - Verify all deployment tests pass
   - Confirm `packageLocation` is present in responses
   - Check HTTP status codes (200, 304, etc.)

---

## Advanced: Extract and Validate packageLocation

**Scenario:** You want to extract the `packageLocation` value and validate it for different VMs

**In Postman**, create a test script:

```javascript
// In Tests tab of GET Deployments request

// Parse response
const response = pm.response.json();

// Extract first deployment's packageLocation
if (response.deployments && response.deployments.length > 0) {
    const deployment = response.deployments[0];
    const packageLocation = deployment.spec?.deploymentProfile?.components?.[0]?.properties?.packageLocation;
    
    // Save for later use
    pm.environment.set("deployment_package_location", packageLocation);
    
    // Validate format
    tests["packageLocation is present"] = packageLocation !== null && packageLocation !== undefined;
    tests["packageLocation is not empty"] = packageLocation.length > 0;
    tests["packageLocation is string"] = typeof packageLocation === 'string';
    
    // Log for debugging
    console.log("Package Location:", packageLocation);
}

tests["Response status is 200"] = responseCode.code === 200;
```

**Then in next request**, use the extracted value:
```
GET {{deployment_package_location}}
```

---

## Test Coverage: Deployment-Related Tests

| Test Name | What It Validates | Expected Result |
|-----------|------------------|-----------------|
| GET Deployments - Headers Validation | Deployments endpoint returns 200 and includes cache headers | ✅ PASS |
| GET Deployments - ETag Validation | 304 Not Modified responses for unchanged deployments | ✅ PASS |
| GET Single Deployment - Not Found | Returns 404 for non-existent deployment ID | ✅ PASS |
| GET Single Deployment - Found | Returns specific deployment with correct packageLocation | ✅ PASS |
| POST Deployment Status | Device can report deployment status back to mock-WFM | ✅ PASS |

---

## Troubleshooting

### Issue: packageLocation is null or empty

**Cause:** Deployment manifest is malformed

**Solution:**
1. Check mock-data/deployments/compose-sample.yaml syntax (YAML validation)
2. Verify metadata.name and spec fields are present
3. Restart mock-WFM: `./start-mock-wfm.sh`

---

### Issue: Cannot reach packageLocation URL

**If HTTP URL:**
```bash
# From device-agent VM, test connectivity
curl -v http://mock-wfm-ip:8080/docker-compose-sample.yaml

# Check mock-WFM server is running HTTP server
ps aux | grep "python"  # or nginx
```

**If relative path:**
```bash
# Check working directory
pwd

# Verify file exists relative to cwd
ls -l ./docker-compose-sample.yaml
```

---

### Issue: Postman tests fail with 403 Forbidden

**Cause:** TLS certificate validation in HTTPS mode

**Solution:**
1. Disable SSL verification in Postman:
   - Settings → SSL Certificate Verification → OFF
   - (Only for testing! Don't use in production)
2. Or set correct environment for HTTPS:
   - Use: `https://localhost:8443/v1alpha2/margo`
   - Copy ca-cert.pem to Postman certificate manager

---

## CI/CD Integration

To run the Postman collection in CI/CD:

```bash
# Install Newman (Postman CLI runner)
npm install -g newman

# Run collection
newman run \
  mock-wfm-comprehensive.postman_collection.json \
  -e environment.json \
  --reporters cli,json \
  --reporter-json-export test-results.json

# Exit code indicates pass/fail
echo "Exit code: $?"
```

---

## See Also

- [mock-wfm-comprehensive.postman_collection.json](./postman/mock-wfm-comprehensive.postman_collection.json) - Full Postman collection
- [POSTMAN_COLLECTION_SETUP_GUIDE.md](../../POSTMAN_COLLECTION_SETUP_GUIDE.md) - Postman setup instructions
- [MULTI_VM_DEPLOYMENT_GUIDE.md](./MULTI_VM_DEPLOYMENT_GUIDE.md) - Multi-VM configuration strategies
- [DEVICE_AGENT_INTEGRATION.md](./DEVICE_AGENT_INTEGRATION.md) - Device-agent integration with mock-WFM
- [wfm-sbi.yaml](./wfm-sbi.yaml) - OpenAPI specification

---

## Quick Command Reference

```bash
# Start mock-WFM
cd /home/margo/nitin/sandbox/standard/cmd/mock-wfm
./start-mock-wfm.sh

# Test HTTP endpoint
curl http://localhost:8090/v1alpha2/margo/api/v1/onboarding/certificate

# Test HTTPS endpoint
curl -k https://localhost:8443/v1alpha2/margo/api/v1/onboarding/certificate

# Verify compose file is accessible (for relative path strategy)
ls -l ./docker-compose-sample.yaml

# Verify compose file is valid
docker-compose -f ./docker-compose-sample.yaml config

# Start HTTP server (for HTTP URL strategy)
cd /home/margo/nitin/sandbox/docker-compose
python3 -m http.server 8080 --bind 0.0.0.0

# Run all tests
cd /home/margo/nitin/sandbox/standard/cmd/mock-wfm
./comprehensive_tests.sh
```

