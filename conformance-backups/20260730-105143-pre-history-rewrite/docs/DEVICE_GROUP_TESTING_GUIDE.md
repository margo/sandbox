# Device-Supplier Group-Based Testing Guide

## Quick Start Commands

### Step 1: Navigate to Conformance Directory

```bash
cd /home/margo/conformance-persona-test/conformance
```

### Step 2: Run the Test Runner CLI

```bash
./run-tests.sh
```

**Expected Output:**
```
╔═══════════════════════════════════════════════════════════════════════════╗
║           Margo Conformance Test Runner                                   ║
║              Execute conformance tests and generate reports                ║
╚═══════════════════════════════════════════════════════════════════════════╝
```

### Step 3: Select Device Supplier (Interactive Menu)

When prompted:
```
Which persona would you like to test?
1. WFM Supplier
2. Device Supplier

Select option (1-2 or Q): 
```

**Enter:** `2` (for Device Supplier)

---

## Device Supplier Test Scenarios Selection

After selecting Device Supplier, you'll see:

```
Which test scenarios would you like to run?
1. Custom test scenarios (~/sandbox/conformance/device-supplier/device-scenarios/test-scenarios.json)
2. User generated test scenarios (~/sandbox/conformance/Data-Generator/device-supplier/test-scenarios.json)
3. Group-based test scenarios (select from available groups)

Select option (1-3 or Q): 
```

### Option 1: Custom Test Scenarios
**Enter:** `1`
- Runs the template/reference test scenarios built into device-supplier
- Good for testing baseline functionality

### Option 2: User-Generated Scenarios
**Enter:** `2`
- Runs scenarios created during CLI-1 (conformance.sh)
- Uses dynamically generated test cases

### Option 3: Group-Based Test Scenarios (NEW!)
**Enter:** `3`
- Displays available device test groups:

```
📋 Available Device Test Groups:
================================================
  1) diamonddevice   (v1.0.0) - 102 tests
  2) silver          (v1.0.0) - 102 tests

Select group (1-2): 
```

**Select a group:**
- Enter `1` for **diamonddevice** group
- Enter `2` for **silver** group

---

## Complete Workflow Example

### Example 1: Run diamonddevice Group Tests

```bash
# Step 1: Navigate and start runner
cd /home/margo/conformance-persona-test/conformance
./run-tests.sh

# Step 2: Interactive prompts (answers below)
# Prompt 1: Select persona
2                    # Device Supplier

# Prompt 2: Select test scenarios
3                    # Group-based test scenarios

# Prompt 3: Select group
1                    # diamonddevice (v1.0.0) - 102 tests
```

**Expected Output:**
```
ℹ️  You selected: Device Supplier
ℹ️  Selected device group: diamonddevice
🚀 Starting Device Supplier Test Execution
   (Mock server + test runner orchestration)
📋 Test Scenarios: test-scenarios.json
📦 Building mock WFM server...
[...]
✅ Mock WFM Server started on https://localhost:3001
✅ Device test runner is ready
[Running 102 test scenarios from diamonddevice group...]
```

### Example 2: Run silver Group Tests

```bash
cd /home/margo/conformance-persona-test/conformance
./run-tests.sh

# Then follow the interactive prompts:
2                    # Device Supplier
3                    # Group-based test scenarios
2                    # silver (v1.0.0) - 102 tests
```

---

## Available Device Groups

| Group Name | Version | Test Count | Description |
|-----------|---------|-----------|-------------|
| **diamonddevice** | 1.0.0 | 102 | Diamond device specification tests |
| **silver** | 1.0.0 | 102 | Silver device specification tests |

Each group includes:
- `group.json` - Group metadata and test case references
- `test-scenarios.json` - Custom test scenario definitions
- `postman_collection.json` - API request collection (for reference)

---

## Using Conformance-2 (with RFC 9421 Signing Proxy)

The same group-based testing is also available in `conformance-2` for RFC 9421 compliance testing:

```bash
# Navigate to conformance-2
cd /home/margo/conformance-persona-test/conformance-2

# Run tests with signing proxy
./run-tests.sh

# Follow same menu flow:
2                    # Device Supplier
3                    # Group-based test scenarios
1                    # Select diamonddevice or silver
```

**Note:** `conformance-2` includes an additional signing proxy on port 18082 for RFC 9421 HTTP message signing.

---

## Command-Line Arguments (Optional)

If you prefer non-interactive mode:

```bash
# Run with explicit device supplier option
./run-tests.sh device

# Run with custom file (non-group)
./run-tests.sh device /path/to/test-scenarios.json
```

---

## Test Execution Flow

When you run group-based tests:

1. **Mock Server Setup**
   - Compiles `cmd/device-supplier` Go binary
   - Starts mock WFM server on `https://localhost:3001`

2. **Test Runner Build**
   - Compiles `run_tests.go`
   - Loads test scenarios from selected group

3. **Test Execution**
   - Runs 102 test scenarios sequentially
   - Each scenario has multiple steps (validations, state checks)
   - Real device-agent tested against mocked WFM

4. **Results Generation**
   - HTML test report generated
   - Results saved to `conformance/Runner/device-supplier/`
   - JSON results for programmatic analysis

---

## Output Files

After running tests, check results in:

```
conformance/Runner/device-supplier/
├── device-test-results-<timestamp>.json    # Detailed results
├── device-conformance-report-<timestamp>.html  # Visual report
└── conformance-test-run.log                # Execution log
```

Example view:
```bash
ls -lah conformance/Runner/device-supplier/
# Shows latest test report files
```

---

## Troubleshooting

### Issue: No groups found
**Solution:** Make sure device groups exist:
```bash
ls -la conformance/Data-Generator/device-supplier/groups/
# Should show: diamonddevice/ and silver/
```

### Issue: Go not installed
**Solution:** Install Go 1.16+:
```bash
# On Ubuntu/Debian
sudo apt-get install golang-go

# Or download from https://golang.org/doc/install
```

### Issue: Port 3001 already in use
**Solution:** Check and kill existing server:
```bash
lsof -i :3001
# Kill the process using that port
```

### Issue: Permission denied
**Solution:** Make run-tests.sh executable:
```bash
chmod +x conformance/run-tests.sh
chmod +x conformance-2/run-tests.sh
```

---

## Key Features of Group-Based Testing

✅ **Curated Test Subsets** - Run specific test groups instead of full 200+ test set
✅ **Version Tracking** - Each group has version metadata
✅ **Easy Selection** - Interactive menu with group descriptions
✅ **Custom Scenarios** - Each group has its own test-scenarios.json
✅ **Parallel Support** - Can run multiple groups in sequence
✅ **Clear Reporting** - Visual HTML reports with pass/fail counts

---

## Next Steps

After running group tests:

1. **Review Results**
   - Open HTML report in browser
   - Check JSON results for detailed pass/fail info

2. **Create New Groups**
   - Use `conformance.sh` CLI-1 to generate new groups
   - Or manually add groups to `Data-Generator/device-supplier/groups/`

3. **Compare Groups**
   - Run different groups to compare test coverage
   - Use results to understand specification differences

