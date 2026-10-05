package encoders

import (
	"crypto/x509"
	"encoding/pem"
)

// encodeCertToPEM encodes an *x509.Certificate back to PEM format so it can
// be passed to helpers that accept []byte (PEM or DER).
func CertToPEM(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})
}

// CertsToPool creates an *x509.CertPool from a slice of *x509.Certificate.
// It is useful for building trust stores to pass to tls.Config.RootCAs,
// tls.Config.ClientCAs, or x509.VerifyOptions.Roots.
func CertsToPool(roots []*x509.Certificate) *x509.CertPool {
	result := x509.NewCertPool()
	for _, ca := range roots {
		result.AddCert(ca)
	}
	return result
}
