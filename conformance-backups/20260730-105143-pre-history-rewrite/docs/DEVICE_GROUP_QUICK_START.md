# Quick Reference: Device-Supplier Group Testing

## TL;DR - Run in 3 Commands

```bash
cd /home/margo/conformance-persona-test/conformance
./run-tests.sh
# Then select: 2 → 3 → 1 (or 2 for silver group)
```

## Complete Step-by-Step

### 1️⃣ Open Terminal and Navigate

```bash
cd /home/margo/conformance-persona-test/conformance
```

### 2️⃣ Run Test Runner

```bash
./run-tests.sh
```

### 3️⃣ Follow Interactive Menu

```
╔═══════════════════════════════════════════════════════════════════════════╗
║           Margo Conformance Test Runner                                   ║
║              Execute conformance tests and generate reports                ║
╚═══════════════════════════════════════════════════════════════════════════╝

Which persona would you like to test?
1. WFM Supplier
2. Device Supplier

Select option (1-2 or Q):
```

**👉 Enter: `2`** (Device Supplier)

---

### 4️⃣ Select Test Scenario Type

```
Which test scenarios would you like to run?
1. Custom test scenarios (~/sandbox/conformance/device-supplier/device-scenarios/test-scenarios.json)
2. User generated test scenarios (~/sandbox/conformance/Data-Generator/device-supplier/test-scenarios.json)
3. Group-based test scenarios (select from available groups)

Select option (1-3 or Q):
```

**👉 Enter: `3`** (Group-based - NEW feature!)

---

### 5️⃣ Select Device Group

```
📋 Available Device Test Groups:
================================================
  1) diamonddevice   (v1.0.0) - 102 tests
  2) silver          (v1.0.0) - 102 tests

Select group (1-2):
```

**👉 Enter: `1`** (for diamonddevice) **OR `2`** (for silver)

---

### 6️⃣ Tests Run Automatically

```
ℹ️  You selected: Device Supplier
ℹ️  Selected device group: diamonddevice
🚀 Starting Device Supplier Test Execution
📦 Building mock WFM server...
✅ Mock WFM Server started on https://localhost:3001
✅ Device test runner is ready
[Running 102 test scenarios...]
[✅ PASS: scenario-1]
[✅ PASS: scenario-2]
...
╔══════════════════════════════════════════════╗
║  Test Results: 102 PASSED, 0 FAILED (Total: 102)
╚══════════════════════════════════════════════╝
```

---

## Available Groups

| Group | Version | Tests | Run Command |
|-------|---------|-------|------------|
| **diamonddevice** | 1.0.0 | 102 | Select `1` |
| **silver** | 1.0.0 | 102 | Select `2` |

---

## View Results

After tests complete:

```bash
# Open HTML report
open conformance/Runner/device-supplier/device-conformance-report-*.html

# Or view JSON results
cat conformance/Runner/device-supplier/device-test-results-*.json | jq
```

---

## For conformance-2 (with RFC 9421 Signing)

```bash
cd /home/margo/conformance-persona-test/conformance-2
./run-tests.sh
# Same menu flow: 2 → 3 → 1 (or 2)
```

---

## Menu Cheat Sheet

### Fast Path for diamonddevice (conformance):
```bash
cd /home/margo/conformance-persona-test/conformance && ./run-tests.sh
# Type: 2
# Type: 3
# Type: 1
# ✅ Tests run automatically
```

### Fast Path for silver (conformance-2):
```bash
cd /home/margo/conformance-persona-test/conformance-2 && ./run-tests.sh
# Type: 2
# Type: 3
# Type: 2
# ✅ Tests run automatically
```

---

## What Gets Tested?

Each group runs:
- ✅ 102 test scenarios
- ✅ Custom test-scenario.json format
- ✅ Real device-agent vs mock WFM server
- ✅ Full compliance validation

---

## Troubleshooting

| Problem | Fix |
|---------|-----|
| "No device groups found" | Run: `ls conformance/Data-Generator/device-supplier/groups/` |
| Permission denied | Run: `chmod +x conformance/run-tests.sh` |
| Port 3001 in use | Run: `lsof -i :3001` and kill the process |
| Go not found | Install Go: `sudo apt-get install golang-go` |

---

## Key Points

🎯 **Group-based testing** = Curated subsets of 102 tests per group
🎯 **Interactive menu** = No need to remember complex commands
🎯 **Custom format** = Each group uses test-scenario.json (not Postman collections)
🎯 **Real device testing** = Tests real device-agent against mock WFM
🎯 **HTML reports** = Beautiful test reports auto-generated

