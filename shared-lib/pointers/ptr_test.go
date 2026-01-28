package pointers

import (
	"testing"
)

func TestPtr(t *testing.T) {
	s := "test"
	ptr := Ptr(s)
	if *ptr != s {
		t.Errorf("Expected %s, got %s", s, *ptr)
	}
}

func TestPtrOrNil(t *testing.T) {
	// Non-zero value
	if ptr := PtrOrNil("test"); ptr == nil || *ptr != "test" {
		t.Error("Expected pointer to 'test'")
	}

	// Zero value
	if ptr := PtrOrNil(""); ptr != nil {
		t.Error("Expected nil for empty string")
	}
}

func TestDeref(t *testing.T) {
	// Non-nil pointer
	s := "test"
	if result := Deref(&s); result != s {
		t.Errorf("Expected %s, got %s", s, result)
	}

	// Nil pointer
	if result := Deref((*string)(nil)); result != "" {
		t.Errorf("Expected empty string, got %s", result)
	}
}

func TestEqual(t *testing.T) {
	a := Ptr("test")
	b := Ptr("test")
	c := Ptr("different")

	if !Equal(a, b) {
		t.Error("Expected equal pointers")
	}

	if Equal(a, c) {
		t.Error("Expected unequal pointers")
	}

	if !Equal((*string)(nil), (*string)(nil)) {
		t.Error("Expected nil pointers to be equal")
	}
}
