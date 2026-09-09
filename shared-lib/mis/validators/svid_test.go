package validators

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Test Helpers ──────────────────────────────────────────────────────────────

type certOptions struct {
	spiffeID  string   // URI SAN; empty = no URI SAN
	extraURIs []string // additional URI SANs (to trigger multi-SAN errors)
	isCA      bool
	notBefore time.Time
	notAfter  time.Time
	parent    *x509.Certificate // nil = self-signed
	parentKey *ecdsa.PrivateKey
}

// generateCert creates a certificate according to opts and returns (certDER, key).
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
	certs, err := getTrustBundleFromJWK(bundle)
	require.NoError(t, err)
	assert.Len(t, certs, 1)
}

func TestGetTrustBundleFromJWK_SkipsNonX509SVID(t *testing.T) {
	// A key with use != "x509-svid" should be skipped
	keys := []jwkKey{{Use: "sig", Kty: "EC", X5C: []string{"aGVsbG8="} /* "hello" */}}
	b, _ := json.Marshal(jwkSet{Keys: keys})
	_, err := getTrustBundleFromJWK(b)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no x509-svid trust anchors found")
}

func TestGetTrustBundleFromJWK_MissingX5C(t *testing.T) {
	keys := []jwkKey{{Use: "x509-svid", Kty: "EC"}} // no X5C
	b, _ := json.Marshal(jwkSet{Keys: keys})
	_, err := getTrustBundleFromJWK(b)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing x5c field")
}

func TestGetTrustBundleFromJWK_InvalidBase64(t *testing.T) {
	keys := []jwkKey{{Use: "x509-svid", Kty: "EC", X5C: []string{"!!!not-base64!!!"}}}
	b, _ := json.Marshal(jwkSet{Keys: keys})
	_, err := getTrustBundleFromJWK(b)
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
	_, err := getTrustBundleFromJWK(b)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse certificate")
}

func TestGetTrustBundleFromJWK_InvalidJSON(t *testing.T) {
	_, err := getTrustBundleFromJWK([]byte("{invalid json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse JWK Set")
}

func TestGetTrustBundleFromJWK_EmptyKeySet(t *testing.T) {
	b, _ := json.Marshal(jwkSet{Keys: []jwkKey{}})
	_, err := getTrustBundleFromJWK(b)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no x509-svid trust anchors found")
}

func TestGetTrustBundleFromJWK_MultipleCAs(t *testing.T) {
	ca1DER, _ := generateCert(t, certOptions{isCA: true})
	ca2DER, _ := generateCert(t, certOptions{isCA: true})
	bundle := buildTrustBundle(t, ca1DER, ca2DER)
	certs, err := getTrustBundleFromJWK(bundle)
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
