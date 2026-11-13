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
// defined in checks (CPU, cache, memory, storage). Returns true if all checks pass,
// along with a human-readable reason string if any check fails.
func (dc *deviceCapabilities) CheckEligibility(
	checks *clModels.CapacityRequirements,
) (bool, string, error) {

	// No requirements to check — device always qualifies.
	if checks == nil {
		return true, "", nil
	}

	if checks.Cpu != nil {
		if !dc.HasEnoughCpu(*checks.Cpu) {
			return false, "cpu requirement not fulfilled", nil
		}
	}

	if checks.Cache != nil {
		result, err := dc.HasEnoughCache(checks.Cache)
		if err != nil {
			return false, "", err
		}
		if !result {
			return false, "cache requirement not fulfilled", nil
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

// HasEnoughCpu maps both the device's CPU capabilities and the
// deployment's CPU requirements into map[cpuType]map[cpuArch]cores, then
// cross-references the required cores against the device's available cores
func (dc *deviceCapabilities) HasEnoughCpu(reqs []clModels.DeploymentCpuRequirement) bool {
	if len(reqs) == 0 {
		return true
	}
	if dc.Properties.Cpus == nil || len(*dc.Properties.Cpus) == 0 {
		return false
	}

	deviceCPUs := dc.mapDeviceCpus()
	requiredCPUs := mapCpuRequirements(reqs)

	for reqType, reqArchMap := range requiredCPUs {
		for reqArch, reqCores := range reqArchMap {
			if availableCpuCores(deviceCPUs, reqType, reqArch) < reqCores {
				return false
			}
		}
	}

	return true
}

func (dc *deviceCapabilities) mapDeviceCpus() map[string]map[string]float32 {
	deviceCpus := make(map[string]map[string]float32)
	if dc.Properties.Cpus == nil {
		return deviceCpus
	}

	for _, cpu := range *dc.Properties.Cpus {
		cpuType := ""
		if cpu.Type != nil {
			cpuType = string(*cpu.Type)
		}

		cpuArch := ""
		if cpu.Architecture != nil {
			cpuArch = string(*cpu.Architecture)
		}

		if deviceCpus[cpuType] == nil {
			deviceCpus[cpuType] = make(map[string]float32)
		}
		deviceCpus[cpuType][cpuArch] += cpu.Cores
	}

	return deviceCpus
}

func mapCpuRequirements(reqs []clModels.DeploymentCpuRequirement) map[string]map[string]float32 {
	requiredCpus := make(map[string]map[string]float32)

	for _, req := range reqs {
		cpuType := ""
		if req.Type != nil {
			cpuType = string(*req.Type)
		}

		if requiredCpus[cpuType] == nil {
			requiredCpus[cpuType] = make(map[string]float32)
		}

		if req.Architectures == nil || len(*req.Architectures) == 0 {
			requiredCpus[cpuType][""] += req.Cores
			continue
		}

		for _, arch := range *req.Architectures {
			requiredCpus[cpuType][string(arch)] += req.Cores
		}
	}

	return requiredCpus
}

func availableCpuCores(deviceCpus map[string]map[string]float32, reqType, reqArch string) float32 {
	var available float32
	for devType, devArchMap := range deviceCpus {
		if reqType != "" && devType != reqType {
			continue
		}
		for devArch, cores := range devArchMap {
			if reqArch != "" && devArch != reqArch {
				continue
			}
			available += cores
		}
	}
	return available
}

// maps both the device's cache capabilities and the
// deployment's cache requirements into map[cacheLevel]sizeInBytes, then
// cross-references the required bytes against the device's available bytes
func (dc *deviceCapabilities) HasEnoughCache(cacheReqs *[]clModels.DeploymentCacheRequirement) (bool, error) {
	if cacheReqs == nil || len(*cacheReqs) == 0 {
		return true, nil
	}
	if dc.Properties.Cache == nil || len(*dc.Properties.Cache) == 0 {
		return false, nil
	}

	deviceCaches, err := dc.mapDeviceCaches()
	if err != nil {
		return false, err
	}

	requiredCaches, err := mapCacheRequirements(*cacheReqs)
	if err != nil {
		return false, err
	}

	for reqLevel, reqBytes := range requiredCaches {
		devBytes, exists := deviceCaches[reqLevel]
		if !exists || devBytes < reqBytes {
			return false, nil
		}
	}

	return true, nil
}

func (dc *deviceCapabilities) mapDeviceCaches() (map[string]int64, error) {
	deviceCaches := make(map[string]int64)
	if dc.Properties.Cache == nil {
		return deviceCaches, nil
	}

	for _, cache := range *dc.Properties.Cache {
		q, err := quantity.Parse(cache.Size)
		if err != nil {
			return nil, fmt.Errorf("failed to check cache requirements, err : %s", err.Error())
		}
		deviceCaches[string(cache.Level)] += q.Bytes()
	}

	return deviceCaches, nil
}

func mapCacheRequirements(reqs []clModels.DeploymentCacheRequirement) (map[string]int64, error) {
	requiredCaches := make(map[string]int64)

	for _, req := range reqs {
		var needed int64
		if req.Size != nil && *req.Size != "" {
			q, err := quantity.Parse(*req.Size)
			if err != nil {
				return nil, fmt.Errorf("failed to check cache requirements, err : %s", err.Error())
			}
			needed = q.Bytes()
		}
		requiredCaches[string(req.Level)] += needed
	}

	return requiredCaches, nil
}

// HasEnoughMemory checks whether the device has at least the required amount of memory
// and supports any requested memory bandwidth allocation strategy.
// A nil requirement is treated as no constraint and always passes.
func (dc *deviceCapabilities) HasEnoughMemory(mem *clModels.DeploymentMemoryRequirement) (bool, error) {
	// No memory requirement specified — always satisfied.
	if mem == nil {
		return true, nil
	}

	if mem.Size != "" {
		if dc.Properties.Memory == nil || dc.Properties.Memory.Size == "" {
			return false, nil
		}

		ok, err := satisfies(mem.Size, dc.Properties.Memory.Size)
		if err != nil {
			return false, fmt.Errorf("failed to check memory requirements, err : %s", err.Error())
		}
		if !ok {
			return false, nil
		}
	}

	if mem.BandwidthAllocation != nil {
		if dc.Properties.Memory == nil || dc.Properties.Memory.BandwidthAllocationTypes == nil {
			return false, nil
		}
		var requiredType clModels.DeviceCapabilitiesManifestPropertiesMemoryBandwidthAllocationTypes
		switch mem.BandwidthAllocation.Unit {
		case clModels.Percent:
			if mem.BandwidthAllocation.Value <= 0 || mem.BandwidthAllocation.Value > 100 {
				return false, nil
			}
			requiredType = clModels.PercentThrottling
		case clModels.MBps:
			if mem.BandwidthAllocation.Value <= 0 {
				return false, nil
			}
			requiredType = clModels.RateLimiting
		default:
			return false, nil
		}
		if !slices.Contains(*dc.Properties.Memory.BandwidthAllocationTypes, requiredType) {
			return false, nil
		}
	}

	return true, nil
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
