package set

type Set map[any]struct{}

// New creates a set from a slice.
func New(values ...any) Set {
	s := make(Set, len(values))
	for _, v := range values {
		s[v] = struct{}{}
	}
	return s
}

// FromSlice creates a set from a slice.
func FromSlice(values []any) Set {
	return New(values...)
}

// Add inserts a value.
func (s Set) Add(v any) {
	s[v] = struct{}{}
}

// Contains checks membership.
func (s Set) Contains(v any) bool {
	_, ok := s[v]
	return ok
}

// ToSlice converts the set to a slice.
func (s Set) ToSlice() []any {
	result := make([]any, 0, len(s))
	for v := range s {
		result = append(result, v)
	}
	return result
}

// Union returns a new set containing elements from both sets.
func (s Set) Union(other Set) Set {
	result := make(Set, len(s)+len(other))

	for v := range s {
		result[v] = struct{}{}
	}

	for v := range other {
		result[v] = struct{}{}
	}

	return result
}

// Intersection returns elements present in both sets.
func (s Set) Intersection(other Set) Set {
	// Iterate over smaller set.
	if len(other) < len(s) {
		s, other = other, s
	}

	result := make(Set)

	for v := range s {
		if _, ok := other[v]; ok {
			result[v] = struct{}{}
		}
	}

	return result
}

// Difference returns elements in s that are not in other.
func (s Set) Difference(other Set) Set {
	result := make(Set)

	for v := range s {
		if _, ok := other[v]; !ok {
			result[v] = struct{}{}
		}
	}

	return result
}

// IsSubset returns true if every element in s exists in other.
func (s Set) IsSubset(other Set) bool {
	if len(s) > len(other) {
		return false
	}

	for v := range s {
		if _, ok := other[v]; !ok {
			return false
		}
	}

	return true
}
