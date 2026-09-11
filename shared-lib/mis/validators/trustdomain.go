package validators

import (
	"regexp"
	"strings"
)

// ValidateTrustDomain validates a trust domain.
// - Must be a top-level domain (e.g., "margo.org")
// - No scheme prefix (http://, https://, etc.)
// - No paths, ports, or query strings
func ValidateTrustDomain(domain string) bool {
	// Reject empty strings
	if strings.TrimSpace(domain) == "" {
		return false
	}

	// Reject if contains a scheme (http://, https://, ftp://, etc.)
	schemePattern := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+\-.]*://`)
	if schemePattern.MatchString(domain) {
		return false
	}

	// Reject if contains paths, ports, or query strings
	if strings.ContainsAny(domain, "/:?#@") {
		return false
	}

	// Validate domain: labels separated by dots, TLD required
	// Each label: 1-63 chars, alphanumeric or hyphens, cannot start/end with hyphen
	domainPattern := regexp.MustCompile(
		`^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`,
	)

	return domainPattern.MatchString(domain)
}
