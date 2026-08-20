// Package quantity provides parsing and comparison utilities for IEC binary
// resource quantity strings as defined by the Margo device constraints spec.
//
// Supported format: ^[0-9]+(Ki|Mi|Gi|Ti|Pi|Ei)$
//
// Examples: "512Mi", "10Gi", "1Ti"
package quantity

import (
	"fmt"
	"regexp"
	"strconv"
)

// quantityPattern is the canonical regex for a valid resource quantity string.
// It matches a non-negative integer followed by an IEC binary suffix.
var quantityPattern = regexp.MustCompile(`^([0-9]+)(Ki|Mi|Gi|Ti|Pi|Ei)$`)

// multipliers maps each IEC binary suffix to its byte equivalent.
// Ordered slice is not needed here because the regex already isolates the
// suffix group — there is no ambiguous prefix matching risk.
var multipliers = map[string]int64{
	"Ki": 1 << 10, // 1,024
	"Mi": 1 << 20, // 1,048,576
	"Gi": 1 << 30, // 1,073,741,824
	"Ti": 1 << 40, // 1,099,511,627,776
	"Pi": 1 << 50, // 1,125,899,906,842,624
	"Ei": 1 << 60, // 1,152,921,504,606,846,976
}

// Quantity represents a parsed IEC binary resource value in bytes.
type Quantity struct {
	raw   string
	bytes int64
}

// Parse parses a quantity string into a Quantity.
// Returns an error if the string does not match ^[0-9]+(Ki|Mi|Gi|Ti|Pi|Ei)$.
func Parse(s string) (Quantity, error) {
	matches := quantityPattern.FindStringSubmatch(s)
	if matches == nil {
		return Quantity{}, fmt.Errorf(
			"invalid quantity %q: must match ^[0-9]+(Ki|Mi|Gi|Ti|Pi|Ei)$", s,
		)
	}

	// matches[1] = numeric part, matches[2] = suffix
	n, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		// Unreachable in practice — the regex guarantees [0-9]+ — but guard anyway.
		return Quantity{}, fmt.Errorf("invalid quantity %q: failed to parse integer part: %w", s, err)
	}

	multiplier, ok := multipliers[matches[2]]
	if !ok {
		// Also unreachable given the regex, but keeps the map lookup safe.
		return Quantity{}, fmt.Errorf("invalid quantity %q: unknown suffix %q", s, matches[2])
	}

	return Quantity{
		raw:   s,
		bytes: n * multiplier,
	}, nil
}

// Bytes returns the quantity expressed as a raw int64 byte count.
func (q Quantity) Bytes() int64 {
	return q.bytes
}

// String returns the original unparsed quantity string.
func (q Quantity) String() string {
	return q.raw
}

// Cmp compares q against other.
//
//	Returns -1 if q <  other
//	Returns  0 if q == other
//	Returns +1 if q >  other
func (q Quantity) Cmp(other Quantity) int {
	switch {
	case q.bytes < other.bytes:
		return -1
	case q.bytes > other.bytes:
		return 1
	default:
		return 0
	}
}

// AtLeast reports whether q >= other (i.e. q satisfies the other as a minimum requirement).
func (q Quantity) AtLeast(other Quantity) bool {
	return q.bytes >= other.bytes
}
