package validators

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

// SpiffeTrustBundle is used to pre-validate the raw structure before passing to the SPIFFE library.
type SpiffeTrustBundle struct {
	Keys              []json.RawMessage `json:"keys"`
	SpiffeSequence    *int64            `json:"spiffe_sequence,omitempty"`
	SpiffeRefreshHint *int64            `json:"spiffe_refresh_hint,omitempty"`
}

// ValidateSpiffeTrustBundle validates a SPIFFE Trust Bundle in JWKS format using the go-spiffe library.
// trustDomain should be the SPIFFE trust domain, e.g. "example.org"
func ValidateSpiffeTrustBundle(data []byte, trustDomain string) error {
	if len(data) == 0 {
		return errors.New("trust bundle is empty")
	}

	// Pre-validate JSON structure and SPIFFE-specific fields
	if err := validateRawStructure(data); err != nil {
		return fmt.Errorf("invalid trust bundle structure: %w", err)
	}

	// Parse the trust domain
	td, err := spiffeid.TrustDomainFromString(trustDomain)
	if err != nil {
		return fmt.Errorf("invalid trust domain %q: %w", trustDomain, err)
	}

	// Use the SPIFFE library to parse and validate the bundle.
	// spiffebundle.Read validates:
	//   - JWKS format correctness
	//   - x5c certificate chain parsing and validity
	//   - Key type support (EC, RSA)
	//   - SPIFFE-specific fields (spiffe_sequence, spiffe_refresh_hint)
	bundle, err := spiffebundle.Read(td, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid SPIFFE trust bundle: %w", err)
	}

	// Ensure the bundle contains at least one X.509 authority
	if len(bundle.X509Authorities()) == 0 {
		return errors.New("trust bundle must contain at least one X.509 authority")
	}

	return nil
}

// validateRawStructure performs lightweight pre-validation of the raw JSON
// before handing off to the SPIFFE library.
func validateRawStructure(data []byte) error {
	var raw SpiffeTrustBundle
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	if len(raw.Keys) == 0 {
		return errors.New("trust bundle must contain at least one key")
	}

	if raw.SpiffeRefreshHint != nil && *raw.SpiffeRefreshHint < 0 {
		return errors.New("spiffe_refresh_hint must be a non-negative integer")
	}

	if raw.SpiffeSequence != nil && *raw.SpiffeSequence < 0 {
		return errors.New("spiffe_sequence must be a non-negative integer")
	}

	return nil
}
