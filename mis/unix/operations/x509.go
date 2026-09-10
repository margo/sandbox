package operations

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/margo/sandbox/mis/pkg/helpers"
	"github.com/margo/sandbox/mis/pkg/types"
)

// GenerateX509SVID creates an X.509 SVID certificate signed by the provided CA.
// caFilePath: path to the PEM-encoded CA certificate on disk.
// caKeyPath:  path to the PEM-encoded CA private key on disk.
func (mo *MintOperations) GenerateX509SVID(
	req *types.MintSVIDRequest,
	caFilePath string,
	caKeyPath string,
) (certPEM []byte, keyPEM []byte, err error) {
	logger := mo.logger.With("operation", "GenerateX509SVID")

	// --- Load CA certificate ---
	logger.Debug("reading CA certificate from disk", "path", caFilePath)
	caCertPEMBytes, err := os.ReadFile(caFilePath)
	if err != nil {
		logger.Error("failed to read CA certificate file", "path", caFilePath, "error", err)
		return nil, nil, fmt.Errorf("reading CA cert file: %w", err)
	}

	caCertBlock, _ := pem.Decode(caCertPEMBytes)
	if caCertBlock == nil {
		return nil, nil, fmt.Errorf(
			"failed to decode PEM block from CA certificate file: %s",
			caFilePath,
		)
	}

	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		logger.Error("failed to parse CA certificate", "error", err)
		return nil, nil, fmt.Errorf("parsing CA certificate: %w", err)
	}
	logger.Debug("CA certificate loaded successfully")

	// --- Load CA private key ---
	logger.Debug("reading CA private key from disk", "path", caKeyPath)
	caKeyPEMBytes, err := os.ReadFile(caKeyPath)
	if err != nil {
		logger.Error("failed to read CA key file", "path", caKeyPath, "error", err)
		return nil, nil, fmt.Errorf("reading CA key file: %w", err)
	}

	caKeyBlock, _ := pem.Decode(caKeyPEMBytes)
	if caKeyBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode PEM block from CA key file: %s", caKeyPath)
	}

	caPrivKey, err := x509.ParseECPrivateKey(caKeyBlock.Bytes)
	if err != nil {
		logger.Error("failed to parse CA EC private key", "error", err)
		return nil, nil, fmt.Errorf("parsing CA private key: %w", err)
	}
	logger.Debug("CA private key loaded successfully")

	// --- Generate SVID private key ---
	logger.Debug("generating ECDSA P-256 private key for SVID")
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		logger.Error("failed to generate ECDSA private key", "error", err)
		return nil, nil, fmt.Errorf("generating ECDSA private key: %w", err)
	}
	logger.Debug("ECDSA private key generated successfully")

	// --- Parse SPIFFE ID ---
	spiffeURI, err := url.Parse(req.SpiffeID)
	if err != nil {
		logger.Error("failed to parse SPIFFE ID as URI", "spiffe_id", req.SpiffeID, "error", err)
		return nil, nil, fmt.Errorf("parsing SPIFFE ID: %w", err)
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
			Organization: []string{spiffeURI.Host},
		},
		NotBefore: now,
		NotAfter:  now.Add(ttl),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		BasicConstraintsValid: true,
		URIs:                  []*url.URL{spiffeURI},
		DNSNames:              req.DNS,
	}

	// --- Sign with CA ---
	logger.Debug("signing X.509 certificate with provided CA")
	certDER, err := x509.CreateCertificate(
		rand.Reader,
		template, // certificate to sign
		caCert,   // CA as parent (not self-signed)
		&privKey.PublicKey,
		caPrivKey, // CA private key for signing
	)
	if err != nil {
		logger.Error("failed to create X.509 certificate", "error", err)
		return nil, nil, fmt.Errorf("creating X.509 certificate: %w", err)
	}
	logger.Debug("X.509 certificate created, encoding to PEM")

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	logger.Debug("marshalling EC private key to DER")
	privKeyDER, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		logger.Error("failed to marshal EC private key", "error", err)
		return nil, nil, fmt.Errorf("marshalling EC private key: %w", err)
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
