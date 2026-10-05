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

	cert, err := CertificateFromBytes(km.certPEM, km.keyPEM)

	require.NoError(t, err)
	assert.NotNil(t, cert.PrivateKey, "private key must be populated")
	assert.Len(t, cert.Certificate, 1, "leaf DER block must be present")
}

func TestCertificateFromBytes_DERCertAndDERKey_ReturnsCertificate(t *testing.T) {
	km := newKeyMaterial(t)

	cert, err := CertificateFromBytes(km.certDER, km.keyDER)

	require.NoError(t, err)
	assert.NotNil(t, cert.PrivateKey)
	assert.Len(t, cert.Certificate, 1)
}

func TestCertificateFromBytes_PEMCertAndDERKey_ReturnsCertificate(t *testing.T) {
	km := newKeyMaterial(t)

	cert, err := CertificateFromBytes(km.certPEM, km.keyDER)

	require.NoError(t, err)
	assert.NotNil(t, cert.PrivateKey)
}

func TestCertificateFromBytes_DERCertAndPEMKey_ReturnsCertificate(t *testing.T) {
	km := newKeyMaterial(t)

	cert, err := CertificateFromBytes(km.certDER, km.keyPEM)

	require.NoError(t, err)
	assert.NotNil(t, cert.PrivateKey)
}

// TestCertificateFromBytes_PEMChain_PreservesIntermediates verifies that a
// PEM input containing multiple CERTIFICATE blocks (leaf + intermediate) is
// passed through intact so tls.X509KeyPair can build the full chain.
func TestCertificateFromBytes_PEMChain_PreservesIntermediates(t *testing.T) {
	leaf := newKeyMaterial(t)
	intermediate := newKeyMaterial(t) // acts as a fake intermediate

	// Concatenate leaf + intermediate PEM blocks into a single chain PEM.
	chainPEM := append(leaf.certPEM, intermediate.certPEM...)

	cert, err := CertificateFromBytes(chainPEM, leaf.keyPEM)

	require.NoError(t, err)
	// tls.X509KeyPair stores all DER blocks from the PEM chain.
	assert.Len(t, cert.Certificate, 2, "chain must contain leaf + intermediate")
}

// TestCertificateFromBytes_ReturnedCertMatchesInput verifies that the leaf DER
// inside the returned tls.Certificate matches the original certificate bytes.
func TestCertificateFromBytes_ReturnedCertMatchesInput(t *testing.T) {
	km := newKeyMaterial(t)

	cert, err := CertificateFromBytes(km.certPEM, km.keyPEM)

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

	cert, err := CertificateFromBytes(certDER, sec1DER)

	require.NoError(t, err)
	assert.NotNil(t, cert.PrivateKey)
}

// ---------------------------------------------------------------------------
// CertificateFromBytes — error-path tests
// ---------------------------------------------------------------------------

func TestCertificateFromBytes_EmptyCertBytes_ReturnsError(t *testing.T) {
	km := newKeyMaterial(t)

	cert, err := CertificateFromBytes([]byte{}, km.keyPEM)

	assert.Equal(t, tls.Certificate{}, cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate")
}

func TestCertificateFromBytes_NilCertBytes_ReturnsError(t *testing.T) {
	km := newKeyMaterial(t)

	cert, err := CertificateFromBytes(nil, km.keyPEM)

	assert.Equal(t, tls.Certificate{}, cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate")
}

func TestCertificateFromBytes_EmptyKeyBytes_ReturnsError(t *testing.T) {
	km := newKeyMaterial(t)

	cert, err := CertificateFromBytes(km.certPEM, []byte{})

	assert.Equal(t, tls.Certificate{}, cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private key")
}

func TestCertificateFromBytes_NilKeyBytes_ReturnsError(t *testing.T) {
	km := newKeyMaterial(t)

	cert, err := CertificateFromBytes(km.certPEM, nil)

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

	cert, err := CertificateFromBytes(badCertPEM, km.keyPEM)

	assert.Equal(t, tls.Certificate{}, cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate")
}

func TestCertificateFromBytes_InvalidKeyPEM_ReturnsError(t *testing.T) {
	km := newKeyMaterial(t)
	badKeyPEM := []byte(
		"-----BEGIN PRIVATE KEY-----\nnot-valid-base64!!!\n-----END PRIVATE KEY-----\n",
	)

	cert, err := CertificateFromBytes(km.certPEM, badKeyPEM)

	assert.Equal(t, tls.Certificate{}, cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private key")
}

func TestCertificateFromBytes_InvalidCertDER_ReturnsError(t *testing.T) {
	km := newKeyMaterial(t)
	// Raw bytes that are not PEM and not valid DER.
	badDER := []byte{0x00, 0x01, 0x02, 0x03}

	cert, err := CertificateFromBytes(badDER, km.keyPEM)

	assert.Equal(t, tls.Certificate{}, cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate")
}

func TestCertificateFromBytes_InvalidKeyDER_ReturnsError(t *testing.T) {
	km := newKeyMaterial(t)
	badDER := []byte{0x00, 0x01, 0x02, 0x03}

	cert, err := CertificateFromBytes(km.certPEM, badDER)

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
	cert, err := CertificateFromBytes(km1.certPEM, km2.keyPEM)

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

	cert, err := CertificateFromBytes(garbage, garbage)

	assert.Equal(t, tls.Certificate{}, cert)
	require.Error(t, err)
}
