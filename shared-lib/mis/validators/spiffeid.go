package validators

import (
	"fmt"
	"regexp"
	"strings"
)

// Principal constants
const (
	PrincipalWFM       = "wfm"
	PrincipalWFMClient = "wfm-client"
)

const spiffeScheme = "spiffe://"

// Precompiled regex patterns for SPIFFE ID validation
var (
	// trustDomainPattern: lowercase letters, digits, dots, dashes, underscores; max 255 chars
	trustDomainPattern = regexp.MustCompile(`^[a-z0-9._-]{1,255}$`)

	// pathSegmentPattern: valid characters in a path segment (no slash)
	pathSegmentPattern = regexp.MustCompile(`^[a-zA-Z0-9._~!$&'()*+,;=:@%-]+$`)

	// wfmPattern: spiffe://<trust-domain>/margo/wfm/<wfm-id>
	wfmPattern = regexp.MustCompile(`^spiffe://[^/]+/margo/wfm/[^/]+$`)

	// wfmClientPattern: spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<wfm-client-id>
	wfmClientPattern = regexp.MustCompile(`^spiffe://[^/]+/margo/wfm/[^/]+/client/[^/]+$`)
)

// ValidateSpiffeID validates whether a string is a valid SPIFFE ID for the given principal.
// principal must be one of PrincipalWFM or PrincipalWFMClient.
func ValidateSpiffeID(spiffeID, principal string) error {
	// Must start with spiffe://
	if !strings.HasPrefix(spiffeID, spiffeScheme) {
		return fmt.Errorf("spiffe ID must start with %q", spiffeScheme)
	}

	// Extract trust domain and path
	remainder := strings.TrimPrefix(spiffeID, spiffeScheme)
	before, after, ok := strings.Cut(remainder, "/")
	if !ok {
		return fmt.Errorf("spiffe ID must contain a path after the trust domain")
	}

	trustDomain := before
	if !trustDomainPattern.MatchString(trustDomain) {
		return fmt.Errorf(
			"invalid trust domain %q: must be lowercase alphanumeric with '.', '-', '_'; max 255 chars",
			trustDomain,
		)
	}

	// Validate path segments
	path := after
	segments := strings.SplitSeq(path, "/")
	for seg := range segments {
		if seg == "" {
			return fmt.Errorf("spiffe ID path must not contain empty segments")
		}
		if !pathSegmentPattern.MatchString(seg) {
			return fmt.Errorf("invalid path segment %q in spiffe ID", seg)
		}
	}

	// Validate principal-specific pattern
	switch principal {
	case PrincipalWFM:
		if !wfmPattern.MatchString(spiffeID) {
			return fmt.Errorf(
				"invalid WFM spiffe ID: expected format spiffe://<trust-domain>/margo/wfm/<wfm-id>, got %q",
				spiffeID,
			)
		}
	case PrincipalWFMClient:
		if !wfmClientPattern.MatchString(spiffeID) {
			return fmt.Errorf(
				"invalid WFM-Client spiffe ID: expected format spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<wfm-client-id>, got %q",
				spiffeID,
			)
		}
	default:
		return fmt.Errorf(
			"unknown principal %q: must be %q or %q",
			principal,
			PrincipalWFM,
			PrincipalWFMClient,
		)
	}

	return nil
}

// ValidateSpiffeIDWithTrustDomain validates a SPIFFE ID for a given principal
// and additionally checks that the trust domain matches the expected value.
func ValidateSpiffeIDWithTrustDomain(spiffeID, trustDomain, principal string) error {
	// Reuse base validation first
	if err := ValidateSpiffeID(spiffeID, principal); err != nil {
		return err
	}

	// Extract and compare trust domain
	remainder := strings.TrimPrefix(spiffeID, spiffeScheme)
	slashIdx := strings.Index(remainder, "/")
	extractedTrustDomain := remainder[:slashIdx]

	if extractedTrustDomain != trustDomain {
		return fmt.Errorf(
			"trust domain mismatch: expected %q, got %q",
			trustDomain, extractedTrustDomain,
		)
	}

	return nil
}
