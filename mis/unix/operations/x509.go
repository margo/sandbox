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
	logger := mo.logger.With("operation", "GenerateX509SVID")
	logger.Debug("generating ECDSA P-256 private key")

	// Generate ECDSA P-256 private key (SPIFFE spec recommends ECDSA)
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		logger.Error("failed to generate ECDSA private key", "error", err)
		return nil, nil, err
	}
	logger.Debug("ECDSA private key generated successfully")

	spiffeURI, err := url.Parse(req.SpiffeID)
	if err != nil {
		logger.Error("failed to parse SPIFFE ID as URI", "spiffe_id", req.SpiffeID, "error", err)
		return nil, nil, err
	}
	logger.Debug("SPIFFE ID parsed", "spiffe_id", req.SpiffeID, "trust_domain", spiffeURI.Host)

	now := time.Now()
	ttl := req.ResolvedTTL()

	logger.Debug(
		"building X.509 certificate template",
		"not_before", now,
		"not_after", now.Add(ttl),
		"ttl", ttl,
		"dns_sans", req.DNS,
	)

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
	logger.Debug("self-signing X.509 certificate")
	certDER, err := x509.CreateCertificate(
		rand.Reader,
		template,
		template,
		&privKey.PublicKey,
		privKey,
	)
	if err != nil {
		logger.Error("failed to create X.509 certificate", "error", err)
		return nil, nil, err
	}
	logger.Debug("X.509 certificate created, encoding to PEM")

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	logger.Debug("marshalling EC private key to DER")
	privKeyDER, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		logger.Error("failed to marshal EC private key", "error", err)
		return nil, nil, err
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privKeyDER})

	logger.Info(
		"X.509 SVID generated successfully",
		"spiffe_id", req.SpiffeID,
		"trust_domain", spiffeURI.Host,
		"ttl", ttl,
		"dns_sans", req.DNS,
	)

	return certPEM, keyPEM, nil
}
