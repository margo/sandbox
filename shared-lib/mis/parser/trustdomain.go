package parser

import (
	"fmt"
	"strings"
)

// extractTrustDomain parses the trust domain out of a fully-qualified SPIFFE ID.
// e.g. "spiffe://example.org/margo/wfm/abc" → "example.org"
func ParseTrustDomainFromSpiffeID(spiffeId string) (string, error) {
	const prefix = "spiffe://"
	if !strings.HasPrefix(spiffeId, prefix) {
		return "", fmt.Errorf("SPIFFE ID %q does not start with %q", spiffeId, prefix)
	}
	remainder := strings.TrimPrefix(spiffeId, prefix)
	slashIdx := strings.Index(remainder, "/")
	if slashIdx < 0 {
		return "", fmt.Errorf("SPIFFE ID %q has no path after the trust domain", spiffeId)
	}
	td := remainder[:slashIdx]
	if td == "" {
		return "", fmt.Errorf("SPIFFE ID %q has an empty trust domain", spiffeId)
	}
	return td, nil
}
