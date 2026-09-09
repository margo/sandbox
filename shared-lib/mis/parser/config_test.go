package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func writeTempFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, content, 0o600))
	return path
}

// Fixtures – replace with real PEM bytes that pass your validators.
// These are intentionally kept as constants so tests are self-contained.
var (
	validCertPEM = []byte(`-----BEGIN CERTIFICATE-----
MIIBrTCCAVOgAwIBAgIQLZTcXrnbxIO6MdWxtQr//DAKBggqhkjOPQQDAjAUMRIw
EAYDVQQKEwltYXJnby5jb20wHhcNMjYwOTAzMTMwOTUxWhcNMjYxMjAyMTMwOTUx
WjAUMRIwEAYDVQQKEwltYXJnby5jb20wWTATBgcqhkjOPQIBBggqhkjOPQMBBwNC
AAT16Wgvj5nQB67tsU6lFz3AGEuF/VYKtCuZkBvZ6YgOxpO8yBM+n19rRl8iWm6x
BOJp/kBZA3uq82xYrHteFz3No4GGMIGDMA4GA1UdDwEB/wQEAwIChDAdBgNVHSUE
FjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwDAYDVR0TAQH/BAIwADBEBgNVHREEPTA7
ghBzeW1waG9ueS5tYWNoaW5lhidzcGlmZmU6Ly9tYXJnby5jb20vbWFyZ28vd2Zt
L3N5bXBob255LTEwCgYIKoZIzj0EAwIDSAAwRQIgZC7r2XAuKawRCaPv8WTnK5Jx
2M9Cq0BsVsGLx0gLAB4CIQD4YB23AvLg6+dy9maX0Ygy6qdKX+QPyKhrSsaZdty2
hA==
-----END CERTIFICATE-----
`)
	validKeyPEM = []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIPL1E95PGphDhCaM/t8yB+YwALJrEVsfcSLsYeB5UqosoAoGCCqGSM49
AwEHoUQDQgAE9eloL4+Z0Aeu7bFOpRc9wBhLhf1WCrQrmZAb2emIDsaTvMgTPp9f
a0ZfIlpusQTiaf5AWQN7qvNsWKx7Xhc9zQ==
-----END EC PRIVATE KEY-----
`)
	validCAPEM = []byte(`-----BEGIN CERTIFICATE-----
MIIGOzCCBCOgAwIBAgIUCvkrG7i+pRjjfrxiH2xEeHyM0kEwDQYJKoZIhvcNAQEL
BQAwgaQxCzAJBgNVBAYTAklOMRAwDgYDVQQIDAdIYXJ5YW5hMREwDwYDVQQHDAhH
dXJ1Z3JhbTESMBAGA1UECgwJQ2FwZ2VtaW5pMRswGQYDVQQLDBJNYXJnbyBTYW5k
Ym94IFRlYW0xGzAZBgNVBAMMEkNhcGdlbWluaSBIVFRQUyBDQTEiMCAGCSqGSIb3
DQEJARYTYWRtaW5AY2FwZ2VtaW5pLmNvbTAeFw0yNjA5MDcwOTMyMTZaFw0zNjA5
MDQwOTMyMTZaMIGkMQswCQYDVQQGEwJJTjEQMA4GA1UECAwHSGFyeWFuYTERMA8G
A1UEBwwIR3VydWdyYW0xEjAQBgNVBAoMCUNhcGdlbWluaTEbMBkGA1UECwwSTWFy
Z28gU2FuZGJveCBUZWFtMRswGQYDVQQDDBJDYXBnZW1pbmkgSFRUUFMgQ0ExIjAg
BgkqhkiG9w0BCQEWE2FkbWluQGNhcGdlbWluaS5jb20wggIiMA0GCSqGSIb3DQEB
AQUAA4ICDwAwggIKAoICAQDoYJVMetPVo+AQ61fYgnQMsqw2ThldxFZfOBByUL48
v+VYTSNeSNsTw72RQkR0VgO0NZyWTwEeAc47ejCXA/2pksnqplM9GwCUbR+wHYV3
HhCamzOb657WUyqmqmm/ULB5DJZji2iIWitlS9ci5A5mHzrR9cMS9q1UugxAr82R
oWsThus6xOZ+m+z7ej9ZPVnWCmT/1OWuJhHdMmRshDbnWC1a9nX9tKTk75OYlYtS
827z0d53TGtjvtk7iGJx64xB9sk0YB5rfSYVVuhCqxKLd5wJLTOgTkGBY4opToA8
gih3h3tET/X0aOp7+yBsve6K5KJ6NWpOlOg3UHaf1W1RxECKfBwYnBCFwHCaNct4
Zoi7IWz2zK+EHwKrbOXAYWkDa1etd+dwA+xcwnI/q5s512jk4+YaXrpkgegML737
SPfENxsUXUSTlfEi4RDB6oQgoPouJ8oaHXEndm0vS8VS/BCpUujuuxd7EHGgfHzr
pxCWjZipi5NyJeI9OdOX8AeOcfkUgwiHEiSdl5wOc9Twea9/+6EZzb2yD+cE64rY
E7SzwkMNe43vM/2KYx4F5tGWz7QmP9PucclwTiyMXm3niAaY+CHyrBmM462V2Xk4
IfLEu5MOEasQ/IxqKBq09mzmUJZ3b5uegTNDb9OkZDCgbPnWrRI3wAhPrZp/acD+
BQIDAQABo2MwYTAfBgNVHSMEGDAWgBQeV7L73v/vmgBEOlJWICMCb9WbyzAPBgNV
HRMBAf8EBTADAQH/MA4GA1UdDwEB/wQEAwIBBjAdBgNVHQ4EFgQUHley+97/75oA
RDpSViAjAm/Vm8swDQYJKoZIhvcNAQELBQADggIBAD4KtDeZmft3er/hV+3Q8wEc
UYaeGGeWkfrGhFxCqzK6PfVei29JBgtPeWWzvToOvcPRgBstCyYUSNmelrcQLgQt
prwXcLMH/8YGfhdhPT6jT5vO0xFRKyEIR0URu28950PRKmyReJFBKZMx18XWUXw2
SZ2aXhiWNV9Fmq5lwlXgzsJse3GBrQGE+WzKc9Kbs0vPdTbV9ViZeUxWavekfET/
hzn0bsgMpzChb37e+M3/4/9Jihh40gTwiaJTzDQB+LlrO4YhDlODSLSg+dXyOf49
lxFj7FQDW+en/OggIgEKVKXCEXZNhXNcaTfkIgFFPrlFiF9lb+skwvNSXw6DQdZ7
8fiUSFoWwhGCmXfRT9cWDrSfV7oqUP3B5i1+BTVT/k4QpnMh4JujV7DB2kS4Moim
ySLPL+gmD5MNkziir/amJM/HJCpEcKofdJsV+9K7jMptyWI93OO0SFaIa5hIXpa0
y340F/Qg+I5X6xZfErw9yzH2AlcRBegfCttvnVrwjeRT4Gn1gxZjOqpGcbf/tIfg
hLdcDlIBV5gTDGoP8Pc1Qlaactnz5iB9ancuS1fVLFDVwvzDcYdUQN6wd6Q+X1fL
XCX2zVhOMtJK4TR7JGnVh/RtdbWAb7+RaIov9vKjloINuIBjAAOln9UVGUP39k1l
ygXsCNlqnR3ygi38rZAe
-----END CERTIFICATE-----
`)
	validTrustBundle = []byte(
		`{ "keys": [ { "use": "x509-svid", "kty": "RSA", "n": "uf9PzM_8FN6uOFNrp-igvKO1vHK8X-HV9ZvG0U4zdy0kLiQEbYQxTpkbcahkgQz7q9eOLFCkKnJu4qwoV5U0SOoXqlm-iGu-5_pHs9yW5sGvvNo01bFx2-W66lPb7cbBhPQcBMhEbo8q4wF2zXb-CzOsyVD266MHGhcrKHYEAQkt92CvfrzNrv2M_eL272IXceJVhwHVtUwMgZzHtIIqbtTnfDFlfz9D75mNN_H2q6Au8vNrMaFlPrEmDO3GkDiGvUmx5KMJyd6xbxUhsyNK2j-tQXsun2KbkvKzMAaN3lJYtEt3WJjLf0Vbzfw_d7KAULaeqAl-412jhHGAiY-9A2a1vduiwyaA77FLKGxAvaVMWfMo_X9n4gqVtV5gu5n_1MPEwPOzq7ifQL_rgocNrZhzx9auTDaXD9djs3IDXHnL2SwBK7wq23wo2MgDTNGijf_hJGNJhdqYHp4vr5gIAw79sOFKnW4iVfw66c32HdhftFS0KUsizZaDSKGIRv6O3hKrJfMh0mRYx1PSQqQNEshPs7RSZmZCZ3fnG0onMRD6p7PnuDUHZjEJ5mp8JhG-wtySOf-lNaKCtEkP0pYwVDH5U4BR7TtjTdvWjhmcHJLiI5f5PLecDb2Pkt-8qQbL7ZftycpVRROI7UhMc3Ewufdz6pvICmhASsi3_1qalI0", "e": "AQAB", "x5c": [ "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" ] } ] }`,
	)
	validSpiffeIDs = mustMarshalJSON([]string{"spiffe://example.org/margo/wfm/device-1"})
)

func mustMarshalJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// buildValidInput creates a fully-populated MIAFInput backed by temp files.
func buildValidInput(t *testing.T, dir string) MIAFInput {
	t.Helper()
	return MIAFInput{
		X509: MIAFx509Input{
			CertPath: writeTempFile(t, dir, "cert.pem", validCertPEM),
			KeyPath:  writeTempFile(t, dir, "key.pem", validKeyPEM),
		},
		MIS: MISInput{
			Endpoint:    "https://mis.example.org",
			CAPath:      writeTempFile(t, dir, "ca.pem", validCAPEM),
			TrustDomain: "example.org",
			TrustBundle: &TrustBundleInput{
				URI:  "https://mis.example.org/bundle",
				Path: writeTempFile(t, dir, "bundle.json", validTrustBundle),
			},
		},
		AuthzPath: writeTempFile(t, dir, "authz.json", validSpiffeIDs),
	}
}

// ── happy path ────────────────────────────────────────────────────────────────

func TestParseMIAFConfig_ValidInput_ReturnsPopulatedConfig(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)

	got, err := ParseMIAFConfig(input, "wfm")

	require.NoError(t, err)
	assert.Equal(t, validCertPEM, got.X509.CertPEM)
	assert.Equal(t, validKeyPEM, got.X509.KeyPEM)
	assert.Equal(t, "https://mis.example.org", got.MIS.Endpoint)
	assert.Equal(t, validCAPEM, got.MIS.CAPEM)
	assert.Equal(t, "example.org", got.MIS.TrustDomain)
	require.NotNil(t, got.MIS.TrustBundle)
	assert.Equal(t, "https://mis.example.org/bundle", got.MIS.TrustBundle.URI)
	assert.Equal(t, validTrustBundle, got.MIS.TrustBundle.BundleJSON)
	assert.Equal(t, []string{"spiffe://example.org/margo/wfm/device-1"}, got.AuthorizedSPIFFEIDs)
}

func TestParseMIAFConfig_OptionalX509Paths_Omitted(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	input.X509.CertPath = ""
	input.X509.KeyPath = ""

	got, err := ParseMIAFConfig(input, "wfm")

	require.NoError(t, err)
	assert.Nil(t, got.X509.CertPEM)
	assert.Nil(t, got.X509.KeyPEM)
}

func TestParseMIAFConfig_NilTrustBundle_Allowed(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	input.MIS.TrustBundle = nil

	got, err := ParseMIAFConfig(input, "wfm")

	require.NoError(t, err)
	assert.Nil(t, got.MIS.TrustBundle)
}

func TestParseMIAFConfig_TrustBundleWithoutPath_URIPassedThrough(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	input.MIS.TrustBundle = &TrustBundleInput{
		URI:  "https://mis.example.org/bundle",
		Path: "", // no local file
	}

	got, err := ParseMIAFConfig(input, "wfm")

	require.NoError(t, err)
	require.NotNil(t, got.MIS.TrustBundle)
	assert.Equal(t, "https://mis.example.org/bundle", got.MIS.TrustBundle.URI)
	assert.Nil(t, got.MIS.TrustBundle.BundleJSON)
}

func TestParseMIAFConfig_AuthzFileWithBlankEntries_FiltersThemOut(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	ids := mustMarshalJSON([]string{
		"  ",
		"spiffe://example.org/margo/wfm/device-1",
		"",
	})
	input.AuthzPath = writeTempFile(t, dir, "authz_blanks.json", ids)

	got, err := ParseMIAFConfig(input, "wfm")

	require.NoError(t, err)
	assert.Equal(t, []string{"spiffe://example.org/margo/wfm/device-1"}, got.AuthorizedSPIFFEIDs)
}

func TestParseMIAFConfig_EndpointPassedThrough(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	input.MIS.Endpoint = "grpc://custom-endpoint:8443"

	got, err := ParseMIAFConfig(input, "wfm")

	require.NoError(t, err)
	assert.Equal(t, "grpc://custom-endpoint:8443", got.MIS.Endpoint)
}

// ── X.509 certificate errors ──────────────────────────────────────────────────

func TestParseMIAFConfig_CertPathNotFound_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	input.X509.CertPath = filepath.Join(dir, "nonexistent.pem")

	_, err := ParseMIAFConfig(input, "wfm")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.x509.certPath")
}

func TestParseMIAFConfig_CertFileEmpty_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	input.X509.CertPath = writeTempFile(t, dir, "empty_cert.pem", []byte("   "))

	_, err := ParseMIAFConfig(input, "wfm")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.x509.certPath")
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestParseMIAFConfig_CertFileInvalidPEM_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	input.X509.CertPath = writeTempFile(t, dir, "bad_cert.pem", []byte("not-a-cert"))

	_, err := ParseMIAFConfig(input, "wfm")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.x509.certPath")
	assert.Contains(t, err.Error(), "not a valid x509 SVID certificate")
}

// ── X.509 private key errors ──────────────────────────────────────────────────

func TestParseMIAFConfig_KeyPathNotFound_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	input.X509.KeyPath = filepath.Join(dir, "nonexistent_key.pem")

	_, err := ParseMIAFConfig(input, "wfm")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.x509.keyPath")
}

func TestParseMIAFConfig_KeyFileEmpty_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	input.X509.KeyPath = writeTempFile(t, dir, "empty_key.pem", []byte("  "))

	_, err := ParseMIAFConfig(input, "wfm")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.x509.keyPath")
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestParseMIAFConfig_KeyFileInvalidPEM_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	input.X509.KeyPath = writeTempFile(t, dir, "bad_key.pem", []byte("not-a-key"))

	_, err := ParseMIAFConfig(input, "wfm")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.x509.keyPath")
	assert.Contains(t, err.Error(), "not a valid x509 SVID certificate key")
}

// ── MIS CA certificate errors ─────────────────────────────────────────────────

func TestParseMIAFConfig_CAPathNotFound_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	input.MIS.CAPath = filepath.Join(dir, "nonexistent_ca.pem")

	_, err := ParseMIAFConfig(input, "wfm")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.mis.caPath")
}

func TestParseMIAFConfig_CAFileEmpty_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	input.MIS.CAPath = writeTempFile(t, dir, "empty_ca.pem", []byte("  "))

	_, err := ParseMIAFConfig(input, "wfm")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.mis.caPath")
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestParseMIAFConfig_CAFileInvalidPEM_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	input.MIS.CAPath = writeTempFile(t, dir, "bad_ca.pem", []byte("not-a-ca"))

	_, err := ParseMIAFConfig(input, "wfm")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.mis.caPath")
	assert.Contains(t, err.Error(), "must be a valid ca certificate")
}

// ── trust domain errors ───────────────────────────────────────────────────────

func TestParseMIAFConfig_InvalidTrustDomain_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	input.MIS.TrustDomain = "INVALID DOMAIN!"

	_, err := ParseMIAFConfig(input, "wfm")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.mis.trustDomain")
	assert.Contains(t, err.Error(), "must be valid")
}

// ── trust bundle errors ───────────────────────────────────────────────────────

func TestParseMIAFConfig_TrustBundlePathNotFound_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	input.MIS.TrustBundle.Path = filepath.Join(dir, "nonexistent_bundle.json")

	_, err := ParseMIAFConfig(input, "wfm")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.mis.trustBundle.path")
}

func TestParseMIAFConfig_TrustBundleFileEmpty_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	input.MIS.TrustBundle.Path = writeTempFile(t, dir, "empty_bundle.json", []byte("  "))

	_, err := ParseMIAFConfig(input, "wfm")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.mis.trustBundle.path")
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestParseMIAFConfig_TrustBundleInvalidJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	input.MIS.TrustBundle.Path = writeTempFile(t, dir, "bad_bundle.json", []byte("not-json"))

	_, err := ParseMIAFConfig(input, "wfm")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.mis.trustBundle.path")
	assert.Contains(t, err.Error(), "valid SPIFFE trust bundle")
}

// ── authz path errors ─────────────────────────────────────────────────────────

func TestParseMIAFConfig_AuthzPathEmpty_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	input.AuthzPath = ""

	_, err := ParseMIAFConfig(input, "wfm")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.authzPath must not be empty")
}

func TestParseMIAFConfig_AuthzPathNotFound_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	input.AuthzPath = filepath.Join(dir, "nonexistent_authz.json")

	_, err := ParseMIAFConfig(input, "wfm")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.authzPath")
}

func TestParseMIAFConfig_AuthzFileInvalidJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	input.AuthzPath = writeTempFile(t, dir, "bad_authz.json", []byte("{not-an-array}"))

	_, err := ParseMIAFConfig(input, "wfm")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.authzPath")
	assert.Contains(t, err.Error(), "failed to parse JSON array")
}

func TestParseMIAFConfig_AuthzFileAllBlankEntries_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	input.AuthzPath = writeTempFile(t, dir, "blank_authz.json", mustMarshalJSON([]string{"", "  "}))

	_, err := ParseMIAFConfig(input, "wfm")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.authzPath")
	assert.Contains(t, err.Error(), "must contain at least one SPIFFE ID")
}

func TestParseMIAFConfig_AuthzFileEmptyArray_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	input := buildValidInput(t, dir)
	input.AuthzPath = writeTempFile(t, dir, "empty_authz.json", mustMarshalJSON([]string{}))

	_, err := ParseMIAFConfig(input, "wfm")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must contain at least one SPIFFE ID")
}

// ── table-driven: principal variants ─────────────────────────────────────────

func TestParseMIAFConfig_PrincipalVariants(t *testing.T) {
	tests := []struct {
		name      string
		principal string
		wantErr   bool
	}{
		{"valid wfm principal", "wfm", false},
		{"valid wfm-client principal", "wfm-client", true},
		{"unknown principal", "admin", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			// Use cert matching the principal under test; adjust fixture as needed.
			input := buildValidInput(t, dir)

			_, err := ParseMIAFConfig(input, tc.principal)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
