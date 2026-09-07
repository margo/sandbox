package capacity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/margo/sandbox/shared-lib/pointers"
	clModels "github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
)

func makeDevice(cpus *[]struct {
	Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
	Cores        float32                                                        `json:"cores"`
}, memory, storage *string) clModels.DeviceCapabilitiesManifest {
	return clModels.DeviceCapabilitiesManifest{
		ApiVersion: "v1",
		Kind:       clModels.DeviceCapabilitiesManifestKindDeviceCapabilitiesManifest,
		Properties: struct {
			Cpus *[]struct {
				Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
				Cores        float32                                                        `json:"cores"`
			} `json:"cpus,omitempty"`
			Id                       clModels.DeviceId                                                        `json:"id"`
			Interfaces               *[]clModels.DeviceCommunicationInterface                                 `json:"interfaces,omitempty"`
			Memory                   *string                                                                  `json:"memory,omitempty"`
			ModelNumber              string                                                                   `json:"modelNumber"`
			OtelCollector            *bool                                                                    `json:"otelCollector,omitempty"`
			Peripherals              *[]clModels.DevicePeripheral                                             `json:"peripherals,omitempty"`
			SerialNumber             string                                                                   `json:"serialNumber"`
			Storage                  *string                                                                  `json:"storage,omitempty"`
			SupportedDeploymentTypes *[]clModels.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypes `json:"supportedDeploymentTypes,omitempty"`
			SupportedRuntimes        *[]clModels.DeviceCapabilitiesManifestPropertiesSupportedRuntimes        `json:"supportedRuntimes,omitempty"`
			Vendor                   string                                                                   `json:"vendor"`
		}{
			Id:           "device-1",
			ModelNumber:  "model-x",
			SerialNumber: "sn-001",
			Vendor:       "acme",
			Cpus:         cpus,
			Memory:       memory,
			Storage:      storage,
		},
	}
}

// ── HasEnoughCPUCores ────────────────────────────────────────────────────────

func TestHasEnoughCPUCores(t *testing.T) {
	amd64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	arm64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureArm64

	cpus := &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		{Architecture: pointers.Ptr(amd64), Cores: 8},
		{Architecture: pointers.Ptr(arm64), Cores: 4},
		{Cores: 2}, // no architecture reported
	}

	tests := []struct {
		name  string
		arch  *[]string
		cores float32
		want  bool
	}{
		{
			name:  "no arch filter, device has enough cores",
			arch:  nil,
			cores: 2,
			want:  true,
		},
		{
			name:  "no arch filter, required cores exceed all CPUs",
			arch:  nil,
			cores: 16,
			want:  false,
		},
		{
			name:  "arch filter matches amd64 with enough cores",
			arch:  &[]string{"amd64"},
			cores: 8,
			want:  true,
		},
		{
			name:  "arch filter matches amd64 but not enough cores",
			arch:  &[]string{"amd64"},
			cores: 10,
			want:  false,
		},
		{
			name:  "arch filter matches arm64 with enough cores",
			arch:  &[]string{"arm64"},
			cores: 4,
			want:  true,
		},
		{
			name:  "arch filter does not match any CPU",
			arch:  &[]string{"riscv64"},
			cores: 1,
			want:  false,
		},
		{
			name:  "CPU without architecture is skipped when arch filter is set",
			arch:  &[]string{"amd64"},
			cores: 1,
			want:  true, // amd64 CPU with 8 cores satisfies
		},
		{
			name:  "no CPUs on device",
			arch:  nil,
			cores: 1,
			want:  false,
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
			assert.Equal(t, tc.want, checker.HasEnoughCPUCores(tc.arch, tc.cores))
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
			ok, err := checker.HasEnoughMemory(tc.required)
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

	cpus := &[]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
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
				Cpu: &clModels.DeploymentCpuRequirement{
					Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{amd64Arch},
					Cores:         4,
				},
				Memory:  pointers.Ptr("4Gi"),
				Storage: pointers.Ptr("50Gi"),
			},
			memory:  pointers.Ptr("8Gi"),
			storage: pointers.Ptr("100Gi"),
			wantOk:  true,
		},
		{
			name: "CPU requirement not fulfilled",
			checks: &clModels.CapacityRequirements{
				Cpu: &clModels.DeploymentCpuRequirement{
					Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{amd64Arch},
					Cores:         16,
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
				Cpu: &clModels.DeploymentCpuRequirement{
					Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{arm64Arch},
					Cores:         4,
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
				Memory: pointers.Ptr("16Gi"),
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
				Memory: pointers.Ptr("not-valid"),
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
				Cpu: &clModels.DeploymentCpuRequirement{
					Architectures: nil,
					Cores:         4,
				},
			},
			memory:  pointers.Ptr("8Gi"),
			storage: pointers.Ptr("100Gi"),
			wantOk:  true,
		},
		{
			name: "only CPU check, no memory or storage requirement",
			checks: &clModels.CapacityRequirements{
				Cpu: &clModels.DeploymentCpuRequirement{
					Cores: 4,
				},
			},
			memory:  nil,
			storage: nil,
			wantOk:  true,
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
