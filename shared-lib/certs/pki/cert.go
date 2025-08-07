// Package pki provides PKI-based device authentication using X.509 certificates.
package pki

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"
)

// CertificateManager handles X.509 certificate operations including parsing,
// validation, and chain verification against trusted CAs.
type CertificateManager struct {
	trustedCAs []*x509.Certificate
}

// NewCertificateManager creates a new certificate manager with the given CA certificates.
// All CA certificates must be valid PEM-encoded X.509 certificates.
func NewCertificateManager(caPEMs [][]byte) (*CertificateManager, error) {
	var cas []*x509.Certificate

	for _, caPEM := range caPEMs {
		ca, err := parseCertificateFromPEM(caPEM)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CA certificate: %w", err)
		}
		cas = append(cas, ca)
	}

	return &CertificateManager{trustedCAs: cas}, nil
}

// ParseDeviceCertificate parses a PEM-encoded X.509 certificate.
func (cm *CertificateManager) ParseDeviceCertificate(certPEM []byte) (*x509.Certificate, error) {
	return parseCertificateFromPEM(certPEM)
}

// VerifyCertificateChain verifies the certificate against the trusted CA certificates.
// The certificate must be valid for client authentication.
func (cm *CertificateManager) VerifyCertificateChain(cert *x509.Certificate) error {
	roots := x509.NewCertPool()
	for _, ca := range cm.trustedCAs {
		roots.AddCert(ca)
	}

	opts := x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	_, err := cert.Verify(opts)
	return err
}

// ValidateCertificateExpiry checks if the certificate is within its validity period.
func (cm *CertificateManager) ValidateCertificateExpiry(cert *x509.Certificate) error {
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("certificate not yet valid")
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("certificate has expired")
	}
	return nil
}

// ExtractDeviceID extracts the device identifier from the certificate's Common Name
// or DNS Subject Alternative Names. Returns an error if no device ID is found.
func (cm *CertificateManager) ExtractDeviceID(cert *x509.Certificate) (string, error) {
	// Try Common Name first
	if cert.Subject.CommonName != "" {
		return cert.Subject.CommonName, nil
	}

	// Try Subject Alternative Names
	for _, name := range cert.DNSNames {
		if name != "" {
			return name, nil
		}
	}

	return "", fmt.Errorf("no device ID found in certificate")
}

// parseCertificateFromPEM parses a PEM-encoded certificate block into an X.509 certificate.
func parseCertificateFromPEM(certPEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert, nil
}
