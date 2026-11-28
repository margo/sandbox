package parser

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const spiffeScheme = "spiffe://"

// ParseSpiffeIdFromX509Svid accepts an X.509 SVID in DER or PEM format ([]byte)
// and extracts the SPIFFE ID from its URI SAN.
// Returns the SPIFFE ID string or an error.
func ParseSpiffeIdFromX509Svid(certBytes []byte) (string, error) {
	if len(certBytes) == 0 {
		return "", errors.New("certificate bytes are empty")
	}

	cert, err := parseCertificate(certBytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse certificate: %w", err)
	}

	return extractSpiffeID(cert)
}

// parseCertificate attempts to parse the certificate as PEM first, then DER.
func parseCertificate(certBytes []byte) (*x509.Certificate, error) {
	// Try PEM decoding first
	block, _ := pem.Decode(certBytes)
	if block != nil {
		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("PEM block is not a certificate, got: %s", block.Type)
		}
		return x509.ParseCertificate(block.Bytes)
	}

	// Fall back to DER
	return x509.ParseCertificate(certBytes)
}

// extractSpiffeID finds and validates the SPIFFE ID in the certificate's URI SANs.
func extractSpiffeID(cert *x509.Certificate) (string, error) {
	if len(cert.URIs) == 0 {
		return "", errors.New("certificate has no URI SANs")
	}

	var spiffeIDs []string
	for _, uri := range cert.URIs {
		if strings.HasPrefix(uri.String(), "spiffe://") {
			spiffeIDs = append(spiffeIDs, uri.String())
		}
	}

	switch len(spiffeIDs) {
	case 0:
		return "", errors.New("no SPIFFE ID found in certificate URI SANs")
	case 1:
		return spiffeIDs[0], nil
	default:
		// Per SPIFFE spec, a valid SVID must have exactly one SPIFFE ID
		return "", fmt.Errorf("certificate contains multiple SPIFFE IDs: %v", spiffeIDs)
	}
}

// ParseWFMID extracts the <wfm-id> segment from a SPIFFE ID.
// Supports both WFM and WFM-Client patterns:
//   - spiffe://<trust-domain>/margo/wfm/<wfm-id>
//   - spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<wfm-client-id>
func ParseWFMID(spiffeID string) (string, error) {
	remainder := strings.TrimPrefix(spiffeID, spiffeScheme)
	// remainder: <trust-domain>/margo/wfm/<wfm-id>[/client/<wfm-client-id>]
	parts := strings.SplitN(remainder, "/", 5)
	// parts: [trust-domain, "margo", "wfm", <wfm-id>, ...]
	if len(parts) < 4 || parts[1] != "margo" || parts[2] != "wfm" || parts[3] == "" {
		return "", fmt.Errorf(
			"cannot extract wfm-id from SPIFFE ID %q: unexpected format",
			spiffeID,
		)
	}
	return parts[3], nil
}

// ParseWFMClientID parses a SPIFFE ID of the format:
//
//	spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<wfm-client-id>
//
// and returns the <wfm-client-id> segment.
// Returns an error if the input does not match the expected format.
func ParseWFMClientID(spiffeID string) (string, error) {
	u, err := url.Parse(spiffeID)
	if err != nil {
		return "", fmt.Errorf("invalid SPIFFE ID %q: %w", spiffeID, err)
	}

	if u.Scheme != "spiffe" {
		return "", fmt.Errorf(
			"invalid SPIFFE ID %q: scheme must be 'spiffe', got %q",
			spiffeID,
			u.Scheme,
		)
	}

	if u.Host == "" {
		return "", fmt.Errorf("invalid SPIFFE ID %q: trust domain is missing", spiffeID)
	}

	// Expected path: /margo/wfm/<wfm-id>/client/<wfm-client-id>
	// Split strips the leading slash, giving: ["margo", "wfm", <wfm-id>, "client", <wfm-client-id>]
	segments := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(segments) != 5 ||
		segments[0] != "margo" ||
		segments[1] != "wfm" ||
		segments[2] == "" ||
		segments[3] != "client" ||
		segments[4] == "" {
		return "", fmt.Errorf(
			"invalid SPIFFE ID %q: path must be /margo/wfm/<wfm-id>/client/<wfm-client-id>",
			spiffeID,
		)
	}

	return segments[4], nil
}

// extractTrustDomain parses the trust domain out of a fully-qualified SPIFFE ID.
// e.g. "spiffe://example.org/margo/wfm/abc" → "example.org"
func ParseTrustDomain(spiffeId string) (string, error) {
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

// ExtractWFMSpiffeID extracts the WFM SPIFFE ID prefix from a WFM client SPIFFE ID.
// Input format:  spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<wfm-client-id>
// Output format: spiffe://<trust-domain>/margo/wfm/<wfm-id>
func ExtractWFMSpiffeID(spiffeID string) (string, error) {
	// wfmClientPattern matches the full WFM client SPIFFE ID format and captures the WFM prefix.
	wfmClientPattern := regexp.MustCompile(`^(spiffe://[^/]+/margo/wfm/[^/]+)/client/[^/]+$`)
	matches := wfmClientPattern.FindStringSubmatch(spiffeID)
	if matches == nil {
		return "", fmt.Errorf(
			"invalid SPIFFE ID format: %q, expected spiffe://<trust-domain>/margo/wfm/<wfm-id>/client/<wfm-client-id>",
			spiffeID,
		)
	}
	return matches[1], nil
}
