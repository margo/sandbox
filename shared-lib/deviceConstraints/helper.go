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
