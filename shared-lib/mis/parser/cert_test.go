package parser

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test fixture builder
// ---------------------------------------------------------------------------

// keyMaterial holds a generated cert/key pair in multiple encodings so each
// test can pick the format it needs without regenerating crypto material.
type keyMaterial struct {
	certPEM []byte // PEM-encoded leaf certificate
	keyPEM  []byte // PEM-encoded PKCS#8 private key
	certDER []byte // raw DER certificate bytes
	keyDER  []byte // raw DER PKCS#8 private key bytes
}

// newKeyMaterial generates a fresh ECDSA P-256 key pair and a self-signed
// X.509 certificate with a SPIFFE URI SAN.  The certificate is intentionally
// minimal — it is not a valid SPIFFE SVID but is sufficient for TLS pairing.
func newKeyMaterial(t *testing.T) keyMaterial {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "generate ECDSA key")

	spiffeURI, err := url.Parse("spiffe://margo.org/margo/wfm/symphony-1")
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{Organization: []string{"margo.org"}},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		URIs:         []*url.URL{spiffeURI},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	require.NoError(t, err, "create certificate")

	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err, "marshal PKCS#8 key")

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	return keyMaterial{
		certPEM: certPEM,
		keyPEM:  keyPEM,
		certDER: certDER,
		keyDER:  keyDER,
	}
}

// newMismatchedKeyMaterial returns a second, independent key pair whose
// certificate does NOT correspond to the first pair's private key.
func newMismatchedKeyMaterial(t *testing.T) keyMaterial {
	t.Helper()
	return newKeyMaterial(t) // independent generation → different key pair
}

// ---------------------------------------------------------------------------
// CertificateFromBytes — happy-path tests
// ---------------------------------------------------------------------------

func TestCertificateFromBytes_PEMCertAndPEMKey_ReturnsCertificate(t *testing.T) {
	km := newKeyMaterial(t)

	cert, err := CertificateFromBytes(km.certPEM, km.keyPEM, nil)

	require.NoError(t, err)
	assert.NotNil(t, cert.PrivateKey, "private key must be populated")
	assert.Len(t, cert.Certificate, 1, "leaf DER block must be present")
}

func TestCertificateFromBytes_DERCertAndDERKey_ReturnsCertificate(t *testing.T) {
	km := newKeyMaterial(t)

	cert, err := CertificateFromBytes(km.certDER, km.keyDER, nil)

	require.NoError(t, err)
	assert.NotNil(t, cert.PrivateKey)
	assert.Len(t, cert.Certificate, 1)
}

func TestCertificateFromBytes_PEMCertAndDERKey_ReturnsCertificate(t *testing.T) {
	km := newKeyMaterial(t)

	cert, err := CertificateFromBytes(km.certPEM, km.keyDER, nil)

	require.NoError(t, err)
	assert.NotNil(t, cert.PrivateKey)
}

func TestCertificateFromBytes_DERCertAndPEMKey_ReturnsCertificate(t *testing.T) {
	km := newKeyMaterial(t)

	cert, err := CertificateFromBytes(km.certDER, km.keyPEM, nil)

	require.NoError(t, err)
	assert.NotNil(t, cert.PrivateKey)
}

// TestCertificateFromBytes_PEMChain_PreservesIntermediates verifies that PEM
// intermediate certificates are appended after the leaf certificate.
func TestCertificateFromBytes_PEMChain_PreservesIntermediates(t *testing.T) {
	leaf := newKeyMaterial(t)
	intermediate := newKeyMaterial(t)

	cert, err := CertificateFromBytes(
		leaf.certPEM,
		leaf.keyPEM,
		intermediate.certPEM,
	)

	require.NoError(t, err)
	assert.Len(t, cert.Certificate, 2,
		"chain must contain the leaf followed by the intermediate")
}

func TestCertificateFromBytes_DERChain_PreservesIntermediate(t *testing.T) {
	leaf := newKeyMaterial(t)
	intermediate := newKeyMaterial(t)

	cert, err := CertificateFromBytes(
		leaf.certDER,
		leaf.keyDER,
		intermediate.certDER,
	)

	require.NoError(t, err)
	assert.Len(t, cert.Certificate, 2,
		"DER chain must contain the leaf followed by the intermediate")
	assert.Equal(t, leaf.certDER, cert.Certificate[0])
	assert.Equal(t, intermediate.certDER, cert.Certificate[1])
}

func TestCertificateFromBytes_InvalidChainPEM_ReturnsError(t *testing.T) {
	leaf := newKeyMaterial(t)
	badChain := []byte(
		"-----BEGIN CERTIFICATE-----\nnot-valid-base64!!!\n-----END CERTIFICATE-----\n",
	)

	cert, err := CertificateFromBytes(
		leaf.certPEM,
		leaf.keyPEM,
		badChain,
	)

	assert.Equal(t, tls.Certificate{}, cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "intermediate chain")
}

func TestCertificateFromBytes_InvalidChainDER_ReturnsError(t *testing.T) {
	leaf := newKeyMaterial(t)
	badDER := []byte{0x00, 0x01, 0x02, 0x03}

	cert, err := CertificateFromBytes(
		leaf.certDER,
		leaf.keyDER,
		badDER,
	)

	assert.Equal(t, tls.Certificate{}, cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "intermediate chain")
}

func TestNormaliseToPEM_PEMCertificate_ReturnsInput(t *testing.T) {
	km := newKeyMaterial(t)

	result, err := normaliseToPEM(km.certPEM, "CERTIFICATE")

	require.NoError(t, err)
	assert.Equal(t, km.certPEM, result)
}

func TestNormaliseToPEM_DERCertificate_ReturnsPEM(t *testing.T) {
	km := newKeyMaterial(t)

	result, err := normaliseToPEM(km.certDER, "CERTIFICATE")

	require.NoError(t, err)

	block, rest := pem.Decode(result)
	require.NotNil(t, block)
	assert.Empty(t, rest)
	assert.Equal(t, "CERTIFICATE", block.Type)
	assert.Equal(t, km.certDER, block.Bytes)
}

func TestNormaliseToPEM_EmptyCertificate_ReturnsError(t *testing.T) {
	result, err := normaliseToPEM(nil, "CERTIFICATE")

	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestNormaliseKeyToPEM_DERFormats_ReturnsPEM(t *testing.T) {
	tests := []struct {
		name     string
		keyBytes []byte
		pemType  string
	}{
		{name: "PKCS8", pemType: "PRIVATE KEY"},
		{name: "SEC1", pemType: "EC PRIVATE KEY"},
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tests[0].keyBytes, err = x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)

	tests[1].keyBytes, err = x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := normaliseKeyToPEM(tt.keyBytes)

			require.NoError(t, err)
			block, rest := pem.Decode(result)
			require.NotNil(t, block)
			assert.Empty(t, rest)
			assert.Equal(t, tt.pemType, block.Type)
			assert.Equal(t, tt.keyBytes, block.Bytes)
		})
	}
}

func TestNormaliseKeyToPEM_InvalidDER_ReturnsError(t *testing.T) {
	result, err := normaliseKeyToPEM([]byte{0x00, 0x01, 0x02, 0x03})

	assert.Nil(t, result)
	require.Error(t, err)
}

// TestCertificateFromBytes_ReturnedCertMatchesInput verifies that the leaf DER
// inside the returned tls.Certificate matches the original certificate bytes.
func TestCertificateFromBytes_ReturnedCertMatchesInput(t *testing.T) {
	km := newKeyMaterial(t)

	cert, err := CertificateFromBytes(km.certPEM, km.keyPEM, nil)

	require.NoError(t, err)
	assert.Equal(t, km.certDER, cert.Certificate[0],
		"leaf DER in tls.Certificate must match the original certificate")
}

// TestCertificateFromBytes_ECKeyVariants checks SEC1 (EC PRIVATE KEY) DER format.
func TestCertificateFromBytes_ECKeyDERSEC1Format_ReturnsCertificate(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	// Marshal as SEC1 (EC PRIVATE KEY) instead of PKCS#8.
	sec1DER, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{Organization: []string{"margo.org"}},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	require.NoError(t, err)

	cert, err := CertificateFromBytes(certDER, sec1DER, nil)

	require.NoError(t, err)
	assert.NotNil(t, cert.PrivateKey)
}

// ---------------------------------------------------------------------------
// CertificateFromBytes — error-path tests
// ---------------------------------------------------------------------------

func TestCertificateFromBytes_EmptyCertBytes_ReturnsError(t *testing.T) {
	km := newKeyMaterial(t)

	cert, err := CertificateFromBytes([]byte{}, km.keyPEM, nil)

	assert.Equal(t, tls.Certificate{}, cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate")
}

func TestCertificateFromBytes_NilCertBytes_ReturnsError(t *testing.T) {
	km := newKeyMaterial(t)

	cert, err := CertificateFromBytes(nil, km.keyPEM, nil)

	assert.Equal(t, tls.Certificate{}, cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate")
}

func TestCertificateFromBytes_EmptyKeyBytes_ReturnsError(t *testing.T) {
	km := newKeyMaterial(t)

	cert, err := CertificateFromBytes(km.certPEM, []byte{}, nil)

	assert.Equal(t, tls.Certificate{}, cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private key")
}

func TestCertificateFromBytes_NilKeyBytes_ReturnsError(t *testing.T) {
	km := newKeyMaterial(t)

	cert, err := CertificateFromBytes(km.certPEM, nil, nil)

	assert.Equal(t, tls.Certificate{}, cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private key")
}

func TestCertificateFromBytes_InvalidCertPEM_ReturnsError(t *testing.T) {
	km := newKeyMaterial(t)
	// Looks like PEM (has the header) but the base64 body is garbage.
	badCertPEM := []byte(
		"-----BEGIN CERTIFICATE-----\nnot-valid-base64!!!\n-----END CERTIFICATE-----\n",
	)

	cert, err := CertificateFromBytes(badCertPEM, km.keyPEM, nil)

	assert.Equal(t, tls.Certificate{}, cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate")
}

func TestCertificateFromBytes_InvalidKeyPEM_ReturnsError(t *testing.T) {
	km := newKeyMaterial(t)
	badKeyPEM := []byte(
		"-----BEGIN PRIVATE KEY-----\nnot-valid-base64!!!\n-----END PRIVATE KEY-----\n",
	)

	cert, err := CertificateFromBytes(km.certPEM, badKeyPEM, nil)

	assert.Equal(t, tls.Certificate{}, cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private key")
}

func TestCertificateFromBytes_InvalidCertDER_ReturnsError(t *testing.T) {
	km := newKeyMaterial(t)
	// Raw bytes that are not PEM and not valid DER.
	badDER := []byte{0x00, 0x01, 0x02, 0x03}

	cert, err := CertificateFromBytes(badDER, km.keyPEM, nil)

	assert.Equal(t, tls.Certificate{}, cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate")
}

func TestCertificateFromBytes_InvalidKeyDER_ReturnsError(t *testing.T) {
	km := newKeyMaterial(t)
	badDER := []byte{0x00, 0x01, 0x02, 0x03}

	cert, err := CertificateFromBytes(km.certPEM, badDER, nil)

	assert.Equal(t, tls.Certificate{}, cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private key")
}

// TestCertificateFromBytes_MismatchedCertAndKey_ReturnsError verifies that a
// certificate and a key that do not form a valid pair are rejected.
func TestCertificateFromBytes_MismatchedCertAndKey_ReturnsError(t *testing.T) {
	km1 := newKeyMaterial(t)
	km2 := newMismatchedKeyMaterial(t)

	// km1 cert paired with km2 key — must fail.
	cert, err := CertificateFromBytes(km1.certPEM, km2.keyPEM, nil)

	assert.Equal(t, tls.Certificate{}, cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create TLS certificate")
}

// TestCertificateFromBytes_RandomGarbageBytes_ReturnsError ensures that
// arbitrary random bytes (neither PEM nor valid DER) are rejected cleanly.
func TestCertificateFromBytes_RandomGarbageBytes_ReturnsError(t *testing.T) {
	garbage := make([]byte, 128)
	_, err := rand.Read(garbage)
	require.NoError(t, err)

	cert, err := CertificateFromBytes(garbage, garbage, nil)

	assert.Equal(t, tls.Certificate{}, cert)
	require.Error(t, err)
}
