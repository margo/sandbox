package matcher

import (
	"encoding/json"
	"fmt"

	clModels "github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
)

// toFloat64 coerces an any value to float64 if it is a numeric type.
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// scalarEqual compares two any values expected to be scalars (string, number, bool).
func scalarEqual(a, b any) bool {
	switch av := a.(type) {
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	default:
		af, aOk := toFloat64(a)
		bf, bOk := toFloat64(b)
		if aOk && bOk {
			return af == bf
		}
		return false
	}
}

// validateValues checks that values conforms to the spec rules for In/NotIn:
//   - values MUST be present and contain one or more entries
//   - booleans MUST have exactly one value
//   - all entries MUST be the same data type (string, number, or bool)
func validateValues(operator string, values *[]any) (string, bool) {
	if values == nil || len(*values) == 0 {
		return fmt.Sprintf("invalid match expression: values is required for %s", operator), false
	}

	candidates := *values
	first := candidates[0]

	var dominantType string
	switch first.(type) {
	case string:
		dominantType = "string"
	case bool:
		dominantType = "bool"
	default:
		if _, ok := toFloat64(first); ok {
			dominantType = "number"
		} else {
			return fmt.Sprintf("invalid match expression: unsupported value type %T in values", first), false
		}
	}

	if dominantType == "bool" && len(candidates) != 1 {
		return fmt.Sprintf("invalid match expression: values MUST contain exactly one boolean for %s", operator), false
	}

	for i, c := range candidates {
		var elemType string
		switch c.(type) {
		case string:
			elemType = "string"
		case bool:
			elemType = "bool"
		default:
			if _, ok := toFloat64(c); ok {
				elemType = "number"
			} else {
				return fmt.Sprintf("invalid match expression: unsupported value type %T at values[%d]", c, i), false
			}
		}
		if elemType != dominantType {
			return fmt.Sprintf(
				"invalid match expression: values MUST be the same data type, got %s at values[0] but %s at values[%d]",
				dominantType, elemType, i,
			), false
		}
	}

	return "", true
}

// validateNumericValues checks that values conforms to the spec rules for Gt/GtEq/Lt/LtEq:
//   - values MUST be present and contain exactly one numeric entry
func validateNumericValues(operator string, values *[]any) (float64, string, bool) {
	if values == nil || len(*values) == 0 {
		return 0, fmt.Sprintf("invalid match expression: values is required for %s", operator), false
	}

	candidates := *values
	if len(candidates) != 1 {
		return 0, fmt.Sprintf(
			"invalid match expression: values MUST contain exactly one number for %s, got %d",
			operator, len(candidates),
		), false
	}

	val, ok := toFloat64(candidates[0])
	if !ok {
		return 0, fmt.Sprintf(
			"invalid match expression: values[0] MUST be a number for %s, got %T",
			operator, candidates[0],
		), false
	}

	return val, "", true
}

// evaluateExists evaluates the Exists operator.
func evaluateExists(exists bool, values *[]any, propertyDesc string) (bool, string) {
	if values != nil && len(*values) > 0 {
		return false, "invalid match expression: values MUST be omitted for Exists"
	}
	if !exists {
		return false, fmt.Sprintf("%s not found", propertyDesc)
	}
	return true, ""
}

// evaluateDoesNotExist evaluates the DoesNotExist operator.
func evaluateDoesNotExist(exists bool, values *[]any, propertyDesc string) (bool, string) {
	if values != nil && len(*values) > 0 {
		return false, "invalid match expression: values MUST be omitted for DoesNotExist"
	}
	if exists {
		return false, fmt.Sprintf("%s is present", propertyDesc)
	}
	return true, ""
}

// evaluateIn evaluates the In operator.
func evaluateIn(resolved any, exists bool, values *[]any, propertyDesc string) (bool, string) {
	if reason, ok := validateValues("In", values); !ok {
		return false, reason
	}
	if !exists {
		return false, fmt.Sprintf("%s not found", propertyDesc)
	}

	candidates := *values
	switch val := resolved.(type) {
	case string:
		for _, c := range candidates {
			if s, ok := c.(string); ok && s == val {
				return true, ""
			}
		}
		return false, fmt.Sprintf("%s value %q not found in match expression values", propertyDesc, val)

	case bool:
		for _, c := range candidates {
			if b, ok := c.(bool); ok && b == val {
				return true, ""
			}
		}
		return false, fmt.Sprintf("%s value %v not found in match expression values", propertyDesc, val)

	case []any:
		if len(val) == 0 {
			return false, fmt.Sprintf("%s is an empty array, nothing to match against", propertyDesc)
		}
		if _, isObj := val[0].(map[string]any); isObj {
			return false, fmt.Sprintf("%s is an array of objects; use ContainsAll or ContainsAny instead of In", propertyDesc)
		}
		for _, elem := range val {
			for _, c := range candidates {
				if scalarEqual(elem, c) {
					return true, ""
				}
			}
		}
		return false, fmt.Sprintf("no element of %s matched any of the match expression values", propertyDesc)

	default:
		if numVal, ok := toFloat64(resolved); ok {
			for _, c := range candidates {
				if cNum, cOk := toFloat64(c); cOk && cNum == numVal {
					return true, ""
				}
			}
			return false, fmt.Sprintf("%s value %v not found in match expression values", propertyDesc, numVal)
		}
		return false, fmt.Sprintf("%s resolved to an unsupported type (%T); In requires string, number, bool, or array thereof", propertyDesc, resolved)
	}
}

// evaluateNotIn evaluates the NotIn operator.
func evaluateNotIn(resolved any, exists bool, values *[]any, propertyDesc string) (bool, string) {
	if reason, ok := validateValues("NotIn", values); !ok {
		return false, reason
	}
	if !exists {
		return false, fmt.Sprintf("%s not found", propertyDesc)
	}

	candidates := *values
	switch val := resolved.(type) {
	case string:
		for _, c := range candidates {
			if s, ok := c.(string); ok && s == val {
				return false, fmt.Sprintf("%s value %q is present in match expression values", propertyDesc, val)
			}
		}
		return true, ""

	case bool:
		for _, c := range candidates {
			if b, ok := c.(bool); ok && b == val {
				return false, fmt.Sprintf("%s value %v is present in match expression values", propertyDesc, val)
			}
		}
		return true, ""

	case []any:
		if len(val) == 0 {
			return false, fmt.Sprintf("%s is an empty array, nothing to match against", propertyDesc)
		}
		if _, isObj := val[0].(map[string]any); isObj {
			return false, fmt.Sprintf("%s is an array of objects; use ContainsAll or ContainsAny instead of NotIn", propertyDesc)
		}
		for _, elem := range val {
			for _, c := range candidates {
				if scalarEqual(elem, c) {
					return false, fmt.Sprintf("%s contains a value that is present in match expression values", propertyDesc)
				}
			}
		}
		return true, ""

	default:
		if numVal, ok := toFloat64(resolved); ok {
			for _, c := range candidates {
				if cNum, cOk := toFloat64(c); cOk && cNum == numVal {
					return false, fmt.Sprintf("%s value %v is present in match expression values", propertyDesc, numVal)
				}
			}
			return true, ""
		}
		return false, fmt.Sprintf("%s resolved to an unsupported type (%T); NotIn requires string, number, bool, or array thereof", propertyDesc, resolved)
	}
}

// evaluateGt evaluates the Gt operator.
func evaluateGt(resolved any, exists bool, values *[]any, propertyDesc string) (bool, string) {
	threshold, reason, ok := validateNumericValues("Gt", values)
	if !ok {
		return false, reason
	}
	if !exists {
		return false, fmt.Sprintf("%s not found", propertyDesc)
	}

	val, ok := toFloat64(resolved)
	if !ok {
		return false, fmt.Sprintf("%s resolved to a non-numeric type (%T); Gt requires a number", propertyDesc, resolved)
	}

	if val > threshold {
		return true, ""
	}
	return false, fmt.Sprintf("%s value %v is not greater than %v", propertyDesc, val, threshold)
}

// evaluateGtEq evaluates the GtEq operator.
func evaluateGtEq(resolved any, exists bool, values *[]any, propertyDesc string) (bool, string) {
	threshold, reason, ok := validateNumericValues("GtEq", values)
	if !ok {
		return false, reason
	}
	if !exists {
		return false, fmt.Sprintf("%s not found", propertyDesc)
	}

	val, ok := toFloat64(resolved)
	if !ok {
		return false, fmt.Sprintf("%s resolved to a non-numeric type (%T); GtEq requires a number", propertyDesc, resolved)
	}

	if val >= threshold {
		return true, ""
	}
	return false, fmt.Sprintf("%s value %v is not greater than or equal to %v", propertyDesc, val, threshold)
}

// evaluateLt evaluates the Lt operator.
func evaluateLt(resolved any, exists bool, values *[]any, propertyDesc string) (bool, string) {
	threshold, reason, ok := validateNumericValues("Lt", values)
	if !ok {
		return false, reason
	}
	if !exists {
		return false, fmt.Sprintf("%s not found", propertyDesc)
	}

	val, ok := toFloat64(resolved)
	if !ok {
		return false, fmt.Sprintf("%s resolved to a non-numeric type (%T); Lt requires a number", propertyDesc, resolved)
	}

	if val < threshold {
		return true, ""
	}
	return false, fmt.Sprintf("%s value %v is not less than %v", propertyDesc, val, threshold)
}

// evaluateLtEq evaluates the LtEq operator.
func evaluateLtEq(resolved any, exists bool, values *[]any, propertyDesc string) (bool, string) {
	threshold, reason, ok := validateNumericValues("LtEq", values)
	if !ok {
		return false, reason
	}
	if !exists {
		return false, fmt.Sprintf("%s not found", propertyDesc)
	}

	val, ok := toFloat64(resolved)
	if !ok {
		return false, fmt.Sprintf("%s resolved to a non-numeric type (%T); LtEq requires a number", propertyDesc, resolved)
	}

	if val <= threshold {
		return true, ""
	}
	return false, fmt.Sprintf("%s value %v is not less than or equal to %v", propertyDesc, val, threshold)
}

// evaluateItemMatchExpression evaluates an individual ItemMatchExpression against a single array element.
func evaluateItemMatchExpression(element any, expr clModels.ItemMatchExpression) (bool, string, error) {
	resolved, exists, err := resolvePointer(element, expr.Property)
	if err != nil {
		return false, "", err
	}

	propDesc := fmt.Sprintf("item property %q", expr.Property)

	switch expr.Operator {
	case clModels.ItemMatchExpressionOperatorExists:
		ok, reason := evaluateExists(exists, expr.Values, propDesc)
		return ok, reason, nil
	case clModels.ItemMatchExpressionOperatorDoesNotExist:
		ok, reason := evaluateDoesNotExist(exists, expr.Values, propDesc)
		return ok, reason, nil
	case clModels.ItemMatchExpressionOperatorIn:
		ok, reason := evaluateIn(resolved, exists, expr.Values, propDesc)
		return ok, reason, nil
	case clModels.ItemMatchExpressionOperatorNotIn:
		ok, reason := evaluateNotIn(resolved, exists, expr.Values, propDesc)
		return ok, reason, nil
	case clModels.ItemMatchExpressionOperatorGt:
		ok, reason := evaluateGt(resolved, exists, expr.Values, propDesc)
		return ok, reason, nil
	case clModels.ItemMatchExpressionOperatorGtEq:
		ok, reason := evaluateGtEq(resolved, exists, expr.Values, propDesc)
		return ok, reason, nil
	case clModels.ItemMatchExpressionOperatorLt:
		ok, reason := evaluateLt(resolved, exists, expr.Values, propDesc)
		return ok, reason, nil
	case clModels.ItemMatchExpressionOperatorLtEq:
		ok, reason := evaluateLtEq(resolved, exists, expr.Values, propDesc)
		return ok, reason, nil
	default:
		return false, "", fmt.Errorf("unsupported item selector operator: %s", expr.Operator)
	}
}
