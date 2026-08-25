# Label Selector Engine

The `label` package implements a label-based device matching engine that evaluates `Selector` expressions against a device's reported label metadata from its `DeviceCapabilitiesManifest`.

## Overview

Labels are key/value pairs attached to a `DeviceCapabilitiesManifest`. Values may be one of:

| Type | Example |
|------|---------|
| `string` | `"arm64"` |
| `float32` (number) | `3.14` |
| `bool` | `true` |
| `[]string` | `["wifi", "ethernet"]` |
| `[]float32` | `[1.0, 2.5]` |

The engine evaluates a `Selector` (a list of `MatchExpression`s with **AND semantics**) against these labels.

---

## Installation

```go
import "github.com/margo/sandbox/shared-lib/constraints/engine/label"
```

---

## Exported Functions

### `New`

```go
func New(labels map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties) common.LabelSelectorEngineIface
```

Creates a new `LabelSelectorEngineIface` bound to the provided device labels map.

**Parameters:**
- `labels` — the `Labels` field from a `DeviceCapabilitiesManifest`

**Returns:** A `LabelSelectorEngineIface` ready to evaluate selectors.

**Example:**
```go
engine := label.New(*device.Labels)
matched, reason := engine.Evaluate(&selector)
```

---

## Interface Methods

All methods below are defined by `LabelSelectorEngineIface` (which aliases `SelectorEngineIface`).

### `Evaluate`

```go
Evaluate(s *clModels.Selector) (bool, string)
```

Evaluates all `MatchExpression`s in the selector with **AND semantics** — all expressions must pass. Returns `(true, "")` on success, or `(false, reason)` on the first failure.

**Example:**
```go
selector := clModels.Selector{
    MatchExpressions: []clModels.MatchExpression{
        {Key: "region", Operator: clModels.In, Values: &[]interface{}{"us-east", "eu-west"}},
        {Key: "gpu", Operator: clModels.Exists},
    },
}
ok, reason := engine.Evaluate(&selector)
if !ok {
    fmt.Println("Device ineligible:", reason)
}
```

---

### `HandleIn`

```go
HandleIn(me *clModels.MatchExpression) (bool, string)
```

Checks that the label value is present in `me.Values`.

- For scalar types (`string`, `float32`, `bool`): the label value must match at least one entry in `Values`.
- For array types (`[]string`, `[]float32`): **every** entry in `Values` must be present in the label's array.

**Requires:** `me.Values` must not be `nil`.

**Example:**
```go
me := clModels.MatchExpression{
    Key:      "arch",
    Operator: clModels.In,
    Values:   &[]interface{}{"amd64", "arm64"},
}
ok, reason := engine.HandleIn(&me)
```

---

### `HandleNotIn`

```go
HandleNotIn(me *clModels.MatchExpression) (bool, string)
```

Checks that the label value is **not** present in `me.Values`.

- For scalar types: the label value must not match any entry in `Values`.
- For array types: **none** of the entries in `Values` may appear in the label's array.

**Requires:** `me.Values` must not be `nil`.

**Example:**
```go
me := clModels.MatchExpression{
    Key:      "env",
    Operator: clModels.NotIn,
    Values:   &[]interface{}{"production"},
}
ok, reason := engine.HandleNotIn(&me)
```

---

### `HandleExists`

```go
HandleExists(me *clModels.MatchExpression) (bool, string)
```

Returns `(true, "")` if the label key is present in the device's labels, regardless of value.

**Example:**
```go
me := clModels.MatchExpression{
    Key:      "gpu",
    Operator: clModels.Exists,
}
ok, reason := engine.HandleExists(&me)
```

---

### `HandleDoesNotExists`

```go
HandleDoesNotExists(me *clModels.MatchExpression) (bool, string)
```

Returns `(true, "")` if the label key is **absent** from the device's labels.

**Example:**
```go
me := clModels.MatchExpression{
    Key:      "deprecated-flag",
    Operator: clModels.DoesNotExist,
}
ok, reason := engine.HandleDoesNotExists(&me)
```

---

### `HandleGt`

```go
HandleGt(me *clModels.MatchExpression) (bool, string)
```

Returns `(true, "")` if the label's numeric value is **greater than** the integer threshold in `me.Values[0]`.

**Constraints:**
- Label value must be a `float32` (number).
- `me.Values[0]` must be a whole-number `float64` (i.e., an integer).

**Example:**
```go
me := clModels.MatchExpression{
    Key:      "cpu-cores",
    Operator: clModels.Gt,
    Values:   &[]interface{}{float64(4)},
}
ok, reason := engine.HandleGt(&me)
```

---

### `HandleLt`

```go
HandleLt(me *clModels.MatchExpression) (bool, string)
```

Returns `(true, "")` if the label's numeric value is **less than** the integer threshold in `me.Values[0]`.

**Constraints:**
- Label value must be a `float32` (number).
- `me.Values[0]` must be a whole-number `float64` (i.e., an integer).

**Example:**
```go
me := clModels.MatchExpression{
    Key:      "latency-ms",
    Operator: clModels.Lt,
    Values:   &[]interface{}{float64(100)},
}
ok, reason := engine.HandleLt(&me)
```

---

## Supported Operators Summary

| Operator | Label Types Supported | `Values` Required |
|---|---|---|
| `In` | `string`, `float32`, `bool`, `[]string`, `[]float32` | Yes |
| `NotIn` | `string`, `float32`, `bool`, `[]string`, `[]float32` | Yes |
| `Exists` | Any | No |
| `DoesNotExist` | Any | No |
| `Gt` | `float32` (number) | Yes — integer |
| `Lt` | `float32` (number) | Yes — integer |

> **Note:** `ContainsAll` and `ContainsAny` operators are **not** supported by the label engine. They are handled by the `PropertySelectorEngineIface` implementation.

---

## Error Handling

All methods return a `(bool, string)` tuple. When the result is `false`, the string contains a human-readable reason. An empty reason string always accompanies a `true` result.

```go
ok, reason := engine.Evaluate(&selector)
if !ok {
    log.Printf("selector failed: %s", reason)
}
```
