package validators

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Helpers ---

func generateRootCACert(t *testing.T, opts ...func(*x509.Certificate)) ([]byte, *x509.Certificate) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test Root CA"},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	for _, opt := range opts {
		opt(template)
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	return pemBytes, cert
}

func generateECDSACert(t *testing.T) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "Test ECDSA Root CA"},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
}

func generateEd25519Cert(t *testing.T) []byte {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(3),
		Subject:               pkix.Name{CommonName: "Test Ed25519 Root CA"},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, pub, priv)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
}

// --- ValidateRootCACertificate Tests ---

func TestValidateRootCACertificate_ValidRSACert(t *testing.T) {
	pemBytes, _ := generateRootCACert(t)

	cert, err := ValidateRootCACertificate(pemBytes)

	require.NoError(t, err)
	assert.NotNil(t, cert)
	assert.True(t, cert.IsCA)
}

func TestValidateRootCACertificate_ValidECDSACert(t *testing.T) {
	pemBytes := generateECDSACert(t)

	cert, err := ValidateRootCACertificate(pemBytes)

	require.NoError(t, err)
	assert.NotNil(t, cert)
	assert.Equal(t, x509.ECDSA, cert.PublicKeyAlgorithm)
}

func TestValidateRootCACertificate_ValidEd25519Cert(t *testing.T) {
	pemBytes := generateEd25519Cert(t)

	cert, err := ValidateRootCACertificate(pemBytes)

	require.NoError(t, err)
	assert.NotNil(t, cert)
	assert.Equal(t, x509.Ed25519, cert.PublicKeyAlgorithm)
}

func TestValidateRootCACertificate_EmptyData(t *testing.T) {
	cert, err := ValidateRootCACertificate([]byte{})

	assert.Nil(t, cert)
	assert.EqualError(t, err, "certificate data is empty")
}

func TestValidateRootCACertificate_InvalidData(t *testing.T) {
	cert, err := ValidateRootCACertificate([]byte("not a certificate"))

	assert.Nil(t, cert)
	assert.ErrorContains(t, err, "failed to parse certificate")
}

// --- validateRootCAConstraints Tests ---

func TestValidateRootCAConstraints_NotCA(t *testing.T) {
	pemBytes, _ := generateRootCACert(t, func(c *x509.Certificate) {
		c.IsCA = false
	})

	// Parse the cert directly to bypass CreateCertificate re-encoding
	block, _ := pem.Decode(pemBytes)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	err = validateRootCAConstraints(cert)
	assert.EqualError(t, err, "certificate is not a CA (BasicConstraints.IsCA is false)")
}

func TestValidateRootCAConstraints_MissingKeyCertSign(t *testing.T) {
	pemBytes, _ := generateRootCACert(t, func(c *x509.Certificate) {
		c.KeyUsage = x509.KeyUsageCRLSign // only CRLSign, no CertSign
	})

	block, _ := pem.Decode(pemBytes)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	err = validateRootCAConstraints(cert)
	assert.EqualError(t, err, "certificate is missing KeyUsageCertSign")
}

func TestValidateRootCAConstraints_MissingCRLSign(t *testing.T) {
	pemBytes, _ := generateRootCACert(t, func(c *x509.Certificate) {
		c.KeyUsage = x509.KeyUsageCertSign // only CertSign, no CRLSign
	})

	block, _ := pem.Decode(pemBytes)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	err = validateRootCAConstraints(cert)
	assert.EqualError(t, err, "certificate is missing KeyUsageCRLSign")
}

func TestValidateRootCAConstraints_NotYetValid(t *testing.T) {
	pemBytes, _ := generateRootCACert(t, func(c *x509.Certificate) {
		c.NotBefore = time.Now().Add(2 * time.Hour)
		c.NotAfter = time.Now().Add(48 * time.Hour)
	})

	block, _ := pem.Decode(pemBytes)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	err = validateRootCAConstraints(cert)
	assert.ErrorContains(t, err, "certificate is not yet valid")
}

func TestValidateRootCAConstraints_Expired(t *testing.T) {
	pemBytes, _ := generateRootCACert(t, func(c *x509.Certificate) {
		c.NotBefore = time.Now().Add(-48 * time.Hour)
		c.NotAfter = time.Now().Add(-1 * time.Hour)
	})

	block, _ := pem.Decode(pemBytes)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	err = validateRootCAConstraints(cert)
	assert.ErrorContains(t, err, "certificate has expired")
}

// --- validatePublicKeyAlgorithm Tests ---

func TestValidatePublicKeyAlgorithm_RSA(t *testing.T) {
	_, cert := generateRootCACert(t)
	err := validatePublicKeyAlgorithm(cert)
	assert.NoError(t, err)
}

func TestValidatePublicKeyAlgorithm_ECDSA(t *testing.T) {
	pemBytes := generateECDSACert(t)
	block, _ := pem.Decode(pemBytes)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	err = validatePublicKeyAlgorithm(cert)
	assert.NoError(t, err)
}

func TestValidatePublicKeyAlgorithm_Ed25519(t *testing.T) {
	pemBytes := generateEd25519Cert(t)
	block, _ := pem.Decode(pemBytes)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	err = validatePublicKeyAlgorithm(cert)
	assert.NoError(t, err)
}

func TestValidatePublicKeyAlgorithm_Unsupported(t *testing.T) {
	// Simulate an unsupported algorithm by directly setting the field
	cert := &x509.Certificate{
		PublicKeyAlgorithm: x509.DSA,
	}

	err := validatePublicKeyAlgorithm(cert)
	assert.ErrorContains(t, err, "unsupported public key algorithm")
}
