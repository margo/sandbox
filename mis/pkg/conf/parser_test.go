package conf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: creates a temp file and returns its path
func createTempFile(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "testfile-*")
	require.NoError(t, err)
	f.Close()
	return f.Name()
}

// helper: builds a fully valid config using real temp files
func validConfig(t *testing.T) *Config {
	t.Helper()
	return &Config{
		TrustDomain:    "example.org",
		TrustBundleURI: ".well-known/spiffe/bundle.json",
		Log:            &LogConfig{Level: "info"},
		CA: &CAConfig{
			Cert: createTempFile(t),
			Key:  createTempFile(t),
		},
		HTTPS: &HTTPSConfig{
			Addr: ":8443",
			CA:   createTempFile(t),
			Cert: createTempFile(t),
			Key:  createTempFile(t),
		},
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) *Config
		wantErr     bool
		errContains []string // all substrings must appear in error
		assertions  func(t *testing.T, cfg *Config)
	}{
		// ── Happy path ────────────────────────────────────────────────────────
		{
			name:    "valid config passes validation",
			setup:   validConfig,
			wantErr: false,
		},

		// ── Rule 1: trustDomain ───────────────────────────────────────────────
		{
			name: "empty trustDomain returns error",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.TrustDomain = ""
				return cfg
			},
			wantErr:     true,
			errContains: []string{"trustDomain cannot be empty"},
		},

		// ── Rule 2: trustBundleURI default ────────────────────────────────────
		{
			name: "empty trustBundleURI gets default value",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.TrustBundleURI = ""
				return cfg
			},
			wantErr: false,
			assertions: func(t *testing.T, cfg *Config) {
				assert.Equal(t, ".well-known/spiffe/bundle.json", cfg.TrustBundleURI)
			},
		},
		{
			name: "non-empty trustBundleURI is preserved",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.TrustBundleURI = "custom/bundle.json"
				return cfg
			},
			wantErr: false,
			assertions: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "custom/bundle.json", cfg.TrustBundleURI)
			},
		},

		// ── Rule 3: log.level ─────────────────────────────────────────────────
		{
			name: "nil log block gets default level info",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.Log = nil
				return cfg
			},
			wantErr: false,
			assertions: func(t *testing.T, cfg *Config) {
				require.NotNil(t, cfg.Log)
				assert.Equal(t, "info", cfg.Log.Level)
			},
		},
		{
			name: "empty log.level gets default value info",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.Log = &LogConfig{Level: ""}
				return cfg
			},
			wantErr: false,
			assertions: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "info", cfg.Log.Level)
			},
		},
		{
			name: "invalid log.level returns error",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.Log = &LogConfig{Level: "verbose"}
				return cfg
			},
			wantErr:     true,
			errContains: []string{`log.level "verbose" is invalid`},
		},
		{
			name: "valid log level - error",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.Log = &LogConfig{Level: "error"}
				return cfg
			},
			wantErr: false,
		},
		{
			name: "valid log level - debug",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.Log = &LogConfig{Level: "debug"}
				return cfg
			},
			wantErr: false,
		},
		{
			name: "valid log level - warn",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.Log = &LogConfig{Level: "warn"}
				return cfg
			},
			wantErr: false,
		},

		// ── Rule 4 & 5: ca.cert and ca.key ───────────────────────────────────
		{
			name: "nil ca block returns error",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.CA = nil
				return cfg
			},
			wantErr:     true,
			errContains: []string{"ca configuration is required"},
		},
		{
			name: "empty ca.cert returns error",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.CA.Cert = ""
				return cfg
			},
			wantErr:     true,
			errContains: []string{"ca.cert cannot be empty"},
		},
		{
			name: "non-existent ca.cert file returns error",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.CA.Cert = filepath.Join(t.TempDir(), "ghost.crt")
				return cfg
			},
			wantErr:     true,
			errContains: []string{"ca.cert file does not exist"},
		},
		{
			name: "empty ca.key returns error",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.CA.Key = ""
				return cfg
			},
			wantErr:     true,
			errContains: []string{"ca.key cannot be empty"},
		},
		{
			name: "non-existent ca.key file returns error",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.CA.Key = filepath.Join(t.TempDir(), "ghost.key")
				return cfg
			},
			wantErr:     true,
			errContains: []string{"ca.key file does not exist"},
		},

		// ── Rule 6: https.addr default ────────────────────────────────────────
		{
			name: "nil https block gets default addr",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.HTTPS = nil
				return cfg
			},
			wantErr: false,
			assertions: func(t *testing.T, cfg *Config) {
				require.NotNil(t, cfg.HTTPS)
				assert.Equal(t, ":8443", cfg.HTTPS.Addr)
			},
		},
		{
			name: "empty https.addr gets default value",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.HTTPS.Addr = ""
				return cfg
			},
			wantErr: false,
			assertions: func(t *testing.T, cfg *Config) {
				assert.Equal(t, ":8443", cfg.HTTPS.Addr)
			},
		},
		{
			name: "custom https.addr is preserved",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.HTTPS.Addr = ":9443"
				return cfg
			},
			wantErr: false,
			assertions: func(t *testing.T, cfg *Config) {
				assert.Equal(t, ":9443", cfg.HTTPS.Addr)
			},
		},

		// ── Rule 7: https.ca, https.cert, https.key ───────────────────────────
		{
			name: "empty https.ca returns error",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.HTTPS.CA = ""
				return cfg
			},
			wantErr:     true,
			errContains: []string{"https.ca cannot be empty"},
		},
		{
			name: "non-existent https.ca file returns error",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.HTTPS.CA = filepath.Join(t.TempDir(), "ghost-ca.crt")
				return cfg
			},
			wantErr:     true,
			errContains: []string{"https.ca file does not exist"},
		},
		{
			name: "empty https.cert returns error",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.HTTPS.Cert = ""
				return cfg
			},
			wantErr:     true,
			errContains: []string{"https.cert cannot be empty"},
		},
		{
			name: "non-existent https.cert file returns error",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.HTTPS.Cert = filepath.Join(t.TempDir(), "ghost.crt")
				return cfg
			},
			wantErr:     true,
			errContains: []string{"https.cert file does not exist"},
		},
		{
			name: "empty https.key returns error",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.HTTPS.Key = ""
				return cfg
			},
			wantErr:     true,
			errContains: []string{"https.key cannot be empty"},
		},
		{
			name: "non-existent https.key file returns error",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.HTTPS.Key = filepath.Join(t.TempDir(), "ghost.key")
				return cfg
			},
			wantErr:     true,
			errContains: []string{"https.key file does not exist"},
		},

		// ── Multiple errors collected ─────────────────────────────────────────
		{
			name: "multiple violations are all reported",
			setup: func(t *testing.T) *Config {
				cfg := validConfig(t)
				cfg.TrustDomain = ""
				cfg.Log = &LogConfig{Level: "trace"}
				cfg.CA.Cert = ""
				cfg.HTTPS.Key = ""
				return cfg
			},
			wantErr: true,
			errContains: []string{
				"trustDomain cannot be empty",
				`log.level "trace" is invalid`,
				"ca.cert cannot be empty",
				"https.key cannot be empty",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.setup(t)

			err := validateConfig(cfg)

			if tc.wantErr {
				require.Error(t, err)
				for _, substr := range tc.errContains {
					assert.ErrorContains(t, err, substr)
				}
			} else {
				require.NoError(t, err)
			}

			if tc.assertions != nil {
				tc.assertions(t, cfg)
			}
		})
	}
}
