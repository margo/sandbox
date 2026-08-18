package deviceconstraints

import (
	"fmt"

	"github.com/margo/sandbox/shared-lib/set"
	clModels "github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
	"go.uber.org/zap"
)

type SelectorEngineIface interface {
	Evaluate(*clModels.Selector) (bool, string)
	HandleIn(*clModels.MatchExpression) (bool, string)
	HandleNotIn(*clModels.MatchExpression) (bool, string)
	HandleExists(*clModels.MatchExpression) (bool, string)
	HandleDoesNotExists(*clModels.MatchExpression) (bool, string)
	HandleGt(*clModels.MatchExpression) (bool, string)
	HandleLt(*clModels.MatchExpression) (bool, string)
}

type LabelSelectorEngine struct {
	logger *zap.SugaredLogger
	labels map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties
}

func NewLabelSelectorEngine(labels *map[string]clModels.DeviceCapabilitiesManifest_Labels_AdditionalProperties) SelectorEngineIface {
	return &LabelSelectorEngine{
		labels: *labels,
	}
}

func (ls *LabelSelectorEngine) Evaluate(s *clModels.Selector) (bool, string) {
	result, touched := false, false
	for _, me := range s.MatchExpressions {
		switch me.Operator {
		case clModels.In:
			ok, reason := ls.HandleIn(&me)
			result, touched, reason = buildResult(ok, reason, result, touched)
			if reason != "" {
				return false, reason
			}
		case clModels.NotIn:
			ok, reason := ls.HandleNotIn(&me)
			result, touched, reason = buildResult(ok, reason, result, touched)
			if reason != "" {
				return false, reason
			}
		case clModels.Exists:
			ok, reason := ls.HandleExists(&me)
			result, touched, reason = buildResult(ok, reason, result, touched)
			if reason != "" {
				return false, reason
			}
		case clModels.DoesNotExist:
			ok, reason := ls.HandleDoesNotExists(&me)
			result, touched, reason = buildResult(ok, reason, result, touched)
			if reason != "" {
				return false, reason
			}
		case clModels.Gt:
			ok, reason := ls.HandleGt(&me)
			result, touched, reason = buildResult(ok, reason, result, touched)
			if reason != "" {
				return false, reason
			}
		case clModels.Lt:
			ok, reason := ls.HandleLt(&me)
			result, touched, reason = buildResult(ok, reason, result, touched)
			if reason != "" {
				return false, reason
			}
		default:
			return false, fmt.Sprintf("unknown or unsupported labelSelector operator - %s", me.Operator)
		}
	}

	return result, ""
}

func (ls *LabelSelectorEngine) HandleIn(me *clModels.MatchExpression) (bool, string) {
	if me.Values == nil {
		return false, "invalid match expression, values is required for IN"
	}

	v, ok := ls.labels[me.Key]
	if !ok {
		return false, fmt.Sprintf("key %s not found in labels", me.Key)
	}

	mv := *me.Values

	// string
	if val, err := v.AsDeviceCapabilitiesManifestLabels0(); err == nil {
		for _, candidate := range mv {
			if s, ok := candidate.(string); ok && s == val {
				return true, ""
			}
		}
		return false, fmt.Sprintf("label %s value %q not found in match expression values", me.Key, val)
	}

	// float32 / number
	if val, err := v.AsDeviceCapabilitiesManifestLabels1(); err == nil {
		for _, candidate := range mv {
			switch c := candidate.(type) {
			case float64:
				if float32(c) == val {
					return true, ""
				}
			case float32:
				if c == val {
					return true, ""
				}
			}
		}
		return false, fmt.Sprintf("label %s value %v not found in match expression values", me.Key, val)
	}

	// bool
	if val, err := v.AsDeviceCapabilitiesManifestLabels2(); err == nil {
		for _, candidate := range mv {
			if b, ok := candidate.(bool); ok && b == val {
				return true, ""
			}
		}
		return false, fmt.Sprintf("label %s value %v not found in match expression values", me.Key, val)
	}

	// []string — every element of me.Values must appear in the label's array
	if labelArr, err := v.AsDeviceCapabilitiesManifestLabels3(); err == nil {
		if len(labelArr) == 0 {
			return false, fmt.Sprintf("label %s value is empty array, nothing to match against", me.Key)
		}
		// Build set from label's array
		labelSet := set.New()
		for _, lv := range labelArr {
			labelSet.Add(lv)
		}
		// Every me.Values element must be in the label set
		for _, candidate := range mv {
			if s, ok := candidate.(string); ok {
				if !labelSet.Contains(s) {
					return false, fmt.Sprintf("match expression value %q not found in label %s", s, me.Key)
				}
			}
		}
		return true, ""
	}

	// []float32 — every element of me.Values must appear in the label's array
	if labelArr, err := v.AsDeviceCapabilitiesManifestLabels4(); err == nil {
		// Build set from label's array (normalize to float32)
		labelSet := set.New()
		for _, lv := range labelArr {
			labelSet.Add(lv)
		}
		// Every me.Values element must be in the label set
		for _, candidate := range mv {
			var f float32
			switch c := candidate.(type) {
			case float64:
				f = float32(c)
			case float32:
				f = c
			default:
				continue
			}
			if !labelSet.Contains(f) {
				return false, fmt.Sprintf("match expression value %v not found in label %s", f, me.Key)
			}
		}
		return true, ""
	}

	return false, fmt.Sprintf("label %s has an unsupported value type", me.Key)
}

func (ls *LabelSelectorEngine) HandleNotIn(me *clModels.MatchExpression) (bool, string) {
	if me.Values == nil {
		return false, "invalid match expression, values is required for NotIn"
	}

	v, ok := ls.labels[me.Key]
	if !ok {
		return false, fmt.Sprintf("key %s not found in labels", me.Key)
	}

	mv := *me.Values

	// string
	if val, err := v.AsDeviceCapabilitiesManifestLabels0(); err == nil {
		for _, candidate := range mv {
			if s, ok := candidate.(string); ok && s == val {
				return false, fmt.Sprintf("label %s value %q is present in match expression values", me.Key, val)
			}
		}
		return true, ""
	}

	// float32 / number
	if val, err := v.AsDeviceCapabilitiesManifestLabels1(); err == nil {
		for _, candidate := range mv {
			switch c := candidate.(type) {
			case float64:
				if float32(c) == val {
					return false, fmt.Sprintf("label %s value %v is present in match expression values", me.Key, val)
				}
			case float32:
				if c == val {
					return false, fmt.Sprintf("label %s value %v is present in match expression values", me.Key, val)
				}
			}
		}
		return true, ""
	}

	// bool
	if val, err := v.AsDeviceCapabilitiesManifestLabels2(); err == nil {
		for _, candidate := range mv {
			if b, ok := candidate.(bool); ok && b == val {
				return false, fmt.Sprintf("label %s value %v is present in match expression values", me.Key, val)
			}
		}
		return true, ""
	}

	// []string — none of me.Values may appear in the label's array
	if labelArr, err := v.AsDeviceCapabilitiesManifestLabels3(); err == nil {
		if len(labelArr) == 0 {
			return false, fmt.Sprintf("label %s value is empty array, nothing to match against", me.Key)
		}
		// Build set from label's array
		labelSet := set.New()
		for _, lv := range labelArr {
			labelSet.Add(lv)
		}
		// None of me.Values elements may be in the label set
		for _, candidate := range mv {
			if s, ok := candidate.(string); ok {
				if labelSet.Contains(s) {
					return false, fmt.Sprintf("match expression value %q is present in label %s", s, me.Key)
				}
			}
		}
		return true, ""
	}

	// []float32 — none of me.Values may appear in the label's array
	if labelArr, err := v.AsDeviceCapabilitiesManifestLabels4(); err == nil {
		// Build set from label's array (normalize to float32)
		labelSet := set.New()
		for _, lv := range labelArr {
			labelSet.Add(lv)
		}
		// None of me.Values elements may be in the label set
		for _, candidate := range mv {
			var f float32
			switch c := candidate.(type) {
			case float64:
				f = float32(c)
			case float32:
				f = c
			default:
				continue
			}
			if labelSet.Contains(f) {
				return false, fmt.Sprintf("match expression value %v is present in label %s", f, me.Key)
			}
		}
		return true, ""
	}

	return false, fmt.Sprintf("label %s has an unsupported value type", me.Key)
}

func (ls *LabelSelectorEngine) HandleExists(me *clModels.MatchExpression) (bool, string) {
	if _, ok := ls.labels[me.Key]; !ok {
		return false, fmt.Sprintf("key %s not found in labels", me.Key)
	}
	return true, ""
}

func (ls *LabelSelectorEngine) HandleDoesNotExists(me *clModels.MatchExpression) (bool, string) {
	if _, ok := ls.labels[me.Key]; ok {
		return false, fmt.Sprintf("key %s is present in labels", me.Key)
	}
	return true, ""
}

func (ls *LabelSelectorEngine) HandleGt(me *clModels.MatchExpression) (bool, string) {
	if me.Values == nil || len(*me.Values) == 0 {
		return false, "invalid match expression, values is required for Gt"
	}

	mv := *me.Values

	// me.Values[0] must be an integer
	threshold, ok := mv[0].(float64)
	if !ok {
		return false, "invalid match expression, values[0] must be an integer for Gt"
	}
	if threshold != float64(int64(threshold)) {
		return false, fmt.Sprintf("invalid match expression, values[0] must be an integer for Gt, got %v", threshold)
	}

	v, ok := ls.labels[me.Key]
	if !ok {
		return false, fmt.Sprintf("key %s not found in labels", me.Key)
	}

	// float32 / number
	if val, err := v.AsDeviceCapabilitiesManifestLabels1(); err == nil {
		if float64(val) > threshold {
			return true, ""
		}
		return false, fmt.Sprintf("label %s value %v is not greater than %v", me.Key, val, int64(threshold))
	}

	return false, fmt.Sprintf("label %s has an unsupported value type for Gt, must be a number", me.Key)
}

func (ls *LabelSelectorEngine) HandleLt(me *clModels.MatchExpression) (bool, string) {
	if me.Values == nil || len(*me.Values) == 0 {
		return false, "invalid match expression, values is required for Lt"
	}

	mv := *me.Values

	// me.Values[0] must be an integer
	threshold, ok := mv[0].(float64)
	if !ok {
		return false, "invalid match expression, values[0] must be an integer for Lt"
	}
	if threshold != float64(int64(threshold)) {
		return false, fmt.Sprintf("invalid match expression, values[0] must be an integer for Lt, got %v", threshold)
	}

	v, ok := ls.labels[me.Key]
	if !ok {
		return false, fmt.Sprintf("key %s not found in labels", me.Key)
	}

	// float32 / number
	if val, err := v.AsDeviceCapabilitiesManifestLabels1(); err == nil {
		if float64(val) < threshold {
			return true, ""
		}
		return false, fmt.Sprintf("label %s value %v is not less than %v", me.Key, val, int64(threshold))
	}

	return false, fmt.Sprintf("label %s has an unsupported value type for Lt, must be a number", me.Key)
}
