package helpers

import "strings"

func GetHeaderValues(hv []string) []string {
	// detect presence of comma

	if len(hv) == 0 {
		return hv
	}

	if len(hv) > 1 {
		return hv
	}

	// break at ", "
	shv := strings.Split(hv[0], ", ")

	// return individual values
	return shv
}
