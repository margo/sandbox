package validators

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test Helpers ---

// generateSelfSignedCert generates a self-signed EC certificate and returns
// the base64 DER-encoded certificate and the private key.
func generateSelfSignedCert(t *testing.T) (string, *ecdsa.PrivateKey) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "failed to generate EC key")

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"SPIFFE Test"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err, "failed to create certificate")

	return base64.StdEncoding.EncodeToString(certDER), key
}

// buildBundle constructs a raw SPIFFE trust bundle JSON from the given fields.
func buildBundle(
	t *testing.T,
	keys []map[string]interface{},
	extras map[string]interface{},
) []byte {
	t.Helper()

	bundle := map[string]interface{}{
		"keys": keys,
	}
	for k, v := range extras {
		bundle[k] = v
	}

	data, err := json.Marshal(bundle)
	require.NoError(t, err)
	return data
}

// validECKey builds a valid EC JWK entry with an x5c certificate.
func validECKey(t *testing.T) map[string]interface{} {
	t.Helper()

	certBase64, key := generateSelfSignedCert(t)

	return map[string]interface{}{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(key.PublicKey.X.Bytes()),
		"y":   base64.RawURLEncoding.EncodeToString(key.PublicKey.Y.Bytes()),
		"use": "x509-svid",
		"x5c": []string{certBase64},
	}
}

// --- Tests for validateRawStructure ---

func TestValidateRawStructure(t *testing.T) {
	t.Run("valid structure with all fields", func(t *testing.T) {
		seq := int64(12)
		hint := int64(86400)
		data := buildBundle(t, []map[string]interface{}{{"kty": "EC"}}, map[string]interface{}{
			"spiffe_sequence":     seq,
			"spiffe_refresh_hint": hint,
		})

		err := validateRawStructure(data)
		assert.NoError(t, err)
	})

	t.Run("valid structure without optional fields", func(t *testing.T) {
		data := buildBundle(t, []map[string]interface{}{{"kty": "EC"}}, nil)

		err := validateRawStructure(data)
		assert.NoError(t, err)
	})

	t.Run("empty byte slice", func(t *testing.T) {
		err := validateRawStructure([]byte{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid JSON")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		err := validateRawStructure([]byte(`{not valid json`))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid JSON")
	})

	t.Run("missing keys field", func(t *testing.T) {
		data := []byte(`{"spiffe_sequence": 1}`)

		err := validateRawStructure(data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one key")
	})

	t.Run("empty keys array", func(t *testing.T) {
		data := buildBundle(t, []map[string]interface{}{}, nil)

		err := validateRawStructure(data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one key")
	})

	t.Run("negative spiffe_refresh_hint", func(t *testing.T) {
		data := buildBundle(t, []map[string]interface{}{{"kty": "EC"}}, map[string]interface{}{
			"spiffe_refresh_hint": -1,
		})

		err := validateRawStructure(data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "spiffe_refresh_hint")
	})

	t.Run("negative spiffe_sequence", func(t *testing.T) {
		data := buildBundle(t, []map[string]interface{}{{"kty": "EC"}}, map[string]interface{}{
			"spiffe_sequence": -5,
		})

		err := validateRawStructure(data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "spiffe_sequence")
	})

	t.Run("zero values for sequence and hint are valid", func(t *testing.T) {
		data := buildBundle(t, []map[string]interface{}{{"kty": "EC"}}, map[string]interface{}{
			"spiffe_sequence":     0,
			"spiffe_refresh_hint": 0,
		})

		err := validateRawStructure(data)
		assert.NoError(t, err)
	})
}

// --- Tests for ValidateSpiffeTrustBundle ---

func TestValidateSpiffeTrustBundle(t *testing.T) {
	t.Run("valid SPIFFE trust bundle", func(t *testing.T) {
		key := validECKey(t)
		data := buildBundle(t, []map[string]interface{}{key}, map[string]interface{}{
			"spiffe_sequence":     12,
			"spiffe_refresh_hint": 86400,
		})

		err := ValidateSpiffeTrustBundle(data, "example.org")
		assert.NoError(t, err)
	})

	t.Run("valid bundle with multiple keys", func(t *testing.T) {
		key1 := validECKey(t)
		key2 := validECKey(t)
		data := buildBundle(t, []map[string]interface{}{key1, key2}, nil)

		err := ValidateSpiffeTrustBundle(data, "example.org")
		assert.NoError(t, err)
	})

	t.Run("valid bundle without optional SPIFFE fields", func(t *testing.T) {
		key := validECKey(t)
		data := buildBundle(t, []map[string]interface{}{key}, nil)

		err := ValidateSpiffeTrustBundle(data, "example.org")
		assert.NoError(t, err)
	})

	t.Run("empty input", func(t *testing.T) {
		err := ValidateSpiffeTrustBundle([]byte{}, "example.org")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "trust bundle is empty")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		err := ValidateSpiffeTrustBundle([]byte(`{bad json`), "example.org")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid trust bundle structure")
	})

	t.Run("empty keys array", func(t *testing.T) {
		data := buildBundle(t, []map[string]interface{}{}, nil)

		err := ValidateSpiffeTrustBundle(data, "example.org")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one key")
	})

	t.Run("invalid trust domain", func(t *testing.T) {
		key := validECKey(t)
		data := buildBundle(t, []map[string]interface{}{key}, nil)

		err := ValidateSpiffeTrustBundle(data, "not a valid domain!!!")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid trust domain")
	})

	t.Run("missing x5c field in key", func(t *testing.T) {
		key := map[string]interface{}{
			"kty": "EC",
			"crv": "P-256",
			"x":   "dGVzdA",
			"y":   "dGVzdA",
			"use": "x509-svid",
			// x5c intentionally omitted
		}
		data := buildBundle(t, []map[string]interface{}{key}, nil)

		err := ValidateSpiffeTrustBundle(data, "example.org")
		assert.Error(t, err)
	})

	t.Run("invalid x5c certificate (not valid DER)", func(t *testing.T) {
		key := map[string]interface{}{
			"kty": "EC",
			"crv": "P-256",
			"x":   "dGVzdA",
			"y":   "dGVzdA",
			"use": "x509-svid",
			"x5c": []string{base64.StdEncoding.EncodeToString([]byte("not-a-real-cert"))},
		}
		data := buildBundle(t, []map[string]interface{}{key}, nil)

		err := ValidateSpiffeTrustBundle(data, "example.org")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid SPIFFE trust bundle")
	})

	t.Run("negative spiffe_refresh_hint", func(t *testing.T) {
		key := validECKey(t)
		data := buildBundle(t, []map[string]interface{}{key}, map[string]interface{}{
			"spiffe_refresh_hint": -100,
		})

		err := ValidateSpiffeTrustBundle(data, "example.org")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "spiffe_refresh_hint")
	})

	t.Run("negative spiffe_sequence", func(t *testing.T) {
		key := validECKey(t)
		data := buildBundle(t, []map[string]interface{}{key}, map[string]interface{}{
			"spiffe_sequence": -1,
		})

		err := ValidateSpiffeTrustBundle(data, "example.org")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "spiffe_sequence")
	})

	t.Run("expired certificate in x5c", func(t *testing.T) {
		key, privKey := func() (map[string]interface{}, *ecdsa.PrivateKey) {
			pk, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			template := &x509.Certificate{
				SerialNumber:          big.NewInt(1),
				NotBefore:             time.Now().Add(-48 * time.Hour),
				NotAfter:              time.Now().Add(-24 * time.Hour), // already expired
				IsCA:                  true,
				BasicConstraintsValid: true,
			}
			certDER, _ := x509.CreateCertificate(rand.Reader, template, template, &pk.PublicKey, pk)
			certB64 := base64.StdEncoding.EncodeToString(certDER)

			return map[string]interface{}{
				"kty": "EC",
				"crv": "P-256",
				"x":   base64.RawURLEncoding.EncodeToString(pk.PublicKey.X.Bytes()),
				"y":   base64.RawURLEncoding.EncodeToString(pk.PublicKey.Y.Bytes()),
				"use": "x509-svid",
				"x5c": []string{certB64},
			}, pk
		}()
		_ = privKey

		data := buildBundle(t, []map[string]interface{}{key}, nil)

		// Note: spiffebundle.Read does not enforce expiry at parse time,
		// so this is expected to pass structural validation.
		// Expiry enforcement happens at SVID verification time.
		err := ValidateSpiffeTrustBundle(data, "example.org")
		assert.NoError(t, err)
	})
}
