package validators

import (
	"bytes"
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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testWFMID       = "spiffe://example.org/margo/wfm/w1"
	testWFMClientID = "spiffe://example.org/margo/wfm/w1/client/c1"
)

// ─────────────────────────────────────────────
// TEST HELPERS
// ─────────────────────────────────────────────

type certOptions struct {
	commonName string
	spiffeID   string
	extraURIs  []string
	isCA       bool
	keyUsage   x509.KeyUsage // explicit key usage override
	notBefore  time.Time
	notAfter   time.Time
	parent     *x509.Certificate
	parentKey  *ecdsa.PrivateKey
}

// generateCert creates an ECDSA certificate and returns its DER bytes and private key.
func generateCert(t *testing.T, opts certOptions) ([]byte, *ecdsa.PrivateKey) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	if opts.commonName == "" {
		opts.commonName = "test"
	}
	if opts.notBefore.IsZero() {
		opts.notBefore = time.Now().Add(-time.Hour)
	}
	if opts.notAfter.IsZero() {
		opts.notAfter = time.Now().Add(time.Hour)
	}

	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: opts.commonName},
		NotBefore:    opts.notBefore,
		NotAfter:     opts.notAfter,
		IsCA:         opts.isCA,
	}

	if opts.isCA {
		tmpl.BasicConstraintsValid = true
		tmpl.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	}
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

func toPEM(der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func parseDER(t *testing.T, der []byte) *x509.Certificate {
	t.Helper()
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert
}

// buildTrustBundle creates a JWK Set JSON with one x509-svid key per given CA DER.
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

// testPKI is a root -> intermediate -> leaf hierarchy.
type testPKI struct {
	rootDER, interDER, leafDER []byte
}

func newTestPKI(t *testing.T) testPKI {
	t.Helper()

	rootDER, rootKey := generateCert(t, certOptions{commonName: "root", isCA: true})
	interDER, interKey := generateCert(t, certOptions{
		commonName: "intermediate",
		isCA:       true,
		parent:     parseDER(t, rootDER),
		parentKey:  rootKey,
	})
	leafDER, _ := generateCert(t, certOptions{
		commonName: "leaf",
		spiffeID:   testWFMID,
		parent:     parseDER(t, interDER),
		parentKey:  interKey,
	})
	return testPKI{rootDER: rootDER, interDER: interDER, leafDER: leafDER}
}

func selfSignedTemplate(cn string, serial int64) *x509.Certificate {
	return &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
}

// generateRSACertAndKey generates a self-signed RSA cert + PKCS#8 key PEM pair.
func generateRSACertAndKey(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tmpl := selfSignedTemplate("test-rsa", 1)
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &privKey.PublicKey, privKey)
	require.NoError(t, err)

	keyDER, err := x509.MarshalPKCS8PrivateKey(privKey)
	require.NoError(t, err)
	return toPEM(certDER), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

// generateECDSACertAndKey generates a self-signed ECDSA cert + SEC1 key PEM pair.
func generateECDSACertAndKey(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := selfSignedTemplate("test-ecdsa", 2)
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &privKey.PublicKey, privKey)
	require.NoError(t, err)

	keyDER, err := x509.MarshalECPrivateKey(privKey)
	require.NoError(t, err)
	return toPEM(certDER), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

// generateEd25519CertAndKey generates a self-signed Ed25519 cert + PKCS#8 key PEM pair.
func generateEd25519CertAndKey(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	tmpl := selfSignedTemplate("test-ed25519", 3)
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, pubKey, privKey)
	require.NoError(t, err)

	keyDER, err := x509.MarshalPKCS8PrivateKey(privKey)
	require.NoError(t, err)
	return toPEM(certDER), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

// ─────────────────────────────────────────────
// ValidateSVIDLeafConstraints
// ─────────────────────────────────────────────

func TestValidateSVIDLeafConstraints(t *testing.T) {
	tests := []struct {
		name        string
		opts        certOptions
		errContains string
	}{
		{name: "valid leaf", opts: certOptions{spiffeID: testWFMID}},
		{
			name:        "CA certificate",
			opts:        certOptions{spiffeID: testWFMID, isCA: true},
			errContains: "must not be a CA",
		},
		{
			name:        "keyCertSign set",
			opts:        certOptions{spiffeID: testWFMID, keyUsage: x509.KeyUsageCertSign},
			errContains: "keyCertSign",
		},
		{
			name:        "cRLSign set",
			opts:        certOptions{spiffeID: testWFMID, keyUsage: x509.KeyUsageCRLSign},
			errContains: "cRLSign",
		},
		{
			name:        "root path",
			opts:        certOptions{spiffeID: "spiffe://example.org/"},
			errContains: "non-root path",
		},
		{
			name:        "empty path",
			opts:        certOptions{spiffeID: "spiffe://example.org"},
			errContains: "non-root path",
		},
		{
			name:        "no URI SAN",
			opts:        certOptions{},
			errContains: "exactly one URI SAN",
		},
		{
			name: "multiple URI SANs",
			opts: certOptions{
				spiffeID:  testWFMID,
				extraURIs: []string{"spiffe://example.org/margo/wfm/w2"},
			},
			errContains: "exactly one URI SAN",
		},
		{
			name:        "non-spiffe scheme",
			opts:        certOptions{spiffeID: "https://example.org/margo/wfm/w1"},
			errContains: "scheme must be 'spiffe'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			der, _ := generateCert(t, tt.opts)
			err := ValidateSVIDLeafConstraints(parseDER(t, der))
			if tt.errContains == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

// ─────────────────────────────────────────────
// parseCertificate
// ─────────────────────────────────────────────

func TestParseCertificate_PEM(t *testing.T) {
	der, _ := generateCert(t, certOptions{spiffeID: testWFMID})
	cert, err := parseCertificate(toPEM(der))
	require.NoError(t, err)
	assert.Equal(t, der, cert.Raw)
}

func TestParseCertificate_InvalidPEMType(t *testing.T) {
	block := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("junk")})
	_, err := parseCertificate(block)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PEM block is not a certificate")
}

func TestParseCertificate_MalformedPEMBody(t *testing.T) {
	block := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("corrupted")})
	_, err := parseCertificate(block)
	require.Error(t, err)
}

func TestParseCertificate_InvalidData(t *testing.T) {
	for name, data := range map[string][]byte{
		"garbage": []byte("not a cert"),
		"empty":   {},
		"nil":     nil,
	} {
		t.Run(name, func(t *testing.T) {
			cert, err := parseCertificate(data)
			require.Error(t, err)
			assert.Nil(t, cert)
		})
	}
}

// ─────────────────────────────────────────────
// validateSPIFFEIDFormat
// ─────────────────────────────────────────────

func TestValidateSPIFFEIDFormat(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		errContains string
	}{
		{name: "valid", uri: testWFMID},
		{name: "wrong scheme", uri: "https://example.org/margo/wfm/w1", errContains: "scheme must be 'spiffe'"},
		{name: "host with port", uri: "spiffe://example.org:8080/margo/wfm/w1", errContains: "must not contain a port"},
		{name: "user info", uri: "spiffe://user@example.org/margo/wfm/w1", errContains: "must not contain user info"},
		{name: "fragment", uri: "spiffe://example.org/margo/wfm/w1#frag", errContains: "must not contain a fragment"},
		{name: "query", uri: "spiffe://example.org/margo/wfm/w1?foo=bar", errContains: "must not contain a query string"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.uri)
			require.NoError(t, err)
			err = validateSPIFFEIDFormat(u)
			if tt.errContains == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestValidateSPIFFEIDFormat_EmptyHost(t *testing.T) {
	err := validateSPIFFEIDFormat(&url.URL{Scheme: "spiffe"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trust domain (host) must not be empty")
}

// ─────────────────────────────────────────────
// validateSPIFFEID
// ─────────────────────────────────────────────

func TestValidateSPIFFEID(t *testing.T) {
	tests := []struct {
		name        string
		opts        certOptions
		principal   string
		errContains string
	}{
		{name: "valid WFM", opts: certOptions{spiffeID: testWFMID}, principal: PrincipalWFM},
		{name: "valid WFM client", opts: certOptions{spiffeID: testWFMClientID}, principal: PrincipalWFMClient},
		{name: "no URI SAN", opts: certOptions{}, principal: PrincipalWFM, errContains: "no URI SANs"},
		{
			name: "multiple URI SANs",
			opts: certOptions{
				spiffeID:  testWFMID,
				extraURIs: []string{"spiffe://example.org/margo/wfm/w2"},
			},
			principal:   PrincipalWFM,
			errContains: "2 URI SANs",
		},
		{
			name:        "invalid format (port)",
			opts:        certOptions{spiffeID: "spiffe://example.org:8080/margo/wfm/w1"},
			principal:   PrincipalWFM,
			errContains: "must not contain a port",
		},
		{
			name:        "WFM ID for WFM client principal",
			opts:        certOptions{spiffeID: testWFMID},
			principal:   PrincipalWFMClient,
			errContains: "invalid WFM-Client spiffe ID",
		},
		{
			name:        "unknown principal",
			opts:        certOptions{spiffeID: testWFMID},
			principal:   "bogus",
			errContains: "unknown principal",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			der, _ := generateCert(t, tt.opts)
			err := validateSPIFFEID(parseDER(t, der), tt.principal)
			if tt.errContains == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

// ─────────────────────────────────────────────
// ValidateX509SVID
// ─────────────────────────────────────────────

func TestValidateX509SVID_Valid(t *testing.T) {
	tests := []struct {
		name      string
		spiffeID  string
		principal string
	}{
		{"WFM", testWFMID, PrincipalWFM},
		{"WFM client", testWFMClientID, PrincipalWFMClient},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			der, _ := generateCert(t, certOptions{spiffeID: tt.spiffeID})
			got, err := ValidateX509SVID(toPEM(der), tt.principal)
			require.NoError(t, err)
			assert.Equal(t, tt.spiffeID, got)
		})
	}
}

func TestValidateX509SVID_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		opts        certOptions
		principal   string
		errContains string
	}{
		{name: "no URI SAN", opts: certOptions{}, principal: PrincipalWFM, errContains: "no URI SANs"},
		{
			name:        "wrong principal",
			opts:        certOptions{spiffeID: testWFMID},
			principal:   PrincipalWFMClient,
			errContains: "invalid WFM-Client spiffe ID",
		},
		{
			name:        "unknown principal",
			opts:        certOptions{spiffeID: testWFMID},
			principal:   "unknown-principal",
			errContains: "unknown principal",
		},
		{
			name: "expired",
			opts: certOptions{
				spiffeID:  testWFMID,
				notBefore: time.Now().Add(-2 * time.Hour),
				notAfter:  time.Now().Add(-time.Hour),
			},
			principal:   PrincipalWFM,
			errContains: "certificate has expired",
		},
		{
			name: "not yet valid",
			opts: certOptions{
				spiffeID:  testWFMID,
				notBefore: time.Now().Add(time.Hour),
				notAfter:  time.Now().Add(2 * time.Hour),
			},
			principal:   PrincipalWFM,
			errContains: "not yet valid",
		},
		{
			name:        "CA certificate",
			opts:        certOptions{spiffeID: testWFMID, isCA: true},
			principal:   PrincipalWFM,
			errContains: "must not be a CA",
		},
		{
			name:        "keyCertSign set",
			opts:        certOptions{spiffeID: testWFMID, keyUsage: x509.KeyUsageCertSign},
			principal:   PrincipalWFM,
			errContains: "keyCertSign",
		},
		{
			name:        "cRLSign set",
			opts:        certOptions{spiffeID: testWFMID, keyUsage: x509.KeyUsageCRLSign},
			principal:   PrincipalWFM,
			errContains: "cRLSign",
		},
		{
			name:        "root path",
			opts:        certOptions{spiffeID: "spiffe://example.org/"},
			principal:   PrincipalWFM,
			errContains: "spiffe ID path must not contain empty segments",
		},
		{
			name:        "empty path",
			opts:        certOptions{spiffeID: "spiffe://example.org"},
			principal:   PrincipalWFM,
			errContains: "spiffe ID must contain a path after the trust domain",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			der, _ := generateCert(t, tt.opts)
			got, err := ValidateX509SVID(toPEM(der), tt.principal)
			assert.Empty(t, got, "error path must return empty string")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestValidateX509SVID_InvalidCertBytes(t *testing.T) {
	for name, data := range map[string][]byte{
		"garbage": []byte("garbage"),
		"empty":   {},
		"nil":     nil,
		"non-certificate PEM": pem.EncodeToMemory(
			&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("junk")},
		),
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ValidateX509SVID(data, PrincipalWFM)
			assert.Empty(t, got)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid x.509 certificate")
		})
	}
}

// ─────────────────────────────────────────────
// GetTrustBundleFromJWK
// ─────────────────────────────────────────────

func TestGetTrustBundleFromJWK_Valid(t *testing.T) {
	caDER, _ := generateCert(t, certOptions{isCA: true})
	certs, err := GetTrustBundleFromJWK(buildTrustBundle(t, caDER))
	require.NoError(t, err)
	require.Len(t, certs, 1)
	assert.Equal(t, caDER, certs[0].Raw)
}

func TestGetTrustBundleFromJWK_MultipleCAs(t *testing.T) {
	ca1DER, _ := generateCert(t, certOptions{isCA: true})
	ca2DER, _ := generateCert(t, certOptions{isCA: true})
	certs, err := GetTrustBundleFromJWK(buildTrustBundle(t, ca1DER, ca2DER))
	require.NoError(t, err)
	assert.Len(t, certs, 2)
}

func TestGetTrustBundleFromJWK_MultipleX5CEntries(t *testing.T) {
	ca1DER, _ := generateCert(t, certOptions{isCA: true})
	ca2DER, _ := generateCert(t, certOptions{isCA: true})
	b, err := json.Marshal(jwkSet{Keys: []jwkKey{{
		Use: "x509-svid",
		Kty: "EC",
		X5C: []string{
			base64.StdEncoding.EncodeToString(ca1DER),
			base64.StdEncoding.EncodeToString(ca2DER),
		},
	}}})
	require.NoError(t, err)

	certs, err := GetTrustBundleFromJWK(b)
	require.NoError(t, err)
	assert.Len(t, certs, 2)
}

func TestGetTrustBundleFromJWK_IgnoresNonX509SVIDKeys(t *testing.T) {
	caDER, _ := generateCert(t, certOptions{isCA: true})
	b, err := json.Marshal(jwkSet{Keys: []jwkKey{
		{Use: "jwt-svid", Kty: "EC", X5C: []string{"aGVsbG8="}},
		{Use: "x509-svid", Kty: "EC", X5C: []string{base64.StdEncoding.EncodeToString(caDER)}},
	}})
	require.NoError(t, err)

	certs, err := GetTrustBundleFromJWK(b)
	require.NoError(t, err)
	assert.Len(t, certs, 1)
}

func TestGetTrustBundleFromJWK_Errors(t *testing.T) {
	marshal := func(keys ...jwkKey) []byte {
		b, err := json.Marshal(jwkSet{Keys: keys})
		require.NoError(t, err)
		return b
	}

	tests := []struct {
		name        string
		input       []byte
		errContains string
	}{
		{"invalid JSON", []byte("{invalid json"), "failed to parse JWK Set"},
		{"empty key set", marshal(), "no x509-svid trust anchors found"},
		{
			"only non x509-svid keys",
			marshal(jwkKey{Use: "sig", Kty: "EC", X5C: []string{"aGVsbG8="}}),
			"no x509-svid trust anchors found",
		},
		{
			"missing x5c",
			marshal(jwkKey{Use: "x509-svid", Kty: "EC"}),
			"missing x5c field",
		},
		{
			"invalid base64",
			marshal(jwkKey{Use: "x509-svid", Kty: "EC", X5C: []string{"!!!not-base64!!!"}}),
			"failed to base64 decode",
		},
		{
			"invalid certificate in x5c",
			marshal(jwkKey{
				Use: "x509-svid",
				Kty: "EC",
				X5C: []string{base64.StdEncoding.EncodeToString([]byte("not-a-cert"))},
			}),
			"failed to parse certificate",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certs, err := GetTrustBundleFromJWK(tt.input)
			assert.Nil(t, certs)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

// ─────────────────────────────────────────────
// parseCertificateChain
// ─────────────────────────────────────────────

func TestParseCertificateChain_PreservesOrder(t *testing.T) {
	der1, _ := generateCert(t, certOptions{commonName: "one", isCA: true})
	der2, _ := generateCert(t, certOptions{commonName: "two", isCA: true})

	certs, err := parseCertificateChain(bytes.Join([][]byte{toPEM(der1), toPEM(der2)}, nil))
	require.NoError(t, err)
	require.Len(t, certs, 2)
	assert.Equal(t, "one", certs[0].Subject.CommonName)
	assert.Equal(t, "two", certs[1].Subject.CommonName)
}

func TestParseCertificateChain_EmptyAndNonPEM(t *testing.T) {
	for name, data := range map[string][]byte{
		"nil":     nil,
		"empty":   {},
		"garbage": []byte("not pem"),
	} {
		t.Run(name, func(t *testing.T) {
			certs, err := parseCertificateChain(data)
			require.NoError(t, err)
			assert.Empty(t, certs)
		})
	}
}

func TestParseCertificateChain_WrongBlockType(t *testing.T) {
	der, _ := generateCert(t, certOptions{isCA: true})
	data := bytes.Join([][]byte{
		toPEM(der),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("junk")}),
	}, nil)

	certs, err := parseCertificateChain(data)
	assert.Nil(t, certs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "block 2: expected CERTIFICATE")
}

func TestParseCertificateChain_MalformedCertificate(t *testing.T) {
	data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("corrupted")})
	certs, err := parseCertificateChain(data)
	assert.Nil(t, certs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "block 1: failed to parse certificate")
}

// ─────────────────────────────────────────────
// validateAgainstTrustBundle
// ─────────────────────────────────────────────

func TestValidateAgainstTrustBundle_Valid(t *testing.T) {
	caDER, caKey := generateCert(t, certOptions{isCA: true})
	leafDER, _ := generateCert(t, certOptions{
		spiffeID:  testWFMID,
		parent:    parseDER(t, caDER),
		parentKey: caKey,
	})

	err := validateAgainstTrustBundle(parseDER(t, leafDER), buildTrustBundle(t, caDER), nil)
	assert.NoError(t, err)
}

func TestValidateAgainstTrustBundle_WithIntermediates(t *testing.T) {
	pki := newTestPKI(t)
	err := validateAgainstTrustBundle(
		parseDER(t, pki.leafDER),
		buildTrustBundle(t, pki.rootDER),
		toPEM(pki.interDER),
	)
	assert.NoError(t, err)
}

func TestValidateAgainstTrustBundle_MissingIntermediate(t *testing.T) {
	pki := newTestPKI(t)
	err := validateAgainstTrustBundle(
		parseDER(t, pki.leafDER),
		buildTrustBundle(t, pki.rootDER),
		nil,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not chain to a trusted SPIFFE bundle")
}

func TestValidateAgainstTrustBundle_InvalidIntermediateChain(t *testing.T) {
	pki := newTestPKI(t)
	err := validateAgainstTrustBundle(
		parseDER(t, pki.leafDER),
		buildTrustBundle(t, pki.rootDER),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("junk")}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse intermediate chain")
}

func TestValidateAgainstTrustBundle_CACertRejected(t *testing.T) {
	caDER, _ := generateCert(t, certOptions{isCA: true})
	err := validateAgainstTrustBundle(parseDER(t, caDER), buildTrustBundle(t, caDER), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CA certificate")
}

func TestValidateAgainstTrustBundle_UntrustedLeaf(t *testing.T) {
	caDER, _ := generateCert(t, certOptions{commonName: "trusted", isCA: true})
	otherCaDER, otherCaKey := generateCert(t, certOptions{commonName: "other", isCA: true})

	leafDER, _ := generateCert(t, certOptions{
		spiffeID:  testWFMID,
		parent:    parseDER(t, otherCaDER),
		parentKey: otherCaKey,
	})

	err := validateAgainstTrustBundle(parseDER(t, leafDER), buildTrustBundle(t, caDER), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not chain to a trusted SPIFFE bundle")
}

func TestValidateAgainstTrustBundle_EmptyBundle(t *testing.T) {
	leafDER, _ := generateCert(t, certOptions{spiffeID: testWFMID})
	b, err := json.Marshal(jwkSet{Keys: []jwkKey{}})
	require.NoError(t, err)

	err = validateAgainstTrustBundle(parseDER(t, leafDER), b, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to obtain trust bundle")
}

// ─────────────────────────────────────────────
// ValidateX509SVIDAgainstTrustBundle
// ─────────────────────────────────────────────

func TestValidateX509SVIDAgainstTrustBundle_Valid(t *testing.T) {
	caDER, caKey := generateCert(t, certOptions{isCA: true})
	leafDER, _ := generateCert(t, certOptions{
		spiffeID:  testWFMID,
		parent:    parseDER(t, caDER),
		parentKey: caKey,
	})

	ok, err := ValidateX509SVIDAgainstTrustBundle(toPEM(leafDER), nil, buildTrustBundle(t, caDER))
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestValidateX509SVIDAgainstTrustBundle_ValidWithIntermediates(t *testing.T) {
	pki := newTestPKI(t)
	ok, err := ValidateX509SVIDAgainstTrustBundle(
		toPEM(pki.leafDER),
		toPEM(pki.interDER),
		buildTrustBundle(t, pki.rootDER),
	)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestValidateX509SVIDAgainstTrustBundle_MissingIntermediates(t *testing.T) {
	pki := newTestPKI(t)
	ok, err := ValidateX509SVIDAgainstTrustBundle(
		toPEM(pki.leafDER),
		nil,
		buildTrustBundle(t, pki.rootDER),
	)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not chain to a trusted SPIFFE bundle")
}

func TestValidateX509SVIDAgainstTrustBundle_InvalidChainPEM(t *testing.T) {
	pki := newTestPKI(t)
	ok, err := ValidateX509SVIDAgainstTrustBundle(
		toPEM(pki.leafDER),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("corrupted")}),
		buildTrustBundle(t, pki.rootDER),
	)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse intermediate chain")
}

func TestValidateX509SVIDAgainstTrustBundle_InvalidCertBytes(t *testing.T) {
	caDER, _ := generateCert(t, certOptions{isCA: true})
	ok, err := ValidateX509SVIDAgainstTrustBundle([]byte("garbage"), nil, buildTrustBundle(t, caDER))
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse SVID certificate")
}

func TestValidateX509SVIDAgainstTrustBundle_UntrustedCert(t *testing.T) {
	caDER, _ := generateCert(t, certOptions{commonName: "trusted", isCA: true})
	leafDER, _ := generateCert(t, certOptions{spiffeID: testWFMID}) // self-signed

	ok, err := ValidateX509SVIDAgainstTrustBundle(toPEM(leafDER), nil, buildTrustBundle(t, caDER))
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not chain to a trusted SPIFFE bundle")
}

func TestValidateX509SVIDAgainstTrustBundle_InvalidTrustBundle(t *testing.T) {
	leafDER, _ := generateCert(t, certOptions{spiffeID: testWFMID})
	ok, err := ValidateX509SVIDAgainstTrustBundle(toPEM(leafDER), nil, []byte("not-json"))
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to obtain trust bundle")
}

func TestValidateX509SVIDAgainstTrustBundle_CACertAsLeaf(t *testing.T) {
	caDER, _ := generateCert(t, certOptions{isCA: true})
	ok, err := ValidateX509SVIDAgainstTrustBundle(toPEM(caDER), nil, buildTrustBundle(t, caDER))
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CA certificate")
}

// ─────────────────────────────────────────────
// ValidateX509SVIDChain
// ─────────────────────────────────────────────

func TestValidateX509SVIDChain_SingleIntermediate(t *testing.T) {
	pki := newTestPKI(t)
	assert.NoError(t, ValidateX509SVIDChain(toPEM(pki.leafDER), toPEM(pki.interDER)))
}

func TestValidateX509SVIDChain_MultipleIntermediates(t *testing.T) {
	rootDER, rootKey := generateCert(t, certOptions{commonName: "root", isCA: true})
	inter2DER, inter2Key := generateCert(t, certOptions{
		commonName: "inter2",
		isCA:       true,
		parent:     parseDER(t, rootDER),
		parentKey:  rootKey,
	})
	inter1DER, inter1Key := generateCert(t, certOptions{
		commonName: "inter1",
		isCA:       true,
		parent:     parseDER(t, inter2DER),
		parentKey:  inter2Key,
	})
	leafDER, _ := generateCert(t, certOptions{
		commonName: "leaf",
		spiffeID:   testWFMID,
		parent:     parseDER(t, inter1DER),
		parentKey:  inter1Key,
	})

	// Ordered from issuing intermediate up to the topmost intermediate.
	chain := bytes.Join([][]byte{toPEM(inter1DER), toPEM(inter2DER)}, nil)
	assert.NoError(t, ValidateX509SVIDChain(toPEM(leafDER), chain))
}

func TestValidateX509SVIDChain_ChainDoesNotMatch(t *testing.T) {
	pki := newTestPKI(t)
	otherDER, _ := generateCert(t, certOptions{commonName: "unrelated", isCA: true})

	err := ValidateX509SVIDChain(toPEM(pki.leafDER), toPEM(otherDER))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not chain to the provided intermediates")
}

func TestValidateX509SVIDChain_MissingIssuingIntermediate(t *testing.T) {
	pki := newTestPKI(t)
	// Root only: the leaf's issuing intermediate is absent.
	err := ValidateX509SVIDChain(toPEM(pki.leafDER), toPEM(pki.rootDER))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not chain to the provided intermediates")
}

func TestValidateX509SVIDChain_EmptyChain(t *testing.T) {
	pki := newTestPKI(t)
	for name, chain := range map[string][]byte{"nil": nil, "empty": {}, "non-PEM": []byte("junk")} {
		t.Run(name, func(t *testing.T) {
			err := ValidateX509SVIDChain(toPEM(pki.leafDER), chain)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "at least one certificate")
		})
	}
}

func TestValidateX509SVIDChain_InvalidChainPEM(t *testing.T) {
	pki := newTestPKI(t)
	tests := map[string][]byte{
		"wrong block type": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("junk")}),
		"malformed cert":   pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("corrupted")}),
	}
	for name, chain := range tests {
		t.Run(name, func(t *testing.T) {
			err := ValidateX509SVIDChain(toPEM(pki.leafDER), chain)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to parse intermediate chain")
		})
	}
}

func TestValidateX509SVIDChain_InvalidLeaf(t *testing.T) {
	pki := newTestPKI(t)
	err := ValidateX509SVIDChain([]byte("garbage"), toPEM(pki.interDER))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse SVID leaf certificate")
}

func TestValidateX509SVIDChain_LeafIsCA(t *testing.T) {
	pki := newTestPKI(t)
	err := ValidateX509SVIDChain(toPEM(pki.interDER), toPEM(pki.rootDER))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a leaf certificate")
}

// ─────────────────────────────────────────────
// ValidateCertKeyPair
// ─────────────────────────────────────────────

func TestValidateCertKeyPair(t *testing.T) {
	rsaCert, rsaKey := generateRSACertAndKey(t)
	ecdsaCert, ecdsaKey := generateECDSACertAndKey(t)
	ed25519Cert, ed25519Key := generateEd25519CertAndKey(t)

	_, rsaKey2 := generateRSACertAndKey(t)
	_, ecdsaKey2 := generateECDSACertAndKey(t)
	_, ed25519Key2 := generateEd25519CertAndKey(t)

	tests := []struct {
		name        string
		certPEM     []byte
		keyPEM      []byte
		errContains string // empty means success expected
	}{
		// Happy path
		{name: "valid RSA (PKCS8)", certPEM: rsaCert, keyPEM: rsaKey},
		{name: "valid ECDSA (SEC1)", certPEM: ecdsaCert, keyPEM: ecdsaKey},
		{name: "valid Ed25519 (PKCS8)", certPEM: ed25519Cert, keyPEM: ed25519Key},

		// Same type, different key
		{name: "mismatched RSA", certPEM: rsaCert, keyPEM: rsaKey2, errContains: "RSA public key mismatch"},
		{name: "mismatched ECDSA", certPEM: ecdsaCert, keyPEM: ecdsaKey2, errContains: "ECDSA public key mismatch"},
		{name: "mismatched Ed25519", certPEM: ed25519Cert, keyPEM: ed25519Key2, errContains: "Ed25519 public key mismatch"},

		// Cross-type mismatches
		{name: "RSA cert with ECDSA key", certPEM: rsaCert, keyPEM: ecdsaKey, errContains: "mismatch"},
		{name: "ECDSA cert with RSA key", certPEM: ecdsaCert, keyPEM: rsaKey, errContains: "mismatch"},
		{name: "Ed25519 cert with RSA key", certPEM: ed25519Cert, keyPEM: rsaKey, errContains: "mismatch"},
		{name: "RSA cert with Ed25519 key", certPEM: rsaCert, keyPEM: ed25519Key, errContains: "mismatch"},

		// Invalid PEM input
		{name: "empty cert PEM", certPEM: []byte{}, keyPEM: rsaKey, errContains: "failed to decode certificate PEM block"},
		{name: "nil cert PEM", certPEM: nil, keyPEM: rsaKey, errContains: "failed to decode certificate PEM block"},
		{name: "garbage cert PEM", certPEM: []byte("not-a-pem-at-all"), keyPEM: rsaKey, errContains: "failed to decode certificate PEM block"},
		{name: "empty key PEM", certPEM: rsaCert, keyPEM: []byte{}, errContains: "failed to decode private key PEM block"},
		{name: "nil key PEM", certPEM: rsaCert, keyPEM: nil, errContains: "failed to decode private key PEM block"},
		{name: "garbage key PEM", certPEM: rsaCert, keyPEM: []byte("not-a-pem-at-all"), errContains: "failed to decode private key PEM block"},

		// Valid PEM header, invalid body
		{
			name:        "malformed cert body",
			certPEM:     pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("corrupted-cert-bytes")}),
			keyPEM:      rsaKey,
			errContains: "failed to parse certificate",
		},
		{
			name:        "malformed PKCS8 key body",
			certPEM:     rsaCert,
			keyPEM:      pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("corrupted-key-bytes")}),
			errContains: "failed to parse private key",
		},
		{
			name:        "malformed SEC1 key body",
			certPEM:     ecdsaCert,
			keyPEM:      pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: []byte("corrupted-key-bytes")}),
			errContains: "failed to parse private key",
		},
		{
			name:        "malformed PKCS1 key body",
			certPEM:     rsaCert,
			keyPEM:      pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte("corrupted-key-bytes")}),
			errContains: "failed to parse private key",
		},

		// Wrong PEM types
		{
			name:        "unsupported key PEM type",
			certPEM:     rsaCert,
			keyPEM:      pem.EncodeToMemory(&pem.Block{Type: "UNKNOWN KEY TYPE", Bytes: []byte("some-bytes")}),
			errContains: "unsupported private key type",
		},
		{name: "cert PEM passed as key", certPEM: rsaCert, keyPEM: rsaCert, errContains: "unsupported private key type"},
		{name: "key PEM passed as cert", certPEM: rsaKey, keyPEM: rsaKey, errContains: "failed to parse certificate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCertKeyPair(tt.certPEM, tt.keyPEM)
			if tt.errContains == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestValidateCertKeyPair_PKCS1RSAKey(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tmpl := selfSignedTemplate("test-pkcs1", 99)
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &privKey.PublicKey, privKey)
	require.NoError(t, err)

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	})

	assert.NoError(t, ValidateCertKeyPair(toPEM(certDER), keyPEM))
}
