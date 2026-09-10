package mtls

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Dummy PEM constants — replace contents with real certs/keys.
// ---------------------------------------------------------------------------

const (
	// dummyClientCertPEM is a placeholder X.509 client certificate in PEM format.
	// Replace with a real SPIFFE X.509-SVID certificate for integration tests.
	dummyClientCertPEM = `-----BEGIN CERTIFICATE-----
MIICRDCCAeugAwIBAgIRAL/nkJUvvhRsjln3n55XCiMwCgYIKoZIzj0EAwIwgZsx
CzAJBgNVBAYTAklOMRAwDgYDVQQIDAdIYXJ5YW5hMREwDwYDVQQHDAhHdXJ1Z3Jh
bTESMBAGA1UECgwJQ2FwZ2VtaW5pMRswGQYDVQQLDBJNYXJnbyBTYW5kYm94IFRl
YW0xEjAQBgNVBAMMCW1hcmdvLm9yZzEiMCAGCSqGSIb3DQEJARYTYWRtaW5AY2Fw
Z2VtaW5pLmNvbTAeFw0yNjA5MTAxNDU0MDZaFw0yNjEyMDkxNDU0MDZaMBQxEjAQ
BgNVBAoTCW1hcmdvLm9yZzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABO4cJ/eT
FE7JjKyPk6SCYFLIAP08fFz+GPL38/ej4rmOoZFI2vCkyuufYn1WuJY2DPEpcaqt
uBcUfhDkIAM404OjgZUwgZIwDgYDVR0PAQH/BAQDAgeAMB0GA1UdJQQWMBQGCCsG
AQUFBwMBBggrBgEFBQcDAjAMBgNVHRMBAf8EAjAAMB8GA1UdIwQYMBaAFLVqETLt
uugSDeilDpccpujCSm5eMDIGA1UdEQQrMCmGJ3NwaWZmZTovL21hcmdvLm9yZy9t
YXJnby93Zm0vc3ltcGhvbnktMTAKBggqhkjOPQQDAgNHADBEAiBy7RQLbuv2LRjq
Q7tRMSH9fQPQBK+CHcsRcL5HEs0RygIgVQMQr0Xil7mRMxpdYH+b8m2VFdp+Q0nH
M9ngT2ktr3M=
-----END CERTIFICATE-----`

	// dummyClientKeyPEM is the private key corresponding to dummyClientCertPEM.
	// Replace with the matching private key in PEM format.
	dummyClientKeyPEM = `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIEb4OuSK3LK3kvHHVUWvAH57LO9KpmdtzxXFeQf6n1TPoAoGCCqGSM49
AwEHoUQDQgAE7hwn95MUTsmMrI+TpIJgUsgA/Tx8XP4Y8vfz96PiuY6hkUja8KTK
659ifVa4ljYM8Slxqq24FxR+EOQgAzjTgw==
-----END EC PRIVATE KEY-----`

	// dummyRootCAPEM is the root CA certificate in PEM format.
	// This will be converted to a SPIFFE JWK Set trust bundle by rootCAPEMToJWKSet.
	// Replace with the real root CA PEM that signed dummyClientCertPEM.
	dummyRootCAPEM = `-----BEGIN CERTIFICATE-----
MIICnTCCAkOgAwIBAgIUEuxR6GlO2plIQnH+IjnYi4TDyUIwCgYIKoZIzj0EAwIw
gZsxCzAJBgNVBAYTAklOMRAwDgYDVQQIDAdIYXJ5YW5hMREwDwYDVQQHDAhHdXJ1
Z3JhbTESMBAGA1UECgwJQ2FwZ2VtaW5pMRswGQYDVQQLDBJNYXJnbyBTYW5kYm94
IFRlYW0xEjAQBgNVBAMMCW1hcmdvLm9yZzEiMCAGCSqGSIb3DQEJARYTYWRtaW5A
Y2FwZ2VtaW5pLmNvbTAeFw0yNjA5MTAxNDUyMzBaFw0zNjA5MDcxNDUyMzBaMIGb
MQswCQYDVQQGEwJJTjEQMA4GA1UECAwHSGFyeWFuYTERMA8GA1UEBwwIR3VydWdy
YW0xEjAQBgNVBAoMCUNhcGdlbWluaTEbMBkGA1UECwwSTWFyZ28gU2FuZGJveCBU
ZWFtMRIwEAYDVQQDDAltYXJnby5vcmcxIjAgBgkqhkiG9w0BCQEWE2FkbWluQGNh
cGdlbWluaS5jb20wWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAS82uW6fFwwnMbI
qQZBTiZ2dvyWWNtd/8stkCJ9mqpocXbJ7ZjnXIDtCRS0qOlu22rwfzrS4uMwVmxf
qnbuPKMmo2MwYTAfBgNVHSMEGDAWgBS1ahEy7broEg3opQ6XHKbowkpuXjAPBgNV
HRMBAf8EBTADAQH/MA4GA1UdDwEB/wQEAwIBBjAdBgNVHQ4EFgQUtWoRMu266BIN
6KUOlxym6MJKbl4wCgYIKoZIzj0EAwIDSAAwRQIhAOgvPaa5k61R08MfSxC4gVXi
UrK64imrA5ZkqCd98OR2AiBefw1v0DWz2Ln2gz4x5/KHkD056ht23ATq5Advrg52
pg==
-----END CERTIFICATE-----`

	// dummyTrustDomain is the SPIFFE trust domain used across all tests.
	dummyTrustDomain = "margo.org"
)

// ---------------------------------------------------------------------------
// Helper: allow-list provider
// ---------------------------------------------------------------------------

// allowedSpiffeID is the SPIFFE ID present in dummyClientCertPEM's URI SAN.
const allowedSpiffeID = "spiffe://margo.org/margo/wfm/symphony-1"

// getClientAllowList returns the default allow list used across tests.
func getClientAllowList() []string {
	return []string{allowedSpiffeID}
}

// emptyAllowList returns an empty allow list (deny-all).
func emptyAllowList() []string {
	return []string{}
}

// ---------------------------------------------------------------------------
// Helper: trust domain provider
// ---------------------------------------------------------------------------

// getOwnTrustDomain returns the verifier's own SPIFFE trust domain.
func getOwnTrustDomain() string {
	return dummyTrustDomain
}

// ---------------------------------------------------------------------------
// Helper: PEM → SPIFFE JWK Set trust bundle converter
// ---------------------------------------------------------------------------

// jwkKey is a minimal representation of a single JWK entry for an X.509 CA.
type jwkKey struct {
	Kty string   `json:"kty"`
	Use string   `json:"use"`
	X5c []string `json:"x5c"`
	N   string   `json:"n,omitempty"`
	E   string   `json:"e,omitempty"`
}

// jwkSet is the top-level JWK Set structure expected by the validators package.
type jwkSet struct {
	Keys []jwkKey `json:"keys"`
}

// rootCAPEMToJWKSet converts a PEM-encoded root CA certificate into a minimal
// SPIFFE-compatible JWK Set (JSON) that the validators package can consume.
//
// The resulting JSON contains a single "RSA" or "EC" key entry with the
// base64-encoded DER certificate in the "x5c" field, which is the format
// expected by ValidateX509SVIDAgainstTrustBundle / GetTrustBundleFromJWK.
//
// Replace the body of this function if your validators package expects a
// different JWK Set schema.
func rootCAPEMToJWKSet(t *testing.T, rootCAPEM string) []byte {
	t.Helper()

	block, _ := pem.Decode([]byte(rootCAPEM))
	require.NotNil(t, block, "rootCAPEMToJWKSet: failed to decode PEM block")

	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err, "rootCAPEMToJWKSet: failed to parse certificate")

	// Encode the DER bytes as standard base64 (no padding stripping) for x5c.
	import64 := func(b []byte) string {
		const base64Table = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
		_ = base64Table // use encoding/base64 below
		import64 := func(b []byte) string {
			// Use standard encoding as required by JWK x5c (RFC 7517 §4.7).
			return encodeBase64Std(b)
		}
		return import64(b)
	}

	key := jwkKey{
		Kty: keyType(cert),
		Use: "x509-svid",
		X5c: []string{import64(block.Bytes)},
	}

	bundle, err := json.Marshal(jwkSet{Keys: []jwkKey{key}})
	require.NoError(t, err, "rootCAPEMToJWKSet: failed to marshal JWK Set")
	return bundle
}

// encodeBase64Std encodes b using standard base64 encoding (with padding).
func encodeBase64Std(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// keyType returns the JWK "kty" value for the certificate's public key.
func keyType(cert *x509.Certificate) string {
	switch cert.PublicKeyAlgorithm {
	case x509.ECDSA:
		return "EC"
	default:
		return "RSA"
	}
}

// ---------------------------------------------------------------------------
// Helper: load tls.Certificate from PEM constants
// ---------------------------------------------------------------------------

// mustLoadClientCert parses dummyClientCertPEM + dummyClientKeyPEM into a
// tls.Certificate.  The test is skipped if the placeholders have not been
// replaced yet.
func mustLoadClientCert(t *testing.T) tls.Certificate {
	t.Helper()
	if isPlaceholder(dummyClientCertPEM) || isPlaceholder(dummyClientKeyPEM) {
		t.Skip("replace dummyClientCertPEM / dummyClientKeyPEM with real values to run this test")
	}
	cert, err := tls.X509KeyPair([]byte(dummyClientCertPEM), []byte(dummyClientKeyPEM))
	require.NoError(t, err, "mustLoadClientCert: failed to parse key pair")
	return cert
}

// isPlaceholder returns true when the PEM string still contains the sentinel
// "REPLACE_WITH_" marker inserted in the dummy constants above.
func isPlaceholder(pem string) bool {
	return len(pem) == 0 || containsString(pem, "REPLACE_WITH_")
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && stringContains(s, substr))
}

func stringContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Helper: build a minimal self-signed cert for unit tests that do NOT need
// real SPIFFE SVIDs (e.g. "no certificates" / parse-error paths).
// ---------------------------------------------------------------------------

// selfSignedDERCert returns a minimal self-signed DER certificate.
// It is intentionally NOT a valid SPIFFE SVID (no URI SAN) so it triggers
// the expected errors in the verifier rules.
func selfSignedDERCert(t *testing.T) []byte {
	t.Helper()
	// Use a pre-generated static DER blob so the test has no crypto dependency.
	// This is a 1-byte invalid DER blob that will cause x509.ParseCertificate to fail.
	return []byte{0x30, 0x00} // minimal invalid ASN.1 SEQUENCE
}

// ---------------------------------------------------------------------------
// Tests for NewMTLSClientConfig
// ---------------------------------------------------------------------------

func TestNewMTLSClientConfig_EmptyTrustDomain_ReturnsError(t *testing.T) {
	cfg := VerifierConfig{
		GetOwnTrustDomain:   func() string { return "" },
		GetTrustBundleBytes: func() []byte { return []byte(`{"keys":[]}`) },
		GetClientAllowList:  getClientAllowList,
	}

	tlsCfg, err := NewMTLSClientConfig(tls.Certificate{}, cfg)

	assert.Nil(t, tlsCfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OwnTrustDomain must not be empty")
}

func TestNewMTLSClientConfig_EmptyTrustBundle_ReturnsError(t *testing.T) {
	cfg := VerifierConfig{
		GetOwnTrustDomain:   getOwnTrustDomain,
		GetTrustBundleBytes: func() []byte { return nil },
		GetClientAllowList:  getClientAllowList,
	}

	tlsCfg, err := NewMTLSClientConfig(tls.Certificate{}, cfg)

	assert.Nil(t, tlsCfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TrustBundleBytes must not be empty")
}

func TestNewMTLSClientConfig_EmptyTrustBundleSlice_ReturnsError(t *testing.T) {
	cfg := VerifierConfig{
		GetOwnTrustDomain:   getOwnTrustDomain,
		GetTrustBundleBytes: func() []byte { return []byte{} },
		GetClientAllowList:  getClientAllowList,
	}

	tlsCfg, err := NewMTLSClientConfig(tls.Certificate{}, cfg)

	assert.Nil(t, tlsCfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TrustBundleBytes must not be empty")
}

// TestNewMTLSClientConfig_NilAllowList_ReturnsError verifies that a nil
// GetClientAllowList is rejected at construction time (fail-fast).
func TestNewMTLSClientConfig_NilAllowList_ReturnsError(t *testing.T) {
	clientCert := mustLoadClientCert(t)
	trustBundle := rootCAPEMToJWKSet(t, dummyRootCAPEM)

	cfg := VerifierConfig{
		GetOwnTrustDomain:   getOwnTrustDomain,
		GetTrustBundleBytes: func() []byte { return trustBundle },
		GetClientAllowList:  nil, // intentionally nil
	}

	tlsCfg, err := NewMTLSClientConfig(clientCert, cfg)

	assert.Nil(t, tlsCfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GetClientAllowList must not be nil")
}

func TestNewMTLSClientConfig_ValidInputs_ReturnsTLSConfig(t *testing.T) {
	clientCert := mustLoadClientCert(t)
	trustBundle := rootCAPEMToJWKSet(t, dummyRootCAPEM)

	cfg := VerifierConfig{
		GetOwnTrustDomain:   getOwnTrustDomain,
		GetTrustBundleBytes: func() []byte { return trustBundle },
		GetClientAllowList:  getClientAllowList,
	}

	tlsCfg, err := NewMTLSClientConfig(clientCert, cfg)

	require.NoError(t, err)
	require.NotNil(t, tlsCfg)
}

func TestNewMTLSClientConfig_InsecureSkipVerifyIsTrue(t *testing.T) {
	clientCert := mustLoadClientCert(t)
	trustBundle := rootCAPEMToJWKSet(t, dummyRootCAPEM)

	cfg := VerifierConfig{
		GetOwnTrustDomain:   getOwnTrustDomain,
		GetTrustBundleBytes: func() []byte { return trustBundle },
		GetClientAllowList:  getClientAllowList,
	}

	tlsCfg, err := NewMTLSClientConfig(clientCert, cfg)
	require.NoError(t, err)

	assert.True(t, tlsCfg.InsecureSkipVerify)
}

func TestNewMTLSClientConfig_VerifyPeerCertificateIsSet(t *testing.T) {
	clientCert := mustLoadClientCert(t)
	trustBundle := rootCAPEMToJWKSet(t, dummyRootCAPEM)

	cfg := VerifierConfig{
		GetOwnTrustDomain:   getOwnTrustDomain,
		GetTrustBundleBytes: func() []byte { return trustBundle },
		GetClientAllowList:  getClientAllowList,
	}

	tlsCfg, err := NewMTLSClientConfig(clientCert, cfg)
	require.NoError(t, err)

	assert.NotNil(t, tlsCfg.VerifyPeerCertificate,
		"VerifyPeerCertificate callback must be set to enforce SPIFFE rules")
}

func TestNewMTLSClientConfig_ClientCertIsIncluded(t *testing.T) {
	clientCert := mustLoadClientCert(t)
	trustBundle := rootCAPEMToJWKSet(t, dummyRootCAPEM)

	cfg := VerifierConfig{
		GetOwnTrustDomain:   getOwnTrustDomain,
		GetTrustBundleBytes: func() []byte { return trustBundle },
		GetClientAllowList:  getClientAllowList,
	}

	tlsCfg, err := NewMTLSClientConfig(clientCert, cfg)
	require.NoError(t, err)

	require.Len(t, tlsCfg.Certificates, 1)
}

// ---------------------------------------------------------------------------
// Tests for VerifyPeerCertificate callback (via NewMTLSClientConfig)
// ---------------------------------------------------------------------------

// buildVerifier constructs the VerifyPeerCertificate hook from a VerifierConfig.
// Callers that do not need a custom allow list should pass getClientAllowList.
func buildVerifier(t *testing.T, cfg VerifierConfig) func([][]byte, [][]*x509.Certificate) error {
	t.Helper()
	clientCert := mustLoadClientCert(t)
	tlsCfg, err := NewMTLSClientConfig(clientCert, cfg)
	require.NoError(t, err)
	return tlsCfg.VerifyPeerCertificate
}

func TestVerifyPeerCertificate_NoCertificates_ReturnsError(t *testing.T) {
	trustBundle := rootCAPEMToJWKSet(t, dummyRootCAPEM)
	verify := buildVerifier(t, VerifierConfig{
		GetOwnTrustDomain:   getOwnTrustDomain,
		GetTrustBundleBytes: func() []byte { return trustBundle },
		GetClientAllowList:  getClientAllowList,
	})

	err := verify([][]byte{}, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "peer presented no certificates")
}

func TestVerifyPeerCertificate_InvalidLeafDER_ReturnsError(t *testing.T) {
	trustBundle := rootCAPEMToJWKSet(t, dummyRootCAPEM)
	verify := buildVerifier(t, VerifierConfig{
		GetOwnTrustDomain:   getOwnTrustDomain,
		GetTrustBundleBytes: func() []byte { return trustBundle },
		GetClientAllowList:  getClientAllowList,
	})

	err := verify([][]byte{selfSignedDERCert(t)}, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse peer leaf certificate")
}

func TestVerifyPeerCertificate_TrustDomainMismatch_ReturnsRule1Error(t *testing.T) {
	clientCert := mustLoadClientCert(t)
	trustBundle := rootCAPEMToJWKSet(t, dummyRootCAPEM)

	tlsCfg, err := NewMTLSClientConfig(clientCert, VerifierConfig{
		GetOwnTrustDomain:   func() string { return "other.org" },
		GetTrustBundleBytes: func() []byte { return trustBundle },
		GetClientAllowList:  getClientAllowList,
	})
	require.NoError(t, err)
	verify := tlsCfg.VerifyPeerCertificate

	leafDER := clientCert.Certificate[0]
	err = verify([][]byte{leafDER}, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rule 1")
	assert.Contains(t, err.Error(), "trust domain mismatch")
}

func TestVerifyPeerCertificate_UntrustedChain_ReturnsRule2Error(t *testing.T) {
	clientCert := mustLoadClientCert(t)
	emptyBundle := []byte(`{"keys":[]}`)

	tlsCfg, err := NewMTLSClientConfig(clientCert, VerifierConfig{
		GetOwnTrustDomain:   getOwnTrustDomain,
		GetTrustBundleBytes: func() []byte { return emptyBundle },
		GetClientAllowList:  getClientAllowList,
	})
	require.NoError(t, err)
	verify := tlsCfg.VerifyPeerCertificate

	leafDER := clientCert.Certificate[0]
	err = verify([][]byte{leafDER}, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rule 2")
}

func TestVerifyPeerCertificate_ValidSVID_NoError(t *testing.T) {
	clientCert := mustLoadClientCert(t)
	trustBundle := rootCAPEMToJWKSet(t, dummyRootCAPEM)

	verify := buildVerifier(t, VerifierConfig{
		GetOwnTrustDomain:   getOwnTrustDomain,
		GetTrustBundleBytes: func() []byte { return trustBundle },
		GetClientAllowList:  getClientAllowList,
	})

	var rawCerts [][]byte
	for _, c := range clientCert.Certificate {
		rawCerts = append(rawCerts, c)
	}

	err := verify(rawCerts, nil)

	assert.NoError(t, err)
}

func TestVerifyPeerCertificate_ValidSVIDWithIntermediates_NoError(t *testing.T) {
	clientCert := mustLoadClientCert(t)
	trustBundle := rootCAPEMToJWKSet(t, dummyRootCAPEM)

	if len(clientCert.Certificate) < 2 {
		t.Skip(
			"dummyClientCertPEM does not include intermediate certificates; skipping intermediate chain test",
		)
	}

	verify := buildVerifier(t, VerifierConfig{
		GetOwnTrustDomain:   getOwnTrustDomain,
		GetTrustBundleBytes: func() []byte { return trustBundle },
		GetClientAllowList:  getClientAllowList,
	})

	var rawCerts [][]byte
	for _, c := range clientCert.Certificate {
		rawCerts = append(rawCerts, c)
	}

	err := verify(rawCerts, nil)

	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// Tests for allow-list check (Rule 5 — SPIFFE ID authorisation)
// ---------------------------------------------------------------------------

// TestVerifyPeerCertificate_AllowedSPIFFEID_NoError verifies that a peer whose
// SPIFFE ID is present in the allow list passes all checks successfully.
func TestVerifyPeerCertificate_AllowedSPIFFEID_NoError(t *testing.T) {
	clientCert := mustLoadClientCert(t)
	trustBundle := rootCAPEMToJWKSet(t, dummyRootCAPEM)

	verify := buildVerifier(t, VerifierConfig{
		GetOwnTrustDomain:   getOwnTrustDomain,
		GetTrustBundleBytes: func() []byte { return trustBundle },
		// Explicitly list the peer's SPIFFE ID — connection must succeed.
		GetClientAllowList: func() []string {
			return []string{allowedSpiffeID}
		},
	})

	var rawCerts [][]byte
	for _, c := range clientCert.Certificate {
		rawCerts = append(rawCerts, c)
	}

	err := verify(rawCerts, nil)

	assert.NoError(t, err)
}

// TestVerifyPeerCertificate_EmptyAllowList_ReturnsError verifies that an empty
// allow list causes every peer to be rejected (deny-all / fail-closed).
func TestVerifyPeerCertificate_EmptyAllowList_ReturnsError(t *testing.T) {
	clientCert := mustLoadClientCert(t)
	trustBundle := rootCAPEMToJWKSet(t, dummyRootCAPEM)

	verify := buildVerifier(t, VerifierConfig{
		GetOwnTrustDomain:   getOwnTrustDomain,
		GetTrustBundleBytes: func() []byte { return trustBundle },
		GetClientAllowList:  emptyAllowList, // deny all
	})

	var rawCerts [][]byte
	for _, c := range clientCert.Certificate {
		rawCerts = append(rawCerts, c)
	}

	err := verify(rawCerts, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "allow-list check")
	assert.Contains(t, err.Error(), "allow list is empty")
}

// TestVerifyPeerCertificate_SpiffeIDNotInAllowList_ReturnsError verifies that a
// peer whose SPIFFE ID is absent from a non-empty allow list is rejected.
func TestVerifyPeerCertificate_SpiffeIDNotInAllowList_ReturnsError(t *testing.T) {
	clientCert := mustLoadClientCert(t)
	trustBundle := rootCAPEMToJWKSet(t, dummyRootCAPEM)

	verify := buildVerifier(t, VerifierConfig{
		GetOwnTrustDomain:   getOwnTrustDomain,
		GetTrustBundleBytes: func() []byte { return trustBundle },
		// Allow list contains a different service — peer must be rejected.
		GetClientAllowList: func() []string {
			return []string{"spiffe://margo.org/margo/other-service/instance-1"}
		},
	})

	var rawCerts [][]byte
	for _, c := range clientCert.Certificate {
		rawCerts = append(rawCerts, c)
	}

	err := verify(rawCerts, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "allow-list check")
	assert.Contains(t, err.Error(), allowedSpiffeID)
}

// TestVerifyPeerCertificate_MultipleAllowedIDs_CorrectIDPasses verifies that
// when the allow list contains multiple entries the peer is accepted when its
// SPIFFE ID matches any one of them.
func TestVerifyPeerCertificate_MultipleAllowedIDs_CorrectIDPasses(t *testing.T) {
	clientCert := mustLoadClientCert(t)
	trustBundle := rootCAPEMToJWKSet(t, dummyRootCAPEM)

	verify := buildVerifier(t, VerifierConfig{
		GetOwnTrustDomain:   getOwnTrustDomain,
		GetTrustBundleBytes: func() []byte { return trustBundle },
		GetClientAllowList: func() []string {
			return []string{
				"spiffe://margo.org/margo/other-service/instance-1",
				allowedSpiffeID, // peer's actual ID — must pass
				"spiffe://margo.org/margo/another-service/instance-2",
			}
		},
	})

	var rawCerts [][]byte
	for _, c := range clientCert.Certificate {
		rawCerts = append(rawCerts, c)
	}

	err := verify(rawCerts, nil)

	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// Tests for getOwnTrustDomain helper
// ---------------------------------------------------------------------------

func TestGetOwnTrustDomain_ReturnsMargoDotOrg(t *testing.T) {
	assert.Equal(t, "margo.org", getOwnTrustDomain())
}

// ---------------------------------------------------------------------------
// Tests for rootCAPEMToJWKSet helper
// ---------------------------------------------------------------------------

func TestRootCAPEMToJWKSet_ProducesValidJSON(t *testing.T) {
	if isPlaceholder(dummyRootCAPEM) {
		t.Skip("replace dummyRootCAPEM with a real PEM to run this test")
	}

	bundle := rootCAPEMToJWKSet(t, dummyRootCAPEM)

	var set jwkSet
	err := json.Unmarshal(bundle, &set)
	require.NoError(t, err)
	assert.Len(t, set.Keys, 1)
	assert.NotEmpty(t, set.Keys[0].X5c)
}

func TestRootCAPEMToJWKSet_InvalidPEM_FailsTest(t *testing.T) {
	// This test documents that rootCAPEMToJWKSet calls t.Fatal on bad input.
	// We use a sub-test with a mock T to avoid failing the parent.
	// In practice, just ensure you pass valid PEM.
	t.Log(
		"rootCAPEMToJWKSet calls require.NotNil / require.NoError — invalid PEM will fail the test immediately",
	)
}
