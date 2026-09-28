package parser

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// CertificateFromBytes loads a tls.Certificate from certificate and private key
// bytes. Both PEM (-----BEGIN ...) and DER (raw ASN.1) formats are accepted for
// each argument independently — e.g. PEM cert with DER key is valid.
//
// PEM inputs may contain a full chain: the first block becomes the leaf and any
// subsequent blocks are appended as intermediates (tls.Certificate.Certificate).
//
// Returns an error if either input cannot be decoded/parsed, or if the cert and
// key do not form a valid pair.
func CertificateFromBytes(certBytes, keyBytes []byte) (tls.Certificate, error) {
	certPEM, err := normaliseToPEM(certBytes, "CERTIFICATE")
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("certificate: %w", err)
	}

	keyPEM, err := normaliseKeyToPEM(keyBytes)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("private key: %w", err)
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf(
			"failed to create TLS certificate from cert/key pair: %w",
			err,
		)
	}

	return cert, nil
}

// normaliseToPEM returns the input unchanged if it is already PEM-encoded,
// otherwise it wraps the raw DER bytes in a PEM block with the given type.
func normaliseToPEM(data []byte, pemType string) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("input is empty")
	}

	// If the first non-whitespace byte is '-' it is almost certainly PEM.
	if isPEM(data) {
		// Validate that at least one block can be decoded.
		block, _ := pem.Decode(data)
		if block == nil {
			return nil, fmt.Errorf("input looks like PEM but no valid block could be decoded")
		}
		return data, nil
	}

	// Treat as DER — attempt a quick parse to catch obviously wrong input.
	if _, err := x509.ParseCertificate(data); err != nil {
		return nil, fmt.Errorf(
			"input is not valid PEM and not a parseable DER certificate: %w",
			err,
		)
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  pemType,
		Bytes: data,
	}), nil
}

// normaliseKeyToPEM returns the input unchanged if it is already PEM-encoded,
// otherwise it attempts to identify the DER key type and wraps it accordingly.
//
// Supported DER formats: PKCS#8 (any algorithm), SEC 1 (EC), PKCS#1 (RSA).
func normaliseKeyToPEM(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("input is empty")
	}

	if isPEM(data) {
		block, _ := pem.Decode(data)
		if block == nil {
			return nil, fmt.Errorf("input looks like PEM but no valid block could be decoded")
		}
		return data, nil
	}

	// Try to identify the DER key format and choose the correct PEM type.
	pemType, err := derKeyPEMType(data)
	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  pemType,
		Bytes: data,
	}), nil
}

// derKeyPEMType attempts to parse DER key bytes and returns the appropriate
// PEM block type string. Tries PKCS#8 first (most common modern format),
// then EC (SEC 1), then RSA (PKCS#1).
func derKeyPEMType(data []byte) (string, error) {
	// PKCS#8 — covers RSA, ECDSA, Ed25519, etc.
	if _, err := x509.ParsePKCS8PrivateKey(data); err == nil {
		return "PRIVATE KEY", nil
	}

	// SEC 1 — EC private key.
	if _, err := x509.ParseECPrivateKey(data); err == nil {
		return "EC PRIVATE KEY", nil
	}

	// PKCS#1 — RSA private key.
	if _, err := x509.ParsePKCS1PrivateKey(data); err == nil {
		return "RSA PRIVATE KEY", nil
	}

	return "", fmt.Errorf(
		"input is not valid PEM and could not be parsed as PKCS#8, EC (SEC1), or RSA (PKCS#1) DER key",
	)
}

// isPEM reports whether data appears to be PEM-encoded by checking for the
// leading "-----" marker (ignoring leading whitespace/newlines).
func isPEM(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	return bytes.HasPrefix(trimmed, []byte("-----"))
}
