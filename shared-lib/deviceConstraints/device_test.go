package deviceconstraints

import (
	"testing"

	clModels "github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helpers

func ptr[T any](v T) *T { return &v }

func makeDevice(
	arch *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture,
	cores float32,
	memory, storage *string,
) clModels.DeviceCapabilitiesManifest {
	cpu := struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		Architecture: arch,
		Cores:        cores,
	}
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
			Cpus: &[]struct {
				Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
				Cores        float32                                                        `json:"cores"`
			}{cpu},
			Memory:       memory,
			Storage:      storage,
			Id:           "device-1",
			ModelNumber:  "model-x",
			SerialNumber: "sn-001",
			Vendor:       "acme",
		},
	}
}

// --- Tests ---

func TestCheckEligibility_NilRequirements_AlwaysPasses(t *testing.T) {
	device := makeDevice(
		ptr(clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64),
		4, ptr("4Gi"), ptr("100Gi"),
	)
	checker := NewDeviceCapabilityChecker(device)

	ok, reason, err := checker.CheckEligibility(nil)

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestCheckEligibility_AllRequirementsMet(t *testing.T) {
	device := makeDevice(
		ptr(clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64),
		8, ptr("16Gi"), ptr("500Gi"),
	)
	checker := NewDeviceCapabilityChecker(device)

	archs := []clModels.DeploymentCpuRequirementArchitectures{
		clModels.DeploymentCpuRequirementArchitecturesAmd64,
	}
	checks := &clModels.CapacityRequirements{
		Cpu: &clModels.DeploymentCpuRequirement{
			Architectures: &archs,
			Cores:         4,
		},
		Memory:  ptr("8Gi"),
		Storage: ptr("200Gi"),
	}

	ok, reason, err := checker.CheckEligibility(checks)

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestCheckEligibility_CPUCoresInsufficient(t *testing.T) {
	device := makeDevice(
		ptr(clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64),
		2, ptr("16Gi"), ptr("500Gi"),
	)
	checker := NewDeviceCapabilityChecker(device)

	archs := []clModels.DeploymentCpuRequirementArchitectures{
		clModels.DeploymentCpuRequirementArchitecturesAmd64,
	}
	checks := &clModels.CapacityRequirements{
		Cpu: &clModels.DeploymentCpuRequirement{
			Architectures: &archs,
			Cores:         4,
		},
	}

	ok, reason, err := checker.CheckEligibility(checks)

	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, "cpu requirement not fulfilled", reason)
}

func TestCheckEligibility_CPUArchitectureMismatch(t *testing.T) {
	device := makeDevice(
		ptr(clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureArm64),
		8, ptr("16Gi"), ptr("500Gi"),
	)
	checker := NewDeviceCapabilityChecker(device)

	archs := []clModels.DeploymentCpuRequirementArchitectures{
		clModels.DeploymentCpuRequirementArchitecturesAmd64,
	}
	checks := &clModels.CapacityRequirements{
		Cpu: &clModels.DeploymentCpuRequirement{
			Architectures: &archs,
			Cores:         4,
		},
	}

	ok, reason, err := checker.CheckEligibility(checks)

	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, "cpu requirement not fulfilled", reason)
}

func TestCheckEligibility_NoCPUArchitectureFilter_AnyArchitecturePasses(t *testing.T) {
	device := makeDevice(
		ptr(clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureArm64),
		8, ptr("16Gi"), ptr("500Gi"),
	)
	checker := NewDeviceCapabilityChecker(device)

	checks := &clModels.CapacityRequirements{
		Cpu: &clModels.DeploymentCpuRequirement{
			Architectures: nil, // no arch filter
			Cores:         4,
		},
	}

	ok, reason, err := checker.CheckEligibility(checks)

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestCheckEligibility_MemoryInsufficient(t *testing.T) {
	device := makeDevice(
		ptr(clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64),
		8, ptr("4Gi"), ptr("500Gi"),
	)
	checker := NewDeviceCapabilityChecker(device)

	archs := []clModels.DeploymentCpuRequirementArchitectures{
		clModels.DeploymentCpuRequirementArchitecturesAmd64,
	}
	checks := &clModels.CapacityRequirements{
		Cpu: &clModels.DeploymentCpuRequirement{
			Architectures: &archs,
			Cores:         4,
		},
		Memory: ptr("16Gi"),
	}

	ok, reason, err := checker.CheckEligibility(checks)

	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, "mem requirement not fulfilled", reason)
}

func TestCheckEligibility_StorageInsufficient(t *testing.T) {
	device := makeDevice(
		ptr(clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64),
		8, ptr("16Gi"), ptr("100Gi"),
	)
	checker := NewDeviceCapabilityChecker(device)

	archs := []clModels.DeploymentCpuRequirementArchitectures{
		clModels.DeploymentCpuRequirementArchitecturesAmd64,
	}
	checks := &clModels.CapacityRequirements{
		Cpu: &clModels.DeploymentCpuRequirement{
			Architectures: &archs,
			Cores:         4,
		},
		Memory:  ptr("8Gi"),
		Storage: ptr("500Gi"),
	}

	ok, reason, err := checker.CheckEligibility(checks)

	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, "storage requirement not fulfilled", reason)
}

func TestCheckEligibility_OnlyCPURequirement_Passes(t *testing.T) {
	device := makeDevice(
		ptr(clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64),
		8, nil, nil,
	)
	checker := NewDeviceCapabilityChecker(device)

	archs := []clModels.DeploymentCpuRequirementArchitectures{
		clModels.DeploymentCpuRequirementArchitecturesAmd64,
	}
	checks := &clModels.CapacityRequirements{
		Cpu: &clModels.DeploymentCpuRequirement{
			Architectures: &archs,
			Cores:         4,
		},
	}

	ok, reason, err := checker.CheckEligibility(checks)

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestCheckEligibility_OnlyMemoryRequirement_Passes(t *testing.T) {
	device := makeDevice(nil, 0, ptr("32Gi"), nil)
	// Remove CPUs from device to test memory-only path
	device.Properties.Cpus = nil
	checker := NewDeviceCapabilityChecker(device)

	checks := &clModels.CapacityRequirements{
		Memory: ptr("16Gi"),
	}

	ok, reason, err := checker.CheckEligibility(checks)

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestCheckEligibility_OnlyStorageRequirement_Passes(t *testing.T) {
	device := makeDevice(nil, 0, nil, ptr("1Ti"))
	device.Properties.Cpus = nil
	checker := NewDeviceCapabilityChecker(device)

	checks := &clModels.CapacityRequirements{
		Storage: ptr("500Gi"),
	}

	ok, reason, err := checker.CheckEligibility(checks)

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestCheckEligibility_CPUNotDefinedOnDevice_ReturnsError(t *testing.T) {
	device := makeDevice(nil, 0, ptr("16Gi"), ptr("500Gi"))
	device.Properties.Cpus = nil
	checker := NewDeviceCapabilityChecker(device)

	archs := []clModels.DeploymentCpuRequirementArchitectures{
		clModels.DeploymentCpuRequirementArchitecturesAmd64,
	}
	checks := &clModels.CapacityRequirements{
		Cpu: &clModels.DeploymentCpuRequirement{
			Architectures: &archs,
			Cores:         4,
		},
	}

	ok, _, err := checker.CheckEligibility(checks)

	require.Error(t, err)
	assert.False(t, ok)
}

func TestCheckEligibility_MemoryNotDefinedOnDevice_ReturnsError(t *testing.T) {
	device := makeDevice(
		ptr(clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64),
		8, nil, ptr("500Gi"),
	)
	checker := NewDeviceCapabilityChecker(device)

	archs := []clModels.DeploymentCpuRequirementArchitectures{
		clModels.DeploymentCpuRequirementArchitecturesAmd64,
	}
	checks := &clModels.CapacityRequirements{
		Cpu: &clModels.DeploymentCpuRequirement{
			Architectures: &archs,
			Cores:         4,
		},
		Memory: ptr("8Gi"),
	}

	ok, _, err := checker.CheckEligibility(checks)

	require.Error(t, err)
	assert.False(t, ok)
}

func TestCheckEligibility_StorageNotDefinedOnDevice_ReturnsError(t *testing.T) {
	device := makeDevice(
		ptr(clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64),
		8, ptr("16Gi"), nil,
	)
	checker := NewDeviceCapabilityChecker(device)

	archs := []clModels.DeploymentCpuRequirementArchitectures{
		clModels.DeploymentCpuRequirementArchitecturesAmd64,
	}
	checks := &clModels.CapacityRequirements{
		Cpu: &clModels.DeploymentCpuRequirement{
			Architectures: &archs,
			Cores:         4,
		},
		Memory:  ptr("8Gi"),
		Storage: ptr("200Gi"),
	}

	ok, _, err := checker.CheckEligibility(checks)

	require.Error(t, err)
	assert.False(t, ok)
}

func TestCheckEligibility_InvalidMemoryQuantity_ReturnsError(t *testing.T) {
	device := makeDevice(
		ptr(clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64),
		8, ptr("not-a-quantity"), nil,
	)
	checker := NewDeviceCapabilityChecker(device)

	archs := []clModels.DeploymentCpuRequirementArchitectures{
		clModels.DeploymentCpuRequirementArchitecturesAmd64,
	}
	checks := &clModels.CapacityRequirements{
		Cpu: &clModels.DeploymentCpuRequirement{
			Architectures: &archs,
			Cores:         4,
		},
		Memory: ptr("8Gi"),
	}

	ok, _, err := checker.CheckEligibility(checks)

	require.Error(t, err)
	assert.False(t, ok)
}

func TestCheckEligibility_ExactCPUCoresMatch_Passes(t *testing.T) {
	device := makeDevice(
		ptr(clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64),
		4, ptr("8Gi"), ptr("200Gi"),
	)
	checker := NewDeviceCapabilityChecker(device)

	archs := []clModels.DeploymentCpuRequirementArchitectures{
		clModels.DeploymentCpuRequirementArchitecturesAmd64,
	}
	checks := &clModels.CapacityRequirements{
		Cpu: &clModels.DeploymentCpuRequirement{
			Architectures: &archs,
			Cores:         4, // exact match
		},
	}

	ok, reason, err := checker.CheckEligibility(checks)

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestCheckEligibility_MultipleArchitectures_MatchingOneIsSufficient(t *testing.T) {
	device := makeDevice(
		ptr(clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureArm64),
		8, ptr("16Gi"), ptr("500Gi"),
	)
	checker := NewDeviceCapabilityChecker(device)

	archs := []clModels.DeploymentCpuRequirementArchitectures{
		clModels.DeploymentCpuRequirementArchitecturesAmd64,
		clModels.DeploymentCpuRequirementArchitecturesArm64,
	}
	checks := &clModels.CapacityRequirements{
		Cpu: &clModels.DeploymentCpuRequirement{
			Architectures: &archs,
			Cores:         4,
		},
	}

	ok, reason, err := checker.CheckEligibility(checks)

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestCheckEligibility_EmptyCapacityRequirements_Passes(t *testing.T) {
	device := makeDevice(
		ptr(clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64),
		4, ptr("8Gi"), ptr("200Gi"),
	)
	checker := NewDeviceCapabilityChecker(device)

	// All fields nil — no constraints
	checks := &clModels.CapacityRequirements{}

	ok, reason, err := checker.CheckEligibility(checks)

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// helpers

func strPtr(s string) *string { return &s }

func makeCPU(arch *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture, cores float32) struct {
	Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
	Cores        float32                                                        `json:"cores"`
} {
	return struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{Architecture: arch, Cores: cores}
}

func makeChecker(manifest clModels.DeviceCapabilitiesManifest) DeviceCheckerIface {
	return NewDeviceCapabilityChecker(manifest)
}

// ── HasEnoughCPUCores ────────────────────────────────────────────────────────

func TestHasEnoughCPUCores(t *testing.T) {
	amd64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	arm64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureArm64

	tests := []struct {
		name string
		cpus *[]struct {
			Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
			Cores        float32                                                        `json:"cores"`
		}
		arch    *[]string
		cores   float32
		want    bool
		wantErr bool
	}{
		{
			name:    "nil cpus returns error",
			cpus:    nil,
			arch:    nil,
			cores:   4,
			want:    false,
			wantErr: true,
		},
		{
			name: "no arch filter — sufficient cores",
			cpus: &[]struct {
				Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
				Cores        float32                                                        `json:"cores"`
			}{makeCPU(&amd64, 8)},
			arch:    nil,
			cores:   4,
			want:    true,
			wantErr: false,
		},
		{
			name: "no arch filter — exact cores match",
			cpus: &[]struct {
				Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
				Cores        float32                                                        `json:"cores"`
			}{makeCPU(&amd64, 4)},
			arch:    nil,
			cores:   4,
			want:    true,
			wantErr: false,
		},
		{
			name: "no arch filter — insufficient cores",
			cpus: &[]struct {
				Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
				Cores        float32                                                        `json:"cores"`
			}{makeCPU(&amd64, 2)},
			arch:    nil,
			cores:   4,
			want:    false,
			wantErr: false,
		},
		{
			name: "arch filter matches — sufficient cores",
			cpus: &[]struct {
				Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
				Cores        float32                                                        `json:"cores"`
			}{makeCPU(&amd64, 8)},
			arch:    &[]string{"amd64"},
			cores:   4,
			want:    true,
			wantErr: false,
		},
		{
			name: "arch filter does not match — returns false",
			cpus: &[]struct {
				Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
				Cores        float32                                                        `json:"cores"`
			}{makeCPU(&amd64, 8)},
			arch:    &[]string{"arm64"},
			cores:   4,
			want:    false,
			wantErr: false,
		},
		{
			name: "cpu with nil architecture is skipped when arch filter set",
			cpus: &[]struct {
				Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
				Cores        float32                                                        `json:"cores"`
			}{makeCPU(nil, 8)},
			arch:    &[]string{"amd64"},
			cores:   4,
			want:    false,
			wantErr: false,
		},
		{
			name: "multiple cpus — second one satisfies arch and cores",
			cpus: &[]struct {
				Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
				Cores        float32                                                        `json:"cores"`
			}{
				makeCPU(&amd64, 2),
				makeCPU(&arm64, 8),
			},
			arch:    &[]string{"arm64"},
			cores:   4,
			want:    true,
			wantErr: false,
		},
		{
			name: "arch filter matches but cores insufficient",
			cpus: &[]struct {
				Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
				Cores        float32                                                        `json:"cores"`
			}{makeCPU(&amd64, 1)},
			arch:    &[]string{"amd64"},
			cores:   4,
			want:    false,
			wantErr: false,
		},
		{
			name: "multiple accepted architectures — first matches",
			cpus: &[]struct {
				Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
				Cores        float32                                                        `json:"cores"`
			}{makeCPU(&amd64, 6)},
			arch:    &[]string{"amd64", "arm64"},
			cores:   4,
			want:    true,
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			manifest := clModels.DeviceCapabilitiesManifest{}
			manifest.Properties.Cpus = tc.cpus

			checker := makeChecker(manifest)
			got, err := checker.HasEnoughCPUCores(tc.arch, tc.cores)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

// ── HasEnoughMemory ──────────────────────────────────────────────────────────

func TestHasEnoughMemory(t *testing.T) {
	tests := []struct {
		name      string
		deviceMem *string // nil means not set on device
		required  *string
		want      bool
		wantErr   bool
	}{
		{
			name:      "nil requirement always passes",
			deviceMem: strPtr("4Gi"),
			required:  nil,
			want:      true,
			wantErr:   false,
		},
		{
			name:      "device memory not defined returns error",
			deviceMem: nil,
			required:  strPtr("2Gi"),
			want:      false,
			wantErr:   true,
		},
		{
			name:      "device meets exact requirement",
			deviceMem: strPtr("4Gi"),
			required:  strPtr("4Gi"),
			want:      true,
			wantErr:   false,
		},
		{
			name:      "device exceeds requirement",
			deviceMem: strPtr("8Gi"),
			required:  strPtr("4Gi"),
			want:      true,
			wantErr:   false,
		},
		{
			name:      "device below requirement",
			deviceMem: strPtr("2Gi"),
			required:  strPtr("4Gi"),
			want:      false,
			wantErr:   false,
		},
		{
			name:      "mebibyte units — sufficient",
			deviceMem: strPtr("512Mi"),
			required:  strPtr("256Mi"),
			want:      true,
			wantErr:   false,
		},
		{
			name:      "invalid required quantity returns error",
			deviceMem: strPtr("4Gi"),
			required:  strPtr("not-a-quantity"),
			want:      false,
			wantErr:   true,
		},
		{
			name:      "invalid device quantity returns error",
			deviceMem: strPtr("bad-value"),
			required:  strPtr("4Gi"),
			want:      false,
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			manifest := clModels.DeviceCapabilitiesManifest{}
			manifest.Properties.Memory = tc.deviceMem

			checker := makeChecker(manifest)
			got, err := checker.HasEnoughMemory(tc.required)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

// ── HasEnoughStorage ─────────────────────────────────────────────────────────

func TestHasEnoughStorage(t *testing.T) {
	tests := []struct {
		name          string
		deviceStorage *string
		required      *string
		want          bool
		wantErr       bool
	}{
		{
			name:          "nil requirement always passes",
			deviceStorage: strPtr("100Gi"),
			required:      nil,
			want:          true,
			wantErr:       false,
		},
		{
			name:          "device storage not defined returns error",
			deviceStorage: nil,
			required:      strPtr("50Gi"),
			want:          false,
			wantErr:       true,
		},
		{
			name:          "device meets exact requirement",
			deviceStorage: strPtr("100Gi"),
			required:      strPtr("100Gi"),
			want:          true,
			wantErr:       false,
		},
		{
			name:          "device exceeds requirement",
			deviceStorage: strPtr("500Gi"),
			required:      strPtr("100Gi"),
			want:          true,
			wantErr:       false,
		},
		{
			name:          "device below requirement",
			deviceStorage: strPtr("20Gi"),
			required:      strPtr("100Gi"),
			want:          false,
			wantErr:       false,
		},
		{
			name:          "tebibyte units — sufficient",
			deviceStorage: strPtr("2Ti"),
			required:      strPtr("1Ti"),
			want:          true,
			wantErr:       false,
		},
		{
			name:          "invalid required quantity returns error",
			deviceStorage: strPtr("100Gi"),
			required:      strPtr("not-a-quantity"),
			want:          false,
			wantErr:       true,
		},
		{
			name:          "invalid device quantity returns error",
			deviceStorage: strPtr("bad-value"),
			required:      strPtr("100Gi"),
			want:          false,
			wantErr:       true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			manifest := clModels.DeviceCapabilitiesManifest{}
			manifest.Properties.Storage = tc.deviceStorage

			checker := makeChecker(manifest)
			got, err := checker.HasEnoughStorage(tc.required)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.want, got)
		})
	}
}
