package common

import (
	clModels "github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
)

type PropertySelectorEngineIface interface {
	SelectorEngineIface
	HandleContainsAll(clModels.MatchExpression) (bool, string)
	HandleContainsAny(clModels.MatchExpression) (bool, string)
}

type LabelSelectorEngineIface SelectorEngineIface

type SelectorEngineIface interface {
	Evaluate(*clModels.Selector) (bool, string)
	HandleIn(*clModels.MatchExpression) (bool, string)
	HandleNotIn(*clModels.MatchExpression) (bool, string)
	HandleExists(*clModels.MatchExpression) (bool, string)
	HandleDoesNotExists(*clModels.MatchExpression) (bool, string)
	HandleGt(*clModels.MatchExpression) (bool, string)
	HandleLt(*clModels.MatchExpression) (bool, string)
}

type DeviceSelectorIface interface {
	SelectEligibleDevice(devices []*clModels.DeviceCapabilitiesManifest, checks *clModels.DeviceConstraints) ([]*clModels.DeviceCapabilitiesManifest, error)
	IsDeviceEligible(device *clModels.DeviceCapabilitiesManifest, checks *clModels.DeviceConstraints) (bool, string, error)
}

// CapacityEligibilityCheckerIface defines the contract for checking whether a device
// meets specific hardware capacity requirements.
type CapacityEligibilityCheckerIface interface {
	HasEnoughCPUCores(arch *[]string, cores float32) bool
	HasEnoughMemory(mem *string) (bool, error)
	HasEnoughStorage(storage *string) (bool, error)
	CheckEligibility(checks *clModels.CapacityRequirements) (bool, string, error)
}
