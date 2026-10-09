package unix

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/url"
	"strings"

	"github.com/margo/sandbox/mis/pkg/types"
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
func validateMintSVIDRequest(req *types.MintSVIDRequest) *ValidationError {
	ve := &ValidationError{Fields: make(map[string]string)}
	csr := false
	if req.CSR != "" {
		if err := validateCSR(req.CSR); err != nil {
			ve.Fields["csr"] = err.Error()
		} else {
			csr = true
		}
	}

	// Validate spiffeID (Required only if CSR is not present)
	if err := validateSpiffeID(req.SpiffeID); !csr && err != nil {
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
func validateSpiffeID(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("spiffeID is required")
	}

	parsed, err := url.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}

	if parsed.Scheme != "spiffe" {
		return fmt.Errorf("scheme must be 'spiffe', got '%s'", parsed.Scheme)
	}

	if parsed.Host == "" {
		return fmt.Errorf("trust domain (host) must not be empty")
	}

	if strings.Contains(parsed.Host, ":") {
		return fmt.Errorf("trust domain must not contain a port")
	}

	if parsed.User != nil {
		return fmt.Errorf("spiffeID must not contain user info")
	}

	if parsed.Fragment != "" {
		return fmt.Errorf("spiffeID must not contain a fragment")
	}

	if parsed.RawQuery != "" {
		return fmt.Errorf("spiffeID must not contain a query string")
	}

	return nil
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

	// Validate URI SANs: exactly one must be present and must be a valid SPIFFE ID
	if len(parsed.URIs) != 1 {
		return fmt.Errorf("CSR must contain exactly one URI SAN, got %d", len(parsed.URIs))
	}

	if err := validateSpiffeID(parsed.URIs[0].String()); err != nil {
		return fmt.Errorf("URI SAN is not a valid SPIFFE ID: %w", err)
	}

	return nil
}
