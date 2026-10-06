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

// --- Tests ---

func TestGenerateX509SVID_HappyPath(t *testing.T) {
	ca := newTestCA(t)
	mo := newMintOps()

	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/service/api",
		DNS:      []string{"api.example.org", "api-internal.example.org"},
		// TTL resolved by ResolvedTTL(); assumes a default is returned when zero
	}

	certPEM, keyPEM, err := mo.GenerateX509SVID(req, ca.certPath, ca.keyPath)

	require.NoError(t, err)
	require.NotEmpty(t, certPEM)
	require.NotEmpty(t, keyPEM)

	// Decode and parse the returned certificate
	block, _ := pem.Decode(certPEM)
	require.NotNil(t, block, "certPEM must contain a valid PEM block")
	assert.Equal(t, "CERTIFICATE", block.Type)

	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	// Decode and parse the returned private key
	keyBlock, _ := pem.Decode(keyPEM)
	require.NotNil(t, keyBlock, "keyPEM must contain a valid PEM block")
	assert.Equal(t, "EC PRIVATE KEY", keyBlock.Type)

	_, err = x509.ParseECPrivateKey(keyBlock.Bytes)
	require.NoError(t, err)

	// Verify SPIFFE URI SAN
	require.Len(t, cert.URIs, 1)
	assert.Equal(t, "spiffe://example.org/service/api", cert.URIs[0].String())

	// Verify DNS SANs
	assert.ElementsMatch(t, []string{"api.example.org", "api-internal.example.org"}, cert.DNSNames)

	// Verify Subject Organization is the trust domain
	assert.Equal(t, []string{"example.org"}, cert.Subject.Organization)

	// Verify key usages
	assert.True(t, cert.KeyUsage&x509.KeyUsageDigitalSignature != 0)
	assert.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageServerAuth)
	assert.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageClientAuth)

	// Verify the cert is signed by the test CA
	pool := x509.NewCertPool()
	pool.AddCert(ca.cert)
	_, err = cert.Verify(x509.VerifyOptions{Roots: pool})
	require.NoError(t, err, "certificate must verify against the test CA")
}

func TestGenerateX509SVID_NoDNSSANs(t *testing.T) {
	ca := newTestCA(t)
	mo := newMintOps()

	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/service/worker",
		DNS:      nil,
	}

	certPEM, keyPEM, err := mo.GenerateX509SVID(req, ca.certPath, ca.keyPath)

	require.NoError(t, err)
	require.NotEmpty(t, certPEM)
	require.NotEmpty(t, keyPEM)

	block, _ := pem.Decode(certPEM)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	assert.Empty(t, cert.DNSNames)
}

func TestGenerateX509SVID_CACertFileNotFound(t *testing.T) {
	mo := newMintOps()

	req := &types.MintSVIDRequest{SpiffeID: "spiffe://example.org/svc"}

	certPEM, keyPEM, err := mo.GenerateX509SVID(req, "/nonexistent/ca.crt", "/nonexistent/ca.key")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reading CA cert file")
	assert.Nil(t, certPEM)
	assert.Nil(t, keyPEM)
}

func TestGenerateX509SVID_CACertInvalidPEM(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.crt")
	require.NoError(t, os.WriteFile(certPath, []byte("not-a-pem"), 0o600))

	mo := newMintOps()
	req := &types.MintSVIDRequest{SpiffeID: "spiffe://example.org/svc"}

	certPEM, keyPEM, err := mo.GenerateX509SVID(req, certPath, "/unused/ca.key")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode PEM block from CA certificate file")
	assert.Nil(t, certPEM)
	assert.Nil(t, keyPEM)
}

func TestGenerateX509SVID_CACertInvalidDER(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.crt")
	garbage := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("garbage")})
	require.NoError(t, os.WriteFile(certPath, garbage, 0o600))

	mo := newMintOps()
	req := &types.MintSVIDRequest{SpiffeID: "spiffe://example.org/svc"}

	certPEM, keyPEM, err := mo.GenerateX509SVID(req, certPath, "/unused/ca.key")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing CA certificate")
	assert.Nil(t, certPEM)
	assert.Nil(t, keyPEM)
}

func TestGenerateX509SVID_CAKeyFileNotFound(t *testing.T) {
	ca := newTestCA(t)
	mo := newMintOps()

	req := &types.MintSVIDRequest{SpiffeID: "spiffe://example.org/svc"}

	certPEM, keyPEM, err := mo.GenerateX509SVID(req, ca.certPath, "/nonexistent/ca.key")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reading CA key file")
	assert.Nil(t, certPEM)
	assert.Nil(t, keyPEM)
}

func TestGenerateX509SVID_CAKeyInvalidPEM(t *testing.T) {
	ca := newTestCA(t)
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "ca.key")
	require.NoError(t, os.WriteFile(keyPath, []byte("not-a-pem"), 0o600))

	mo := newMintOps()
	req := &types.MintSVIDRequest{SpiffeID: "spiffe://example.org/svc"}

	certPEM, keyPEM, err := mo.GenerateX509SVID(req, ca.certPath, keyPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode PEM block from CA key file")
	assert.Nil(t, certPEM)
	assert.Nil(t, keyPEM)
}

func TestGenerateX509SVID_CAKeyInvalidECKey(t *testing.T) {
	ca := newTestCA(t)
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "ca.key")
	garbage := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: []byte("garbage")})
	require.NoError(t, os.WriteFile(keyPath, garbage, 0o600))

	mo := newMintOps()
	req := &types.MintSVIDRequest{SpiffeID: "spiffe://example.org/svc"}

	certPEM, keyPEM, err := mo.GenerateX509SVID(req, ca.certPath, keyPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing CA private key")
	assert.Nil(t, certPEM)
	assert.Nil(t, keyPEM)
}

func TestGenerateX509SVID_InvalidSpiffeID(t *testing.T) {
	ca := newTestCA(t)
	mo := newMintOps()

	// A raw control character makes url.Parse fail
	req := &types.MintSVIDRequest{SpiffeID: "spiffe://exam ple.org/\x00bad"}

	certPEM, keyPEM, err := mo.GenerateX509SVID(req, ca.certPath, ca.keyPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing SPIFFE ID")
	assert.Nil(t, certPEM)
	assert.Nil(t, keyPEM)
}

func TestGenerateX509SVID_CertValidity(t *testing.T) {
	ca := newTestCA(t)
	mo := newMintOps()

	before := time.Now().Add(-time.Second)

	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/svc",
	}

	certPEM, _, err := mo.GenerateX509SVID(req, ca.certPath, ca.keyPath)
	require.NoError(t, err)

	after := time.Now().Add(time.Second)

	block, _ := pem.Decode(certPEM)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	assert.True(t, cert.NotBefore.After(before) || cert.NotBefore.Equal(before),
		"NotBefore should be >= test start")
	assert.True(t, cert.NotAfter.After(after),
		"NotAfter should be in the future")
	assert.True(t, cert.NotAfter.After(cert.NotBefore),
		"NotAfter must be after NotBefore")
}

func TestGenerateX509SVIDUsingCSR_HappyPath(t *testing.T) {
	ca := newTestCA(t)
	mo := newMintOps()

	csrPEM := newTestCSR(t, "spiffe://example.org/service/api")

	req := &types.MintSVIDRequest{
		CSR: csrPEM,
		DNS: []string{"api.example.org", "api-internal.example.org"},
	}

	certPEM, err := mo.GenerateX509SVIDUsingCSR(req, ca.certPath, ca.keyPath)

	require.NoError(t, err)
	require.NotEmpty(t, certPEM)

	block, _ := pem.Decode(certPEM)
	require.NotNil(t, block)
	assert.Equal(t, "CERTIFICATE", block.Type)

	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	// SPIFFE URI SAN sourced from CSR
	require.Len(t, cert.URIs, 1)
	assert.Equal(t, "spiffe://example.org/service/api", cert.URIs[0].String())

	// DNS SANs
	assert.ElementsMatch(t, []string{"api.example.org", "api-internal.example.org"}, cert.DNSNames)

	// Subject Organization is the trust domain
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

func TestGenerateX509SVIDUsingCSR_NoDNSSANs(t *testing.T) {
	ca := newTestCA(t)
	mo := newMintOps()

	req := &types.MintSVIDRequest{
		CSR: newTestCSR(t, "spiffe://example.org/service/worker"),
		DNS: nil,
	}

	certPEM, err := mo.GenerateX509SVIDUsingCSR(req, ca.certPath, ca.keyPath)

	require.NoError(t, err)
	require.NotEmpty(t, certPEM)

	block, _ := pem.Decode(certPEM)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	assert.Empty(t, cert.DNSNames)
}

func TestGenerateX509SVIDUsingCSR_CACertFileNotFound(t *testing.T) {
	mo := newMintOps()

	req := &types.MintSVIDRequest{CSR: newTestCSR(t, "spiffe://example.org/svc")}

	certPEM, err := mo.GenerateX509SVIDUsingCSR(req, "/nonexistent/ca.crt", "/nonexistent/ca.key")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reading CA cert file")
	assert.Nil(t, certPEM)
}

func TestGenerateX509SVIDUsingCSR_CACertInvalidPEM(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.crt")
	require.NoError(t, os.WriteFile(certPath, []byte("not-a-pem"), 0o600))

	mo := newMintOps()
	req := &types.MintSVIDRequest{CSR: newTestCSR(t, "spiffe://example.org/svc")}

	certPEM, err := mo.GenerateX509SVIDUsingCSR(req, certPath, "/unused/ca.key")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode PEM block from CA certificate file")
	assert.Nil(t, certPEM)
}

func TestGenerateX509SVIDUsingCSR_CACertInvalidDER(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.crt")
	garbage := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("garbage")})
	require.NoError(t, os.WriteFile(certPath, garbage, 0o600))

	mo := newMintOps()
	req := &types.MintSVIDRequest{CSR: newTestCSR(t, "spiffe://example.org/svc")}

	certPEM, err := mo.GenerateX509SVIDUsingCSR(req, certPath, "/unused/ca.key")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing CA certificate")
	assert.Nil(t, certPEM)
}

func TestGenerateX509SVIDUsingCSR_CAKeyFileNotFound(t *testing.T) {
	ca := newTestCA(t)
	mo := newMintOps()

	req := &types.MintSVIDRequest{CSR: newTestCSR(t, "spiffe://example.org/svc")}

	certPEM, err := mo.GenerateX509SVIDUsingCSR(req, ca.certPath, "/nonexistent/ca.key")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reading CA key file")
	assert.Nil(t, certPEM)
}

func TestGenerateX509SVIDUsingCSR_CAKeyInvalidPEM(t *testing.T) {
	ca := newTestCA(t)
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "ca.key")
	require.NoError(t, os.WriteFile(keyPath, []byte("not-a-pem"), 0o600))

	mo := newMintOps()
	req := &types.MintSVIDRequest{CSR: newTestCSR(t, "spiffe://example.org/svc")}

	certPEM, err := mo.GenerateX509SVIDUsingCSR(req, ca.certPath, keyPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode PEM block from CA key file")
	assert.Nil(t, certPEM)
}

func TestGenerateX509SVIDUsingCSR_CAKeyInvalidECKey(t *testing.T) {
	ca := newTestCA(t)
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "ca.key")
	garbage := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: []byte("garbage")})
	require.NoError(t, os.WriteFile(keyPath, garbage, 0o600))

	mo := newMintOps()
	req := &types.MintSVIDRequest{CSR: newTestCSR(t, "spiffe://example.org/svc")}

	certPEM, err := mo.GenerateX509SVIDUsingCSR(req, ca.certPath, keyPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing CA private key")
	assert.Nil(t, certPEM)
}

func TestGenerateX509SVIDUsingCSR_InvalidCSRPEM(t *testing.T) {
	ca := newTestCA(t)
	mo := newMintOps()

	req := &types.MintSVIDRequest{CSR: "not-a-pem"}

	certPEM, err := mo.GenerateX509SVIDUsingCSR(req, ca.certPath, ca.keyPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode PEM block from CSR")
	assert.Nil(t, certPEM)
}

func TestGenerateX509SVIDUsingCSR_InvalidCSRDER(t *testing.T) {
	ca := newTestCA(t)
	mo := newMintOps()

	garbage := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: []byte("garbage")})
	req := &types.MintSVIDRequest{CSR: string(garbage)}

	certPEM, err := mo.GenerateX509SVIDUsingCSR(req, ca.certPath, ca.keyPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing CSR")
	assert.Nil(t, certPEM)
}

func TestGenerateX509SVIDUsingCSR_NoURISAN(t *testing.T) {
	ca := newTestCA(t)
	mo := newMintOps()

	// CSR with no URI SANs
	req := &types.MintSVIDRequest{CSR: newTestCSR(t, "")}

	certPEM, err := mo.GenerateX509SVIDUsingCSR(req, ca.certPath, ca.keyPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one URI SAN")
	assert.Nil(t, certPEM)
}

func TestGenerateX509SVIDUsingCSR_MultipleURISANs(t *testing.T) {
	ca := newTestCA(t)
	mo := newMintOps()

	// Manually build a CSR with two URI SANs
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

	req := &types.MintSVIDRequest{CSR: csrPEM}

	certPEM, err := mo.GenerateX509SVIDUsingCSR(req, ca.certPath, ca.keyPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one URI SAN")
	assert.Nil(t, certPEM)
}

func TestGenerateX509SVIDUsingCSR_CertValidity(t *testing.T) {
	ca := newTestCA(t)
	mo := newMintOps()

	before := time.Now().Add(-time.Second)

	req := &types.MintSVIDRequest{
		CSR: newTestCSR(t, "spiffe://example.org/svc"),
	}

	certPEM, err := mo.GenerateX509SVIDUsingCSR(req, ca.certPath, ca.keyPath)
	require.NoError(t, err)

	after := time.Now().Add(time.Second)

	block, _ := pem.Decode(certPEM)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	assert.True(t, cert.NotBefore.After(before) || cert.NotBefore.Equal(before),
		"NotBefore should be >= test start")
	assert.True(t, cert.NotAfter.After(after),
		"NotAfter should be in the future")
	assert.True(t, cert.NotAfter.After(cert.NotBefore),
		"NotAfter must be after NotBefore")
}
