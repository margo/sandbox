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

// generateIntermediateCACert creates a PEM-encoded intermediate CA cert signed by a root.
// The intermediate has a different Subject/Issuer by design.
func generateIntermediateCACert(
	t *testing.T,
	rootCert *x509.Certificate,
	rootKey *rsa.PrivateKey,
	opts ...func(*x509.Certificate),
) []byte {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(10),
		Subject:               pkix.Name{CommonName: "Test Intermediate CA"},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	for _, opt := range opts {
		opt(template)
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, rootCert, &key.PublicKey, rootKey)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
}

// newRootKeyAndCert returns a parsed root CA cert and its private key for signing intermediates.
func newRootKeyAndCert(t *testing.T) (*rsa.PrivateKey, *x509.Certificate) {
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

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	return key, cert
}

// concatPEM joins multiple PEM byte slices into one.
func concatPEM(blocks ...[]byte) []byte {
	var result []byte
	for _, b := range blocks {
		result = append(result, b...)
	}
	return result
}

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

// --- ValidateIntermediateCACertificates Tests ---

func TestValidateIntermediateCACertificates_EmptyData(t *testing.T) {
	err := ValidateIntermediateCACertificates([]byte{})

	assert.EqualError(t, err, "intermediate chain data is empty")
}

func TestValidateIntermediateCACertificates_ExceedsMaxBytes(t *testing.T) {
	oversized := make([]byte, 64*1024+1)

	err := ValidateIntermediateCACertificates(oversized)

	assert.ErrorContains(t, err, "exceeds the maximum allowed")
}

func TestValidateIntermediateCACertificates_NoPEMBlocks(t *testing.T) {
	err := ValidateIntermediateCACertificates([]byte("not a pem block"))

	assert.EqualError(t, err, "intermediate chain contains no valid PEM blocks")
}

func TestValidateIntermediateCACertificates_ValidSingleIntermediate(t *testing.T) {
	rootKey, rootCert := newRootKeyAndCert(t)
	intermPEM := generateIntermediateCACert(t, rootCert, rootKey)

	err := ValidateIntermediateCACertificates(intermPEM)

	assert.NoError(t, err)
}

func TestValidateIntermediateCACertificates_ValidMaxIntermediates(t *testing.T) {
	rootKey, rootCert := newRootKeyAndCert(t)

	var chain [][]byte
	for i := 0; i < 4; i++ {
		chain = append(chain, generateIntermediateCACert(t, rootCert, rootKey))
	}

	err := ValidateIntermediateCACertificates(concatPEM(chain...))

	assert.NoError(t, err)
}

func TestValidateIntermediateCACertificates_TooManyIntermediates(t *testing.T) {
	rootKey, rootCert := newRootKeyAndCert(t)

	var chain [][]byte
	for i := 0; i < 5; i++ {
		chain = append(chain, generateIntermediateCACert(t, rootCert, rootKey))
	}

	err := ValidateIntermediateCACertificates(concatPEM(chain...))

	assert.ErrorContains(t, err, "exceeding the maximum of 4")
}

func TestValidateIntermediateCACertificates_ContainsRootCA(t *testing.T) {
	// A self-signed cert (root CA) must be rejected in the intermediate chain.
	rootPEM, _ := generateRootCACert(t)

	err := ValidateIntermediateCACertificates(rootPEM)

	assert.ErrorContains(t, err, "Subject and Issuer are identical")
}

func TestValidateIntermediateCACertificates_ExpiredIntermediate(t *testing.T) {
	rootKey, rootCert := newRootKeyAndCert(t)
	intermPEM := generateIntermediateCACert(t, rootCert, rootKey, func(c *x509.Certificate) {
		c.NotBefore = time.Now().Add(-48 * time.Hour)
		c.NotAfter = time.Now().Add(-1 * time.Hour)
	})

	err := ValidateIntermediateCACertificates(intermPEM)

	assert.ErrorContains(t, err, "expired")
}

func TestValidateIntermediateCACertificates_NotYetValidIntermediate(t *testing.T) {
	rootKey, rootCert := newRootKeyAndCert(t)
	intermPEM := generateIntermediateCACert(t, rootCert, rootKey, func(c *x509.Certificate) {
		c.NotBefore = time.Now().Add(2 * time.Hour)
		c.NotAfter = time.Now().Add(48 * time.Hour)
	})

	err := ValidateIntermediateCACertificates(intermPEM)

	assert.ErrorContains(t, err, "not yet valid")
}

func TestValidateIntermediateCACertificates_MissingKeyCertSign(t *testing.T) {
	rootKey, rootCert := newRootKeyAndCert(t)
	intermPEM := generateIntermediateCACert(t, rootCert, rootKey, func(c *x509.Certificate) {
		c.KeyUsage = x509.KeyUsageCRLSign
	})

	err := ValidateIntermediateCACertificates(intermPEM)

	assert.ErrorContains(t, err, "missing KeyUsageCertSign")
}

func TestValidateIntermediateCACertificates_MissingCRLSign(t *testing.T) {
	rootKey, rootCert := newRootKeyAndCert(t)
	intermPEM := generateIntermediateCACert(t, rootCert, rootKey, func(c *x509.Certificate) {
		c.KeyUsage = x509.KeyUsageCertSign
	})

	err := ValidateIntermediateCACertificates(intermPEM)

	assert.ErrorContains(t, err, "missing KeyUsageCRLSign")
}

func TestValidateIntermediateCACertificates_NotCA(t *testing.T) {
	rootKey, rootCert := newRootKeyAndCert(t)
	intermPEM := generateIntermediateCACert(t, rootCert, rootKey, func(c *x509.Certificate) {
		c.IsCA = false
	})

	err := ValidateIntermediateCACertificates(intermPEM)

	assert.ErrorContains(t, err, "not a CA")
}

func TestValidateIntermediateCACertificates_MissingBasicConstraints(t *testing.T) {
	rootKey, rootCert := newRootKeyAndCert(t)
	intermPEM := generateIntermediateCACert(t, rootCert, rootKey, func(c *x509.Certificate) {
		c.BasicConstraintsValid = false
	})

	err := ValidateIntermediateCACertificates(intermPEM)

	assert.ErrorContains(t, err, "missing BasicConstraints extension")
}

// --- validateIntermediateCAConstraints Tests ---

func TestValidateIntermediateCAConstraints_Valid(t *testing.T) {
	rootKey, rootCert := newRootKeyAndCert(t)
	intermPEM := generateIntermediateCACert(t, rootCert, rootKey)

	block, _ := pem.Decode(intermPEM)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	err = validateIntermediateCAConstraints(cert, 1)
	assert.NoError(t, err)
}

func TestValidateIntermediateCAConstraints_SelfSigned(t *testing.T) {
	// A self-signed cert has matching Subject and Issuer — must be rejected.
	pemBytes, _ := generateRootCACert(t)
	block, _ := pem.Decode(pemBytes)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	err = validateIntermediateCAConstraints(cert, 1)
	assert.ErrorContains(t, err, "Subject and Issuer are identical")
}

func TestValidateIntermediateCAConstraints_ErrorMessageIncludesPosition(t *testing.T) {
	rootKey, rootCert := newRootKeyAndCert(t)
	intermPEM := generateIntermediateCACert(t, rootCert, rootKey, func(c *x509.Certificate) {
		c.KeyUsage = x509.KeyUsageCRLSign
	})

	block, _ := pem.Decode(intermPEM)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	err = validateIntermediateCAConstraints(cert, 3)
	assert.ErrorContains(t, err, "intermediate certificate 3")
}

func TestValidateIntermediateCAConstraints_UnsupportedPublicKeyAlgorithm(t *testing.T) {
	cert := &x509.Certificate{
		Subject:               pkix.Name{CommonName: "Intermediate"},
		Issuer:                pkix.Name{CommonName: "Root"},
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		PublicKeyAlgorithm:    x509.DSA,
	}

	err := validateIntermediateCAConstraints(cert, 1)
	assert.ErrorContains(t, err, "unsupported public key algorithm")
}
