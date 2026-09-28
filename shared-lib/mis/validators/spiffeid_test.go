package validators

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSpiffeID(t *testing.T) {
	tests := []struct {
		name      string
		spiffeID  string
		principal string
		wantErr   bool
		errMsg    string
	}{
		// ── PrincipalWFM happy paths ──────────────────────────────────────────
		{
			name:      "valid WFM spiffe ID",
			spiffeID:  "spiffe://example.org/margo/wfm/my-wfm-123",
			principal: PrincipalWFM,
			wantErr:   false,
		},
		{
			name:      "valid WFM spiffe ID with complex trust domain",
			spiffeID:  "spiffe://my.trust-domain_1/margo/wfm/wfm-abc",
			principal: PrincipalWFM,
			wantErr:   false,
		},
		{
			name:      "valid WFM spiffe ID with alphanumeric wfm-id",
			spiffeID:  "spiffe://acme.org/margo/wfm/abc123",
			principal: PrincipalWFM,
			wantErr:   false,
		},

		// ── PrincipalWFMClient happy paths ────────────────────────────────────
		{
			name:      "valid WFM-Client spiffe ID",
			spiffeID:  "spiffe://example.org/margo/wfm/my-wfm-123/client/client-456",
			principal: PrincipalWFMClient,
			wantErr:   false,
		},
		{
			name:      "valid WFM-Client spiffe ID with complex IDs",
			spiffeID:  "spiffe://trust.domain/margo/wfm/wfm-001/client/cli_abc.1",
			principal: PrincipalWFMClient,
			wantErr:   false,
		},

		// ── Missing / wrong scheme ────────────────────────────────────────────
		{
			name:      "missing spiffe scheme",
			spiffeID:  "http://example.org/margo/wfm/my-wfm-123",
			principal: PrincipalWFM,
			wantErr:   true,
			errMsg:    `spiffe ID must start with "spiffe://"`,
		},
		{
			name:      "no scheme at all",
			spiffeID:  "example.org/margo/wfm/my-wfm-123",
			principal: PrincipalWFM,
			wantErr:   true,
			errMsg:    `spiffe ID must start with "spiffe://"`,
		},
		{
			name:      "empty string",
			spiffeID:  "",
			principal: PrincipalWFM,
			wantErr:   true,
			errMsg:    `spiffe ID must start with "spiffe://"`,
		},

		// ── Trust domain errors ───────────────────────────────────────────────
		{
			name:      "missing path after trust domain",
			spiffeID:  "spiffe://example.org",
			principal: PrincipalWFM,
			wantErr:   true,
			errMsg:    "spiffe ID must contain a path after the trust domain",
		},
		{
			name:      "uppercase letters in trust domain",
			spiffeID:  "spiffe://Example.Org/margo/wfm/my-wfm-123",
			principal: PrincipalWFM,
			wantErr:   true,
			errMsg:    "invalid trust domain",
		},
		{
			name:      "trust domain exceeds 255 characters",
			spiffeID:  "spiffe://" + string(make([]byte, 256)) + "/margo/wfm/id",
			principal: PrincipalWFM,
			wantErr:   true,
		},

		// ── Path segment errors ───────────────────────────────────────────────
		{
			name:      "empty path segment (double slash in path)",
			spiffeID:  "spiffe://example.org/margo//wfm/my-wfm-123",
			principal: PrincipalWFM,
			wantErr:   true,
			errMsg:    "spiffe ID path must not contain empty segments",
		},
		{
			name:      "invalid character in path segment",
			spiffeID:  "spiffe://example.org/margo/wfm/my wfm",
			principal: PrincipalWFM,
			wantErr:   true,
			errMsg:    "invalid path segment",
		},

		// ── Wrong principal pattern ───────────────────────────────────────────
		{
			name:      "WFM-Client ID validated as WFM principal",
			spiffeID:  "spiffe://example.org/margo/wfm/my-wfm-123/client/client-456",
			principal: PrincipalWFM,
			wantErr:   true,
			errMsg:    "invalid WFM spiffe ID",
		},
		{
			name:      "WFM ID validated as WFM-Client principal",
			spiffeID:  "spiffe://example.org/margo/wfm/my-wfm-123",
			principal: PrincipalWFMClient,
			wantErr:   true,
			errMsg:    "invalid WFM-Client spiffe ID",
		},
		{
			name:      "missing /margo/ prefix in path",
			spiffeID:  "spiffe://example.org/wfm/my-wfm-123",
			principal: PrincipalWFM,
			wantErr:   true,
			errMsg:    "invalid WFM spiffe ID",
		},
		{
			name:      "missing wfm-id segment",
			spiffeID:  "spiffe://example.org/margo/wfm/",
			principal: PrincipalWFM,
			wantErr:   true,
		},
		{
			name:      "missing client-id segment for WFM-Client",
			spiffeID:  "spiffe://example.org/margo/wfm/my-wfm-123/client/",
			principal: PrincipalWFMClient,
			wantErr:   true,
		},

		// ── Unknown principal ─────────────────────────────────────────────────
		{
			name:      "unknown principal",
			spiffeID:  "spiffe://example.org/margo/wfm/my-wfm-123",
			principal: "admin",
			wantErr:   true,
			errMsg:    `unknown principal "admin"`,
		},
		{
			name:      "empty principal",
			spiffeID:  "spiffe://example.org/margo/wfm/my-wfm-123",
			principal: "",
			wantErr:   true,
			errMsg:    "unknown principal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSpiffeID(tt.spiffeID, tt.principal)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateSpiffeIDWithTrustDomain(t *testing.T) {
	tests := []struct {
		name        string
		spiffeID    string
		trustDomain string
		principal   string
		wantErr     bool
		errMsg      string
	}{
		// ── Happy paths ───────────────────────────────────────────────────────
		{
			name:        "valid WFM ID with matching trust domain",
			spiffeID:    "spiffe://example.org/margo/wfm/my-wfm-123",
			trustDomain: "example.org",
			principal:   PrincipalWFM,
			wantErr:     false,
		},
		{
			name:        "valid WFM-Client ID with matching trust domain",
			spiffeID:    "spiffe://example.org/margo/wfm/my-wfm-123/client/client-456",
			trustDomain: "example.org",
			principal:   PrincipalWFMClient,
			wantErr:     false,
		},
		{
			name:        "valid WFM ID with complex matching trust domain",
			spiffeID:    "spiffe://my.trust-domain_1/margo/wfm/wfm-abc",
			trustDomain: "my.trust-domain_1",
			principal:   PrincipalWFM,
			wantErr:     false,
		},

		// ── Trust domain mismatch ─────────────────────────────────────────────
		{
			name:        "trust domain mismatch",
			spiffeID:    "spiffe://other.org/margo/wfm/my-wfm-123",
			trustDomain: "example.org",
			principal:   PrincipalWFM,
			wantErr:     true,
			errMsg:      `trust domain mismatch: expected "example.org", got "other.org"`,
		},
		{
			name:        "trust domain mismatch for WFM-Client",
			spiffeID:    "spiffe://other.org/margo/wfm/my-wfm-123/client/client-456",
			trustDomain: "example.org",
			principal:   PrincipalWFMClient,
			wantErr:     true,
			errMsg:      "trust domain mismatch",
		},
		{
			name:        "empty trust domain argument causes mismatch",
			spiffeID:    "spiffe://example.org/margo/wfm/my-wfm-123",
			trustDomain: "",
			principal:   PrincipalWFM,
			wantErr:     true,
			errMsg:      "trust domain mismatch",
		},

		// ── Base validation failures propagated ───────────────────────────────
		{
			name:        "invalid spiffe ID propagates base error",
			spiffeID:    "http://example.org/margo/wfm/my-wfm-123",
			trustDomain: "example.org",
			principal:   PrincipalWFM,
			wantErr:     true,
			errMsg:      `spiffe ID must start with "spiffe://"`,
		},
		{
			name:        "wrong principal propagates base error",
			spiffeID:    "spiffe://example.org/margo/wfm/my-wfm-123",
			trustDomain: "example.org",
			principal:   "unknown",
			wantErr:     true,
			errMsg:      "unknown principal",
		},
		{
			name:        "WFM-Client ID with WFM principal propagates base error",
			spiffeID:    "spiffe://example.org/margo/wfm/my-wfm-123/client/client-456",
			trustDomain: "example.org",
			principal:   PrincipalWFM,
			wantErr:     true,
			errMsg:      "invalid WFM spiffe ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSpiffeIDWithTrustDomain(tt.spiffeID, tt.trustDomain, tt.principal)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
