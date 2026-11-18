package matcher

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-openapi/jsonpointer"
)

// normalizeJSON converts an arbitrary Go value into generic JSON types
// (map[string]any, []any, float64, string, bool, nil) by round-tripping through JSON.
// This normalizes numeric types to float64 and allows uniform jsonpointer traversal.
func normalizeJSON(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	var b []byte
	var err error
	switch data := v.(type) {
	case []byte:
		b = data
	case json.RawMessage:
		b = data
	default:
		b, err = json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON properties: %w", err)
		}
	}

	var doc any
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON properties: %w", err)
	}
	return doc, nil
}

// resolvePointer resolves an RFC 6901 JSON Pointer against a JSON document.
// The document should be normalized (e.g. from normalizeJSON).
// Returns:
//   - val: the resolved value (if exists)
//   - exists: true if the path exists in doc
//   - err: non-nil if the pointer syntax is invalid
func resolvePointer(doc any, pointer string) (any, bool, error) {
	if doc == nil {
		return nil, false, nil
	}

	// RFC 6901: empty string refers to the whole document.
	if pointer == "" {
		return doc, true, nil
	}

	if !strings.HasPrefix(pointer, "/") {
		return nil, false, fmt.Errorf("invalid JSON Pointer %q: must start with '/'", pointer)
	}

	jp, err := jsonpointer.New(pointer)
	if err != nil {
		return nil, false, fmt.Errorf("invalid JSON Pointer %q: %w", pointer, err)
	}

	val, _, err := jp.Get(doc)
	if err != nil {
		errMsg := err.Error()
		// Missing key, index out of range, or descending into nil are absent paths.
		if strings.Contains(errMsg, "object has no key") ||
			strings.Contains(errMsg, "index out of range") ||
			strings.Contains(errMsg, "nil value") {
			return nil, false, nil
		}
		// Any other navigation failure on absent elements is treated as absent.
		return nil, false, nil
	}

	return val, true, nil
}
