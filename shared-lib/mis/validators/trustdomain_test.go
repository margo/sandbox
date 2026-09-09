package validators

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateTrustDomain(t *testing.T) {
	tests := []struct {
		name     string
		domain   string
		expected bool
	}{
		// Valid domains
		{
			name:     "valid simple domain",
			domain:   "margo.org",
			expected: true,
		},
		{
			name:     "valid domain with com TLD",
			domain:   "example.com",
			expected: true,
		},
		{
			name:     "valid subdomain",
			domain:   "sub.margo.org",
			expected: true,
		},
		{
			name:     "valid multi-level subdomain",
			domain:   "a.b.margo.org",
			expected: true,
		},
		{
			name:     "valid domain with hyphens",
			domain:   "my-domain.org",
			expected: true,
		},
		{
			name:     "valid long TLD",
			domain:   "example.technology",
			expected: true,
		},
		{
			name:     "valid alphanumeric domain",
			domain:   "abc123.com",
			expected: true,
		},

		// Invalid — scheme prefix
		{
			name:     "invalid https scheme",
			domain:   "https://margo.org",
			expected: false,
		},
		{
			name:     "invalid http scheme",
			domain:   "http://margo.org",
			expected: false,
		},
		{
			name:     "invalid ftp scheme",
			domain:   "ftp://margo.org",
			expected: false,
		},

		// Invalid — missing TLD
		{
			name:     "invalid no TLD",
			domain:   "margo",
			expected: false,
		},
		{
			name:     "invalid single character TLD",
			domain:   "margo.o",
			expected: false,
		},

		// Invalid — illegal characters
		{
			name:     "invalid with path",
			domain:   "margo.org/path",
			expected: false,
		},
		{
			name:     "invalid with port",
			domain:   "margo.org:8080",
			expected: false,
		},
		{
			name:     "invalid with query string",
			domain:   "margo.org?q=1",
			expected: false,
		},
		{
			name:     "invalid with fragment",
			domain:   "margo.org#section",
			expected: false,
		},
		{
			name:     "invalid with @ symbol",
			domain:   "user@margo.org",
			expected: false,
		},

		// Invalid — hyphen rules
		{
			name:     "invalid label starts with hyphen",
			domain:   "-margo.org",
			expected: false,
		},
		{
			name:     "invalid label ends with hyphen",
			domain:   "margo-.org",
			expected: false,
		},

		// Invalid — empty/blank
		{
			name:     "invalid empty string",
			domain:   "",
			expected: false,
		},
		{
			name:     "invalid whitespace only",
			domain:   "   ",
			expected: false,
		},

		// Invalid — numeric TLD
		{
			name:     "invalid numeric TLD",
			domain:   "margo.123",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateTrustDomain(tt.domain)
			assert.Equal(
				t, tt.expected, result,
				"ValidateTrustDomain(%q) expected %v but got %v",
				tt.domain, tt.expected, result,
			)
		})
	}
}
