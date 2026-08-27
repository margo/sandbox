package operations

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/margo/sandbox/mis/pkg/types"
	"github.com/margo/sandbox/shared-lib/pointers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMintOps() *MintOperations {
	return New()
}

// parseCert decodes PEM and parses the X.509 certificate.
func parseCert(t *testing.T, certPEM []byte) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode(certPEM)
	require.NotNil(t, block, "expected PEM block in certificate output")
	require.Equal(t, "CERTIFICATE", block.Type)

	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	return cert
}

// parseKey decodes PEM and parses the EC private key.
func parseKey(t *testing.T, keyPEM []byte) *ecdsa.PrivateKey {
	t.Helper()
	block, _ := pem.Decode(keyPEM)
	require.NotNil(t, block, "expected PEM block in key output")
	require.Equal(t, "EC PRIVATE KEY", block.Type)

	key, err := x509.ParseECPrivateKey(block.Bytes)
	require.NoError(t, err)
	return key
}

func TestGenerateX509SVID_HappyPath(t *testing.T) {
	mo := newMintOps()
	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/workload",
		// Assumes ResolvedTTL() returns a non-zero duration; adjust field names
		// to match your actual MintSVIDRequest struct.
		TTL: pointers.Ptr(time.Hour),
	}

	certPEM, keyPEM, err := mo.GenerateX509SVID(req)

	require.NoError(t, err)
	require.NotEmpty(t, certPEM)
	require.NotEmpty(t, keyPEM)
}

func TestGenerateX509SVID_ReturnsPEMEncodedCertificate(t *testing.T) {
	mo := newMintOps()
	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/workload",
		TTL:      pointers.Ptr(time.Hour),
	}

	certPEM, _, err := mo.GenerateX509SVID(req)
	require.NoError(t, err)

	cert := parseCert(t, certPEM)
	assert.NotNil(t, cert.SerialNumber)
}

func TestGenerateX509SVID_ReturnsPEMEncodedECPrivateKey(t *testing.T) {
	mo := newMintOps()
	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/workload",
		TTL:      pointers.Ptr(time.Hour),
	}

	_, keyPEM, err := mo.GenerateX509SVID(req)
	require.NoError(t, err)

	key := parseKey(t, keyPEM)
	assert.Equal(t, 256, key.Curve.Params().BitSize, "expected P-256 curve")
}

func TestGenerateX509SVID_CertificatePublicKeyMatchesPrivateKey(t *testing.T) {
	mo := newMintOps()
	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/workload",
		TTL:      pointers.Ptr(time.Hour),
	}

	certPEM, keyPEM, err := mo.GenerateX509SVID(req)
	require.NoError(t, err)

	cert := parseCert(t, certPEM)
	key := parseKey(t, keyPEM)

	certPubKey, ok := cert.PublicKey.(*ecdsa.PublicKey)
	require.True(t, ok, "certificate public key should be ECDSA")
	assert.True(t, certPubKey.Equal(&key.PublicKey), "cert public key must match private key")
}

func TestGenerateX509SVID_SPIFFEIDSetAsURISAN(t *testing.T) {
	mo := newMintOps()
	spiffeID := "spiffe://example.org/ns/default/sa/myservice"
	req := &types.MintSVIDRequest{
		SpiffeID: spiffeID,
		TTL:      pointers.Ptr(time.Hour),
	}

	certPEM, _, err := mo.GenerateX509SVID(req)
	require.NoError(t, err)

	cert := parseCert(t, certPEM)
	require.Len(t, cert.URIs, 1, "expected exactly one URI SAN")
	assert.Equal(t, spiffeID, cert.URIs[0].String())
}

func TestGenerateX509SVID_SubjectOrganizationIsTrustDomain(t *testing.T) {
	mo := newMintOps()
	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/workload",
		TTL:      pointers.Ptr(time.Hour),
	}

	certPEM, _, err := mo.GenerateX509SVID(req)
	require.NoError(t, err)

	cert := parseCert(t, certPEM)
	require.NotEmpty(t, cert.Subject.Organization)
	assert.Equal(t, "example.org", cert.Subject.Organization[0])
}

func TestGenerateX509SVID_KeyUsageFlags(t *testing.T) {
	mo := newMintOps()
	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/workload",
		TTL:      pointers.Ptr(time.Hour),
	}

	certPEM, _, err := mo.GenerateX509SVID(req)
	require.NoError(t, err)

	cert := parseCert(t, certPEM)
	assert.True(
		t,
		cert.KeyUsage&x509.KeyUsageDigitalSignature != 0,
		"DigitalSignature usage expected",
	)
	assert.True(t, cert.KeyUsage&x509.KeyUsageCertSign != 0, "CertSign usage expected")
}

func TestGenerateX509SVID_ExtKeyUsage(t *testing.T) {
	mo := newMintOps()
	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/workload",
		TTL:      pointers.Ptr(time.Hour),
	}

	certPEM, _, err := mo.GenerateX509SVID(req)
	require.NoError(t, err)

	cert := parseCert(t, certPEM)
	assert.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageServerAuth)
	assert.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageClientAuth)
}

func TestGenerateX509SVID_ValidityPeriodRespectsTTL(t *testing.T) {
	mo := newMintOps()
	ttl := pointers.Ptr((2 * time.Hour))
	before := time.Now()

	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/workload",
		TTL:      ttl,
	}

	certPEM, _, err := mo.GenerateX509SVID(req)
	require.NoError(t, err)

	after := time.Now()
	cert := parseCert(t, certPEM)

	assert.True(t, cert.NotBefore.After(before.Add(-time.Second)), "NotBefore should be ~now")
	assert.True(t, cert.NotBefore.Before(after.Add(time.Second)), "NotBefore should be ~now")

	expectedExpiry := cert.NotBefore.Add(time.Duration(*ttl))
	// fmt.Println(expectedExpiry.String(), cert.NotAfter.String())
	// Allow 2-second drift for test execution time
	assert.WithinDuration(t, expectedExpiry, cert.NotAfter, 2*time.Second)
}

func TestGenerateX509SVID_DNSNamesPopulated(t *testing.T) {
	mo := newMintOps()
	dnsNames := []string{"myservice.default.svc.cluster.local", "myservice"}
	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/workload",
		TTL:      pointers.Ptr(time.Hour),
		DNS:      dnsNames,
	}

	certPEM, _, err := mo.GenerateX509SVID(req)
	require.NoError(t, err)

	cert := parseCert(t, certPEM)
	assert.Equal(t, dnsNames, cert.DNSNames)
}

func TestGenerateX509SVID_NoDNSNames(t *testing.T) {
	mo := newMintOps()
	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/workload",
		TTL:      pointers.Ptr(time.Hour),
		DNS:      nil,
	}

	certPEM, _, err := mo.GenerateX509SVID(req)
	require.NoError(t, err)

	cert := parseCert(t, certPEM)
	assert.Empty(t, cert.DNSNames)
}

func TestGenerateX509SVID_InvalidSpiffeID_ReturnsError(t *testing.T) {
	mo := newMintOps()
	req := &types.MintSVIDRequest{
		// Colon in host makes this an invalid URL
		SpiffeID: "://invalid url\x00",
		TTL:      pointers.Ptr(time.Hour),
	}

	certPEM, keyPEM, err := mo.GenerateX509SVID(req)

	assert.Error(t, err)
	assert.Nil(t, certPEM)
	assert.Nil(t, keyPEM)
}

func TestGenerateX509SVID_BasicConstraintsValid(t *testing.T) {
	mo := newMintOps()
	req := &types.MintSVIDRequest{
		SpiffeID: "spiffe://example.org/workload",
		TTL:      pointers.Ptr(time.Hour),
	}

	certPEM, _, err := mo.GenerateX509SVID(req)
	require.NoError(t, err)

	cert := parseCert(t, certPEM)
	assert.True(t, cert.BasicConstraintsValid)
}
