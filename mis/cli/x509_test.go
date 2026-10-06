package cli

// cli/x509_test.go

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── validateCSR ───────────────────────────────────────────────────────────────

// generateTestCSR creates a self-signed CSR with the given SPIFFE URI SANs.
// Returns PEM-encoded CSR bytes.
func generateTestCSR(t *testing.T, spiffeIDs []string) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	var uris []*url.URL
	for _, id := range spiffeIDs {
		u, err := url.Parse(id)
		require.NoError(t, err)
		uris = append(uris, u)
	}

	tmpl := &x509.CertificateRequest{URIs: uris}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, tmpl, key)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
}

// generateTestCSRBase64 returns a base64-encoded DER CSR (no PEM wrapper).
func generateTestCSRBase64(t *testing.T, spiffeIDs []string) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	var uris []*url.URL
	for _, id := range spiffeIDs {
		u, err := url.Parse(id)
		require.NoError(t, err)
		uris = append(uris, u)
	}

	tmpl := &x509.CertificateRequest{URIs: uris}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, tmpl, key)
	require.NoError(t, err)

	return base64.StdEncoding.EncodeToString(csrDER)
}

func TestValidateCSR(t *testing.T) {
	tests := []struct {
		name    string
		csr     func(t *testing.T) string
		wantErr string
	}{
		{
			name: "valid PEM CSR with one SPIFFE URI SAN",
			csr: func(t *testing.T) string {
				return string(generateTestCSR(t, []string{"spiffe://example.org/myservice"}))
			},
		},
		{
			name: "valid base64 DER CSR with one SPIFFE URI SAN",
			csr: func(t *testing.T) string {
				return generateTestCSRBase64(t, []string{"spiffe://example.org/myservice"})
			},
		},
		{
			name: "valid PEM CSR with nested SPIFFE path",
			csr: func(t *testing.T) string {
				return string(
					generateTestCSR(t, []string{"spiffe://trust-domain.io/ns/default/sa/myapp"}),
				)
			},
		},
		{
			name: "PEM block with wrong type",
			csr: func(t *testing.T) string {
				// Encode valid DER under a wrong PEM type
				key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				tmpl := &x509.CertificateRequest{}
				der, _ := x509.CreateCertificateRequest(rand.Reader, tmpl, key)
				return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
			},
			wantErr: "PEM block type must be 'CERTIFICATE REQUEST'",
		},
		{
			name: "not PEM and not valid base64",
			csr: func(t *testing.T) string {
				return "this is not a valid CSR at all!!!"
			},
			wantErr: "must be a valid PEM or base64-encoded DER CSR",
		},
		{
			name: "base64 of random bytes (not a valid DER CSR)",
			csr: func(t *testing.T) string {
				return base64.StdEncoding.EncodeToString([]byte("random garbage bytes"))
			},
			wantErr: "invalid CSR",
		},
		{
			name: "PEM CSR with no URI SANs",
			csr: func(t *testing.T) string {
				return string(generateTestCSR(t, []string{}))
			},
			wantErr: "CSR must contain exactly one URI SAN, got 0",
		},
		{
			name: "PEM CSR with multiple URI SANs",
			csr: func(t *testing.T) string {
				return string(generateTestCSR(t, []string{
					"spiffe://example.org/service-a",
					"spiffe://example.org/service-b",
				}))
			},
			wantErr: "CSR must contain exactly one URI SAN, got 2",
		},
		{
			name: "PEM CSR with non-SPIFFE URI SAN",
			csr: func(t *testing.T) string {
				return string(generateTestCSR(t, []string{"https://example.org/myservice"}))
			},
			wantErr: "URI SAN is not a valid SPIFFE ID",
		},
		{
			name: "PEM CSR with SPIFFE ID missing path",
			csr: func(t *testing.T) string {
				return string(generateTestCSR(t, []string{"spiffe://example.org"}))
			},
			wantErr: "URI SAN is not a valid SPIFFE ID",
		},
		{
			name: "PEM CSR with SPIFFE ID root path only",
			csr: func(t *testing.T) string {
				return string(generateTestCSR(t, []string{"spiffe://example.org/"}))
			},
			wantErr: "URI SAN is not a valid SPIFFE ID",
		},
		{
			name: "empty string",
			csr: func(t *testing.T) string {
				return ""
			},
			wantErr: "invalid CSR",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCSR(tc.csr(t))
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

// ── validateSpiffeID ──────────────────────────────────────────────────────────

func TestValidateSpiffeID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr string
	}{
		{
			name: "valid spiffe ID",
			id:   "spiffe://example.org/myservice",
		},
		{
			name: "valid spiffe ID with nested path",
			id:   "spiffe://trust-domain.io/ns/default/sa/myapp",
		},
		{
			name:    "empty string",
			id:      "",
			wantErr: "spiffeID cannot be empty",
		},
		{
			name:    "whitespace only",
			id:      "   ",
			wantErr: "spiffeID cannot be empty",
		},
		{
			name:    "wrong scheme",
			id:      "https://example.org/myservice",
			wantErr: `scheme must be "spiffe"`,
		},
		{
			name:    "missing scheme",
			id:      "example.org/myservice",
			wantErr: `scheme must be "spiffe"`,
		},
		{
			name:    "missing trust domain (host)",
			id:      "spiffe:///myservice",
			wantErr: "trust domain (host) cannot be empty",
		},
		{
			name:    "missing path",
			id:      "spiffe://example.org",
			wantErr: "path cannot be empty",
		},
		{
			name:    "root path only",
			id:      "spiffe://example.org/",
			wantErr: "path cannot be empty",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSpiffeID(tc.id)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

// ── validateOutputDir ─────────────────────────────────────────────────────────

func TestValidateOutputDir(t *testing.T) {
	t.Run("valid writable directory", func(t *testing.T) {
		dir := t.TempDir()
		assert.NoError(t, validateOutputDir(dir))
	})

	t.Run("empty string", func(t *testing.T) {
		err := validateOutputDir("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "outputDir cannot be empty")
	})

	t.Run("whitespace only", func(t *testing.T) {
		err := validateOutputDir("   ")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "outputDir cannot be empty")
	})

	t.Run("non-existent directory", func(t *testing.T) {
		err := validateOutputDir("/tmp/this-path-should-not-exist-mis-test")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})

	t.Run("path is a file not a directory", func(t *testing.T) {
		dir := t.TempDir()
		file, err := os.CreateTemp(dir, "testfile-*")
		require.NoError(t, err)
		file.Close()

		err = validateOutputDir(file.Name())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not a directory")
	})

	t.Run("non-writable directory", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("skipping permission test: running as root")
		}

		dir := t.TempDir()
		require.NoError(t, os.Chmod(dir, 0o600)) // read + execute only
		t.Cleanup(func() { os.Chmod(dir, 0o600) })

		err := validateOutputDir(dir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not writable")
	})
}

// ── validateX509Flags ─────────────────────────────────────────────────────────

func TestValidateX509Flags(t *testing.T) {
	validDir := func(t *testing.T) string {
		t.Helper()
		return t.TempDir()
	}

	validFlags := func(t *testing.T) x509Flags {
		t.Helper()
		return x509Flags{
			SpiffeID:  "spiffe://example.org/myservice",
			TTL:       3600,
			OutputDir: validDir(t),
			DNSNames:  []string{},
		}
	}

	t.Run("valid flags — no DNS SANs", func(t *testing.T) {
		assert.NoError(t, validateX509Flags(validFlags(t)))
	})

	t.Run("valid flags — with DNS SANs", func(t *testing.T) {
		f := validFlags(t)
		f.DNSNames = []string{"myservice.example.com", "api.example.com"}
		assert.NoError(t, validateX509Flags(f))
	})

	t.Run("invalid spiffe ID", func(t *testing.T) {
		f := validFlags(t)
		f.SpiffeID = "https://example.org/myservice"
		err := validateX509Flags(f)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `scheme must be "spiffe"`)
	})

	t.Run("zero TTL", func(t *testing.T) {
		f := validFlags(t)
		f.TTL = 0
		err := validateX509Flags(f)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a positive integer")
	})

	t.Run("negative TTL", func(t *testing.T) {
		f := validFlags(t)
		f.TTL = -100
		err := validateX509Flags(f)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a positive integer")
	})

	t.Run("non-existent output directory", func(t *testing.T) {
		f := validFlags(t)
		f.OutputDir = "/tmp/non-existent-mis-dir"
		err := validateX509Flags(f)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})

	t.Run("empty DNS name in list", func(t *testing.T) {
		f := validFlags(t)
		f.DNSNames = []string{"valid.example.com", "   ", "other.example.com"}
		err := validateX509Flags(f)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "DNS name at index 1 is empty")
	})

	t.Run("all DNS names empty", func(t *testing.T) {
		f := validFlags(t)
		f.DNSNames = []string{""}
		err := validateX509Flags(f)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "DNS name at index 0 is empty")
	})

	t.Run("default TTL is valid", func(t *testing.T) {
		f := validFlags(t)
		f.TTL = defaultTTL
		assert.NoError(t, validateX509Flags(f))
	})

	t.Run("output dir is a file", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "notadir.pem")
		require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

		f := validFlags(t)
		f.OutputDir = file
		err := validateX509Flags(f)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not a directory")
	})
}
