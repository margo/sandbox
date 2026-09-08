package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lestrrat-go/htmsig"
	"github.com/lestrrat-go/htmsig/component"
	"github.com/lestrrat-go/htmsig/input"
)

// ===== RFC 9421 CLIENT-SIDE SIGNING + TLS TRUST =====
//
// Everything the runner needs to present itself as a device-agent: build the
// Content-Digest, load a signing key, produce an RFC 9421 signature with a
// chosen algorithm (MI-011/012/014), and — for MI-018 — verify the WFM's TLS
// certificate against the fetched root CA.

// defaultDeviceKeyPath is the private key used to sign requests unless a
// scenario/step selects another. Matches ./certs/device-cert.pem.
const defaultDeviceKeyPath = "./certs/device-key.pem"

func getDeviceKeyPath() string {
	if customPath := strings.TrimSpace(os.Getenv("DEVICE_PRIVATE_KEY_PATH")); customPath != "" {
		return customPath
	}
	return defaultDeviceKeyPath
}

func buildContentDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
}

// firstNonEmpty returns the first non-empty string, or "".
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// ctxString reads a string value from ctx.Data (empty if absent/other type).
func ctxString(ctx *TestContext, key string) string {
	if s, ok := ctx.Data[key].(string); ok {
		return s
	}
	return ""
}

// caVerifyingClient returns an HTTP client that verifies the server's TLS
// certificate against the root CA in certs/ca-cert.pem — the CA a device fetches
// via the Certificate API (MI-018).
func caVerifyingClient() (*http.Client, error) {
	caPEM, err := os.ReadFile(filepath.Join(certDir, "ca-cert.pem"))
	if err != nil {
		return nil, fmt.Errorf("read ca-cert.pem: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("ca-cert.pem is not a valid certificate")
	}
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}},
	}, nil
}

// loadPrivateKey loads and parses a PEM private key from path.
func loadPrivateKey(keyPath string) (interface{}, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("private key not found at %s: %w", keyPath, err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to PEM-decode private key")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("unrecognized private key format")
}

// signRequest adds RFC 9421 Signature-Input, Signature, and Content-Digest
// headers. keyPath selects the signing key; alg, when set, pins the RFC 9421
// algorithm (otherwise htmsig derives it from the key type).
func signRequest(req *http.Request, bodyBytes []byte, keyPath, alg string) error {
	key, err := loadPrivateKey(keyPath)
	if err != nil {
		return fmt.Errorf("could not load signing key: %w", err)
	}

	comps := []component.Identifier{component.Method(), component.TargetURI()}
	if len(bodyBytes) > 0 {
		req.Header.Set("Content-Digest", buildContentDigest(bodyBytes))
		comps = append(comps, component.New("content-digest"))
	}

	// Build the signature definition directly (not via the http Signer wrapper)
	// so a non-default algorithm can be pinned — the wrapper's NewSigner has no
	// exported algorithm option.
	builder := input.NewDefinitionBuilder().
		Label("sig1").
		KeyID("device-key").
		Components(comps...).
		Created(time.Now().Unix())
	if alg != "" {
		builder = builder.Algorithm(alg)
	}
	def, err := builder.Build()
	if err != nil {
		return fmt.Errorf("build signature definition: %w", err)
	}
	inputValue := input.NewValueBuilder().AddDefinition(def).MustBuild()
	ctx := component.WithRequestInfoFromHTTP(context.Background(), req)
	if err := htmsig.SignRequest(ctx, req.Header, inputValue, key); err != nil {
		return fmt.Errorf("htmsig SignRequest failed: %w", err)
	}
	return nil
}
