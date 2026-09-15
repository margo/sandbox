package parser

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

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
