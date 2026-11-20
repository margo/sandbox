package matcher_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/margo/sandbox/shared-lib/matcher"
	"github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string {
	return &s
}

func vals(v ...any) *[]any {
	return &v
}

func toPlacementConstraints(exprs []sbi.MatchExpression) []sbi.PlacementConstraints {
	return []sbi.PlacementConstraints{
		{
			MatchExpressions: exprs,
		},
	}
}

func sampleProfile(t *testing.T) sbi.Profile {
	t.Helper()

	data, err := os.ReadFile("testdata/profile_test.json")
	require.NoError(t, err)

	var profile sbi.Profile
	require.NoError(t, json.Unmarshal(data, &profile))
	return profile
}

// ---------------------------------------------------------------------------
// 1. Mandatory Deployment Key Matching
// ---------------------------------------------------------------------------

func TestDeploymentKeyMatching(t *testing.T) {
	m := matcher.New()
	p := sampleProfile(t)

	t.Run("Supported deployment technology passes", func(t *testing.T) {
		ok, reason, err := m.IsTargetEligible(&p, nil)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Empty(t, reason)
	})

	t.Run("Unsupported deployment technology fails", func(t *testing.T) {

		p.Characteristics[0].Key = "margo.org/deployment/custom" // change to unsupported key
		ok, reason, err := m.IsTargetEligible(&p, nil)
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "device does not support required deployment technology")
	})

	t.Run("Empty deployment technology key is skipped", func(t *testing.T) {
		p.Characteristics[0].Key = "" // empty key
		ok, reason, err := m.IsTargetEligible(&p, nil)
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "device does not support required deployment technology")
	})

	t.Run("Nil profile returns false", func(t *testing.T) {
		ok, reason, err := m.IsTargetEligible(nil, nil)
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "profile is nil")
	})
}

// ---------------------------------------------------------------------------
// 2. Exists and DoesNotExist Operators
// ---------------------------------------------------------------------------

func TestExistsAndDoesNotExist(t *testing.T) {
	m := matcher.New()
	p := sampleProfile(t)

	t.Run("Exists without property: key present", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/interface/canbus",
				Operator: sbi.MatchExpressionOperatorExists,
			},
		}
		ok, _, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("Exists without property: key absent", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/interface/zigbee",
				Operator: sbi.MatchExpressionOperatorExists,
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "not found")
	})

	t.Run("Exists with property: property present", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/cpu",
				Property: strPtr("/totalCores"),
				Operator: sbi.MatchExpressionOperatorExists,
			},
		}
		ok, _, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("Exists with property: property absent", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/cpu",
				Property: strPtr("/nonExistentField"),
				Operator: sbi.MatchExpressionOperatorExists,
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "not found")
	})

	t.Run("Exists with values provided fails spec rule", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/cpu",
				Property: strPtr("/totalCores"),
				Operator: sbi.MatchExpressionOperatorExists,
				Values:   vals("invalid"),
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "values MUST be omitted for Exists")
	})

	t.Run("DoesNotExist without property: key absent", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/interface/zigbee",
				Operator: sbi.MatchExpressionOperatorDoesNotExist,
			},
		}
		ok, _, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("DoesNotExist without property: key present fails", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/interface/canbus",
				Operator: sbi.MatchExpressionOperatorDoesNotExist,
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "is present")
	})

	t.Run("DoesNotExist with property: property absent", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/cpu",
				Property: strPtr("/nonExistentField"),
				Operator: sbi.MatchExpressionOperatorDoesNotExist,
			},
		}
		ok, _, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("DoesNotExist with property: key itself absent", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/interface/zigbee",
				Property: strPtr("/channel"),
				Operator: sbi.MatchExpressionOperatorDoesNotExist,
			},
		}
		ok, _, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.True(t, ok)
	})
}

// ---------------------------------------------------------------------------
// 3. In and NotIn Operators
// ---------------------------------------------------------------------------

func TestInAndNotIn(t *testing.T) {
	m := matcher.New()
	p := sampleProfile(t)

	t.Run("In scalar string match", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/cpu",
				Property: strPtr("/architecture"),
				Operator: sbi.MatchExpressionOperatorIn,
				Values:   vals("arm64", "amd64"),
			},
		}
		ok, _, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("In scalar string no match", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/cpu",
				Property: strPtr("/architecture"),
				Operator: sbi.MatchExpressionOperatorIn,
				Values:   vals("arm64", "riscv64"),
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "not found in match expression values")
	})

	t.Run("In array of scalars matches when one element equals candidate", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/interface/canbus",
				Property: strPtr("/protocols"),
				Operator: sbi.MatchExpressionOperatorIn,
				Values:   vals("CANFD", "CAN1.0"),
			},
		}
		ok, _, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("In array of objects fails (must use ContainsAll/ContainsAny)", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/cpu",
				Property: strPtr("/cpus"),
				Operator: sbi.MatchExpressionOperatorIn,
				Values:   vals("amd64"),
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "array of objects")
	})

	t.Run("In boolean requires exactly one value", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/peripherals/gpu",
				Property: strPtr("/gpus/0/allocatable"),
				Operator: sbi.MatchExpressionOperatorIn,
				Values:   vals(true, false),
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "exactly one boolean")
	})

	t.Run("In boolean single match", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/peripherals/gpu",
				Property: strPtr("/gpus/0/allocatable"),
				Operator: sbi.MatchExpressionOperatorIn,
				Values:   vals(true),
			},
		}
		ok, _, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("NotIn scalar string success", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/cpu",
				Property: strPtr("/architecture"),
				Operator: sbi.MatchExpressionOperatorNotIn,
				Values:   vals("arm64", "riscv64"),
			},
		}
		ok, _, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("NotIn scalar string failure when value present", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/cpu",
				Property: strPtr("/architecture"),
				Operator: sbi.MatchExpressionOperatorNotIn,
				Values:   vals("amd64", "arm64"),
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "is present in match expression values")
	})

	t.Run("NotIn requires key and property to exist", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/cpu",
				Property: strPtr("/nonExistent"),
				Operator: sbi.MatchExpressionOperatorNotIn,
				Values:   vals("val"),
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "not found")
	})
}

// ---------------------------------------------------------------------------
// 4. Comparison Operators (Gt, GtEq, Lt, LtEq)
// ---------------------------------------------------------------------------

func TestComparisonOperators(t *testing.T) {
	m := matcher.New()
	p := sampleProfile(t)

	t.Run("Gt: strictly greater", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/cpu",
				Property: strPtr("/totalCores"),
				Operator: sbi.MatchExpressionOperatorGt,
				Values:   vals(4.0),
			},
		}
		ok, _, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("Gt: equal fails", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/cpu",
				Property: strPtr("/totalCores"),
				Operator: sbi.MatchExpressionOperatorGt,
				Values:   vals(8.0),
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "not greater than")
	})

	t.Run("GtEq: equal succeeds", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/cpu",
				Property: strPtr("/totalCores"),
				Operator: sbi.MatchExpressionOperatorGtEq,
				Values:   vals(8.0),
			},
		}
		ok, _, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("GtEq: less fails", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/cpu",
				Property: strPtr("/totalCores"),
				Operator: sbi.MatchExpressionOperatorGtEq,
				Values:   vals(16.0),
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "not greater than or equal to")
	})

	t.Run("Lt: strictly less succeeds", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/memory",
				Property: strPtr("/available"),
				Operator: sbi.MatchExpressionOperatorLt,
				Values:   vals(32.0),
			},
		}
		ok, _, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("Lt: equal fails", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/memory",
				Property: strPtr("/available"),
				Operator: sbi.MatchExpressionOperatorLt,
				Values:   vals(16.0),
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "not less than")
	})

	t.Run("LtEq: equal succeeds", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/memory",
				Property: strPtr("/available"),
				Operator: sbi.MatchExpressionOperatorLtEq,
				Values:   vals(16.0),
			},
		}
		ok, _, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("Comparison on non-numeric property fails", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/cpu",
				Property: strPtr("/architecture"),
				Operator: sbi.MatchExpressionOperatorGt,
				Values:   vals(4.0),
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "non-numeric type")
	})
}

// ---------------------------------------------------------------------------
// 5. ContainsAll and ContainsAny Operators
// ---------------------------------------------------------------------------

func TestContainsAllAndContainsAny(t *testing.T) {
	m := matcher.New()
	p := sampleProfile(t)

	t.Run("ContainsAll: single element satisfies all expressions", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/peripherals/gpu",
				Property: strPtr("/gpus"),
				Operator: sbi.MatchExpressionOperatorContainsAll,
				ItemSelector: &sbi.ItemSelector{
					MatchExpressions: []sbi.ItemMatchExpression{
						{
							Property: "/manufacturer",
							Operator: sbi.ItemMatchExpressionOperatorIn,
							Values:   vals("NVIDIA"),
						},
						{
							Property: "/vram",
							Operator: sbi.ItemMatchExpressionOperatorGtEq,
							Values:   vals(80.0),
						},
						{
							Property: "/allocatable",
							Operator: sbi.ItemMatchExpressionOperatorIn,
							Values:   vals(true),
						},
					},
				},
			},
		}
		ok, _, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("ContainsAll: no element satisfies all expressions", func(t *testing.T) {
		// NVIDIA has vram 80 and allocatable true; AMD has allocatable false.
		// Asking for AMD with allocatable true will fail for all elements.
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/peripherals/gpu",
				Property: strPtr("/gpus"),
				Operator: sbi.MatchExpressionOperatorContainsAll,
				ItemSelector: &sbi.ItemSelector{
					MatchExpressions: []sbi.ItemMatchExpression{
						{
							Property: "/manufacturer",
							Operator: sbi.ItemMatchExpressionOperatorIn,
							Values:   vals("AMD"),
						},
						{
							Property: "/allocatable",
							Operator: sbi.ItemMatchExpressionOperatorIn,
							Values:   vals(true),
						},
					},
				},
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "no element of")
	})

	t.Run("ContainsAny: element satisfies at least one expression", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/peripherals/gpu",
				Property: strPtr("/gpus"),
				Operator: sbi.MatchExpressionOperatorContainsAny,
				ItemSelector: &sbi.ItemSelector{
					MatchExpressions: []sbi.ItemMatchExpression{
						{
							Property: "/manufacturer",
							Operator: sbi.ItemMatchExpressionOperatorIn,
							Values:   vals("Intel"), // none
						},
						{
							Property: "/vram",
							Operator: sbi.ItemMatchExpressionOperatorGt,
							Values:   vals(70.0), // matches NVIDIA (80)
						},
					},
				},
			},
		}
		ok, _, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("ContainsAny: no element satisfies any expression", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/peripherals/gpu",
				Property: strPtr("/gpus"),
				Operator: sbi.MatchExpressionOperatorContainsAny,
				ItemSelector: &sbi.ItemSelector{
					MatchExpressions: []sbi.ItemMatchExpression{
						{
							Property: "/manufacturer",
							Operator: sbi.ItemMatchExpressionOperatorIn,
							Values:   vals("Intel"),
						},
						{
							Property: "/vram",
							Operator: sbi.ItemMatchExpressionOperatorGt,
							Values:   vals(100.0),
						},
					},
				},
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "no element of")
	})

	t.Run("ContainsAll with values returns spec error", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/peripherals/gpu",
				Property: strPtr("/gpus"),
				Operator: sbi.MatchExpressionOperatorContainsAll,
				Values:   vals("invalid"),
				ItemSelector: &sbi.ItemSelector{
					MatchExpressions: []sbi.ItemMatchExpression{
						{
							Property: "/allocatable",
							Operator: sbi.ItemMatchExpressionOperatorExists,
						},
					},
				},
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "values MUST be omitted for ContainsAll")
	})

	t.Run("ContainsAll without itemSelector returns spec error", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:          "margo.org/peripherals/gpu",
				Property:     strPtr("/gpus"),
				Operator:     sbi.MatchExpressionOperatorContainsAll,
				ItemSelector: nil,
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "itemSelector with at least one matchExpression is required")
	})
}

// ---------------------------------------------------------------------------
// 6. Characteristics Groups (OR across groups)
// ---------------------------------------------------------------------------

func TestCharacteristicsGroups(t *testing.T) {
	m := matcher.New()
	p := sampleProfile(t)

	t.Run("OR logic: first group fails, second group passes", func(t *testing.T) {
		groups := []sbi.PlacementConstraints{
			// Group 1: requires TPU (absent)
			{
				MatchExpressions: []sbi.MatchExpression{
					{
						Key:      "margo.org/peripherals/tpu",
						Operator: sbi.MatchExpressionOperatorExists,
					},
				},
			},
			// Group 2: requires GPU (present)
			{
				MatchExpressions: []sbi.MatchExpression{
					{
						Key:      "margo.org/peripherals/gpu",
						Operator: sbi.MatchExpressionOperatorExists,
					},
				},
			},
		}

		ok, _, err := m.IsTargetEligible(&p, groups)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("OR logic: all groups fail", func(t *testing.T) {
		groups := []sbi.PlacementConstraints{
			{
				MatchExpressions: []sbi.MatchExpression{
					{
						Key:      "margo.org/peripherals/tpu",
						Operator: sbi.MatchExpressionOperatorExists,
					},
				},
			},
			{
				MatchExpressions: []sbi.MatchExpression{
					{
						Key:      "margo.org/resource/memory",
						Property: strPtr("/available"),
						Operator: sbi.MatchExpressionOperatorGt,
						Values:   vals(100.0),
					},
				},
			},
		}

		ok, reason, err := m.IsTargetEligible(&p, groups)
		require.NoError(t, err)
		assert.False(t, ok)
		assert.NotEmpty(t, reason)
	})
}

// ---------------------------------------------------------------------------
// 7. SelectEligibleTargets Filtering
// ---------------------------------------------------------------------------

func TestSelectEligibleTargets(t *testing.T) {
	m := matcher.New()

	p1 := sampleProfile(t)

	p2 := sampleProfile(t)
	// p2 only supports Compose, not Helm
	p2.Characteristics[0] = sbi.Characteristic{
		Key:        "margo.org/deployment/compose",
		Properties: map[string]any{},
	}

	targets := []matcher.TargetProfile{
		{TargetID: "target-1", Profile: p1},
		{TargetID: "target-2", Profile: p2},
	}

	t.Run("Filters targets by deployment key", func(t *testing.T) {
		var eligible []matcher.TargetProfile
		for _, target := range targets {
			ok, _, err := m.IsTargetEligible(&target.Profile, nil)
			require.NoError(t, err)
			if ok {
				eligible = append(eligible, target)
			}
		}
		require.Len(t, eligible, 2)
		assert.Equal(t, "target-1", eligible[0].TargetID)
		assert.Equal(t, "target-2", eligible[1].TargetID)

	})

	t.Run("Filters targets by match expressions", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/memory",
				Property: strPtr("/available"),
				Operator: sbi.MatchExpressionOperatorGtEq,
				Values:   vals(16.0),
			},
		}
		var eligible []matcher.TargetProfile
		for _, target := range targets {
			ok, _, err := m.IsTargetEligible(&target.Profile, toPlacementConstraints(exprs))
			require.NoError(t, err)
			if ok {
				eligible = append(eligible, target)
			}
		}
		require.Len(t, eligible, 2)
	})
}

// ---------------------------------------------------------------------------
// 8. Error Handling
// ---------------------------------------------------------------------------

func TestInvalidPointerError(t *testing.T) {
	m := matcher.New()
	p := sampleProfile(t)

	exprs := []sbi.MatchExpression{
		{
			Key:      "margo.org/resource/cpu",
			Property: strPtr("invalidPointerWithoutLeadingSlash"),
			Operator: sbi.MatchExpressionOperatorExists,
		},
	}
	_, _, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must start with '/'")
}

// ---------------------------------------------------------------------------
// 9. Additional Operator and Edge Case Tests
// ---------------------------------------------------------------------------

func TestEdgeCasesAndValidation(t *testing.T) {
	m := matcher.New()
	p := sampleProfile(t)

	t.Run("In with numeric scalar", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/memory",
				Property: strPtr("/total"),
				Operator: sbi.MatchExpressionOperatorIn,
				Values:   vals(16.0, 32.0),
			},
		}
		ok, _, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("In with missing values", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/memory",
				Property: strPtr("/total"),
				Operator: sbi.MatchExpressionOperatorIn,
				Values:   nil,
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "values is required for In")
	})

	t.Run("In with mixed types in values", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/memory",
				Property: strPtr("/total"),
				Operator: sbi.MatchExpressionOperatorIn,
				Values:   vals(32.0, "stringVal"),
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "values MUST be the same data type")
	})

	t.Run("In with mixed types when string comes first", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/memory",
				Property: strPtr("/total"),
				Operator: sbi.MatchExpressionOperatorIn,
				Values:   vals("stringVal", 32.0),
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "values MUST be the same data type")
	})

	t.Run("Gt with missing values", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/memory",
				Property: strPtr("/total"),
				Operator: sbi.MatchExpressionOperatorGt,
				Values:   nil,
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "values is required for Gt")
	})

	t.Run("Gt with more than one value", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/memory",
				Property: strPtr("/total"),
				Operator: sbi.MatchExpressionOperatorGt,
				Values:   vals(10.0, 20.0),
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "MUST contain exactly one number")
	})

	t.Run("ContainsAll on non-existent property", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/peripherals/gpu",
				Property: strPtr("/nonExistentArray"),
				Operator: sbi.MatchExpressionOperatorContainsAll,
				ItemSelector: &sbi.ItemSelector{
					MatchExpressions: []sbi.ItemMatchExpression{
						{Property: "/model", Operator: sbi.ItemMatchExpressionOperatorExists},
					},
				},
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "not found")
	})

	t.Run("ContainsAll on non-array property", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/memory",
				Property: strPtr("/total"),
				Operator: sbi.MatchExpressionOperatorContainsAll,
				ItemSelector: &sbi.ItemSelector{
					MatchExpressions: []sbi.ItemMatchExpression{
						{Property: "/model", Operator: sbi.ItemMatchExpressionOperatorExists},
					},
				},
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "non-array type")
	})

	t.Run("ContainsAll on scalar array fails", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/interface/canbus",
				Property: strPtr("/protocols"),
				Operator: sbi.MatchExpressionOperatorContainsAll,
				ItemSelector: &sbi.ItemSelector{
					MatchExpressions: []sbi.ItemMatchExpression{
						{Property: "/model", Operator: sbi.ItemMatchExpressionOperatorExists},
					},
				},
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "array of scalars")
	})

	t.Run("ItemMatchExpression with Lt, LtEq, DoesNotExist", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/peripherals/gpu",
				Property: strPtr("/gpus"),
				Operator: sbi.MatchExpressionOperatorContainsAll,
				ItemSelector: &sbi.ItemSelector{
					MatchExpressions: []sbi.ItemMatchExpression{
						{
							Property: "/vram",
							Operator: sbi.ItemMatchExpressionOperatorLt,
							Values:   vals(100.0),
						},
						{
							Property: "/vram",
							Operator: sbi.ItemMatchExpressionOperatorLtEq,
							Values:   vals(80.0),
						},
						{
							Property: "/nonExistentTag",
							Operator: sbi.ItemMatchExpressionOperatorDoesNotExist,
						},
					},
				},
			},
		}
		ok, _, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("ItemMatchExpression with unsupported operator", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/peripherals/gpu",
				Property: strPtr("/gpus"),
				Operator: sbi.MatchExpressionOperatorContainsAll,
				ItemSelector: &sbi.ItemSelector{
					MatchExpressions: []sbi.ItemMatchExpression{
						{
							Property: "/vram",
							Operator: "UnsupportedOp",
						},
					},
				},
			},
		}
		_, _, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported item selector operator")
	})

	t.Run("Missing property pointer for operator requiring property", func(t *testing.T) {
		exprs := []sbi.MatchExpression{
			{
				Key:      "margo.org/resource/cpu",
				Property: nil,
				Operator: sbi.MatchExpressionOperatorGt,
				Values:   vals(4.0),
			},
		}
		ok, reason, err := m.IsTargetEligible(&p, toPlacementConstraints(exprs))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Contains(t, reason, "property pointer is required")
	})
}
