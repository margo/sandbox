# Device Certificate Architecture - Self-Contained Conformance Suite

## Overview

The Device Supplier Conformance Test Suite now uses **self-contained, locally-generated certificates** instead of depending on external device agent locations. This makes the test suite portable, isolated, and production-ready.

## Problem: Old Design (❌ Not Scalable)

**Old Code:**
```go
// Load device certificate from sandbox location (external path)
certData, err := os.ReadFile("/home/margo/sandbox/poc/device/agent/config/device-public.crt")
```

**Issues:**
- ❌ **Hard-coded external path** - Breaks when device agent is on different VM
- ❌ **Not portable** - Cannot run on different machines or environments  
- ❌ **External dependency** - Test suite depends on device agent being available
- ❌ **Test isolation failure** - Tests tightly coupled to development environment
- ❌ **Parallel testing blocked** - Multiple test runs interfere with same external resource

**Real-world impact:**
- Device agent on VM1, test runner on VM2 → Test cannot find certificate
- CI/CD pipeline → External paths don't exist in containers
- Multiple teams testing → Conflicts over certificate paths

---

## Solution: New Design (✅ Self-Contained)

**New Code:**
```go
// Load device certificate from conformance suite's local certs folder
certPath := "./certs/device-cert.pem"
certData, err := os.ReadFile(certPath)
```

**Architecture:**
```
device_supplier/
├── generate-certs.sh          ← Generates test certificates
├── certs/                      ← LOCAL, self-contained certificates
│   ├── ca-cert.pem             (CA certificate)
│   ├── server-cert.pem         (Server TLS certificate)
│   ├── device-cert.pem   (✅ Valid device for positive tests)
│   ├── device-invalid-cert.pem (❌ Expired device for negative tests)
│   └── device-revoked-cert.pem (⛔ Revoked device for rejection tests)
├── manifests/
│   └── assertions.json         ← References actual cert content in rejection list
└── run_tests.go                ← Loads from ./certs/ (relative path)
```

---

## Certificate Generation

**Step 1:** Generate all certificates (done automatically):
```bash
bash generate-certs.sh ./certs
```

**Generated files:**
- `device-cert.pem` - Valid for 365 days, used for positive test scenarios
- `device-invalid-cert.pem` - Valid for 1 day, used for negative tests  
- `device-revoked-cert.pem` - Valid for 365 days, but registered in rejection list

**Step 2:** Rejection list (`assertions.json`):
```json
{
  "rejected_certificates": [
    "MIID-rejected-device-cert",     // Placeholder for test scenarios
    "-----BEGIN CERTIFICATE-----\n..." // Actual revoked certificate content
  ]
}
```

---

## How Certificate Injection Works

### Scenario 1: Positive Test (Valid Device Onboard) ✅

**Test Scenario:**
```json
{
  "step": "Onboard Trusted Device",
  "request_body": {
    "certificate": "MIID-valid-device-cert"  // Placeholder
  },
  "skip_certificate_injection": false  // Default - will be replaced
}
```

**Execution Flow:**
1. Test runner loads: `./certs/device-cert.pem` (1342 bytes)
2. Server receives actual PEM certificate (starts with `-----BEGIN CERTIFICATE-----`)
3. validate content, store client record ✅ HTTP 201 Created

### Scenario 2: Negative Test (Rejected Device) ⛔

**Test Scenario:**
```json
{
  "step": "Reject Blocklisted Certificate",
  "request_body": {
    "certificate": "MIID-rejected-device-cert"  // Placeholder
  },
  "skip_certificate_injection": true  // Keep placeholder as-is
}
```

**Execution Flow:**
1. Placeholder `"MIID-rejected-device-cert"` is NOT replaced
2. Server checks: is this certificate in `rejected_certificates` list? ✅ YES
3. Server responds: HTTP 403 Forbidden - "Client rejected" ⛔

### Scenario 3: Real Device Connection (Production-like) 🚀

**Device Agent brings its actual certificate:**
```
Device.onboard({
  certificate: "-----BEGIN CERTIFICATE-----\nMIIDvzCCAq...\n-----END CERTIFICATE-----"
})
```

**Server checks:**
1. Is this certificate in rejection list? 
   - If YES → 403 Forbidden
   - If NO → 201 Created + store client

---

## Test Scenarios Supported

| Scenario | Certificate | Injection | Expected Result |
|----------|------------|-----------|-----------------|
| Onboard valid device | `device-cert.pem` | ✅ Injected | 201 Created |
| Reject blocklisted device | Placeholder string | ❌ Not injected | 403 Forbidden |
| Real revoked device | `device-revoked-cert.pem` | From device | 403 Forbidden |
| Capabilities report | `device-cert.pem` | ✅ Injected | 201 Created |
| Deployment retrieval | `device-cert.pem` | ✅ Injected | 200 OK |
| Status update | `device-cert.pem` | ✅ Injected | 200 OK |

---

## Benefits

✅ **Portable** - Run on any machine, any environment  
✅ **Self-Contained** - No external dependencies  
✅ **Test Isolation** - Each test run uses fresh, local certificates  
✅ **Parallel Testing** - Multiple test runners don't conflict  
✅ **Production-Ready** - Matches real device onboarding flow  
✅ **CI/CD Compatible** - Works in containers, doesn't need external paths  
✅ **Maintainable** - Certificates are part of test suite repository  

---

## File Changes

### 1. `generate-certs.sh`
Added device certificate generation:
```bash
openssl genrsa -out "$OUTPUT_DIR/device-key.pem" 2048
openssl req -new -x509 -days $DAYS_VALID -key "$OUTPUT_DIR/device-key.pem" \
  -out "$OUTPUT_DIR/device-cert.pem" \
  -subj "/C=IN/ST=GGN/L=Sector48/O=AcmeCorp/OU=Devices/CN=device-001"
```

### 2. `run_tests.go`
Changed from:
```go
certData, err := os.ReadFile("/home/margo/sandbox/poc/device/agent/config/device-public.crt")
```

Changed to:
```go
certPath := "./certs/device-cert.pem"
certData, err := os.ReadFile(certPath)
```

### 3. `manifests/assertions.json`
Added actual certificates to rejection list:
```json
"rejected_certificates": [
  "MIID-rejected-device-cert",           // placeholder
  "-----BEGIN CERTIFICATE-----\n..."     // actual cert
]
```

---

## How Real Margo Devices Work

When a real Margo device connects from a different VM:

1. **Device generates its own certificate** (in its own VM)
2. **Device sends certificate** in onboarding request:
   ```json
   {
     "apiVersion": "onboarding.margo.org/v1alpha1",
     "kind": "OnboardingRequest",
     "certificate": "-----BEGIN CERTIFICATE-----\n[ACTUAL PEM]\n-----END CERTIFICATE-----"
   }
   ```

3. **Server checks rejection list:**
   - Does server's rejection list contain this certificate?
   - If yes → 403 Forbidden  
   - If no → 201 Created ✅

4. **Certificate is stored** for future signature verification

**Key insight:** The conformance suite test certificates simulate this same flow! They allow testing without needing actual physical devices.

---

## Running Tests

### Generate Certificates
```bash
bash generate-certs.sh ./certs
```

### Build Test Suite
```bash
make build
```

### Run Specific Test
```bash
./bin/run_tests -scenario scenario-onboarding -step step-1.3
```

### Run Full Test Suite
```bash
make demo
```

---

## Troubleshooting

**Error: Cannot load device certificate from ./certs/device-cert.pem**
- Run: `bash generate-certs.sh ./certs`
- Verify: `ls -la certs/device*.pem`

**Test says "Client rejected" but I expected success**
- Check: Device certificate is in rejection list?
- Verify: `grep "device-revoked" manifests/assertions.json`

**Certificate injection not working**
- Check: `skip_certificate_injection` flag in test scenario
- Default is `false` (injection enabled)
- Set to `true` to use placeholder strings

---

## References

- [Test Scenarios Definition](device-scenarios/test-scenarios.json)
- [Server Assertion Rules](manifests/assertions.json)  
- [Test Runner Logic](run_tests.go#L213)
- [Onboarding Validation](server.go#L800)
- [Certificate Rejection Check](server.go#L858)

