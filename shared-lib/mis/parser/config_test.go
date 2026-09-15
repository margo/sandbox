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
MIIBmjCCAT+gAwIBAgIQS+xxM/ZIy+evT75FOSTx+jAKBggqhkjOPQQDAjAUMRIw
EAYDVQQKEwltYXJnby5vcmcwHhcNMjYwOTEwMTM1MTIxWhcNMjYxMjA5MTM1MTIx
WjAUMRIwEAYDVQQKEwltYXJnby5vcmcwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNC
AASOl2/mbxKjkrQ8dsmahX4ct+3J64a7aEogoBMDGwm6GIrUOzwMF1CTP7EXqajC
sE4fq0uYQDRK87vCtKKYoQ6Vo3MwcTAOBgNVHQ8BAf8EBAMCB4AwHQYDVR0lBBYw
FAYIKwYBBQUHAwEGCCsGAQUFBwMCMAwGA1UdEwEB/wQCMAAwMgYDVR0RBCswKYYn
c3BpZmZlOi8vbWFyZ28ub3JnL21hcmdvL3dmbS9zeW1waG9ueS0xMAoGCCqGSM49
BAMCA0kAMEYCIQD6BvCAFB1p6qIhaNab5Tcr1mZrMIAPVYmI5KRHsfuywAIhAIx1
2B3ZnapZMXdimq0gZT+uXB2g50MMAn9Yp+NCmHjX
-----END CERTIFICATE-----
`)
	validKeyPEM = []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIMa5hiTGl5Nh2AJqCcr6LaUzdMGVqQMh14PfEIMSSSABoAoGCCqGSM49
AwEHoUQDQgAEjpdv5m8So5K0PHbJmoV+HLftyeuGu2hKIKATAxsJuhiK1Ds8DBdQ
kz+xF6mowrBOH6tLmEA0SvO7wrSimKEOlQ==
-----END EC PRIVATE KEY-----
`)
	validCAPEM = []byte(`-----BEGIN CERTIFICATE-----
MIIGKTCCBBGgAwIBAgIUbWUeu5ORZV7nWDURJSdbXdCUXiUwDQYJKoZIhvcNAQEL
BQAwgZsxCzAJBgNVBAYTAklOMRAwDgYDVQQIDAdIYXJ5YW5hMREwDwYDVQQHDAhH
dXJ1Z3JhbTESMBAGA1UECgwJQ2FwZ2VtaW5pMRswGQYDVQQLDBJNYXJnbyBTYW5k
Ym94IFRlYW0xEjAQBgNVBAMMCW1hcmdvLm9yZzEiMCAGCSqGSIb3DQEJARYTYWRt
aW5AY2FwZ2VtaW5pLmNvbTAeFw0yNjA5MTAxMzQ5MTBaFw0zNjA5MDcxMzQ5MTBa
MIGbMQswCQYDVQQGEwJJTjEQMA4GA1UECAwHSGFyeWFuYTERMA8GA1UEBwwIR3Vy
dWdyYW0xEjAQBgNVBAoMCUNhcGdlbWluaTEbMBkGA1UECwwSTWFyZ28gU2FuZGJv
eCBUZWFtMRIwEAYDVQQDDAltYXJnby5vcmcxIjAgBgkqhkiG9w0BCQEWE2FkbWlu
QGNhcGdlbWluaS5jb20wggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQCN
PRn/Qziw+KrPpncm6bfG11fbTlYmRQbp1IoUGxQSvY3zK4qMa+kYVptNcHrRiv9Y
pkRGqf/LxAYPknQtpbK41Ifn59z7BSFrn2/r60ElKha3tauR+Fch/peEZ8se88wC
Of6Gm2LPjSSmsJjWU5v0o86XVXz7rp/7xcJUJ+NSAKorWs9eyd0+oPRUZCQp7n+K
sws5zvWAHTGxd8rTsh2XWXfkIIUI1J8AVJsGdIvMKGDUBkEW0FWSub+v4xZezcBu
ztdccBJCVMZyCZ0L0UReiSlFCya7q78jCkpRBbyDYbCd8wUac4syusHpcJN07t9X
T5zz15r/A3SHuJ9QGPrGGnisqzoQbDXR116sc9sn5GvF0Qgs1XajB6s2Lk8HqkKk
cYYRvPu/V6yQpF61u8D1V7Zfu+dLsZwXThmxyCnzcs15IkymQpRjBCCBJocoYHyt
I8zxeamDEuPQT1fhTJDvDnLTXhziqyzMYVoKtLD+ds/YAwBoWaGbiaRzbAciprpY
DsQCRVnAKCmLnRvAiD/7Uzh/qNytz1KJ4vQiUTD53mCr9B4hm2dsPsXCLpjxdXjT
Ts7pqZL1Syq9XLFRsY5nZq3oksSdCp+boZLc7xvAAcFfg5pzkqs9PYlCiSbYx6S1
kaSAP5KctTLTEFI2GBHbPA/tVxxuylf6TlSQxBFMEwIDAQABo2MwYTAfBgNVHSME
GDAWgBSNGm14DgSwIgyBx4x8xZVgPzIBbzAPBgNVHRMBAf8EBTADAQH/MA4GA1Ud
DwEB/wQEAwIBBjAdBgNVHQ4EFgQUjRpteA4EsCIMgceMfMWVYD8yAW8wDQYJKoZI
hvcNAQELBQADggIBAEZRe4sHKr9RDFd9hCuBXLSeWFhDhLbxgOFHjCOEXdQCaZAR
i3scuV+fEbhYsxi0IaCTttTxQli5cR6D05jfXoFycL9oWAwPNrj2eE6jRODe8yjj
bHWpPZxiaAxLh9/GlzR8Sj/NHgYpleWr/oSNYg3kyxSXdQyBASD1lJTHj4dHvtfT
Y99JFwkeLhZ5y3lihte1WOK4zAP2EgOdSdUn22S9znvfCX0eRLS8OSpFf7ITZqfl
EzY8Fg1SYmV++wVq4zCaYSCcIE8IpC8MZnQDHMMguhPyNi6CtEJfYkKts4f/A92c
ijIO7o+BDmtbCeM17VizjeH40te4aiVPxK5qQIcWhs4vyUE0O4vZq0aoBchmwNfV
kjfp5Y7DFSSXkRBUvjUOqgEbOHLnSxazuY0CHsKGItJZ06besDajCXz0jrJdo8Ij
E8d2Gcf+pE9zBkU32e+V6h07KXNPYS/eRLHszX0Lbo783hKWYqY7fXs40bu1nksn
bcFBlwMUNmPxHAIxAD3VYLLM80jNVKKMJiubR8p/3nI0HRsB01FUJFd0Jsd7wxzc
Da0vtZns/6k8sa+UW/LjLTnqDsMNRK3f1+Lq+chsaPhQSIjJfzBZn6AXGpFxBWT4
gUsr2t4uQ5TMhU+mGQq7AYLCrOgFiAp+1uU+FqhEHwm2XJt7jISDBpf/eMMv
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
