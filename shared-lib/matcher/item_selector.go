package matcher

import (
	"fmt"

	clModels "github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
)

// evaluateContainsAll evaluates the ContainsAll operator on an array property.
// Per Margo spec:
//   - values MUST be omitted.
//   - itemSelector MUST be present and contain at least one matchExpression.
//   - The property MUST resolve to an array of objects.
//   - True when AT LEAST ONE array element satisfies ALL itemSelector.matchExpressions (AND logic).
func evaluateContainsAll(
	resolved any,
	exists bool,
	me *clModels.MatchExpression,
	propertyDesc string,
) (bool, string, error) {
	if me.Values != nil && len(*me.Values) > 0 {
		return false, "invalid match expression: values MUST be omitted for ContainsAll", nil
	}

	if me.ItemSelector == nil || len(me.ItemSelector.MatchExpressions) == 0 {
		return false, fmt.Sprintf(
			"invalid match expression: itemSelector with at least one matchExpression is required for ContainsAll (%s)",
			propertyDesc,
		), nil
	}

	if !exists {
		return false, fmt.Sprintf("%s not found", propertyDesc), nil
	}

	arr, ok := resolved.([]any)
	if !ok {
		return false, fmt.Sprintf(
			"%s resolved to a non-array type (%T); ContainsAll requires an array of objects",
			propertyDesc, resolved,
		), nil
	}

	if len(arr) == 0 {
		return false, fmt.Sprintf("%s is an empty array, nothing to match against", propertyDesc), nil
	}

	// First element must be an object
	if _, isObj := arr[0].(map[string]any); !isObj {
		return false, fmt.Sprintf(
			"%s is an array of scalars; ContainsAll requires an array of objects",
			propertyDesc,
		), nil
	}

	for _, elem := range arr {
		elemMatchesAll := true
		for _, itemExpr := range me.ItemSelector.MatchExpressions {
			ok, _, err := evaluateItemMatchExpression(elem, itemExpr)
			if err != nil {
				return false, "", err
			}
			if !ok {
				elemMatchesAll = false
				break
			}
		}
		if elemMatchesAll {
			return true, "", nil
		}
	}

	return false, fmt.Sprintf("no element of %s satisfied all itemSelector.matchExpressions", propertyDesc), nil
}

// evaluateContainsAny evaluates the ContainsAny operator on an array property.
// Per Margo spec:
//   - values MUST be omitted.
//   - itemSelector MUST be present and contain at least one matchExpression.
//   - The property MUST resolve to an array of objects.
//   - True when AT LEAST ONE array element satisfies ANY itemSelector.matchExpression (OR logic).
func evaluateContainsAny(
	resolved any,
	exists bool,
	me *clModels.MatchExpression,
	propertyDesc string,
) (bool, string, error) {
	if me.Values != nil && len(*me.Values) > 0 {
		return false, "invalid match expression: values MUST be omitted for ContainsAny", nil
	}

	if me.ItemSelector == nil || len(me.ItemSelector.MatchExpressions) == 0 {
		return false, fmt.Sprintf(
			"invalid match expression: itemSelector with at least one matchExpression is required for ContainsAny (%s)",
			propertyDesc,
		), nil
	}

	if !exists {
		return false, fmt.Sprintf("%s not found", propertyDesc), nil
	}

	arr, ok := resolved.([]any)
	if !ok {
		return false, fmt.Sprintf(
			"%s resolved to a non-array type (%T); ContainsAny requires an array of objects",
			propertyDesc, resolved,
		), nil
	}

	if len(arr) == 0 {
		return false, fmt.Sprintf("%s is an empty array, nothing to match against", propertyDesc), nil
	}

	// First element must be an object
	if _, isObj := arr[0].(map[string]any); !isObj {
		return false, fmt.Sprintf(
			"%s is an array of scalars; ContainsAny requires an array of objects",
			propertyDesc,
		), nil
	}

	for _, elem := range arr {
		for _, itemExpr := range me.ItemSelector.MatchExpressions {
			ok, _, err := evaluateItemMatchExpression(elem, itemExpr)
			if err != nil {
				return false, "", err
			}
			if ok {
				return true, "", nil
			}
		}
	}

	return false, fmt.Sprintf("no element of %s satisfied any itemSelector.matchExpression", propertyDesc), nil
}
