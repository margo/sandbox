# Property Selector Engine

The `property` package implements the **property selector engine** for evaluating device capability constraints against a `DeviceCapabilitiesManifest`. It resolves RFC 6901 JSON Pointers into device properties and applies match expression operators defined in the SUP specification.

---

## Installation

```go
import (
    "github.com/margo/sandbox/shared-lib/constraints/engine/property"
    clModels "github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
)
```

---

## Creating an Engine

### `New`

```go
func New(device *clModels.DeviceCapabilitiesManifest) common.PropertySelectorEngineIface
```

Creates a new property selector engine bound to a specific device manifest.

**Example:**

```go
device := &clModels.DeviceCapabilitiesManifest{ /* ... */ }
engine := property.New(device)
```

The returned engine implements `common.PropertySelectorEngineIface`, which extends `SelectorEngineIface` with `HandleContainsAll` and `HandleContainsAny`.

---

## Evaluating Selectors

### `Evaluate`

```go
Evaluate(s *clModels.Selector) (bool, string)
```

Evaluates all `matchExpressions` in a `Selector` with **AND semantics** — all expressions must pass for the result to be `true`. Returns `(true, "")` on success, or `(false, reason)` on the first failure.

**Example:**

```go
selector := &clModels.Selector{
    MatchExpressions: []clModels.MatchExpression{
        {Key: "/vendor", Operator: clModels.In, Values: &[]any{"Acme", "Globex"}},
        {Key: "/cpus/0/cores", Operator: clModels.Gt, Values: &[]any{float64(4)}},
    },
}

ok, reason := engine.Evaluate(selector)
if !ok {
    fmt.Println("Device does not match:", reason)
}
```

---

## Match Expression Handlers

Each handler corresponds to a `MatchExpressionOperator` and can be called directly when you need to evaluate a single expression.

### `HandleIn`

```go
HandleIn(me *clModels.MatchExpression) (bool, string)
```

Returns `true` when the resolved property value equals **any** entry in `values`.

| Resolved Type        | Behaviour                                                  |
|----------------------|------------------------------------------------------------|
| `string`/`number`/`bool` | Matches if the scalar equals any candidate value       |
| `[]any` (scalars)    | Matches if at least one element equals any candidate value |
| `[]any` (objects)    | Always `false` — use `ContainsAll`/`ContainsAny` instead  |

**Constraints:**
- `values` must be non-empty, same type throughout.
- Boolean `values` must contain exactly one entry.

```go
me := &clModels.MatchExpression{
    Key:      "/vendor",
    Operator: clModels.In,
    Values:   &[]any{"Acme", "Globex"},
}
ok, reason := engine.HandleIn(me)
```

---

### `HandleNotIn`

```go
HandleNotIn(me *clModels.MatchExpression) (bool, string)
```

Returns `true` when the key **exists** and its value does **not** match any entry in `values`. The key must be present — an absent key returns `false`.

```go
me := &clModels.MatchExpression{
    Key:      "/vendor",
    Operator: clModels.NotIn,
    Values:   &[]any{"BadVendor"},
}
ok, reason := engine.HandleNotIn(me)
```

---

### `HandleExists`

```go
HandleExists(me *clModels.MatchExpression) (bool, string)
```

Returns `true` when the JSON Pointer key is **present** in device properties, regardless of its value. `values` must be omitted.

```go
me := &clModels.MatchExpression{
    Key:      "/memory",
    Operator: clModels.Exists,
}
ok, reason := engine.HandleExists(me)
```

---

### `HandleDoesNotExists`

```go
HandleDoesNotExists(me *clModels.MatchExpression) (bool, string)
```

Returns `true` when the JSON Pointer key is **absent** from device properties. `values` must be omitted. A malformed pointer surfaces as an explicit error rather than silently returning `true`.

```go
me := &clModels.MatchExpression{
    Key:      "/peripherals",
    Operator: clModels.DoesNotExist,
}
ok, reason := engine.HandleDoesNotExists(me)
```

---

### `HandleGt`

```go
HandleGt(me *clModels.MatchExpression) (bool, string)
```

Returns `true` when the resolved numeric property is **strictly greater than** `values[0]`. `values` must contain exactly one number; the resolved property must also be numeric.

```go
me := &clModels.MatchExpression{
    Key:      "/cpus/0/cores",
    Operator: clModels.Gt,
    Values:   &[]any{float64(4)},
}
ok, reason := engine.HandleGt(me)
```

---

### `HandleLt`

```go
HandleLt(me *clModels.MatchExpression) (bool, string)
```

Returns `true` when the resolved numeric property is **strictly less than** `values[0]`. Same constraints as `HandleGt`.

```go
me := &clModels.MatchExpression{
    Key:      "/cpus/0/cores",
    Operator: clModels.Lt,
    Values:   &[]any{float64(16)},
}
ok, reason := engine.HandleLt(me)
```

---

### `HandleContainsAll`

```go
HandleContainsAll(me clModels.MatchExpression) (bool, string)
```

Returns `true` when **at least one** element of an object array satisfies **all** `itemSelector.matchExpressions` (AND logic). The key must resolve to a non-empty array of objects. `values` must be omitted.

```go
me := clModels.MatchExpression{
    Key:      "/peripherals",
    Operator: clModels.ContainsAll,
    ItemSelector: &clModels.Selector{
        MatchExpressions: []clModels.MatchExpression{
            {Key: "/type", Operator: clModels.In, Values: &[]any{"camera"}},
            {Key: "/manufacturer", Operator: clModels.Exists},
        },
    },
}
ok, reason := engine.HandleContainsAll(me)
```

> **Note:** `itemSelector` keys are JSON Pointers **relative to each array element** (e.g. `"/type"`). The engine rewrites them to absolute pointers internally.

---

### `HandleContainsAny`

```go
HandleContainsAny(me clModels.MatchExpression) (bool, string)
```

Returns `true` when **at least one** element of an object array satisfies **any** `itemSelector.matchExpression` (OR logic). Same structural requirements as `HandleContainsAll`.

```go
me := clModels.MatchExpression{
    Key:      "/peripherals",
    Operator: clModels.ContainsAny,
    ItemSelector: &clModels.Selector{
        MatchExpressions: []clModels.MatchExpression{
            {Key: "/type", Operator: clModels.In, Values: &[]any{"gpu"}},
            {Key: "/type", Operator: clModels.In, Values: &[]any{"display"}},
        },
    },
}
ok, reason := engine.HandleContainsAny(me)
```

---

## Key Concepts

### JSON Pointer Keys (RFC 6901)

All `Key` fields in property selectors are `[RFC 6901](https://datatracker.ietf.org/doc/html/rfc6901)` JSON Pointers relative to `device.Properties`:

| Pointer              | Resolves to                        |
|----------------------|------------------------------------|
| `/vendor`            | `properties.vendor`                |
| `/cpus/0/cores`      | `properties.cpus[0].cores`         |
| `/peripherals/1/type`| `properties.peripherals[1].type`   |

Use `~0` to escape `~` and `~1` to escape `/` within token names.

### Operator Summary

| Operator       | `values` required | `itemSelector` required | Target type          |
|----------------|:-----------------:|:-----------------------:|----------------------|
| `In`           | ✅                | ❌                      | scalar or scalar array |
| `NotIn`        | ✅                | ❌                      | scalar or scalar array |
| `Exists`       | ❌ (omit)         | ❌                      | any                  |
| `DoesNotExist` | ❌ (omit)         | ❌                      | any                  |
| `Gt`           | ✅ (one number)   | ❌                      | number               |
| `Lt`           | ✅ (one number)   | ❌                      | number               |
| `ContainsAll`  | ❌ (omit)         | ✅                      | object array         |
| `ContainsAny`  | ❌ (omit)         | ✅                      | object array         |

### Return Values

All handlers return `(bool, string)`:
- `(true, "")` — expression satisfied.
- `(false, reason)` — expression failed; `reason` describes why.