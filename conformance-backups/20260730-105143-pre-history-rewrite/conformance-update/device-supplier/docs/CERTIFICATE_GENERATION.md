# 🔐 TLS Certificate Generation for Mock WFM Server

This document explains how to generate and deploy TLS certificates for testing the Margo device-agent with the mock WFM server.

## Overview

The `generate-certs.sh` script creates a certificate chain:
- **CA Certificate** (`ca-cert.pem`) — Root CA that signs all certificates
- **CA Private Key** (`ca-private.key`) — Secret key for signing server certificate
- **Server Certificate** (`server.crt`) — TLS certificate for HTTPS on mock-server
- **Server Private Key** (`server.key`) — Secret key for HTTPS decryption

## Quick Start

### Step 1: Generate Certificates

```bash
cd /home/margo/nitin/margo/personas/device_supplier

# Generate in default location (./certs)
bash generate-certs.sh

# OR specify custom location and hostname
bash generate-certs.sh ./certs 192.168.1.100
```

**Output:**
```
🔐 Generating TLS Certificates for Margo Mock WFM Server
════════════════════════════════════════════════════════
   Output directory: ./certs
   Server host: localhost
   Validity: 365 days

[1/5] Generating CA private key...
[2/5] Generating CA certificate...
[3/5] Generating server private key...
[4/5] Generating server CSR...
[5/5] Generating server certificate...

✅ Certificate generation complete!
```

### Step 2: Start Mock WFM Server

```bash
cd /home/margo/nitin/margo/personas/device_supplier
go run server.go

# Output:
# ✓ Loaded assertions from: manifests/assertions.json
# ✓ Using existing server TLS certificates
# 🚀 Mock WFM Server starting on https://localhost:3001
```

### Step 3: Copy CA Certificate to Device-Agent VM

**Option A — Direct SCP (if on same network):**
```bash
mkdir -p /root/certs
scp /home/margo/nitin/margo/personas/device_supplier/certs/ca-cert.pem \
    root@DEVICE_VM:/root/certs/
```

**Option B — Manual copy:**
1. Copy file from: `/home/margo/nitin/margo/personas/device_supplier/certs/ca-cert.pem`
2. Paste to device VM at: `/root/certs/ca-cert.pem`

**Option C — Auto-discovery (copy to home):**
```bash
mkdir -p ~/.certs
cp /home/margo/nitin/margo/personas/device_supplier/certs/ca-cert.pem ~/.certs/
```

### Step 4: Verify Setup

On device-agent VM:
```bash
ls -la /root/certs/ca-cert.pem
# Output: -rw-rw-r-- 1 root root 1294 Apr 16 09:16 /root/certs/ca-cert.pem
```

---

## Certificate File Locations

After generation, certificates are located at:

| File | Purpose | Permissions | Location |
|------|---------|-------------|----------|
| `ca-cert.pem` | Root CA certificate (public) | 644 | `./certs/ca-cert.pem` |
| `ca-private.key` | CA private key (secret) | 600 | `./certs/ca-private.key` |
| `server.crt` | Server HTTPS certificate | 644 | `./certs/server.crt` |
| `server.key` | Server HTTPS private key | 600 | `./certs/server.key` |

---

## How Mock-Server Uses Certificates

```
Mock-Server Startup (server.go)
├─ ensureTLSCertificates() function
├─ Check ./certs/ca-cert.pem (copy from home if missing)
├─ Check ./certs/server.crt
├─ If missing → generateCASignedServerCert()
│   ├─ Signs server.crt using ca-cert.pem + ca-private.key
│   └─ Writes server.crt + server.key
├─ Listen HTTPS localhost:3001
├─ Load server.crt + server.key
└─ Ready for HTTPS connections
```

---

## How Device-Agent Uses Certificates

```
Device-Agent Connection (device-agent.sh)
├─ Read config: sbiUrl = https://localhost:3001/v1alpha2/margo
├─ Load /root/certs/ca-cert.pem (trust this CA)
├─ Connect to server HTTPS endpoint
├─ Verify server cert signed by trusted CA
│   └─ Server cert must match CN=localhost or IP=127.0.0.1
├─ Load /root/certs/device-private.key
├─ Create RFC 9421 signed request
├─ Add signatures + Content-Digest headers
└─ Send request to mock-server
```

---

## Certificate Validation

### Verify CA Certificate
```bash
openssl x509 -in certs/ca-cert.pem -text -noout
```

**Expected output includes:**
```
Issuer: C = IN, ST = GGN, L = Sector48, O = Margo, OU = WFM, CN = Mock-WFM-CA
Subject: C = IN, ST = GGN, L = Sector48, O = Margo, OU = WFM, CN = Mock-WFM-CA
Not Before: Apr 16 09:16:20 2026 GMT
Not After : Apr 16 09:16:20 2027 GMT
Public-Key: (2048 bit, RSA)
```

### Verify Server Certificate
```bash
openssl x509 -in certs/server.crt -text -noout
```

**Expected output includes:**
```
Issuer: C = IN, ST = GGN, L = Sector48, O = Margo, OU = WFM, CN = Mock-WFM-CA
Subject: C = US, ST = State, L = City, O = Margo, OU = WFM, CN = localhost
DNS:localhost, IP Address:127.0.0.1
```

### Verify Certificate Chain
```bash
openssl verify -CAfile certs/ca-cert.pem certs/server.crt

# Expected output:
# certs/server.crt: OK
```

---

## Testing End-to-End

### Terminal 1: Start Mock-Server
```bash
cd /home/margo/nitin/margo/personas/device_supplier
go run server.go
```

### Terminal 2: Run Conformance Tests
```bash
cd /home/margo/nitin/margo/personas/device_supplier
go run run_tests.go
```

**Expected output:**
```
✓ Loaded device certificate (1294 bytes)
✅ WFM Server is ready

▶ Running Scenario: Device Onboarding
  → Step: Get Root CA Certificate
    ✅ PASS (HTTP 200)
  → Step: Onboard Trusted Device
    ✅ PASS (HTTP 201)
...

║  Test Results: 33 PASSED, 3 FAILED (Total: 36)
║  Success Rate: 91.7%
```

---

## Troubleshooting

### "Certificate not found" error
**Problem:** Server starts but can't find certificates

**Solution:**
```bash
# Regenerate certificates
cd /home/margo/nitin/margo/personas/device_supplier
bash generate-certs.sh
```

### "Connection refused" on device-agent
**Problem:** Device-agent can't connect to mock-server

**Cause:** Possible issues
1. Mock-server not running
2. Device-agent pointing to wrong URL
3. CA certificate not copied to device VM

**Solution:**
```bash
# Verify mock-server is running
lsof -i :3001

# Verify ca-cert.pem exists on device VM
ssh root@DEVICE_VM "ls -la /root/certs/ca-cert.pem"

# Verify device-agent config
ssh root@DEVICE_VM "grep sbiUrl /config/config.yaml"
```

### "Certificate verification failed"
**Problem:** Device-agent rejects server certificate

**Cause:** CA certificate mismatch or hostname validation

**Solution:**
```bash
# Verify server cert is signed by CA
openssl verify -CAfile certs/ca-cert.pem certs/server.crt

# Check certificate CN matches request hostname
openssl x509 -in certs/server.crt -noout | grep Subject

# Should include: CN=localhost or CN=127.0.0.1
```

---

## File Dependencies

```
generate-certs.sh (this script)
├─ Reads: OpenSSL binary
├─ Creates: ./certs/
│  ├─ ca-cert.pem (generated)
│  ├─ ca-private.key (generated)
│  ├─ server.crt (generated)
│  ├─ server.key (generated)
│  └─ ca-cert.srl (OpenSSL serial tracker)
└─ Output: All certificates ready

server.go (mock-server)
├─ Reads: ./certs/ca-cert.pem
├─ Reads: ./certs/ca-private.key (optional, for signing)
├─ Reads: ./certs/server.crt
├─ Reads: ./certs/server.key
└─ Listens: HTTPS localhost:3001

Device-Agent
├─ Reads: /root/certs/ca-cert.pem (trust store)
├─ Reads: /root/certs/device-private.key (for signing)
└─ Connects: https://localhost:3001/v1alpha2/margo
```

---

## Certificate Lifecycle

| Event | Action | Duration |
|-------|--------|----------|
| Certificate Generated | Valid for 365 days | Now → 1 year |
| Server Starts | Loads certificates | On startup |
| Device-Agent Connects | Validates server cert | Each request |
| Cert Expires | Regenerate certificates | After 365 days |

---

## See Also

- [README.md](README.md) — Main documentation
- [server.go](server.go#L1283) — Certificate handling code
- [OpenAPI Spec](https://raw.githubusercontent.com/margo/specification/pre-draft/system-design/specification/margo-management-interface/workload-management-api-1.0.0.yaml) — TLS requirements

