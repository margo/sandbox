package deviceconstraints

import "fmt"

func GetSpecificSlice[T string | int](arr []any) []T {
	if len(arr) == 0 {
		return []T{}
	}

	fe := arr[0]
	t := fmt.Sprintf("%T", fe)
	result := make([]T, len(arr))
	switch t {
	case "string", "int":
		for _, a := range arr {
			result = append(result, a.(T))
		}

	}
	return result

}

func buildResult(ok bool, reason string, result, touched bool) (bool, bool, string) {
	if ok {
		if touched {
			result = result && ok
			return result, touched, ""
		}
		result = ok
		touched = true
		return result, touched, ""
	}
	return result, touched, reason
}

// toFloat64 coerces a scalar any value to float64.
// JSON-decoded numbers are always float64, but me.Values may contain float32
// from the generated model, so both numeric types are handled.
// Returns the float64 value and true on success, or 0 and false for non-numeric types.
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	default:
		return 0, false
	}
}

// scalarEqual compares two any values expected to be scalars
// (string, float64, float32, bool). JSON-decoded numbers are always float64,
// but me.Values may contain float32 from the generated model, so both are handled.
func scalarEqual(a, b any) bool {
	switch av := a.(type) {
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case float64:
		switch bv := b.(type) {
		case float64:
			return av == bv
		case float32:
			return av == float64(bv)
		}
		return false
	case float32:
		switch bv := b.(type) {
		case float64:
			return float64(av) == bv
		case float32:
			return av == bv
		}
		return false
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	default:
		return false
	}
}
