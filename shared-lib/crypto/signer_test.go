package crypto

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func generateTestKeyPair(t *testing.T) (privatePEM string, publicPEM string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privDER := x509.MarshalPKCS1PrivateKey(key)
	privBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privDER}
	privPEM := pem.EncodeToMemory(privBlock)

	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	pubBlock := &pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}
	pubPEM := pem.EncodeToMemory(pubBlock)

	return string(privPEM), string(pubPEM)
}

func TestComputeKeyIDFromPrivateKeyPEM(t *testing.T) {
	priv, _ := generateTestKeyPair(t)
	kid, err := ComputeKeyIDFromPrivateKeyPEM(priv)
	require.NoError(t, err)
	require.Len(t, kid, 64)
}

func TestSignVerifyRoundTrip(t *testing.T) {
	priv, pub := generateTestKeyPair(t)
	pub = base64.StdEncoding.EncodeToString([]byte(pub))

	kid, err := ComputeKeyIDFromPrivateKeyPEM(priv)
	require.NoError(t, err)

	signer, err := NewSigner(priv, kid, "rsa", "sha256", "sig1")
	require.NoError(t, err)

	// create request with body
	body := []byte("hello world")
	url := "https://somerandomurl.willnothit.com/api/v1/resource"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 2. Create the request with that context
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	err = signer.SignRequest(context.Background(), req)
	require.NoError(t, err)

	// verify using a verifier with the public key
	verifier, err := NewVerifier(pub, true)
	require.NoError(t, err)
	err = verifier.VerifyRequest(context.Background(), req)
	require.NoError(t, err)
}
