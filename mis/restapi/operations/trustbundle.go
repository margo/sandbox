package operations

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

// GetTrustBundle loads the CA cert and returns a SPIFFE BundleSet (Bundle Map)
// for the given trust domain.
func (o *Operation) GetTrustBundle() (*spiffebundle.Bundle, error) {
	// Parse trust domain
	td, err := spiffeid.TrustDomainFromString(o.trustDomain)
	if err != nil {
		return nil, fmt.Errorf("invalid trust domain %q: %w", o.trustDomain, err)
	}

	// Load CA cert from disk
	caCertPEM, err := os.ReadFile(o.ca.Cert)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert %q: %w", o.ca.Cert, err)
	}

	// Decode PEM block
	block, _ := pem.Decode(caCertPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from %q", o.ca.Cert)
	}

	// Parse X.509 certificate
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	// Create SPIFFE bundle for the trust domain
	bundle := spiffebundle.New(td)
	bundle.AddX509Authority(caCert)

	return bundle, nil
}
