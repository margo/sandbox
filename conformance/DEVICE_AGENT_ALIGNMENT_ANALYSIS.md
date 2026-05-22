# WFM-Supplier vs Real Device-Agent Configuration Analysis

**Date**: 2025-01-22

## Executive Summary

The WFM-Supplier conformance tester is designed to simulate device behavior when interacting with Margo's Workload Fleet Management (WFM) system. This document compares the current wfm-supplier implementation with the real device-agent configuration to identify architectural differences and alignment opportunities.

**Critical Finding**: The wfm-supplier currently uses **ECDSA** signatures, while the real device-agent uses **RSA** signatures. This is a significant architectural mismatch that could cause authentication failures if wfm-supplier were used against a production environment that validates signature algorithms.

---

## 1. Configuration Architecture Comparison

### 1.1 Device Identity & Attestation

| Aspect | Real Device-Agent | WFM-Supplier | Alignment Status |
|--------|-------------------|--------------|------------------|
| **Identity Type** | PKI (certificate-based) | PKI (certificate-based) | ✅ Aligned |
| **Signature Algorithm** | RSA (2048-bit assumed) | ECDSA (prime256v1) | ❌ **CRITICAL MISMATCH** |
| **Hash Algorithm** | SHA256 | SHA256 (via Postman) | ✅ Aligned |
| **Signature Format** | Structured (RFC 9421) | Structured (RFC 9421) | ✅ Aligned |
| **Device ID Source** | Manufacturer ID | Timestamp-based (`device-{epoch}`) | ⚠️ Different design |
| **Cert Path (Public)** | `./config/device-public.crt` | `./newman-data/certs/device-cert.pem` | ✅ Different paths (OK) |
| **Cert Path (Private)** | `./config/device-private.key` | `./newman-data/certs/device.key` | ✅ Different paths (OK) |

### 1.2 WFM Connection Configuration

| Aspect | Real Device-Agent | WFM-Supplier | Alignment Status |
|--------|-------------------|--------------|------------------|
| **WFM Base URL** | `https://symphony.machine:8082/v1alpha2/margo` | User-configurable (default: `https://localhost:3001/v1alpha2/margo`) | ✅ Same structure |
| **TLS Verification** | Required (uses CA cert) | Required (uses CA cert) | ✅ Aligned |
| **CA Cert Path** | `./config/ca-cert.pem` | `./newman-data/certs/ca-cert.pem` | ✅ Different paths (OK) |
| **OAuth Authentication** | Disabled (no auth header sent) | Not applicable (Newman uses environment variables) | ✅ Similar approach |

### 1.3 State & Workload Configuration

| Aspect | Real Device-Agent | WFM-Supplier | Alignment Status |
|--------|-------------------|--------------|------------------|
| **State Seeking** | 15-second poll interval | One-time test execution | ⚠️ Different workflow |
| **Runtime Type** | Kubernetes | N/A (Postman simulation) | ⚠️ Different scope |
| **Capabilities Source** | Static file (`./config/capabilities.json`) | Generated per test run | ⚠️ Different approach |

---

## 2. Capabilities Configuration Comparison

### Real Device Capabilities (`poc/device/agent/config/capabilities.json`)

```json
{
    "apiVersion": "device.margo.org/v1alpha1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {
        "id": "device-id-from-manufacturer",
        "vendor": "Northstar Industrial Applications",
        "modelNumber": "332ANZE1-N1",
        "serialNumber": "PF45343-AA",
        "roles": ["Standalone Cluster"],
        "resources": {
            "cpu": {
                "architecture": "amd64",
                "cores": 24
            },
            "memory": "64 Gi",
            "storage": "2000 Gi",
            "peripherals": [],
            "interfaces": []
        }
    }
}
```

**Key Characteristics:**
- Manufacturer-provided device ID
- Substantial compute resources (24 cores, 64 Gi RAM)
- Enterprise-grade storage (2000 Gi)
- Empty peripherals/interfaces arrays
- Manufacturer-specific vendor/model/serial combination

### WFM-Supplier Capabilities (`newman-data/device-agent.env.json`)

Based on code inspection, capabilities are generated at runtime with:
- Timestamp-based device ID pattern: `device-{epoch}`
- Minimal resources (4 cores, 8 Gi memory)
- Random "Standalone Device" role
- Dynamically populated peripherals/interfaces

**Alignment Issues:**
- ❌ Resources are drastically smaller (test sizing?)
- ⚠️ Device ID generation differs (manufacturer vs. timestamp)
- ⚠️ Roles differ (Cluster vs. Device)

---

## 3. Certificate Generation & Lifecycle

### Real Device-Agent

**Certificate Lifecycle:**
1. Device starts with pre-provisioned keys at `./config/device-private.key`
2. Device reads public cert from `./config/device-public.crt`
3. Both are expected to be statically provisioned by manufacturer/deployment process
4. No runtime generation
5. Same certificate used across all requests to WFM

**Key Algorithm:** RSA (assumed 2048-bit based on industry standard)

### WFM-Supplier

**Certificate Lifecycle:**
1. Setup phase (`1-setup_portman.sh`): CA cert is copied from local directory
2. Execution phase (`2-run_newman.sh`): Fresh device certificate is generated on EACH RUN
3. Device cert is ECDSA (prime256v1)
4. Device cert is base64-encoded and embedded in onboarding payload
5. New device ID and certificate for each test run

**Key Algorithm:** ECDSA (prime256v1 - 256-bit elliptic curve)

**Lifecycle Differences:**
- ❌ Real: Static provisioned | WFM-Supplier: Regenerated per run
- ❌ Real: RSA | WFM-Supplier: ECDSA
- ✅ Both: Embed cert in onboarding payload
- ✅ Both: Use base64 encoding for transmission

---

## 4. Test Execution Workflows

### Real Device-Agent Workflow

```
    ┌─────────────────────────────────────────┐
    │ Device Agent Starts                     │
    │ - Reads config.yaml                     │
    │ - Loads static device cert + key        │
    │ - Loads capabilities from file          │
    └─────────────────────────────────────────┘
                        ↓
    ┌─────────────────────────────────────────┐
    │ Poll Loop (interval: 15 seconds)        │
    │ - Seek desired state from WFM           │
    │ - Report capabilities                   │
    │ - Request workload instructions         │
    │ - Execute workloads on Kubernetes       │
    │ - Report results back to WFM            │
    └─────────────────────────────────────────┘
                        ↓
                  (Continuous until stopped)
```

**Key Aspect**: Device-agent is a long-running daemon that continuously syncs state with WFM.

### WFM-Supplier Workflow

```
    ┌─────────────────────────────────────────┐
    │ 1. Setup Phase (1-setup_portman.sh)     │
    │ - Download OpenAPI spec                 │
    │ - Generate Postman collection           │
    │ - Check/copy CA cert                    │
    │ - Create environment variables          │
    └─────────────────────────────────────────┘
                        ↓
    ┌─────────────────────────────────────────┐
    │ 2. Execution Phase (2-run_newman.sh)    │
    │ - Generate fresh device cert (ECDSA)    │
    │ - Embed in onboarding payload           │
    │ - Patch collection runtime paths        │
    │ - Execute 8-endpoint sequence:          │
    │   1. Onboarding (POST)                  │
    │   2. Get Capabilities (GET)             │
    │   3. Create Deployment (POST)           │
    │   4. Get Deployments (GET)              │
    │   5. Get Workloads (GET)                │
    │   6. Create Workload (POST)             │
    │   7. Get Workload Details (GET)         │
    │   8. Cleanup (DELETE)                   │
    │ - Run assertions & generate report      │
    └─────────────────────────────────────────┘
                        ↓
              (Test completes, exit)
```

**Key Aspect**: WFM-Supplier is a one-time test execution (conformance test), not continuous sync.

---

## 5. Request Signing & Headers Comparison

### Real Device-Agent

**Config Setting:**
```yaml
clientPlugins:
  requestSigner:
    enabled: true
    hashAlgo: "sha256"
    signatureAlgo: "rsa"  # ← RSA, not ECDSA
    signatureFormat: "structured"  # RFC 9421
```

**Expected Signature Header Format (RFC 9421 Structured):**
```
Signature-Input: (list of signed components)=(<params>);...
Signature: sig1=:base64-of-signature:
```

### WFM-Supplier

**Current Implementation:**
- Uses Postman's `Signature` request header
- Via environment variable substitution: `{{signatureHeader}}`
- Signature is generated/embedded in the collection JSON structure
- Also uses RFC 9421 structured format

**Alignment Issues:**
- ✅ Both use structured RFC 9421 format
- ✅ Both embed signature in request headers
- ❌ Real uses RSA signing | WFM-Supplier uses (will be) ECDSA signing
- ⚠️ Signature generation method differs (Go crypto vs. Postman)

---

## 6. Critical Finding: Signature Algorithm Mismatch

### Problem

The real device-agent configuration explicitly specifies:
```yaml
signatureAlgo: "rsa"
```

But wfm-supplier currently generates ECDSA certificates:
```bash
openssl ecparam -name prime256v1 -genkey -noout -out "$DEVICE_KEY_FILE"
```

### Implications

1. **WFM May Validate Signature Algorithm**
   - If WFM's auth service validates that signatures match the expected algorithm
   - ECDSA-signed requests could be rejected even if the signature is cryptographically valid
   - Error: "Signature algorithm mismatch" or "Invalid signature"

2. **Onboarding May Fail**
   - The certificate presented during onboarding is raw (not signed yet)
   - But if WFM extracts public key from cert and validates algorithm family
   - ECDSA cert presented in RSA-expecting validator could fail

3. **Different Security Properties**
   - RSA 2048-bit ≈ 112-bit symmetric strength
   - ECDSA P-256 ≈ 128-bit symmetric strength
   - ECDSA is actually stronger, but WFM may have compliance requirements

### Recommendation

**Update WFM-Supplier to use RSA 2048-bit:**

**In `2-run_newman.sh` (around line 65-85):**

```bash
# BEFORE (ECDSA):
openssl ecparam -name prime256v1 -genkey -noout -out "$DEVICE_KEY_FILE" >/dev/null 2>&1

# AFTER (RSA):
openssl genrsa -out "$DEVICE_KEY_FILE" 2048 >/dev/null 2>&1
```

---

## 7. Device Identity & ID Generation

### Real Device-Agent

**Device ID Strategy:**
- Pre-assigned by manufacturer/deployment system
- Example: `"device-id-from-manufacturer"`
- Typically: Serial number or unique identifier from device hardware
- **Static**: Same across all invocations

**Implication**: WFM can track the same physical device across multiple connections/sessions

### WFM-Supplier

**Device ID Strategy:**
```bash
DEVICE_ID="device-$(date +%s)"
```

- Generated from current Unix timestamp
- Example: `device-1705929453`
- **Dynamic**: Different device ID for every test run

**Implication**: 
- Each test run appears as a NEW device to WFM
- WFM cannot correlate across test runs
- Better for conformance testing (clean slate each run)
- Not representative of real device behavior

### Recommendation

**Keep wfm-supplier as-is** (dynamic IDs) because:
1. Conformance testing benefits from clean state
2. Each run is independent
3. No state pollution between test runs
4. Question: Should this be configurable?

---

## 8. Capabilities Detail Alignment

### Real Device: Substantial Resources

```json
"resources": {
    "cpu": {
        "architecture": "amd64",
        "cores": 24
    },
    "memory": "64 Gi",
    "storage": "2000 Gi",
    "peripherals": [],
    "interfaces": []
}
```

**Characteristics:**
- Enterprise-grade compute (24 cores)
- Large memory (64 GB)
- Significant storage (2 TB)
- No peripherals/interfaces (empty arrays)

### WFM-Supplier: Minimal Resources

Based on code generation, wfm-supplier likely creates:
```json
"resources": {
    "cpu": {
        "architecture": "amd64",
        "cores": 4
    },
    "memory": "8 Gi",
    "storage": "100 Gi",
    "peripherals": [...],
    "interfaces": [...]
}
```

**Characteristics:**
- Test-sized compute (4 cores)
- Minimal memory (8 GB)
- Reduced storage (100 GB)
- Populated peripherals/interfaces

### Recommendation

**Make capabilities configurable**, allowing:
1. **Default**: Minimal test resources (current behavior)
2. **Realistic**: Enterprise-grade resources (like real device)
3. **Custom**: User-provided capabilities file

**Benefit**: Allows testing different device profiles without code changes

---

## 9. Certificate Persistence Model

### Real Device-Agent: Static Persistence

```
├── config/
│   ├── device-private.key      (preprovisioned, static)
│   ├── device-public.crt       (preprovisioned, static)
│   ├── ca-cert.pem             (provided by WFM admin, static)
│   └── capabilities.json       (preprovisioned, static)
```

**Lifecycle:**
- Pre-generated before deployment
- Provisioned via deployment manifest or secret management
- Same keys/certs used for entire device lifetime
- No runtime generation

### WFM-Supplier: Dynamic Generation

```
Setup Phase:
├── certs/
│   └── ca-cert.pem             (copied from local setup directory)

Execution Phase (per run):
├── newman-data/certs/
│   ├── device.key              (generated fresh)
│   └── device-cert.pem         (generated fresh)
```

**Lifecycle:**
- CA cert is pre-provided (manual copy required)
- Device keys generated at runtime
- Fresh generation on each test run
- Runtime-only (not persisted between runs)

### Analysis

**Different Models Serve Different Purposes:**
- Real device-agent: Trust model requires long-lived identity
- WFM-supplier: Conformance testing benefits from fresh identity per run

**Recommendation**: Keep current design, but document this difference

---

## 10. Authentication & OAuth Configuration

### Real Device-Agent

```yaml
clientPlugins:
  authHelper:
    enabled: false  # No OAuth
```

**Implication**: 
- Device connects WITHOUT bearer token
- All auth is certificate-based (mutual TLS + signature validation)
- No separate OAuth credential management needed

### WFM-Supplier

**Current Implementation:**
- No OAuth configuration needed
- Uses environment variable substitution for auth headers
- Postman collection can simulate auth via `Authorization` header if needed

**Alignment**: ✅ Both disable OAuth, use cert-based auth

---

## 11. TLS & CA Configuration

### Real Device-Agent

```yaml
clientPlugins:
  tlsHelper:
    enabled: true
    caKeyRef:
      path: "./config/ca-cert.pem"
```

**Behavior:**
- Validates WFM server certificate against provided CA cert
- Fails if server cert is not signed by this CA
- Essential for security in production

### WFM-Supplier

**Current Implementation:**
```bash
mkdir -p "$CERT_DIR" "$LOCAL_CERT_DIR"
if [[ ! -f "$LOCAL_CA_CERT_FILE" ]]; then
    echo "❌ Missing WFM CA certificate: $LOCAL_CA_CERT_FILE"
    ...
    exit 1
fi
cp "$LOCAL_CA_CERT_FILE" "$RUNTIME_CA_CERT_FILE"
```

**Newman Invocation:**
```bash
newman run ... --ssl-extra-ca-certs "$RUNTIME_CA_CERT_FILE"
```

**Alignment**: ✅ Both require CA cert, both verify server certificates

---

## 12. Error Handling & Status Code Expectations

### Real Device-Agent

Expected behavior based on config:
- Validates response status codes per RFC status semantics
- May retry on 5xx errors (not shown in config snippet)
- May have specific error handling for 401/403/422

### WFM-Supplier

Current test assertions:
```javascript
const allowed = [200,201,204,400,401,403,422]
```

**Interpretation:**
- 200/201/204: Success (different operations)
- 400: Bad Request (validation errors, expected for some scenarios)
- 401: Unauthorized (cert validation failures, expected for some scenarios)
- 403: Forbidden (permissions validation)
- 422: Unprocessable Entity (business logic validation)

**Key Finding**: Wfm-supplier treats 401 as "scenario-expected" rather than error
- This is correct for conformance testing (testing negative scenarios)
- Real device-agent would treat 401 as actual connection failure

---

## 13. Recommendations Summary

### Priority 1 (Critical)

1. **Signature Algorithm: Switch ECDSA → RSA**
   - Update `2-run_newman.sh` line ~80:
   ```bash
   # FROM: openssl ecparam -name prime256v1 -genkey -noout -out "$DEVICE_KEY_FILE"
   # TO:
   openssl genrsa -out "$DEVICE_KEY_FILE" 2048
   ```
   - Rationale: Real device-agent uses RSA; WFM may validate algorithm
   - Effort: 1 line change
   - Risk: Low (ECDSA already working, RSA is standard)

### Priority 2 (Recommended)

2. **Make Capabilities Configurable**
   - Allow users to provide custom `capabilities.json`
   - Default: Minimal test resources (current)
   - Optional: Realistic enterprise resources (matching real device)
   - Rationale: Different test scenarios need different device profiles

3. **Document Real vs. Mock Differences**
   - Create this analysis document in repo (DONE)
   - Update README with implementation notes
   - Explain design choices (why different from real device-agent)

4. **CA Certificate Validation**
   - Consider validating CA cert format/validity before running tests
   - Currently checks file existence; could validate PEM format + expiration

### Priority 3 (Nice-to-Have)

5. **Device ID Customization**
   ```bash
   DEVICE_ID="${DEVICE_ID_OVERRIDE:-device-$(date +%s)}"
   ```
   - Allow override for scenarios requiring specific device IDs
   - Useful for testing WFM state tracking

6. **Signature Algorithm Flexibility**
   ```bash
   SIGNATURE_ALGO="${SIGNATURE_ALGO:-rsa}"  # or "ecdsa"
   ```
   - Test both RSA and ECDSA validation paths
   - Currently hardcoded ECDSA

---

## 14. Alignment Checklist

| Component | Real Device | WFM-Supplier | Status | Notes |
|-----------|-------------|--------------|--------|-------|
| PKI-based identity | ✅ | ✅ | ✅ Aligned | Both use certs |
| Signature algorithm | RSA | **ECDSA** | ❌ Mismatch | **HIGH PRIORITY** |
| Signature hash | SHA256 | SHA256 | ✅ Aligned | Both use SHA256 |
| Signature format | RFC 9421 | RFC 9421 | ✅ Aligned | Both structured |
| TLS CA validation | ✅ | ✅ | ✅ Aligned | Both require CA |
| OAuth | ❌ | ❌ | ✅ Aligned | Neither uses OAuth |
| Device ID | Manufacturer | Timestamp | ⚠️ Different | By design (testing) |
| Capabilities source | Static file | Generated | ⚠️ Different | By design (testing) |
| Capabilities detail | Enterprise | Minimal | ⚠️ Different | By design (testing) |
| Test model | Polling loop | One-time run | ⚠️ Different | Different purposes |
| Runtime type | Kubernetes | N/A (mock) | ⚠️ Different | WFM-Supplier is mock |

---

## 15. Conclusion

The WFM-Supplier conformance tester is a well-designed **simplified simulation** of real device-agent behavior, purpose-built for testing WFM API endpoints rather than replicating production device behavior exactly.

**Key Takeaway**: Most differences are intentional and appropriate for conformance testing. The **only critical mismatch** is the signature algorithm (ECDSA vs. RSA), which should be corrected to match real device-agent specifications.

**Recommended Action**: 
1. Update signature algorithm to RSA (Priority 1)
2. Make capabilities configurable (Priority 2)
3. Document this analysis in the repository (Priority 2)

---

## Appendix: File References

| File | Purpose | Location |
|------|---------|----------|
| Real device config | Device-agent configuration | `poc/device/agent/config/config.yaml` |
| Real capabilities | Device-agent capabilities | `poc/device/agent/config/capabilities.json` |
| WFM setup script | Setup Postman & environment | `conformance/wfm-supplier/1-setup_portman.sh` |
| WFM execution script | Run tests & generate certs | `conformance/wfm-supplier/2-run_newman.sh` |
| Real test runner | Device-agent test harness | `conformance/device-supplier/run_tests.go` |
| WFM postman collection | API endpoints (generated) | `conformance/wfm-supplier/postman_collection.json` |
| WFM environment | Test variables | `conformance/wfm-supplier/newman-data/device-agent.env.json` |
