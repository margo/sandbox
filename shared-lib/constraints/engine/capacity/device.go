package capacity

import (
	"fmt"
	"slices"

	"github.com/margo/sandbox/shared-lib/constraints/common"
	"github.com/margo/sandbox/shared-lib/quantity"
	clModels "github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
)

// New creates a new CapacityEligibilityCheckerIface backed by
// the provided DeviceCapabilitiesManifest.
func New(i clModels.DeviceCapabilitiesManifest) common.CapacityEligibilityCheckerIface {
	dc := deviceCapabilities(i)
	return &dc
}

// deviceCapabilities is a type alias over DeviceCapabilitiesManifest that
// implements DeviceCheckerIface with hardware constraint checking methods.
type deviceCapabilities clModels.DeviceCapabilitiesManifest

// CheckEligibility validates that a device meets all capacity requirements
// defined in checks (CPU, memory, storage). Returns true if all checks pass,
// along with a human-readable reason string if any check fails.
func (dc *deviceCapabilities) CheckEligibility(checks *clModels.CapacityRequirements) (bool, string, error) {

	// No requirements to check — device always qualifies.
	if checks == nil {
		return true, "", nil
	}

	if checks.Cpu != nil {
		// Convert architecture enum slice to string slice for comparison.
		var archStrings *[]string
		if checks.Cpu.Architectures != nil {
			archs := make([]string, len(*checks.Cpu.Architectures))
			for i, a := range *checks.Cpu.Architectures {
				archs[i] = string(a)
			}
			archStrings = &archs
		}

		result := dc.HasEnoughCPUCores(archStrings, checks.Cpu.Cores)
		if !result {
			return result, "cpu requirement not fulfilled", nil
		}
	}

	if checks.Memory != nil {
		result, err := dc.HasEnoughMemory(checks.Memory)
		if err != nil {
			return result, "", err
		}

		if !result {
			return result, "mem requirement not fulfilled", nil
		}
	}

	if checks.Storage != nil {
		result, err := dc.HasEnoughStorage(checks.Storage)
		if err != nil {
			return result, "", err
		}
		if !result {
			return result, "storage requirement not fulfilled", nil
		}
	}

	return true, "", nil
}

// HasEnoughCPUCores checks whether the device has at least the required number
// of CPU cores. If arch is provided, only CPUs matching one of the specified
// architectures are considered. If arch is nil, any CPU with sufficient cores satisfies the requirement.
func (dc *deviceCapabilities) HasEnoughCPUCores(arch *[]string, cores float32) bool {
	if dc.Properties.Cpus == nil {
		return false
	}

	for _, c := range *dc.Properties.Cpus {

		// No architecture filter specified — any CPU with enough cores satisfies the requirement.
		if arch == nil {
			if c.Cores >= cores {
				return true
			}
			continue
		}

		// Skip CPUs that don't report their architecture, as we cannot verify compatibility.
		if c.Architecture == nil {
			continue
		}

		// Skip CPUs whose architecture is not in the required architectures list.
		if i := slices.Index(*arch, string(*c.Architecture)); i == -1 {
			continue
		}

		if c.Cores >= cores {
			return true
		}

	}

	return false
}

// HasEnoughMemory checks whether the device has at least the required amount of memory.
// A nil requirement is treated as no constraint and always passes.
func (dc *deviceCapabilities) HasEnoughMemory(mem *string) (bool, error) {
	// No memory requirement specified — always satisfied.
	if mem == nil || *mem == "" {
		return true, nil
	}

	if dc.Properties.Memory == nil || *dc.Properties.Memory == "" {
		return false, nil
	}

	ok, err := satisfies(*mem, *dc.Properties.Memory)
	if err != nil {
		return false, fmt.Errorf("failed to check memory requirements, err : %s", err.Error())
	}
	return ok, nil
}

// HasEnoughStorage checks whether the device has at least the required amount of storage.
// A nil requirement is treated as no constraint and always passes.
func (dc *deviceCapabilities) HasEnoughStorage(storage *string) (bool, error) {
	// No storage requirement specified — always satisfied.
	if storage == nil || *storage == "" {
		return true, nil
	}

	if dc.Properties.Storage == nil || *dc.Properties.Storage == "" {
		return false, nil
	}

	ok, err := satisfies(*storage, *dc.Properties.Storage)
	if err != nil {
		return false, fmt.Errorf("failed to check storage requirements, err : %s", err.Error())
	}
	return ok, nil
}

// Satisfies parses required and actual quantity strings and returns true when
// actual >= required.
//
// Returns an error if either string fails to parse.
func satisfies(required, actual string) (bool, error) {
	req, err := quantity.Parse(required)
	if err != nil {
		return false, fmt.Errorf("failed to parse required quantity: %w", err)
	}

	act, err := quantity.Parse(actual)
	if err != nil {
		return false, fmt.Errorf("failed to parse actual quantity: %w", err)
	}

	return act.AtLeast(req), nil
}
