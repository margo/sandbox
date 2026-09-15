package parser

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// oidExtensionSubjectAltName is the OID for Subject Alternative Name
var oidExtensionSubjectAltName = asn1.ObjectIdentifier{2, 5, 29, 17}

// --- Test Helpers ---

type certConfig struct {
	spiffeIDs  []string
	extraURIs  []string
	noURISAN   bool
	invalidPEM bool
}

func generateTestCert(t *testing.T, cfg certConfig) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "failed to generate key")

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(time.Hour),
	}

	// Build URI SANs
	var uris []*url.URL
	for _, id := range cfg.spiffeIDs {
		u, err := url.Parse(id)
		require.NoError(t, err)
		uris = append(uris, u)
	}
	for _, extra := range cfg.extraURIs {
		u, err := url.Parse(extra)
		require.NoError(t, err)
		uris = append(uris, u)
	}
	template.URIs = uris

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err, "failed to create certificate")

	return certDER
}

func toPEM(t *testing.T, derBytes []byte) []byte {
	t.Helper()
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})
}

// --- Tests for ParseSpiffeIdFromX509Svid ---

func TestParseSpiffeIdFromX509Svid(t *testing.T) {
	tests := []struct {
		name        string
		setupCert   func(t *testing.T) []byte
		expectedID  string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid PEM certificate with SPIFFE ID",
			setupCert: func(t *testing.T) []byte {
				der := generateTestCert(t, certConfig{
					spiffeIDs: []string{"spiffe://example.org/service/backend"},
				})
				return toPEM(t, der)
			},
			expectedID:  "spiffe://example.org/service/backend",
			expectError: false,
		},
		{
			name: "valid DER certificate with SPIFFE ID",
			setupCert: func(t *testing.T) []byte {
				return generateTestCert(t, certConfig{
					spiffeIDs: []string{"spiffe://example.org/service/frontend"},
				})
			},
			expectedID:  "spiffe://example.org/service/frontend",
			expectError: false,
		},
		{
			name: "valid PEM with nested SPIFFE ID path",
			setupCert: func(t *testing.T) []byte {
				der := generateTestCert(t, certConfig{
					spiffeIDs: []string{"spiffe://trust-domain.io/ns/default/sa/myservice"},
				})
				return toPEM(t, der)
			},
			expectedID:  "spiffe://trust-domain.io/ns/default/sa/myservice",
			expectError: false,
		},
		{
			name: "certificate with no URI SANs",
			setupCert: func(t *testing.T) []byte {
				der := generateTestCert(t, certConfig{noURISAN: true})
				return toPEM(t, der)
			},
			expectError: true,
			errorMsg:    "certificate has no URI SANs",
		},
		{
			name: "certificate with URI SAN but no SPIFFE ID",
			setupCert: func(t *testing.T) []byte {
				der := generateTestCert(t, certConfig{
					extraURIs: []string{"https://example.org/not-a-spiffe-id"},
				})
				return toPEM(t, der)
			},
			expectError: true,
			errorMsg:    "no SPIFFE ID found in certificate URI SANs",
		},
		{
			name: "certificate with multiple SPIFFE IDs",
			setupCert: func(t *testing.T) []byte {
				der := generateTestCert(t, certConfig{
					spiffeIDs: []string{
						"spiffe://example.org/service/a",
						"spiffe://example.org/service/b",
					},
				})
				return toPEM(t, der)
			},
			expectError: true,
			errorMsg:    "certificate contains multiple SPIFFE IDs",
		},
		{
			name: "certificate with SPIFFE ID and non-SPIFFE URI",
			setupCert: func(t *testing.T) []byte {
				der := generateTestCert(t, certConfig{
					spiffeIDs: []string{"spiffe://example.org/service/backend"},
					extraURIs: []string{"https://example.org/extra"},
				})
				return toPEM(t, der)
			},
			expectedID:  "spiffe://example.org/service/backend",
			expectError: false,
		},
		{
			name: "empty certificate bytes",
			setupCert: func(t *testing.T) []byte {
				return []byte{}
			},
			expectError: true,
			errorMsg:    "certificate bytes are empty",
		},
		{
			name: "nil certificate bytes",
			setupCert: func(t *testing.T) []byte {
				return nil
			},
			expectError: true,
			errorMsg:    "certificate bytes are empty",
		},
		{
			name: "invalid PEM data",
			setupCert: func(t *testing.T) []byte {
				return []byte(
					"-----BEGIN CERTIFICATE-----\ninvalidbase64!!!\n-----END CERTIFICATE-----",
				)
			},
			expectError: true,
			errorMsg:    "failed to parse certificate",
		},
		{
			name: "random garbage bytes",
			setupCert: func(t *testing.T) []byte {
				return []byte("not a certificate at all")
			},
			expectError: true,
			errorMsg:    "failed to parse certificate",
		},
		{
			name: "PEM block with wrong type",
			setupCert: func(t *testing.T) []byte {
				return pem.EncodeToMemory(&pem.Block{
					Type:  "PRIVATE KEY",
					Bytes: []byte("fake key data"),
				})
			},
			expectError: true,
			errorMsg:    "PEM block is not a certificate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certBytes := tt.setupCert(t)

			spiffeID, err := ParseSpiffeIdFromX509Svid(certBytes)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Empty(t, spiffeID)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedID, spiffeID)
			}
		})
	}
}

// --- Tests for parseCertificate ---

func TestParseCertificate(t *testing.T) {
	t.Run("parses valid PEM certificate", func(t *testing.T) {
		der := generateTestCert(t, certConfig{
			spiffeIDs: []string{"spiffe://example.org/test"},
		})
		pemBytes := toPEM(t, der)

		cert, err := parseCertificate(pemBytes)

		require.NoError(t, err)
		require.NotNil(t, cert)
	})

	t.Run("parses valid DER certificate", func(t *testing.T) {
		der := generateTestCert(t, certConfig{
			spiffeIDs: []string{"spiffe://example.org/test"},
		})

		cert, err := parseCertificate(der)

		require.NoError(t, err)
		require.NotNil(t, cert)
	})

	t.Run("returns error for wrong PEM block type", func(t *testing.T) {
		wrongPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: []byte("fake"),
		})

		cert, err := parseCertificate(wrongPEM)

		require.Error(t, err)
		assert.Nil(t, cert)
		assert.Contains(t, err.Error(), "PEM block is not a certificate")
	})

	t.Run("returns error for invalid DER bytes", func(t *testing.T) {
		cert, err := parseCertificate([]byte{0x00, 0x01, 0x02})

		require.Error(t, err)
		assert.Nil(t, cert)
	})
}

// --- Tests for extractSpiffeID ---

func TestExtractSpiffeID(t *testing.T) {
	t.Run("extracts SPIFFE ID from single URI SAN", func(t *testing.T) {
		der := generateTestCert(t, certConfig{
			spiffeIDs: []string{"spiffe://example.org/workload"},
		})
		cert, err := x509.ParseCertificate(der)
		require.NoError(t, err)

		spiffeID, err := extractSpiffeID(cert)

		require.NoError(t, err)
		assert.Equal(t, "spiffe://example.org/workload", spiffeID)
	})

	t.Run("returns error when no URI SANs present", func(t *testing.T) {
		der := generateTestCert(t, certConfig{noURISAN: true})
		cert, err := x509.ParseCertificate(der)
		require.NoError(t, err)

		spiffeID, err := extractSpiffeID(cert)

		require.Error(t, err)
		assert.Empty(t, spiffeID)
		assert.Contains(t, err.Error(), "certificate has no URI SANs")
	})

	t.Run("returns error when URI SANs exist but none are SPIFFE", func(t *testing.T) {
		der := generateTestCert(t, certConfig{
			extraURIs: []string{"https://not-spiffe.example.org"},
		})
		cert, err := x509.ParseCertificate(der)
		require.NoError(t, err)

		spiffeID, err := extractSpiffeID(cert)

		require.Error(t, err)
		assert.Empty(t, spiffeID)
		assert.Contains(t, err.Error(), "no SPIFFE ID found in certificate URI SANs")
	})

	t.Run("returns error when multiple SPIFFE IDs found", func(t *testing.T) {
		der := generateTestCert(t, certConfig{
			spiffeIDs: []string{
				"spiffe://example.org/svc/a",
				"spiffe://example.org/svc/b",
			},
		})
		cert, err := x509.ParseCertificate(der)
		require.NoError(t, err)

		spiffeID, err := extractSpiffeID(cert)

		require.Error(t, err)
		assert.Empty(t, spiffeID)
		assert.Contains(t, err.Error(), "certificate contains multiple SPIFFE IDs")
	})

	t.Run("returns SPIFFE ID when mixed URIs present", func(t *testing.T) {
		der := generateTestCert(t, certConfig{
			spiffeIDs: []string{"spiffe://example.org/svc/mixed"},
			extraURIs: []string{"https://example.org/other"},
		})
		cert, err := x509.ParseCertificate(der)
		require.NoError(t, err)

		spiffeID, err := extractSpiffeID(cert)

		require.NoError(t, err)
		assert.Equal(t, "spiffe://example.org/svc/mixed", spiffeID)
	})
}
