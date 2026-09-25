package trustbundle

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Test Helpers ──────────────────────────────────────────────────────────────

type testCA struct {
	certPEM []byte
	tlsCert tls.Certificate
	cert    *x509.Certificate
}

func newTestCA(t *testing.T) *testCA {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)

	return &testCA{certPEM: certPEM, tlsCert: tlsCert, cert: cert}
}

func newMISServer(
	t *testing.T,
	ca *testCA,
	discoveryHandler, bundleHandler http.HandlerFunc,
) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/margo", discoveryHandler)
	mux.HandleFunc("/.well-known/spiffe/bundle.json", bundleHandler)

	server := httptest.NewUnstartedServer(mux)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{ca.tlsCert}}
	server.StartTLS()
	t.Cleanup(server.Close)

	return server
}

func validSPIFFEBundle(t *testing.T) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "spiffe-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	bundle := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "EC",
				"use": "x509-svid",
				"x5c": []string{base64.StdEncoding.EncodeToString(der)},
			},
		},
	}
	b, err := json.Marshal(bundle)
	require.NoError(t, err)
	return b
}

func discoveryDoc(trustBundleURI, trustDomain string) []byte {
	doc := map[string]string{
		"trustBundleUri": trustBundleURI,
		"trustDomain":    trustDomain,
	}
	b, _ := json.Marshal(doc)
	return b
}

// ── validateSPIFFEBundle unit tests ──────────────────────────────────────────

func TestValidateSPIFFEBundle_EmptyInput_ReturnsError(t *testing.T) {
	err := validateSPIFFEBundle(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestValidateSPIFFEBundle_InvalidJSON_ReturnsError(t *testing.T) {
	err := validateSPIFFEBundle([]byte(`not-json`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not valid JSON")
}

func TestValidateSPIFFEBundle_MissingKeysField_ReturnsError(t *testing.T) {
	// Valid JSON but "keys" field absent entirely — json.Unmarshal leaves Keys nil.
	err := validateSPIFFEBundle([]byte(`{"foo":"bar"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "\"keys\" array")
}

func TestValidateSPIFFEBundle_EmptyKeysArray_ReturnsError(t *testing.T) {
	// "keys" present but empty — zero X.509 trust anchors.
	err := validateSPIFFEBundle([]byte(`{"keys":[]}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no X.509 trust anchors")
}

func TestValidateSPIFFEBundle_KeysWithoutX5c_ReturnsError(t *testing.T) {
	// JWK entries exist but none carry an x5c field.
	bundle := []byte(`{"keys":[{"kty":"EC","use":"x509-svid"}]}`)
	err := validateSPIFFEBundle(bundle)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no X.509 trust anchors")
}

func TestValidateSPIFFEBundle_InvalidBase64InX5c_ReturnsError(t *testing.T) {
	bundle := []byte(`{"keys":[{"kty":"EC","x5c":["!!!not-base64!!!"]}]}`)
	err := validateSPIFFEBundle(bundle)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "base64 decode failed")
}

func TestValidateSPIFFEBundle_ValidBase64ButNotCert_ReturnsError(t *testing.T) {
	// Valid base64 but decoded bytes are not a DER-encoded certificate.
	garbage := base64.StdEncoding.EncodeToString([]byte("this is not a certificate"))
	bundle, err := json.Marshal(map[string]interface{}{
		"keys": []map[string]interface{}{
			{"kty": "EC", "x5c": []string{garbage}},
		},
	})
	require.NoError(t, err)

	err = validateSPIFFEBundle(bundle)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid X.509 certificate")
}

func TestValidateSPIFFEBundle_ValidBundle_ReturnsNil(t *testing.T) {
	require.NoError(t, validateSPIFFEBundle(validSPIFFEBundle(t)))
}

func TestValidateSPIFFEBundle_MultipleKeys_OnlyOneWithX5c_Passes(t *testing.T) {
	// Mix: one JWT key (no x5c), one X.509 anchor (x5c present) — should pass.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "anchor"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	bundle, err := json.Marshal(map[string]interface{}{
		"keys": []map[string]interface{}{
			{"kty": "EC", "use": "jwt-svid"}, // no x5c
			{
				"kty": "EC",
				"use": "x509-svid",
				"x5c": []string{base64.StdEncoding.EncodeToString(der)},
			},
		},
	})
	require.NoError(t, err)

	require.NoError(t, validateSPIFFEBundle(bundle))
}

func TestValidateSPIFFEBundle_SecondCertInX5cInvalid_ReturnsIndexedError(t *testing.T) {
	// First x5c entry valid, second is garbage — error must name keys[0].x5c[1].
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(3),
		Subject:               pkix.Name{CommonName: "anchor"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	bundle, err := json.Marshal(map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "EC",
				"x5c": []string{
					base64.StdEncoding.EncodeToString(der),
					base64.StdEncoding.EncodeToString([]byte("garbage")),
				},
			},
		},
	})
	require.NoError(t, err)

	err = validateSPIFFEBundle(bundle)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "keys[0].x5c[1]")
	assert.Contains(t, err.Error(), "not a valid X.509 certificate")
}

// ── Discovery succeeds, ETag present ─────────────────────────────────────────

func TestGetTrustBundle_DiscoverySuccess_ReturnsBundleAndDomain(t *testing.T) {
	ca := newTestCA(t)
	bundle := validSPIFFEBundle(t)
	const trustDomain = "margo.org"
	const expectedETag = `"abc123"`

	var server *httptest.Server

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/margo", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(discoveryDoc(server.URL+"/.well-known/spiffe/bundle.json", trustDomain))
	})
	mux.HandleFunc("/.well-known/spiffe/bundle.json", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", expectedETag)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bundle)
	})

	server = httptest.NewUnstartedServer(mux)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{ca.tlsCert}}
	server.StartTLS()
	t.Cleanup(server.Close)

	getter, err := New(server.URL, ca.certPEM, "/.well-known/spiffe/bundle.json", nil, "")
	require.NoError(t, err)

	gotDomain, gotBundle, gotETag, err := getter.GetTrustBundle(context.Background(), "")

	require.NoError(t, err)
	assert.Equal(t, trustDomain, gotDomain)
	assert.JSONEq(t, string(bundle), string(gotBundle))
	assert.Equal(t, expectedETag, gotETag)
}

// ── Discovery succeeds, ETag absent → empty string ───────────────────────────

func TestGetTrustBundle_DiscoverySuccess_NoETag_ReturnsEmptyETag(t *testing.T) {
	ca := newTestCA(t)
	bundle := validSPIFFEBundle(t)
	const trustDomain = "margo.org"

	var server *httptest.Server

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/margo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(discoveryDoc(server.URL+"/.well-known/spiffe/bundle.json", trustDomain))
	})
	mux.HandleFunc("/.well-known/spiffe/bundle.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// No ETag header set intentionally.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bundle)
	})

	server = httptest.NewUnstartedServer(mux)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{ca.tlsCert}}
	server.StartTLS()
	t.Cleanup(server.Close)

	getter, err := New(server.URL, ca.certPEM, "/.well-known/spiffe/bundle.json", nil, "")
	require.NoError(t, err)

	gotDomain, gotBundle, gotETag, err := getter.GetTrustBundle(context.Background(), "")

	require.NoError(t, err)
	assert.Equal(t, trustDomain, gotDomain)
	assert.JSONEq(t, string(bundle), string(gotBundle))
	assert.Empty(t, gotETag)
}

// ── Bundle fetch returns non-200 → static fallback, empty ETag ───────────────

func TestGetTrustBundle_BundleFetchNon200_StaticFallback(t *testing.T) {
	ca := newTestCA(t)
	staticBundle := validSPIFFEBundle(t)
	const trustDomain = "margo.org"

	var server *httptest.Server

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/margo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(discoveryDoc(server.URL+"/.well-known/spiffe/bundle.json", trustDomain))
	})
	mux.HandleFunc("/.well-known/spiffe/bundle.json", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	server = httptest.NewUnstartedServer(mux)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{ca.tlsCert}}
	server.StartTLS()
	t.Cleanup(server.Close)

	getter, err := New(server.URL, ca.certPEM, "", staticBundle, trustDomain)
	require.NoError(t, err)

	gotDomain, gotBundle, gotETag, err := getter.GetTrustBundle(context.Background(), "")

	require.NoError(t, err)
	assert.Equal(t, trustDomain, gotDomain)
	assert.Equal(t, staticBundle, gotBundle)
	assert.Empty(t, gotETag)
}

// ── Bundle fetch returns 200 with empty body → static fallback, empty ETag ───

func TestGetTrustBundle_BundleFetch200ButEmptyBody_StaticFallback(t *testing.T) {
	ca := newTestCA(t)
	staticBundle := validSPIFFEBundle(t)
	const trustDomain = "margo.org"

	var server *httptest.Server

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/margo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(discoveryDoc(server.URL+"/.well-known/spiffe/bundle.json", trustDomain))
	})
	mux.HandleFunc("/.well-known/spiffe/bundle.json", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// no body written intentionally
	})

	server = httptest.NewUnstartedServer(mux)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{ca.tlsCert}}
	server.StartTLS()
	t.Cleanup(server.Close)

	getter, err := New(server.URL, ca.certPEM, "", staticBundle, trustDomain)
	require.NoError(t, err)

	gotDomain, gotBundle, gotETag, err := getter.GetTrustBundle(context.Background(), "")

	require.NoError(t, err)
	assert.Equal(t, trustDomain, gotDomain)
	assert.Equal(t, staticBundle, gotBundle)
	assert.Empty(t, gotETag)
}

// ── Discovery returns non-200 → URI fallback succeeds, ETag returned ─────────

func TestGetTrustBundle_DiscoveryNon200_URIFallbackSucceeds(t *testing.T) {
	ca := newTestCA(t)
	bundle := validSPIFFEBundle(t)
	const trustDomain = "margo.org"
	const expectedETag = `"fallback-etag"`

	server := newMISServer(
		t, ca,
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("ETag", expectedETag)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(bundle)
		},
	)

	getter, err := New(server.URL, ca.certPEM, "/.well-known/spiffe/bundle.json", nil, trustDomain)
	require.NoError(t, err)

	gotDomain, gotBundle, gotETag, err := getter.GetTrustBundle(context.Background(), "")

	require.NoError(t, err)
	assert.Equal(t, trustDomain, gotDomain)
	assert.JSONEq(t, string(bundle), string(gotBundle))
	assert.Equal(t, expectedETag, gotETag)
}

// ── Discovery returns empty trustBundleUri → URI fallback succeeds ────────────

func TestGetTrustBundle_DiscoveryEmptyBundleURI_URIFallbackSucceeds(t *testing.T) {
	ca := newTestCA(t)
	bundle := validSPIFFEBundle(t)
	const trustDomain = "margo.org"
	const expectedETag = `"uri-fallback-etag"`

	server := newMISServer(
		t, ca,
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(discoveryDoc("", trustDomain))
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("ETag", expectedETag)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(bundle)
		},
	)

	getter, err := New(server.URL, ca.certPEM, "/.well-known/spiffe/bundle.json", nil, trustDomain)
	require.NoError(t, err)

	gotDomain, gotBundle, gotETag, err := getter.GetTrustBundle(context.Background(), "")

	require.NoError(t, err)
	assert.Equal(t, trustDomain, gotDomain)
	assert.JSONEq(t, string(bundle), string(gotBundle))
	assert.Equal(t, expectedETag, gotETag)
}

// ── All strategies fail → error propagated ───────────────────────────────────

func TestGetTrustBundle_AllStrategiesFail_ReturnsError(t *testing.T) {
	ca := newTestCA(t)

	server := newMISServer(
		t,
		ca,
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) },
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) },
	)

	getter, err := New(server.URL, ca.certPEM, "/.well-known/spiffe/bundle.json", nil, "")
	require.NoError(t, err)

	_, _, _, err = getter.GetTrustBundle(context.Background(), "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no static trust bundle or trust domain configured")
}

// ── Context cancellation mid-request ─────────────────────────────────────────

func TestGetTrustBundle_ContextCancelledBeforeRequest_ReturnsError(t *testing.T) {
	ca := newTestCA(t)

	server := newMISServer(
		t, ca,
		func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() },
		func(w http.ResponseWriter, r *http.Request) {},
	)

	getter, err := New(server.URL, ca.certPEM, "", nil, "")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call

	_, _, _, err = getter.GetTrustBundle(ctx, "")

	require.Error(t, err)
}

// ── ETag match via discovery → ErrNotModified, no static fallback ────────────

func TestGetTrustBundle_DiscoveryBundle304_ReturnsErrNotModified(t *testing.T) {
	ca := newTestCA(t)
	const trustDomain = "margo.org"
	const cachedETag = `"abc123"`

	var server *httptest.Server

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/margo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(discoveryDoc(server.URL+"/.well-known/spiffe/bundle.json", trustDomain))
	})
	mux.HandleFunc("/.well-known/spiffe/bundle.json", func(w http.ResponseWriter, r *http.Request) {
		// Simulate server honouring If-None-Match.
		assert.Equal(t, cachedETag, r.Header.Get("If-None-Match"))
		w.WriteHeader(http.StatusNotModified)
	})

	server = httptest.NewUnstartedServer(mux)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{ca.tlsCert}}
	server.StartTLS()
	t.Cleanup(server.Close)

	getter, err := New(
		server.URL,
		ca.certPEM,
		"/.well-known/spiffe/bundle.json",
		validSPIFFEBundle(t), // static bundle present — must NOT be used on 304
		trustDomain,
	)
	require.NoError(t, err)

	gotDomain, gotBundle, gotETag, err := getter.GetTrustBundle(context.Background(), cachedETag)

	require.ErrorIs(t, err, ErrNotModified)
	assert.Equal(t, trustDomain, gotDomain)
	assert.Nil(t, gotBundle)
	assert.Empty(t, gotETag)
}

// ── ETag match via URI fallback → ErrNotModified ─────────────────────────────

func TestGetTrustBundle_URIFallbackBundle304_ReturnsErrNotModified(t *testing.T) {
	ca := newTestCA(t)
	const trustDomain = "margo.org"
	const cachedETag = `"fallback-etag"`

	server := newMISServer(
		t,
		ca,
		// Discovery fails → triggers URI fallback.
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) },
		// URI fallback returns 304.
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotModified) },
	)

	getter, err := New(
		server.URL,
		ca.certPEM,
		"/.well-known/spiffe/bundle.json",
		validSPIFFEBundle(t), // static bundle present — must NOT be used on 304
		trustDomain,
	)
	require.NoError(t, err)

	gotDomain, gotBundle, gotETag, err := getter.GetTrustBundle(context.Background(), cachedETag)

	require.ErrorIs(t, err, ErrNotModified)
	assert.Equal(t, trustDomain, gotDomain)
	assert.Nil(t, gotBundle)
	assert.Empty(t, gotETag)
}
