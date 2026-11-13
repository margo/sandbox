package misc

import (
	"encoding/json"
	"fmt"
)

// generic function to clone an object of any type T using JSON marshal and unmarshal
func CloneObject[T any](src *T) (*T, error) {
	if src == nil {
		return nil, nil
	}

	data, err := json.Marshal(src)
	if err != nil {
		return nil, fmt.Errorf("clone marshal failed: %w", err)
	}
	var dst T
	if err := json.Unmarshal(data, &dst); err != nil {
		return nil, fmt.Errorf("clone unmarshal failed: %w", err)
	}
	return &dst, nil
}
