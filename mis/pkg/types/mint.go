package types

import "time"

// MintSVIDRequest represents the JSON request body for /mint/svid/x509
type MintSVIDRequest struct {
	DNS      []string       `json:"dns,omitempty"` // Optional: DNS SANs to include in SVID
	SpiffeID string         `json:"spiffeID"`      // Required: must follow "spiffe://<trust-domain>/<path>"
	TTL      *time.Duration `json:"ttl,omitempty"` // Optional: seconds; defaults to 86400 (24h)
}

// MintSVIDResponse represents the JSON response body
type MintSVIDResponse struct {
	Certificate string `json:"certificate"` // Base64-encoded PEM certificate
	Key         string `json:"key"`         // Base64-encoded PEM private key
}

// ErrorResponse represents a JSON error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// DefaultTTL is 24 hours in seconds
const DefaultTTL = 24 * 60 * 60

// ResolvedTTL returns the TTL duration, applying the default if not set
func (r *MintSVIDRequest) ResolvedTTL() time.Duration {
	if r.TTL == nil || *r.TTL <= 0 {
		return DefaultTTL * time.Second
	}
	return *r.TTL
}
