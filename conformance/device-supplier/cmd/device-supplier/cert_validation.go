package main

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"fmt"
	"time"
)

// certRFC5280Violation checks the subset of RFC 5280 the mock can verify on a
// device certificate presented at onboarding, and returns a human-readable
// reason string when the certificate does not conform (empty string = OK).
// Exercises MARGO-DEV-MANAGEMENTINTERFACE-004 ("All client certificates MUST
// conform to RFC 5280 standards").
//
// It is intentionally conservative — it only flags unambiguous violations so a
// real conforming device cert is never rejected:
//   - outside its validity window (RFC 5280 §4.1.2.5)
//   - empty subject with no subjectAltName (§4.1.2.6)
//   - key strength below the modern floor (RSA < 2048, EC < P-256)
func certRFC5280Violation(certPEM string) string {
	cert, err := parseClientCert(certPEM)
	if err != nil {
		// Unparseable certs are handled separately (400) before this is called.
		return ""
	}

	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Sprintf("certificate is not yet valid (notBefore %s)", cert.NotBefore.UTC().Format(time.RFC3339))
	}
	if now.After(cert.NotAfter) {
		return fmt.Sprintf("certificate has expired (notAfter %s)", cert.NotAfter.UTC().Format(time.RFC3339))
	}

	if len(cert.RawSubject) <= 2 && len(cert.DNSNames) == 0 && len(cert.IPAddresses) == 0 &&
		len(cert.EmailAddresses) == 0 && len(cert.URIs) == 0 {
		return "certificate has an empty subject and no subjectAltName"
	}

	switch k := cert.PublicKey.(type) {
	case *rsa.PublicKey:
		if k.N.BitLen() < 2048 {
			return fmt.Sprintf("RSA public key is %d bits, below the 2048-bit minimum", k.N.BitLen())
		}
	case *ecdsa.PublicKey:
		if k.Curve.Params().BitSize < 256 {
			return fmt.Sprintf("EC public key curve %s is weaker than P-256", k.Curve.Params().Name)
		}
	}

	return ""
}
