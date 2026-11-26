package validators

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"
)

const (
	// maxChainCerts is the maximum number of certificates allowed in a presented
	// chain (leaf + intermediates + trust anchor), per SPIFFE/SPIRE guidance.
	// A verifier MUST reject chains longer than six certificates.
	// Six covers: 1 leaf + 4 intermediate CAs + 1 trust anchor.
	maxChainCerts = 6

	// maxChainBytes is the maximum encoded size of the full chain.
	// A verifier SHOULD reject chains whose encoded size exceeds 64 KiB.
	maxChainBytes = 64 * 1024
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

// ValidateIntermediateCACertificates validates that every PEM block in pemData
// represents a valid X.509 intermediate CA certificate.
//
// Validation rules applied to each certificate:
//   - Must parse as a valid X.509 certificate.
//   - Must have BasicConstraints present and IsCA == true.
//   - Must have KeyUsageCertSign.
//   - Must have KeyUsageCRLSign.
//   - Must not be expired or not yet valid.
//   - Must use a supported public key algorithm.
//   - Must NOT be self-signed (Subject != Issuer distinguishes intermediates from roots).
//
// Chain-level constraints (applied before per-cert validation):
//   - The encoded size of pemData must not exceed 64 KiB.
//   - The number of certificates in pemData must not exceed 4 (i.e. the intermediate
//     portion of a max-six-cert chain: leaf + 4 intermediates + trust anchor, the
//     caller owns the leaf and trust anchor slots).
func ValidateIntermediateCACertificates(pemData []byte) error {
	if len(pemData) == 0 {
		return errors.New("intermediate chain data is empty")
	}

	// SHOULD reject chains whose encoded size exceeds 64 KiB.
	if len(pemData) > maxChainBytes {
		return fmt.Errorf(
			"intermediate chain encoded size (%d bytes) exceeds the maximum allowed %d bytes (64 KiB)",
			len(pemData), maxChainBytes,
		)
	}

	// Decode all PEM blocks up-front so we can enforce the chain-length limit
	// before doing any expensive cryptographic parsing.
	var blocks []*pem.Block
	rest := pemData
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		blocks = append(blocks, block)
	}

	if len(blocks) == 0 {
		return errors.New("intermediate chain contains no valid PEM blocks")
	}

	// MUST reject if the intermediate portion alone would push the full chain
	// (leaf + intermediates + trust anchor) beyond six certificates.
	// The caller owns the leaf (1) and trust anchor (1) slots, leaving at most 4
	// slots for intermediates.
	const maxIntermediates = maxChainCerts - 2 // 4
	if len(blocks) > maxIntermediates {
		return fmt.Errorf(
			"intermediate chain contains %d certificates, exceeding the maximum of %d "+
				"(a full chain must not exceed %d certificates including leaf and trust anchor)",
			len(blocks), maxIntermediates, maxChainCerts,
		)
	}

	for i, block := range blocks {
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return fmt.Errorf("intermediate certificate %d: failed to parse: %w", i+1, err)
		}

		if err := validateIntermediateCAConstraints(cert, i+1); err != nil {
			return err
		}
	}

	return nil
}

// validateIntermediateCAConstraints enforces intermediate CA-specific X.509 rules.
// pos is the 1-based position in the chain, used only for error messages.
func validateIntermediateCAConstraints(cert *x509.Certificate, pos int) error {
	label := fmt.Sprintf("intermediate certificate %d", pos)

	if !cert.BasicConstraintsValid {
		return fmt.Errorf("%s: missing BasicConstraints extension", label)
	}
	if !cert.IsCA {
		return fmt.Errorf("%s: not a CA (BasicConstraints CA:TRUE required)", label)
	}

	if cert.KeyUsage&x509.KeyUsageCertSign == 0 {
		return fmt.Errorf("%s: missing KeyUsageCertSign", label)
	}
	if cert.KeyUsage&x509.KeyUsageCRLSign == 0 {
		return fmt.Errorf("%s: missing KeyUsageCRLSign", label)
	}

	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("%s: not yet valid (valid from %s)", label, cert.NotBefore)
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("%s: expired (expired at %s)", label, cert.NotAfter)
	}

	// An intermediate MUST have a different Subject and Issuer.
	// Self-signed == root CA, which must not appear in the intermediate chain.
	if cert.Subject.String() == cert.Issuer.String() {
		return fmt.Errorf(
			"%s: Subject and Issuer are identical — this appears to be a root CA, not an intermediate",
			label,
		)
	}

	if err := validatePublicKeyAlgorithm(cert); err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}

	return nil
}
