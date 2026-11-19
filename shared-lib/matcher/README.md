# Workload Target Profile Matcher

The `matcher` package implements the schema-agnostic Workload Target Profile and Characteristics matching engine for the Margo Workload Fleet Manager (WFM) in compliance with the Margo Specification Update Proposal (SUP: *Replace device capabilities with schema-agnostic device profile and characteristics*) and Margo Management Interface specification (`profiles.md`).

---

## Features

- **Schema-Agnostic Matching:** Evaluates compatibility without hardcoding hardware or deployment technology schemas into the WFM.
- **Mandatory Deployment Technology Checking:** Verifies support for the application's deployment technology key (e.g., `margo.org/deployment/helm`, `margo.org/deployment/compose`).
- **RFC 6901 JSON Pointer Traversal:** Uses standard JSON Pointers relative to the characteristic's opaque `properties` object.
- **Full Operator Support:** Implements all standard Margo matching operators:
  - `Exists` / `DoesNotExist` (with or without `property` pointer)
  - `In` / `NotIn` (scalar strings, numbers, booleans, and scalar arrays)
  - `Gt` / `GtEq` / `Lt` / `LtEq` (numeric comparisons)
  - `ContainsAll` / `ContainsAny` (array of objects evaluation with `ItemSelector`)
- **Group Logic:** Evaluates multiple `Characteristics` blocks with OR semantics across groups and AND semantics within each group's `matchExpressions`.

---

## Installation

```go
import "github.com/margo/sandbox/shared-lib/matcher"
```

---

## Quick Start

```go
package main

import (
    "fmt"
    "log"

    "github.com/margo/sandbox/shared-lib/matcher"
    clModels "github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
)

func main() {
    engine := matcher.New()

    // 1. Target device profile reported by device client
    profile := clModels.Profile{
        Characteristics: []clModels.Characteristic{
            {
                Key: "margo.org/deployment/helm",
                Properties: map[string]any{},
            },
            {
                Key: "margo.org/resource/cpu",
                Properties: map[string]any{
                    "totalCores": 16.0,
                    "architecture": "amd64",
                },
            },
        },
    }

    // 2. Application requirements
    deploymentKey := "margo.org/deployment/helm"
    prop := "/totalCores"
    characteristics := []clModels.Characteristics{
        {
            MatchExpressions: []clModels.MatchExpression{
                {
                    Key:      "margo.org/resource/cpu",
                    Property: &prop,
                    Operator: clModels.MatchExpressionOperatorGtEq,
                    Values:   &[]any{8.0},
                },
            },
        },
    }

    // 3. Evaluate eligibility
    eligible, reason, err := engine.IsTargetEligible(&profile, deploymentKey, characteristics)
    if err != nil {
        log.Fatalf("Fatal evaluation error: %v", err)
    }

    if eligible {
        fmt.Println("Target is eligible!")
    } else {
        fmt.Printf("Target is not eligible: %s\n", reason)
    }
}
```

---

## API Reference

### `New() MatcherIface`

Constructs a new matcher engine instance.

```go
engine := matcher.New()
```

### `MatcherIface`

```go
type MatcherIface interface {
    IsTargetEligible(
        profile *clModels.Profile,
        deploymentKey string,
        characteristics []clModels.Characteristics,
    ) (bool, string, error)
}
```

---

## Matching Operators

| Operator | Target Type | `property` Pointer | `values` | `itemSelector` | Description |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `Exists` | Any | Optional | Omitted | Omitted | True if key exists (when `property` omitted), or if property path exists in `properties`. |
| `DoesNotExist` | Any | Optional | Omitted | Omitted | True if key is absent (when `property` omitted), or if property path is absent in `properties`. |
| `In` | Scalar / Scalar Array | Required | 1+ entries | Omitted | True if scalar value equals any candidate, or if scalar array contains any candidate. |
| `NotIn` | Scalar / Scalar Array | Required | 1+ entries | Omitted | True if target exists and scalar does not equal any candidate, or no array element matches. |
| `Gt` | Number | Required | Exactly 1 number | Omitted | True if target value is strictly greater than `values[0]`. |
| `GtEq` | Number | Required | Exactly 1 number | Omitted | True if target value is greater than or equal to `values[0]`. |
| `Lt` | Number | Required | Exactly 1 number | Omitted | True if target value is strictly less than `values[0]`. |
| `LtEq` | Number | Required | Exactly 1 number | Omitted | True if target value is less than or equal to `values[0]`. |
| `ContainsAll` | Array of Objects | Required | Omitted | Required | True if AT LEAST ONE array object satisfies ALL `itemSelector.matchExpressions`. |
| `ContainsAny` | Array of Objects | Required | Omitted | Required | True if AT LEAST ONE array object satisfies ANY `itemSelector.matchExpressions`. |


