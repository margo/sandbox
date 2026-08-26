package cli

// cli/x509_test.go

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// validateSpiffeID
// ---------------------------------------------------------------------------

func TestValidateSpiffeID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr string // substring expected in error; empty means no error
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
			name:    "wrong scheme — https",
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
			name:    "path is root slash only",
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

// ---------------------------------------------------------------------------
// validateOutputDir
// ---------------------------------------------------------------------------

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
		err := validateOutputDir("/tmp/this-path-should-never-exist-svidctl-test")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})

	t.Run("path is a file not a directory", func(t *testing.T) {
		dir := t.TempDir()
		file, err := os.CreateTemp(dir, "not-a-dir-*")
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
		require.NoError(t, os.Chmod(dir, 0o555)) // read+execute only
		t.Cleanup(func() { os.Chmod(dir, 0o755) })

		err := validateOutputDir(dir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not writable")
	})
}

// ---------------------------------------------------------------------------
// validateFileExists
// ---------------------------------------------------------------------------

func TestValidateFileExists(t *testing.T) {
	t.Run("valid existing file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "ca.pem")
		require.NoError(t, os.WriteFile(path, []byte("cert"), 0o644))

		assert.NoError(t, validateFileExists(path, "CAcert"))
	})

	t.Run("empty path", func(t *testing.T) {
		err := validateFileExists("", "CAcert")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CAcert cannot be empty")
	})

	t.Run("whitespace only path", func(t *testing.T) {
		err := validateFileExists("   ", "CAkey")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CAkey cannot be empty")
	})

	t.Run("non-existent file", func(t *testing.T) {
		err := validateFileExists("/tmp/ghost-cert-svidctl.pem", "CAcert")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist on disk")
	})

	t.Run("path points to a directory", func(t *testing.T) {
		dir := t.TempDir()
		err := validateFileExists(dir, "CAcert")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is a directory, expected a file")
	})

	t.Run("flag name appears in error message", func(t *testing.T) {
		err := validateFileExists("", "CAkey")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CAkey")
	})
}

// ---------------------------------------------------------------------------
// validateX509Flags
// ---------------------------------------------------------------------------

func TestValidateX509Flags(t *testing.T) {
	// helpers to build valid on-disk fixtures once per test
	makeDir := func(t *testing.T) string {
		t.Helper()
		return t.TempDir()
	}
	makeFile := func(t *testing.T, dir, name string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte("data"), 0o644))
		return path
	}

	validFlags := func(t *testing.T) x509Flags {
		t.Helper()
		dir := makeDir(t)
		return x509Flags{
			SpiffeID:  "spiffe://example.org/svc",
			TTL:       3600,
			OutputDir: dir,
			CAcert:    makeFile(t, dir, "ca.pem"),
			CAkey:     makeFile(t, dir, "ca-key.pem"),
			DNSNames:  []string{},
		}
	}

	t.Run("all valid flags", func(t *testing.T) {
		assert.NoError(t, validateX509Flags(validFlags(t)))
	})

	t.Run("invalid spiffe ID", func(t *testing.T) {
		f := validFlags(t)
		f.SpiffeID = "https://example.org/svc"
		err := validateX509Flags(f)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `scheme must be "spiffe"`)
	})

	t.Run("TTL zero", func(t *testing.T) {
		f := validFlags(t)
		f.TTL = 0
		err := validateX509Flags(f)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a positive integer")
	})

	t.Run("TTL negative", func(t *testing.T) {
		f := validFlags(t)
		f.TTL = -1
		err := validateX509Flags(f)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a positive integer")
	})

	t.Run("non-existent output directory", func(t *testing.T) {
		f := validFlags(t)
		f.OutputDir = "/tmp/no-such-dir-svidctl"
		err := validateX509Flags(f)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})

	t.Run("empty DNS name in list", func(t *testing.T) {
		f := validFlags(t)
		f.DNSNames = []string{"valid.example.com", "  ", "other.example.com"}
		err := validateX509Flags(f)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "DNS name at index 1 is empty")
	})

	t.Run("valid DNS names", func(t *testing.T) {
		f := validFlags(t)
		f.DNSNames = []string{"svc.example.com", "svc-internal.example.com"}
		assert.NoError(t, validateX509Flags(f))
	})

	t.Run("missing CAcert file", func(t *testing.T) {
		f := validFlags(t)
		f.CAcert = "/tmp/ghost-ca.pem"
		err := validateX509Flags(f)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist on disk")
	})

	t.Run("missing CAkey file", func(t *testing.T) {
		f := validFlags(t)
		f.CAkey = "/tmp/ghost-ca-key.pem"
		err := validateX509Flags(f)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist on disk")
	})

	t.Run("CAcert is a directory", func(t *testing.T) {
		f := validFlags(t)
		f.CAcert = makeDir(t)
		err := validateX509Flags(f)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is a directory, expected a file")
	})
}
