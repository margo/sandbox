package crypto

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"

	"github.com/lestrrat-go/htmsig/component"
	htmsighttp "github.com/lestrrat-go/htmsig/http"
)

type HTTPVerifier interface {
	VerifyRequest(ctx context.Context, req *http.Request) error
	VerifyResponse(ctx context.Context, resp *http.ResponseWriter) error
}

type HTMPayloadVerifier struct {
	publicKeyPEM string
	verifier     *htmsighttp.Verifier
}

func NewVerifier(publicKey string, isPubKeyBase64 bool) (*HTMPayloadVerifier, error) {
	// if the input is base64 DER, decode first
	var data []byte
	var err error
	if isPubKeyBase64 {
		data, err = base64.StdEncoding.DecodeString(publicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 public key: %w", err)
		}
	} else {
		// try to treat input as PEM text
		data = []byte(publicKey)
	}

	// If PEM encoded, decode block(s)
	var block *pem.Block
	block, rest := pem.Decode(data)
	if block != nil {
		// if there are trailing bytes that look like DER (base64 case), prefer block.Bytes
		data = block.Bytes
		// If block is a certificate, extract the public key
		if block.Type == "CERTIFICATE" {
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("failed to parse certificate PEM: %w", err)
			}
			parsedKey := cert.PublicKey
			resolver := htmsighttp.StaticKeyResolver(parsedKey)
			verifier := htmsighttp.NewVerifier(resolver, htmsighttp.WithComponents(
				component.Method(),
				component.TargetURI(),
				component.Authority(),
			))
			return &HTMPayloadVerifier{publicKeyPEM: publicKey, verifier: verifier}, nil
		}
		// else fallthrough to try parsing block.Bytes as public key
		_ = rest
	}

	// Try parse as PKIX public key (DER)
	parsedKey, parseErr := x509.ParsePKIXPublicKey(data)
	if parseErr == nil {
		resolver := htmsighttp.StaticKeyResolver(parsedKey)
		verifier := htmsighttp.NewVerifier(resolver, htmsighttp.WithComponents(
			component.Method(),
			component.TargetURI(),
			component.Authority(),
		))
		return &HTMPayloadVerifier{publicKeyPEM: publicKey, verifier: verifier}, nil
	}

	// Try parse as PKCS1 RSA public key (DER)
	if rsaPub, err := x509.ParsePKCS1PublicKey(data); err == nil {
		resolver := htmsighttp.StaticKeyResolver(rsaPub)
		verifier := htmsighttp.NewVerifier(resolver, htmsighttp.WithComponents(
			component.Method(),
			component.TargetURI(),
			component.Authority(),
		))
		return &HTMPayloadVerifier{publicKeyPEM: publicKey, verifier: verifier}, nil
	}

	// Try parse as X.509 certificate (DER)
	if cert, err := x509.ParseCertificate(data); err == nil {
		resolver := htmsighttp.StaticKeyResolver(cert.PublicKey)
		verifier := htmsighttp.NewVerifier(resolver, htmsighttp.WithComponents(
			component.Method(),
			component.TargetURI(),
			component.Authority(),
		))
		return &HTMPayloadVerifier{publicKeyPEM: publicKey, verifier: verifier}, nil
	}

	// Last resort: return original PKIX parse error for clarity
	return nil, fmt.Errorf(
		"failed to parse public key (tried PKIX, PKCS1 and certificate): %v",
		parseErr,
	)
}

func (payloadVerifier *HTMPayloadVerifier) VerifyRequest(
	ctx context.Context,
	req *http.Request,
) error {
	return payloadVerifier.verifier.VerifyRequest(ctx, req)
}

func (payloadVerifier *HTMPayloadVerifier) VerifyResponse(
	ctx context.Context,
	resp *http.ResponseWriter,
) error {
	return fmt.Errorf("response verifier is not implemented")
}
