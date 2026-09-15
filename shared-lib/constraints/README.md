
# Device Selector

The `constraints` package provides device eligibility evaluation against deployment constraints. It checks whether devices meet capacity requirements (CPU, memory, storage) and eligibility rules (label and property selectors).

## Installation

```go
import "github.com/margo/sandbox/shared-lib/constraints"
```

## Quick Start

```go
selector := constraints.New()

eligible, err := selector.SelectEligibleDevice(devices, constraints)
if err != nil {
    log.Fatal(err)
}

for _, d := range eligible {
    fmt.Println("Eligible device:", d.Properties.Id)
}
```

---

## API Reference

### `New() DeviceSelectorIface`

Creates and returns a new `DeviceSelectorIface` instance.

```go
selector := constraints.New()
```

---

### `SelectEligibleDevice`

```go
func (ds *deviceSelector) SelectEligibleDevice(
    devices []*clModels.DeviceCapabilitiesManifest,
    checks *clModels.DeviceConstraints,
) ([]*clModels.DeviceCapabilitiesManifest, error)
```

Evaluates a list of devices against the provided constraints and returns all devices that satisfy every eligibility check.

**Behavior:**
- Returns all devices that pass both capacity and eligibility rule checks.
- On a **fatal error** (e.g., malformed quantity string), evaluation halts immediately and returns `nil, error`. No partial results are returned.
- Ineligible devices are silently skipped; only fatal errors stop evaluation.
- If `checks` is `nil`, all devices are returned as eligible.

**Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `devices` | `[]*DeviceCapabilitiesManifest` | List of devices to evaluate |
| `checks` | `*DeviceConstraints` | Constraints to evaluate against |

**Returns:**

| Return | Description |
|--------|-------------|
| `[]*DeviceCapabilitiesManifest` | Devices satisfying all constraints. Empty (not nil) when none qualify. `nil` on error. |
| `error` | Non-nil only on fatal evaluation failure |

**Example:**

```go
selector := constraints.New()

cores := float32(2.0)
mem := "4Gi"
arch := []clModels.DeploymentCpuRequirementArchitectures{
    clModels.DeploymentCpuRequirementArchitecturesAmd64,
}

checks := &clModels.DeviceConstraints{
    CapacityRequirements: &clModels.CapacityRequirements{
        Cpu: &clModels.DeploymentCpuRequirement{
            Cores:         cores,
            Architectures: &arch,
        },
        Memory: &mem,
    },
}

eligible, err := selector.SelectEligibleDevice(devices, checks)
if err != nil {
    log.Fatalf("evaluation error: %v", err)
}
fmt.Printf("%d eligible device(s) found\n", len(eligible))
```

---

### `IsDeviceEligible`

```go
func (ds *deviceSelector) IsDeviceEligible(
    device *clModels.DeviceCapabilitiesManifest,
    checks *clModels.DeviceConstraints,
) (bool, string, error)
```

Evaluates a **single device** against the provided constraints.

**Behavior:**
- Checks are applied in order: **capacity → eligibility rules**.
- Capacity check (CPU cores/architecture, memory, storage) is always evaluated first.
- Eligibility rules (`EligibilityRules`) use **OR** semantics across rules — at least one rule must pass.
- Within each rule, label selector and property selector use **AND** semantics — both must pass if present.
- If `checks` is `nil`, the device is immediately considered eligible.
- If `checks.EligibilityRules` is `nil`, only capacity is checked.

**Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `device` | `*DeviceCapabilitiesManifest` | The device to evaluate |
| `checks` | `*DeviceConstraints` | Constraints to evaluate against |

**Returns:**

| Return | Type | Description |
|--------|------|-------------|
| `eligible` | `bool` | `true` if device satisfies all constraints |
| `reason` | `string` | Human-readable explanation of the first failed check. Empty string when eligible or on error. |
| `error` | `error` | Non-nil only on fatal evaluation failure. When non-nil, `eligible` is `false` and `reason` is `""`. |

**Example:**

```go
selector := constraints.New()

storage := "100Gi"
checks := &clModels.DeviceConstraints{
    CapacityRequirements: &clModels.CapacityRequirements{
        Storage: &storage,
    },
    EligibilityRules: &[]clModels.EligibilityRule{
        {
            LabelSelector: &clModels.Selector{
                MatchExpressions: []clModels.MatchExpression{
                    {
                        Key:      "region",
                        Operator: clModels.In,
                        Values:   &[]interface{}{"us-east", "eu-west"},
                    },
                },
            },
        },
    },
}

ok, reason, err := selector.IsDeviceEligible(device, checks)
if err != nil {
    log.Fatalf("fatal error: %v", err)
}
if !ok {
    fmt.Println("Device ineligible:", reason)
}
```

---

## Constraint Model Reference

### `DeviceConstraints`

```go
type DeviceConstraints struct {
    CapacityRequirements *CapacityRequirements `json:"capacityRequirements,omitempty"`
    EligibilityRules     *[]EligibilityRule    `json:"eligibilityRules,omitempty"`
}
```

### `CapacityRequirements`

```go
type CapacityRequirements struct {
    Cpu     *DeploymentCpuRequirement `json:"cpu,omitempty"`
    Memory  *string                   `json:"memory,omitempty"`  // e.g. "4Gi"
    Storage *string                   `json:"storage,omitempty"` // e.g. "100Gi"
}
```

Memory and storage values use binary unit suffixes: `Ki`, `Mi`, `Gi`, `Ti`, `Pi`, `Ei`.

### `EligibilityRule`

```go
type EligibilityRule struct {
    LabelSelector    *Selector `json:"labelSelector,omitempty"`
    PropertySelector *Selector `json:"propertySelector,omitempty"`
}
```

### `Selector` and `MatchExpression`

```go
type Selector struct {
    MatchExpressions []MatchExpression `json:"matchExpressions"`
}

type MatchExpression struct {
    Key          string                  `json:"key"`
    Operator     MatchExpressionOperator `json:"operator"`
    Values       *[]interface{}          `json:"values,omitempty"`
    ItemSelector *Selector               `json:"itemSelector,omitempty"`
}
```

**Supported operators:**

| Operator | Applies To | Description |
|----------|-----------|-------------|
| `In` | Labels, Properties | Value equals any entry in `values` |
| `NotIn` | Labels, Properties | Value does not equal any entry in `values` |
| `Exists` | Labels, Properties | Key is present |
| `DoesNotExist` | Labels, Properties | Key is absent |
| `Gt` | Labels (number), Properties (number) | Value is greater than `values[0]` |
| `Lt` | Labels (number), Properties (number) | Value is less than `values[0]` |
| `ContainsAll` | Properties (array of objects) | At least one array element satisfies **all** `itemSelector` expressions |
| `ContainsAny` | Properties (array of objects) | At least one array element satisfies **any** `itemSelector` expression |

> **Note:** For `PropertySelector`, `key` must be an `[RFC 6901 JSON Pointer](https://www.rfc-editor.org/rfc/rfc6901)` relative to the device's `properties` object (e.g. `"/vendor"`, `"/cpus/0/cores"`). For `LabelSelector`, `key` is the exact label key string.

---

## Evaluation Logic Summary

```
SelectEligibleDevice(devices, checks)
└── for each device → IsDeviceEligible(device, checks)
    ├── checks == nil                    → eligible ✓
    ├── CapacityRequirements
    │   ├── CPU cores (+ optional arch filter)
    │   ├── Memory (actual >= required)
    │   └── Storage (actual >= required)
    └── EligibilityRules  [OR across rules]
        └── each rule     [AND within rule]
            ├── LabelSelector    (if present)
            └── PropertySelector (if present)
```
