// Package pki provides PKI-based device authentication using X.509 certificates.
package pki

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
)

// SignatureVerifier handles cryptographic signature verification using RSA and ECDSA algorithms.
type SignatureVerifier struct{}

// NewSignatureVerifier creates a new signature verifier.
func NewSignatureVerifier() *SignatureVerifier {
	return &SignatureVerifier{}
}

// VerifySignature verifies a signature against data using the provided public key.
// It supports RSA (PKCS#1 v1.5) and ECDSA (ASN.1) signatures with SHA-256 hashing.
func (sv *SignatureVerifier) VerifySignature(input SignatureVerificationInput) error {
	hash := sha256.Sum256(input.Data)

	switch pubKey := input.PublicKey.(type) {
	case *rsa.PublicKey:
		return rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hash[:], input.Signature)
	case *ecdsa.PublicKey:
		return sv.verifyECDSASignature(pubKey, hash[:], input.Signature)
	default:
		return fmt.Errorf("unsupported public key type: %T", input.PublicKey)
	}
}

// verifyECDSASignature verifies an ECDSA signature in ASN.1 DER format.
func (sv *SignatureVerifier) verifyECDSASignature(pubKey *ecdsa.PublicKey, hash, signature []byte) error {
	if !ecdsa.VerifyASN1(pubKey, hash, signature) {
		return fmt.Errorf("ECDSA signature verification failed")
	}
	return nil
}
