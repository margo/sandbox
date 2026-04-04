# Mock WFM Server - Device Agent Integration Guide

This guide explains how to integrate the Mock WFM Server with the Device Agent for HTTPS-based testing in a multi-VM environment.

## Architecture Overview

```
┌─────────────────────────────┐          ┌──────────────────────────────┐
│   Mock WFM Server VM        │          │   Device Agent VM            │
├─────────────────────────────┤          ├──────────────────────────────┤
│ Port 8090 (HTTP)            │          │ ~/sandbox/poc/device/agent/  │
│ Port 9090 (HTTPS)  ◄─────────────────►  config/config.yaml           │
│                             │          │ config/ca-cert.pem (copied)  │
│ ├─ server-cert.pem          │          │                              │
│ ├─ server-key.pem           │          └──────────────────────────────┘
│ └─ ca-cert.pem (share)      │
└─────────────────────────────┘
```

## Setup Steps

### Step 1: Generate TLS Certificates (Mock WFM Server VM)

```bash
cd /home/margo/nitin/sandbox/standard/cmd/mock-wfm
bash generate-certs.sh . <server-ip-or-hostname>
```

This generates:
- `ca-cert.pem` - Root CA certificate (copy to device-agent VM)
- `server-cert.pem` - Server certificate
- `server-key.pem` - Server private key

**Example for IP address:**
```bash
bash generate-certs.sh . 192.168.1.100
```

### Step 2: Build the Mock Server (Mock WFM Server VM)

```bash
cd /home/margo/nitin/sandbox/standard/cmd/mock-wfm
go build -o mock-wfm .
```

### Step 3: Start the Mock WFM Server (Mock WFM Server VM)

```bash
cd /home/margo/nitin/sandbox/standard/cmd/mock-wfm
bash start-mock-wfm.sh
```

Output should show:
```
🚀 Starting Mock WFM Server...
✅ Server started successfully (PID: XXXX)

📍 Server URLs:
   HTTP:  http://localhost:8090/v1alpha2/margo
   HTTPS: https://localhost:8443/v1alpha2/margo

🔐 HTTPS Configuration:
   Port: 8443
   Certificate: .../server-cert.pem
   Key: .../server-key.pem
   CA Certificate: .../ca-cert.pem
```

### Step 4: Copy CA Certificate to Device Agent VM

From Mock WFM Server VM to Device Agent VM:

```bash
# On Mock WFM Server VM
scp /home/margo/nitin/sandbox/standard/cmd/mock-wfm/ca-cert.pem \
    user@device-agent-vm:/home/user/sandbox/poc/device/agent/config/

# Or manually copy the content and paste on Device Agent VM
cat /home/margo/nitin/sandbox/standard/cmd/mock-wfm/ca-cert.pem
```

### Step 5: Configure Device Agent (Device Agent VM)

Update the device-agent configuration at `~/sandbox/poc/device/agent/config/config.yaml`:

```yaml
wfm:
  sbiUrl: https://<mock-wfm-server-ip>:9090/v1alpha2/margo
  clientPlugins:
    tlsHelper:
      enabled: true
      caKeyRef:
        path: ./config/ca-cert.pem
```

**Example with IP 192.168.1.100:**
```yaml
wfm:
  sbiUrl: https://192.168.1.100:9090/v1alpha2/margo
```

### Step 6: Test Integration

#### Option A: Quick Validation (from Mock WFM Server VM)

```bash
cd /home/margo/nitin/sandbox/standard/cmd/mock-wfm
bash device-agent-integration-test.sh <server-ip> 9090 ./ca-cert.pem
```

**Example:**
```bash
bash device-agent-integration-test.sh 192.168.1.100 9090 ./ca-cert.pem
```

#### Option B: Run Device Agent (from Device Agent VM)

```bash
cd ~/sandbox/poc/device/agent
# Build device-agent binary first
go build -o device-agent .

# Run onboarding
./device-agent --onboard

# Run status updates
./device-agent --status
```

## API Endpoints

All endpoints are available at: `https://<server-ip>:9090/v1alpha2/margo`

### Onboarding
```
GET  /api/v1/onboarding/certificate
POST /api/v1/onboarding
```

### Device Capabilities
```
PUT  /api/v1/clients/{clientId}/capabilities
POST /api/v1/clients/{clientId}/capabilities
```

### Deployments
```
GET  /api/v1/clients/{clientId}/deployments
GET  /api/v1/clients/{clientId}/deployments/{deploymentId}/{digest}
POST /api/v1/clients/{clientId}/deployments/{deploymentId}/status
```

### Bundles
```
GET  /api/v1/clients/{clientId}/bundles/{digest}
```

## Testing Scenarios

### Scenario 1: Certificate-based Authentication
```bash
curl --cacert ca-cert.pem \
     -H "X-Payload-Signature: test-sig" \
     https://server-ip:8443/v1alpha2/margo/api/v1/onboarding/certificate
```

### Scenario 2: Device Registration
```bash
curl --cacert ca-cert.pem -X POST \
     -H "Content-Type: application/json" \
     -d '{"apiVersion":"v1","kind":"DeviceCapabilitiesManifest",...}' \
     https://server-ip:8443/v1alpha2/margo/api/v1/clients/device-1/capabilities
```

### Scenario 3: Deployment Status Report
```bash
curl --cacert ca-cert.pem -X POST \
     -H "Content-Type: application/json" \
     -d '{"apiVersion":"v1","kind":"DeploymentStatusManifest",...}' \
     https://server-ip:8443/v1alpha2/margo/api/v1/clients/device-1/deployments/deploy-1/status
```

## Troubleshooting

### Issue: Certificate Verification Failed
```
curl: (60) SSL certificate problem: self signed certificate
```

**Solution:** Use the `--cacert` flag with the CA certificate path:
```bash
curl --cacert ca-cert.pem https://server-ip:8443/...
```

### Issue: Connection Refused
```
curl: (7) Failed to connect to server-ip port 8443
```

**Solutions:**
1. Check if server is running: `netstat -tlnp | grep 8443`
2. Check firewall: `sudo ufw status`
3. Open port if needed: `sudo ufw allow 8443`

### Issue: Port Already in Use
```
bind: address already in use
```

**Solution:** Kill existing process:
```bash
pkill -f "./mock-wfm"
# Wait a moment
bash start-mock-wfm.sh
```

### Issue: Certificate Not Found
```
⚠️  HTTPS certificates not found
```

**Solution:** Generate certificates:
```bash
bash generate-certs.sh . <server-ip>
go build -o mock-wfm .
bash start-mock-wfm.sh
```

## Certificate Management

### View CA Certificate Details
```bash
openssl x509 -in ca-cert.pem -noout -text
```

### View Server Certificate Details
```bash
openssl x509 -in server-cert.pem -noout -text
```

### Verify Certificate Chain
```bash
openssl verify -CAfile ca-cert.pem server-cert.pem
```

### Regenerate Certificates with Different Host
```bash
# For new server IP or hostname
bash generate-certs.sh . 10.0.0.50
go build -o mock-wfm .
bash start-mock-wfm.sh
```

## Firewall Configuration

If running on different networks:

```bash
# Allow port 9090 for HTTPS
sudo ufw allow 9090/tcp

# For HTTP testing (optional)
sudo ufw allow 8090/tcp

# Check status
sudo ufw status
```

## Performance Testing

### Load Testing with Multiple Connections
```bash
# Install Apache Bench
sudo apt-get install apache2-utils

# Test HTTPS endpoint
ab -n 1000 -c 10 \
   -E ca-cert.pem \
   https://server-ip:9090/v1alpha2/margo/api/v1/clients/test/deployments
```

## Integration Validation Checklist

- [ ] Mock WFM server running on port 8443 with HTTPS
- [ ] TLS certificates generated and verified
- [ ] CA certificate copied to device-agent VM
- [ ] device-agent config.yaml updated with correct server URL
- [ ] device-agent config.yaml has caKeyRef pointing to ca-cert.pem
- [ ] Integration tests passing (10/10 or more)
- [ ] Device-agent can successfully connect and authenticate
- [ ] Deployment status updates received by mock server

## Files Generated

After setup, you should have:

```
/home/margo/nitin/sandbox/standard/cmd/mock-wfm/
├── mock-wfm (binary)
├── ca-cert.pem (copy to device-agent VM)
├── ca-key.pem (keep secure)
├── server-cert.pem
├── server-key.pem
├── generate-certs.sh
├── start-mock-wfm.sh
├── device-agent-integration-test.sh
└── DEVICE_AGENT_INTEGRATION.md (this file)
```

## Additional Resources

- Device-agent config: `~/sandbox/poc/device/agent/config/config.yaml`
- Mock server code: `~/sandbox/standard/cmd/mock-wfm/`
- Integration tests: `device-agent-integration-test.sh`
- Test results: Check console output or logs

## Support

For issues or questions:
1. Check troubleshooting section above
2. Review test output from `device-agent-integration-test.sh`
3. Check server logs: Watch output of `start-mock-wfm.sh`
4. Verify certificates: Use openssl commands in "Certificate Management" section
