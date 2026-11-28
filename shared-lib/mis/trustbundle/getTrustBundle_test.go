// filepath: getTrustBundle_test.go
package trustbundle

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
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

// ── Test helpers ──────────────────────────────────────────────────────────────

const (
	testBundlePath    = "/.well-known/spiffe/bundle.json"
	testDiscoveryPath = "/.well-known/margo"
)

type testCA struct {
	certPEM []byte
	tlsCert tls.Certificate
	pin     string // "sha256/<base64 SPKI digest>", used for IP-addressed URLs
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
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)

	spki, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	digest := sha256.Sum256(spki)

	return &testCA{
		certPEM: certPEM,
		tlsCert: tlsCert,
		pin:     "sha256/" + base64.StdEncoding.EncodeToString(digest[:]),
	}
}

// newTLSServer starts an HTTPS server serving the given path→handler map.
func newTLSServer(t *testing.T, ca *testCA, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	for path, h := range handlers {
		mux.HandleFunc(path, h)
	}

	server := httptest.NewUnstartedServer(mux)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{ca.tlsCert}}
	server.StartTLS()
	t.Cleanup(server.Close)

	return server
}

// discoveryHandler returns a discovery document whose trustBundleUri points at
// bundlePath on the host the request was made to.
func discoveryHandler(trustDomain, bundlePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"trustBundleUri": "https://" + r.Host + bundlePath,
			"trustDomain":    trustDomain,
		})
	}
}

func statusHandler(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
	}
}

func bundleHandler(etag string, body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if etag != "" {
			w.Header().Set("ETag", etag)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}
}

// validSPIFFEBundle returns a JWKS bundle that passes validators.ValidateSpiffeTrustBundle:
// an EC key with crv/x/y, use "x509-svid", and an x5c CA certificate with
// IsCA=true and KeyUsageCertSign.
func validSPIFFEBundle(t *testing.T) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "spiffe-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	jwk := map[string]interface{}{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(key.PublicKey.X.FillBytes(make([]byte, 32))),
		"y":   base64.RawURLEncoding.EncodeToString(key.PublicKey.Y.FillBytes(make([]byte, 32))),
		"use": "x509-svid",
		"x5c": []string{base64.StdEncoding.EncodeToString(der)},
	}
	b, err := json.Marshal(map[string]interface{}{"keys": []interface{}{jwk}})
	require.NoError(t, err)
	return b
}

// validConfig builds a discovery-mode config. IP-addressed server URLs require pins.
func validConfig(ca *testCA, server *httptest.Server, t *testing.T) TrustBundleGetterConfig {
	t.Helper()
	return TrustBundleGetterConfig{
		DiscoveryURL: server.URL + testDiscoveryPath,
		MISTrustPins: []string{ca.pin},
		TrustDomain:  "static.org",
		StaticBundle: validSPIFFEBundle(t),
	}
}

func newTestMISClient(t *testing.T, ca *testCA) *client.MISClient {
	t.Helper()
	c, err := client.New(client.TrustConfig{SPKIPins: []string{ca.pin}})
	require.NoError(t, err)
	return c
}

// ── New ───────────────────────────────────────────────────────────────────────

func TestNew_ValidDiscoveryConfig_ReturnsGetter(t *testing.T) {
	ca := newTestCA(t)
	cfg := TrustBundleGetterConfig{
		DiscoveryURL: "https://192.0.2.10/.well-known/margo",
		MISRootCA:    ca.certPEM,
		StaticBundle: validSPIFFEBundle(t),
		TrustDomain:  "margo.org",
	}

	getter, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, getter)
}

func TestNew_EmptyConfig_ReturnsError(t *testing.T) {
	getter, err := New(TrustBundleGetterConfig{})
	require.Error(t, err)
	assert.Nil(t, getter)
}

// ── validateNewInputs ─────────────────────────────────────────────────────────

func TestValidateNewInputs_NoSourceNoStatic_ReturnsModeError(t *testing.T) {
	err := validateNewInputs(TrustBundleGetterConfig{})
	require.Error(t, err)
	assert.ErrorContains(t, err, "either DiscoveryURL, TrustBundleURI, or StaticBundle must be provided")
}

func TestValidateNewInputs_NetworkWithoutTrustAnchor_ReturnsError(t *testing.T) {
	cfg := TrustBundleGetterConfig{
		DiscoveryURL: "https://mis.example.com/.well-known/margo",
		StaticBundle: validSPIFFEBundle(t),
		TrustDomain:  "margo.org",
	}
	err := validateNewInputs(cfg)
	require.Error(t, err)
	assert.ErrorContains(t, err, "requires a trust anchor")
}

func TestValidateNewInputs_TrustAnchorWithoutNetwork_ReturnsError(t *testing.T) {
	ca := newTestCA(t)
	cfg := TrustBundleGetterConfig{
		MISRootCA:    ca.certPEM,
		StaticBundle: validSPIFFEBundle(t),
		TrustDomain:  "margo.org",
	}
	err := validateNewInputs(cfg)
	require.Error(t, err)
	assert.ErrorContains(t, err, "requires DiscoveryURL or TrustBundleURI")
}

func TestValidateNewInputs_NetworkWithoutStaticBundle_ReturnsError(t *testing.T) {
	ca := newTestCA(t)
	cfg := TrustBundleGetterConfig{
		DiscoveryURL: "https://mis.example.com/.well-known/margo",
		MISRootCA:    ca.certPEM,
	}
	err := validateNewInputs(cfg)
	require.Error(t, err)
	assert.ErrorContains(t, err, "static trust bundle is required")
}

func TestValidateNewInputs_InvalidDiscoveryURL_ReturnsError(t *testing.T) {
	ca := newTestCA(t)
	cfg := TrustBundleGetterConfig{
		DiscoveryURL: "http://mis.example.com/.well-known/margo",
		MISRootCA:    ca.certPEM,
		StaticBundle: validSPIFFEBundle(t),
		TrustDomain:  "margo.org",
	}
	err := validateNewInputs(cfg)
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid DiscoveryURL")
}

func TestValidateNewInputs_InvalidTrustBundleURI_ReturnsError(t *testing.T) {
	ca := newTestCA(t)
	cfg := TrustBundleGetterConfig{
		TrustBundleURI: "http://mis.example.com/bundle",
		MISRootCA:      ca.certPEM,
		StaticBundle:   validSPIFFEBundle(t),
		TrustDomain:    "margo.org",
	}
	err := validateNewInputs(cfg)
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid TrustBundleURI")
}

func TestValidateNewInputs_InvalidStaticBundle_ReturnsError(t *testing.T) {
	ca := newTestCA(t)
	cfg := TrustBundleGetterConfig{
		DiscoveryURL: "https://mis.example.com/.well-known/margo",
		MISRootCA:    ca.certPEM,
		StaticBundle: []byte(`{"keys":[]}`),
		TrustDomain:  "margo.org",
	}
	err := validateNewInputs(cfg)
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid static trust bundle")
}

func TestValidateNewInputs_InvalidTrustDomain_ReturnsError(t *testing.T) {
	ca := newTestCA(t)
	cfg := TrustBundleGetterConfig{
		DiscoveryURL: "https://mis.example.com/.well-known/margo",
		MISRootCA:    ca.certPEM,
		StaticBundle: validSPIFFEBundle(t),
		TrustDomain:  "https://margo.org/path",
	}
	err := validateNewInputs(cfg)
	require.Error(t, err)
	assert.ErrorContains(t, err, "valid TrustDomain is required")
}

func TestValidateNewInputs_MissingTrustDomain_ReturnsError(t *testing.T) {
	ca := newTestCA(t)
	cfg := TrustBundleGetterConfig{
		DiscoveryURL: "https://mis.example.com/.well-known/margo",
		MISRootCA:    ca.certPEM,
		StaticBundle: validSPIFFEBundle(t),
	}
	err := validateNewInputs(cfg)
	require.Error(t, err)
	assert.ErrorContains(t, err, "valid TrustDomain is required")
}

func TestValidateNewInputs_DiscoveryWithRootCA_ReturnsNil(t *testing.T) {
	ca := newTestCA(t)
	cfg := TrustBundleGetterConfig{
		DiscoveryURL: "https://mis.example.com/.well-known/margo",
		MISRootCA:    ca.certPEM,
		StaticBundle: validSPIFFEBundle(t),
		TrustDomain:  "margo.org",
	}
	require.NoError(t, validateNewInputs(cfg))
}

func TestValidateNewInputs_DiscoveryWithTrustPins_ReturnsNil(t *testing.T) {
	ca := newTestCA(t)
	cfg := TrustBundleGetterConfig{
		DiscoveryURL: "https://192.0.2.10/.well-known/margo",
		MISTrustPins: []string{ca.pin},
		StaticBundle: validSPIFFEBundle(t),
		TrustDomain:  "margo.org",
	}
	require.NoError(t, validateNewInputs(cfg))
}

func TestValidateNewInputs_AllSourcesConfigured_ReturnsNil(t *testing.T) {
	ca := newTestCA(t)
	cfg := TrustBundleGetterConfig{
		DiscoveryURL:   "https://mis.example.com/.well-known/margo",
		TrustBundleURI: "https://mis.example.com/.well-known/spiffe/bundle.json",
		MISRootCA:      ca.certPEM,
		StaticBundle:   validSPIFFEBundle(t),
		TrustDomain:    "margo.org",
	}
	require.NoError(t, validateNewInputs(cfg))
}

// ── validateModeRequirements ──────────────────────────────────────────────────

func TestValidateModeRequirements_Empty_ReturnsError(t *testing.T) {
	err := validateModeRequirements(TrustBundleGetterConfig{})
	require.Error(t, err)
	assert.ErrorContains(t, err, "either DiscoveryURL, TrustBundleURI, or StaticBundle must be provided")
}

func TestValidateModeRequirements_StaticOnly_ReturnsNil(t *testing.T) {
	require.NoError(t, validateModeRequirements(TrustBundleGetterConfig{
		StaticBundle: validSPIFFEBundle(t),
	}))
}

func TestValidateModeRequirements_TrustBundleURIWithoutAnchor_ReturnsError(t *testing.T) {
	err := validateModeRequirements(TrustBundleGetterConfig{
		TrustBundleURI: "https://mis.example.com/bundle",
		StaticBundle:   validSPIFFEBundle(t),
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "requires a trust anchor")
}

func TestValidateModeRequirements_TrustPinsWithoutNetwork_ReturnsError(t *testing.T) {
	ca := newTestCA(t)
	err := validateModeRequirements(TrustBundleGetterConfig{
		MISTrustPins: []string{ca.pin},
		StaticBundle: validSPIFFEBundle(t),
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "requires DiscoveryURL or TrustBundleURI")
}

// ── validateTrustAnchors ──────────────────────────────────────────────────────

func TestValidateTrustAnchors_ValidRootCA_ReturnsNil(t *testing.T) {
	ca := newTestCA(t)
	require.NoError(t, validateTrustAnchors(TrustBundleGetterConfig{MISRootCA: ca.certPEM}))
}

func TestValidateTrustAnchors_InvalidRootCA_ReturnsError(t *testing.T) {
	err := validateTrustAnchors(TrustBundleGetterConfig{MISRootCA: []byte("not-a-cert")})
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid MIS root CA")
}

func TestValidateTrustAnchors_ValidTrustPin_ReturnsNil(t *testing.T) {
	ca := newTestCA(t)
	require.NoError(t, validateTrustAnchors(TrustBundleGetterConfig{
		MISTrustPins: []string{ca.pin},
	}))
}

func TestValidateTrustAnchors_InvalidTrustPin_ReturnsError(t *testing.T) {
	err := validateTrustAnchors(TrustBundleGetterConfig{
		MISTrustPins: []string{"!!!not-a-pin!!!"},
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid MIS trust pin")
}

func TestValidateTrustAnchors_PinWithWrongDigestLength_ReturnsError(t *testing.T) {
	err := validateTrustAnchors(TrustBundleGetterConfig{
		MISTrustPins: []string{"sha256/" + base64.StdEncoding.EncodeToString([]byte("short"))},
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid MIS trust pin")
}

func TestValidateTrustAnchors_NoAnchors_ReturnsNil(t *testing.T) {
	require.NoError(t, validateTrustAnchors(TrustBundleGetterConfig{}))
}

// ── validateHTTPSURL ──────────────────────────────────────────────────────────

func TestValidateHTTPSURL_Empty_ReturnsError(t *testing.T) {
	err := validateHTTPSURL("")
	require.Error(t, err)
	assert.ErrorContains(t, err, "must not be empty")
}

func TestValidateHTTPSURL_Unparseable_ReturnsError(t *testing.T) {
	err := validateHTTPSURL("://bad-url")
	require.Error(t, err)
	assert.ErrorContains(t, err, "not a valid URL")
}

func TestValidateHTTPSURL_HTTPScheme_ReturnsError(t *testing.T) {
	err := validateHTTPSURL("http://example.com/bundle")
	require.Error(t, err)
	assert.ErrorContains(t, err, `scheme must be "https"`)
}

func TestValidateHTTPSURL_NoHost_ReturnsError(t *testing.T) {
	err := validateHTTPSURL("https://")
	require.Error(t, err)
	assert.ErrorContains(t, err, "URL must include a host")
}

func TestValidateHTTPSURL_ValidURL_ReturnsNil(t *testing.T) {
	require.NoError(t, validateHTTPSURL("https://mis.example.com/.well-known/margo"))
}

func TestValidateHTTPSURL_ValidURLWithPort_ReturnsNil(t *testing.T) {
	require.NoError(t, validateHTTPSURL("https://192.0.2.10:8443/.well-known/margo"))
}

// ── newClient ─────────────────────────────────────────────────────────────────

func TestNewClient_RootCAOnly_ReturnsClient(t *testing.T) {
	ca := newTestCA(t)
	impl := &trustBundleGetterImpl{misRootCA: ca.certPEM}

	c, err := impl.newClient()
	require.NoError(t, err)
	assert.NotNil(t, c)
}

func TestNewClient_NoTrustMaterial_ReturnsError(t *testing.T) {
	impl := &trustBundleGetterImpl{}

	c, err := impl.newClient()
	require.Error(t, err)
	assert.Nil(t, c)
}

// ── staticFallback ────────────────────────────────────────────────────────────

func TestStaticFallback_NoStaticBundle_ReturnsWrappedError(t *testing.T) {
	impl := &trustBundleGetterImpl{trustDomain: "margo.org"}

	_, _, _, err := impl.staticFallback(errors.New("root cause"))
	require.Error(t, err)
	assert.ErrorContains(t, err, "root cause")
	assert.ErrorContains(t, err, "no static trust bundle configured as fallback")
}

func TestStaticFallback_StaticBundleConfigured_ReturnsBundleAndDomain(t *testing.T) {
	bundle := validSPIFFEBundle(t)
	impl := &trustBundleGetterImpl{staticBundle: bundle, trustDomain: "margo.org"}

	td, b, etag, err := impl.staticFallback(errors.New("preceding"))
	require.NoError(t, err)
	assert.Equal(t, "margo.org", td)
	assert.Equal(t, bundle, b)
	assert.Empty(t, etag)
}

// ── fetchBundleFromURL ────────────────────────────────────────────────────────

func TestFetchBundleFromURL_HTTP200_ReturnsBundleAndEtag(t *testing.T) {
	ca := newTestCA(t)
	server := newTLSServer(t, ca, map[string]http.HandlerFunc{
		testBundlePath: bundleHandler(`"etag-value"`, validSPIFFEBundle(t)),
	})
	impl := &trustBundleGetterImpl{}

	b, etag, err := impl.fetchBundleFromURL(
		context.Background(), newTestMISClient(t, ca), "", server.URL+testBundlePath,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, b)
	assert.Equal(t, `"etag-value"`, etag)
}

func TestFetchBundleFromURL_HTTP304_ReturnsErrNotModified(t *testing.T) {
	ca := newTestCA(t)
	server := newTLSServer(t, ca, map[string]http.HandlerFunc{
		testBundlePath: statusHandler(http.StatusNotModified),
	})
	impl := &trustBundleGetterImpl{}

	b, etag, err := impl.fetchBundleFromURL(
		context.Background(), newTestMISClient(t, ca), `"etag"`, server.URL+testBundlePath,
	)
	assert.ErrorIs(t, err, ErrNotModified)
	assert.Nil(t, b)
	assert.Empty(t, etag)
}

func TestFetchBundleFromURL_UnexpectedStatus_ReturnsError(t *testing.T) {
	ca := newTestCA(t)
	server := newTLSServer(t, ca, map[string]http.HandlerFunc{
		testBundlePath: statusHandler(http.StatusForbidden),
	})
	impl := &trustBundleGetterImpl{}

	_, _, err := impl.fetchBundleFromURL(
		context.Background(), newTestMISClient(t, ca), "", server.URL+testBundlePath,
	)
	require.Error(t, err)
	assert.ErrorContains(t, err, "unexpected HTTP status 403")
}

func TestFetchBundleFromURL_HTTP200EmptyBody_ReturnsError(t *testing.T) {
	ca := newTestCA(t)
	server := newTLSServer(t, ca, map[string]http.HandlerFunc{
		testBundlePath: bundleHandler("", []byte("")),
	})
	impl := &trustBundleGetterImpl{}

	_, _, err := impl.fetchBundleFromURL(
		context.Background(), newTestMISClient(t, ca), "", server.URL+testBundlePath,
	)
	require.Error(t, err)
}

func TestFetchBundleFromURL_UnreachableHost_ReturnsError(t *testing.T) {
	ca := newTestCA(t)
	impl := &trustBundleGetterImpl{}

	_, _, err := impl.fetchBundleFromURL(
		context.Background(), newTestMISClient(t, ca), "", "https://127.0.0.1:1"+testBundlePath,
	)
	require.Error(t, err)
	assert.ErrorContains(t, err, "GetTrustBundle request")
}

// ── fallbackToURIOrStatic ─────────────────────────────────────────────────────

func TestFallbackToURIOrStatic_NoURINoStatic_ReturnsWrappedPrecedingError(t *testing.T) {
	ca := newTestCA(t)
	impl := &trustBundleGetterImpl{}

	_, _, _, err := impl.fallbackToURIOrStatic(
		context.Background(), newTestMISClient(t, ca), "", errors.New("discovery failed"),
	)
	require.Error(t, err)
	assert.ErrorContains(t, err, "discovery failed")
	assert.ErrorContains(t, err, "no static trust bundle configured as fallback")
}

func TestFallbackToURIOrStatic_NoURIWithStatic_ReturnsStaticBundle(t *testing.T) {
	ca := newTestCA(t)
	bundle := validSPIFFEBundle(t)
	impl := &trustBundleGetterImpl{staticBundle: bundle, trustDomain: "static.org"}

	td, b, etag, err := impl.fallbackToURIOrStatic(
		context.Background(), newTestMISClient(t, ca), "", errors.New("discovery failed"),
	)
	require.NoError(t, err)
	assert.Equal(t, "static.org", td)
	assert.Equal(t, bundle, b)
	assert.Empty(t, etag)
}

func TestFallbackToURIOrStatic_URISucceeds_ReturnsURIBundle(t *testing.T) {
	ca := newTestCA(t)
	bundle := validSPIFFEBundle(t)
	server := newTLSServer(t, ca, map[string]http.HandlerFunc{
		testBundlePath: bundleHandler(`"fallback-etag"`, bundle),
	})
	impl := &trustBundleGetterImpl{
		trustBundleURI: server.URL + testBundlePath,
		trustDomain:    "margo.org",
	}

	td, b, etag, err := impl.fallbackToURIOrStatic(
		context.Background(), newTestMISClient(t, ca), "", errors.New("discovery failed"),
	)
	require.NoError(t, err)
	assert.Equal(t, "margo.org", td)
	assert.NotEmpty(t, b)
	assert.Equal(t, `"fallback-etag"`, etag)
}

func TestFallbackToURIOrStatic_URI304_ReturnsErrNotModified(t *testing.T) {
	ca := newTestCA(t)
	server := newTLSServer(t, ca, map[string]http.HandlerFunc{
		testBundlePath: statusHandler(http.StatusNotModified),
	})
	impl := &trustBundleGetterImpl{
		trustBundleURI: server.URL + testBundlePath,
		staticBundle:   validSPIFFEBundle(t),
		trustDomain:    "margo.org",
	}

	td, b, etag, err := impl.fallbackToURIOrStatic(
		context.Background(), newTestMISClient(t, ca), `"cached"`, errors.New("discovery failed"),
	)
	assert.ErrorIs(t, err, ErrNotModified)
	assert.Equal(t, "margo.org", td)
	assert.Nil(t, b)
	assert.Empty(t, etag)
}

func TestFallbackToURIOrStatic_URIHTTPError_FallsBackToStatic(t *testing.T) {
	ca := newTestCA(t)
	staticBundle := validSPIFFEBundle(t)
	server := newTLSServer(t, ca, map[string]http.HandlerFunc{
		testBundlePath: statusHandler(http.StatusInternalServerError),
	})
	impl := &trustBundleGetterImpl{
		trustBundleURI: server.URL + testBundlePath,
		staticBundle:   staticBundle,
		trustDomain:    "static.org",
	}

	td, b, _, err := impl.fallbackToURIOrStatic(
		context.Background(), newTestMISClient(t, ca), "", errors.New("discovery failed"),
	)
	require.NoError(t, err)
	assert.Equal(t, "static.org", td)
	assert.Equal(t, staticBundle, b)
}

func TestFallbackToURIOrStatic_URIInvalidBundle_FallsBackToStatic(t *testing.T) {
	ca := newTestCA(t)
	staticBundle := validSPIFFEBundle(t)
	server := newTLSServer(t, ca, map[string]http.HandlerFunc{
		testBundlePath: bundleHandler("", []byte(`{"keys":[]}`)),
	})
	impl := &trustBundleGetterImpl{
		trustBundleURI: server.URL + testBundlePath,
		staticBundle:   staticBundle,
		trustDomain:    "static.org",
	}

	td, b, _, err := impl.fallbackToURIOrStatic(
		context.Background(), newTestMISClient(t, ca), "", errors.New("discovery failed"),
	)
	require.NoError(t, err)
	assert.Equal(t, "static.org", td)
	assert.Equal(t, staticBundle, b)
}

// ── GetTrustBundle — discovery success ────────────────────────────────────────

func TestGetTrustBundle_DiscoverySuccess_ReturnsDiscoveredDomainBundleAndEtag(t *testing.T) {
	ca := newTestCA(t)
	server := newTLSServer(t, ca, map[string]http.HandlerFunc{
		testDiscoveryPath: discoveryHandler("margo.org", testBundlePath),
		testBundlePath:    bundleHandler(`"abc123"`, validSPIFFEBundle(t)),
	})
	getter, err := New(validConfig(ca, server, t))
	require.NoError(t, err)

	td, b, etag, err := getter.GetTrustBundle(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "margo.org", td)
	assert.NotEmpty(t, b)
	assert.Equal(t, `"abc123"`, etag)
}

func TestGetTrustBundle_DiscoveryBundle304_ReturnsErrNotModifiedWithDiscoveredDomain(t *testing.T) {
	ca := newTestCA(t)
	server := newTLSServer(t, ca, map[string]http.HandlerFunc{
		testDiscoveryPath: discoveryHandler("margo.org", testBundlePath),
		testBundlePath:    statusHandler(http.StatusNotModified),
	})
	getter, err := New(validConfig(ca, server, t))
	require.NoError(t, err)

	td, b, etag, err := getter.GetTrustBundle(context.Background(), `"abc123"`)
	assert.ErrorIs(t, err, ErrNotModified)
	assert.Equal(t, "margo.org", td)
	assert.Nil(t, b)
	assert.Empty(t, etag)
}

// ── GetTrustBundle — discovery failure → URI / static ────────────────────────

func TestGetTrustBundle_DiscoveryNon200_FallsBackToStatic(t *testing.T) {
	ca := newTestCA(t)
	server := newTLSServer(t, ca, map[string]http.HandlerFunc{
		testDiscoveryPath: statusHandler(http.StatusServiceUnavailable),
	})
	cfg := validConfig(ca, server, t)
	getter, err := New(cfg)
	require.NoError(t, err)

	td, b, etag, err := getter.GetTrustBundle(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "static.org", td)
	assert.Equal(t, cfg.StaticBundle, b)
	assert.Empty(t, etag)
}

func TestGetTrustBundle_DiscoveryMissingTrustBundleURI_FallsBackToStatic(t *testing.T) {
	ca := newTestCA(t)
	server := newTLSServer(t, ca, map[string]http.HandlerFunc{
		testDiscoveryPath: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"trustBundleUri":"","trustDomain":"margo.org"}`))
		},
	})
	cfg := validConfig(ca, server, t)
	getter, err := New(cfg)
	require.NoError(t, err)

	td, b, _, err := getter.GetTrustBundle(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "static.org", td)
	assert.Equal(t, cfg.StaticBundle, b)
}

func TestGetTrustBundle_DiscoveredBundleInvalid_FallsBackToStatic(t *testing.T) {
	ca := newTestCA(t)
	server := newTLSServer(t, ca, map[string]http.HandlerFunc{
		testDiscoveryPath: discoveryHandler("margo.org", testBundlePath),
		testBundlePath:    bundleHandler("", []byte(`{"keys":[]}`)),
	})
	cfg := validConfig(ca, server, t)
	getter, err := New(cfg)
	require.NoError(t, err)

	td, b, etag, err := getter.GetTrustBundle(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "static.org", td)
	assert.Equal(t, cfg.StaticBundle, b)
	assert.Empty(t, etag)
}

func TestGetTrustBundle_DiscoveryFailsURISucceeds_ReturnsConfiguredDomain(t *testing.T) {
	ca := newTestCA(t)
	bundle := validSPIFFEBundle(t)
	server := newTLSServer(t, ca, map[string]http.HandlerFunc{
		testDiscoveryPath: statusHandler(http.StatusInternalServerError),
		testBundlePath:    bundleHandler(`"uri-etag"`, bundle),
	})
	cfg := validConfig(ca, server, t)
	cfg.TrustBundleURI = server.URL + testBundlePath
	getter, err := New(cfg)
	require.NoError(t, err)

	td, b, etag, err := getter.GetTrustBundle(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "static.org", td)
	assert.NotEmpty(t, b)
	assert.Equal(t, `"uri-etag"`, etag)
}

func TestGetTrustBundle_DiscoveryFailsURI304_ReturnsErrNotModified(t *testing.T) {
	ca := newTestCA(t)
	server := newTLSServer(t, ca, map[string]http.HandlerFunc{
		testDiscoveryPath: statusHandler(http.StatusInternalServerError),
		testBundlePath:    statusHandler(http.StatusNotModified),
	})
	cfg := validConfig(ca, server, t)
	cfg.TrustBundleURI = server.URL + testBundlePath
	getter, err := New(cfg)
	require.NoError(t, err)

	td, b, etag, err := getter.GetTrustBundle(context.Background(), `"cached"`)
	assert.ErrorIs(t, err, ErrNotModified)
	assert.Equal(t, "static.org", td)
	assert.Nil(t, b)
	assert.Empty(t, etag)
}

func TestGetTrustBundle_DiscoveryAndURIFail_FallsBackToStatic(t *testing.T) {
	ca := newTestCA(t)
	server := newTLSServer(t, ca, map[string]http.HandlerFunc{
		testDiscoveryPath: statusHandler(http.StatusInternalServerError),
		testBundlePath:    statusHandler(http.StatusInternalServerError),
	})
	cfg := validConfig(ca, server, t)
	cfg.TrustBundleURI = server.URL + testBundlePath
	getter, err := New(cfg)
	require.NoError(t, err)

	td, b, _, err := getter.GetTrustBundle(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "static.org", td)
	assert.Equal(t, cfg.StaticBundle, b)
}

// ── GetTrustBundle — total failure and no-network paths ──────────────────────

func TestGetTrustBundle_AllSourcesFailNoStatic_ReturnsError(t *testing.T) {
	ca := newTestCA(t)
	server := newTLSServer(t, ca, map[string]http.HandlerFunc{
		testDiscoveryPath: statusHandler(http.StatusInternalServerError),
		testBundlePath:    statusHandler(http.StatusInternalServerError),
	})
	impl := &trustBundleGetterImpl{
		discoveryURL:   server.URL + testDiscoveryPath,
		misTrustPins:   []string{ca.pin},
		trustBundleURI: server.URL + testBundlePath,
		trustDomain:    "margo.org",
	}

	_, _, _, err := impl.GetTrustBundle(context.Background(), "")
	require.Error(t, err)
	assert.ErrorContains(t, err, "no static trust bundle configured as fallback")
}

func TestGetTrustBundle_NoDiscoveryURLNoURI_UsesStatic(t *testing.T) {
	ca := newTestCA(t)
	bundle := validSPIFFEBundle(t)
	impl := &trustBundleGetterImpl{
		misRootCA:    ca.certPEM,
		staticBundle: bundle,
		trustDomain:  "static.org",
	}

	td, b, etag, err := impl.GetTrustBundle(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "static.org", td)
	assert.Equal(t, bundle, b)
	assert.Empty(t, etag)
}

func TestGetTrustBundle_NoDiscoveryURLNoStatic_ReturnsError(t *testing.T) {
	ca := newTestCA(t)
	impl := &trustBundleGetterImpl{misRootCA: ca.certPEM}

	_, _, _, err := impl.GetTrustBundle(context.Background(), "")
	require.Error(t, err)
	assert.ErrorContains(t, err, "discovery skipped: no DiscoveryURL configured")
}

func TestGetTrustBundle_StaticFallbackNotAttemptedOn304(t *testing.T) {
	ca := newTestCA(t)
	server := newTLSServer(t, ca, map[string]http.HandlerFunc{
		testDiscoveryPath: discoveryHandler("margo.org", testBundlePath),
		testBundlePath:    statusHandler(http.StatusNotModified),
	})
	cfg := validConfig(ca, server, t)
	getter, err := New(cfg)
	require.NoError(t, err)

	td, b, _, err := getter.GetTrustBundle(context.Background(), `"cached"`)
	assert.ErrorIs(t, err, ErrNotModified)
	assert.Nil(t, b, "static bundle must not be returned on 304")
	assert.Equal(t, "margo.org", td)
}
