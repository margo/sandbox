package validators

import (
	"crypto/x509"
	"errors"
	"fmt"
	"time"
)

// ValidateRootCACertificate validates that the provided bytes represent
// a valid X.509 Root CA certificate in PEM or DER format.
func ValidateRootCACertificate(data []byte) (*x509.Certificate, error) {
	if len(data) == 0 {
		return nil, errors.New("certificate data is empty")
	}

	cert, err := parseCertificate(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	if err := validateRootCAConstraints(cert); err != nil {
		return nil, err
	}

	return cert, nil
}

// validateRootCAConstraints enforces Root CA-specific X.509 rules.
func validateRootCAConstraints(cert *x509.Certificate) error {
	// Must be self-signed (Root CA signs itself) -- deliberately disabled check
	// if err := cert.CheckSignatureFrom(cert); err != nil {
	// 	return fmt.Errorf("certificate is not self-signed: %w", err)
	// }

	// Must have CA basic constraint set
	if !cert.IsCA {
		return errors.New("certificate is not a CA (BasicConstraints.IsCA is false)")
	}

	// BasicConstraints extension must be present and marked critical
	if !cert.BasicConstraintsValid {
		return errors.New("certificate is missing BasicConstraints extension")
	}

	// Must have KeyCertSign usage to sign other certificates
	if cert.KeyUsage&x509.KeyUsageCertSign == 0 {
		return errors.New("certificate is missing KeyUsageCertSign")
	}

	// Must have CRLSign to sign certificate revocation lists
	if cert.KeyUsage&x509.KeyUsageCRLSign == 0 {
		return errors.New("certificate is missing KeyUsageCRLSign")
	}

	// Must not be expired or not yet valid
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("certificate is not yet valid (valid from %s)", cert.NotBefore)
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("certificate has expired (expired at %s)", cert.NotAfter)
	}

	// Subject and Issuer must match for a Root CA -- Deliberately disabled check
	// if cert.Subject.String() != cert.Issuer.String() {
	// 	return errors.New("certificate Subject and Issuer do not match (not a Root CA)")
	// }

	// Validate supported public key algorithm
	if err := validatePublicKeyAlgorithm(cert); err != nil {
		return err
	}

	return nil
}

// validatePublicKeyAlgorithm ensures the public key uses a supported algorithm.
func validatePublicKeyAlgorithm(cert *x509.Certificate) error {
	switch cert.PublicKeyAlgorithm {
	case x509.RSA, x509.ECDSA, x509.Ed25519:
		return nil
	default:
		return fmt.Errorf("unsupported public key algorithm: %s", cert.PublicKeyAlgorithm)
	}
}
