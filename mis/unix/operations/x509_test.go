package operations

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log/slog"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/margo/sandbox/mis/pkg/types"
)

// testCA holds a self-signed CA cert and key written to temp files.
type testCA struct {
	certPath string
	keyPath  string
	cert     *x509.Certificate
}

// newTestCA generates a self-signed ECDSA CA and writes PEM files to a temp dir.
func newTestCA(t *testing.T) *testCA {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &caKey.PublicKey, caKey)
	require.NoError(t, err)

	caCert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	dir := t.TempDir()

	certPath := filepath.Join(dir, "ca.crt")
	keyPath := filepath.Join(dir, "ca.key")

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	require.NoError(t, os.WriteFile(certPath, certPEM, 0o600))

	keyDER, err := x509.MarshalECPrivateKey(caKey)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0o600))

	return &testCA{certPath: certPath, keyPath: keyPath, cert: caCert}
}

func newMintOps() *MintOperations {
	return New(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
}

// newTestCSR generates a CSR with the given SPIFFE ID as a URI SAN.
// Pass spiffeID as empty string to generate a CSR with no URI SANs.
func newTestCSR(t *testing.T, spiffeID string) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	var uris []*url.URL
	if spiffeID != "" {
		u, err := url.Parse(spiffeID)
		require.NoError(t, err)
		uris = []*url.URL{u}
	}

	template := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "test"},
		URIs:    uris,
	}

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	require.NoError(t, err)

	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
	return string(csrPEM)
}

// ---------------------------------------------------------------------------
// GenerateSVID — installed mode (no CSR)
// ---------------------------------------------------------------------------

func TestGenerateSVID_InstalledMode_HappyPath(t *testing.T) {
	ca := newTestCA(t)
	mo := newMintOps()

	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/service/api",
		DNS:      []string{"api.example.org", "api-internal.example.org"},
	}

	result, err := mo.GenerateSVID(req, ca.certPath, ca.keyPath)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, SVIDGenerationTypeInstalled, result.GenerationType)
	require.NotEmpty(t, result.Cert)
	require.NotEmpty(t, result.Key)

	// Cert must be a valid PEM-encoded X.509 certificate.
	certBlock, _ := pem.Decode(result.Cert)
	require.NotNil(t, certBlock)
	assert.Equal(t, "CERTIFICATE", certBlock.Type)

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	require.NoError(t, err)

	// Key must be a valid PEM-encoded EC private key.
	keyBlock, _ := pem.Decode(result.Key)
	require.NotNil(t, keyBlock)
	assert.Equal(t, "EC PRIVATE KEY", keyBlock.Type)

	_, err = x509.ParseECPrivateKey(keyBlock.Bytes)
	require.NoError(t, err)

	// SPIFFE URI SAN
	require.Len(t, cert.URIs, 1)
	assert.Equal(t, "spiffe://example.org/service/api", cert.URIs[0].String())

	// DNS SANs
	assert.ElementsMatch(t, []string{"api.example.org", "api-internal.example.org"}, cert.DNSNames)

	// Subject Organization == trust domain
	assert.Equal(t, []string{"example.org"}, cert.Subject.Organization)

	// Key usages
	assert.True(t, cert.KeyUsage&x509.KeyUsageDigitalSignature != 0)
	assert.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageServerAuth)
	assert.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageClientAuth)

	// Signed by test CA
	pool := x509.NewCertPool()
	pool.AddCert(ca.cert)
	_, err = cert.Verify(x509.VerifyOptions{Roots: pool})
	require.NoError(t, err, "certificate must verify against the test CA")
}

func TestGenerateSVID_InstalledMode_NoDNSSANs(t *testing.T) {
	ca := newTestCA(t)
	mo := newMintOps()

	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/service/worker",
		DNS:      nil,
	}

	result, err := mo.GenerateSVID(req, ca.certPath, ca.keyPath)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, SVIDGenerationTypeInstalled, result.GenerationType)

	block, _ := pem.Decode(result.Cert)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	assert.Empty(t, cert.DNSNames)
}

func TestGenerateSVID_InstalledMode_CertValidity(t *testing.T) {
	ca := newTestCA(t)
	mo := newMintOps()

	before := time.Now().Add(-time.Second)

	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/svc",
	}

	result, err := mo.GenerateSVID(req, ca.certPath, ca.keyPath)
	require.NoError(t, err)

	after := time.Now().Add(time.Second)

	block, _ := pem.Decode(result.Cert)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	assert.True(t, cert.NotBefore.After(before) || cert.NotBefore.Equal(before),
		"NotBefore should be >= test start")
	assert.True(t, cert.NotAfter.After(after),
		"NotAfter should be in the future")
	assert.True(t, cert.NotAfter.After(cert.NotBefore),
		"NotAfter must be after NotBefore")
}

func TestGenerateSVID_InstalledMode_KeyNilInEnrolledResult(t *testing.T) {
	// Sanity check: installed mode must always populate Key.
	ca := newTestCA(t)
	mo := newMintOps()

	req := &types.MintSVIDRequest{SpiffeID: "spiffe://example.org/svc"}

	result, err := mo.GenerateSVID(req, ca.certPath, ca.keyPath)

	require.NoError(t, err)
	assert.Equal(t, SVIDGenerationTypeInstalled, result.GenerationType)
	assert.NotNil(t, result.Key, "installed mode must return a private key")
}

// ---------------------------------------------------------------------------
// GenerateSVID — enrolled mode (CSR provided)
// ---------------------------------------------------------------------------

func TestGenerateSVID_EnrolledMode_HappyPath(t *testing.T) {
	ca := newTestCA(t)
	mo := newMintOps()

	csrPEM := newTestCSR(t, "spiffe://example.org/service/api")

	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/service/api",
		CSR:      csrPEM,
		DNS:      []string{"api.example.org", "api-internal.example.org"},
	}

	result, err := mo.GenerateSVID(req, ca.certPath, ca.keyPath)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, SVIDGenerationTypeEnrolled, result.GenerationType)
	require.NotEmpty(t, result.Cert)
	assert.Nil(t, result.Key, "enrolled mode must NOT return a private key")

	block, _ := pem.Decode(result.Cert)
	require.NotNil(t, block)
	assert.Equal(t, "CERTIFICATE", block.Type)

	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	// SPIFFE URI SAN sourced from req.SpiffeID
	require.Len(t, cert.URIs, 1)
	assert.Equal(t, "spiffe://example.org/service/api", cert.URIs[0].String())

	// DNS SANs
	assert.ElementsMatch(t, []string{"api.example.org", "api-internal.example.org"}, cert.DNSNames)

	// Subject Organization == trust domain
	assert.Equal(t, []string{"example.org"}, cert.Subject.Organization)

	// Key usages
	assert.True(t, cert.KeyUsage&x509.KeyUsageDigitalSignature != 0)
	assert.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageServerAuth)
	assert.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageClientAuth)

	// Signed by test CA
	pool := x509.NewCertPool()
	pool.AddCert(ca.cert)
	_, err = cert.Verify(x509.VerifyOptions{Roots: pool})
	require.NoError(t, err, "certificate must verify against the test CA")
}

func TestGenerateSVID_EnrolledMode_SpiffeIDFromReqNotCSR(t *testing.T) {
	// When req.SpiffeID differs from the CSR URI SAN, req.SpiffeID is authoritative.
	ca := newTestCA(t)
	mo := newMintOps()

	csrPEM := newTestCSR(t, "spiffe://example.org/old-service")

	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/new-service", // authoritative
		CSR:      csrPEM,
	}

	result, err := mo.GenerateSVID(req, ca.certPath, ca.keyPath)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, SVIDGenerationTypeEnrolled, result.GenerationType)

	block, _ := pem.Decode(result.Cert)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	require.Len(t, cert.URIs, 1)
	assert.Equal(t, "spiffe://example.org/new-service", cert.URIs[0].String(),
		"cert URI SAN must reflect req.SpiffeID, not the CSR's URI SAN")
}

func TestGenerateSVID_EnrolledMode_NoDNSSANs(t *testing.T) {
	ca := newTestCA(t)
	mo := newMintOps()

	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/service/worker",
		CSR:      newTestCSR(t, "spiffe://example.org/service/worker"),
		DNS:      nil,
	}

	result, err := mo.GenerateSVID(req, ca.certPath, ca.keyPath)

	require.NoError(t, err)
	require.NotNil(t, result)

	block, _ := pem.Decode(result.Cert)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	assert.Empty(t, cert.DNSNames)
}

func TestGenerateSVID_EnrolledMode_CertValidity(t *testing.T) {
	ca := newTestCA(t)
	mo := newMintOps()

	before := time.Now().Add(-time.Second)

	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/svc",
		CSR:      newTestCSR(t, "spiffe://example.org/svc"),
	}

	result, err := mo.GenerateSVID(req, ca.certPath, ca.keyPath)
	require.NoError(t, err)

	after := time.Now().Add(time.Second)

	block, _ := pem.Decode(result.Cert)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	assert.True(t, cert.NotBefore.After(before) || cert.NotBefore.Equal(before),
		"NotBefore should be >= test start")
	assert.True(t, cert.NotAfter.After(after),
		"NotAfter should be in the future")
	assert.True(t, cert.NotAfter.After(cert.NotBefore),
		"NotAfter must be after NotBefore")
}

func TestGenerateSVID_EnrolledMode_InvalidCSRPEM(t *testing.T) {
	// Non-empty but non-PEM CSR → hard error, no fallback to installed mode.
	ca := newTestCA(t)
	mo := newMintOps()

	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/svc",
		CSR:      "not-a-pem",
	}

	result, err := mo.GenerateSVID(req, ca.certPath, ca.keyPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode PEM block from CSR")
	assert.Nil(t, result)
}

func TestGenerateSVID_EnrolledMode_InvalidCSRDER(t *testing.T) {
	ca := newTestCA(t)
	mo := newMintOps()

	garbage := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: []byte("garbage")})
	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/svc",
		CSR:      string(garbage),
	}

	result, err := mo.GenerateSVID(req, ca.certPath, ca.keyPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing CSR")
	assert.Nil(t, result)
}

func TestGenerateSVID_EnrolledMode_TamperedCSRSignature(t *testing.T) {
	// Corrupt the CSR bytes after creation to invalidate the signature.
	ca := newTestCA(t)
	mo := newMintOps()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	u, _ := url.Parse("spiffe://example.org/svc")
	template := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "test"},
		URIs:    []*url.URL{u},
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	require.NoError(t, err)

	// Flip a byte in the signature area (last 8 bytes).
	csrDER[len(csrDER)-1] ^= 0xFF

	csrPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER}))

	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/svc",
		CSR:      csrPEM,
	}

	result, err := mo.GenerateSVID(req, ca.certPath, ca.keyPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CSR signature invalid")
	assert.Nil(t, result)
}

func TestGenerateSVID_EnrolledMode_NoURISAN_IsWarningOnly(t *testing.T) {
	// A CSR with no URI SANs logs a warning but does NOT error in enrolled mode
	// (the SPIFFE ID comes from req.SpiffeID).
	ca := newTestCA(t)
	mo := newMintOps()

	csrPEM := newTestCSR(t, "") // no URI SAN

	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/svc",
		CSR:      csrPEM,
	}

	result, err := mo.GenerateSVID(req, ca.certPath, ca.keyPath)

	// The implementation only warns; it should still succeed.
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, SVIDGenerationTypeEnrolled, result.GenerationType)
}

func TestGenerateSVID_EnrolledMode_MultipleURISANs_IsWarningOnly(t *testing.T) {
	// A CSR with multiple URI SANs logs a warning but does NOT error in enrolled mode.
	ca := newTestCA(t)
	mo := newMintOps()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	u1, _ := url.Parse("spiffe://example.org/svc1")
	u2, _ := url.Parse("spiffe://example.org/svc2")
	template := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "test"},
		URIs:    []*url.URL{u1, u2},
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	require.NoError(t, err)
	csrPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER}))

	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/svc1",
		CSR:      csrPEM,
	}

	result, err := mo.GenerateSVID(req, ca.certPath, ca.keyPath)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, SVIDGenerationTypeEnrolled, result.GenerationType)
}

// ---------------------------------------------------------------------------
// GenerateSVID — shared CA / SPIFFE ID error paths (both modes)
// ---------------------------------------------------------------------------

func TestGenerateSVID_CACertFileNotFound(t *testing.T) {
	mo := newMintOps()

	req := &types.MintSVIDRequest{SpiffeID: "spiffe://example.org/svc"}

	result, err := mo.GenerateSVID(req, "/nonexistent/ca.crt", "/nonexistent/ca.key")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reading CA cert file")
	assert.Nil(t, result)
}

func TestGenerateSVID_CACertInvalidPEM(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.crt")
	require.NoError(t, os.WriteFile(certPath, []byte("not-a-pem"), 0o600))

	mo := newMintOps()
	req := &types.MintSVIDRequest{SpiffeID: "spiffe://example.org/svc"}

	result, err := mo.GenerateSVID(req, certPath, "/unused/ca.key")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode PEM block from CA certificate file")
	assert.Nil(t, result)
}

func TestGenerateSVID_CACertInvalidDER(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.crt")
	garbage := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("garbage")})
	require.NoError(t, os.WriteFile(certPath, garbage, 0o600))

	mo := newMintOps()
	req := &types.MintSVIDRequest{SpiffeID: "spiffe://example.org/svc"}

	result, err := mo.GenerateSVID(req, certPath, "/unused/ca.key")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing CA certificate")
	assert.Nil(t, result)
}

func TestGenerateSVID_CAKeyFileNotFound(t *testing.T) {
	ca := newTestCA(t)
	mo := newMintOps()

	req := &types.MintSVIDRequest{SpiffeID: "spiffe://example.org/svc"}

	result, err := mo.GenerateSVID(req, ca.certPath, "/nonexistent/ca.key")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reading CA key file")
	assert.Nil(t, result)
}

func TestGenerateSVID_CAKeyInvalidPEM(t *testing.T) {
	ca := newTestCA(t)
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "ca.key")
	require.NoError(t, os.WriteFile(keyPath, []byte("not-a-pem"), 0o600))

	mo := newMintOps()
	req := &types.MintSVIDRequest{SpiffeID: "spiffe://example.org/svc"}

	result, err := mo.GenerateSVID(req, ca.certPath, keyPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode PEM block from CA key file")
	assert.Nil(t, result)
}

func TestGenerateSVID_CAKeyInvalidECKey(t *testing.T) {
	ca := newTestCA(t)
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "ca.key")
	garbage := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: []byte("garbage")})
	require.NoError(t, os.WriteFile(keyPath, garbage, 0o600))

	mo := newMintOps()
	req := &types.MintSVIDRequest{SpiffeID: "spiffe://example.org/svc"}

	result, err := mo.GenerateSVID(req, ca.certPath, keyPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing CA private key")
	assert.Nil(t, result)
}

func TestGenerateSVID_InvalidSpiffeID(t *testing.T) {
	ca := newTestCA(t)
	mo := newMintOps()

	// Raw control character forces url.Parse to fail.
	req := &types.MintSVIDRequest{SpiffeID: "spiffe://exam ple.org/\x00bad"}

	result, err := mo.GenerateSVID(req, ca.certPath, ca.keyPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing SPIFFE ID")
	assert.Nil(t, result)
}
