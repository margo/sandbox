package trustbundle

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
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
	certPEM []byte          // PEM-encoded CA cert (used as misRootCA)
	tlsCert tls.Certificate // used to configure httptest.NewTLSServer
	cert    *x509.Certificate
}

// newTestCA generates a self-signed CA and returns both the PEM bytes and a
// tls.Certificate so the same cert can be used to back an httptest.Server.
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

	return &testCA{
		certPEM: certPEM,
		tlsCert: tlsCert,
		cert:    cert,
	}
}

// newMISServer starts an httptest.Server that serves:
//   - GET /.well-known/margo  → discovery document
//   - GET /bundle endpoint    → SPIFFE bundle
//
// The server uses the provided testCA certificate so the MIS client can verify it.
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
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{ca.tlsCert},
	}
	server.StartTLS()
	t.Cleanup(server.Close)

	return server
}

// validSPIFFEBundle returns a minimal valid SPIFFE JWKS bundle JSON.
func validSPIFFEBundle() []byte {
	bundle := map[string]interface{}{
		"keys": []map[string]interface{}{
			{"kty": "EC", "use": "x509-svid"},
		},
	}
	b, _ := json.Marshal(bundle)
	return b
}

// discoveryDoc builds a minimal MIS discovery document JSON response.
func discoveryDoc(trustBundleURI, trustDomain string) []byte {
	doc := map[string]string{
		"trustBundleUri": trustBundleURI,
		"trustDomain":    trustDomain,
	}
	b, _ := json.Marshal(doc)
	return b
}

// ── Discovery succeeds, ETag present ─────────────────────────────────────────

func TestGetTrustBundle_DiscoverySuccess_ReturnsBundleAndDomain(t *testing.T) {
	ca := newTestCA(t)
	bundle := validSPIFFEBundle()
	const trustDomain = "margo.org"
	const expectedETag = `"abc123"`

	var server *httptest.Server

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/margo", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(discoveryDoc(
			server.URL+"/.well-known/spiffe/bundle.json",
			trustDomain,
		))
	})
	mux.HandleFunc("/.well-known/spiffe/bundle.json", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", expectedETag)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bundle)
	})

	server = httptest.NewUnstartedServer(mux)
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{ca.tlsCert},
	}
	server.StartTLS()
	t.Cleanup(server.Close)

	getter, err := New(
		server.URL,
		ca.certPEM,
		"/.well-known/spiffe/bundle.json",
		nil,
		"",
	)
	require.NoError(t, err)

	gotDomain, gotBundle, gotETag, err := getter.GetTrustBundle(context.Background())

	require.NoError(t, err)
	assert.Equal(t, trustDomain, gotDomain)
	assert.JSONEq(t, string(bundle), string(gotBundle))
	assert.Equal(t, expectedETag, gotETag)
}

// ── Discovery succeeds, ETag absent → empty string ───────────────────────────

func TestGetTrustBundle_DiscoverySuccess_NoETag_ReturnsEmptyETag(t *testing.T) {
	ca := newTestCA(t)
	bundle := validSPIFFEBundle()
	const trustDomain = "margo.org"

	var server *httptest.Server

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/margo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(discoveryDoc(
			server.URL+"/.well-known/spiffe/bundle.json",
			trustDomain,
		))
	})
	mux.HandleFunc("/.well-known/spiffe/bundle.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// No ETag header set intentionally.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bundle)
	})

	server = httptest.NewUnstartedServer(mux)
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{ca.tlsCert},
	}
	server.StartTLS()
	t.Cleanup(server.Close)

	getter, err := New(server.URL, ca.certPEM, "/.well-known/spiffe/bundle.json", nil, "")
	require.NoError(t, err)

	gotDomain, gotBundle, gotETag, err := getter.GetTrustBundle(context.Background())

	require.NoError(t, err)
	assert.Equal(t, trustDomain, gotDomain)
	assert.JSONEq(t, string(bundle), string(gotBundle))
	assert.Empty(t, gotETag)
}

// ── Bundle fetch returns non-200 → static fallback, empty ETag ───────────────

func TestGetTrustBundle_BundleFetchNon200_StaticFallback(t *testing.T) {
	ca := newTestCA(t)
	staticBundle := validSPIFFEBundle()
	const trustDomain = "margo.org"

	var server *httptest.Server

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/margo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(discoveryDoc(
			server.URL+"/.well-known/spiffe/bundle.json",
			trustDomain,
		))
	})
	mux.HandleFunc("/.well-known/spiffe/bundle.json", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	server = httptest.NewUnstartedServer(mux)
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{ca.tlsCert},
	}
	server.StartTLS()
	t.Cleanup(server.Close)

	getter, err := New(
		server.URL,
		ca.certPEM,
		"",
		staticBundle,
		trustDomain,
	)
	require.NoError(t, err)

	gotDomain, gotBundle, gotETag, err := getter.GetTrustBundle(context.Background())

	require.NoError(t, err)
	assert.Equal(t, trustDomain, gotDomain)
	assert.Equal(t, staticBundle, gotBundle)
	assert.Empty(t, gotETag) // static fallback always returns empty ETag
}

// ── Bundle fetch returns 200 with empty body → static fallback, empty ETag ───

func TestGetTrustBundle_BundleFetch200ButEmptyBody_StaticFallback(t *testing.T) {
	ca := newTestCA(t)
	staticBundle := validSPIFFEBundle()
	const trustDomain = "margo.org"

	var server *httptest.Server

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/margo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(discoveryDoc(
			server.URL+"/.well-known/spiffe/bundle.json",
			trustDomain,
		))
	})
	mux.HandleFunc("/.well-known/spiffe/bundle.json", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// no body written intentionally
	})

	server = httptest.NewUnstartedServer(mux)
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{ca.tlsCert},
	}
	server.StartTLS()
	t.Cleanup(server.Close)

	getter, err := New(
		server.URL,
		ca.certPEM,
		"",
		staticBundle,
		trustDomain,
	)
	require.NoError(t, err)

	gotDomain, gotBundle, gotETag, err := getter.GetTrustBundle(context.Background())

	require.NoError(t, err)
	assert.Equal(t, trustDomain, gotDomain)
	assert.Equal(t, staticBundle, gotBundle)
	assert.Empty(t, gotETag) // static fallback always returns empty ETag
}

// ── Discovery returns non-200 → URI fallback succeeds, ETag returned ─────────

func TestGetTrustBundle_DiscoveryNon200_URIFallbackSucceeds(t *testing.T) {
	ca := newTestCA(t)
	bundle := validSPIFFEBundle()
	const trustDomain = "margo.org"
	const expectedETag = `"fallback-etag"`

	server := newMISServer(
		t, ca,
		// Discovery returns 503.
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		},
		// Bundle handler serves correctly with ETag.
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("ETag", expectedETag)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(bundle)
		},
	)

	getter, err := New(
		server.URL,
		ca.certPEM,
		"/.well-known/spiffe/bundle.json",
		nil,
		trustDomain,
	)
	require.NoError(t, err)

	gotDomain, gotBundle, gotETag, err := getter.GetTrustBundle(context.Background())

	require.NoError(t, err)
	assert.Equal(t, trustDomain, gotDomain)
	assert.JSONEq(t, string(bundle), string(gotBundle))
	assert.Equal(t, expectedETag, gotETag)
}

// ── Discovery returns empty trustBundleUri → URI fallback succeeds ────────────

func TestGetTrustBundle_DiscoveryEmptyBundleURI_URIFallbackSucceeds(t *testing.T) {
	ca := newTestCA(t)
	bundle := validSPIFFEBundle()
	const trustDomain = "margo.org"
	const expectedETag = `"uri-fallback-etag"`

	server := newMISServer(
		t, ca,
		// Discovery returns 200 but with no trustBundleUri.
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

	getter, err := New(
		server.URL,
		ca.certPEM,
		"/.well-known/spiffe/bundle.json",
		nil,
		trustDomain,
	)
	require.NoError(t, err)

	gotDomain, gotBundle, gotETag, err := getter.GetTrustBundle(context.Background())

	require.NoError(t, err)
	assert.Equal(t, trustDomain, gotDomain)
	assert.JSONEq(t, string(bundle), string(gotBundle))
	assert.Equal(t, expectedETag, gotETag)
}

// ── All strategies fail → error propagated ───────────────────────────────────

func TestGetTrustBundle_AllStrategiesFail_ReturnsError(t *testing.T) {
	ca := newTestCA(t)

	server := newMISServer(
		t, ca,
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		},
	)

	getter, err := New(
		server.URL,
		ca.certPEM,
		"/.well-known/spiffe/bundle.json",
		nil, // no static bundle
		"",  // no trust domain
	)
	require.NoError(t, err)

	_, _, _, err = getter.GetTrustBundle(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no static trust bundle or trust domain configured")
}

// ── Context cancellation mid-request ─────────────────────────────────────────

func TestGetTrustBundle_ContextCancelledBeforeRequest_ReturnsError(t *testing.T) {
	ca := newTestCA(t)

	// Server that blocks until the test ends — context should cancel first.
	server := newMISServer(
		t, ca,
		func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		},
		func(w http.ResponseWriter, r *http.Request) {},
	)

	getter, err := New(
		server.URL,
		ca.certPEM,
		"",
		nil,
		"",
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call

	_, _, _, err = getter.GetTrustBundle(ctx)

	require.Error(t, err)
}
