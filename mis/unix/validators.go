package unix

import (
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

	// Validate spiffeID (Required)
	if err := validateSpiffeID(req.SpiffeID); err != nil {
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
