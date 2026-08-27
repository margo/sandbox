package operations

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"net/url"
	"time"

	"github.com/margo/sandbox/mis/pkg/helpers"
	"github.com/margo/sandbox/mis/pkg/types"
)

// generateX509SVID creates a self-signed X.509 SVID certificate and private key
func (mo *MintOperations) GenerateX509SVID(
	req *types.MintSVIDRequest,
) (certPEM []byte, keyPEM []byte, err error) {
	// Generate ECDSA P-256 private key (SPIFFE spec recommends ECDSA)
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	spiffeURI, err := url.Parse(req.SpiffeID)
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()
	ttl := req.ResolvedTTL()

	template := &x509.Certificate{
		SerialNumber: helpers.NewSerialNumber(),
		Subject: pkix.Name{
			Organization: []string{spiffeURI.Host}, // trust domain as org
		},
		NotBefore: now,
		NotAfter:  now.Add(ttl),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		BasicConstraintsValid: true,
		URIs:                  []*url.URL{spiffeURI}, // SPIFFE ID as URI SAN
		DNSNames:              req.DNS,
	}

	// Self-sign (in production, this would be signed by a SPIRE/CA)
	certDER, err := x509.CreateCertificate(
		rand.Reader,
		template,
		template,
		&privKey.PublicKey,
		privKey,
	)
	if err != nil {
		return nil, nil, err
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	privKeyDER, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return nil, nil, err
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privKeyDER})

	return certPEM, keyPEM, nil
}
