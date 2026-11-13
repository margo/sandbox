package capacity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/margo/sandbox/shared-lib/pointers"
	clModels "github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
)

type deviceCPU = struct {
	Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
	Class        *clModels.DeviceCapabilitiesManifestPropertiesCpusClass        `json:"class,omitempty"`
	Cores        float32                                                        `json:"cores"`
	Frequency    *struct {
		BaseMHz *float32 `json:"baseMHz,omitempty"`
		MaxMHz  *float32 `json:"maxMHz,omitempty"`
	} `json:"frequency,omitempty"`
	Type *clModels.DeviceCapabilitiesManifestPropertiesCpusType `json:"type,omitempty"`
}

func makeDevice(cpus *[]deviceCPU, memory, storage *string) clModels.DeviceCapabilitiesManifest {
	var d clModels.DeviceCapabilitiesManifest
	d.Properties.Id = "device-1"
	d.Properties.ModelNumber = "model-x"
	d.Properties.SerialNumber = "sn-001"
	d.Properties.Vendor = "acme"
	d.Properties.Cpus = cpus
	if memory != nil {
		d.Properties.Memory = &clModels.DeviceCapabilitiesManifestPropertiesMemory{Size: *memory}
	}
	d.Properties.Storage = storage
	return d
}

// ── HasEnoughCpu ─────────────────────────────────────────────────────────────

func TestHasEnoughCpu(t *testing.T) {
	amd64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	arm64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureArm64
	isolatedDev := clModels.DeviceCapabilitiesManifestPropertiesCpusTypeIsolated
	sharedDev := clModels.DeviceCapabilitiesManifestPropertiesCpusTypeShared

	reqAmd64 := clModels.DeploymentCpuRequirementArchitecturesAmd64
	reqArm64 := clModels.DeploymentCpuRequirementArchitecturesArm64
	reqRiscv64 := clModels.DeploymentCpuRequirementArchitecturesRiscv64
	reqIsolated := clModels.DeploymentCpuRequirementTypeIsolated
	reqShared := clModels.DeploymentCpuRequirementTypeShared

	cpus := &[]deviceCPU{
		{Architecture: pointers.Ptr(amd64), Type: pointers.Ptr(isolatedDev), Cores: 8},
		{Architecture: pointers.Ptr(arm64), Type: pointers.Ptr(sharedDev), Cores: 4},
		{Cores: 2}, // no architecture or type reported
	}

	tests := []struct {
		name string
		reqs []clModels.DeploymentCpuRequirement
		want bool
	}{
		{
			name: "empty requirements always passes",
			reqs: nil,
			want: true,
		},
		{
			name: "no arch filter, device has enough cores",
			reqs: []clModels.DeploymentCpuRequirement{
				{Cores: 2},
			},
			want: true,
		},
		{
			name: "no arch filter, required cores exceed all CPUs",
			reqs: []clModels.DeploymentCpuRequirement{
				{Cores: 16},
			},
			want: false,
		},
		{
			name: "arch filter matches amd64 with enough cores",
			reqs: []clModels.DeploymentCpuRequirement{
				{
					Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{reqAmd64},
					Cores:         8,
				},
			},
			want: true,
		},
		{
			name: "arch filter matches amd64 but not enough cores",
			reqs: []clModels.DeploymentCpuRequirement{
				{
					Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{reqAmd64},
					Cores:         10,
				},
			},
			want: false,
		},
		{
			name: "arch filter matches arm64 with enough cores",
			reqs: []clModels.DeploymentCpuRequirement{
				{
					Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{reqArm64},
					Cores:         4,
				},
			},
			want: true,
		},
		{
			name: "arch filter does not match any CPU",
			reqs: []clModels.DeploymentCpuRequirement{
				{
					Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{reqRiscv64},
					Cores:         1,
				},
			},
			want: false,
		},
		{
			name: "CPU without architecture is skipped when arch filter is set",
			reqs: []clModels.DeploymentCpuRequirement{
				{
					Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{reqAmd64},
					Cores:         1,
				},
			},
			want: true, // amd64 CPU with 8 cores satisfies
		},
		{
			name: "type and arch filter matches isolated amd64",
			reqs: []clModels.DeploymentCpuRequirement{
				{
					Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{reqAmd64},
					Type:          pointers.Ptr(reqIsolated),
					Cores:         4,
				},
				{
					Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{reqAmd64},
					Type:          pointers.Ptr(reqIsolated),
					Cores:         4,
				},
			},
			want: true,
		},
		{
			name: "multiple requirements exceed isolated amd64 capacity",
			reqs: []clModels.DeploymentCpuRequirement{
				{
					Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{reqAmd64},
					Type:          pointers.Ptr(reqIsolated),
					Cores:         5,
				},
				{
					Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{reqAmd64},
					Type:          pointers.Ptr(reqIsolated),
					Cores:         4,
				},
			},
			want: false,
		},
		{
			name: "type filter mismatch fails",
			reqs: []clModels.DeploymentCpuRequirement{
				{
					Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{reqAmd64},
					Type:          pointers.Ptr(reqShared),
					Cores:         1,
				},
			},
			want: false,
		},
		{
			name: "no CPUs on device",
			reqs: []clModels.DeploymentCpuRequirement{
				{Cores: 1},
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deviceCPUs := cpus
			if tc.name == "no CPUs on device" {
				deviceCPUs = nil
			}
			d := makeDevice(deviceCPUs, nil, nil)
			checker := New(d)
			assert.Equal(t, tc.want, checker.HasEnoughCpu(tc.reqs))
		})
	}
}

// ── HasEnoughCache ───────────────────────────────────────────────────────────

func TestHasEnoughCache(t *testing.T) {
	type deviceCache = struct {
		AllocationTypes []clModels.DeviceCapabilitiesManifestPropertiesCacheAllocationTypes `json:"allocationTypes"`
		Level           clModels.DeviceCapabilitiesManifestPropertiesCacheLevel             `json:"level"`
		Size            string                                                              `json:"size"`
	}

	devL3 := clModels.DeviceCapabilitiesManifestPropertiesCacheLevel("L3")
	devL2 := clModels.DeviceCapabilitiesManifestPropertiesCacheLevel("L2")

	reqL3 := clModels.DeploymentCacheRequirementLevelL3
	reqL2 := clModels.DeploymentCacheRequirementLevelL2
	reqL1d := clModels.DeploymentCacheRequirementLevelL1d
	reqExclusive := clModels.DeploymentCacheRequirementAllocationExclusive
	reqShared := clModels.DeploymentCacheRequirementAllocationShared

	defaultCaches := &[]deviceCache{
		{Level: devL3, Size: "8Mi"},
		{Level: devL2, Size: "2Mi"},
	}

	tests := []struct {
		name         string
		deviceCaches *[]deviceCache
		reqs         *[]clModels.DeploymentCacheRequirement
		wantOk       bool
		wantErr      bool
	}{
		{
			name:         "nil requirements always passes",
			deviceCaches: defaultCaches,
			reqs:         nil,
			wantOk:       true,
		},
		{
			name:         "empty requirements always passes",
			deviceCaches: defaultCaches,
			reqs:         &[]clModels.DeploymentCacheRequirement{},
			wantOk:       true,
		},
		{
			name:         "no cache on device fails",
			deviceCaches: nil,
			reqs: &[]clModels.DeploymentCacheRequirement{
				{Level: reqL3, Allocation: reqExclusive, Size: pointers.Ptr("4Mi")},
			},
			wantOk: false,
		},
		{
			name:         "single cache requirement satisfied",
			deviceCaches: defaultCaches,
			reqs: &[]clModels.DeploymentCacheRequirement{
				{Level: reqL3, Allocation: reqExclusive, Size: pointers.Ptr("8Mi")},
			},
			wantOk: true,
		},
		{
			name:         "exclusive and shared summed within level and satisfied",
			deviceCaches: defaultCaches,
			reqs: &[]clModels.DeploymentCacheRequirement{
				{Level: reqL3, Allocation: reqExclusive, Size: pointers.Ptr("4Mi")},
				{Level: reqL3, Allocation: reqShared, Size: pointers.Ptr("4Mi")},
			},
			wantOk: true,
		},
		{
			name:         "exclusive and shared summed exceeds level capacity",
			deviceCaches: defaultCaches,
			reqs: &[]clModels.DeploymentCacheRequirement{
				{Level: reqL3, Allocation: reqExclusive, Size: pointers.Ptr("5Mi")},
				{Level: reqL3, Allocation: reqShared, Size: pointers.Ptr("4Mi")},
			},
			wantOk: false,
		},
		{
			name:         "missing cache level on device fails",
			deviceCaches: defaultCaches,
			reqs: &[]clModels.DeploymentCacheRequirement{
				{Level: reqL1d, Allocation: reqShared, Size: pointers.Ptr("32Ki")},
			},
			wantOk: false,
		},
		{
			name: "multiple device cache entries of same level are summed",
			deviceCaches: &[]deviceCache{
				{Level: devL3, Size: "4Mi"},
				{Level: devL3, Size: "4Mi"},
			},
			reqs: &[]clModels.DeploymentCacheRequirement{
				{Level: reqL3, Allocation: reqExclusive, Size: pointers.Ptr("8Mi")},
			},
			wantOk: true,
		},
		{
			name:         "invalid required cache size returns error",
			deviceCaches: defaultCaches,
			reqs: &[]clModels.DeploymentCacheRequirement{
				{Level: reqL3, Allocation: reqExclusive, Size: pointers.Ptr("invalid")},
			},
			wantOk:  false,
			wantErr: true,
		},
		{
			name: "invalid device cache size returns error",
			deviceCaches: &[]deviceCache{
				{Level: devL3, Size: "invalid"},
			},
			reqs: &[]clModels.DeploymentCacheRequirement{
				{Level: reqL3, Allocation: reqExclusive, Size: pointers.Ptr("4Mi")},
			},
			wantOk:  false,
			wantErr: true,
		},
		{
			name:         "multiple levels satisfied simultaneously",
			deviceCaches: defaultCaches,
			reqs: &[]clModels.DeploymentCacheRequirement{
				{Level: reqL3, Allocation: reqExclusive, Size: pointers.Ptr("4Mi")},
				{Level: reqL2, Allocation: reqShared, Size: pointers.Ptr("2Mi")},
			},
			wantOk: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := makeDevice(nil, nil, nil)
			d.Properties.Cache = tc.deviceCaches
			checker := New(d)
			ok, err := checker.HasEnoughCache(tc.reqs)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.wantOk, ok)
		})
	}
}

// ── HasEnoughMemory ──────────────────────────────────────────────────────────

func TestHasEnoughMemory(t *testing.T) {
	tests := []struct {
		name         string
		deviceMemory *string
		required     *string
		wantOk       bool
		wantErr      bool
	}{
		{
			name:         "nil requirement always passes",
			deviceMemory: pointers.Ptr("4Gi"),
			required:     nil,
			wantOk:       true,
		},
		{
			name:         "empty string requirement always passes",
			deviceMemory: pointers.Ptr("4Gi"),
			required:     pointers.Ptr(""),
			wantOk:       true,
		},
		{
			name:         "device has exactly required memory",
			deviceMemory: pointers.Ptr("4Gi"),
			required:     pointers.Ptr("4Gi"),
			wantOk:       true,
		},
		{
			name:         "device exceeds required memory",
			deviceMemory: pointers.Ptr("8Gi"),
			required:     pointers.Ptr("4Gi"),
			wantOk:       true,
		},
		{
			name:         "device has less than required memory",
			deviceMemory: pointers.Ptr("2Gi"),
			required:     pointers.Ptr("4Gi"),
			wantOk:       false,
		},
		{
			name:         "device has no memory reported",
			deviceMemory: nil,
			required:     pointers.Ptr("4Gi"),
			wantOk:       false,
		},
		{
			name:         "device memory is empty string",
			deviceMemory: pointers.Ptr(""),
			required:     pointers.Ptr("4Gi"),
			wantOk:       false,
		},
		{
			name:         "invalid required quantity returns error",
			deviceMemory: pointers.Ptr("4Gi"),
			required:     pointers.Ptr("not-a-quantity"),
			wantOk:       false,
			wantErr:      true,
		},
		{
			name:         "invalid device quantity returns error",
			deviceMemory: pointers.Ptr("bad-value"),
			required:     pointers.Ptr("4Gi"),
			wantOk:       false,
			wantErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := makeDevice(nil, tc.deviceMemory, nil)
			checker := New(d)
			var required *clModels.DeploymentMemoryRequirement
			if tc.required != nil {
				required = &clModels.DeploymentMemoryRequirement{Size: *tc.required}
			}
			ok, err := checker.HasEnoughMemory(required)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.wantOk, ok)
		})
	}
}

// ── HasEnoughStorage ─────────────────────────────────────────────────────────

func TestHasEnoughStorage(t *testing.T) {
	tests := []struct {
		name          string
		deviceStorage *string
		required      *string
		wantOk        bool
		wantErr       bool
	}{
		{
			name:          "nil requirement always passes",
			deviceStorage: pointers.Ptr("100Gi"),
			required:      nil,
			wantOk:        true,
		},
		{
			name:          "empty string requirement always passes",
			deviceStorage: pointers.Ptr("100Gi"),
			required:      pointers.Ptr(""),
			wantOk:        true,
		},
		{
			name:          "device has exactly required storage",
			deviceStorage: pointers.Ptr("100Gi"),
			required:      pointers.Ptr("100Gi"),
			wantOk:        true,
		},
		{
			name:          "device exceeds required storage",
			deviceStorage: pointers.Ptr("500Gi"),
			required:      pointers.Ptr("100Gi"),
			wantOk:        true,
		},
		{
			name:          "device has less than required storage",
			deviceStorage: pointers.Ptr("50Gi"),
			required:      pointers.Ptr("100Gi"),
			wantOk:        false,
		},
		{
			name:          "device has no storage reported",
			deviceStorage: nil,
			required:      pointers.Ptr("100Gi"),
			wantOk:        false,
		},
		{
			name:          "device storage is empty string",
			deviceStorage: pointers.Ptr(""),
			required:      pointers.Ptr("100Gi"),
			wantOk:        false,
		},
		{
			name:          "invalid required quantity returns error",
			deviceStorage: pointers.Ptr("100Gi"),
			required:      pointers.Ptr("not-a-quantity"),
			wantOk:        false,
			wantErr:       true,
		},
		{
			name:          "invalid device quantity returns error",
			deviceStorage: pointers.Ptr("bad-value"),
			required:      pointers.Ptr("100Gi"),
			wantOk:        false,
			wantErr:       true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := makeDevice(nil, nil, tc.deviceStorage)
			checker := New(d)
			ok, err := checker.HasEnoughStorage(tc.required)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.wantOk, ok)
		})
	}
}

// ── CheckEligibility ─────────────────────────────────────────────────────────

func TestCheckEligibility(t *testing.T) {
	amd64Arch := clModels.DeploymentCpuRequirementArchitecturesAmd64
	arm64Arch := clModels.DeploymentCpuRequirementArchitecturesArm64

	amd64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64

	cpus := &[]deviceCPU{
		{Architecture: pointers.Ptr(amd64), Cores: 8},
	}

	tests := []struct {
		name       string
		checks     *clModels.CapacityRequirements
		memory     *string
		storage    *string
		wantOk     bool
		wantReason string
		wantErr    bool
	}{
		{
			name:    "nil requirements always eligible",
			checks:  nil,
			memory:  pointers.Ptr("8Gi"),
			storage: pointers.Ptr("100Gi"),
			wantOk:  true,
		},
		{
			name: "all requirements satisfied",
			checks: &clModels.CapacityRequirements{
				Cpu: &[]clModels.DeploymentCpuRequirement{
					{
						Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{amd64Arch},
						Cores:         4,
					},
				},
				Memory:  &clModels.DeploymentMemoryRequirement{Size: "4Gi"},
				Storage: pointers.Ptr("50Gi"),
			},
			memory:  pointers.Ptr("8Gi"),
			storage: pointers.Ptr("100Gi"),
			wantOk:  true,
		},
		{
			name: "CPU requirement not fulfilled",
			checks: &clModels.CapacityRequirements{
				Cpu: &[]clModels.DeploymentCpuRequirement{
					{
						Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{amd64Arch},
						Cores:         16,
					},
				},
			},
			memory:     pointers.Ptr("8Gi"),
			storage:    pointers.Ptr("100Gi"),
			wantOk:     false,
			wantReason: "cpu requirement not fulfilled",
		},
		{
			name: "CPU arch not matched",
			checks: &clModels.CapacityRequirements{
				Cpu: &[]clModels.DeploymentCpuRequirement{
					{
						Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{arm64Arch},
						Cores:         4,
					},
				},
			},
			memory:     pointers.Ptr("8Gi"),
			storage:    pointers.Ptr("100Gi"),
			wantOk:     false,
			wantReason: "cpu requirement not fulfilled",
		},
		{
			name: "memory requirement not fulfilled",
			checks: &clModels.CapacityRequirements{
				Memory: &clModels.DeploymentMemoryRequirement{Size: "16Gi"},
			},
			memory:     pointers.Ptr("8Gi"),
			storage:    pointers.Ptr("100Gi"),
			wantOk:     false,
			wantReason: "mem requirement not fulfilled",
		},
		{
			name: "storage requirement not fulfilled",
			checks: &clModels.CapacityRequirements{
				Storage: pointers.Ptr("500Gi"),
			},
			memory:     pointers.Ptr("8Gi"),
			storage:    pointers.Ptr("100Gi"),
			wantOk:     false,
			wantReason: "storage requirement not fulfilled",
		},
		{
			name: "invalid memory quantity returns error",
			checks: &clModels.CapacityRequirements{
				Memory: &clModels.DeploymentMemoryRequirement{Size: "not-valid"},
			},
			memory:  pointers.Ptr("8Gi"),
			storage: pointers.Ptr("100Gi"),
			wantOk:  false,
			wantErr: true,
		},
		{
			name: "invalid storage quantity returns error",
			checks: &clModels.CapacityRequirements{
				Storage: pointers.Ptr("not-valid"),
			},
			memory:  pointers.Ptr("8Gi"),
			storage: pointers.Ptr("100Gi"),
			wantOk:  false,
			wantErr: true,
		},
		{
			name: "nil CPU architectures — any arch with enough cores passes",
			checks: &clModels.CapacityRequirements{
				Cpu: &[]clModels.DeploymentCpuRequirement{
					{
						Architectures: nil,
						Cores:         4,
					},
				},
			},
			memory:  pointers.Ptr("8Gi"),
			storage: pointers.Ptr("100Gi"),
			wantOk:  true,
		},
		{
			name: "only CPU check, no memory or storage requirement",
			checks: &clModels.CapacityRequirements{
				Cpu: &[]clModels.DeploymentCpuRequirement{
					{
						Cores: 4,
					},
				},
			},
			memory:  nil,
			storage: nil,
			wantOk:  true,
		},
		{
			name: "multiple CPU requirements all satisfied",
			checks: &clModels.CapacityRequirements{
				Cpu: &[]clModels.DeploymentCpuRequirement{
					{
						Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{amd64Arch},
						Cores:         4,
					},
					{
						Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{amd64Arch},
						Cores:         2,
					},
				},
				Memory:  &clModels.DeploymentMemoryRequirement{Size: "4Gi"},
				Storage: pointers.Ptr("50Gi"),
			},
			memory:  pointers.Ptr("8Gi"),
			storage: pointers.Ptr("100Gi"),
			wantOk:  true,
		},
		{
			name: "multiple CPU requirements one not satisfied",
			checks: &clModels.CapacityRequirements{
				Cpu: &[]clModels.DeploymentCpuRequirement{
					{
						Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{amd64Arch},
						Cores:         4,
					},
					{
						Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{arm64Arch},
						Cores:         2,
					},
				},
				Memory:  &clModels.DeploymentMemoryRequirement{Size: "4Gi"},
				Storage: pointers.Ptr("50Gi"),
			},
			memory:     pointers.Ptr("8Gi"),
			storage:    pointers.Ptr("100Gi"),
			wantOk:     false,
			wantReason: "cpu requirement not fulfilled",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := makeDevice(cpus, tc.memory, tc.storage)
			checker := New(d)
			ok, reason, err := checker.CheckEligibility(tc.checks)

			if tc.wantErr {
				require.Error(t, err)
				assert.False(t, ok)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantOk, ok)
			assert.Equal(t, tc.wantReason, reason)
		})
	}
}
