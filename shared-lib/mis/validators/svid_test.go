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

// ── helpers ───────────────────────────────────────────────────────────────────

func generateKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return key
}

func newCACert(t *testing.T, key *ecdsa.PrivateKey) *x509.Certificate {
	t.Helper()
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert
}

// leafCertOptions holds configurable fields for newLeafCert.
type leafCertOptions struct {
	spiffeID  string
	notBefore time.Time
	notAfter  time.Time
	extraURIs []*url.URL
	clearURIs bool
}

func newLeafCert(
	t *testing.T,
	caKey *ecdsa.PrivateKey,
	caCert *x509.Certificate,
	opts leafCertOptions,
) *x509.Certificate {
	t.Helper()
	leafKey := generateKey(t)

	notBefore := opts.notBefore
	if notBefore.IsZero() {
		notBefore = time.Now().Add(-time.Minute)
	}
	notAfter := opts.notAfter
	if notAfter.IsZero() {
		notAfter = time.Now().Add(time.Hour)
	}

	var uris []*url.URL
	if !opts.clearURIs && opts.spiffeID != "" {
		u, err := url.Parse(opts.spiffeID)
		require.NoError(t, err)
		uris = append(uris, u)
	}
	uris = append(uris, opts.extraURIs...)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test-leaf"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		URIs:         uris,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &leafKey.PublicKey, caKey)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert
}

func certToPEM(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
}

func buildJWKSet(t *testing.T, certs ...*x509.Certificate) []byte {
	t.Helper()
	var keys []jwkKey
	for _, c := range certs {
		keys = append(keys, jwkKey{
			Use: "x509-svid",
			Kty: "EC",
			X5C: []string{base64.StdEncoding.EncodeToString(c.Raw)},
		})
	}
	data, err := json.Marshal(jwkSet{Keys: keys})
	require.NoError(t, err)
	return data
}

// ── getTrustBundleFromJWK ─────────────────────────────────────────────────────

func TestGetTrustBundleFromJWK(t *testing.T) {
	caKey := generateKey(t)
	caCert := newCACert(t, caKey)

	t.Run("valid single x509-svid key", func(t *testing.T) {
		jwk := buildJWKSet(t, caCert)
		certs, err := getTrustBundleFromJWK(jwk)
		require.NoError(t, err)
		require.Len(t, certs, 1)
		assert.Equal(t, caCert.Raw, certs[0].Raw)
	})

	t.Run("skips non-x509-svid keys", func(t *testing.T) {
		set := jwkSet{Keys: []jwkKey{
			{Use: "jwt-svid", Kty: "EC", X5C: []string{}},
			{
				Use: "x509-svid",
				Kty: "EC",
				X5C: []string{base64.StdEncoding.EncodeToString(caCert.Raw)},
			},
		}}
		data, err := json.Marshal(set)
		require.NoError(t, err)
		certs, err := getTrustBundleFromJWK(data)
		require.NoError(t, err)
		assert.Len(t, certs, 1)
	})

	t.Run("multiple x509-svid keys returns all certs", func(t *testing.T) {
		caKey2 := generateKey(t)
		caCert2 := newCACert(t, caKey2)
		jwk := buildJWKSet(t, caCert, caCert2)
		certs, err := getTrustBundleFromJWK(jwk)
		require.NoError(t, err)
		assert.Len(t, certs, 2)
	})

	t.Run("error when x509-svid key has empty x5c", func(t *testing.T) {
		set := jwkSet{Keys: []jwkKey{
			{Use: "x509-svid", Kty: "EC", X5C: []string{}},
		}}
		data, err := json.Marshal(set)
		require.NoError(t, err)
		_, err = getTrustBundleFromJWK(data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing x5c field")
	})

	t.Run("error on invalid base64 in x5c", func(t *testing.T) {
		set := jwkSet{Keys: []jwkKey{
			{Use: "x509-svid", Kty: "EC", X5C: []string{"!!!not-base64!!!"}},
		}}
		data, err := json.Marshal(set)
		require.NoError(t, err)
		_, err = getTrustBundleFromJWK(data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to base64 decode")
	})

	t.Run("error on invalid DER in x5c", func(t *testing.T) {
		set := jwkSet{Keys: []jwkKey{
			{
				Use: "x509-svid",
				Kty: "EC",
				X5C: []string{base64.StdEncoding.EncodeToString([]byte("not-a-cert"))},
			},
		}}
		data, err := json.Marshal(set)
		require.NoError(t, err)
		_, err = getTrustBundleFromJWK(data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse certificate")
	})

	t.Run("error when no x509-svid keys present", func(t *testing.T) {
		set := jwkSet{Keys: []jwkKey{
			{Use: "jwt-svid", Kty: "EC", X5C: []string{}},
		}}
		data, err := json.Marshal(set)
		require.NoError(t, err)
		_, err = getTrustBundleFromJWK(data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no x509-svid trust anchors found")
	})

	t.Run("error on malformed JSON", func(t *testing.T) {
		_, err := getTrustBundleFromJWK([]byte("{invalid json"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse JWK Set")
	})
}

// ── parseCertificate ──────────────────────────────────────────────────────────

func TestParseCertificate(t *testing.T) {
	caKey := generateKey(t)
	caCert := newCACert(t, caKey)

	t.Run("parses PEM encoded certificate", func(t *testing.T) {
		cert, err := parseCertificate(certToPEM(caCert))
		require.NoError(t, err)
		assert.Equal(t, caCert.Raw, cert.Raw)
	})

	t.Run("parses DER encoded certificate", func(t *testing.T) {
		cert, err := parseCertificate(caCert.Raw)
		require.NoError(t, err)
		assert.Equal(t, caCert.Raw, cert.Raw)
	})

	t.Run("error for PEM block with wrong type", func(t *testing.T) {
		wrongPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("data")})
		_, err := parseCertificate(wrongPEM)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "PEM block is not a certificate")
	})

	t.Run("error for invalid DER data", func(t *testing.T) {
		_, err := parseCertificate([]byte("not a cert"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse as PEM or DER")
	})
}

// ── validateSPIFFEIDFormat ────────────────────────────────────────────────────

func TestValidateSPIFFEIDFormat(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr string
	}{
		{
			name: "valid SPIFFE ID with path",
			uri:  "spiffe://example.org/ns/default/sa/myapp",
		},
		{
			name: "valid SPIFFE ID without path",
			uri:  "spiffe://example.org",
		},
		{
			name:    "wrong scheme http",
			uri:     "https://example.org/path",
			wantErr: "scheme must be 'spiffe'",
		},
		{
			name:    "empty host",
			uri:     "spiffe:///path",
			wantErr: "trust domain (host) must not be empty",
		},
		{
			name:    "host with port",
			uri:     "spiffe://example.org:8080/path",
			wantErr: "trust domain must not contain a port",
		},
		{
			name:    "with user info",
			uri:     "spiffe://user@example.org/path",
			wantErr: "must not contain user info",
		},
		{
			name:    "with fragment",
			uri:     "spiffe://example.org/path#frag",
			wantErr: "must not contain a fragment",
		},
		{
			name:    "with query string",
			uri:     "spiffe://example.org/path?q=1",
			wantErr: "must not contain a query string",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, err := url.Parse(tc.uri)
			require.NoError(t, err)

			err = validateSPIFFEIDFormat(u)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

// ── validateSPIFFEID ──────────────────────────────────────────────────────────

func TestValidateSPIFFEID(t *testing.T) {
	caKey := generateKey(t)
	caCert := newCACert(t, caKey)

	t.Run("valid single SPIFFE URI SAN", func(t *testing.T) {
		leaf := newLeafCert(t, caKey, caCert, leafCertOptions{
			spiffeID: "spiffe://example.org/svc",
		})
		assert.NoError(t, validateSPIFFEID(leaf))
	})

	t.Run("no URI SANs", func(t *testing.T) {
		leaf := newLeafCert(t, caKey, caCert, leafCertOptions{
			clearURIs: true,
		})
		err := validateSPIFFEID(leaf)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no URI SANs")
	})

	t.Run("multiple URI SANs", func(t *testing.T) {
		extra, err := url.Parse("spiffe://other.org/svc")
		require.NoError(t, err)
		leaf := newLeafCert(t, caKey, caCert, leafCertOptions{
			spiffeID:  "spiffe://example.org/svc",
			extraURIs: []*url.URL{extra},
		})
		err = validateSPIFFEID(leaf)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exactly one SPIFFE ID is required")
	})

	t.Run("invalid SPIFFE ID format", func(t *testing.T) {
		leaf := newLeafCert(t, caKey, caCert, leafCertOptions{
			spiffeID: "https://example.org/svc",
		})
		err := validateSPIFFEID(leaf)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid SPIFFE ID")
	})
}

// ── validateAgainstTrustBundle ────────────────────────────────────────────────

func TestValidateAgainstTrustBundle(t *testing.T) {
	caKey := generateKey(t)
	caCert := newCACert(t, caKey)
	jwk := buildJWKSet(t, caCert)

	t.Run("valid leaf certificate chains to trust bundle", func(t *testing.T) {
		leaf := newLeafCert(t, caKey, caCert, leafCertOptions{
			spiffeID: "spiffe://example.org/svc",
		})
		assert.NoError(t, validateAgainstTrustBundle(leaf, jwk))
	})

	t.Run("CA certificate rejected as SVID", func(t *testing.T) {
		err := validateAgainstTrustBundle(caCert, jwk)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CA certificate")
	})

	t.Run("certificate signed by unknown CA", func(t *testing.T) {
		otherKey := generateKey(t)
		otherCA := newCACert(t, otherKey)
		leaf := newLeafCert(t, otherKey, otherCA, leafCertOptions{
			spiffeID: "spiffe://example.org/svc",
		})
		err := validateAgainstTrustBundle(leaf, jwk)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not chain to a trusted SPIFFE bundle")
	})

	t.Run("malformed trust bundle", func(t *testing.T) {
		leaf := newLeafCert(t, caKey, caCert, leafCertOptions{
			spiffeID: "spiffe://example.org/svc",
		})
		err := validateAgainstTrustBundle(leaf, []byte("bad json"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to obtain trust bundle")
	})

	t.Run("empty trust bundle JSON", func(t *testing.T) {
		leaf := newLeafCert(t, caKey, caCert, leafCertOptions{
			spiffeID: "spiffe://example.org/svc",
		})
		emptySet, err := json.Marshal(
			jwkSet{Keys: []jwkKey{{Use: "jwt-svid", Kty: "EC", X5C: []string{}}}},
		)
		require.NoError(t, err)
		err = validateAgainstTrustBundle(leaf, emptySet)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to obtain trust bundle")
	})
}

// ── ValidateX509SVID (no trust bundle) ───────────────────────────

func TestValidateX509SVID(t *testing.T) {
	caKey := generateKey(t)
	caCert := newCACert(t, caKey)

	t.Run("valid PEM SVID", func(t *testing.T) {
		leaf := newLeafCert(t, caKey, caCert, leafCertOptions{
			spiffeID: "spiffe://example.org/svc",
		})
		ok, err := ValidateX509SVID(certToPEM(leaf))
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("valid DER SVID", func(t *testing.T) {
		leaf := newLeafCert(t, caKey, caCert, leafCertOptions{
			spiffeID: "spiffe://example.org/svc",
		})
		ok, err := ValidateX509SVID(leaf.Raw)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("invalid certificate bytes", func(t *testing.T) {
		ok, err := ValidateX509SVID([]byte("garbage"))
		assert.False(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid x.509 certificate")
	})

	t.Run("expired certificate", func(t *testing.T) {
		leaf := newLeafCert(t, caKey, caCert, leafCertOptions{
			spiffeID:  "spiffe://example.org/svc",
			notBefore: time.Now().Add(-2 * time.Hour),
			notAfter:  time.Now().Add(-time.Hour),
		})
		ok, err := ValidateX509SVID(certToPEM(leaf))
		assert.False(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "certificate has expired")
	})

	t.Run("not yet valid certificate", func(t *testing.T) {
		leaf := newLeafCert(t, caKey, caCert, leafCertOptions{
			spiffeID:  "spiffe://example.org/svc",
			notBefore: time.Now().Add(time.Hour),
			notAfter:  time.Now().Add(2 * time.Hour),
		})
		ok, err := ValidateX509SVID(certToPEM(leaf))
		assert.False(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not yet valid")
	})

	t.Run("missing SPIFFE ID", func(t *testing.T) {
		leaf := newLeafCert(t, caKey, caCert, leafCertOptions{
			clearURIs: true,
		})
		ok, err := ValidateX509SVID(certToPEM(leaf))
		assert.False(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no URI SANs")
	})

	// NOTE: "untrusted certificate" and "malformed trust bundle" cases are removed —
	// ValidateX509SVID no longer checks trust bundle. See TestValidateX509SVIDAgainstTrustBundle.
}

// ── ValidateX509SVIDAgainstTrustBundle ───────────────────────────────────────

func TestValidateX509SVIDAgainstTrustBundle(t *testing.T) {
	caKey := generateKey(t)
	caCert := newCACert(t, caKey)
	jwk := buildJWKSet(t, caCert)

	// ── Happy path ────────────────────────────────────────────────────────────

	t.Run("valid PEM SVID chains to trust bundle", func(t *testing.T) {
		leaf := newLeafCert(t, caKey, caCert, leafCertOptions{
			spiffeID: "spiffe://example.org/svc",
		})
		ok, err := ValidateX509SVIDAgainstTrustBundle(certToPEM(leaf), jwk)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("valid DER SVID chains to trust bundle", func(t *testing.T) {
		leaf := newLeafCert(t, caKey, caCert, leafCertOptions{
			spiffeID: "spiffe://example.org/svc",
		})
		ok, err := ValidateX509SVIDAgainstTrustBundle(leaf.Raw, jwk)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("valid SVID with multiple CAs in trust bundle", func(t *testing.T) {
		// Add a second unrelated CA to the bundle — cert should still validate
		otherKey := generateKey(t)
		otherCA := newCACert(t, otherKey)
		multiJWK := buildJWKSet(t, caCert, otherCA)

		leaf := newLeafCert(t, caKey, caCert, leafCertOptions{
			spiffeID: "spiffe://example.org/svc",
		})
		ok, err := ValidateX509SVIDAgainstTrustBundle(certToPEM(leaf), multiJWK)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	// ── Invalid SVID bytes ────────────────────────────────────────────────────

	t.Run("invalid certificate bytes", func(t *testing.T) {
		ok, err := ValidateX509SVIDAgainstTrustBundle([]byte("garbage"), jwk)
		assert.False(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse SVID certificate")
	})

	t.Run("empty certificate bytes", func(t *testing.T) {
		ok, err := ValidateX509SVIDAgainstTrustBundle([]byte{}, jwk)
		assert.False(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse SVID certificate")
	})

	t.Run("PEM block with wrong type", func(t *testing.T) {
		wrongPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("data")})
		ok, err := ValidateX509SVIDAgainstTrustBundle(wrongPEM, jwk)
		assert.False(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse SVID certificate")
	})

	// ── Chain validation failures ─────────────────────────────────────────────

	t.Run("certificate signed by unknown CA", func(t *testing.T) {
		otherKey := generateKey(t)
		otherCA := newCACert(t, otherKey)
		leaf := newLeafCert(t, otherKey, otherCA, leafCertOptions{
			spiffeID: "spiffe://example.org/svc",
		})
		ok, err := ValidateX509SVIDAgainstTrustBundle(certToPEM(leaf), jwk)
		assert.False(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not chain to a trusted SPIFFE bundle")
	})

	t.Run("CA certificate rejected as SVID", func(t *testing.T) {
		// CA certs must not be accepted as SVIDs
		ok, err := ValidateX509SVIDAgainstTrustBundle(certToPEM(caCert), jwk)
		assert.False(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CA certificate")
	})

	t.Run("self-signed leaf not in trust bundle", func(t *testing.T) {
		// Leaf signed by its own key — not in any trust bundle
		selfKey := generateKey(t)
		selfCA := newCACert(t, selfKey)
		leaf := newLeafCert(t, selfKey, selfCA, leafCertOptions{
			spiffeID: "spiffe://example.org/svc",
		})
		ok, err := ValidateX509SVIDAgainstTrustBundle(certToPEM(leaf), jwk)
		assert.False(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not chain to a trusted SPIFFE bundle")
	})

	// ── Trust bundle failures ─────────────────────────────────────────────────

	t.Run("malformed trust bundle JSON", func(t *testing.T) {
		leaf := newLeafCert(t, caKey, caCert, leafCertOptions{
			spiffeID: "spiffe://example.org/svc",
		})
		ok, err := ValidateX509SVIDAgainstTrustBundle(certToPEM(leaf), []byte("{bad json"))
		assert.False(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to obtain trust bundle")
	})

	t.Run("empty trust bundle — no x509-svid keys", func(t *testing.T) {
		leaf := newLeafCert(t, caKey, caCert, leafCertOptions{
			spiffeID: "spiffe://example.org/svc",
		})
		emptyBundle, err := json.Marshal(jwkSet{Keys: []jwkKey{
			{Use: "jwt-svid", Kty: "EC", X5C: []string{}},
		}})
		require.NoError(t, err)
		ok, err := ValidateX509SVIDAgainstTrustBundle(certToPEM(leaf), emptyBundle)
		assert.False(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to obtain trust bundle")
	})

	t.Run("empty trust bundle bytes", func(t *testing.T) {
		leaf := newLeafCert(t, caKey, caCert, leafCertOptions{
			spiffeID: "spiffe://example.org/svc",
		})
		ok, err := ValidateX509SVIDAgainstTrustBundle(certToPEM(leaf), []byte{})
		assert.False(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to obtain trust bundle")
	})

	t.Run("trust bundle with invalid base64 in x5c", func(t *testing.T) {
		leaf := newLeafCert(t, caKey, caCert, leafCertOptions{
			spiffeID: "spiffe://example.org/svc",
		})
		badBundle, err := json.Marshal(jwkSet{Keys: []jwkKey{
			{Use: "x509-svid", Kty: "EC", X5C: []string{"!!!not-base64!!!"}},
		}})
		require.NoError(t, err)
		ok, err := ValidateX509SVIDAgainstTrustBundle(certToPEM(leaf), badBundle)
		assert.False(t, ok)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to obtain trust bundle")
	})
}
