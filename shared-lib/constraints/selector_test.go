package constraints

import (
	"encoding/json"
	"testing"

	"github.com/margo/sandbox/shared-lib/constraints/common"
	"github.com/margo/sandbox/shared-lib/pointers"
	clModels "github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helper / builder functions
// ---------------------------------------------------------------------------

// buildDevice constructs a minimal DeviceCapabilitiesManifest with the given
// identity fields. All optional fields are left nil.
func buildDevice(
	id, vendor, modelNumber, serialNumber string,
) *clModels.DeviceCapabilitiesManifest {
	d := &clModels.DeviceCapabilitiesManifest{
		ApiVersion: "margo.org/v1",
		Kind:       clModels.DeviceCapabilitiesManifestKindDeviceCapabilitiesManifest,
	}
	d.Properties.Id = id
	d.Properties.Vendor = vendor
	d.Properties.ModelNumber = modelNumber
	d.Properties.SerialNumber = serialNumber
	return d
}

// withCPU appends a CPU entry to the device's Cpus slice.
func withCPU(
	device *clModels.DeviceCapabilitiesManifest,
	cores float32,
	arch clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture,
) *clModels.DeviceCapabilitiesManifest {
	cpu := struct {
		Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
		Cores        float32                                                        `json:"cores"`
	}{
		Architecture: &arch,
		Cores:        cores,
	}
	if device.Properties.Cpus == nil {
		device.Properties.Cpus = &[]struct {
			Architecture *clModels.DeviceCapabilitiesManifestPropertiesCpusArchitecture `json:"architecture,omitempty"`
			Cores        float32                                                        `json:"cores"`
		}{}
	}
	*device.Properties.Cpus = append(*device.Properties.Cpus, cpu)
	return device
}

// withMemory sets the device's memory field.
func withMemory(
	device *clModels.DeviceCapabilitiesManifest,
	memory string,
) *clModels.DeviceCapabilitiesManifest {
	device.Properties.Memory = &memory
	return device
}

// withStorage sets the device's storage field.
func withStorage(
	device *clModels.DeviceCapabilitiesManifest,
	storage string,
) *clModels.DeviceCapabilitiesManifest {
	device.Properties.Storage = &storage
	return device
}

// withLabels converts a map[string]interface{} into the generated union-type
// label map and attaches it to the device.
func withLabels(
	device *clModels.DeviceCapabilitiesManifest,
	labels map[string]interface{},
) *clModels.DeviceCapabilitiesManifest {
	result := make(
		map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties,
		len(labels),
	)
	for k, v := range labels {
		raw, err := json.Marshal(v)
		if err != nil {
			panic("withLabels: failed to marshal label value: " + err.Error())
		}
		var prop clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties
		if err := json.Unmarshal(raw, &prop); err != nil {
			panic("withLabels: failed to unmarshal label value: " + err.Error())
		}
		result[k] = prop
	}
	device.Labels = &result
	return device
}

// buildConstraints constructs a DeviceConstraints with the given capacity
// requirements and eligibility rules. Either argument may be nil.
func buildConstraints(
	cap *clModels.CapacityRequirements,
	rules []clModels.EligibilityRule,
) *clModels.DeviceConstraints {
	dc := &clModels.DeviceConstraints{
		CapacityRequirements: cap,
	}
	if rules != nil {
		dc.EligibilityRules = &rules
	}
	return dc
}

// buildLabelSelector wraps a slice of MatchExpressions in a Selector.
func buildLabelSelector(expressions []clModels.MatchExpression) *clModels.Selector {
	return &clModels.Selector{MatchExpressions: expressions}
}

// buildPropertySelector wraps a slice of MatchExpressions in a Selector.
func buildPropertySelector(expressions []clModels.MatchExpression) *clModels.Selector {
	return &clModels.Selector{MatchExpressions: expressions}
}

// buildMatchExpression constructs a MatchExpression. Pass nil for values when
// the operator does not require them (e.g. Exists / DoesNotExist).
func buildMatchExpression(
	key string,
	op clModels.MatchExpressionOperator,
	values []any,
) clModels.MatchExpression {
	me := clModels.MatchExpression{
		Key:      key,
		Operator: op,
	}
	if values != nil {
		me.Values = &values
	}
	return me
}

// newTestSelector returns a DeviceSelectorIface backed by a no-op logger,
// suitable for use in all unit tests in this file.
func newTestSelector() common.DeviceSelectorIface {
	return New()
}

// ---------------------------------------------------------------------------
// TestIsDeviceEligible_NilAndZeroValueCases
// ---------------------------------------------------------------------------

func TestIsDeviceEligible_NilAndZeroValueCases(t *testing.T) {
	sel := newTestSelector()

	tests := []struct {
		name            string
		device          *clModels.DeviceCapabilitiesManifest
		checks          *clModels.DeviceConstraints
		wantEligible    bool
		wantReasonEmpty bool
		wantErr         bool
	}{
		{
			name:            "nil checks → device is always eligible",
			device:          buildDevice("dev-001", "Stark Industries", "MK-I", "SN-0001"),
			checks:          nil,
			wantEligible:    true,
			wantReasonEmpty: true,
			wantErr:         false,
		},
		{
			name:   "non-nil checks with nil CapacityRequirements and nil EligibilityRules → eligible",
			device: buildDevice("dev-002", "Stark Industries", "MK-II", "SN-0002"),
			checks: &clModels.DeviceConstraints{
				CapacityRequirements: nil,
				EligibilityRules:     nil,
			},
			wantEligible:    true,
			wantReasonEmpty: true,
			wantErr:         false,
		},
		{
			name:   "non-nil checks with empty EligibilityRules slice → not eligible",
			device: buildDevice("dev-003", "Stark Industries", "MK-III", "SN-0003"),
			checks: buildConstraints(nil, []clModels.EligibilityRule{}),
			// No rule can pass when the slice is empty; the loop body never executes
			// so the function falls through to return false.
			wantEligible:    false,
			wantReasonEmpty: false, // reason may be empty string but eligibility is false
			wantErr:         false,
		},
		{
			name:            "device with no optional properties set and nil checks → eligible",
			device:          buildDevice("dev-004", "Stark Industries", "MK-IV", "SN-0004"),
			checks:          nil,
			wantEligible:    true,
			wantReasonEmpty: true,
			wantErr:         false,
		},
	}

	for _, tc := range tests {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			eligible, reason, err := sel.IsDeviceEligible(tc.device, tc.checks)

			if tc.wantErr {
				require.Error(t, err)
				assert.False(t, eligible)
				assert.Empty(t, reason)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantEligible, eligible)

			if tc.wantReasonEmpty {
				assert.Empty(t, reason)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestIsDeviceEligible_CapacityRequirements_CPU
// ---------------------------------------------------------------------------

func TestIsDeviceEligible_CapacityRequirements_CPU(t *testing.T) {
	sel := newTestSelector()

	amd64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	arm64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureArm64

	reqAmd64 := clModels.DeploymentCpuRequirementArchitecturesAmd64
	reqArm64 := clModels.DeploymentCpuRequirementArchitecturesArm64

	tests := []struct {
		name         string
		device       *clModels.DeviceCapabilitiesManifest
		checks       *clModels.DeviceConstraints
		wantEligible bool
		wantErr      bool
	}{
		{
			name: "device has exactly required CPU cores → eligible",
			device: withCPU(
				buildDevice("dev-101", "Stark Industries", "MK-I", "SN-0101"),
				4,
				amd64,
			),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Cpu: &clModels.DeploymentCpuRequirement{Cores: 4},
			}, nil),
			wantEligible: true,
		},
		{
			name: "device has more than required CPU cores → eligible",
			device: withCPU(
				buildDevice("dev-102", "Stark Industries", "MK-II", "SN-0102"),
				8,
				amd64,
			),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Cpu: &clModels.DeploymentCpuRequirement{Cores: 4},
			}, nil),
			wantEligible: true,
		},
		{
			name: "device has less than required CPU cores → not eligible",
			device: withCPU(
				buildDevice("dev-103", "Stark Industries", "MK-III", "SN-0103"),
				2,
				amd64,
			),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Cpu: &clModels.DeploymentCpuRequirement{Cores: 4},
			}, nil),
			wantEligible: false,
		},
		{
			name: "required architecture amd64, device has amd64 → eligible",
			device: withCPU(
				buildDevice("dev-104", "Stark Industries", "MK-IV", "SN-0104"),
				4,
				amd64,
			),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Cpu: &clModels.DeploymentCpuRequirement{
					Cores:         4,
					Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{reqAmd64},
				},
			}, nil),
			wantEligible: true,
		},
		{
			name: "required architecture arm64, device has amd64 → not eligible",
			device: withCPU(
				buildDevice("dev-105", "Stark Industries", "MK-V", "SN-0105"),
				4,
				amd64,
			),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Cpu: &clModels.DeploymentCpuRequirement{
					Cores:         4,
					Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{reqArm64},
				},
			}, nil),
			wantEligible: false,
		},
		{
			name: "required architecture arm64, device has arm64 → eligible",
			device: withCPU(
				buildDevice("dev-106", "Stark Industries", "MK-VI", "SN-0106"),
				4,
				arm64,
			),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Cpu: &clModels.DeploymentCpuRequirement{
					Cores:         4,
					Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{reqArm64},
				},
			}, nil),
			wantEligible: true,
		},
		{
			name: "multiple architectures required [amd64, arm64], device has amd64 → eligible",
			device: withCPU(
				buildDevice("dev-107", "Stark Industries", "MK-VII", "SN-0107"),
				4,
				amd64,
			),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Cpu: &clModels.DeploymentCpuRequirement{
					Cores: 4,
					Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{
						reqAmd64,
						reqArm64,
					},
				},
			}, nil),
			wantEligible: true,
		},
		{
			name: "CapacityRequirements.Cpu is nil → eligible regardless of device CPU",
			device: withCPU(
				buildDevice("dev-108", "Stark Industries", "MK-VIII", "SN-0108"),
				1,
				amd64,
			),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Cpu: nil,
			}, nil),
			wantEligible: true,
		},
		{
			name:   "device has no CPU defined, CPU requirement is set → not eligible",
			device: buildDevice("dev-109", "Stark Industries", "MK-IX", "SN-0109"),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Cpu: &clModels.DeploymentCpuRequirement{Cores: 4},
			}, nil),
			wantEligible: false,
		},
		{
			name: "device has multiple CPU entries summing above requirement → not eligible",
			device: withCPU(
				withCPU(buildDevice("dev-110", "Stark Industries", "MK-X", "SN-0110"), 2, amd64),
				2, amd64,
			),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Cpu: &clModels.DeploymentCpuRequirement{Cores: 4},
			}, nil),
			wantEligible: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			eligible, reason, err := sel.IsDeviceEligible(tc.device, tc.checks)

			if tc.wantErr {
				require.Error(t, err)
				assert.False(t, eligible)
				assert.Empty(t, reason)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantEligible, eligible)

			if !tc.wantEligible {
				assert.NotEmpty(t, reason, "expected a non-empty reason for ineligible device")
			} else {
				assert.Empty(t, reason)
			}
		})
	}
}

func TestIsDeviceEligible_CapacityRequirements_MemoryAndStorage(t *testing.T) {
	sel := newTestSelector()

	tests := []struct {
		name         string
		device       *clModels.DeviceCapabilitiesManifest
		checks       *clModels.DeviceConstraints
		wantEligible bool
		wantErr      bool
	}{
		// ── Memory cases ────────────────────────────────────────────────────────
		{
			name:   "device memory == required memory (512Mi) → eligible",
			device: withMemory(buildDevice("dev-201", "Acme", "M-I", "SN-0201"), "512Mi"),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Memory: pointers.Ptr("512Mi"),
			}, nil),
			wantEligible: true,
		},
		{
			name:   "device memory (1Gi) > required memory (512Mi) → eligible",
			device: withMemory(buildDevice("dev-202", "Acme", "M-II", "SN-0202"), "1Gi"),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Memory: pointers.Ptr("512Mi"),
			}, nil),
			wantEligible: true,
		},
		{
			name:   "device memory (256Mi) < required memory (512Mi) → not eligible",
			device: withMemory(buildDevice("dev-203", "Acme", "M-III", "SN-0203"), "256Mi"),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Memory: pointers.Ptr("512Mi"),
			}, nil),
			wantEligible: false,
		},
		{
			name:   "CapacityRequirements.Memory is nil → no memory constraint, eligible",
			device: withMemory(buildDevice("dev-204", "Acme", "M-IV", "SN-0204"), "128Mi"),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Memory: nil,
			}, nil),
			wantEligible: true,
		},
		{
			name:   "device has no memory field set, memory requirement present → not eligible",
			device: buildDevice("dev-205", "Acme", "M-V", "SN-0205"),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Memory: pointers.Ptr("512Mi"),
			}, nil),
			wantEligible: false,
		},

		// ── Storage cases ────────────────────────────────────────────────────────
		{
			name:   "device storage == required storage (10Gi) → eligible",
			device: withStorage(buildDevice("dev-206", "Acme", "S-I", "SN-0206"), "10Gi"),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Storage: pointers.Ptr("10Gi"),
			}, nil),
			wantEligible: true,
		},
		{
			name:   "device storage (20Gi) > required storage (10Gi) → eligible",
			device: withStorage(buildDevice("dev-207", "Acme", "S-II", "SN-0207"), "20Gi"),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Storage: pointers.Ptr("10Gi"),
			}, nil),
			wantEligible: true,
		},
		{
			name:   "device storage (5Gi) < required storage (10Gi) → not eligible",
			device: withStorage(buildDevice("dev-208", "Acme", "S-III", "SN-0208"), "5Gi"),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Storage: pointers.Ptr("10Gi"),
			}, nil),
			wantEligible: false,
		},
		{
			name:   "CapacityRequirements.Storage is nil → no storage constraint, eligible",
			device: withStorage(buildDevice("dev-209", "Acme", "S-IV", "SN-0209"), "1Ti"),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Storage: nil,
			}, nil),
			wantEligible: true,
		},
		{
			name:   "device has no storage field set, storage requirement present → not eligible",
			device: buildDevice("dev-210", "Acme", "S-V", "SN-0210"),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Storage: pointers.Ptr("10Gi"),
			}, nil),
			wantEligible: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			eligible, reason, err := sel.IsDeviceEligible(tc.device, tc.checks)

			if tc.wantErr {
				require.Error(t, err)
				assert.False(t, eligible)
				assert.Empty(t, reason)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantEligible, eligible)

			if !tc.wantEligible {
				assert.NotEmpty(t, reason, "expected a non-empty reason for ineligible device")
			} else {
				assert.Empty(t, reason)
			}
		})
	}
}

func TestIsDeviceEligible_CapacityRequirements_Combined(t *testing.T) {
	sel := newTestSelector()

	amd64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	reqAmd64 := clModels.DeploymentCpuRequirementArchitecturesAmd64

	// Helper: build a label-based EligibilityRule using Exists operator on key "env"
	existsRule := func(key string) clModels.EligibilityRule {
		return clModels.EligibilityRule{
			LabelSelector: buildLabelSelector([]clModels.MatchExpression{
				buildMatchExpression(key, clModels.Exists, nil),
			}),
		}
	}

	tests := []struct {
		name         string
		device       *clModels.DeviceCapabilitiesManifest
		checks       *clModels.DeviceConstraints
		wantEligible bool
		wantErr      bool
	}{
		// ── Combined capacity cases ──────────────────────────────────────────────
		{
			// 1. All three (CPU, memory, storage) satisfied → eligible
			name: "all three capacity requirements satisfied → eligible",
			device: withStorage(
				withMemory(
					withCPU(buildDevice("dev-401", "Acme", "C-I", "SN-0401"), 4, amd64),
					"2Gi",
				),
				"20Gi",
			),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Cpu: &clModels.DeploymentCpuRequirement{
					Cores:         4,
					Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{reqAmd64},
				},
				Memory:  pointers.Ptr("2Gi"),
				Storage: pointers.Ptr("20Gi"),
			}, nil),
			wantEligible: true,
		},
		{
			// 2. CPU satisfied, memory not satisfied → not eligible
			name: "CPU satisfied, memory not satisfied → not eligible",
			device: withStorage(
				withMemory(
					withCPU(buildDevice("dev-402", "Acme", "C-II", "SN-0402"), 4, amd64),
					"256Mi",
				),
				"20Gi",
			),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Cpu: &clModels.DeploymentCpuRequirement{
					Cores: 4,
				},
				Memory:  pointers.Ptr("512Mi"),
				Storage: pointers.Ptr("20Gi"),
			}, nil),
			wantEligible: false,
		},
		{
			// 3. CPU satisfied, storage not satisfied → not eligible
			name: "CPU satisfied, storage not satisfied → not eligible",
			device: withStorage(
				withMemory(
					withCPU(buildDevice("dev-403", "Acme", "C-III", "SN-0403"), 4, amd64),
					"2Gi",
				),
				"5Gi",
			),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Cpu: &clModels.DeploymentCpuRequirement{
					Cores: 4,
				},
				Memory:  pointers.Ptr("2Gi"),
				Storage: pointers.Ptr("10Gi"),
			}, nil),
			wantEligible: false,
		},
		{
			// 4. Memory satisfied, CPU not satisfied → not eligible
			name: "memory satisfied, CPU not satisfied → not eligible",
			device: withStorage(
				withMemory(
					withCPU(buildDevice("dev-404", "Acme", "C-IV", "SN-0404"), 2, amd64),
					"2Gi",
				),
				"20Gi",
			),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Cpu: &clModels.DeploymentCpuRequirement{
					Cores: 4,
				},
				Memory:  pointers.Ptr("2Gi"),
				Storage: pointers.Ptr("20Gi"),
			}, nil),
			wantEligible: false,
		},
		{
			// 5. All three not satisfied → not eligible
			name: "all three capacity requirements not satisfied → not eligible",
			device: withStorage(
				withMemory(
					withCPU(buildDevice("dev-405", "Acme", "C-V", "SN-0405"), 1, amd64),
					"128Mi",
				),
				"1Gi",
			),
			checks: buildConstraints(&clModels.CapacityRequirements{
				Cpu: &clModels.DeploymentCpuRequirement{
					Cores: 4,
				},
				Memory:  pointers.Ptr("512Mi"),
				Storage: pointers.Ptr("10Gi"),
			}, nil),
			wantEligible: false,
		},

		// ── Ordering / capacity-first rule ───────────────────────────────────────

		{
			// 6. Device fails capacity check AND has eligibilityRules that would match →
			//    result must be NOT eligible (capacity checked first)
			name: "fails capacity, matching eligibilityRule present → not eligible (capacity checked first)",
			device: withLabels(
				withCPU(buildDevice("dev-406", "Acme", "C-VI", "SN-0406"), 1, amd64),
				map[string]interface{}{"env": "production"},
			),
			checks: &clModels.DeviceConstraints{
				CapacityRequirements: &clModels.CapacityRequirements{
					Cpu: &clModels.DeploymentCpuRequirement{
						Cores: 8, // device only has 1 core → fails
					},
				},
				EligibilityRules: &[]clModels.EligibilityRule{
					existsRule("env"), // would match if capacity were not checked first
				},
			},
			wantEligible: false,
		},
		{
			// 7. Device passes capacity check AND has eligibilityRules that match → eligible
			name: "passes capacity, matching eligibilityRule present → eligible",
			device: withLabels(
				withCPU(buildDevice("dev-407", "Acme", "C-VII", "SN-0407"), 4, amd64),
				map[string]interface{}{"env": "production"},
			),
			checks: &clModels.DeviceConstraints{
				CapacityRequirements: &clModels.CapacityRequirements{
					Cpu: &clModels.DeploymentCpuRequirement{
						Cores: 4,
					},
				},
				EligibilityRules: &[]clModels.EligibilityRule{
					existsRule("env"),
				},
			},
			wantEligible: true,
		},
		{
			// 8. Device passes capacity check AND has eligibilityRules that don't match → not eligible
			name: "passes capacity, non-matching eligibilityRule present → not eligible",
			device: withLabels(
				withCPU(buildDevice("dev-408", "Acme", "C-VIII", "SN-0408"), 4, amd64),
				map[string]interface{}{"region": "us-east"},
			),
			checks: &clModels.DeviceConstraints{
				CapacityRequirements: &clModels.CapacityRequirements{
					Cpu: &clModels.DeploymentCpuRequirement{
						Cores: 4,
					},
				},
				EligibilityRules: &[]clModels.EligibilityRule{
					existsRule("env"), // device has "region" label, not "env" → no match
				},
			},
			wantEligible: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			eligible, reason, err := sel.IsDeviceEligible(tc.device, tc.checks)

			if tc.wantErr {
				require.Error(t, err)
				assert.False(t, eligible)
				assert.Empty(t, reason)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantEligible, eligible)

			if !tc.wantEligible {
				assert.NotEmpty(t, reason, "expected a non-empty reason for ineligible device")
			} else {
				assert.Empty(t, reason)
			}
		})
	}
}

func TestIsDeviceEligible_LabelSelector_ExistsDoesNotExist(t *testing.T) {
	sel := newTestSelector()

	// Single eligibility rule with a label selector containing the given expressions.
	makeChecks := func(expressions []clModels.MatchExpression) *clModels.DeviceConstraints {
		return buildConstraints(nil, []clModels.EligibilityRule{
			{
				LabelSelector: buildLabelSelector(expressions),
			},
		})
	}

	tests := []struct {
		name         string
		device       *clModels.DeviceCapabilitiesManifest
		checks       *clModels.DeviceConstraints
		wantEligible bool
		wantErr      bool
	}{
		// ── Exists operator ──────────────────────────────────────────────────────
		{
			// 1. Label exists on device → eligible
			name: "Exists: label starkindustries.org/region exists on device → eligible",
			device: withLabels(
				buildDevice("dev-501", "Stark Industries", "MK-I", "SN-0501"),
				map[string]interface{}{"starkindustries.org/region": "us-east"},
			),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression("starkindustries.org/region", clModels.Exists, nil),
			}),
			wantEligible: true,
		},
		{
			// 2. Label does NOT exist on device → not eligible
			name: "Exists: label starkindustries.org/region does not exist on device → not eligible",
			device: withLabels(
				buildDevice("dev-502", "Stark Industries", "MK-II", "SN-0502"),
				map[string]interface{}{"starkindustries.org/env": "production"},
			),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression("starkindustries.org/region", clModels.Exists, nil),
			}),
			wantEligible: false,
		},
		{
			// 3. Device has no labels at all, Exists check → not eligible, reason mentions no labels
			name:   "Exists: device has no labels at all → not eligible",
			device: buildDevice("dev-503", "Stark Industries", "MK-III", "SN-0503"),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression("starkindustries.org/region", clModels.Exists, nil),
			}),
			wantEligible: false,
		},
		{
			// 4. Multiple Exists expressions (AND): both labels exist → eligible
			name: "Exists: multiple expressions AND, both labels exist → eligible",
			device: withLabels(
				buildDevice("dev-504", "Stark Industries", "MK-IV", "SN-0504"),
				map[string]interface{}{
					"starkindustries.org/region": "us-east",
					"starkindustries.org/env":    "production",
				},
			),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression("starkindustries.org/region", clModels.Exists, nil),
				buildMatchExpression("starkindustries.org/env", clModels.Exists, nil),
			}),
			wantEligible: true,
		},
		{
			// 5. Multiple Exists expressions (AND): one label missing → not eligible
			name: "Exists: multiple expressions AND, one label missing → not eligible",
			device: withLabels(
				buildDevice("dev-505", "Stark Industries", "MK-V", "SN-0505"),
				map[string]interface{}{
					"starkindustries.org/region": "us-east",
				},
			),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression("starkindustries.org/region", clModels.Exists, nil),
				buildMatchExpression("starkindustries.org/env", clModels.Exists, nil),
			}),
			wantEligible: false,
		},

		// ── DoesNotExist operator ────────────────────────────────────────────────

		{
			// 6. Label does NOT exist → eligible
			name: "DoesNotExist: label starkindustries.org/deprecated does not exist → eligible",
			device: withLabels(
				buildDevice("dev-506", "Stark Industries", "MK-VI", "SN-0506"),
				map[string]interface{}{"starkindustries.org/env": "staging"},
			),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression("starkindustries.org/deprecated", clModels.DoesNotExist, nil),
			}),
			wantEligible: true,
		},
		{
			// 7. Label exists → not eligible
			name: "DoesNotExist: label starkindustries.org/deprecated exists → not eligible",
			device: withLabels(
				buildDevice("dev-507", "Stark Industries", "MK-VII", "SN-0507"),
				map[string]interface{}{
					"starkindustries.org/deprecated": "true",
				},
			),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression("starkindustries.org/deprecated", clModels.DoesNotExist, nil),
			}),
			wantEligible: false,
		},
		{
			// 8. Multiple DoesNotExist expressions: none of the labels exist → eligible
			name: "DoesNotExist: multiple expressions AND, none of the labels exist → eligible",
			device: withLabels(
				buildDevice("dev-508", "Stark Industries", "MK-VIII", "SN-0508"),
				map[string]interface{}{"starkindustries.org/env": "production"},
			),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression("starkindustries.org/deprecated", clModels.DoesNotExist, nil),
				buildMatchExpression("starkindustries.org/legacy", clModels.DoesNotExist, nil),
			}),
			wantEligible: true,
		},
		{
			// 9. Multiple DoesNotExist expressions: one label exists → not eligible
			name: "DoesNotExist: multiple expressions AND, one label exists → not eligible",
			device: withLabels(
				buildDevice("dev-509", "Stark Industries", "MK-IX", "SN-0509"),
				map[string]interface{}{
					"starkindustries.org/deprecated": "true",
				},
			),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression("starkindustries.org/deprecated", clModels.DoesNotExist, nil),
				buildMatchExpression("starkindustries.org/legacy", clModels.DoesNotExist, nil),
			}),
			wantEligible: false,
		},

		// ── Mixed Exists + DoesNotExist in same selector (AND) ───────────────────

		{
			// 10. env Exists AND deprecated DoesNotExist, both conditions met → eligible
			name: "Mixed: starkindustries.org/env Exists AND starkindustries.org/deprecated DoesNotExist, both conditions met → eligible",
			device: withLabels(
				buildDevice("dev-510", "Stark Industries", "MK-X", "SN-0510"),
				map[string]interface{}{
					"starkindustries.org/env": "production",
				},
			),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression("starkindustries.org/env", clModels.Exists, nil),
				buildMatchExpression("starkindustries.org/deprecated", clModels.DoesNotExist, nil),
			}),
			wantEligible: true,
		},
		{
			// 11. env Exists AND deprecated DoesNotExist, deprecated label present → not eligible
			name: "Mixed: starkindustries.org/env Exists AND starkindustries.org/deprecated DoesNotExist, deprecated label present → not eligible",
			device: withLabels(
				buildDevice("dev-511", "Stark Industries", "MK-XI", "SN-0511"),
				map[string]interface{}{
					"starkindustries.org/env":        "production",
					"starkindustries.org/deprecated": "true",
				},
			),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression("starkindustries.org/env", clModels.Exists, nil),
				buildMatchExpression("starkindustries.org/deprecated", clModels.DoesNotExist, nil),
			}),
			wantEligible: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			eligible, reason, err := sel.IsDeviceEligible(tc.device, tc.checks)

			if tc.wantErr {
				require.Error(t, err)
				assert.False(t, eligible)
				assert.Empty(t, reason)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantEligible, eligible)

			if !tc.wantEligible {
				assert.NotEmpty(t, reason, "expected a non-empty reason for ineligible device")
			} else {
				assert.Empty(t, reason)
			}
		})
	}
}

func TestIsDeviceEligible_PropertySelector_ExistsDoesNotExistInNotIn(t *testing.T) {
	sel := newTestSelector()

	amd64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	arm := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureArm

	otelTrue := true

	// makeChecks builds a DeviceConstraints with a single EligibilityRule using
	// a PropertySelector containing the given expressions. No CapacityRequirements.
	makeChecks := func(expressions []clModels.MatchExpression) *clModels.DeviceConstraints {
		return buildConstraints(nil, []clModels.EligibilityRule{
			{
				PropertySelector: buildPropertySelector(expressions),
			},
		})
	}

	tests := []struct {
		name         string
		device       *clModels.DeviceCapabilitiesManifest
		checks       *clModels.DeviceConstraints
		wantEligible bool
		wantErr      bool
	}{
		// ── Exists operator ──────────────────────────────────────────────────────
		{
			// 1. /memory exists (device has memory set) → eligible
			name: "Exists: /memory exists on device → eligible",
			device: withMemory(
				buildDevice("dev-701", "Stark Industries", "MK-I", "SN-0701"),
				"4Gi",
			),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression("/memory", clModels.Exists, nil),
			}),
			wantEligible: true,
		},
		{
			// 2. /memory does not exist (device memory is nil) → not eligible
			name:   "Exists: /memory not set on device → not eligible",
			device: buildDevice("dev-702", "Stark Industries", "MK-II", "SN-0702"),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression("/memory", clModels.Exists, nil),
			}),
			wantEligible: false,
		},
		{
			// 3. /vendor exists → eligible (vendor is always set via buildDevice)
			name:   "Exists: /vendor exists on device → eligible",
			device: buildDevice("dev-703", "Stark Industries", "MK-III", "SN-0703"),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression("/vendor", clModels.Exists, nil),
			}),
			wantEligible: true,
		},
		{
			// 4. /otelCollector exists (bool field set to true) → eligible
			name: "Exists: /otelCollector set to true on device → eligible",
			device: func() *clModels.DeviceCapabilitiesManifest {
				d := buildDevice("dev-704", "Stark Industries", "MK-IV", "SN-0704")
				d.Properties.OtelCollector = &otelTrue
				return d
			}(),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression("/otelCollector", clModels.Exists, nil),
			}),
			wantEligible: true,
		},
		{
			// 5. /otelCollector not set (nil) → not eligible
			name:   "Exists: /otelCollector not set (nil) on device → not eligible",
			device: buildDevice("dev-705", "Stark Industries", "MK-V", "SN-0705"),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression("/otelCollector", clModels.Exists, nil),
			}),
			wantEligible: false,
		},

		// ── DoesNotExist operator ────────────────────────────────────────────────

		{
			// 6. /memory is nil → eligible
			name:   "DoesNotExist: /memory is nil on device → eligible",
			device: buildDevice("dev-706", "Stark Industries", "MK-VI", "SN-0706"),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression("/memory", clModels.DoesNotExist, nil),
			}),
			wantEligible: true,
		},
		{
			// 7. /memory is set → not eligible
			name: "DoesNotExist: /memory is set on device → not eligible",
			device: withMemory(
				buildDevice("dev-707", "Stark Industries", "MK-VII", "SN-0707"),
				"2Gi",
			),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression("/memory", clModels.DoesNotExist, nil),
			}),
			wantEligible: false,
		},

		// ── In operator ──────────────────────────────────────────────────────────

		{
			// 8. /vendor = "Stark Industries", values=["Stark Industries","Wayne Enterprises"] → eligible
			name:   "In: /vendor matches one of the allowed values → eligible",
			device: buildDevice("dev-708", "Stark Industries", "MK-VIII", "SN-0708"),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression(
					"/vendor",
					clModels.In,
					[]interface{}{"Stark Industries", "Wayne Enterprises"},
				),
			}),
			wantEligible: true,
		},
		{
			// 9. /vendor = "Oscorp", values=["Stark Industries","Wayne Enterprises"] → not eligible
			name:   "In: /vendor does not match any allowed value → not eligible",
			device: buildDevice("dev-709", "Oscorp", "MK-IX", "SN-0709"),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression(
					"/vendor",
					clModels.In,
					[]interface{}{"Stark Industries", "Wayne Enterprises"},
				),
			}),
			wantEligible: false,
		},
		{
			// 10. /cpus/0/architecture = "amd64", values=["amd64","arm64"] → eligible
			name: "In: /cpus/0/architecture amd64 matches allowed architectures → eligible",
			device: withCPU(
				buildDevice("dev-710", "Stark Industries", "MK-X", "SN-0710"),
				4,
				amd64,
			),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression(
					"/cpus/0/architecture",
					clModels.In,
					[]interface{}{"amd64", "arm64"},
				),
			}),
			wantEligible: true,
		},
		{
			// 11. /cpus/0/architecture = "arm", values=["amd64","arm64"] → not eligible
			name:   "In: /cpus/0/architecture arm does not match allowed architectures → not eligible",
			device: withCPU(buildDevice("dev-711", "Stark Industries", "MK-XI", "SN-0711"), 4, arm),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression(
					"/cpus/0/architecture",
					clModels.In,
					[]interface{}{"amd64", "arm64"},
				),
			}),
			wantEligible: false,
		},

		// ── NotIn operator ───────────────────────────────────────────────────────

		{
			// 12. /vendor = "Oscorp", values=["Stark Industries"] → eligible
			name:   "NotIn: /vendor not in excluded values → eligible",
			device: buildDevice("dev-712", "Oscorp", "MK-XII", "SN-0712"),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression("/vendor", clModels.NotIn, []interface{}{"Stark Industries"}),
			}),
			wantEligible: true,
		},
		{
			// 13. /vendor = "Stark Industries", values=["Stark Industries"] → not eligible
			name:   "NotIn: /vendor is in excluded values → not eligible",
			device: buildDevice("dev-713", "Stark Industries", "MK-XIII", "SN-0713"),
			checks: makeChecks([]clModels.MatchExpression{
				buildMatchExpression("/vendor", clModels.NotIn, []interface{}{"Stark Industries"}),
			}),
			wantEligible: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			eligible, reason, err := sel.IsDeviceEligible(tc.device, tc.checks)

			if tc.wantErr {
				require.Error(t, err)
				assert.False(t, eligible)
				assert.Empty(t, reason)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantEligible, eligible)

			if !tc.wantEligible {
				assert.NotEmpty(t, reason, "expected a non-empty reason for ineligible device")
			} else {
				assert.Empty(t, reason)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestIsDeviceEligible_MultipleEligibilityRules_WithCapacity
//
// Covers edge cases where multiple EligibilityRules (OR semantics) are
// combined with CapacityRequirements. Each test exercises a distinct
// boundary condition across the full rule evaluation pipeline.
// ---------------------------------------------------------------------------

func TestIsDeviceEligible_MultipleEligibilityRules_WithCapacity(t *testing.T) {
	sel := newTestSelector()

	amd64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64
	arm64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureArm64
	reqAmd64 := clModels.DeploymentCpuRequirementArchitecturesAmd64

	// ── Shared rule builders ─────────────────────────────────────────────────

	// labelRule builds an EligibilityRule with only a LabelSelector.
	labelRule := func(expressions ...clModels.MatchExpression) clModels.EligibilityRule {
		return clModels.EligibilityRule{
			LabelSelector: buildLabelSelector(expressions),
		}
	}

	// propertyRule builds an EligibilityRule with only a PropertySelector.
	propertyRule := func(expressions ...clModels.MatchExpression) clModels.EligibilityRule {
		return clModels.EligibilityRule{
			PropertySelector: buildPropertySelector(expressions),
		}
	}

	// combinedRule builds an EligibilityRule with both selectors (AND between them).
	combinedRule := func(labelExprs []clModels.MatchExpression, propExprs []clModels.MatchExpression) clModels.EligibilityRule {
		return clModels.EligibilityRule{
			LabelSelector:    buildLabelSelector(labelExprs),
			PropertySelector: buildPropertySelector(propExprs),
		}
	}

	tests := []struct {
		name         string
		device       *clModels.DeviceCapabilitiesManifest
		checks       *clModels.DeviceConstraints
		wantEligible bool
		wantErr      bool
	}{
		{
			// 1. Two eligibility rules (OR): first rule fails (wrong label value),
			//    second rule passes (correct vendor via propertySelector).
			//    Capacity is satisfied. Device must be eligible via the second rule.
			name: "OR: first label rule fails, second property rule passes → eligible",
			device: withLabels(
				withCPU(buildDevice("dev-801", "Stark Industries", "MK-I", "SN-0801"), 4, amd64),
				map[string]interface{}{
					"example.com/os": "Windows",
				},
			),
			checks: buildConstraints(
				&clModels.CapacityRequirements{
					Cpu: &clModels.DeploymentCpuRequirement{Cores: 4},
				},
				[]clModels.EligibilityRule{
					// Rule 1: label example.com/os must be "Zephyr RTOS" → fails
					labelRule(
						buildMatchExpression(
							"example.com/os",
							clModels.In,
							[]interface{}{"Zephyr RTOS"},
						),
					),
					// Rule 2: /vendor must be "Stark Industries" → passes
					propertyRule(
						buildMatchExpression(
							"/vendor",
							clModels.In,
							[]interface{}{"Stark Industries"},
						),
					),
				},
			),
			wantEligible: true,
		},
		{
			// 2. Two eligibility rules (OR): both rules fail.
			//    Capacity is satisfied. Device must NOT be eligible.
			name: "OR: both rules fail → not eligible",
			device: withLabels(
				withCPU(buildDevice("dev-802", "Oscorp", "MK-II", "SN-0802"), 4, amd64),
				map[string]interface{}{
					"example.com/os": "Windows",
				},
			),
			checks: buildConstraints(
				&clModels.CapacityRequirements{
					Cpu: &clModels.DeploymentCpuRequirement{Cores: 4},
				},
				[]clModels.EligibilityRule{
					// Rule 1: label example.com/os must be "Zephyr RTOS" → fails
					labelRule(
						buildMatchExpression(
							"example.com/os",
							clModels.In,
							[]interface{}{"Zephyr RTOS"},
						),
					),
					// Rule 2: /vendor must be "Stark Industries" → fails (device is "Oscorp")
					propertyRule(
						buildMatchExpression(
							"/vendor",
							clModels.In,
							[]interface{}{"Stark Industries"},
						),
					),
				},
			),
			wantEligible: false,
		},
		{
			// 3. Combined rule (AND between labelSelector and propertySelector):
			//    label matches but property does NOT match → rule fails.
			//    No other rule present → not eligible.
			//    Capacity is satisfied.
			name: "AND within combined rule: label passes, property fails → not eligible",
			device: withLabels(
				withCPU(buildDevice("dev-803", "Oscorp", "MK-III", "SN-0803"), 4, amd64),
				map[string]interface{}{
					"example.com/hypervisor": "hyper-v",
				},
			),
			checks: buildConstraints(
				&clModels.CapacityRequirements{
					Cpu: &clModels.DeploymentCpuRequirement{Cores: 4},
				},
				[]clModels.EligibilityRule{
					// Single combined rule: label AND property must both pass
					combinedRule(
						[]clModels.MatchExpression{
							buildMatchExpression(
								"example.com/hypervisor",
								clModels.In,
								[]interface{}{"hyper-v"},
							),
						},
						[]clModels.MatchExpression{
							// vendor must be "EdgeCircuit Systems" but device is "Oscorp" → fails
							buildMatchExpression(
								"/vendor",
								clModels.In,
								[]interface{}{"EdgeCircuit Systems", "NanoEdge Devices"},
							),
						},
					),
				},
			),
			wantEligible: false,
		},
		{
			// 4. Three eligibility rules (OR): first two fail, third passes.
			//    Capacity (CPU + memory + storage) all satisfied.
			//    Validates OR short-circuits correctly on the third rule.
			name: "OR: three rules, first two fail, third passes → eligible",
			device: withLabels(
				withStorage(
					withMemory(
						withCPU(
							buildDevice("dev-804", "NanoEdge Devices", "MK-IV", "SN-0804"),
							8,
							arm64,
						),
						"4Gi",
					),
					"50Gi",
				),
				map[string]interface{}{
					"example.com/wasm.runtime": "WAMR",
				},
			),
			checks: buildConstraints(
				&clModels.CapacityRequirements{
					Cpu: &clModels.DeploymentCpuRequirement{
						Cores:         4,
						Architectures: &[]clModels.DeploymentCpuRequirementArchitectures{reqAmd64},
					},
					Memory:  pointers.Ptr("2Gi"),
					Storage: pointers.Ptr("20Gi"),
				},
				[]clModels.EligibilityRule{
					// Rule 1: label example.com/os must be "Zephyr RTOS" → fails
					labelRule(
						buildMatchExpression(
							"example.com/os",
							clModels.In,
							[]interface{}{"Zephyr RTOS"},
						),
					),
					// Rule 2: /vendor must be "Stark Industries" → fails
					propertyRule(
						buildMatchExpression(
							"/vendor",
							clModels.In,
							[]interface{}{"Stark Industries"},
						),
					),
					// Rule 3: label example.com/wasm.runtime must be "WAMR" → passes
					labelRule(
						buildMatchExpression(
							"example.com/wasm.runtime",
							clModels.In,
							[]interface{}{"WAMR"},
						),
					),
				},
			),
			// NOTE: capacity check uses reqAmd64 but device has arm64 → capacity fails first
			// Adjust: capacity must pass for eligibility rules to be evaluated.
			// Device has arm64, requirement is amd64 → NOT eligible (capacity fails).
			wantEligible: false,
		},
		{
			// 5. Capacity passes. Multiple eligibility rules where one rule uses
			//    DoesNotExist on a label AND NotIn on a property (combined rule).
			//    Device satisfies both conditions → eligible.
			name: "combined rule: DoesNotExist label AND NotIn property, both pass → eligible",
			device: withLabels(
				withCPU(buildDevice("dev-805", "Oscorp", "MK-V", "SN-0805"), 4, amd64),
				map[string]interface{}{
					"example.com/env": "production",
					// "example.com/deprecated" is intentionally absent
				},
			),
			checks: buildConstraints(
				&clModels.CapacityRequirements{
					Cpu: &clModels.DeploymentCpuRequirement{Cores: 2},
				},
				[]clModels.EligibilityRule{
					// Rule 1: label example.com/hypervisor must exist → fails (not present)
					labelRule(buildMatchExpression("example.com/hypervisor", clModels.Exists, nil)),
					// Rule 2 (combined): deprecated label DoesNotExist AND vendor NotIn excluded list
					combinedRule(
						[]clModels.MatchExpression{
							buildMatchExpression(
								"example.com/deprecated",
								clModels.DoesNotExist,
								nil,
							),
						},
						[]clModels.MatchExpression{
							buildMatchExpression(
								"/vendor",
								clModels.NotIn,
								[]interface{}{"Stark Industries", "Wayne Enterprises"},
							),
						},
					),
				},
			),
			wantEligible: true,
		},
		{
			// 6. Capacity fails (insufficient storage) even though all eligibility
			//    rules would match. Validates capacity-first evaluation order.
			//    Device must NOT be eligible regardless of matching rules.
			name: "capacity fails (storage), all eligibility rules would match → not eligible",
			device: withLabels(
				withStorage(
					withMemory(
						withCPU(
							buildDevice("dev-806", "Stark Industries", "MK-VI", "SN-0806"),
							8,
							amd64,
						),
						"8Gi",
					),
					"2Gi", // only 2Gi but 10Gi required
				),
				map[string]interface{}{
					"example.com/hypervisor": "hyper-v",
					"example.com/env":        "production",
				},
			),
			checks: &clModels.DeviceConstraints{
				CapacityRequirements: &clModels.CapacityRequirements{
					Cpu:     &clModels.DeploymentCpuRequirement{Cores: 4},
					Memory:  pointers.Ptr("4Gi"),
					Storage: pointers.Ptr("10Gi"), // device has only 2Gi → fails
				},
				EligibilityRules: &[]clModels.EligibilityRule{
					// Rule 1: both label conditions pass
					labelRule(
						buildMatchExpression(
							"example.com/hypervisor",
							clModels.In,
							[]interface{}{"hyper-v"},
						),
						buildMatchExpression("example.com/env", clModels.Exists, nil),
					),
					// Rule 2: vendor property passes
					propertyRule(
						buildMatchExpression(
							"/vendor",
							clModels.In,
							[]interface{}{"Stark Industries"},
						),
					),
				},
			},
			wantEligible: false,
		},
		{
			// 7. Multiple eligibility rules where the first rule has a nil device
			//    labels map (device has no labels). First rule requires a label →
			//    fails. Second rule uses only a propertySelector → passes.
			//    Validates that a missing labels map does not block property-only rules.
			name: "no device labels: label rule fails, property-only rule passes → eligible",
			device: withCPU(
				buildDevice("dev-807", "EdgeCircuit Systems", "MK-VII", "SN-0807"),
				4, amd64,
			),
			// device.Labels is nil (buildDevice does not set labels)
			checks: buildConstraints(
				&clModels.CapacityRequirements{
					Cpu: &clModels.DeploymentCpuRequirement{Cores: 2},
				},
				[]clModels.EligibilityRule{
					// Rule 1: requires label → fails because device has no labels
					labelRule(buildMatchExpression("example.com/hypervisor", clModels.Exists, nil)),
					// Rule 2: property-only rule → passes
					propertyRule(
						buildMatchExpression(
							"/vendor",
							clModels.In,
							[]interface{}{"EdgeCircuit Systems"},
						),
					),
				},
			),
			wantEligible: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			eligible, reason, err := sel.IsDeviceEligible(tc.device, tc.checks)

			if tc.wantErr {
				require.Error(t, err)
				assert.False(t, eligible)
				assert.Empty(t, reason)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantEligible, eligible)

			if !tc.wantEligible {
				assert.NotEmpty(t, reason, "expected a non-empty reason for ineligible device")
			} else {
				assert.Empty(t, reason)
			}
		})
	}
}

func TestIsDeviceEligible_ContainsAllContainsAny_WithEligibilityRules(t *testing.T) {
	sel := newTestSelector()

	amd64 := clModels.DeviceCapabilitiesManifestPropertiesCpusArchitectureAmd64

	// ── Peripheral / Interface builder helpers ───────────────────────────────

	withPeripherals := func(
		device *clModels.DeviceCapabilitiesManifest,
		peripherals []clModels.DevicePeripheral,
	) *clModels.DeviceCapabilitiesManifest {
		device.Properties.Peripherals = &peripherals
		return device
	}

	withInterfaces := func(
		device *clModels.DeviceCapabilitiesManifest,
		ifaces []clModels.DeviceCommunicationInterface,
	) *clModels.DeviceCapabilitiesManifest {
		device.Properties.Interfaces = &ifaces
		return device
	}

	// ── Rule builders ────────────────────────────────────────────────────────

	labelRule := func(expressions ...clModels.MatchExpression) clModels.EligibilityRule {
		return clModels.EligibilityRule{
			LabelSelector: buildLabelSelector(expressions),
		}
	}

	propertyRule := func(expressions ...clModels.MatchExpression) clModels.EligibilityRule {
		return clModels.EligibilityRule{
			PropertySelector: buildPropertySelector(expressions),
		}
	}

	combinedRule := func(labelExprs []clModels.MatchExpression, propExprs []clModels.MatchExpression) clModels.EligibilityRule {
		return clModels.EligibilityRule{
			LabelSelector:    buildLabelSelector(labelExprs),
			PropertySelector: buildPropertySelector(propExprs),
		}
	}

	// containsAllExpr builds a ContainsAll MatchExpression with an itemSelector.
	containsAllExpr := func(key string, itemExprs []clModels.MatchExpression) clModels.MatchExpression {
		return clModels.MatchExpression{
			Key:      key,
			Operator: clModels.ContainsAll,
			ItemSelector: &clModels.Selector{
				MatchExpressions: itemExprs,
			},
		}
	}

	// containsAnyExpr builds a ContainsAny MatchExpression with an itemSelector.
	containsAnyExpr := func(key string, itemExprs []clModels.MatchExpression) clModels.MatchExpression {
		return clModels.MatchExpression{
			Key:      key,
			Operator: clModels.ContainsAny,
			ItemSelector: &clModels.Selector{
				MatchExpressions: itemExprs,
			},
		}
	}

	tests := []struct {
		name         string
		device       *clModels.DeviceCapabilitiesManifest
		checks       *clModels.DeviceConstraints
		wantEligible bool
		wantErr      bool
	}{
		{
			// 1. ContainsAll on /peripherals: device has a GPU from NVIDIA → eligible.
			//    Single property rule, no capacity requirements.
			name: "ContainsAll: device has GPU peripheral from NVIDIA → eligible",
			device: withPeripherals(
				buildDevice("dev-901", "Stark Industries", "MK-I", "SN-0901"),
				[]clModels.DevicePeripheral{
					{Type: clModels.Camera, Manufacturer: pointers.Ptr("Sony")},
					{Type: clModels.Gpu, Manufacturer: pointers.Ptr("NVIDIA")},
				},
			),
			checks: buildConstraints(nil, []clModels.EligibilityRule{
				propertyRule(
					containsAllExpr("/peripherals", []clModels.MatchExpression{
						buildMatchExpression("/type", clModels.In, []interface{}{"gpu"}),
						buildMatchExpression("/manufacturer", clModels.In, []interface{}{"NVIDIA"}),
					}),
				),
			}),
			wantEligible: true,
		},
		{
			// 2. ContainsAll on /peripherals: device has GPU but from AMD, not NVIDIA → not eligible.
			name: "ContainsAll: device has GPU but wrong manufacturer → not eligible",
			device: withPeripherals(
				buildDevice("dev-902", "Stark Industries", "MK-II", "SN-0902"),
				[]clModels.DevicePeripheral{
					{Type: clModels.Gpu, Manufacturer: pointers.Ptr("AMD")},
				},
			),
			checks: buildConstraints(nil, []clModels.EligibilityRule{
				propertyRule(
					containsAllExpr("/peripherals", []clModels.MatchExpression{
						buildMatchExpression("/type", clModels.In, []interface{}{"gpu"}),
						buildMatchExpression("/manufacturer", clModels.In, []interface{}{"NVIDIA"}),
					}),
				),
			}),
			wantEligible: false,
		},
		{
			// 3. ContainsAny on /peripherals: device has either a GPU or a Display.
			//    Device has a Display → eligible (OR logic within itemSelector for ContainsAny).
			name: "ContainsAny: device has Display peripheral, rule requires gpu OR display → eligible",
			device: withPeripherals(
				buildDevice("dev-903", "Stark Industries", "MK-III", "SN-0903"),
				[]clModels.DevicePeripheral{
					{Type: clModels.Display, Manufacturer: pointers.Ptr("Samsung")},
				},
			),
			checks: buildConstraints(nil, []clModels.EligibilityRule{
				propertyRule(
					containsAnyExpr("/peripherals", []clModels.MatchExpression{
						buildMatchExpression("/type", clModels.In, []interface{}{"gpu"}),
						buildMatchExpression("/type", clModels.In, []interface{}{"display"}),
					}),
				),
			}),
			wantEligible: true,
		},
		{
			// 4. ContainsAny on /peripherals: device has only a Camera.
			//    Rule requires gpu OR display → not eligible.
			name: "ContainsAny: device has only Camera, rule requires gpu OR display → not eligible",
			device: withPeripherals(
				buildDevice("dev-904", "Stark Industries", "MK-IV", "SN-0904"),
				[]clModels.DevicePeripheral{
					{Type: clModels.Camera, Manufacturer: pointers.Ptr("Sony")},
				},
			),
			checks: buildConstraints(nil, []clModels.EligibilityRule{
				propertyRule(
					containsAnyExpr("/peripherals", []clModels.MatchExpression{
						buildMatchExpression("/type", clModels.In, []interface{}{"gpu"}),
						buildMatchExpression("/type", clModels.In, []interface{}{"display"}),
					}),
				),
			}),
			wantEligible: false,
		},
		{
			// 5. ContainsAll on /interfaces: device has an Ethernet interface → eligible.
			//    Single property rule, no capacity requirements.
			name: "ContainsAll: device has Ethernet interface → eligible",
			device: withInterfaces(
				buildDevice("dev-905", "Stark Industries", "MK-V", "SN-0905"),
				[]clModels.DeviceCommunicationInterface{
					{Type: clModels.Wifi},
					{Type: clModels.Ethernet},
				},
			),
			checks: buildConstraints(nil, []clModels.EligibilityRule{
				propertyRule(
					containsAllExpr("/interfaces", []clModels.MatchExpression{
						buildMatchExpression("/type", clModels.In, []interface{}{"ethernet"}),
					}),
				),
			}),
			wantEligible: true,
		},
		{
			// 6. ContainsAll on /interfaces: device has only Wifi, rule requires Ethernet → not eligible.
			name: "ContainsAll: device has only Wifi, rule requires Ethernet → not eligible",
			device: withInterfaces(
				buildDevice("dev-906", "Stark Industries", "MK-VI", "SN-0906"),
				[]clModels.DeviceCommunicationInterface{
					{Type: clModels.Wifi},
				},
			),
			checks: buildConstraints(nil, []clModels.EligibilityRule{
				propertyRule(
					containsAllExpr("/interfaces", []clModels.MatchExpression{
						buildMatchExpression("/type", clModels.In, []interface{}{"ethernet"}),
					}),
				),
			}),
			wantEligible: false,
		},
		{
			// 7. OR across two eligibility rules:
			//    Rule 1 (ContainsAll on /peripherals for NVIDIA GPU) → fails (device has AMD GPU).
			//    Rule 2 (label starkindustries.org/edge-certified Exists) → passes.
			//    Capacity: CPU 4 cores amd64 satisfied.
			//    Expected: eligible via Rule 2.
			name: "OR: ContainsAll peripheral rule fails, label Exists rule passes → eligible",
			device: withLabels(
				withCPU(
					withPeripherals(
						buildDevice("dev-907", "Stark Industries", "MK-VII", "SN-0907"),
						[]clModels.DevicePeripheral{
							{Type: clModels.Gpu, Manufacturer: pointers.Ptr("AMD")},
						},
					),
					4, amd64,
				),
				map[string]interface{}{
					"starkindustries.org/edge-certified": "true",
				},
			),
			checks: buildConstraints(
				&clModels.CapacityRequirements{
					Cpu: &clModels.DeploymentCpuRequirement{Cores: 4},
				},
				[]clModels.EligibilityRule{
					// Rule 1: ContainsAll requires NVIDIA GPU → fails
					propertyRule(
						containsAllExpr("/peripherals", []clModels.MatchExpression{
							buildMatchExpression("/type", clModels.In, []interface{}{"gpu"}),
							buildMatchExpression(
								"/manufacturer",
								clModels.In,
								[]interface{}{"NVIDIA"},
							),
						}),
					),
					// Rule 2: label starkindustries.org/edge-certified must exist → passes
					labelRule(
						buildMatchExpression(
							"starkindustries.org/edge-certified",
							clModels.Exists,
							nil,
						),
					),
				},
			),
			wantEligible: true,
		},
		{
			// 8. Combined rule (AND between labelSelector and propertySelector):
			//    Label starkindustries.org/tier = "premium" (Exists) → passes.
			//    PropertySelector ContainsAll on /peripherals for GPU from NVIDIA → passes.
			//    Both conditions met → eligible.
			//    Capacity: nil (isolated selector logic).
			name: "combined rule: label Exists AND ContainsAll peripheral both pass → eligible",
			device: withLabels(
				withPeripherals(
					buildDevice("dev-908", "Stark Industries", "MK-VIII", "SN-0908"),
					[]clModels.DevicePeripheral{
						{Type: clModels.Gpu, Manufacturer: pointers.Ptr("NVIDIA")},
					},
				),
				map[string]interface{}{
					"starkindustries.org/tier": "premium",
				},
			),
			checks: buildConstraints(nil, []clModels.EligibilityRule{
				combinedRule(
					[]clModels.MatchExpression{
						buildMatchExpression("starkindustries.org/tier", clModels.Exists, nil),
					},
					[]clModels.MatchExpression{
						containsAllExpr("/peripherals", []clModels.MatchExpression{
							buildMatchExpression("/type", clModels.In, []interface{}{"gpu"}),
							buildMatchExpression(
								"/manufacturer",
								clModels.In,
								[]interface{}{"NVIDIA"},
							),
						}),
					},
				),
			}),
			wantEligible: true,
		},
		{
			// 9. Combined rule (AND): label passes, ContainsAll on /peripherals fails
			//    (device has no GPU at all) → rule fails. No other rule → not eligible.
			name: "combined rule: label passes, ContainsAll peripheral fails (no GPU) → not eligible",
			device: withLabels(
				withPeripherals(
					buildDevice("dev-909", "Stark Industries", "MK-IX", "SN-0909"),
					[]clModels.DevicePeripheral{
						{Type: clModels.Camera, Manufacturer: pointers.Ptr("Sony")},
					},
				),
				map[string]interface{}{
					"starkindustries.org/tier": "premium",
				},
			),
			checks: buildConstraints(nil, []clModels.EligibilityRule{
				combinedRule(
					[]clModels.MatchExpression{
						buildMatchExpression("starkindustries.org/tier", clModels.Exists, nil),
					},
					[]clModels.MatchExpression{
						containsAllExpr("/peripherals", []clModels.MatchExpression{
							buildMatchExpression("/type", clModels.In, []interface{}{"gpu"}),
							buildMatchExpression(
								"/manufacturer",
								clModels.In,
								[]interface{}{"NVIDIA"},
							),
						}),
					},
				),
			}),
			wantEligible: false,
		},
		{
			// 10. Three eligibility rules (OR), capacity satisfied:
			//     Rule 1: ContainsAll on /peripherals (NVIDIA GPU) → fails (device has AMD GPU).
			//     Rule 2: ContainsAll on /interfaces (Cellular) → fails (device has only Wifi).
			//     Rule 3: ContainsAny on /peripherals (gpu OR display) AND label
			//             starkindustries.org/region Exists → passes (device has GPU + label).
			//     Expected: eligible via Rule 3.
			name: "OR three rules: first two ContainsAll fail, third ContainsAny+label passes → eligible",
			device: withLabels(
				withInterfaces(
					withPeripherals(
						withCPU(
							buildDevice("dev-910", "Stark Industries", "MK-X", "SN-0910"),
							8,
							amd64,
						),
						[]clModels.DevicePeripheral{
							{Type: clModels.Gpu, Manufacturer: pointers.Ptr("AMD")},
						},
					),
					[]clModels.DeviceCommunicationInterface{
						{Type: clModels.Wifi},
					},
				),
				map[string]interface{}{
					"starkindustries.org/region": "us-east",
				},
			),
			checks: buildConstraints(
				&clModels.CapacityRequirements{
					Cpu: &clModels.DeploymentCpuRequirement{Cores: 4},
				},
				[]clModels.EligibilityRule{
					// Rule 1: ContainsAll requires NVIDIA GPU → fails (AMD GPU present)
					propertyRule(
						containsAllExpr("/peripherals", []clModels.MatchExpression{
							buildMatchExpression("/type", clModels.In, []interface{}{"gpu"}),
							buildMatchExpression(
								"/manufacturer",
								clModels.In,
								[]interface{}{"NVIDIA"},
							),
						}),
					),
					// Rule 2: ContainsAll requires Cellular interface → fails (only Wifi)
					propertyRule(
						containsAllExpr("/interfaces", []clModels.MatchExpression{
							buildMatchExpression("/type", clModels.In, []interface{}{"cellular"}),
						}),
					),
					// Rule 3: ContainsAny (gpu OR display) AND label region Exists → passes
					combinedRule(
						[]clModels.MatchExpression{
							buildMatchExpression(
								"starkindustries.org/region",
								clModels.Exists,
								nil,
							),
						},
						[]clModels.MatchExpression{
							containsAnyExpr("/peripherals", []clModels.MatchExpression{
								buildMatchExpression("/type", clModels.In, []interface{}{"gpu"}),
								buildMatchExpression(
									"/type",
									clModels.In,
									[]interface{}{"display"},
								),
							}),
						},
					),
				},
			),
			wantEligible: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			eligible, reason, err := sel.IsDeviceEligible(tc.device, tc.checks)

			if tc.wantErr {
				require.Error(t, err)
				assert.False(t, eligible)
				assert.Empty(t, reason)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantEligible, eligible)

			if !tc.wantEligible {
				assert.NotEmpty(t, reason, "expected a non-empty reason for ineligible device")
			} else {
				assert.Empty(t, reason)
			}
		})
	}
}
