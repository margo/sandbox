
# capacity — Device Eligibility Checker

Package `capacity` provides hardware-based eligibility checking for devices against deployment capacity requirements. It implements the `CapacityEligibilityCheckerIface` defined in `common/types.go`.

---

## Installation

```go
import (
    "github.com/margo/sandbox/shared-lib/constraints/engine/capacity"
    clModels "github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
)
```

---

## Exported Functions

### `New`

```go
func New(i clModels.DeviceCapabilitiesManifest) common.CapacityEligibilityCheckerIface
```

Creates a new `CapacityEligibilityCheckerIface` backed by a `DeviceCapabilitiesManifest`. This is the primary entry point for the package.

**Example:**

```go
manifest := clModels.DeviceCapabilitiesManifest{
    ApiVersion: "v1",
    Kind:       "DeviceCapabilitiesManifest",
    Properties: struct {
        Cpus    *[]struct { ... } `json:"cpus,omitempty"`
        Memory  *string          `json:"memory,omitempty"`
        Storage *string          `json:"storage,omitempty"`
        // ...
    }{
        Memory:  strPtr("4Gi"),
        Storage: strPtr("100Gi"),
        Cpus: &[]struct {
            Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
            Cores        float32 `json:"cores"`
        }{
            {Cores: 4, Architecture: archPtr(clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64)},
        },
    },
}

checker := capacity.New(manifest)
```

---

## Interface Methods

All methods below are available on the value returned by `New`, via the `CapacityEligibilityCheckerIface` interface.

---

### `CheckEligibility`

```go
CheckEligibility(checks *clModels.CapacityRequirements) (bool, string, error)
```

Validates a device against all capacity requirements (CPU, memory, storage) in a single call. Returns:

| Return | Description |
|--------|-------------|
| `bool` | `true` if all requirements are met |
| `string` | Human-readable reason if a check fails (empty on success) |
| `error` | Non-nil if quantity parsing fails |

A `nil` `checks` argument always returns `(true, "", nil)`.

**Example:**

```go
cores := float32(2)
mem   := "2Gi"
arch  := clModels.DeploymentCpuRequirementArchitecturesAmd64

requirements := &clModels.CapacityRequirements{
    Cpu: &clModels.DeploymentCpuRequirement{
        Cores:         cores,
        Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{arch},
    },
    Memory:  &mem,
    Storage: strPtr("50Gi"),
}

ok, reason, err := checker.CheckEligibility(requirements)
if err != nil {
    log.Fatal(err)
}
if !ok {
    fmt.Println("Device ineligible:", reason)
}
```

---

### `HasEnoughCPUCores`

```go
HasEnoughCPUCores(arch *[]string, cores float32) bool
```

Checks whether the device has at least `cores` CPU cores. If `arch` is non-nil, only CPUs matching one of the specified architecture strings are considered. CPUs without a reported architecture are skipped when an architecture filter is active.

**Example — any architecture:**

```go
ok := checker.HasEnoughCPUCores(nil, 4)
```

**Example — architecture-filtered:**

```go
archs := []string{"amd64", "arm64"}
ok := checker.HasEnoughCPUCores(&archs, 2)
```

---

### `HasEnoughMemory`

```go
HasEnoughMemory(mem *string) (bool, error)
```

Checks whether the device's reported memory meets or exceeds the required amount. Values use binary unit suffixes (`Ki`, `Mi`, `Gi`). A `nil` or empty requirement always passes.

**Example:**

```go
required := "2Gi"
ok, err := checker.HasEnoughMemory(&required)
```

---

### `HasEnoughStorage`

```go
HasEnoughStorage(storage *string) (bool, error)
```

Checks whether the device's reported storage meets or exceeds the required amount. Values use binary unit suffixes (`Ki`, `Mi`, `Gi`, `Ti`, `Pi`, `Ei`). A `nil` or empty requirement always passes.

**Example:**

```go
required := "100Gi"
ok, err := checker.HasEnoughStorage(&required)
```

---

## Failure Reasons

| Scenario | `reason` string |
|----------|----------------|
| CPU cores/arch not met | `"cpu requirement not fulfilled"` |
| Memory insufficient | `"mem requirement not fulfilled"` |
| Storage insufficient | `"storage requirement not fulfilled"` |

---

## Interface Reference

```go
// common.CapacityEligibilityCheckerIface
type CapacityEligibilityCheckerIface interface {
    HasEnoughCPUCores(arch *[]string, cores float32) bool
    HasEnoughMemory(mem *string) (bool, error)
    HasEnoughStorage(storage *string) (bool, error)
    CheckEligibility(checks *clModels.CapacityRequirements) (bool, string, error)
}
```
