package validators

import (
	"bytes"
	"crypto/x509"
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
func ValidateSpiffeTrustBundle(data []byte) error {
	if len(data) == 0 {
		return errors.New("trust bundle is empty")
	}

	// Pre-validate JSON structure and SPIFFE-specific fields
	if err := validateRawStructure(data); err != nil {
		return fmt.Errorf("invalid trust bundle structure: %w", err)
	}

	// Use the SPIFFE library to parse and validate the bundle.
	// spiffebundle.Read validates:
	//   - JWKS format correctness
	//   - x5c certificate chain parsing and validity
	//   - Key type support (EC, RSA)
	//   - SPIFFE-specific fields (spiffe_sequence, spiffe_refresh_hint)
	// Here, we do not need trust domain, hence passing empty object

	bundle, err := spiffebundle.Read(spiffeid.TrustDomain{}, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid SPIFFE trust bundle: %w", err)
	}

	// Ensure the bundle contains at least one X.509 authority
	if len(bundle.X509Authorities()) == 0 {
		return errors.New("trust bundle must contain at least one X.509 authority")
	}

	// At least one authority must be a valid CA cert:
	// - BasicConstraints must be present with cA=true
	// - KeyUsage, if present (non-zero), must include KeyCertSign
	hasValidAuthority := false
	for _, cert := range bundle.X509Authorities() {
		if !cert.IsCA {
			continue // ignore: cA not set in BasicConstraints
		}
		if cert.KeyUsage != 0 && cert.KeyUsage&x509.KeyUsageCertSign == 0 {
			continue // ignore: KeyUsage present but KeyCertSign not set
		}
		hasValidAuthority = true
		break
	}

	if !hasValidAuthority {
		return errors.New(
			"trust bundle contains no valid CA authority: at least one X.509 authority must have cA=true and, if key usage is present, must include keyCertSign",
		)
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
