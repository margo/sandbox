package unix

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/margo/sandbox/mis/pkg/types"
	"github.com/margo/sandbox/shared-lib/mis/validators"
)

// ValidationError holds a list of field-level validation errors
type ValidationError struct {
	Fields map[string]string
}

func (v *ValidationError) Error() string {
	var errs []string
	for field, msg := range v.Fields {
		errs = append(errs, fmt.Sprintf("%s: %s", field, msg))
	}
	return strings.Join(errs, "; ")
}

func (v *ValidationError) HasErrors() bool {
	return len(v.Fields) > 0
}

// ValidateMintSVIDRequest validates the incoming request fields
func (m *MintRestAPI) validateMintSVIDRequest(req *types.MintSVIDRequest) *ValidationError {
	ve := &ValidationError{Fields: make(map[string]string)}
	if req.CSR != "" {
		if err := validateCSR(req.CSR); err != nil {
			ve.Fields["csr"] = err.Error()
		}
	}

	// spiffeID must exist & be validated.
	if err := validateSpiffeID(req.SpiffeID, m.cnf.TrustDomain); err != nil {
		ve.Fields["spiffeID"] = err.Error()
	}

	// Validate TTL if provided
	if req.TTL != nil && *req.TTL < 0 {
		ve.Fields["ttl"] = "must be a non-negative integer"
	}

	// Validate DNS names if provided
	for i, dns := range req.DNS {
		if strings.TrimSpace(dns) == "" {
			ve.Fields[fmt.Sprintf("dns[%d]", i)] = "DNS name must not be empty"
		}
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}

// validateSpiffeID ensures the spiffeID follows the "spiffe://<trust-domain>/<path>" format
func validateSpiffeID(id, td string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("spiffeID is required")
	}

	principals := []string{validators.PrincipalWFM, validators.PrincipalWFMClient}

	for _, principal := range principals {
		if err := validators.ValidateSpiffeIDWithTrustDomain(id, td, principal); err == nil {
			return nil
		}
	}

	return fmt.Errorf(
		"invalid spiffe ID %q: must match either WFM format spiffe://<trust-domain>/margo/wfm/<wfm-id> "+
			"or WFM-Client format spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<wfm-client-id>"+
			"or does not belong to configured trust domain",
		id,
	)

}

// validateCSR parses and verifies the signature of a PEM or base64-encoded DER CSR.
func validateCSR(csr string) error {
	var derBytes []byte

	block, _ := pem.Decode([]byte(csr))
	if block != nil {
		if block.Type != "CERTIFICATE REQUEST" {
			return fmt.Errorf("PEM block type must be 'CERTIFICATE REQUEST', got '%s'", block.Type)
		}
		derBytes = block.Bytes
	} else {
		var err error
		derBytes, err = base64.StdEncoding.DecodeString(csr)
		if err != nil {
			return fmt.Errorf("must be a valid PEM or base64-encoded DER CSR")
		}
	}

	parsed, err := x509.ParseCertificateRequest(derBytes)
	if err != nil {
		return fmt.Errorf("invalid CSR: %w", err)
	}

	if err := parsed.CheckSignature(); err != nil {
		return fmt.Errorf("CSR signature verification failed: %w", err)
	}

	return nil
}
