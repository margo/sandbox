package cli

// cli/x509_test.go

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
