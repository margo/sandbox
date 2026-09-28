package validators

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Helpers ---

func generateRSAPKCS1PEM(t *testing.T, bits int) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, bits)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

func generateECPEM(t *testing.T, curve elliptic.Curve) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(curve, rand.Reader)
	require.NoError(t, err)
	keyBytes, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})
}

func generatePKCS8RSAPEM(t *testing.T, bits int) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, bits)
	require.NoError(t, err)
	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyBytes,
	})
}

func generatePKCS8ECPEM(t *testing.T, curve elliptic.Curve) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(curve, rand.Reader)
	require.NoError(t, err)
	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyBytes,
	})
}

func generatePKCS8Ed25519PEM(t *testing.T) []byte {
	t.Helper()
	// crypto/ed25519 key via x509
	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	keyBytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyBytes,
	})
}

// --- Tests ---

func TestValidatePrivateKey_NoPEMData(t *testing.T) {
	err := ValidatePrivateKey([]byte("not a pem block"))
	assert.EqualError(t, err, "failed to decode PEM block: invalid or missing PEM data")
}

func TestValidatePrivateKey_EmptyInput(t *testing.T) {
	err := ValidatePrivateKey([]byte(""))
	assert.EqualError(t, err, "failed to decode PEM block: invalid or missing PEM data")
}

func TestValidatePrivateKey_UnsupportedPEMType(t *testing.T) {
	block := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: []byte("dummy"),
	})
	err := ValidatePrivateKey(block)
	assert.ErrorContains(t, err, "unsupported PEM block type: CERTIFICATE")
}

// --- RSA PKCS#1 ---

func TestValidatePrivateKey_ValidRSAPKCS1_2048(t *testing.T) {
	err := ValidatePrivateKey(generateRSAPKCS1PEM(t, 2048))
	assert.NoError(t, err)
}

func TestValidatePrivateKey_ValidRSAPKCS1_4096(t *testing.T) {
	err := ValidatePrivateKey(generateRSAPKCS1PEM(t, 4096))
	assert.NoError(t, err)
}

func TestValidatePrivateKey_InvalidRSAPKCS1_CorruptBytes(t *testing.T) {
	block := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: []byte("corrupted-bytes"),
	})
	err := ValidatePrivateKey(block)
	assert.ErrorContains(t, err, "invalid RSA private key")
}

// --- EC PRIVATE KEY ---

func TestValidatePrivateKey_ValidECP256(t *testing.T) {
	err := ValidatePrivateKey(generateECPEM(t, elliptic.P256()))
	assert.NoError(t, err)
}

func TestValidatePrivateKey_ValidECP384(t *testing.T) {
	err := ValidatePrivateKey(generateECPEM(t, elliptic.P384()))
	assert.NoError(t, err)
}

func TestValidatePrivateKey_ValidECP521(t *testing.T) {
	err := ValidatePrivateKey(generateECPEM(t, elliptic.P521()))
	assert.NoError(t, err)
}

func TestValidatePrivateKey_InvalidECKey_CorruptBytes(t *testing.T) {
	block := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: []byte("corrupted-ec-bytes"),
	})
	err := ValidatePrivateKey(block)
	assert.ErrorContains(t, err, "invalid ECDSA private key")
}

// --- PKCS#8 ---

func TestValidatePrivateKey_ValidPKCS8RSA(t *testing.T) {
	err := ValidatePrivateKey(generatePKCS8RSAPEM(t, 2048))
	assert.NoError(t, err)
}

func TestValidatePrivateKey_ValidPKCS8ECP256(t *testing.T) {
	err := ValidatePrivateKey(generatePKCS8ECPEM(t, elliptic.P256()))
	assert.NoError(t, err)
}

func TestValidatePrivateKey_ValidPKCS8ECP384(t *testing.T) {
	err := ValidatePrivateKey(generatePKCS8ECPEM(t, elliptic.P384()))
	assert.NoError(t, err)
}

func TestValidatePrivateKey_ValidPKCS8ECP521(t *testing.T) {
	err := ValidatePrivateKey(generatePKCS8ECPEM(t, elliptic.P521()))
	assert.NoError(t, err)
}

func TestValidatePrivateKey_ValidPKCS8Ed25519(t *testing.T) {
	err := ValidatePrivateKey(generatePKCS8Ed25519PEM(t))
	assert.NoError(t, err)
}

func TestValidatePrivateKey_InvalidPKCS8_CorruptBytes(t *testing.T) {
	block := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: []byte("corrupted-pkcs8-bytes"),
	})
	err := ValidatePrivateKey(block)
	assert.ErrorContains(t, err, "invalid PKCS#8 private key")
}

// --- Table-driven: EC curve coverage ---

func TestValidatePrivateKey_ECCurves(t *testing.T) {
	curves := []struct {
		name  string
		curve elliptic.Curve
	}{
		{"P-256", elliptic.P256()},
		{"P-384", elliptic.P384()},
		{"P-521", elliptic.P521()},
	}

	for _, tc := range curves {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePrivateKey(generateECPEM(t, tc.curve))
			assert.NoError(t, err)
		})
	}
}
