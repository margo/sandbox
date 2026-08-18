package deviceconstraints

import (
	"testing"

	clModels "github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func strPtr(s string) *string { return &s }

func archPtr(a clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture) *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture {
	return &a
}

type cpuEntry struct {
	arch  *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture
	cores float32
}

func makeCPUs(entries ...cpuEntry) *[]struct {
	Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
	Cores        float32                                                        `json:"cores"`
} {
	cpus := make([]struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}, len(entries))
	for i, e := range entries {
		cpus[i].Architecture = e.arch
		cpus[i].Cores = e.cores
	}
	return &cpus
}

func makeManifest(cpus *[]struct {
	Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
	Cores        float32                                                        `json:"cores"`
}, memory, storage *string) clModels.DeviceCapabilitiesManifest {
	m := clModels.DeviceCapabilitiesManifest{}
	m.Properties.Cpus = cpus
	m.Properties.Memory = memory
	m.Properties.Storage = storage
	return m
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
		wantOk  bool
		wantErr bool
	}{
		{
			name:    "nil cpus returns error",
			cpus:    nil,
			cores:   4,
			wantErr: true,
		},
		{
			name:   "no arch filter – enough cores",
			cpus:   makeCPUs(cpuEntry{cores: 8}),
			arch:   nil,
			cores:  4,
			wantOk: true,
		},
		{
			name:   "no arch filter – exact cores match",
			cpus:   makeCPUs(cpuEntry{cores: 4}),
			arch:   nil,
			cores:  4,
			wantOk: true,
		},
		{
			name:   "no arch filter – not enough cores",
			cpus:   makeCPUs(cpuEntry{cores: 2}),
			arch:   nil,
			cores:  4,
			wantOk: false,
		},
		{
			name:   "arch match – enough cores",
			cpus:   makeCPUs(cpuEntry{arch: &amd64, cores: 8}),
			arch:   &[]string{"amd64"},
			cores:  4,
			wantOk: true,
		},
		{
			name:   "arch match – not enough cores",
			cpus:   makeCPUs(cpuEntry{arch: &amd64, cores: 2}),
			arch:   &[]string{"amd64"},
			cores:  4,
			wantOk: false,
		},
		{
			name:   "arch mismatch – skipped, returns false",
			cpus:   makeCPUs(cpuEntry{arch: &arm64, cores: 8}),
			arch:   &[]string{"amd64"},
			cores:  4,
			wantOk: false,
		},
		{
			name:   "cpu with nil architecture skipped when arch filter set",
			cpus:   makeCPUs(cpuEntry{arch: nil, cores: 8}),
			arch:   &[]string{"amd64"},
			cores:  4,
			wantOk: false,
		},
		{
			name: "multiple cpus – second satisfies arch and cores",
			cpus: makeCPUs(
				cpuEntry{arch: &amd64, cores: 2},
				cpuEntry{arch: &amd64, cores: 8},
			),
			arch:   &[]string{"amd64"},
			cores:  4,
			wantOk: true,
		},
		{
			name:   "multiple arch filter – one matches",
			cpus:   makeCPUs(cpuEntry{arch: &arm64, cores: 8}),
			arch:   &[]string{"amd64", "arm64"},
			cores:  4,
			wantOk: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			manifest := clModels.DeviceCapabilitiesManifest{}
			manifest.Properties.Cpus = tc.cpus

			dc := NewDeviceCapabilityChecker(manifest)
			ok, err := dc.HasEnoughCPUCores(tc.arch, tc.cores)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "cpu not defined for device")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantOk, ok)
		})
	}
}

// ── HasEnoughMemory ──────────────────────────────────────────────────────────

func TestHasEnoughMemory(t *testing.T) {
	tests := []struct {
		name           string
		deviceMemory   *string
		requiredMemory *string
		wantOk         bool
		wantErr        bool
		errContains    string
	}{
		{
			name:           "nil requirement always passes",
			deviceMemory:   strPtr("4Gi"),
			requiredMemory: nil,
			wantOk:         true,
		},
		{
			name:           "nil device memory returns error",
			deviceMemory:   nil,
			requiredMemory: strPtr("2Gi"),
			wantErr:        true,
			errContains:    "memory not defined for device",
		},
		{
			name:           "device has more than enough memory",
			deviceMemory:   strPtr("8Gi"),
			requiredMemory: strPtr("4Gi"),
			wantOk:         true,
		},
		{
			name:           "device has exact memory",
			deviceMemory:   strPtr("4Gi"),
			requiredMemory: strPtr("4Gi"),
			wantOk:         true,
		},
		{
			name:           "device has insufficient memory",
			deviceMemory:   strPtr("2Gi"),
			requiredMemory: strPtr("4Gi"),
			wantOk:         false,
		},
		{
			name:           "invalid required quantity returns error",
			deviceMemory:   strPtr("8Gi"),
			requiredMemory: strPtr("not-a-quantity"),
			wantErr:        true,
			errContains:    "failed to check memory requirements",
		},
		{
			name:           "invalid device quantity returns error",
			deviceMemory:   strPtr("not-a-quantity"),
			requiredMemory: strPtr("4Gi"),
			wantErr:        true,
			errContains:    "failed to check memory requirements",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			manifest := makeManifest(nil, tc.deviceMemory, nil)
			dc := NewDeviceCapabilityChecker(manifest)
			ok, err := dc.HasEnoughMemory(tc.requiredMemory)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantOk, ok)
		})
	}
}

// ── HasEnoughStorage ─────────────────────────────────────────────────────────

func TestHasEnoughStorage(t *testing.T) {
	tests := []struct {
		name            string
		deviceStorage   *string
		requiredStorage *string
		wantOk          bool
		wantErr         bool
		errContains     string
	}{
		{
			name:            "nil requirement always passes",
			deviceStorage:   strPtr("100Gi"),
			requiredStorage: nil,
			wantOk:          true,
		},
		{
			name:            "nil device storage returns error",
			deviceStorage:   nil,
			requiredStorage: strPtr("50Gi"),
			wantErr:         true,
			errContains:     "storage not defined for device",
		},
		{
			name:            "device has more than enough storage",
			deviceStorage:   strPtr("200Gi"),
			requiredStorage: strPtr("100Gi"),
			wantOk:          true,
		},
		{
			name:            "device has exact storage",
			deviceStorage:   strPtr("100Gi"),
			requiredStorage: strPtr("100Gi"),
			wantOk:          true,
		},
		{
			name:            "device has insufficient storage",
			deviceStorage:   strPtr("50Gi"),
			requiredStorage: strPtr("100Gi"),
			wantOk:          false,
		},
		{
			name:            "invalid required quantity returns error",
			deviceStorage:   strPtr("200Gi"),
			requiredStorage: strPtr("bad-value"),
			wantErr:         true,
			errContains:     "failed to check storage requirements",
		},
		{
			name:            "invalid device quantity returns error",
			deviceStorage:   strPtr("bad-value"),
			requiredStorage: strPtr("100Gi"),
			wantErr:         true,
			errContains:     "failed to check storage requirements",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			manifest := makeManifest(nil, nil, tc.deviceStorage)
			dc := NewDeviceCapabilityChecker(manifest)
			ok, err := dc.HasEnoughStorage(tc.requiredStorage)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantOk, ok)
		})
	}
}

// ── satisfies (internal) ─────────────────────────────────────────────────────

func TestSatisfies(t *testing.T) {
	tests := []struct {
		name     string
		required string
		actual   string
		wantOk   bool
		wantErr  bool
	}{
		{"actual equals required", "4Gi", "4Gi", true, false},
		{"actual exceeds required", "4Gi", "8Gi", true, false},
		{"actual less than required", "8Gi", "4Gi", false, false},
		{"works with Mi units", "512Mi", "1Gi", true, false},
		{"invalid required quantity", "bad", "4Gi", false, true},
		{"invalid actual quantity", "4Gi", "bad", false, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := satisfies(tc.required, tc.actual)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantOk, ok)
		})
	}
}
