// Package pki provides PKI-based device authentication using X.509 certificates.
package pki

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// PKIClient handles client-side PKI operations including challenge signing
// and certificate management for device authentication.
type PKIClient struct {
	deviceID   string
	privateKey interface{} // RSA or ECDSA private key
	certPEM    []byte
}

// NewPKIClient creates a new PKI client with the given device credentials.
// The private key must be in PEM format and can be RSA, ECDSA, PKCS#1, or PKCS#8.
func NewPKIClient(deviceID string, privateKeyPEM, certPEM []byte) (*PKIClient, error) {
	privateKey, err := parsePrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return &PKIClient{
		deviceID:   deviceID,
		privateKey: privateKey,
		certPEM:    certPEM,
	}, nil
}

// SignChallenge signs the challenge data using the device's private key.
// It supports both RSA (PKCS#1 v1.5) and ECDSA (ASN.1) signatures with SHA-256.
func (client *PKIClient) SignChallenge(challengeData []byte) ([]byte, error) {
	hash := sha256.Sum256(challengeData)

	switch privKey := client.privateKey.(type) {
	case *rsa.PrivateKey:
		return rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, hash[:])
	case *ecdsa.PrivateKey:
		return ecdsa.SignASN1(rand.Reader, privKey, hash[:])
	default:
		return nil, fmt.Errorf("unsupported private key type: %T", client.privateKey)
	}
}

// GetDeviceID returns the device identifier.
func (client *PKIClient) GetDeviceID() string {
	return client.deviceID
}

// GetCertificatePEM returns the device certificate in PEM format.
func (client *PKIClient) GetCertificatePEM() []byte {
	return client.certPEM
}

// parsePrivateKeyFromPEM parses a PEM-encoded private key in various formats.
// It attempts PKCS#1, PKCS#8, and EC private key formats.
func parsePrivateKeyFromPEM(privateKeyPEM []byte) (interface{}, error) {
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM private key")
	}

	// Try different private key formats
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	return nil, fmt.Errorf("unsupported private key format")
}
