package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test Helpers ---

// writeFile creates a temp file with the given content and returns its path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// writeJSONFile marshals v into JSON and writes it to a temp file.
func writeJSONFile(t *testing.T, dir, name string, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return writeFile(t, dir, name, string(data))
}

// validInput builds a fully-populated MIAFInput backed by real temp files.
func validInput(t *testing.T, dir string) MIAFInput {
	t.Helper()
	return MIAFInput{
		X509: MIAFx509Input{
			CertPath: writeFile(t, dir, "cert.pem", "CERT_CONTENT"),
			KeyPath:  writeFile(t, dir, "key.pem", "KEY_CONTENT"),
		},
		MIS: MISInput{
			Endpoint:    "https://mis.margo.org:9443",
			CAPath:      writeFile(t, dir, "ca.crt", "CA_CONTENT"),
			TrustDomain: "margo.org",
			TrustBundle: &TrustBundleInput{
				URI:  "/.well-known/spiffe/bundle.json",
				Path: writeFile(t, dir, "trust-bundle.json", `{"keys":[]}`),
			},
		},
		AuthzPath: writeJSONFile(t, dir, "authorized.json", []string{
			"spiffe://margo.org/device/abc",
			"spiffe://margo.org/device/xyz",
		}),
	}
}

// --- Happy Path ---

func TestParseMIAFConfig_FullyPopulated(t *testing.T) {
	dir := t.TempDir()
	input := validInput(t, dir)

	got, err := ParseMIAFConfig(input)

	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, []byte("CERT_CONTENT"), got.X509.CertPEM)
	assert.Equal(t, []byte("KEY_CONTENT"), got.X509.KeyPEM)

	assert.Equal(t, "https://mis.margo.org:9443", got.MIS.Endpoint)
	assert.Equal(t, []byte("CA_CONTENT"), got.MIS.CAPEM)
	assert.Equal(t, "margo.org", got.MIS.TrustDomain)

	require.NotNil(t, got.MIS.TrustBundle)
	assert.Equal(t, "/.well-known/spiffe/bundle.json", got.MIS.TrustBundle.URI)
	assert.Equal(t, []byte(`{"keys":[]}`), got.MIS.TrustBundle.BundleJSON)

	assert.Equal(t, []string{
		"spiffe://margo.org/device/abc",
		"spiffe://margo.org/device/xyz",
	}, got.AuthorizedSPIFFEIDs)
}

// Static trust bundle mode: no endpoint/caPath, only trustBundle.path + trustDomain.
func TestParseMIAFConfig_StaticTrustBundleMode(t *testing.T) {
	dir := t.TempDir()
	input := MIAFInput{
		X509: MIAFx509Input{
			CertPath: writeFile(t, dir, "cert.pem", "CERT"),
			KeyPath:  writeFile(t, dir, "key.pem", "KEY"),
		},
		MIS: MISInput{
			TrustDomain: "margo.org",
			TrustBundle: &TrustBundleInput{
				Path: writeFile(t, dir, "trust-bundle.json", `{"keys":[]}`),
			},
		},
		AuthzPath: writeJSONFile(
			t,
			dir,
			"authorized.json",
			[]string{"spiffe://margo.org/device/abc"},
		),
	}

	got, err := ParseMIAFConfig(input)

	require.NoError(t, err)
	assert.Empty(t, got.MIS.Endpoint)
	assert.Nil(t, got.MIS.CAPEM)
	assert.NotNil(t, got.MIS.TrustBundle)
	assert.NotEmpty(t, got.MIS.TrustBundle.BundleJSON)
}

// --- Empty File Content Errors ---

func TestParseMIAFConfig_CertPathEmptyContent(t *testing.T) {
	dir := t.TempDir()
	input := validInput(t, dir)
	input.X509.CertPath = writeFile(t, dir, "empty-cert.pem", "")

	_, err := ParseMIAFConfig(input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.x509.certPath")
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestParseMIAFConfig_KeyPathEmptyContent(t *testing.T) {
	dir := t.TempDir()
	input := validInput(t, dir)
	input.X509.KeyPath = writeFile(t, dir, "empty-key.pem", "")

	_, err := ParseMIAFConfig(input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.x509.keyPath")
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestParseMIAFConfig_CAPathEmptyContent(t *testing.T) {
	dir := t.TempDir()
	input := validInput(t, dir)
	input.MIS.CAPath = writeFile(t, dir, "empty-ca.crt", "   ")

	_, err := ParseMIAFConfig(input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.mis.caPath")
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestParseMIAFConfig_TrustBundlePathEmptyContent(t *testing.T) {
	dir := t.TempDir()
	input := validInput(t, dir)
	input.MIS.TrustBundle.Path = writeFile(t, dir, "empty-bundle.json", "\t  \n")

	_, err := ParseMIAFConfig(input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.mis.trustBundle.path")
	assert.Contains(t, err.Error(), "must not be empty")
}

// Optional file fields left empty should produce nil byte slices, not errors.
func TestParseMIAFConfig_OptionalFileFieldsEmpty(t *testing.T) {
	dir := t.TempDir()
	input := MIAFInput{
		X509: MIAFx509Input{}, // both paths empty
		MIS: MISInput{
			Endpoint:    "https://mis.margo.org:9443",
			TrustDomain: "margo.org",
			// CAPath empty, TrustBundle nil
		},
		AuthzPath: writeJSONFile(
			t,
			dir,
			"authorized.json",
			[]string{"spiffe://margo.org/device/abc"},
		),
	}

	got, err := ParseMIAFConfig(input)

	require.NoError(t, err)
	assert.Nil(t, got.X509.CertPEM)
	assert.Nil(t, got.X509.KeyPEM)
	assert.Nil(t, got.MIS.CAPEM)
	assert.Nil(t, got.MIS.TrustBundle)
}

// TrustBundle configured with only URI (no local path) should not error.
func TestParseMIAFConfig_TrustBundleURIOnly(t *testing.T) {
	dir := t.TempDir()
	input := validInput(t, dir)
	input.MIS.TrustBundle = &TrustBundleInput{
		URI: "/.well-known/spiffe/bundle.json",
		// Path intentionally empty
	}

	got, err := ParseMIAFConfig(input)

	require.NoError(t, err)
	require.NotNil(t, got.MIS.TrustBundle)
	assert.Equal(t, "/.well-known/spiffe/bundle.json", got.MIS.TrustBundle.URI)
	assert.Nil(t, got.MIS.TrustBundle.BundleJSON)
}

// String-only fields must be passed through unchanged.
func TestParseMIAFConfig_PassThroughFields(t *testing.T) {
	dir := t.TempDir()
	input := validInput(t, dir)
	input.MIS.Endpoint = "https://custom-endpoint:1234"
	input.MIS.TrustDomain = "custom.domain"
	input.MIS.TrustBundle.URI = "https://custom.domain/bundle"

	got, err := ParseMIAFConfig(input)

	require.NoError(t, err)
	assert.Equal(t, "https://custom-endpoint:1234", got.MIS.Endpoint)
	assert.Equal(t, "custom.domain", got.MIS.TrustDomain)
	assert.Equal(t, "https://custom.domain/bundle", got.MIS.TrustBundle.URI)
}

// --- X.509 File Errors ---

func TestParseMIAFConfig_CertPathNotFound(t *testing.T) {
	dir := t.TempDir()
	input := validInput(t, dir)
	input.X509.CertPath = filepath.Join(dir, "nonexistent-cert.pem")

	_, err := ParseMIAFConfig(input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.x509.certPath")
}

func TestParseMIAFConfig_KeyPathNotFound(t *testing.T) {
	dir := t.TempDir()
	input := validInput(t, dir)
	input.X509.KeyPath = filepath.Join(dir, "nonexistent-key.pem")

	_, err := ParseMIAFConfig(input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.x509.keyPath")
}

// --- MIS CA File Errors ---

func TestParseMIAFConfig_CAPathNotFound(t *testing.T) {
	dir := t.TempDir()
	input := validInput(t, dir)
	input.MIS.CAPath = filepath.Join(dir, "nonexistent-ca.crt")

	_, err := ParseMIAFConfig(input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.mis.caPath")
}

// --- Trust Bundle File Errors ---

func TestParseMIAFConfig_TrustBundlePathNotFound(t *testing.T) {
	dir := t.TempDir()
	input := validInput(t, dir)
	input.MIS.TrustBundle.Path = filepath.Join(dir, "nonexistent-bundle.json")

	_, err := ParseMIAFConfig(input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.mis.trustBundle.path")
}

// --- AuthzPath Validation ---

func TestParseMIAFConfig_AuthzPathEmpty(t *testing.T) {
	dir := t.TempDir()
	input := validInput(t, dir)
	input.AuthzPath = ""

	_, err := ParseMIAFConfig(input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.authzPath")
}

func TestParseMIAFConfig_AuthzPathNotFound(t *testing.T) {
	dir := t.TempDir()
	input := validInput(t, dir)
	input.AuthzPath = filepath.Join(dir, "nonexistent-authz.json")

	_, err := ParseMIAFConfig(input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.authzPath")
}

func TestParseMIAFConfig_AuthzFileInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	input := validInput(t, dir)
	input.AuthzPath = writeFile(t, dir, "bad-authz.json", `not valid json`)

	_, err := ParseMIAFConfig(input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.authzPath")
	assert.Contains(t, err.Error(), "parse JSON")
}

func TestParseMIAFConfig_AuthzFileNotAnArray(t *testing.T) {
	dir := t.TempDir()
	input := validInput(t, dir)
	// JSON object instead of array
	input.AuthzPath = writeFile(t, dir, "obj-authz.json", `{"id":"spiffe://margo.org/device/abc"}`)

	_, err := ParseMIAFConfig(input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "miaf.authzPath")
}

func TestParseMIAFConfig_AuthzFileEmptyArray(t *testing.T) {
	dir := t.TempDir()
	input := validInput(t, dir)
	input.AuthzPath = writeJSONFile(t, dir, "empty-authz.json", []string{})

	_, err := ParseMIAFConfig(input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one SPIFFE ID")
}

func TestParseMIAFConfig_AuthzFileOnlyBlankEntries(t *testing.T) {
	dir := t.TempDir()
	input := validInput(t, dir)
	input.AuthzPath = writeJSONFile(t, dir, "blank-authz.json", []string{"", "   ", "\t"})

	_, err := ParseMIAFConfig(input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one SPIFFE ID")
}

func TestParseMIAFConfig_AuthzFileBlankEntriesFiltered(t *testing.T) {
	dir := t.TempDir()
	input := validInput(t, dir)
	// Mix of valid and blank entries — blanks should be silently dropped.
	input.AuthzPath = writeJSONFile(t, dir, "mixed-authz.json", []string{
		"spiffe://margo.org/device/abc",
		"",
		"   ",
		"spiffe://margo.org/device/xyz",
	})

	got, err := ParseMIAFConfig(input)

	require.NoError(t, err)
	assert.Equal(t, []string{
		"spiffe://margo.org/device/abc",
		"spiffe://margo.org/device/xyz",
	}, got.AuthorizedSPIFFEIDs)
}

func TestParseMIAFConfig_AuthzFileSingleEntry(t *testing.T) {
	dir := t.TempDir()
	input := validInput(t, dir)
	input.AuthzPath = writeJSONFile(t, dir, "single-authz.json", []string{
		"spiffe://margo.org/device/only-one",
	})

	got, err := ParseMIAFConfig(input)

	require.NoError(t, err)
	assert.Len(t, got.AuthorizedSPIFFEIDs, 1)
	assert.Equal(t, "spiffe://margo.org/device/only-one", got.AuthorizedSPIFFEIDs[0])
}
