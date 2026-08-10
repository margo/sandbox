# Quick Reference: Group-Based Testing

## Overview

Both `conformance` and `conformance-2` now support group-based testing for Device Supplier persona. Groups allow you to organize and execute related test scenarios together.

---

## Available Groups

### diamonddevice
- **Version**: 1.0.0
- **Tests**: 102 scenarios
- **Location**: `Data-Generator/device-supplier/groups/diamonddevice/`

### silver
- **Version**: 1.0.0
- **Tests**: 102 scenarios
- **Location**: `Data-Generator/device-supplier/groups/silver/`

---

## Two-Step Workflow

### Step 1: Generate Tests (conformance.sh)

```bash
cd /home/margo/conformance-persona-test/conformance
./conformance.sh
```

**Interactive Menu:**
```
Which Margo Persona do you want to create test-cases for?
1. WFM Supplier
2. Device Supplier        ← Select this
```

**Device Supplier Options:**
```
1. Use existing test-scenarios.json
2. Provide your own test scenarios
3. Group-based test scenarios    ← Select this
```

**Group Management Menu:**
```
Available Groups
  1) diamonddevice
  2) silver

0 → Create new group
```

**Result:**
- Selected group's test scenarios copied to `Data-Generator/device-supplier/`
- Ready for execution with `run-tests.sh`

---

### Step 2: Run Tests (run-tests.sh)

```bash
./run-tests.sh
```

**Interactive Menu:**
```
What type of test case do you want to execute?
1. WFM Supplier
2. Device Supplier       ← Select this
```

**Device Supplier Test Options:**
```
1. Newman (Postman) Tests
2. Go-based Tests
3. Group-based scenarios      ← Select this
```

**Group Selection:**
```
Available Groups
  1) diamonddevice
  2) silver
```

**Result:**
- Tests from selected group execute
- HTML report generated
- Results displayed in terminal

---

## Quick Commands

### List all available groups:
```bash
ls -la conformance/Data-Generator/device-supplier/groups/
```

### View group metadata:
```bash
cat conformance/Data-Generator/device-supplier/groups/diamonddevice/group.json
```

### Create new group (interactive):
```bash
cd conformance
./conformance.sh
# Select 2 → 3 → 0
```

### Select existing group directly:
```bash
cd conformance
./conformance.sh
# Select 2 → 3 → 1 (for diamonddevice)
```

### Run tests from group:
```bash
cd conformance
./run-tests.sh
# Select 2 → 3 → 1 (for diamonddevice)
```

---

## Example: Full Workflow

### 1. Generate tests from silver group:
```bash
cd /home/margo/conformance-persona-test/conformance
./conformance.sh
# Input: 2 (Device Supplier)
# Input: 3 (Group-based)
# Input: 2 (silver group)
# Wait for: "Test Scenarios Summary"
```

### 2. Run the tests:
```bash
./run-tests.sh
# Input: 2 (Device Supplier)
# Input: 3 (Group-based scenarios)
# Input: 2 (silver group)
# Watch tests execute...
```

### 3. Check results:
```bash
# HTML report location will be shown
# Look for: "Test report saved to: ..."
```

---

## Troubleshooting

### Group not found?
```bash
# Verify groups exist
ls conformance/Data-Generator/device-supplier/groups/

# Should show: diamonddevice  silver
```

### No test scenarios?
```bash
# Check group content
ls conformance/Data-Generator/device-supplier/groups/diamonddevice/

# Should include: group.json  test-scenarios.json
```

### Script error?
```bash
# Verify syntax
bash -n conformance/conformance.sh
bash -n conformance/run-tests.sh

# Both should return with no output
```

### Permission denied?
```bash
chmod +x conformance/conformance.sh
chmod +x conformance/run-tests.sh
chmod +x conformance/device-supplier/run_tests.go
```

---

## File Structure Reference

```
conformance/
├── conformance.sh                    # Generator (now with group support)
├── run-tests.sh                      # Test runner (now with group support)
├── Data-Generator/
│   └── device-supplier/
│       ├── groups/                   # Group storage
│       │   ├── diamonddevice/
│       │   │   ├── group.json
│       │   │   ├── test-scenarios.json
│       │   │   └── postman_collection.json
│       │   └── silver/
│       │       ├── group.json
│       │       ├── test-scenarios.json
│       │       └── postman_collection.json
│       └── device-supplier.json      # Active group (generated on demand)
└── device-supplier/
    ├── device-scenarios/             # Default scenarios
    │   ├── test-scenarios.json
    │   └── TEMPLATE_custom_test_scenario.json
    └── run_tests.go                  # Test executor binary
```

---

## Key Features

✅ **Interactive Group Selection** - Menu-driven group picker  
✅ **Group Metadata** - Version and test count displayed  
✅ **Test Organization** - Group-related tests together  
✅ **One-Click Execution** - Select group, tests run automatically  
✅ **HTML Reports** - Auto-generated with test results  
✅ **Full Sync** - Both conformance and conformance-2 updated

---

## Additional Resources

- [Integration Verification Report](./INTEGRATION_VERIFICATION_REPORT.md)
- [Conformance Workflow Guide](./CONFORMANCE_WORKFLOW_VERIFICATION.md)
- [Device Group Testing Guide](./DEVICE_GROUP_TESTING_GUIDE.md)

---

## Notes

- **Conformance vs Conformance-2**: Choose based on your testing environment (conformance-2 includes RFC 9421 signing proxy)
- **Group Switching**: You can switch groups between runs without re-generating
- **Custom Groups**: Create new groups via menu option 0 in group selection
- **Backward Compatible**: All existing test modes still work alongside group feature

