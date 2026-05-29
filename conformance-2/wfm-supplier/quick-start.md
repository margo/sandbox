# WFM Supplier Conformance - Quick Start

Quick reference for running WFM conformance tests.

## Prerequisites

Before starting, ensure:
1. **WFM Server is running** at `https://<host>:<port>/v1alpha2/margo`
2. **CA Certificate is copied** from WFM server to `./certs/ca-cert.pem`

```bash
# Copy WFM CA certificate (run this on WFM server machine)
cp /margo/home/symphony/api/certificates/ca-cert.pem \
   /home/margo/nitin/sandbox/conformance/wfm-supplier/certs/ca-cert.pem
```

## One-Command Execution (Recommended)

Run setup and tests in one go:

```bash
cd /home/margo/nitin/sandbox/conformance/wfm-supplier

# With default patching (for Portman-generated collections)
./1-setup_portman.sh "https://symphony.machine:8082/v1alpha2/margo"
./2-run_newman.sh "https://symphony.machine:8082/v1alpha2/margo"

# With patching disabled (for user-provided collections)
PATCH_COLLECTION=false ./2-run_newman.sh "https://symphony.machine:8082/v1alpha2/margo"
```

## Interactive CLI (For Non-Technical Users)

Use the guided menu-driven approach:

```bash
cd /home/margo/nitin/sandbox/conformance

# Start the interactive CLI
./conformance_cli.sh

# Select: WFM Supplier
# Select: WFM Supplier menu option
# Follow the on-screen prompts
```

## Step-by-Step Manual Execution

### Step 1: Setup (One-Time or Fresh Start)

Generate collection and environment:

```bash
cd /home/margo/nitin/sandbox/conformance/wfm-supplier

bash 1-setup_portman.sh "https://symphony.machine:8082/v1alpha2/margo"
```

**Output files created:**
- `postman_collection.json` — API test cases
- `newman-data/device-agent.env.json` — Test variables and payloads
- `newman-data/certs/device.key` — Mock device private key
- `newman-data/certs/device-cert.pem` — Mock device certificate

### Step 2: Run Tests (Each Execution)

Execute tests against WFM:

```bash
bash 2-run_newman.sh "https://symphony.machine:8082/v1alpha2/margo"
```

**Output:**
- Console summary with assertion results
- `report_YYYYMMDD_HHMMSS.html` — Detailed HTML report

### Step 3: View Results

```bash
# List reports (newest first)
ls -lrt report_*.html | tail -5

# Open in browser
open report_YYYYMMDD_HHMMSS.html  # macOS
xdg-open report_YYYYMMDD_HHMMSS.html  # Linux
```

## Common Commands

### Run with Different WFM URLs

```bash
# Local development WFM
./1-setup_portman.sh "https://localhost:3001/v1alpha2/margo"
./2-run_newman.sh "https://localhost:3001/v1alpha2/margo"

# Production WFM
./1-setup_portman.sh "https://wfm.example.com:8082/v1alpha2/margo"
./2-run_newman.sh "https://wfm.example.com:8082/v1alpha2/margo"
```

### Skip Setup (Reuse Collection)

If you already ran setup, just run tests multiple times:

```bash
./2-run_newman.sh "https://symphony.machine:8082/v1alpha2/margo"
./2-run_newman.sh "https://symphony.machine:8082/v1alpha2/margo"  # Runs again
```

### Use Custom Postman Collection

If you have your own collection that doesn't need patching:

```bash
PATCH_COLLECTION=false ./2-run_newman.sh "https://symphony.machine:8082/v1alpha2/margo"
```

### Reset for Fresh Onboarding

Stop WFM and get new CA certificate:

```bash
# 1. Stop WFM server (wherever it's running)
# 2. Copy fresh CA certificate
cp /margo/home/symphony/api/certificates/ca-cert.pem \
   ./certs/ca-cert.pem

# 3. Run fresh conformance
./1-setup_portman.sh "https://symphony.machine:8082/v1alpha2/margo"
./2-run_newman.sh "https://symphony.machine:8082/v1alpha2/margo"
```

## Troubleshooting

### Missing CA Certificate
```
❌ Missing WFM CA certificate
   Copy to: ./certs/ca-cert.pem
```
**Solution:** Copy CA certificate from WFM server (see Prerequisites above)

### Missing Setup Files
```
❌ Missing postman_collection.json. Run './1-setup_portman.sh' first.
```
**Solution:** Run `./1-setup_portman.sh` first before `./2-run_newman.sh`

### Tests Fail with 401/400 Errors

This is **normal behavior** for some endpoints without RFC 9421 signatures.

The test script uses flexible assertions that accept these errors gracefully. Check the HTML report for details.

### npm/newman Not Found
```
❌ Missing required command: newman
   How to install: npm install -g newman
```
**Solution:** Scripts auto-install, but if manual install needed:
```bash
sudo npm install -g newman newman-reporter-htmlextra
```

## What Gets Tested?

The conformance suite tests 8 API endpoints:

1. ✅ **Get Root CA Certificate** — Download WFM's CA certificate
2. ✅ **Onboard Device** — Register device, get clientId
3. ✅ **Report Capabilities** — Send device hardware/features
4. ✅ **Update Capabilities** — Simulate feature upgrade
5. ✅ **Get Deployments** — Retrieve assigned workloads
6. ✅ **Get Deployment YAML** — Retrieve workload details
7. ✅ **Report Deployment Status** — Send execution status back
8. ✅ **Get Bundles** — Retrieve workload bundles

**Expected Results:**
- 8 requests executed
- 11+ assertions passed
- 0 failures
- ~300ms total execution time

## Next Steps

- **View the full documentation:** Read [summary.md](summary.md) for detailed explanation
- **Explore the scripts:** Review [1-setup_portman.sh](1-setup_portman.sh) and [2-run_newman.sh](2-run_newman.sh)
- **Use the CLI:** Run `./conformance_cli.sh` from conformance root for guided experience
- **Check reports:** Open HTML report for detailed test results and API responses

## Support

For issues or questions:
1. Check HTML report for test details
2. Review [summary.md](summary.md) for architecture explanation
3. Examine script output for error messages
