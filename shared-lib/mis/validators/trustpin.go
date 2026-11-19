package validators

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// ValidateTrustPin validates a single SPKI SHA-256 pin per RFC 7469 §2.4
// and miaf2.md §4. A valid pin has the form:
//
//	sha256/<base64-encoded-SHA-256-of-SPKI>
//
// where the base64 portion decodes to exactly 32 bytes.
func ValidateTrustPin(pin string) error {
	if !strings.HasPrefix(pin, "sha256/") {
		return fmt.Errorf("pin %q: must start with \"sha256/\" per RFC 7469 §2.4", pin)
	}

	encoded := strings.TrimPrefix(pin, "sha256/")
	if encoded == "" {
		return fmt.Errorf("pin %q: base64 portion must not be empty", pin)
	}

	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("pin %q: base64 portion is not valid standard base64: %w", pin, err)
	}

	if len(b) != 32 {
		return fmt.Errorf("pin %q: decoded to %d bytes, want 32 (SHA-256)", pin, len(b))
	}

	return nil
}
