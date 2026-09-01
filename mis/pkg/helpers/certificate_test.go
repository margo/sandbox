package helpers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: write a temp cert file with given content, return its path.
func writeTempCert(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "cert-*.crt")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

func TestCreateBundleFile_NoCertPaths(t *testing.T) {
	bundlePath, err := CreateBundleFile()

	assert.Empty(t, bundlePath)
	assert.EqualError(t, err, "at least one certificate path must be provided")
}

func TestCreateBundleFile_SingleCertificate(t *testing.T) {
	certContent := "-----BEGIN CERTIFICATE-----\nABCDEF\n-----END CERTIFICATE-----\n"
	certPath := writeTempCert(t, certContent)

	bundlePath, err := CreateBundleFile(certPath)
	require.NoError(t, err)
	defer os.Remove(bundlePath)

	assert.True(t, filepath.IsAbs(bundlePath), "returned path must be absolute")
	assert.FileExists(t, bundlePath)

	//nolint:gosec // filePath is a trusted system configuration variable; this is a unit test file
	got, err := os.ReadFile(bundlePath)
	require.NoError(t, err)
	assert.Equal(t, certContent, string(got))
}

func TestCreateBundleFile_MultipleCertificates(t *testing.T) {
	cert1 := "-----BEGIN CERTIFICATE-----\nROOT\n-----END CERTIFICATE-----\n"
	cert2 := "-----BEGIN CERTIFICATE-----\nINTERMEDIATE\n-----END CERTIFICATE-----\n"

	path1 := writeTempCert(t, cert1)
	path2 := writeTempCert(t, cert2)

	bundlePath, err := CreateBundleFile(path1, path2)
	require.NoError(t, err)
	defer os.Remove(bundlePath)

	//nolint:gosec // filePath is a trusted system configuration variable; this is a unit test file
	got, err := os.ReadFile(bundlePath)
	require.NoError(t, err)

	assert.Contains(t, string(got), cert1)
	assert.Contains(t, string(got), cert2)
	// cert1 must appear before cert2 (order preserved)
	assert.Less(t, strings.Index(string(got), cert1), strings.Index(string(got), cert2))
}

func TestCreateBundleFile_NewlineSeparatorAdded(t *testing.T) {
	// Cert without trailing newline — separator must be injected.
	cert1 := "-----BEGIN CERTIFICATE-----\nROOT\n-----END CERTIFICATE-----"
	cert2 := "-----BEGIN CERTIFICATE-----\nINTER\n-----END CERTIFICATE-----\n"

	path1 := writeTempCert(t, cert1)
	path2 := writeTempCert(t, cert2)

	bundlePath, err := CreateBundleFile(path1, path2)
	require.NoError(t, err)
	defer os.Remove(bundlePath)

	//nolint:gosec // filePath is a trusted system configuration variable; this is a unit test file
	got, err := os.ReadFile(bundlePath)
	require.NoError(t, err)

	// The injected newline ensures cert2 starts on its own line.
	assert.Contains(t, string(got), cert1+"\n"+cert2)
}

func TestCreateBundleFile_NoNewlineSeparatorWhenAlreadyPresent(t *testing.T) {
	// Cert already ends with newline — no extra newline should be added.
	cert1 := "-----BEGIN CERTIFICATE-----\nROOT\n-----END CERTIFICATE-----\n"
	cert2 := "-----BEGIN CERTIFICATE-----\nINTER\n-----END CERTIFICATE-----\n"

	path1 := writeTempCert(t, cert1)
	path2 := writeTempCert(t, cert2)

	bundlePath, err := CreateBundleFile(path1, path2)
	require.NoError(t, err)
	defer os.Remove(bundlePath)

	//nolint:gosec // filePath is a trusted system configuration variable; this is a unit test file
	got, err := os.ReadFile(bundlePath)
	require.NoError(t, err)

	// Must not contain double newline between certs.
	assert.NotContains(t, string(got), "\n\n")
}

func TestCreateBundleFile_NonExistentCertPath(t *testing.T) {
	bundlePath, err := CreateBundleFile("/nonexistent/path/cert.crt")

	assert.Empty(t, bundlePath)
	assert.ErrorContains(t, err, "read certificate")
	assert.ErrorContains(t, err, "/nonexistent/path/cert.crt")
}

func TestCreateBundleFile_SecondCertNonExistent(t *testing.T) {
	cert1 := "-----BEGIN CERTIFICATE-----\nROOT\n-----END CERTIFICATE-----\n"
	path1 := writeTempCert(t, cert1)

	bundlePath, err := CreateBundleFile(path1, "/nonexistent/inter.crt")

	assert.Empty(t, bundlePath)
	assert.ErrorContains(t, err, "read certificate")
	// Temp bundle must be cleaned up on error.
	matches, _ := filepath.Glob("./cert-bundle-*.crt")
	assert.Empty(t, matches, "temporary bundle file must be removed on error")
}

func TestCreateBundleFile_EmptyCertFile(t *testing.T) {
	path := writeTempCert(t, "")

	bundlePath, err := CreateBundleFile(path)
	require.NoError(t, err)
	defer os.Remove(bundlePath)

	//nolint:gosec // filePath is a trusted system configuration variable; this is a unit test file
	got, err := os.ReadFile(bundlePath)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestCreateBundleFile_ReturnedPathIsAbsolute(t *testing.T) {
	certPath := writeTempCert(t, "-----BEGIN CERTIFICATE-----\nDATA\n-----END CERTIFICATE-----\n")

	bundlePath, err := CreateBundleFile(certPath)
	require.NoError(t, err)
	defer os.Remove(bundlePath)

	assert.True(t, filepath.IsAbs(bundlePath))
}

func TestCreateBundleFile_BundleFileNamePattern(t *testing.T) {
	certPath := writeTempCert(t, "-----BEGIN CERTIFICATE-----\nDATA\n-----END CERTIFICATE-----\n")

	bundlePath, err := CreateBundleFile(certPath)
	require.NoError(t, err)
	defer os.Remove(bundlePath)

	assert.Regexp(t, `cert-bundle-.+\.crt$`, filepath.Base(bundlePath))
}
