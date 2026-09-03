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
	o.logger.Debug("loading trust bundle", "trust_domain", o.trustDomain)

	// Parse trust domain
	td, err := spiffeid.TrustDomainFromString(o.trustDomain)
	if err != nil {
		o.logger.Error("invalid trust domain", "trust_domain", o.trustDomain, "err", err)
		return nil, fmt.Errorf("invalid trust domain %q: %w", o.trustDomain, err)
	}
	o.logger.Debug("trust domain parsed successfully", "trust_domain", td.String())

	// Load CA cert from disk
	o.logger.Debug("reading CA certificate from disk", "path", o.ca.Cert)
	caCertPEM, err := os.ReadFile(o.ca.Cert)
	if err != nil {
		o.logger.Error("failed to read CA certificate", "path", o.ca.Cert, "err", err)
		return nil, fmt.Errorf("failed to read CA cert %q: %w", o.ca.Cert, err)
	}
	o.logger.Debug(
		"CA certificate file read successfully",
		"path",
		o.ca.Cert,
		"size_bytes",
		len(caCertPEM),
	)

	// Decode PEM block
	block, _ := pem.Decode(caCertPEM)
	if block == nil {
		o.logger.Error("failed to decode PEM block", "path", o.ca.Cert)
		return nil, fmt.Errorf("failed to decode PEM block from %q", o.ca.Cert)
	}
	o.logger.Debug("PEM block decoded successfully", "type", block.Type)

	// Parse X.509 certificate
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		o.logger.Error("failed to parse CA certificate", "path", o.ca.Cert, "err", err)
		return nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}
	o.logger.Debug(
		"CA certificate parsed successfully",
		"subject", caCert.Subject.CommonName,
		"issuer", caCert.Issuer.CommonName,
		"not_before", caCert.NotBefore,
		"not_after", caCert.NotAfter,
	)

	// Create SPIFFE bundle for the trust domain
	bundle := spiffebundle.New(td)
	bundle.AddX509Authority(caCert)
	o.logger.Info(
		"trust bundle loaded successfully",
		"trust_domain",
		td.String(),
		"subject",
		caCert.Subject.CommonName,
	)

	return bundle, nil
}
