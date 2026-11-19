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
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/margo/sandbox/shared-lib/mis/client"
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

// ── staticFallback ────────────────────────────────────────────────────────────

func TestStaticFallback_NoBundleNoTrustDomain_ReturnsError(t *testing.T) {
	impl := &trustBundleGetterImpl{}
	_, _, _, err := impl.staticFallback(fmt.Errorf("preceding error"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no static trust bundle or trust domain configured as fallback")
}

func TestStaticFallback_BundleSetButTrustDomainEmpty_ReturnsError(t *testing.T) {
	impl := &trustBundleGetterImpl{
		staticBundle: validSPIFFEBundle(t),
	}
	_, _, _, err := impl.staticFallback(fmt.Errorf("preceding error"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trust domain is empty")
}

func TestStaticFallback_TrustDomainSetButBundleEmpty_ReturnsError(t *testing.T) {
	impl := &trustBundleGetterImpl{
		trustDomain: "margo.org",
	}
	_, _, _, err := impl.staticFallback(fmt.Errorf("preceding error"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "static trust bundle is empty")
}

func TestStaticFallback_BothConfigured_ReturnsBundleAndDomain(t *testing.T) {
	bundle := validSPIFFEBundle(t)
	impl := &trustBundleGetterImpl{
		staticBundle: bundle,
		trustDomain:  "margo.org",
	}
	td, b, etag, err := impl.staticFallback(fmt.Errorf("preceding error"))
	require.NoError(t, err)
	assert.Equal(t, "margo.org", td)
	assert.Equal(t, bundle, b)
	assert.Empty(t, etag)
}

func TestStaticFallback_PrecedingErrorIsWrapped(t *testing.T) {
	impl := &trustBundleGetterImpl{}
	precedingErr := fmt.Errorf("root cause")
	_, _, _, err := impl.staticFallback(precedingErr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "root cause")
}

// ── GetTrustBundle — discovery success path ───────────────────────────────────

func TestGetTrustBundle_DiscoverySuccess_ReturnsBundleAndEtag(t *testing.T) {
	ca := newTestCA(t)
	bundle := validSPIFFEBundle(t)

	var rawBundle map[string]interface{}
	require.NoError(t, json.Unmarshal(bundle, &rawBundle))

	server := newMISServer(t, ca,
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(discoveryDoc(
				"https://"+r.Host+"/.well-known/spiffe/bundle.json",
				"margo.org",
			))
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("ETag", `"abc123"`)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(bundle)
		},
	)

	getter, err := New(TrustBundleGetterConfig{
		MISEndpoint: server.URL,
		MISRootCA:   ca.certPEM,
	})
	require.NoError(t, err)

	td, b, etag, err := getter.GetTrustBundle(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "margo.org", td)
	assert.NotEmpty(t, b)
	assert.Equal(t, `"abc123"`, etag)
}

func TestGetTrustBundle_Discovery304_ReturnsErrNotModified(t *testing.T) {
	ca := newTestCA(t)

	server := newMISServer(t, ca,
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(discoveryDoc(
				"https://"+r.Host+"/.well-known/spiffe/bundle.json",
				"margo.org",
			))
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotModified)
		},
	)

	getter, err := New(TrustBundleGetterConfig{
		MISEndpoint: server.URL,
		MISRootCA:   ca.certPEM,
	})
	require.NoError(t, err)

	td, b, etag, err := getter.GetTrustBundle(context.Background(), `"abc123"`)
	assert.ErrorIs(t, err, ErrNotModified)
	assert.Equal(t, "margo.org", td)
	assert.Nil(t, b)
	assert.Empty(t, etag)
}

func TestGetTrustBundle_DiscoveryReturnsInvalidBundle_FallsBackToStatic(t *testing.T) {
	ca := newTestCA(t)
	staticBundle := validSPIFFEBundle(t)

	server := newMISServer(t, ca,
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(discoveryDoc(
				"https://"+r.Host+"/.well-known/spiffe/bundle.json",
				"margo.org",
			))
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Return an invalid bundle (empty keys array).
			w.Write([]byte(`{"keys":[]}`))
		},
	)

	getter, err := New(TrustBundleGetterConfig{
		MISEndpoint:  server.URL,
		MISRootCA:    ca.certPEM,
		StaticBundle: staticBundle,
		TrustDomain:  "static.org",
	})
	require.NoError(t, err)

	td, b, etag, err := getter.GetTrustBundle(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "static.org", td)
	assert.Equal(t, staticBundle, b)
	assert.Empty(t, etag)
}

func TestGetTrustBundle_DiscoveryDocMissingTrustBundleURI_FallsBackToStatic(t *testing.T) {
	ca := newTestCA(t)
	staticBundle := validSPIFFEBundle(t)

	server := newMISServer(t, ca,
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// trustBundleUri is empty.
			w.Write(discoveryDoc("", "margo.org"))
		},
		func(w http.ResponseWriter, r *http.Request) {
			// Should not be called.
			w.WriteHeader(http.StatusInternalServerError)
		},
	)

	getter, err := New(TrustBundleGetterConfig{
		MISEndpoint:  server.URL,
		MISRootCA:    ca.certPEM,
		StaticBundle: staticBundle,
		TrustDomain:  "static.org",
	})
	require.NoError(t, err)

	td, b, _, err := getter.GetTrustBundle(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "static.org", td)
	assert.Equal(t, staticBundle, b)
}

func TestGetTrustBundle_DiscoveryNon200_FallsBackToStatic(t *testing.T) {
	ca := newTestCA(t)
	staticBundle := validSPIFFEBundle(t)

	server := newMISServer(t, ca,
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	)

	getter, err := New(TrustBundleGetterConfig{
		MISEndpoint:  server.URL,
		MISRootCA:    ca.certPEM,
		StaticBundle: staticBundle,
		TrustDomain:  "static.org",
	})
	require.NoError(t, err)

	td, b, _, err := getter.GetTrustBundle(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "static.org", td)
	assert.Equal(t, staticBundle, b)
}

// ── GetTrustBundle — URI fallback path ───────────────────────────────────────

func TestGetTrustBundle_DiscoveryFails_URIFallbackSucceeds(t *testing.T) {
	ca := newTestCA(t)
	bundle := validSPIFFEBundle(t)

	server := newMISServer(t, ca,
		func(w http.ResponseWriter, r *http.Request) {
			// Discovery fails.
			w.WriteHeader(http.StatusInternalServerError)
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("ETag", `"uri-etag"`)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(bundle)
		},
	)

	getter, err := New(TrustBundleGetterConfig{
		MISEndpoint:    server.URL,
		MISRootCA:      ca.certPEM,
		TrustBundleURI: "/.well-known/spiffe/bundle.json",
		TrustDomain:    "margo.org",
	})
	require.NoError(t, err)

	td, b, etag, err := getter.GetTrustBundle(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "margo.org", td)
	assert.NotEmpty(t, b)
	assert.Equal(t, `"uri-etag"`, etag)
}

func TestGetTrustBundle_DiscoveryFails_URIFallback304_ReturnsErrNotModified(t *testing.T) {
	ca := newTestCA(t)

	server := newMISServer(t, ca,
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotModified)
		},
	)

	getter, err := New(TrustBundleGetterConfig{
		MISEndpoint:    server.URL,
		MISRootCA:      ca.certPEM,
		TrustBundleURI: "/.well-known/spiffe/bundle.json",
		TrustDomain:    "margo.org",
	})
	require.NoError(t, err)

	td, b, etag, err := getter.GetTrustBundle(context.Background(), `"cached-etag"`)
	assert.ErrorIs(t, err, ErrNotModified)
	assert.Equal(t, "margo.org", td)
	assert.Nil(t, b)
	assert.Empty(t, etag)
}

func TestGetTrustBundle_DiscoveryFails_URIFallbackInvalidBundle_FallsBackToStatic(t *testing.T) {
	ca := newTestCA(t)
	staticBundle := validSPIFFEBundle(t)

	server := newMISServer(t, ca,
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"keys":[]}`)) // invalid bundle
		},
	)

	getter, err := New(TrustBundleGetterConfig{
		MISEndpoint:    server.URL,
		MISRootCA:      ca.certPEM,
		TrustBundleURI: "/.well-known/spiffe/bundle.json",
		StaticBundle:   staticBundle,
		TrustDomain:    "static.org",
	})
	require.NoError(t, err)

	td, b, _, err := getter.GetTrustBundle(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "static.org", td)
	assert.Equal(t, staticBundle, b)
}

// ── GetTrustBundle — all sources fail ────────────────────────────────────────

func TestGetTrustBundle_AllSourcesFail_NoStaticBundle_ReturnsError(t *testing.T) {
	ca := newTestCA(t)

	server := newMISServer(t, ca,
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	)

	getter, err := New(TrustBundleGetterConfig{
		MISEndpoint:    server.URL,
		MISRootCA:      ca.certPEM,
		TrustBundleURI: "/.well-known/spiffe/bundle.json",
		TrustDomain:    "margo.org",
		// No StaticBundle — static fallback will fail.
	})
	require.NoError(t, err)

	_, _, _, err = getter.GetTrustBundle(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "static trust bundle is empty")
}

// ── fetchBundleFromURL ────────────────────────────────────────────────────────

// newBundleOnlyServer starts a TLS server that only serves the bundle endpoint.
func newBundleOnlyServer(t *testing.T, ca *testCA, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/spiffe/bundle.json", handler)
	server := httptest.NewUnstartedServer(mux)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{ca.tlsCert}}
	server.StartTLS()
	t.Cleanup(server.Close)
	return server
}

func TestFetchBundleFromURL_HTTP200_ReturnsBundleAndEtag(t *testing.T) {
	ca := newTestCA(t)
	bundle := validSPIFFEBundle(t)

	server := newBundleOnlyServer(t, ca, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"etag-value"`)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(bundle)
	})

	impl := &trustBundleGetterImpl{
		misEndpoint: server.URL,
		misRootCA:   ca.certPEM,
	}

	c, err := client.New(server.URL, client.TrustConfig{RootCAPEM: ca.certPEM})
	require.NoError(t, err)

	fullURL := server.URL + "/.well-known/spiffe/bundle.json"
	b, etag, err := impl.fetchBundleFromURL(context.Background(), c, "", fullURL)
	require.NoError(t, err)
	assert.NotEmpty(t, b)
	assert.Equal(t, `"etag-value"`, etag)
}

func TestFetchBundleFromURL_HTTP304_ReturnsErrNotModified(t *testing.T) {
	ca := newTestCA(t)

	server := newBundleOnlyServer(t, ca, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	})

	impl := &trustBundleGetterImpl{
		misEndpoint: server.URL,
		misRootCA:   ca.certPEM,
	}

	c, err := client.New(server.URL, client.TrustConfig{RootCAPEM: ca.certPEM})
	require.NoError(t, err)

	fullURL := server.URL + "/.well-known/spiffe/bundle.json"
	b, etag, err := impl.fetchBundleFromURL(context.Background(), c, `"etag"`, fullURL)
	assert.ErrorIs(t, err, ErrNotModified)
	assert.Nil(t, b)
	assert.Empty(t, etag)
}

func TestFetchBundleFromURL_UnexpectedStatus_ReturnsError(t *testing.T) {
	ca := newTestCA(t)

	server := newBundleOnlyServer(t, ca, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	impl := &trustBundleGetterImpl{
		misEndpoint: server.URL,
		misRootCA:   ca.certPEM,
	}

	c, err := client.New(server.URL, client.TrustConfig{RootCAPEM: ca.certPEM})
	require.NoError(t, err)

	fullURL := server.URL + "/.well-known/spiffe/bundle.json"
	_, _, err = impl.fetchBundleFromURL(context.Background(), c, "", fullURL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected HTTP status 403")
}

func TestFetchBundleFromURL_HTTP200ButNilBody_ReturnsError(t *testing.T) {
	ca := newTestCA(t)

	// Return 200 with a body that the client cannot parse as a SPIFFE bundle.
	server := newBundleOnlyServer(t, ca, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Empty body — JSON200 will be nil after parsing.
		w.Write([]byte(""))
	})

	impl := &trustBundleGetterImpl{
		misEndpoint: server.URL,
		misRootCA:   ca.certPEM,
	}

	c, err := client.New(server.URL, client.TrustConfig{RootCAPEM: ca.certPEM})
	require.NoError(t, err)

	fullURL := server.URL + "/.well-known/spiffe/bundle.json"
	_, _, err = impl.fetchBundleFromURL(context.Background(), c, "", fullURL)
	require.Error(t, err)
}

// ── fallbackToURIOrStatic ─────────────────────────────────────────────────────

func TestFallbackToURIOrStatic_NoURINoStatic_ReturnsWrappedDiscoveryError(t *testing.T) {
	impl := &trustBundleGetterImpl{} // no trustBundleURI, no staticBundle
	discoveryErr := fmt.Errorf("discovery failed")

	_, _, _, err := impl.fallbackToURIOrStatic(context.Background(), "", discoveryErr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "discovery failed")
	assert.Contains(t, err.Error(), "no static trust bundle or trust domain configured as fallback")
}

func TestFallbackToURIOrStatic_NoURIWithStatic_ReturnsStaticBundle(t *testing.T) {
	bundle := validSPIFFEBundle(t)
	impl := &trustBundleGetterImpl{
		staticBundle: bundle,
		trustDomain:  "static.org",
	}

	td, b, etag, err := impl.fallbackToURIOrStatic(
		context.Background(), "", fmt.Errorf("discovery failed"),
	)
	require.NoError(t, err)
	assert.Equal(t, "static.org", td)
	assert.Equal(t, bundle, b)
	assert.Empty(t, etag)
}

func TestFallbackToURIOrStatic_URIConfigured_SuccessfulFetch(t *testing.T) {
	ca := newTestCA(t)
	bundle := validSPIFFEBundle(t)

	server := newBundleOnlyServer(t, ca, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"fallback-etag"`)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(bundle)
	})

	impl := &trustBundleGetterImpl{
		misEndpoint:    server.URL,
		misRootCA:      ca.certPEM,
		trustBundleURI: "/.well-known/spiffe/bundle.json",
		trustDomain:    "margo.org",
	}

	td, b, etag, err := impl.fallbackToURIOrStatic(
		context.Background(), "", fmt.Errorf("discovery failed"),
	)
	require.NoError(t, err)
	assert.Equal(t, "margo.org", td)
	assert.NotEmpty(t, b)
	assert.Equal(t, `"fallback-etag"`, etag)
}

func TestFallbackToURIOrStatic_URIConfigured_304_ReturnsErrNotModified(t *testing.T) {
	ca := newTestCA(t)

	server := newBundleOnlyServer(t, ca, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	})

	impl := &trustBundleGetterImpl{
		misEndpoint:    server.URL,
		misRootCA:      ca.certPEM,
		trustBundleURI: "/.well-known/spiffe/bundle.json",
		trustDomain:    "margo.org",
	}

	td, b, etag, err := impl.fallbackToURIOrStatic(
		context.Background(), `"etag"`, fmt.Errorf("discovery failed"),
	)
	assert.ErrorIs(t, err, ErrNotModified)
	assert.Equal(t, "margo.org", td)
	assert.Nil(t, b)
	assert.Empty(t, etag)
}

func TestFallbackToURIOrStatic_URIFails_FallsBackToStatic(t *testing.T) {
	ca := newTestCA(t)
	staticBundle := validSPIFFEBundle(t)

	server := newBundleOnlyServer(t, ca, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	impl := &trustBundleGetterImpl{
		misEndpoint:    server.URL,
		misRootCA:      ca.certPEM,
		trustBundleURI: "/.well-known/spiffe/bundle.json",
		staticBundle:   staticBundle,
		trustDomain:    "static.org",
	}

	td, b, _, err := impl.fallbackToURIOrStatic(
		context.Background(), "", fmt.Errorf("discovery failed"),
	)
	require.NoError(t, err)
	assert.Equal(t, "static.org", td)
	assert.Equal(t, staticBundle, b)
}

// ── validateSPIFFEBundle unit tests ──────────────────────

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
	err := validateSPIFFEBundle([]byte(`{"foo":"bar"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "\"keys\" array")
}

func TestValidateSPIFFEBundle_EmptyKeysArray_ReturnsError(t *testing.T) {
	err := validateSPIFFEBundle([]byte(`{"keys":[]}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no X.509 trust anchors")
}

func TestValidateSPIFFEBundle_KeysWithoutX5c_ReturnsError(t *testing.T) {
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
			{"kty": "EC", "use": "jwt-svid"},
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

// ── validateMISEndpoint ───────────────────────────────────────────────────────

func TestValidateMISEndpoint_Empty_ReturnsError(t *testing.T) {
	err := validateMISEndpoint("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestValidateMISEndpoint_InvalidURL_ReturnsError(t *testing.T) {
	err := validateMISEndpoint("://bad-url")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid URL")
}

func TestValidateMISEndpoint_HTTPScheme_ReturnsError(t *testing.T) {
	err := validateMISEndpoint("http://example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scheme must be \"https\"")
}

func TestValidateMISEndpoint_NoHost_ReturnsError(t *testing.T) {
	err := validateMISEndpoint("https://")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "URL must include a host")
}

func TestValidateMISEndpoint_ValidHTTPS_ReturnsNil(t *testing.T) {
	require.NoError(t, validateMISEndpoint("https://mis.example.com"))
}

func TestValidateMISEndpoint_ValidHTTPSWithPort_ReturnsNil(t *testing.T) {
	require.NoError(t, validateMISEndpoint("https://mis.example.com:8443"))
}

func TestValidateMISEndpoint_ValidHTTPSWithPath_ReturnsNil(t *testing.T) {
	require.NoError(t, validateMISEndpoint("https://mis.example.com/api/v1"))
}

// ── validateRootCA ────────────────────────────────────────────────────────────

func TestValidateRootCA_Empty_ReturnsError(t *testing.T) {
	err := validateRootCA([]byte{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestValidateRootCA_Nil_ReturnsError(t *testing.T) {
	err := validateRootCA(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestValidateRootCA_NotPEM_ReturnsError(t *testing.T) {
	err := validateRootCA([]byte("this is not a PEM certificate"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no valid PEM-encoded X.509 certificate found")
}

func TestValidateRootCA_ValidCA_ReturnsNil(t *testing.T) {
	ca := newTestCA(t)
	require.NoError(t, validateRootCA(ca.certPEM))
}

func TestValidateRootCA_GarbagePEMBlock_ReturnsError(t *testing.T) {
	// Valid PEM structure but garbage DER content inside.
	garbage := []byte("-----BEGIN CERTIFICATE-----\nbm90LWEtY2VydA==\n-----END CERTIFICATE-----\n")
	err := validateRootCA(garbage)
	require.Error(t, err)
	// AppendCertsFromPEM will reject it, or ParseCertificate will fail.
	assert.Error(t, err)
}

// ── validateTrustBundleURI ────────────────────────────────────────────────────

func TestValidateTrustBundleURI_Empty_ReturnsError(t *testing.T) {
	err := validateTrustBundleURI("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestValidateTrustBundleURI_FullURL_ReturnsError(t *testing.T) {
	err := validateTrustBundleURI("https://example.com/.well-known/spiffe/bundle.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a full URL")
}

func TestValidateTrustBundleURI_NoLeadingSlash_ReturnsError(t *testing.T) {
	err := validateTrustBundleURI("well-known/spiffe/bundle.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path must start with \"/\"")
}

func TestValidateTrustBundleURI_ValidPath_ReturnsNil(t *testing.T) {
	require.NoError(t, validateTrustBundleURI("/.well-known/spiffe/bundle.json"))
}

func TestValidateTrustBundleURI_RootPath_ReturnsNil(t *testing.T) {
	require.NoError(t, validateTrustBundleURI("/bundle"))
}

func TestValidateTrustBundleURI_HasSchemeOnly_ReturnsError(t *testing.T) {
	// "http:" parses as a scheme, no host — but scheme is non-empty so it's rejected.
	err := validateTrustBundleURI("http:/bundle")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a full URL")
}

// ── validateModeRequirements ──────────────────────────────────────────────────

func TestValidateModeRequirements_EndpointWithoutTrustAnchor_ReturnsError(t *testing.T) {
	cfg := TrustBundleGetterConfig{
		MISEndpoint: "https://mis.example.com",
	}
	err := validateModeRequirements(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be configured together")
}

func TestValidateModeRequirements_TrustAnchorWithoutEndpoint_ReturnsError(t *testing.T) {
	ca := newTestCA(t)
	cfg := TrustBundleGetterConfig{
		MISRootCA: ca.certPEM,
	}
	err := validateModeRequirements(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be configured together")
}

func TestValidateModeRequirements_NoSourceConfigured_ReturnsError(t *testing.T) {
	err := validateModeRequirements(TrustBundleGetterConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "either MISEndpoint with a trust anchor, or a StaticBundle must be provided")
}

func TestValidateModeRequirements_StaticBundleWithoutTrustDomain_ReturnsError(t *testing.T) {
	cfg := TrustBundleGetterConfig{
		StaticBundle: validSPIFFEBundle(t),
		// TrustDomain intentionally omitted
	}
	err := validateModeRequirements(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TrustDomain is required")
}

func TestValidateModeRequirements_EndpointWithRootCA_ReturnsNil(t *testing.T) {
	ca := newTestCA(t)
	cfg := TrustBundleGetterConfig{
		MISEndpoint: "https://mis.example.com",
		MISRootCA:   ca.certPEM,
	}
	require.NoError(t, validateModeRequirements(cfg))
}

func TestValidateModeRequirements_EndpointWithTrustPins_ReturnsNil(t *testing.T) {
	cfg := TrustBundleGetterConfig{
		MISEndpoint:  "https://mis.example.com",
		MISTrustPins: []string{"somepin"},
	}
	require.NoError(t, validateModeRequirements(cfg))
}

func TestValidateModeRequirements_StaticBundleWithTrustDomain_ReturnsNil(t *testing.T) {
	cfg := TrustBundleGetterConfig{
		StaticBundle: validSPIFFEBundle(t),
		TrustDomain:  "margo.org",
	}
	require.NoError(t, validateModeRequirements(cfg))
}

func TestValidateModeRequirements_BothModesConfigured_ReturnsNil(t *testing.T) {
	ca := newTestCA(t)
	cfg := TrustBundleGetterConfig{
		MISEndpoint:  "https://mis.example.com",
		MISRootCA:    ca.certPEM,
		StaticBundle: validSPIFFEBundle(t),
		TrustDomain:  "margo.org",
	}
	require.NoError(t, validateModeRequirements(cfg))
}

// ── validateEndpointInputs ────────────────────────────────────────────────────

func TestValidateEndpointInputs_InvalidEndpoint_ReturnsError(t *testing.T) {
	cfg := TrustBundleGetterConfig{
		MISEndpoint: "http://not-https.com",
		MISRootCA:   newTestCA(t).certPEM,
	}
	err := validateEndpointInputs(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid MIS endpoint")
}

func TestValidateEndpointInputs_InvalidRootCA_ReturnsError(t *testing.T) {
	cfg := TrustBundleGetterConfig{
		MISEndpoint: "https://mis.example.com",
		MISRootCA:   []byte("not-a-cert"),
	}
	err := validateEndpointInputs(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid MIS root CA")
}

func TestValidateEndpointInputs_ValidRootCA_ReturnsNil(t *testing.T) {
	ca := newTestCA(t)
	cfg := TrustBundleGetterConfig{
		MISEndpoint: "https://mis.example.com",
		MISRootCA:   ca.certPEM,
	}
	require.NoError(t, validateEndpointInputs(cfg))
}

func TestValidateEndpointInputs_NoTrustMaterial_ReturnsNil(t *testing.T) {
	// validateEndpointInputs does not enforce trust anchor presence — that's
	// validateModeRequirements' responsibility.
	cfg := TrustBundleGetterConfig{
		MISEndpoint: "https://mis.example.com",
	}
	require.NoError(t, validateEndpointInputs(cfg))
}

// ── validateNewInputs (integration of all validators) ────────────────────────

func TestValidateNewInputs_EndpointOnlyNoTrustAnchor_ReturnsError(t *testing.T) {
	cfg := TrustBundleGetterConfig{
		MISEndpoint: "https://mis.example.com",
	}
	err := validateNewInputs(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be configured together")
}

func TestValidateNewInputs_InvalidTrustBundleURI_ReturnsError(t *testing.T) {
	ca := newTestCA(t)
	cfg := TrustBundleGetterConfig{
		MISEndpoint:    "https://mis.example.com",
		MISRootCA:      ca.certPEM,
		TrustBundleURI: "https://full-url-not-allowed.com/bundle",
	}
	err := validateNewInputs(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid trust bundle URI")
}

func TestValidateNewInputs_InvalidStaticBundle_ReturnsError(t *testing.T) {
	cfg := TrustBundleGetterConfig{
		StaticBundle: []byte(`{"keys":[]}`),
		TrustDomain:  "margo.org",
	}
	err := validateNewInputs(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid static trust bundle")
}

func TestValidateNewInputs_ValidEndpointMode_ReturnsNil(t *testing.T) {
	ca := newTestCA(t)
	cfg := TrustBundleGetterConfig{
		MISEndpoint: "https://mis.example.com",
		MISRootCA:   ca.certPEM,
	}
	require.NoError(t, validateNewInputs(cfg))
}

func TestValidateNewInputs_ValidStaticBundleMode_ReturnsNil(t *testing.T) {
	cfg := TrustBundleGetterConfig{
		StaticBundle: validSPIFFEBundle(t),
		TrustDomain:  "margo.org",
	}
	require.NoError(t, validateNewInputs(cfg))
}

func TestValidateNewInputs_ValidBothModes_ReturnsNil(t *testing.T) {
	ca := newTestCA(t)
	cfg := TrustBundleGetterConfig{
		MISEndpoint:    "https://mis.example.com",
		MISRootCA:      ca.certPEM,
		TrustBundleURI: "/.well-known/spiffe/bundle.json",
		StaticBundle:   validSPIFFEBundle(t),
		TrustDomain:    "margo.org",
	}
	require.NoError(t, validateNewInputs(cfg))
}

func TestValidateNewInputs_ValidEndpointWithTrustPins_ReturnsNil(t *testing.T) {
	cfg := TrustBundleGetterConfig{
		MISEndpoint:  "https://mis.example.com",
		MISTrustPins: []string{"sha256/6DtZYsyT+9vuAwfAKHCLpjQv2pQHCc9n7nnZsltwEU4="},
	}
	require.NoError(t, validateNewInputs(cfg))
}
