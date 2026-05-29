# Margo Conformance Testing - Quick Start Guide

**Get started with conformance testing in 5 minutes!**

---

## Prerequisites

Ensure you have:
- Bash shell access to `/home/margo/nitin/sandbox/conformance`
- A running Margo WFM server (or device to test)
- Internet connection for downloading OpenAPI specs (WFM Supplier tests only)

---

## Installation

```bash
cd /home/margo/nitin/sandbox/conformance
chmod +x conformance.sh run-tests.sh
```

---

## Quick Commands

### ✅ Option 1: Interactive Menu (Recommended for First Time)

#### Step 1: Generate Test Cases

```bash
cd /home/margo/nitin/sandbox/conformance
bash conformance.sh
```

**Menu prompts:**
- Select persona: `1` for WFM Supplier or `2` for Device Supplier
- Follow the on-screen instructions

**Output location:** `Data-Generator/wfm-supplier/` or `Data-Generator/device-supplier/`

#### Step 2: Execute Tests

```bash
bash run-tests.sh
```

**Menu prompts:**
- Select the same persona
- Provide component information (name, version, IP/hostname, port)
- Tests run automatically

**Output location:** `Runner/wfm-supplier/` or `Runner/device-supplier/`

---

### ⚡ Option 2: Direct Commands (Fast)

#### WFM Supplier - Full Workflow

```bash
cd /home/margo/nitin/sandbox/conformance

# Step 1: Generate tests from OpenAPI spec
bash conformance.sh wfm openapi "https://symphony.machine:8082/v1alpha2/margo/api/v1/swagger.json"

# Step 2: Run tests against WFM
bash run-tests.sh wfm
```

#### Device Supplier - Full Workflow

```bash
cd /home/margo/nitin/sandbox/conformance

# Step 1: Generate tests from test scenarios
bash conformance.sh device "./device-supplier/device-scenarios/test-scenarios.json"

# Step 2: Run tests
bash run-tests.sh device
```

---

## One-Line Execution

### WFM Supplier (Generate + Execute)
```bash
cd /home/margo/nitin/sandbox/conformance && \
bash conformance.sh wfm openapi "https://symphony.machine:8082/v1alpha2/margo/api/v1/swagger.json" && \
bash run-tests.sh wfm
```

### Device Supplier (Generate + Execute)
```bash
cd /home/margo/nitin/sandbox/conformance && \
bash conformance.sh device "./device-supplier/device-scenarios/test-scenarios.json" && \
bash run-tests.sh device
```

---

## View Test Results

### WFM Supplier Report
```bash
# Open HTML report in browser
open Runner/wfm-supplier/report_*.html
# Or view on Linux
xdg-open Runner/wfm-supplier/report_*.html
```

### Device Supplier Report
```bash
open Runner/device-supplier/conformance-report-*.html
```

---

## Advanced: Custom Collections (User-Provided)

If you have your own Postman collection:

```bash
# Generate WFM tests from custom collection (no patching)
bash conformance.sh wfm functional "/path/to/your/collection.json"

# Run with collection patching disabled
cd wfm-supplier
PATCH_COLLECTION=false bash 2-run_newman.sh "https://your-wfm-server:port/v1alpha2/tenant"
```

---

## Troubleshooting

### "Missing required files" Error
**Solution:** Run `conformance.sh` first to generate test data

```bash
bash conformance.sh wfm openapi "your-spec-url"
```

### "WFM Server not reachable"
**Solution:** Verify your WFM server is running and accessible

```bash
curl -k https://symphony.machine:8082/v1alpha2/margo/health
```

### "Newman not found"
**Solution:** Newman will auto-install, or manually install:

```bash
sudo npm install -g newman newman-reporter-htmlextra
```

### Help & Documentation
```bash
bash conformance.sh help
bash run-tests.sh help
```

---

## File Organization

```
conformance/
├── conformance.sh          ← CLI #1: Generate tests
├── run-tests.sh            ← CLI #2: Execute tests
├── Data-Generator/         ← Generated test data
│   ├── wfm-supplier/
│   │   ├── postman_collection.json
│   │   └── newman-data/
│   └── device-supplier/
│       └── test-scenarios.json
└── Runner/                 ← Test execution reports
    ├── wfm-supplier/
    │   └── report_*.html
    └── device-supplier/
        └── conformance-report-*.html
```

---

## Next Steps

- 📖 Read [summary.md](./summary.md) for detailed architecture and workflows
- 🔧 Configure [env-setup.md](./env-setup.md) for multi-VM deployments
- 📚 See [setup-guide.md](./setup-guide.md) for full environment setup

---

**Need help?** All tests are logged. Check output files in `Data-Generator/` and `Runner/` directories.
