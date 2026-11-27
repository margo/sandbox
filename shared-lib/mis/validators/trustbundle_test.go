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

	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
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
	keys []map[string]any,
	extras map[string]any,
) []byte {
	t.Helper()

	bundle := map[string]any{
		"keys": keys,
	}
	for k, v := range extras {
		bundle[k] = v
	}

	data, err := json.Marshal(bundle)
	require.NoError(t, err)
	return data
}

// validECKey builds a valid EC JWK entry with an x5c certificate (IsCA=true, KeyUsageCertSign set).
func validECKey(t *testing.T) map[string]any {
	t.Helper()

	certBase64, _ := generateSelfSignedCert(t)
	certDER, err := base64.StdEncoding.DecodeString(certBase64)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	bundle := spiffebundle.New(spiffeid.TrustDomain{})
	bundle.AddX509Authority(cert)

	bundleJSON, err := bundle.Marshal()
	require.NoError(t, err)

	var trustBundle map[string]any
	require.NoError(t, json.Unmarshal(bundleJSON, &trustBundle))

	keys, ok := trustBundle["keys"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, keys)

	jwk, ok := keys[0].(map[string]any)
	require.True(t, ok)

	return jwk
}

// generateCertWithOptions allows fine-grained cert generation for negative test cases.
func generateCertWithOptions(
	t *testing.T,
	isCA bool,
	keyUsage x509.KeyUsage,
	setKeyUsage bool,
) map[string]any {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{Organization: []string{"SPIFFE Test"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		BasicConstraintsValid: true,
		IsCA:                  isCA,
	}
	if setKeyUsage {
		template.KeyUsage = keyUsage
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	bundle := spiffebundle.New(spiffeid.TrustDomain{})
	bundle.AddX509Authority(cert)

	bundleJSON, err := bundle.Marshal()
	require.NoError(t, err)

	var trustBundle map[string]any
	require.NoError(t, json.Unmarshal(bundleJSON, &trustBundle))

	keys, ok := trustBundle["keys"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, keys)

	jwk, ok := keys[0].(map[string]any)
	require.True(t, ok)

	return jwk
}

// --- Tests for validateRawStructure ---
// UNCHANGED — no modifications needed here since validateRawStructure signature is unchanged.

func TestValidateRawStructure(t *testing.T) {
	t.Run("valid structure with all fields", func(t *testing.T) {
		seq := int64(12)
		hint := int64(86400)
		data := buildBundle(t, []map[string]any{{"kty": "EC"}}, map[string]any{
			"spiffe_sequence":     seq,
			"spiffe_refresh_hint": hint,
		})

		err := validateRawStructure(data)
		assert.NoError(t, err)
	})

	t.Run("valid structure without optional fields", func(t *testing.T) {
		data := buildBundle(t, []map[string]any{{"kty": "EC"}}, nil)

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
		data := buildBundle(t, []map[string]any{}, nil)

		err := validateRawStructure(data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one key")
	})

	t.Run("negative spiffe_refresh_hint", func(t *testing.T) {
		data := buildBundle(t, []map[string]any{{"kty": "EC"}}, map[string]any{
			"spiffe_refresh_hint": -1,
		})

		err := validateRawStructure(data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "spiffe_refresh_hint")
	})

	t.Run("negative spiffe_sequence", func(t *testing.T) {
		data := buildBundle(t, []map[string]any{{"kty": "EC"}}, map[string]any{
			"spiffe_sequence": -5,
		})

		err := validateRawStructure(data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "spiffe_sequence")
	})

	t.Run("zero values for sequence and hint are valid", func(t *testing.T) {
		data := buildBundle(t, []map[string]any{{"kty": "EC"}}, map[string]any{
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
		data := buildBundle(t, []map[string]any{key}, map[string]any{
			"spiffe_sequence":     12,
			"spiffe_refresh_hint": 86400,
		})

		err := ValidateSpiffeTrustBundle(data) // UPDATED: removed trust domain arg
		assert.NoError(t, err)
	})

	t.Run("valid bundle with multiple keys", func(t *testing.T) {
		key1 := validECKey(t)
		key2 := validECKey(t)
		data := buildBundle(t, []map[string]any{key1, key2}, nil)

		err := ValidateSpiffeTrustBundle(data) // UPDATED: removed trust domain arg
		assert.NoError(t, err)
	})

	t.Run("valid bundle without optional SPIFFE fields", func(t *testing.T) {
		key := validECKey(t)
		data := buildBundle(t, []map[string]any{key}, nil)

		err := ValidateSpiffeTrustBundle(data) // UPDATED: removed trust domain arg
		assert.NoError(t, err)
	})

	t.Run("empty input", func(t *testing.T) {
		err := ValidateSpiffeTrustBundle([]byte{}) // UPDATED: removed trust domain arg
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "trust bundle is empty")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		err := ValidateSpiffeTrustBundle([]byte(`{bad json`)) // UPDATED: removed trust domain arg
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid trust bundle structure")
	})

	t.Run("empty keys array", func(t *testing.T) {
		data := buildBundle(t, []map[string]any{}, nil)

		err := ValidateSpiffeTrustBundle(data) // UPDATED: removed trust domain arg
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one key")
	})

	t.Run("missing x5c field in key", func(t *testing.T) {
		key := map[string]any{
			"kty": "EC",
			"crv": "P-256",
			"x":   "dGVzdA",
			"y":   "dGVzdA",
			"use": "x509-svid",
			// x5c intentionally omitted
		}
		data := buildBundle(t, []map[string]any{key}, nil)

		err := ValidateSpiffeTrustBundle(data) // UPDATED: removed trust domain arg
		assert.Error(t, err)
	})

	t.Run("invalid x5c certificate (not valid DER)", func(t *testing.T) {
		key := map[string]any{
			"kty": "EC",
			"crv": "P-256",
			"x":   "dGVzdA",
			"y":   "dGVzdA",
			"use": "x509-svid",
			"x5c": []string{base64.StdEncoding.EncodeToString([]byte("not-a-real-cert"))},
		}
		data := buildBundle(t, []map[string]any{key}, nil)

		err := ValidateSpiffeTrustBundle(data) // UPDATED: removed trust domain arg
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid SPIFFE trust bundle")
	})

	t.Run("negative spiffe_refresh_hint", func(t *testing.T) {
		key := validECKey(t)
		data := buildBundle(t, []map[string]any{key}, map[string]any{
			"spiffe_refresh_hint": -100,
		})

		err := ValidateSpiffeTrustBundle(data) // UPDATED: removed trust domain arg
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "spiffe_refresh_hint")
	})

	t.Run("negative spiffe_sequence", func(t *testing.T) {
		key := validECKey(t)
		data := buildBundle(t, []map[string]any{key}, map[string]any{
			"spiffe_sequence": -1,
		})

		err := ValidateSpiffeTrustBundle(data) // UPDATED: removed trust domain arg
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "spiffe_sequence")
	})

	t.Run("expired certificate in x5c", func(t *testing.T) {
		key, privKey := func() (map[string]any, *ecdsa.PrivateKey) {
			pk, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			template := &x509.Certificate{
				SerialNumber:          big.NewInt(1),
				NotBefore:             time.Now().Add(-48 * time.Hour),
				NotAfter:              time.Now().Add(-24 * time.Hour), // already expired
				IsCA:                  true,
				BasicConstraintsValid: true,
				KeyUsage:              x509.KeyUsageCertSign,
			}
			certDER, _ := x509.CreateCertificate(rand.Reader, template, template, &pk.PublicKey, pk)
			certB64 := base64.StdEncoding.EncodeToString(certDER)

			return map[string]any{
				"kty": "EC",
				"crv": "P-256",
				"x": base64.RawURLEncoding.EncodeToString(
					//nolint:staticcheck // ignore QF1008 embedding is by standard package
					pk.PublicKey.X.Bytes(),
				),
				"y": base64.RawURLEncoding.EncodeToString(
					//nolint:staticcheck // ignore QF1008 embedding is by standard package
					pk.PublicKey.Y.Bytes(),
				),
				"use": "x509-svid",
				"x5c": []string{certB64},
			}, pk
		}()
		_ = privKey

		data := buildBundle(t, []map[string]any{key}, nil)

		// spiffebundle.Read does not enforce expiry — expiry is enforced at SVID verification time.
		// Expired cert still passes structural + CA authority validation.
		err := ValidateSpiffeTrustBundle(data) // UPDATED: removed trust domain arg
		assert.NoError(t, err)
	})

	// --- NEW: CA authority validation tests ---

	t.Run("authority with IsCA=false is rejected as sole authority", func(t *testing.T) {
		// cert has no BasicConstraints cA=true — must fail hasValidAuthority check
		key := generateCertWithOptions(t, false, x509.KeyUsageCertSign, true)
		data := buildBundle(t, []map[string]any{key}, nil)

		err := ValidateSpiffeTrustBundle(data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no valid CA authority")
	})

	t.Run(
		"authority with IsCA=true but KeyUsage set without KeyCertSign is rejected",
		func(t *testing.T) {
			// KeyUsage present but only CRLSign — must fail hasValidAuthority check
			key := generateCertWithOptions(t, true, x509.KeyUsageCRLSign, true)
			data := buildBundle(t, []map[string]any{key}, nil)

			err := ValidateSpiffeTrustBundle(data)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "no valid CA authority")
		},
	)

	t.Run("authority with IsCA=true and no KeyUsage extension is valid", func(t *testing.T) {
		// KeyUsage absent (zero) — rule says 'where present', so this must pass
		key := generateCertWithOptions(t, true, 0, false)
		data := buildBundle(t, []map[string]any{key}, nil)

		err := ValidateSpiffeTrustBundle(data)
		assert.NoError(t, err)
	})

	t.Run("authority with IsCA=true and KeyCertSign set is valid", func(t *testing.T) {
		// Explicit: IsCA=true + KeyUsageCertSign — canonical valid case
		key := generateCertWithOptions(t, true, x509.KeyUsageCertSign, true)
		data := buildBundle(t, []map[string]any{key}, nil)

		err := ValidateSpiffeTrustBundle(data)
		assert.NoError(t, err)
	})

	t.Run("one valid CA authority among multiple invalid ones passes", func(t *testing.T) {
		// Rule: at least ONE must satisfy — mix of invalid + one valid
		invalidKey1 := generateCertWithOptions(t, false, x509.KeyUsageCertSign, true) // IsCA=false
		invalidKey2 := generateCertWithOptions(
			t,
			true,
			x509.KeyUsageCRLSign,
			true,
		) // wrong KeyUsage
		validKey := generateCertWithOptions(t, true, x509.KeyUsageCertSign, true) // valid

		data := buildBundle(t, []map[string]any{invalidKey1, invalidKey2, validKey}, nil)

		err := ValidateSpiffeTrustBundle(data)
		assert.NoError(t, err)
	})

	t.Run("all authorities invalid fails", func(t *testing.T) {
		// All certs fail CA authority check — must return error
		invalidKey1 := generateCertWithOptions(t, false, x509.KeyUsageCertSign, true)
		invalidKey2 := generateCertWithOptions(t, true, x509.KeyUsageCRLSign, true)

		data := buildBundle(t, []map[string]any{invalidKey1, invalidKey2}, nil)

		err := ValidateSpiffeTrustBundle(data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no valid CA authority")
	})
}
