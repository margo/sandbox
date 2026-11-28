package validators

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateTrustPin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		pin         string
		expectErr   bool
		errContains string
	}{
		// ── Valid cases ────────────────────────────────────────────────────────
		{
			name:      "valid pin from miaf2.md example",
			pin:       "sha256/6DtZYsyT+9vuAwfAKHCLpjQv2pQHCc9n7nnZsltwEU4=",
			expectErr: false,
		},
		{
			name:      "valid pin with different base64 value",
			pin:       "sha256/47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=",
			expectErr: false,
		},

		// ── Missing or wrong prefix ────────────────────────────────────────────
		{
			name:        "missing sha256/ prefix entirely",
			pin:         "6DtZYsyT+9vuAwfAKHCLpjQv2pQHCc9n7nnZsltwEU4=",
			expectErr:   true,
			errContains: "sha256/",
		},
		{
			name:        "wrong algorithm prefix sha1/",
			pin:         "sha1/6DtZYsyT+9vuAwfAKHCLpjQv2pQHCc9n7nnZsltwEU4=",
			expectErr:   true,
			errContains: "sha256/",
		},
		{
			name:        "wrong algorithm prefix sha512/",
			pin:         "sha512/6DtZYsyT+9vuAwfAKHCLpjQv2pQHCc9n7nnZsltwEU4=",
			expectErr:   true,
			errContains: "sha256/",
		},
		{
			name:        "prefix in wrong case SHA256/",
			pin:         "SHA256/6DtZYsyT+9vuAwfAKHCLpjQv2pQHCc9n7nnZsltwEU4=",
			expectErr:   true,
			errContains: "sha256/",
		},

		// ── Empty / blank values ───────────────────────────────────────────────
		{
			name:        "empty string",
			pin:         "",
			expectErr:   true,
			errContains: "sha256/",
		},
		{
			name:        "prefix only with no base64 portion",
			pin:         "sha256/",
			expectErr:   true,
			errContains: "base64 portion must not be empty",
		},
		{
			name:        "whitespace only after prefix",
			pin:         "sha256/   ",
			expectErr:   true,
			errContains: "not valid standard base64",
		},

		// ── Invalid base64 ────────────────────────────────────────────────────
		{
			name:        "invalid base64 characters",
			pin:         "sha256/not-valid-base64!!!!",
			expectErr:   true,
			errContains: "not valid standard base64",
		},
		{
			name:        "url-safe base64 with - instead of +",
			pin:         "sha256/6DtZYsyT-9vuAwfAKHCLpjQv2pQHCc9n7nnZsltwEU4=",
			expectErr:   true,
			errContains: "not valid standard base64",
		},
		{
			name:        "url-safe base64 with _ instead of /",
			pin:         "sha256/6DtZYsyT+9vuAwfAKHCLpjQv2pQHCc9n7nnZsltwEU4_",
			expectErr:   true,
			errContains: "not valid standard base64",
		},
		{
			name:        "missing base64 padding",
			pin:         "sha256/6DtZYsyT+9vuAwfAKHCLpjQv2pQHCc9n7nnZsltwEU4",
			expectErr:   true,
			errContains: "not valid standard base64",
		},

		// ── Wrong decoded length ───────────────────────────────────────────────
		{
			name: "base64 decodes to less than 32 bytes (SHA-1 sized)",
			// 20 bytes base64-encoded
			pin:         "sha256/dGVzdHRlc3R0ZXN0dGVzdA==",
			expectErr:   true,
			errContains: "want 32",
		},
		{
			name: "base64 decodes to more than 32 bytes (SHA-512 sized)",
			// 64 bytes base64-encoded
			pin:         "sha256/dGVzdHRlc3R0ZXN0dGVzdHRlc3R0ZXN0dGVzdHRlc3R0ZXN0dGVzdHRlc3R0ZXN0dGVzdA==",
			expectErr:   true,
			errContains: "want 32",
		},
	}

	for _, tc := range tests {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateTrustPin(tc.pin)

			if tc.expectErr {
				require.Error(t, err, "expected an error but got none")
				assert.ErrorContains(t, err, tc.errContains,
					"error message should mention %q", tc.errContains)
			} else {
				require.NoError(t, err, "expected no error but got: %v", err)
			}
		})
	}
}
