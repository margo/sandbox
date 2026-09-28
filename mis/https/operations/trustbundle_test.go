package operations

// restapi/operations/trustbundle_test.go

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/margo/sandbox/mis/pkg/conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// generateSelfSignedCACert creates a minimal self-signed CA certificate and
// writes it as a PEM file to dir. Returns the file path and the parsed cert.
func generateSelfSignedCACert(t *testing.T, dir string) (string, *x509.Certificate) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "generate ECDSA key")

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err, "create certificate")

	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err, "parse generated certificate")

	pemPath := filepath.Join(dir, "ca.pem")
	//nolint:gosec // filePath is a trusted system configuration variable; this is a unit test file
	f, err := os.Create(pemPath)
	require.NoError(t, err, "create PEM file")
	defer f.Close()

	require.NoError(t, pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}))

	return pemPath, cert
}

// newOperation constructs an Operation directly (same package) without going
// through New(), so we can set unexported fields for testing.
func newOperation(trustDomain string, caCertPath string) *Operation {
	return &Operation{
		trustDomain: trustDomain,
		ca:          conf.CAConfig{Cert: caCertPath},
		logger:      slog.Default(),
	}
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestGetTrustBundle_Success(t *testing.T) {
	dir := t.TempDir()
	certPath, caCert := generateSelfSignedCACert(t, dir)

	op := newOperation("example.org", certPath)
	bundle, err := op.GetTrustBundle()

	require.NoError(t, err)
	require.NotNil(t, bundle)

	// Trust domain must match
	assert.Equal(t, "example.org", bundle.TrustDomain().String())

	// The CA cert must be present in the bundle's X.509 authorities
	authorities := bundle.X509Authorities()
	require.Len(t, authorities, 1)
	assert.Equal(t, caCert.Raw, authorities[0].Raw)
}

func TestGetTrustBundle_InvalidTrustDomain(t *testing.T) {
	dir := t.TempDir()
	certPath, _ := generateSelfSignedCACert(t, dir)

	tests := []struct {
		name        string
		trustDomain string
	}{
		{"empty trust domain", ""},
		{"invalid characters", "exam ple.org"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			op := newOperation(tc.trustDomain, certPath)
			bundle, err := op.GetTrustBundle()

			require.Error(t, err)
			assert.Nil(t, bundle)
			assert.Contains(t, err.Error(), "invalid trust domain")
		})
	}
}

func TestGetTrustBundle_CACertFileNotFound(t *testing.T) {
	op := newOperation("example.org", "/tmp/non-existent-ca-cert.pem")
	bundle, err := op.GetTrustBundle()

	require.Error(t, err)
	assert.Nil(t, bundle)
	assert.Contains(t, err.Error(), "failed to read CA cert")
}

func TestGetTrustBundle_InvalidPEMBlock(t *testing.T) {
	dir := t.TempDir()
	badPEM := filepath.Join(dir, "bad.pem")
	require.NoError(t, os.WriteFile(badPEM, []byte("this is not valid PEM data"), 0o600))

	op := newOperation("example.org", badPEM)
	bundle, err := op.GetTrustBundle()

	require.Error(t, err)
	assert.Nil(t, bundle)
	assert.Contains(t, err.Error(), "failed to decode PEM block")
}

func TestGetTrustBundle_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	emptyFile := filepath.Join(dir, "empty.pem")
	require.NoError(t, os.WriteFile(emptyFile, []byte{}, 0o600))

	op := newOperation("example.org", emptyFile)
	bundle, err := op.GetTrustBundle()

	require.Error(t, err)
	assert.Nil(t, bundle)
	assert.Contains(t, err.Error(), "failed to decode PEM block")
}

func TestGetTrustBundle_InvalidCertificateBytes(t *testing.T) {
	dir := t.TempDir()
	// Valid PEM envelope but garbage DER content
	block := &pem.Block{Type: "CERTIFICATE", Bytes: []byte("not-valid-der")}
	corruptPEM := filepath.Join(dir, "corrupt.pem")

	//nolint:gosec // filePath is a trusted system configuration variable; this is a unit test file
	f, err := os.Create(corruptPEM)
	require.NoError(t, err)
	require.NoError(t, pem.Encode(f, block))
	f.Close()

	op := newOperation("example.org", corruptPEM)
	bundle, err := op.GetTrustBundle()

	require.Error(t, err)
	assert.Nil(t, bundle)
	assert.Contains(t, err.Error(), "failed to parse CA certificate")
}
