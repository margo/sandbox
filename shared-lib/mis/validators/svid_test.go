package validators

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Test Helpers ──────────────────────────────────────────────────────────────
// ── generateCert: add keyUsage support to certOptions ────────────────────────

// Add keyUsage field to certOptions (update the existing struct):
type certOptions struct {
	spiffeID  string
	extraURIs []string
	isCA      bool
	keyUsage  x509.KeyUsage // NEW: explicit key usage override for leaf certs
	notBefore time.Time
	notAfter  time.Time
	parent    *x509.Certificate
	parentKey *ecdsa.PrivateKey
}

// ─────────────────────────────────────────────
// HELPER
// ─────────────────────────────────────────────

func writeTempFile(t *testing.T, data []byte) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "test-*")
	require.NoError(t, err)
	_, err = f.Write(data)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

// ─────────────────────────────────────────────
// TEST HELPERS
// ─────────────────────────────────────────────

// generateRSACertAndKey generates a self-signed RSA cert + key PEM pair
func generateRSACertAndKey(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-rsa"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privKey.PublicKey, privKey)
	require.NoError(t, err)

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalPKCS8PrivateKey(privKey)
	require.NoError(t, err)
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM
}

// generateECDSACertAndKey generates a self-signed ECDSA cert + key PEM pair
func generateECDSACertAndKey(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test-ecdsa"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privKey.PublicKey, privKey)
	require.NoError(t, err)

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(privKey)
	require.NoError(t, err)
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM
}

// generateEd25519CertAndKey generates a self-signed Ed25519 cert + key PEM pair
func generateEd25519CertAndKey(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "test-ed25519"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, pubKey, privKey)
	require.NoError(t, err)

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalPKCS8PrivateKey(privKey)
	require.NoError(t, err)
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM
}

// Update generateCert to apply keyUsage when set (update the existing function):
func generateCert(t *testing.T, opts certOptions) ([]byte, *ecdsa.PrivateKey) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	if opts.notBefore.IsZero() {
		opts.notBefore = time.Now().Add(-time.Hour)
	}
	if opts.notAfter.IsZero() {
		opts.notAfter = time.Now().Add(time.Hour)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    opts.notBefore,
		NotAfter:     opts.notAfter,
		IsCA:         opts.isCA,
	}

	if opts.isCA {
		tmpl.BasicConstraintsValid = true
		tmpl.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	}

	// NEW: apply explicit key usage override (for leaf constraint tests)
	if opts.keyUsage != 0 {
		tmpl.KeyUsage = opts.keyUsage
	}

	if opts.spiffeID != "" {
		u, err := url.Parse(opts.spiffeID)
		require.NoError(t, err)
		tmpl.URIs = append(tmpl.URIs, u)
	}
	for _, extra := range opts.extraURIs {
		u, err := url.Parse(extra)
		require.NoError(t, err)
		tmpl.URIs = append(tmpl.URIs, u)
	}

	signerCert := tmpl
	signerKey := key
	if opts.parent != nil {
		signerCert = opts.parent
		signerKey = opts.parentKey
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, signerCert, &key.PublicKey, signerKey)
	require.NoError(t, err)
	return der, key
}

// ── validateSVIDLeafConstraints ───────────────────────────────────────────────

func TestValidateSVIDLeafConstraints_Valid(t *testing.T) {
	der, _ := generateCert(t, certOptions{spiffeID: "spiffe://example.org/margo/wfm/w1"})
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	assert.NoError(t, ValidateSVIDLeafConstraints(cert))
}

func TestValidateSVIDLeafConstraints_IsCA(t *testing.T) {
	der, _ := generateCert(t, certOptions{
		spiffeID: "spiffe://example.org/margo/wfm/w1",
		isCA:     true,
	})
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	err = ValidateSVIDLeafConstraints(cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be a CA")
}

func TestValidateSVIDLeafConstraints_KeyCertSignSet(t *testing.T) {
	der, _ := generateCert(t, certOptions{
		spiffeID: "spiffe://example.org/margo/wfm/w1",
		keyUsage: x509.KeyUsageCertSign,
	})
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	err = ValidateSVIDLeafConstraints(cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "keyCertSign")
}

func TestValidateSVIDLeafConstraints_CRLSignSet(t *testing.T) {
	der, _ := generateCert(t, certOptions{
		spiffeID: "spiffe://example.org/margo/wfm/w1",
		keyUsage: x509.KeyUsageCRLSign,
	})
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	err = ValidateSVIDLeafConstraints(cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cRLSign")
}

func TestValidateSVIDLeafConstraints_RootPath(t *testing.T) {
	// SPIFFE ID with root path "/" — must be rejected
	der, _ := generateCert(t, certOptions{spiffeID: "spiffe://example.org/"})
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	err = ValidateSVIDLeafConstraints(cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-root path")
}

func TestValidateSVIDLeafConstraints_EmptyPath(t *testing.T) {
	// SPIFFE ID with no path — must be rejected
	der, _ := generateCert(t, certOptions{spiffeID: "spiffe://example.org"})
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	err = ValidateSVIDLeafConstraints(cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-root path")
}

func TestValidateSVIDLeafConstraints_NoURISAN(t *testing.T) {
	der, _ := generateCert(t, certOptions{}) // no spiffeID
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	err = ValidateSVIDLeafConstraints(cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one URI SAN")
}

func TestValidateSVIDLeafConstraints_MultipleURISANs(t *testing.T) {
	der, _ := generateCert(t, certOptions{
		spiffeID:  "spiffe://example.org/margo/wfm/w1",
		extraURIs: []string{"spiffe://example.org/margo/wfm/w2"},
	})
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	err = ValidateSVIDLeafConstraints(cert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one URI SAN")
}

// ── ValidateX509SVID: new leaf constraint cases ───────────────────────────────

func TestValidateX509SVID_RejectsCACert(t *testing.T) {
	der, _ := generateCert(t, certOptions{
		spiffeID: "spiffe://example.org/margo/wfm/w1",
		isCA:     true,
	})
	ok, err := ValidateX509SVID(toPEM(der), PrincipalWFM)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be a CA")
}

func TestValidateX509SVID_RejectsKeyCertSign(t *testing.T) {
	der, _ := generateCert(t, certOptions{
		spiffeID: "spiffe://example.org/margo/wfm/w1",
		keyUsage: x509.KeyUsageCertSign,
	})
	ok, err := ValidateX509SVID(toPEM(der), PrincipalWFM)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "keyCertSign")
}

func TestValidateX509SVID_RejectsCRLSign(t *testing.T) {
	der, _ := generateCert(t, certOptions{
		spiffeID: "spiffe://example.org/margo/wfm/w1",
		keyUsage: x509.KeyUsageCRLSign,
	})
	ok, err := ValidateX509SVID(toPEM(der), PrincipalWFM)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cRLSign")
}

func TestValidateX509SVID_RejectsRootPath(t *testing.T) {
	der, _ := generateCert(t, certOptions{spiffeID: "spiffe://example.org/"})
	ok, err := ValidateX509SVID(toPEM(der), PrincipalWFM)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "spiffe ID path must not contain empty segments")
}

func TestValidateX509SVID_RejectsEmptyPath(t *testing.T) {
	der, _ := generateCert(t, certOptions{spiffeID: "spiffe://example.org"})
	ok, err := ValidateX509SVID(toPEM(der), PrincipalWFM)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "spiffe ID must contain a path after the trust domain")
}

// toPEM wraps DER bytes in a CERTIFICATE PEM block.
func toPEM(der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

// buildTrustBundle creates a JWK Set JSON containing the given CA cert DER bytes.
func buildTrustBundle(t *testing.T, caDERs ...[]byte) []byte {
	t.Helper()
	var keys []jwkKey
	for _, der := range caDERs {
		keys = append(keys, jwkKey{
			Use: "x509-svid",
			Kty: "EC",
			X5C: []string{base64.StdEncoding.EncodeToString(der)},
		})
	}
	b, err := json.Marshal(jwkSet{Keys: keys})
	require.NoError(t, err)
	return b
}

// ── parseCertificate ──────────────────────────────────────────────────────────

func TestParseCertificate_PEM(t *testing.T) {
	der, _ := generateCert(t, certOptions{spiffeID: "spiffe://example.org/margo/wfm/w1"})
	cert, err := parseCertificate(toPEM(der))
	require.NoError(t, err)
	assert.NotNil(t, cert)
}

func TestParseCertificate_DER(t *testing.T) {
	der, _ := generateCert(t, certOptions{spiffeID: "spiffe://example.org/margo/wfm/w1"})
	cert, err := parseCertificate(der)
	require.NoError(t, err)
	assert.NotNil(t, cert)
}

func TestParseCertificate_InvalidPEMType(t *testing.T) {
	block := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("junk")})
	_, err := parseCertificate(block)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PEM block is not a certificate")
}

func TestParseCertificate_InvalidData(t *testing.T) {
	_, err := parseCertificate([]byte("not a cert"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse as PEM or DER")
}

// ── validateSPIFFEIDFormat ────────────────────────────────────────────────────

func TestValidateSPIFFEIDFormat_Valid(t *testing.T) {
	u, _ := url.Parse("spiffe://example.org/margo/wfm/w1")
	assert.NoError(t, validateSPIFFEIDFormat(u))
}

func TestValidateSPIFFEIDFormat_WrongScheme(t *testing.T) {
	u, _ := url.Parse("https://example.org/margo/wfm/w1")
	err := validateSPIFFEIDFormat(u)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scheme must be 'spiffe'")
}

func TestValidateSPIFFEIDFormat_EmptyHost(t *testing.T) {
	u := &url.URL{Scheme: "spiffe", Host: ""}
	err := validateSPIFFEIDFormat(u)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trust domain (host) must not be empty")
}

func TestValidateSPIFFEIDFormat_HostWithPort(t *testing.T) {
	u, _ := url.Parse("spiffe://example.org:8080/margo/wfm/w1")
	err := validateSPIFFEIDFormat(u)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trust domain must not contain a port")
}

func TestValidateSPIFFEIDFormat_WithUserInfo(t *testing.T) {
	u, _ := url.Parse("spiffe://user@example.org/margo/wfm/w1")
	err := validateSPIFFEIDFormat(u)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not contain user info")
}

func TestValidateSPIFFEIDFormat_WithFragment(t *testing.T) {
	u, _ := url.Parse("spiffe://example.org/margo/wfm/w1#frag")
	err := validateSPIFFEIDFormat(u)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not contain a fragment")
}

func TestValidateSPIFFEIDFormat_WithQuery(t *testing.T) {
	u, _ := url.Parse("spiffe://example.org/margo/wfm/w1?foo=bar")
	err := validateSPIFFEIDFormat(u)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not contain a query string")
}

// ── validateSPIFFEID (internal) ───────────────────────────────────────────────

func TestValidateSPIFFEID_NoURISAN(t *testing.T) {
	der, _ := generateCert(t, certOptions{}) // no spiffeID
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	err = validateSPIFFEID(cert, PrincipalWFM)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no URI SANs")
}

func TestValidateSPIFFEID_MultipleURISANs(t *testing.T) {
	der, _ := generateCert(t, certOptions{
		spiffeID:  "spiffe://example.org/margo/wfm/w1",
		extraURIs: []string{"spiffe://example.org/margo/wfm/w2"},
	})
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	err = validateSPIFFEID(cert, PrincipalWFM)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "2 URI SANs")
}

func TestValidateSPIFFEID_ValidWFM(t *testing.T) {
	der, _ := generateCert(t, certOptions{spiffeID: "spiffe://example.org/margo/wfm/w1"})
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	assert.NoError(t, validateSPIFFEID(cert, PrincipalWFM))
}

func TestValidateSPIFFEID_ValidWFMClient(t *testing.T) {
	der, _ := generateCert(t, certOptions{
		spiffeID: "spiffe://example.org/margo/wfm/w1/client/c1",
	})
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	assert.NoError(t, validateSPIFFEID(cert, PrincipalWFMClient))
}

// ── ValidateX509SVID ──────────────────────────────────────────────────────────

func TestValidateX509SVID_ValidWFM(t *testing.T) {
	der, _ := generateCert(t, certOptions{spiffeID: "spiffe://example.org/margo/wfm/w1"})
	ok, err := ValidateX509SVID(toPEM(der), PrincipalWFM)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestValidateX509SVID_ValidWFMClient(t *testing.T) {
	der, _ := generateCert(t, certOptions{
		spiffeID: "spiffe://example.org/margo/wfm/w1/client/c1",
	})
	ok, err := ValidateX509SVID(toPEM(der), PrincipalWFMClient)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestValidateX509SVID_ValidDEREncoding(t *testing.T) {
	der, _ := generateCert(t, certOptions{spiffeID: "spiffe://example.org/margo/wfm/w1"})
	ok, err := ValidateX509SVID(der, PrincipalWFM)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestValidateX509SVID_InvalidCertBytes(t *testing.T) {
	ok, err := ValidateX509SVID([]byte("garbage"), PrincipalWFM)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid x.509 certificate")
}

func TestValidateX509SVID_NoURISAN(t *testing.T) {
	der, _ := generateCert(t, certOptions{})
	ok, err := ValidateX509SVID(toPEM(der), PrincipalWFM)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no URI SANs")
}

func TestValidateX509SVID_WrongPrincipal(t *testing.T) {
	// WFM cert validated as WFMClient should fail
	der, _ := generateCert(t, certOptions{spiffeID: "spiffe://example.org/margo/wfm/w1"})
	ok, err := ValidateX509SVID(toPEM(der), PrincipalWFMClient)
	assert.False(t, ok)
	require.Error(t, err)
}

func TestValidateX509SVID_ExpiredCertificate(t *testing.T) {
	der, _ := generateCert(t, certOptions{
		spiffeID:  "spiffe://example.org/margo/wfm/w1",
		notBefore: time.Now().Add(-2 * time.Hour),
		notAfter:  time.Now().Add(-time.Hour), // already expired
	})
	ok, err := ValidateX509SVID(toPEM(der), PrincipalWFM)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate has expired")
}

func TestValidateX509SVID_NotYetValidCertificate(t *testing.T) {
	der, _ := generateCert(t, certOptions{
		spiffeID:  "spiffe://example.org/margo/wfm/w1",
		notBefore: time.Now().Add(time.Hour), // future
		notAfter:  time.Now().Add(2 * time.Hour),
	})
	ok, err := ValidateX509SVID(toPEM(der), PrincipalWFM)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not yet valid")
}

func TestValidateX509SVID_UnknownPrincipal(t *testing.T) {
	der, _ := generateCert(t, certOptions{spiffeID: "spiffe://example.org/margo/wfm/w1"})
	ok, err := ValidateX509SVID(toPEM(der), "unknown-principal")
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown principal")
}

// ── getTrustBundleFromJWK ─────────────────────────────────────────────────────

func TestGetTrustBundleFromJWK_Valid(t *testing.T) {
	caDER, _ := generateCert(t, certOptions{isCA: true})
	bundle := buildTrustBundle(t, caDER)
	certs, err := GetTrustBundleFromJWK(bundle)
	require.NoError(t, err)
	assert.Len(t, certs, 1)
}

func TestGetTrustBundleFromJWK_SkipsNonX509SVID(t *testing.T) {
	// A key with use != "x509-svid" should be skipped
	keys := []jwkKey{{Use: "sig", Kty: "EC", X5C: []string{"aGVsbG8="} /* "hello" */}}
	b, _ := json.Marshal(jwkSet{Keys: keys})
	_, err := GetTrustBundleFromJWK(b)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no x509-svid trust anchors found")
}

func TestGetTrustBundleFromJWK_MissingX5C(t *testing.T) {
	keys := []jwkKey{{Use: "x509-svid", Kty: "EC"}} // no X5C
	b, _ := json.Marshal(jwkSet{Keys: keys})
	_, err := GetTrustBundleFromJWK(b)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing x5c field")
}

func TestGetTrustBundleFromJWK_InvalidBase64(t *testing.T) {
	keys := []jwkKey{{Use: "x509-svid", Kty: "EC", X5C: []string{"!!!not-base64!!!"}}}
	b, _ := json.Marshal(jwkSet{Keys: keys})
	_, err := GetTrustBundleFromJWK(b)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to base64 decode")
}

func TestGetTrustBundleFromJWK_InvalidCertInX5C(t *testing.T) {
	// Valid base64 but not a valid DER certificate
	keys := []jwkKey{{
		Use: "x509-svid",
		Kty: "EC",
		X5C: []string{base64.StdEncoding.EncodeToString([]byte("not-a-cert"))},
	}}
	b, _ := json.Marshal(jwkSet{Keys: keys})
	_, err := GetTrustBundleFromJWK(b)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse certificate")
}

func TestGetTrustBundleFromJWK_InvalidJSON(t *testing.T) {
	_, err := GetTrustBundleFromJWK([]byte("{invalid json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse JWK Set")
}

func TestGetTrustBundleFromJWK_EmptyKeySet(t *testing.T) {
	b, _ := json.Marshal(jwkSet{Keys: []jwkKey{}})
	_, err := GetTrustBundleFromJWK(b)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no x509-svid trust anchors found")
}

func TestGetTrustBundleFromJWK_MultipleCAs(t *testing.T) {
	ca1DER, _ := generateCert(t, certOptions{isCA: true})
	ca2DER, _ := generateCert(t, certOptions{isCA: true})
	bundle := buildTrustBundle(t, ca1DER, ca2DER)
	certs, err := GetTrustBundleFromJWK(bundle)
	require.NoError(t, err)
	assert.Len(t, certs, 2)
}

// ── validateAgainstTrustBundle ────────────────────────────────────────────────

func TestValidateAgainstTrustBundle_Valid(t *testing.T) {
	caDER, caKey := generateCert(t, certOptions{isCA: true})
	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)

	leafDER, _ := generateCert(t, certOptions{
		spiffeID:  "spiffe://example.org/margo/wfm/w1",
		parent:    caCert,
		parentKey: caKey,
	})
	leafCert, err := x509.ParseCertificate(leafDER)
	require.NoError(t, err)

	bundle := buildTrustBundle(t, caDER)
	assert.NoError(t, validateAgainstTrustBundle(leafCert, bundle))
}

func TestValidateAgainstTrustBundle_CACertRejected(t *testing.T) {
	caDER, _ := generateCert(t, certOptions{isCA: true})
	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)

	bundle := buildTrustBundle(t, caDER)
	err = validateAgainstTrustBundle(caCert, bundle)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CA certificate")
}

func TestValidateAgainstTrustBundle_UntrustedLeaf(t *testing.T) {
	// CA used for bundle
	caDER, _ := generateCert(t, certOptions{isCA: true})

	// Leaf signed by a different CA (not in bundle)
	otherCaDER, otherCaKey := generateCert(t, certOptions{isCA: true})
	otherCaCert, err := x509.ParseCertificate(otherCaDER)
	require.NoError(t, err)

	leafDER, _ := generateCert(t, certOptions{
		spiffeID:  "spiffe://example.org/margo/wfm/w1",
		parent:    otherCaCert,
		parentKey: otherCaKey,
	})
	leafCert, err := x509.ParseCertificate(leafDER)
	require.NoError(t, err)

	bundle := buildTrustBundle(t, caDER)
	err = validateAgainstTrustBundle(leafCert, bundle)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not chain to a trusted SPIFFE bundle")
}

func TestValidateAgainstTrustBundle_EmptyBundle(t *testing.T) {
	leafDER, _ := generateCert(t, certOptions{spiffeID: "spiffe://example.org/margo/wfm/w1"})
	leafCert, err := x509.ParseCertificate(leafDER)
	require.NoError(t, err)

	// Build a bundle with no x509-svid keys
	b, _ := json.Marshal(jwkSet{Keys: []jwkKey{}})
	err = validateAgainstTrustBundle(leafCert, b)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to obtain trust bundle")
}

// ── ValidateX509SVIDAgainstTrustBundle ───────────────────────────────────────

func TestValidateX509SVIDAgainstTrustBundle_Valid(t *testing.T) {
	caDER, caKey := generateCert(t, certOptions{isCA: true})
	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)

	leafDER, _ := generateCert(t, certOptions{
		spiffeID:  "spiffe://example.org/margo/wfm/w1",
		parent:    caCert,
		parentKey: caKey,
	})

	bundle := buildTrustBundle(t, caDER)
	ok, err := ValidateX509SVIDAgainstTrustBundle(toPEM(leafDER), bundle)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestValidateX509SVIDAgainstTrustBundle_InvalidCertBytes(t *testing.T) {
	caDER, _ := generateCert(t, certOptions{isCA: true})
	bundle := buildTrustBundle(t, caDER)

	ok, err := ValidateX509SVIDAgainstTrustBundle([]byte("garbage"), bundle)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse SVID certificate")
}

func TestValidateX509SVIDAgainstTrustBundle_UntrustedCert(t *testing.T) {
	caDER, _ := generateCert(t, certOptions{isCA: true})
	bundle := buildTrustBundle(t, caDER)

	// Self-signed leaf (not signed by the CA in the bundle)
	leafDER, _ := generateCert(t, certOptions{spiffeID: "spiffe://example.org/margo/wfm/w1"})

	ok, err := ValidateX509SVIDAgainstTrustBundle(toPEM(leafDER), bundle)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not chain to a trusted SPIFFE bundle")
}

func TestValidateX509SVIDAgainstTrustBundle_InvalidTrustBundle(t *testing.T) {
	leafDER, _ := generateCert(t, certOptions{spiffeID: "spiffe://example.org/margo/wfm/w1"})

	ok, err := ValidateX509SVIDAgainstTrustBundle(toPEM(leafDER), []byte("not-json"))
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to obtain trust bundle")
}

func TestValidateX509SVIDAgainstTrustBundle_CACertAsLeaf(t *testing.T) {
	caDER, _ := generateCert(t, certOptions{isCA: true})
	bundle := buildTrustBundle(t, caDER)

	ok, err := ValidateX509SVIDAgainstTrustBundle(toPEM(caDER), bundle)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CA certificate")
}

func TestValidateX509SVIDAgainstTrustBundle_DEREncodedLeaf(t *testing.T) {
	caDER, caKey := generateCert(t, certOptions{isCA: true})
	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)

	leafDER, _ := generateCert(t, certOptions{
		spiffeID:  "spiffe://example.org/margo/wfm/w1",
		parent:    caCert,
		parentKey: caKey,
	})

	bundle := buildTrustBundle(t, caDER)
	ok, err := ValidateX509SVIDAgainstTrustBundle(leafDER, bundle) // raw DER, not PEM
	require.NoError(t, err)
	assert.True(t, ok)
}

// ─────────────────────────────────────────────
// UNIT TESTS
// ─────────────────────────────────────────────

func TestValidateCertKeyPair(t *testing.T) {

	// ── Generate all key pairs upfront ──
	rsaCert, rsaKey := generateRSACertAndKey(t)
	ecdsaCert, ecdsaKey := generateECDSACertAndKey(t)
	ed25519Cert, ed25519Key := generateEd25519CertAndKey(t)

	// Generate mismatched keys (different key of same type)
	_, rsaKey2 := generateRSACertAndKey(t)
	_, ecdsaKey2 := generateECDSACertAndKey(t)
	_, ed25519Key2 := generateEd25519CertAndKey(t)

	tests := []struct {
		name        string
		certPEM     []byte
		keyPEM      []byte
		wantErr     bool
		errContains string
	}{
		// ── Happy Path ──
		{
			name:    "valid RSA cert-key pair (PKCS8)",
			certPEM: rsaCert,
			keyPEM:  rsaKey,
			wantErr: false,
		},
		{
			name:    "valid ECDSA cert-key pair (SEC1)",
			certPEM: ecdsaCert,
			keyPEM:  ecdsaKey,
			wantErr: false,
		},
		{
			name:    "valid Ed25519 cert-key pair (PKCS8)",
			certPEM: ed25519Cert,
			keyPEM:  ed25519Key,
			wantErr: false,
		},

		// ── Mismatched Pairs (same key type, different key) ──
		{
			name:        "mismatched RSA cert and RSA key",
			certPEM:     rsaCert,
			keyPEM:      rsaKey2,
			wantErr:     true,
			errContains: "RSA public key mismatch",
		},
		{
			name:        "mismatched ECDSA cert and ECDSA key",
			certPEM:     ecdsaCert,
			keyPEM:      ecdsaKey2,
			wantErr:     true,
			errContains: "ECDSA public key mismatch",
		},
		{
			name:        "mismatched Ed25519 cert and Ed25519 key",
			certPEM:     ed25519Cert,
			keyPEM:      ed25519Key2,
			wantErr:     true,
			errContains: "Ed25519 public key mismatch",
		},

		// ── Cross-Type Mismatches ──
		{
			name:        "RSA cert with ECDSA key",
			certPEM:     rsaCert,
			keyPEM:      ecdsaKey,
			wantErr:     true,
			errContains: "mismatch",
		},
		{
			name:        "ECDSA cert with RSA key",
			certPEM:     ecdsaCert,
			keyPEM:      rsaKey,
			wantErr:     true,
			errContains: "mismatch",
		},
		{
			name:        "Ed25519 cert with RSA key",
			certPEM:     ed25519Cert,
			keyPEM:      rsaKey,
			wantErr:     true,
			errContains: "mismatch",
		},
		{
			name:        "RSA cert with Ed25519 key",
			certPEM:     rsaCert,
			keyPEM:      ed25519Key,
			wantErr:     true,
			errContains: "mismatch",
		},

		// ── Invalid PEM Inputs ──
		{
			name:        "empty cert PEM",
			certPEM:     []byte{},
			keyPEM:      rsaKey,
			wantErr:     true,
			errContains: "failed to decode certificate PEM block",
		},
		{
			name:        "empty key PEM",
			certPEM:     rsaCert,
			keyPEM:      []byte{},
			wantErr:     true,
			errContains: "failed to decode private key PEM block",
		},
		{
			name:        "nil cert PEM",
			certPEM:     nil,
			keyPEM:      rsaKey,
			wantErr:     true,
			errContains: "failed to decode certificate PEM block",
		},
		{
			name:        "nil key PEM",
			certPEM:     rsaCert,
			keyPEM:      nil,
			wantErr:     true,
			errContains: "failed to decode private key PEM block",
		},
		{
			name:        "garbage cert PEM",
			certPEM:     []byte("not-a-pem-at-all"),
			keyPEM:      rsaKey,
			wantErr:     true,
			errContains: "failed to decode certificate PEM block",
		},
		{
			name:        "garbage key PEM",
			certPEM:     rsaCert,
			keyPEM:      []byte("not-a-pem-at-all"),
			wantErr:     true,
			errContains: "failed to decode private key PEM block",
		},

		// ── Malformed PEM (valid header, invalid body) ──
		{
			name: "malformed cert body",
			certPEM: pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: []byte("corrupted-cert-bytes"),
			}),
			keyPEM:      rsaKey,
			wantErr:     true,
			errContains: "failed to parse certificate",
		},
		{
			name:    "malformed key body",
			certPEM: rsaCert,
			keyPEM: pem.EncodeToMemory(&pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: []byte("corrupted-key-bytes"),
			}),
			wantErr:     true,
			errContains: "failed to parse private key",
		},

		// ── Unsupported Key Type ──
		{
			name:    "unsupported key PEM type",
			certPEM: rsaCert,
			keyPEM: pem.EncodeToMemory(&pem.Block{
				Type:  "UNKNOWN KEY TYPE",
				Bytes: []byte("some-bytes"),
			}),
			wantErr:     true,
			errContains: "unsupported private key type",
		},

		// ── Key PEM Contains Cert Instead of Key ──
		{
			name:        "cert PEM passed as key PEM",
			certPEM:     rsaCert,
			keyPEM:      rsaCert, // cert passed where key expected
			wantErr:     true,
			errContains: "unsupported private key type",
		},
		{
			name:        "key PEM passed as cert PEM",
			certPEM:     rsaKey, // key passed where cert expected
			keyPEM:      rsaKey,
			wantErr:     true,
			errContains: "failed to parse certificate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCertKeyPair(tt.certPEM, tt.keyPEM)

			if tt.wantErr {
				require.Error(t, err, "expected error but got nil")
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains,
						"error message mismatch")
				}
			} else {
				require.NoError(t, err, "expected no error but got: %v", err)
			}
		})
	}
}

// ─────────────────────────────────────────────
// PKCS1 RSA KEY FORMAT TEST (legacy format)
// ─────────────────────────────────────────────

func TestValidateCertKeyPair_PKCS1RSAKey(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(99),
		Subject:      pkix.Name{CommonName: "test-pkcs1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privKey.PublicKey, privKey)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	// PKCS1 format (legacy RSA PRIVATE KEY)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	})

	err = ValidateCertKeyPair(certPEM, keyPEM)
	assert.NoError(t, err, "PKCS1 RSA key should be valid")
}
