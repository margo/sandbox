package crypto

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/lestrrat-go/htmsig/component"
	htmsighttp "github.com/lestrrat-go/htmsig/http"
)

type HTTPSigner interface {
	SignRequest(ctx context.Context, req *http.Request) error
	SignResponse(ctx context.Context, resp http.ResponseWriter) error
}

type HTMPayloadSigner struct {
	privateKey   []byte
	signer       htmsighttp.Signer // for requests with body (includes content-digest)
	signerNoBody htmsighttp.Signer // for requests without body (no content-digest)
	// configuration echoed from RequestSignerConfig
	signatureAlgo   string
	hashAlgo        string
	signatureFormat string
}

func NewSignerFromFile(keyPath, signatureAlgo, hashAlgo, signatureFormat string) (HTTPSigner, error) {
	keyBytes, err := os.ReadFile(filepath.Clean(keyPath))
	if err != nil {
		return nil, fmt.Errorf("failed to read request signer key from %s: %w", keyPath, err)
	}
	// derive a deterministic keyid from the private key so signatures include a stable key identifier
	keyid, err := ComputeKeyIDFromPrivateKeyPEM(string(keyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to derive keyid from private key: %w", err)
	}

	return NewSigner(string(keyBytes), keyid,
		signatureAlgo,
		hashAlgo,
		signatureFormat)
}

// NewSigner creates a signer. The signatureAlgo, hashAlgo, and format are
// currently not all mapped; for now we support defaulting to rsa/ecdsa with
// sha-256 and the default signature format. This function accepts a keyid
// parameter which will be included in the produced Signature header.
func NewSigner(privateKeyPEM string, keyid string, signatureAlgo string, hashAlgo string, signatureFormat string) (HTTPSigner, error) {
	// validate basic config values (we keep mapping to htmsig minimal for now)
	switch strings.ToLower(signatureAlgo) {
	case "", "auto", "rsa", "ecdsa":
		// allowed
	default:
		return nil, fmt.Errorf("unsupported signatureAlgo: %s", signatureAlgo)
	}

	switch strings.ToLower(hashAlgo) {
	case "", "sha256", "sha512":
		// allowed
	default:
		return nil, fmt.Errorf("unsupported hashAlgo: %s", hashAlgo)
	}

	// signatureFormat is deliberately permissive for now; accept empty or common values
	switch strings.ToLower(signatureFormat) {
	case "", "sig1", "structured", "compact", "http-signature":
		// allowed
	default:
		return nil, fmt.Errorf("unsupported signatureFormat: %s", signatureFormat)
	}

	// parse private key from PEM so htmsig picks an asymmetric signer
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to decode private key PEM")
	}

	var parsedKey any
	var err error
	switch block.Type {
	case "RSA PRIVATE KEY":
		parsedKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse RSA private key: %w", err)
		}
	case "EC PRIVATE KEY":
		parsedKey, err = x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse EC private key: %w", err)
		}
	default:
		parsedKey, err = x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("unsupported or invalid private key PEM (type=%s): %w", block.Type, err)
		}
	}

	// Validate that the configured signatureAlgo matches key type
	switch signatureAlgo {
	case "", "auto":
		// accept any; choose based on key
	case "rsa":
		if _, ok := parsedKey.(*rsa.PrivateKey); !ok {
			return nil, fmt.Errorf("signatureAlgo=rsa but key is not RSA")
		}
	case "ecdsa":
		if _, ok := parsedKey.(*ecdsa.PrivateKey); !ok {
			return nil, fmt.Errorf("signatureAlgo=ecdsa but key is not ECDSA")
		}
	default:
		return nil, fmt.Errorf("unsupported signatureAlgo: %s", signatureAlgo)
	}

	// Per Margo spec, content-digest MUST be part of the signature for requests with a body.
	// For requests without a body (e.g., GET, DELETE), content-digest is omitted.
	// See: https://docs.margo.org/specification/margo-management-interface/api-requirements-and-security
	requestSignerWithBody := htmsighttp.NewSigner(
		parsedKey,
		keyid,
		htmsighttp.WithComponents(
			component.Method(),
			component.TargetURI(),
			component.Authority(),
			component.New("content-digest"),
		))

	requestSignerNoBody := htmsighttp.NewSigner(
		parsedKey,
		keyid,
		htmsighttp.WithComponents(
			component.Method(),
			component.TargetURI(),
			component.Authority(),
		))

	return &HTMPayloadSigner{
		signer:          requestSignerWithBody,
		signerNoBody:    requestSignerNoBody,
		privateKey:      []byte(privateKeyPEM),
		signatureAlgo:   signatureAlgo,
		hashAlgo:        hashAlgo,
		signatureFormat: signatureFormat,
	}, nil
}

func (s *HTMPayloadSigner) SignRequest(ctx context.Context, req *http.Request) error {
	// Check if request has a body by reading it
	var bodyBytes []byte
	hasBody := false
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return fmt.Errorf("failed to read request body: %w", err)
		}
		hasBody = len(bodyBytes) > 0
		// restore body
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	// If body present and no Content-Digest header, compute a SHA-256 content-digest
	if hasBody && req.Header.Get("Content-Digest") == "" {
		// compute sha256
		h := sha256.Sum256(bodyBytes)
		digest := base64.StdEncoding.EncodeToString(h[:])
		// RFC9530 expects e.g., sha-256=:BASE64:
		req.Header.Set("Content-Digest", fmt.Sprintf("sha-256=:%s:", digest))
	}

	// Use appropriate signer based on body presence
	if hasBody {
		return s.signer.SignRequest(ctx, req)
	}
	return s.signerNoBody.SignRequest(ctx, req)
}

func (s *HTMPayloadSigner) SignResponse(ctx context.Context, resp http.ResponseWriter) error {
	return fmt.Errorf("the response signer is not implemented")
}
