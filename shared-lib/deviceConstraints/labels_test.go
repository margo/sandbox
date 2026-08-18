package deviceconstraints

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	clModels "github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
)

// helpers

func newLogger(t *testing.T) *zap.SugaredLogger {
	t.Helper()
	l, err := zap.NewDevelopment()
	require.NoError(t, err)
	return l.Sugar()
}

func strLabel(v string) clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties {
	var p clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties
	_ = p.FromDeviceCapabilitiesManifestLabels0(v)
	return p
}

func numLabel(v float32) clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties {
	var p clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties
	_ = p.FromDeviceCapabilitiesManifestLabels1(v)
	return p
}

func boolLabel(v bool) clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties {
	var p clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties
	_ = p.FromDeviceCapabilitiesManifestLabels2(v)
	return p
}

func strSliceLabel(v []string) clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties {
	var p clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties
	_ = p.FromDeviceCapabilitiesManifestLabels3(v)
	return p
}

func numSliceLabel(v []float32) clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties {
	var p clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties
	_ = p.FromDeviceCapabilitiesManifestLabels4(v)
	return p
}

func vals(v ...interface{}) *[]interface{} { return &v }

func newEngine(
	t *testing.T,
	labels map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties,
) SelectorEngineIface {
	t.Helper()
	return NewLabelSelectorEngine(&labels)
}

// ── In ───────────────────────────────────────────────────────────────────────

func TestEvaluate_In_String_Match(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("production"),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "env", Operator: clModels.In, Values: vals("production", "staging")},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestEvaluate_In_String_NoMatch(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("dev"),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "env", Operator: clModels.In, Values: vals("production", "staging")},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestEvaluate_In_Number_Match(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"cores": numLabel(4),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "cores", Operator: clModels.In, Values: vals(float64(4), float64(8))},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestEvaluate_In_Bool_Match(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"gpu": boolLabel(true),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "gpu", Operator: clModels.In, Values: vals(true)},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestEvaluate_In_MissingKey(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "env", Operator: clModels.In, Values: vals("production")},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.Contains(t, reason, "env")
}

func TestEvaluate_In_NilValues(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("production"),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "env", Operator: clModels.In, Values: nil},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ── NotIn ─────────────────────────────────────────────────────────────────────

func TestEvaluate_NotIn_String_NotPresent(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("dev"),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "env", Operator: clModels.NotIn, Values: vals("production", "staging")},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestEvaluate_NotIn_String_Present(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("production"),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "env", Operator: clModels.NotIn, Values: vals("production")},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestEvaluate_NotIn_Bool_Present(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"gpu": boolLabel(false),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "gpu", Operator: clModels.NotIn, Values: vals(false)},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestEvaluate_NotIn_StringSlice_NonePresent(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"tags": strSliceLabel([]string{"x", "y"}),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "tags", Operator: clModels.NotIn, Values: vals("a", "b")},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestEvaluate_NotIn_StringSlice_OnePresent(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"tags": strSliceLabel([]string{"a", "y"}),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "tags", Operator: clModels.NotIn, Values: vals("a", "b")},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ── Exists ────────────────────────────────────────────────────────────────────

func TestEvaluate_Exists_KeyPresent(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"region": strLabel("us-east"),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "region", Operator: clModels.Exists},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestEvaluate_Exists_KeyAbsent(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "region", Operator: clModels.Exists},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.Contains(t, reason, "region")
}

// ── DoesNotExist ──────────────────────────────────────────────────────────────

func TestEvaluate_DoesNotExist_KeyAbsent(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "deprecated", Operator: clModels.DoesNotExist},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestEvaluate_DoesNotExist_KeyPresent(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"deprecated": boolLabel(true),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "deprecated", Operator: clModels.DoesNotExist},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.Contains(t, reason, "deprecated")
}

// ── Gt ───────────────────────────────────────────────────────────────────────

func TestEvaluate_Gt_Above(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"memory": numLabel(16),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "memory", Operator: clModels.Gt, Values: vals(float64(8))},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestEvaluate_Gt_Equal(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"memory": numLabel(8),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "memory", Operator: clModels.Gt, Values: vals(float64(8))},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestEvaluate_Gt_Below(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"memory": numLabel(4),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "memory", Operator: clModels.Gt, Values: vals(float64(8))},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestEvaluate_Gt_NonIntegerThreshold(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"memory": numLabel(16),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "memory", Operator: clModels.Gt, Values: vals(float64(8.5))},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.Contains(t, reason, "integer")
}

func TestEvaluate_Gt_NilValues(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"memory": numLabel(16),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "memory", Operator: clModels.Gt, Values: nil},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestEvaluate_Gt_StringLabel_Unsupported(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("production"),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "env", Operator: clModels.Gt, Values: vals(float64(1))},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.Contains(t, reason, "unsupported value type")
}

// ── Lt ───────────────────────────────────────────────────────────────────────

func TestEvaluate_Lt_Below(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"latency": numLabel(5),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "latency", Operator: clModels.Lt, Values: vals(float64(10))},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestEvaluate_Lt_Equal(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"latency": numLabel(10),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "latency", Operator: clModels.Lt, Values: vals(float64(10))},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestEvaluate_Lt_Above(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"latency": numLabel(20),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "latency", Operator: clModels.Lt, Values: vals(float64(10))},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestEvaluate_Lt_NonIntegerThreshold(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"latency": numLabel(5),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "latency", Operator: clModels.Lt, Values: vals(float64(9.9))},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.Contains(t, reason, "integer")
}

// ── Unknown operator ──────────────────────────────────────────────────────────

func TestEvaluate_UnknownOperator(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("production"),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "env", Operator: "Between", Values: vals("a", "z")},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.Contains(t, reason, "unknown or unsupported")
}

// ── Empty selector ────────────────────────────────────────────────────────────

func TestEvaluate_EmptyMatchExpressions(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{}
	s := &clModels.Selector{MatchExpressions: []clModels.MatchExpression{}}
	ok, reason := newEngine(t, labels).Evaluate(s)
	// result stays false (never touched), no error
	assert.False(t, ok)
	assert.Empty(t, reason)
}

// ── Multiple expressions (AND semantics) ──────────────────────────────────────

func TestEvaluate_MultipleExpressions_AllPass(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env":    strLabel("production"),
		"region": strLabel("us-east"),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "env", Operator: clModels.In, Values: vals("production")},
			{Key: "region", Operator: clModels.Exists},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestEvaluate_MultipleExpressions_OneFails(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env":    strLabel("production"),
		"region": strLabel("eu-west"),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "env", Operator: clModels.In, Values: vals("production")},
			{Key: "region", Operator: clModels.In, Values: vals("us-east")},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestEvaluate_MultipleExpressions_MixedOperators(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env":    strLabel("production"),
		"cores":  numLabel(16),
		"legacy": boolLabel(false),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "env", Operator: clModels.In, Values: vals("production", "staging")},
			{Key: "cores", Operator: clModels.Gt, Values: vals(float64(8))},
			{Key: "legacy", Operator: clModels.NotIn, Values: vals(true)},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleIn_String_ExactMatch(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("production"),
	}
	me := &clModels.MatchExpression{
		Key:      "env",
		Operator: clModels.In,
		Values:   vals("production", "staging"),
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleIn_String_NoMatch(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("dev"),
	}
	me := &clModels.MatchExpression{
		Key:      "env",
		Operator: clModels.In,
		Values:   vals("production", "staging"),
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "dev")
}

func TestHandleIn_String_SingleValueMatch(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"tier": strLabel("gold"),
	}
	me := &clModels.MatchExpression{
		Key:      "tier",
		Operator: clModels.In,
		Values:   vals("gold"),
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleIn_String_CaseSensitive(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("Production"), // capital P
	}
	me := &clModels.MatchExpression{
		Key:      "env",
		Operator: clModels.In,
		Values:   vals("production"), // lowercase p
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ── Number (float32) ──────────────────────────────────────────────────────────

func TestHandleIn_Number_MatchViaFloat64(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"cores": numLabel(4),
	}
	me := &clModels.MatchExpression{
		Key:      "cores",
		Operator: clModels.In,
		Values:   vals(float64(4), float64(8)),
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleIn_Number_MatchViaFloat32(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"cores": numLabel(4),
	}
	me := &clModels.MatchExpression{
		Key:      "cores",
		Operator: clModels.In,
		Values:   vals(float32(4)),
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleIn_Number_NoMatch(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"cores": numLabel(2),
	}
	me := &clModels.MatchExpression{
		Key:      "cores",
		Operator: clModels.In,
		Values:   vals(float64(4), float64(8)),
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "cores")
}

func TestHandleIn_Number_ZeroValue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"cores": numLabel(0),
	}
	me := &clModels.MatchExpression{
		Key:      "cores",
		Operator: clModels.In,
		Values:   vals(float64(0)),
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ── Bool ──────────────────────────────────────────────────────────────────────

func TestHandleIn_Bool_TrueMatch(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"gpu": boolLabel(true),
	}
	me := &clModels.MatchExpression{
		Key:      "gpu",
		Operator: clModels.In,
		Values:   vals(true, false),
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleIn_Bool_FalseMatch(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"gpu": boolLabel(false),
	}
	me := &clModels.MatchExpression{
		Key:      "gpu",
		Operator: clModels.In,
		Values:   vals(false),
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleIn_Bool_NoMatch(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"gpu": boolLabel(false),
	}
	me := &clModels.MatchExpression{
		Key:      "gpu",
		Operator: clModels.In,
		Values:   vals(true),
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "gpu")
}

func TestHandleIn_StringSlice_ExactSetMatch(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"tags": strSliceLabel([]string{"x", "y"}),
	}
	me := &clModels.MatchExpression{
		Key:      "tags",
		Operator: clModels.In,
		Values:   vals("x", "y"),
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleIn_StringSlice_EmptyLabelSlice(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"tags": strSliceLabel([]string{}),
	}
	me := &clModels.MatchExpression{
		Key:      "tags",
		Operator: clModels.In,
		Values:   vals("a", "b"),
	}

	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "empty")
}

func TestHandleIn_NumSlice_PartialMatch_Fails(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"speeds": numSliceLabel([]float32{1.0, 9.0}),
	}
	me := &clModels.MatchExpression{
		Key:      "speeds",
		Operator: clModels.In,
		Values:   vals(float64(1.0), float64(2.0)),
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "speeds")
}

func TestHandleIn_NumSlice_MatchViaFloat32Candidate(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"speeds": numSliceLabel([]float32{3.0}),
	}
	me := &clModels.MatchExpression{
		Key:      "speeds",
		Operator: clModels.In,
		Values:   vals(float32(3.0)),
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ── Missing key / nil values ──────────────────────────────────────────────────

func TestHandleIn_NilValues_ReturnsError(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("production"),
	}
	me := &clModels.MatchExpression{
		Key:      "env",
		Operator: clModels.In,
		Values:   nil,
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "values is required for IN")
}

func TestHandleIn_MissingKey_ReturnsError(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{}
	me := &clModels.MatchExpression{
		Key:      "missing",
		Operator: clModels.In,
		Values:   vals("x"),
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "missing")
}

func TestHandleIn_EmptyValues_StringLabel_NoMatch(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("production"),
	}
	me := &clModels.MatchExpression{
		Key:      "env",
		Operator: clModels.In,
		Values:   vals(), // empty but non-nil
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleIn_WrongCandidateType_StringLabel_NoMatch(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("production"),
	}
	// candidate is an int, not a string — type assertion inside HandleIn will skip it
	me := &clModels.MatchExpression{
		Key:      "env",
		Operator: clModels.In,
		Values:   vals(42),
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ── string (Labels0) ──────────────────────────────────────────────────────────

func TestHandleNotIn_String_NotPresent_ReturnsTrue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("dev"),
	}
	me := &clModels.MatchExpression{
		Key:      "env",
		Operator: clModels.NotIn,
		Values:   vals("production", "staging"),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleNotIn_String_Present_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("production"),
	}
	me := &clModels.MatchExpression{
		Key:      "env",
		Operator: clModels.NotIn,
		Values:   vals("production", "staging"),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "production")
}

func TestHandleNotIn_String_SingleValue_NotPresent(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"tier": strLabel("silver"),
	}
	me := &clModels.MatchExpression{
		Key:      "tier",
		Operator: clModels.NotIn,
		Values:   vals("gold"),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleNotIn_String_SingleValue_Present(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"tier": strLabel("gold"),
	}
	me := &clModels.MatchExpression{
		Key:      "tier",
		Operator: clModels.NotIn,
		Values:   vals("gold"),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "gold")
}

func TestHandleNotIn_String_CaseSensitive_DifferentCase_ReturnsTrue(t *testing.T) {
	// "Production" != "production" — should NOT match, so NotIn passes
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("Production"),
	}
	me := &clModels.MatchExpression{
		Key:      "env",
		Operator: clModels.NotIn,
		Values:   vals("production"),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleNotIn_String_EmptyValues_ReturnsTrue(t *testing.T) {
	// non-nil but empty values slice — nothing to match against, so NotIn passes
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("production"),
	}
	me := &clModels.MatchExpression{
		Key:      "env",
		Operator: clModels.NotIn,
		Values:   vals(),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ── float32 / number (Labels1) ────────────────────────────────────────────────

func TestHandleNotIn_Number_NotPresent_ViaFloat64_ReturnsTrue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"cores": numLabel(2),
	}
	me := &clModels.MatchExpression{
		Key:      "cores",
		Operator: clModels.NotIn,
		Values:   vals(float64(4), float64(8)),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleNotIn_Number_Present_ViaFloat64_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"cores": numLabel(4),
	}
	me := &clModels.MatchExpression{
		Key:      "cores",
		Operator: clModels.NotIn,
		Values:   vals(float64(4), float64(8)),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "cores")
}

func TestHandleNotIn_Number_Present_ViaFloat32_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"cores": numLabel(4),
	}
	me := &clModels.MatchExpression{
		Key:      "cores",
		Operator: clModels.NotIn,
		Values:   vals(float32(4)),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "cores")
}

func TestHandleNotIn_Number_ZeroValue_Present_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"cores": numLabel(0),
	}
	me := &clModels.MatchExpression{
		Key:      "cores",
		Operator: clModels.NotIn,
		Values:   vals(float64(0)),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "cores")
}

func TestHandleNotIn_Number_ZeroValue_NotPresent_ReturnsTrue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"cores": numLabel(0),
	}
	me := &clModels.MatchExpression{
		Key:      "cores",
		Operator: clModels.NotIn,
		Values:   vals(float64(1), float64(2)),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ── bool (Labels2) ────────────────────────────────────────────────────────────

func TestHandleNotIn_Bool_False_NotInValues_ReturnsTrue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"gpu": boolLabel(false),
	}
	me := &clModels.MatchExpression{
		Key:      "gpu",
		Operator: clModels.NotIn,
		Values:   vals(true),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleNotIn_Bool_True_Present_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"gpu": boolLabel(true),
	}
	me := &clModels.MatchExpression{
		Key:      "gpu",
		Operator: clModels.NotIn,
		Values:   vals(true),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "gpu")
}

func TestHandleNotIn_Bool_False_Present_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"gpu": boolLabel(false),
	}
	me := &clModels.MatchExpression{
		Key:      "gpu",
		Operator: clModels.NotIn,
		Values:   vals(false),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "gpu")
}

func TestHandleNotIn_Bool_BothValues_Present_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"gpu": boolLabel(true),
	}
	me := &clModels.MatchExpression{
		Key:      "gpu",
		Operator: clModels.NotIn,
		Values:   vals(true, false),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "gpu")
}

// ── []string (Labels3) ────────────────────────────────────────────────────────

func TestHandleNotIn_StringSlice_NonePresent_ReturnsTrue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"tags": strSliceLabel([]string{"x", "y"}),
	}
	me := &clModels.MatchExpression{
		Key:      "tags",
		Operator: clModels.NotIn,
		Values:   vals("a", "b", "c"),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleNotIn_StringSlice_OnePresent_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"tags": strSliceLabel([]string{"a", "z"}),
	}
	me := &clModels.MatchExpression{
		Key:      "tags",
		Operator: clModels.NotIn,
		Values:   vals("a", "b"),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "a")
}

func TestHandleNotIn_StringSlice_AllPresent_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"tags": strSliceLabel([]string{"a", "b"}),
	}
	me := &clModels.MatchExpression{
		Key:      "tags",
		Operator: clModels.NotIn,
		Values:   vals("a", "b"),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleNotIn_StringSlice_EmptyLabelSlice_ReturnsFalse(t *testing.T) {
	// updated code: empty label slice is an explicit error
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"tags": strSliceLabel([]string{}),
	}
	me := &clModels.MatchExpression{
		Key:      "tags",
		Operator: clModels.NotIn,
		Values:   vals("a", "b"),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "empty array")
}

func TestHandleNotIn_StringSlice_EmptyValuesSlice_NoneCanMatch_ReturnsTrue(t *testing.T) {
	// non-nil but empty values — candidateSet is empty, no label value can be found in it
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"tags": strSliceLabel([]string{"a", "b"}),
	}
	me := &clModels.MatchExpression{
		Key:      "tags",
		Operator: clModels.NotIn,
		Values:   vals(),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ── []float32 (Labels4) ───────────────────────────────────────────────────────

func TestHandleNotIn_NumSlice_NonePresent_ReturnsTrue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"speeds": numSliceLabel([]float32{5.0, 6.0}),
	}
	me := &clModels.MatchExpression{
		Key:      "speeds",
		Operator: clModels.NotIn,
		Values:   vals(float64(1.0), float64(2.0)),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleNotIn_NumSlice_OnePresent_ViaFloat64_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"speeds": numSliceLabel([]float32{1.0, 9.0}),
	}
	me := &clModels.MatchExpression{
		Key:      "speeds",
		Operator: clModels.NotIn,
		Values:   vals(float64(1.0), float64(2.0)),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "speeds")
}

func TestHandleNotIn_NumSlice_OnePresent_ViaFloat32_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"speeds": numSliceLabel([]float32{3.0}),
	}
	me := &clModels.MatchExpression{
		Key:      "speeds",
		Operator: clModels.NotIn,
		Values:   vals(float32(3.0)),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "speeds")
}

func TestHandleNotIn_NumSlice_AllPresent_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"speeds": numSliceLabel([]float32{1.0, 2.0}),
	}
	me := &clModels.MatchExpression{
		Key:      "speeds",
		Operator: clModels.NotIn,
		Values:   vals(float64(1.0), float64(2.0), float64(3.0)),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestHandleNotIn_NumSlice_EmptyValuesSlice_ReturnsTrue(t *testing.T) {
	// empty candidateSet — no label value can be found in it
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"speeds": numSliceLabel([]float32{1.0, 2.0}),
	}
	me := &clModels.MatchExpression{
		Key:      "speeds",
		Operator: clModels.NotIn,
		Values:   vals(),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ── Error paths ───────────────────────────────────────────────────────────────

func TestHandleNotIn_NilValues_ReturnsError(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("production"),
	}
	me := &clModels.MatchExpression{
		Key:      "env",
		Operator: clModels.NotIn,
		Values:   nil,
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "values is required for NotIn")
}

func TestHandleNotIn_MissingKey_ReturnsError(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{}
	me := &clModels.MatchExpression{
		Key:      "missing",
		Operator: clModels.NotIn,
		Values:   vals("x"),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "missing")
}

func TestHandleNotIn_WrongCandidateType_StringLabel_ReturnsTrue(t *testing.T) {
	// candidate is int — type assertion skips it, so label value never found → NotIn passes
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("production"),
	}
	me := &clModels.MatchExpression{
		Key:      "env",
		Operator: clModels.NotIn,
		Values:   vals(42),
	}
	ok, reason := newEngine(t, labels).HandleNotIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ═══════════════════════════════════════════════════════════════════════════════
// HandleExists
// ═══════════════════════════════════════════════════════════════════════════════

func TestHandleExists_StringLabel_KeyPresent_ReturnsTrue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("production"),
	}
	me := &clModels.MatchExpression{
		Key:      "env",
		Operator: clModels.Exists,
	}
	ok, reason := newEngine(t, labels).HandleExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleExists_NumberLabel_KeyPresent_ReturnsTrue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"cores": numLabel(8),
	}
	me := &clModels.MatchExpression{
		Key:      "cores",
		Operator: clModels.Exists,
	}
	ok, reason := newEngine(t, labels).HandleExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleExists_BoolLabel_KeyPresent_ReturnsTrue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"gpu": boolLabel(false),
	}
	me := &clModels.MatchExpression{
		Key:      "gpu",
		Operator: clModels.Exists,
	}
	ok, reason := newEngine(t, labels).HandleExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleExists_StringSliceLabel_KeyPresent_ReturnsTrue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"tags": strSliceLabel([]string{"a", "b"}),
	}
	me := &clModels.MatchExpression{
		Key:      "tags",
		Operator: clModels.Exists,
	}
	ok, reason := newEngine(t, labels).HandleExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleExists_NumSliceLabel_KeyPresent_ReturnsTrue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"speeds": numSliceLabel([]float32{1.0, 2.0}),
	}
	me := &clModels.MatchExpression{
		Key:      "speeds",
		Operator: clModels.Exists,
	}
	ok, reason := newEngine(t, labels).HandleExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleExists_KeyAbsent_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{}
	me := &clModels.MatchExpression{
		Key:      "region",
		Operator: clModels.Exists,
	}
	ok, reason := newEngine(t, labels).HandleExists(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "region")
}

func TestHandleExists_EmptyLabelsMap_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{}
	me := &clModels.MatchExpression{
		Key:      "any-key",
		Operator: clModels.Exists,
	}
	ok, reason := newEngine(t, labels).HandleExists(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "any-key")
}

func TestHandleExists_SimilarKeyName_DoesNotMatch(t *testing.T) {
	// "env" is present but "ENV" is not — key lookup is exact/case-sensitive
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("production"),
	}
	me := &clModels.MatchExpression{
		Key:      "ENV",
		Operator: clModels.Exists,
	}
	ok, reason := newEngine(t, labels).HandleExists(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "ENV")
}

func TestHandleExists_MultipleLabels_TargetKeyPresent_ReturnsTrue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env":    strLabel("production"),
		"region": strLabel("us-east"),
		"cores":  numLabel(4),
	}
	me := &clModels.MatchExpression{
		Key:      "region",
		Operator: clModels.Exists,
	}
	ok, reason := newEngine(t, labels).HandleExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleExists_MultipleLabels_TargetKeyAbsent_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env":   strLabel("production"),
		"cores": numLabel(4),
	}
	me := &clModels.MatchExpression{
		Key:      "region",
		Operator: clModels.Exists,
	}
	ok, reason := newEngine(t, labels).HandleExists(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "region")
}

func TestHandleExists_ReasonContainsKeyName(t *testing.T) {
	// Verify the error message explicitly names the missing key
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{}
	me := &clModels.MatchExpression{
		Key:      "vendor.acme/tier",
		Operator: clModels.Exists,
	}
	ok, reason := newEngine(t, labels).HandleExists(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "vendor.acme/tier")
}

// ═══════════════════════════════════════════════════════════════════════════════
// HandleDoesNotExists
// ═══════════════════════════════════════════════════════════════════════════════

func TestHandleDoesNotExists_KeyAbsent_ReturnsTrue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{}
	me := &clModels.MatchExpression{
		Key:      "deprecated",
		Operator: clModels.DoesNotExist,
	}
	ok, reason := newEngine(t, labels).HandleDoesNotExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleDoesNotExists_StringLabel_KeyPresent_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"deprecated": strLabel("true"),
	}
	me := &clModels.MatchExpression{
		Key:      "deprecated",
		Operator: clModels.DoesNotExist,
	}
	ok, reason := newEngine(t, labels).HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "deprecated")
}

func TestHandleDoesNotExists_NumberLabel_KeyPresent_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"cores": numLabel(4),
	}
	me := &clModels.MatchExpression{
		Key:      "cores",
		Operator: clModels.DoesNotExist,
	}
	ok, reason := newEngine(t, labels).HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "cores")
}

func TestHandleDoesNotExists_BoolLabel_KeyPresent_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"gpu": boolLabel(false),
	}
	me := &clModels.MatchExpression{
		Key:      "gpu",
		Operator: clModels.DoesNotExist,
	}
	ok, reason := newEngine(t, labels).HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "gpu")
}

func TestHandleDoesNotExists_StringSliceLabel_KeyPresent_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"tags": strSliceLabel([]string{"a", "b"}),
	}
	me := &clModels.MatchExpression{
		Key:      "tags",
		Operator: clModels.DoesNotExist,
	}
	ok, reason := newEngine(t, labels).HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "tags")
}

func TestHandleDoesNotExists_NumSliceLabel_KeyPresent_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"speeds": numSliceLabel([]float32{1.0, 2.0}),
	}
	me := &clModels.MatchExpression{
		Key:      "speeds",
		Operator: clModels.DoesNotExist,
	}
	ok, reason := newEngine(t, labels).HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "speeds")
}

func TestHandleDoesNotExists_EmptyLabelsMap_ReturnsTrue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{}
	me := &clModels.MatchExpression{
		Key:      "any-key",
		Operator: clModels.DoesNotExist,
	}
	ok, reason := newEngine(t, labels).HandleDoesNotExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleDoesNotExists_SimilarKeyName_AbsentKey_ReturnsTrue(t *testing.T) {
	// "env" exists but "ENV" does not — key lookup is exact/case-sensitive
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("production"),
	}
	me := &clModels.MatchExpression{
		Key:      "ENV",
		Operator: clModels.DoesNotExist,
	}
	ok, reason := newEngine(t, labels).HandleDoesNotExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleDoesNotExists_MultipleLabels_TargetKeyAbsent_ReturnsTrue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env":   strLabel("production"),
		"cores": numLabel(4),
	}
	me := &clModels.MatchExpression{
		Key:      "deprecated",
		Operator: clModels.DoesNotExist,
	}
	ok, reason := newEngine(t, labels).HandleDoesNotExists(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleDoesNotExists_MultipleLabels_TargetKeyPresent_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env":        strLabel("production"),
		"deprecated": boolLabel(true),
	}
	me := &clModels.MatchExpression{
		Key:      "deprecated",
		Operator: clModels.DoesNotExist,
	}
	ok, reason := newEngine(t, labels).HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "deprecated")
}

func TestHandleDoesNotExists_ReasonContainsKeyName(t *testing.T) {
	// Verify the error message explicitly names the present key
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"vendor.acme/tier": strLabel("gold"),
	}
	me := &clModels.MatchExpression{
		Key:      "vendor.acme/tier",
		Operator: clModels.DoesNotExist,
	}
	ok, reason := newEngine(t, labels).HandleDoesNotExists(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "vendor.acme/tier")
}

// ── Inverse symmetry ──────────────────────────────────────────────────────────

func TestHandleExists_And_HandleDoesNotExists_AreInverse_KeyPresent(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("production"),
	}
	me := &clModels.MatchExpression{Key: "env"}
	engine := newEngine(t, labels)

	existsOk, _ := engine.HandleExists(me)
	doesNotExistOk, _ := engine.HandleDoesNotExists(me)

	assert.True(t, existsOk)
	assert.False(t, doesNotExistOk)
	assert.NotEqual(t, existsOk, doesNotExistOk)
}

func TestHandleExists_And_HandleDoesNotExists_AreInverse_KeyAbsent(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{}
	me := &clModels.MatchExpression{Key: "missing"}
	engine := newEngine(t, labels)

	existsOk, _ := engine.HandleExists(me)
	doesNotExistOk, _ := engine.HandleDoesNotExists(me)

	assert.False(t, existsOk)
	assert.True(t, doesNotExistOk)
	assert.NotEqual(t, existsOk, doesNotExistOk)
}

// ═══════════════════════════════════════════════════════════════════════════════
// HandleGt — threshold validation
// ═══════════════════════════════════════════════════════════════════════════════

func TestHandleGt_NilValues_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"memory": numLabel(16),
	}
	me := &clModels.MatchExpression{
		Key:      "memory",
		Operator: clModels.Gt,
		Values:   nil,
	}
	ok, reason := newEngine(t, labels).HandleGt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "values is required for Gt")
}

func TestHandleGt_EmptyValues_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"memory": numLabel(16),
	}
	me := &clModels.MatchExpression{
		Key:      "memory",
		Operator: clModels.Gt,
		Values:   vals(),
	}
	ok, reason := newEngine(t, labels).HandleGt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "values is required for Gt")
}

func TestHandleGt_ThresholdIsFloat_NotInteger_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"memory": numLabel(16),
	}
	me := &clModels.MatchExpression{
		Key:      "memory",
		Operator: clModels.Gt,
		Values:   vals(float64(8.5)),
	}
	ok, reason := newEngine(t, labels).HandleGt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "integer")
}

func TestHandleGt_ThresholdIsString_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"memory": numLabel(16),
	}
	me := &clModels.MatchExpression{
		Key:      "memory",
		Operator: clModels.Gt,
		Values:   vals("8"),
	}
	ok, reason := newEngine(t, labels).HandleGt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "integer")
}

func TestHandleGt_ThresholdIsBool_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"memory": numLabel(16),
	}
	me := &clModels.MatchExpression{
		Key:      "memory",
		Operator: clModels.Gt,
		Values:   vals(true),
	}
	ok, reason := newEngine(t, labels).HandleGt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "integer")
}

func TestHandleGt_ThresholdIsNil_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"memory": numLabel(16),
	}
	me := &clModels.MatchExpression{
		Key:      "memory",
		Operator: clModels.Gt,
		Values:   vals(nil),
	}
	ok, reason := newEngine(t, labels).HandleGt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "integer")
}

// ═══════════════════════════════════════════════════════════════════════════════
// HandleGt — key lookup
// ═══════════════════════════════════════════════════════════════════════════════

func TestHandleGt_MissingKey_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{}
	me := &clModels.MatchExpression{
		Key:      "memory",
		Operator: clModels.Gt,
		Values:   vals(float64(8)),
	}
	ok, reason := newEngine(t, labels).HandleGt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "memory")
}

func TestHandleGt_EmptyLabelsMap_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{}
	me := &clModels.MatchExpression{
		Key:      "cores",
		Operator: clModels.Gt,
		Values:   vals(float64(4)),
	}
	ok, reason := newEngine(t, labels).HandleGt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "cores")
}

// ═══════════════════════════════════════════════════════════════════════════════
// HandleGt — numeric comparison (Labels1 / float32)
// ═══════════════════════════════════════════════════════════════════════════════

func TestHandleGt_Number_LabelAboveThreshold_ReturnsTrue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"memory": numLabel(16),
	}
	me := &clModels.MatchExpression{
		Key:      "memory",
		Operator: clModels.Gt,
		Values:   vals(float64(8)),
	}
	ok, reason := newEngine(t, labels).HandleGt(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleGt_Number_LabelEqualToThreshold_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"memory": numLabel(8),
	}
	me := &clModels.MatchExpression{
		Key:      "memory",
		Operator: clModels.Gt,
		Values:   vals(float64(8)),
	}
	ok, reason := newEngine(t, labels).HandleGt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "not greater than")
}

func TestHandleGt_Number_LabelBelowThreshold_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"memory": numLabel(4),
	}
	me := &clModels.MatchExpression{
		Key:      "memory",
		Operator: clModels.Gt,
		Values:   vals(float64(8)),
	}
	ok, reason := newEngine(t, labels).HandleGt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "not greater than")
}

func TestHandleGt_Number_ZeroLabelAboveNegativeThreshold_ReturnsTrue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"offset": numLabel(0),
	}
	me := &clModels.MatchExpression{
		Key:      "offset",
		Operator: clModels.Gt,
		Values:   vals(float64(-1)),
	}
	ok, reason := newEngine(t, labels).HandleGt(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleGt_Number_ZeroLabelEqualToZeroThreshold_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"offset": numLabel(0),
	}
	me := &clModels.MatchExpression{
		Key:      "offset",
		Operator: clModels.Gt,
		Values:   vals(float64(0)),
	}
	ok, reason := newEngine(t, labels).HandleGt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "not greater than")
}

func TestHandleGt_Number_LargeValue_AboveThreshold_ReturnsTrue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"storage": numLabel(1024),
	}
	me := &clModels.MatchExpression{
		Key:      "storage",
		Operator: clModels.Gt,
		Values:   vals(float64(512)),
	}
	ok, reason := newEngine(t, labels).HandleGt(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleGt_Number_ReasonContainsKeyAndValues(t *testing.T) {
	// Verify the failure message names both the key and the threshold
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"cores": numLabel(2),
	}
	me := &clModels.MatchExpression{
		Key:      "cores",
		Operator: clModels.Gt,
		Values:   vals(float64(4)),
	}
	ok, reason := newEngine(t, labels).HandleGt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "cores")
	assert.Contains(t, reason, "4")
}

// ═══════════════════════════════════════════════════════════════════════════════
// HandleGt — incorrect label value types (type mismatch)
// ═══════════════════════════════════════════════════════════════════════════════

func TestHandleGt_StringLabel_ReturnsFalse_UnsupportedType(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("production"),
	}
	me := &clModels.MatchExpression{
		Key:      "env",
		Operator: clModels.Gt,
		Values:   vals(float64(1)),
	}
	ok, reason := newEngine(t, labels).HandleGt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "unsupported value type")
	assert.Contains(t, reason, "number")
}

func TestHandleGt_BoolLabel_ReturnsFalse_UnsupportedType(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"gpu": boolLabel(true),
	}
	me := &clModels.MatchExpression{
		Key:      "gpu",
		Operator: clModels.Gt,
		Values:   vals(float64(0)),
	}
	ok, reason := newEngine(t, labels).HandleGt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "unsupported value type")
	assert.Contains(t, reason, "number")
}

func TestHandleGt_StringSliceLabel_ReturnsFalse_UnsupportedType(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"tags": strSliceLabel([]string{"a", "b"}),
	}
	me := &clModels.MatchExpression{
		Key:      "tags",
		Operator: clModels.Gt,
		Values:   vals(float64(1)),
	}
	ok, reason := newEngine(t, labels).HandleGt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "unsupported value type")
	assert.Contains(t, reason, "number")
}

func TestHandleGt_NumSliceLabel_ReturnsFalse_UnsupportedType(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"speeds": numSliceLabel([]float32{1.0, 2.0}),
	}
	me := &clModels.MatchExpression{
		Key:      "speeds",
		Operator: clModels.Gt,
		Values:   vals(float64(1)),
	}
	ok, reason := newEngine(t, labels).HandleGt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "unsupported value type")
	assert.Contains(t, reason, "number")
}

// ═══════════════════════════════════════════════════════════════════════════════
// HandleGt — only first value in Values slice is used as threshold
// ═══════════════════════════════════════════════════════════════════════════════

func TestHandleGt_OnlyFirstValueUsedAsThreshold_SecondIgnored(t *testing.T) {
	// label=10, Values[0]=5 (passes), Values[1]=20 (would fail if used — but it's ignored)
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"cores": numLabel(10),
	}
	me := &clModels.MatchExpression{
		Key:      "cores",
		Operator: clModels.Gt,
		Values:   vals(float64(5), float64(20)),
	}
	ok, reason := newEngine(t, labels).HandleGt(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleGt_OnlyFirstValueUsedAsThreshold_FirstFails(t *testing.T) {
	// label=3, Values[0]=5 (fails), Values[1]=1 (would pass if used — but it's ignored)
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"cores": numLabel(3),
	}
	me := &clModels.MatchExpression{
		Key:      "cores",
		Operator: clModels.Gt,
		Values:   vals(float64(5), float64(1)),
	}
	ok, reason := newEngine(t, labels).HandleGt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "not greater than")
}

// ═══════════════════════════════════════════════════════════════════════════════
// HandleLt — threshold validation
// ═══════════════════════════════════════════════════════════════════════════════

func TestHandleLt_NilValues_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"latency": numLabel(5),
	}
	me := &clModels.MatchExpression{
		Key:      "latency",
		Operator: clModels.Lt,
		Values:   nil,
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "values is required for Lt")
}

func TestHandleLt_EmptyValues_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"latency": numLabel(5),
	}
	me := &clModels.MatchExpression{
		Key:      "latency",
		Operator: clModels.Lt,
		Values:   vals(),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "values is required for Lt")
}

func TestHandleLt_ThresholdIsFloat_NotInteger_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"latency": numLabel(5),
	}
	me := &clModels.MatchExpression{
		Key:      "latency",
		Operator: clModels.Lt,
		Values:   vals(float64(9.9)),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "integer")
}

func TestHandleLt_ThresholdIsString_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"latency": numLabel(5),
	}
	me := &clModels.MatchExpression{
		Key:      "latency",
		Operator: clModels.Lt,
		Values:   vals("10"),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "integer")
}

func TestHandleLt_ThresholdIsBool_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"latency": numLabel(5),
	}
	me := &clModels.MatchExpression{
		Key:      "latency",
		Operator: clModels.Lt,
		Values:   vals(false),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "integer")
}

func TestHandleLt_ThresholdIsNil_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"latency": numLabel(5),
	}
	me := &clModels.MatchExpression{
		Key:      "latency",
		Operator: clModels.Lt,
		Values:   vals(nil),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "integer")
}

func TestHandleLt_ThresholdIsSlice_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"latency": numLabel(5),
	}
	me := &clModels.MatchExpression{
		Key:      "latency",
		Operator: clModels.Lt,
		Values:   vals([]float64{10.0}),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "integer")
}

// ═══════════════════════════════════════════════════════════════════════════════
// HandleLt — key lookup
// ═══════════════════════════════════════════════════════════════════════════════

func TestHandleLt_MissingKey_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{}
	me := &clModels.MatchExpression{
		Key:      "latency",
		Operator: clModels.Lt,
		Values:   vals(float64(10)),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "latency")
}

func TestHandleLt_EmptyLabelsMap_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{}
	me := &clModels.MatchExpression{
		Key:      "cores",
		Operator: clModels.Lt,
		Values:   vals(float64(4)),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "cores")
}

// ═══════════════════════════════════════════════════════════════════════════════
// HandleLt — numeric comparison (Labels1 / float32)
// ═══════════════════════════════════════════════════════════════════════════════

func TestHandleLt_Number_LabelBelowThreshold_ReturnsTrue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"latency": numLabel(5),
	}
	me := &clModels.MatchExpression{
		Key:      "latency",
		Operator: clModels.Lt,
		Values:   vals(float64(10)),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleLt_Number_LabelEqualToThreshold_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"latency": numLabel(10),
	}
	me := &clModels.MatchExpression{
		Key:      "latency",
		Operator: clModels.Lt,
		Values:   vals(float64(10)),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "not less than")
}

func TestHandleLt_Number_LabelAboveThreshold_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"latency": numLabel(20),
	}
	me := &clModels.MatchExpression{
		Key:      "latency",
		Operator: clModels.Lt,
		Values:   vals(float64(10)),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "not less than")
}

func TestHandleLt_Number_ZeroLabelBelowPositiveThreshold_ReturnsTrue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"offset": numLabel(0),
	}
	me := &clModels.MatchExpression{
		Key:      "offset",
		Operator: clModels.Lt,
		Values:   vals(float64(1)),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleLt_Number_ZeroLabelEqualToZeroThreshold_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"offset": numLabel(0),
	}
	me := &clModels.MatchExpression{
		Key:      "offset",
		Operator: clModels.Lt,
		Values:   vals(float64(0)),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "not less than")
}

func TestHandleLt_Number_NegativeLabelBelowZeroThreshold_ReturnsTrue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"temperature": numLabel(-5),
	}
	me := &clModels.MatchExpression{
		Key:      "temperature",
		Operator: clModels.Lt,
		Values:   vals(float64(0)),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleLt_Number_NegativeLabelAboveNegativeThreshold_ReturnsFalse(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"temperature": numLabel(-1),
	}
	me := &clModels.MatchExpression{
		Key:      "temperature",
		Operator: clModels.Lt,
		Values:   vals(float64(-5)),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "not less than")
}

func TestHandleLt_Number_LargeThreshold_LabelWellBelow_ReturnsTrue(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"storage": numLabel(128),
	}
	me := &clModels.MatchExpression{
		Key:      "storage",
		Operator: clModels.Lt,
		Values:   vals(float64(1024)),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleLt_Number_ReasonContainsKeyAndThreshold(t *testing.T) {
	// Verify the failure message names both the key and the threshold
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"latency": numLabel(20),
	}
	me := &clModels.MatchExpression{
		Key:      "latency",
		Operator: clModels.Lt,
		Values:   vals(float64(10)),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "latency")
	assert.Contains(t, reason, "10")
}

// ═══════════════════════════════════════════════════════════════════════════════
// HandleLt — incorrect label value types (type mismatch)
// ═══════════════════════════════════════════════════════════════════════════════

func TestHandleLt_StringLabel_ReturnsFalse_UnsupportedType(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"env": strLabel("production"),
	}
	me := &clModels.MatchExpression{
		Key:      "env",
		Operator: clModels.Lt,
		Values:   vals(float64(10)),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "unsupported value type")
	assert.Contains(t, reason, "number")
}

func TestHandleLt_BoolLabel_ReturnsFalse_UnsupportedType(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"active": boolLabel(true),
	}
	me := &clModels.MatchExpression{
		Key:      "active",
		Operator: clModels.Lt,
		Values:   vals(float64(1)),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "unsupported value type")
	assert.Contains(t, reason, "number")
}

func TestHandleLt_StringSliceLabel_ReturnsFalse_UnsupportedType(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"tags": strSliceLabel([]string{"a", "b"}),
	}
	me := &clModels.MatchExpression{
		Key:      "tags",
		Operator: clModels.Lt,
		Values:   vals(float64(5)),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "unsupported value type")
	assert.Contains(t, reason, "number")
}

func TestHandleLt_NumSliceLabel_ReturnsFalse_UnsupportedType(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"speeds": numSliceLabel([]float32{1.0, 2.0}),
	}
	me := &clModels.MatchExpression{
		Key:      "speeds",
		Operator: clModels.Lt,
		Values:   vals(float64(5)),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "unsupported value type")
	assert.Contains(t, reason, "number")
}

// ═══════════════════════════════════════════════════════════════════════════════
// HandleLt — only first value in Values slice is used as threshold
// ═══════════════════════════════════════════════════════════════════════════════

func TestHandleLt_OnlyFirstValueUsedAsThreshold_SecondIgnored_Passes(t *testing.T) {
	// label=3, Values[0]=10 (passes), Values[1]=1 (would fail if used — but it's ignored)
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"latency": numLabel(3),
	}
	me := &clModels.MatchExpression{
		Key:      "latency",
		Operator: clModels.Lt,
		Values:   vals(float64(10), float64(1)),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestHandleLt_OnlyFirstValueUsedAsThreshold_SecondIgnored_Fails(t *testing.T) {
	// label=15, Values[0]=10 (fails), Values[1]=20 (would pass if used — but it's ignored)
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"latency": numLabel(15),
	}
	me := &clModels.MatchExpression{
		Key:      "latency",
		Operator: clModels.Lt,
		Values:   vals(float64(10), float64(20)),
	}
	ok, reason := newEngine(t, labels).HandleLt(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "not less than")
}

// ═══════════════════════════════════════════════════════════════════════════════
// HandleLt vs HandleGt — inverse symmetry on same label
// ═══════════════════════════════════════════════════════════════════════════════

func TestHandleLt_And_HandleGt_AreInverse_LabelBelowThreshold(t *testing.T) {
	// label=3, threshold=10 → Lt passes, Gt fails
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"latency": numLabel(3),
	}
	engine := newEngine(t, labels)
	meLt := &clModels.MatchExpression{Key: "latency", Operator: clModels.Lt, Values: vals(float64(10))}
	meGt := &clModels.MatchExpression{Key: "latency", Operator: clModels.Gt, Values: vals(float64(10))}

	ltOk, _ := engine.HandleLt(meLt)
	gtOk, _ := engine.HandleGt(meGt)

	assert.True(t, ltOk)
	assert.False(t, gtOk)
}

func TestHandleLt_And_HandleGt_AreInverse_LabelAboveThreshold(t *testing.T) {
	// label=20, threshold=10 → Lt fails, Gt passes
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"latency": numLabel(20),
	}
	engine := newEngine(t, labels)
	meLt := &clModels.MatchExpression{Key: "latency", Operator: clModels.Lt, Values: vals(float64(10))}
	meGt := &clModels.MatchExpression{Key: "latency", Operator: clModels.Gt, Values: vals(float64(10))}

	ltOk, _ := engine.HandleLt(meLt)
	gtOk, _ := engine.HandleGt(meGt)

	assert.False(t, ltOk)
	assert.True(t, gtOk)
}

func TestHandleLt_And_HandleGt_BothFalse_WhenLabelEqualsThreshold(t *testing.T) {
	// label=10, threshold=10 → neither Lt nor Gt passes (strict inequalities)
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"latency": numLabel(10),
	}
	engine := newEngine(t, labels)
	meLt := &clModels.MatchExpression{Key: "latency", Operator: clModels.Lt, Values: vals(float64(10))}
	meGt := &clModels.MatchExpression{Key: "latency", Operator: clModels.Gt, Values: vals(float64(10))}

	ltOk, ltReason := engine.HandleLt(meLt)
	gtOk, gtReason := engine.HandleGt(meGt)

	assert.False(t, ltOk)
	assert.False(t, gtOk)
	assert.NotEmpty(t, ltReason)
	assert.NotEmpty(t, gtReason)
}

// ═══════════════════════════════════════════════════════════════════════════════
// Real-world scenario: Stark Industries Edge AI Inference Node
//
// Device label map (reported via DeviceCapabilitiesManifest):
//
//  starkindustries.org/env              → "production"          (string)
//  starkindustries.org/region           → "us-east-1"           (string)
//  starkindustries.org/gpu              → true                  (bool)
//  starkindustries.org/cpu-cores        → 16                    (float32)
//  starkindustries.org/memory-gb        → 64                    (float32)
//  starkindustries.org/latency-ms       → 4                     (float32)
//  starkindustries.org/supported-arch   → ["arm64","amd64"]     ([]string)  — multi-arch node
//  starkindustries.org/runtimes         → ["oci","wasm"]        ([]string)  — supported runtimes
//  starkindustries.org/gpu-memory-gb    → 24                    (float32)
//  starkindustries.org/storage-tb       → 2                     (float32)
//
// "deprecated" and "starkindustries.org/maintenance" are intentionally absent.
// ═══════════════════════════════════════════════════════════════════════════════

func buildStarkDeviceLabels() map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties {
	return map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"starkindustries.org/env":            strLabel("production"),
		"starkindustries.org/region":         strLabel("us-east-1"),
		"starkindustries.org/gpu":            boolLabel(true),
		"starkindustries.org/cpu-cores":      numLabel(16),
		"starkindustries.org/memory-gb":      numLabel(64),
		"starkindustries.org/latency-ms":     numLabel(4),
		"starkindustries.org/supported-arch": strSliceLabel([]string{"arm64", "amd64"}),
		"starkindustries.org/runtimes":       strSliceLabel([]string{"oci", "wasm"}),
		"starkindustries.org/gpu-memory-gb":  numLabel(24),
		"starkindustries.org/storage-tb":     numLabel(2),
	}
}

// ── Scenario 1 ────────────────────────────────────────────────────────────────
// AI Inference workload deployment:
//   Requires a production GPU node in us-east-1 with sufficient compute,
//   running OCI + WASM runtimes, supporting arm64, with low latency.
//
//   starkindustries.org/env            In  [production, staging]
//   starkindustries.org/region         In  [us-east-1, us-west-2]
//   starkindustries.org/gpu            In  [true]
//   starkindustries.org/cpu-cores      Gt  8
//   starkindustries.org/memory-gb      Gt  32
//   starkindustries.org/latency-ms     Lt  10
//   starkindustries.org/runtimes       In  [oci, wasm]          ← array label
//   starkindustries.org/supported-arch In  [arm64, amd64, x86]  ← array label
//   starkindustries.org/maintenance    DoesNotExist
//
// Expected: PASS — device satisfies every constraint.

func TestEvaluate_Stark_AIInferenceWorkload_AllConstraintsSatisfied_ReturnsTrue(t *testing.T) {
	labels := buildStarkDeviceLabels()
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "starkindustries.org/env", Operator: clModels.In, Values: vals("production", "staging")},
			{Key: "starkindustries.org/region", Operator: clModels.In, Values: vals("us-east-1", "us-west-2")},
			{Key: "starkindustries.org/gpu", Operator: clModels.In, Values: vals(true)},
			{Key: "starkindustries.org/cpu-cores", Operator: clModels.Gt, Values: vals(float64(8))},
			{Key: "starkindustries.org/memory-gb", Operator: clModels.Gt, Values: vals(float64(32))},
			{Key: "starkindustries.org/latency-ms", Operator: clModels.Lt, Values: vals(float64(10))},
			{Key: "starkindustries.org/runtimes", Operator: clModels.In, Values: vals("oci", "wasm")},
			{Key: "starkindustries.org/supported-arch", Operator: clModels.In, Values: vals("arm64", "amd64")},
			{Key: "starkindustries.org/maintenance", Operator: clModels.DoesNotExist},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ── Scenario 2 ────────────────────────────────────────────────────────────────
// High-security compliance workload:
//   Must NOT run in dev/staging, must NOT be in EU regions (data residency),
//   must have GPU, sufficient GPU memory, and must NOT support runtimes
//   outside the approved set (wasm is not approved here).
//
//   starkindustries.org/env            NotIn [dev, staging]
//   starkindustries.org/region         NotIn [eu-west-1, eu-central-1]
//   starkindustries.org/gpu            Exists
//   starkindustries.org/gpu-memory-gb  Gt    16
//   starkindustries.org/cpu-cores      Gt    8
//   starkindustries.org/storage-tb     Gt    1
//   starkindustries.org/maintenance    DoesNotExist
//
// Expected: PASS — device is production/us-east-1 with gpu-memory=24 > 16.

func TestEvaluate_Stark_ComplianceWorkload_DataResidencyAndGpuMemory_ReturnsTrue(t *testing.T) {
	labels := buildStarkDeviceLabels()
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "starkindustries.org/env", Operator: clModels.NotIn, Values: vals("dev", "staging")},
			{Key: "starkindustries.org/region", Operator: clModels.NotIn, Values: vals("eu-west-1", "eu-central-1")},
			{Key: "starkindustries.org/gpu", Operator: clModels.Exists},
			{Key: "starkindustries.org/gpu-memory-gb", Operator: clModels.Gt, Values: vals(float64(16))},
			{Key: "starkindustries.org/cpu-cores", Operator: clModels.Gt, Values: vals(float64(8))},
			{Key: "starkindustries.org/storage-tb", Operator: clModels.Gt, Values: vals(float64(1))},
			{Key: "starkindustries.org/maintenance", Operator: clModels.DoesNotExist},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ── Scenario 3 ────────────────────────────────────────────────────────────────
// Lightweight IoT analytics workload:
//   Targets low-power nodes — explicitly requires cpu-cores Lt 8.
//   Device has 16 cores → fails this constraint.
//
//   starkindustries.org/env        In  [production]
//   starkindustries.org/cpu-cores  Lt  8              ← FAILS (device has 16)
//   starkindustries.org/memory-gb  Gt  4
//   starkindustries.org/gpu        NotIn [true]       ← would also fail, never reached
//
// Expected: FAIL — cpu-cores constraint fails first, short-circuits.

func TestEvaluate_Stark_IoTWorkload_CpuCoresExceedLimit_ReturnsFalse(t *testing.T) {
	labels := buildStarkDeviceLabels()
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "starkindustries.org/env", Operator: clModels.In, Values: vals("production")},
			{Key: "starkindustries.org/cpu-cores", Operator: clModels.Lt, Values: vals(float64(8))},
			{Key: "starkindustries.org/memory-gb", Operator: clModels.Gt, Values: vals(float64(4))},
			{Key: "starkindustries.org/gpu", Operator: clModels.NotIn, Values: vals(true)},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.Contains(t, reason, "starkindustries.org/cpu-cores")
	assert.Contains(t, reason, "not less than")
	// short-circuit: gpu constraint never evaluated
	assert.NotContains(t, reason, "starkindustries.org/gpu")
}

// ── Scenario 4 ────────────────────────────────────────────────────────────────
// Multi-tenant batch processing workload:
//   Requires x86 architecture support — device only has arm64 + amd64.
//   "x86" is NOT in starkindustries.org/supported-arch → In check fails.
//
//   starkindustries.org/env            In  [production]
//   starkindustries.org/cpu-cores      Gt  4
//   starkindustries.org/memory-gb      Gt  16
//   starkindustries.org/supported-arch In  [arm64, x86]   ← "x86" missing → FAILS
//   starkindustries.org/runtimes       In  [oci, wasm]
//
// Expected: FAIL — supported-arch does not contain "x86".

func TestEvaluate_Stark_BatchWorkload_MissingArchitecture_ReturnsFalse(t *testing.T) {
	labels := buildStarkDeviceLabels()
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "starkindustries.org/env", Operator: clModels.In, Values: vals("production")},
			{Key: "starkindustries.org/cpu-cores", Operator: clModels.Gt, Values: vals(float64(4))},
			{Key: "starkindustries.org/memory-gb", Operator: clModels.Gt, Values: vals(float64(16))},
			{Key: "starkindustries.org/supported-arch", Operator: clModels.In, Values: vals("arm64", "x86")},
			{Key: "starkindustries.org/runtimes", Operator: clModels.In, Values: vals("oci", "wasm")},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.Contains(t, reason, "starkindustries.org/supported-arch")
	assert.Contains(t, reason, "x86")
}

// ── Scenario 5 ────────────────────────────────────────────────────────────────
// Edge case — maintenance window injected at runtime:
//   Device enters maintenance; label starkindustries.org/maintenance is added.
//   All compute constraints pass, but DoesNotExist on maintenance fails.
//
//   starkindustries.org/env          In  [production]
//   starkindustries.org/cpu-cores    Gt  8
//   starkindustries.org/memory-gb    Gt  32
//   starkindustries.org/gpu          Exists
//   starkindustries.org/maintenance  DoesNotExist   ← FAILS (label injected)
//
// Expected: FAIL — maintenance label is present.

func TestEvaluate_Stark_MaintenanceLabelInjected_DoesNotExistFails(t *testing.T) {
	labels := buildStarkDeviceLabels()
	labels["starkindustries.org/maintenance"] = strLabel("scheduled") // runtime injection
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "starkindustries.org/env", Operator: clModels.In, Values: vals("production")},
			{Key: "starkindustries.org/cpu-cores", Operator: clModels.Gt, Values: vals(float64(8))},
			{Key: "starkindustries.org/memory-gb", Operator: clModels.Gt, Values: vals(float64(32))},
			{Key: "starkindustries.org/gpu", Operator: clModels.Exists},
			{Key: "starkindustries.org/maintenance", Operator: clModels.DoesNotExist},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.Contains(t, reason, "starkindustries.org/maintenance")
}

// ── Scenario 6 ────────────────────────────────────────────────────────────────
// Edge case — gpu-memory boundary: workload requires exactly more than 24 GB
//   gpu-memory. Device has exactly 24 GB → Gt 24 is a strict inequality → fails.
//   All other constraints pass.
//
//   starkindustries.org/env           In  [production]
//   starkindustries.org/gpu           Exists
//   starkindustries.org/gpu-memory-gb Gt  24          ← FAILS (device has exactly 24)
//   starkindustries.org/latency-ms    Lt  10
//
// Expected: FAIL — strict Gt boundary not met.

func TestEvaluate_Stark_GpuMemoryAtExactBoundary_StrictGt_ReturnsFalse(t *testing.T) {
	labels := buildStarkDeviceLabels() // gpu-memory-gb = 24
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "starkindustries.org/env", Operator: clModels.In, Values: vals("production")},
			{Key: "starkindustries.org/gpu", Operator: clModels.Exists},
			{Key: "starkindustries.org/gpu-memory-gb", Operator: clModels.Gt, Values: vals(float64(24))},
			{Key: "starkindustries.org/latency-ms", Operator: clModels.Lt, Values: vals(float64(10))},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.Contains(t, reason, "starkindustries.org/gpu-memory-gb")
	assert.Contains(t, reason, "not greater than")
}

// ── Scenario 7 ────────────────────────────────────────────────────────────────
// Edge case — same workload as Scenario 6 but threshold relaxed to 23:
//   gpu-memory-gb Gt 23 → device has 24 → passes.
//   Validates that one unit below the label value is sufficient.
//
// Expected: PASS — strict Gt boundary met with threshold one below label value.

func TestEvaluate_Stark_GpuMemoryOneAboveBoundary_StrictGt_ReturnsTrue(t *testing.T) {
	labels := buildStarkDeviceLabels() // gpu-memory-gb = 24
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "starkindustries.org/env", Operator: clModels.In, Values: vals("production")},
			{Key: "starkindustries.org/gpu", Operator: clModels.Exists},
			{Key: "starkindustries.org/gpu-memory-gb", Operator: clModels.Gt, Values: vals(float64(23))},
			{Key: "starkindustries.org/latency-ms", Operator: clModels.Lt, Values: vals(float64(10))},
			{Key: "starkindustries.org/runtimes", Operator: clModels.In, Values: vals("oci", "wasm")},
			{Key: "starkindustries.org/maintenance", Operator: clModels.DoesNotExist},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// Label ["a","b"], me.Values ["a","b","c"]:
// "c" is NOT in the label set {a,b} → FAIL
func TestEvaluate_In_StringSlice_AllPresent(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"tags": strSliceLabel([]string{"a", "b"}),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "tags", Operator: clModels.In, Values: vals("a", "b", "c")},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// Label ["a","z"], me.Values ["a","b"]:
// "b" is NOT in the label set {a,z} → FAIL (unchanged result, but renamed for clarity)
func TestEvaluate_In_StringSlice_MissingElement(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"tags": strSliceLabel([]string{"a", "z"}),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "tags", Operator: clModels.In, Values: vals("a", "b")},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ── New: TestEvaluate_In_StringSlice_AllMeValuesInLabel ───────────────────────
// Label ["a","b","c"], me.Values ["a","b"]:
// All me.Values present in label set {a,b,c} → PASS
func TestEvaluate_In_StringSlice_AllMeValuesInLabel(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"tags": strSliceLabel([]string{"a", "b", "c"}),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "tags", Operator: clModels.In, Values: vals("a", "b")},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ── Corrected: TestEvaluate_In_NumSlice_AllPresent ────────────────────────────
// Label [1.0,2.0], me.Values [1.0,2.0,3.0]:
// 3.0 is NOT in label set {1.0,2.0} → FAIL
func TestEvaluate_In_NumSlice_AllPresent(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"speeds": numSliceLabel([]float32{1.0, 2.0}),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "speeds", Operator: clModels.In, Values: vals(float64(1.0), float64(2.0), float64(3.0))},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

// ── New: TestEvaluate_In_NumSlice_AllMeValuesInLabel ──────────────────────────
// Label [1.0,2.0,3.0], me.Values [1.0,2.0]:
// All me.Values present in label set {1.0,2.0,3.0} → PASS
func TestEvaluate_In_NumSlice_AllMeValuesInLabel(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"speeds": numSliceLabel([]float32{1.0, 2.0, 3.0}),
	}
	s := &clModels.Selector{
		MatchExpressions: []clModels.MatchExpression{
			{Key: "speeds", Operator: clModels.In, Values: vals(float64(1.0), float64(2.0))},
		},
	}
	ok, reason := newEngine(t, labels).Evaluate(s)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ── Corrected: TestHandleIn_StringSlice_AllElementsPresent ───────────────────
// Label ["a","b"], me.Values ["a","b","c"]:
// "c" is NOT in label set {a,b} → FAIL
func TestHandleIn_StringSlice_AllElementsPresent(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"tags": strSliceLabel([]string{"a", "b"}),
	}
	me := &clModels.MatchExpression{
		Key:      "tags",
		Operator: clModels.In,
		Values:   vals("a", "b", "c"),
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "c")
}

// ── New: TestHandleIn_StringSlice_AllMeValuesInLabel ─────────────────────────
// Label ["a","b","c"], me.Values ["a","b"]:
// All me.Values in label set {a,b,c} → PASS
func TestHandleIn_StringSlice_AllMeValuesInLabel(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"tags": strSliceLabel([]string{"a", "b", "c"}),
	}
	me := &clModels.MatchExpression{
		Key:      "tags",
		Operator: clModels.In,
		Values:   vals("a", "b"),
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ── Corrected: TestHandleIn_StringSlice_PartialMatch_Fails ───────────────────
// Label ["a","z"], me.Values ["a","b"]:
// "b" is NOT in label set {a,z} → FAIL; error names "b" (the failing me.Values element)
func TestHandleIn_StringSlice_PartialMatch_Fails(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"tags": strSliceLabel([]string{"a", "z"}),
	}
	me := &clModels.MatchExpression{
		Key:      "tags",
		Operator: clModels.In,
		Values:   vals("a", "b"),
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "b") // "b" is the me.Values element not found in label
}

// ── Corrected: TestHandleIn_NumSlice_AllElementsPresent ──────────────────────
// Label [1.0,2.0], me.Values [1.0,2.0,3.0]:
// 3.0 is NOT in label set {1.0,2.0} → FAIL
func TestHandleIn_NumSlice_AllElementsPresent(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"speeds": numSliceLabel([]float32{1.0, 2.0}),
	}
	me := &clModels.MatchExpression{
		Key:      "speeds",
		Operator: clModels.In,
		Values:   vals(float64(1.0), float64(2.0), float64(3.0)),
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.False(t, ok)
	assert.Contains(t, reason, "speeds")
}

// ── New: TestHandleIn_NumSlice_AllMeValuesInLabel ────────────────────────────
// Label [1.0,2.0,3.0], me.Values [1.0,2.0]:
// All me.Values in label set {1.0,2.0,3.0} → PASS
func TestHandleIn_NumSlice_AllMeValuesInLabel(t *testing.T) {
	labels := map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties{
		"speeds": numSliceLabel([]float32{1.0, 2.0, 3.0}),
	}
	me := &clModels.MatchExpression{
		Key:      "speeds",
		Operator: clModels.In,
		Values:   vals(float64(1.0), float64(2.0)),
	}
	ok, reason := newEngine(t, labels).HandleIn(me)
	assert.True(t, ok)
	assert.Empty(t, reason)
}
