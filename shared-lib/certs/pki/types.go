// Package pki provides PKI-based device authentication using X.509 certificates.
package pki

import (
	"crypto/x509"
	"time"
)

// DeviceIdentity represents a device's PKI identity containing its certificate
// and extracted public key for authentication operations.
type DeviceIdentity struct {
	DeviceID    string            // Unique device identifier extracted from certificate
	Certificate *x509.Certificate // Device's X.509 certificate
	PublicKey   interface{}       // Public key from certificate (RSA or ECDSA)
}

// Challenge represents a time-bound cryptographic challenge used in
// challenge-response authentication protocols.
type Challenge struct {
	Value     []byte    // Random bytes that must be signed by the device
	DeviceID  string    // Device identifier this challenge was issued for
	CreatedAt time.Time // When the challenge was generated
	ExpiresAt time.Time // When the challenge expires
}

// AuthenticationResult contains the outcome of PKI authentication verification
// including success status and error details.
type AuthenticationResult struct {
	Success      bool              // Whether authentication succeeded
	DeviceID     string            // Device identifier being authenticated
	Certificate  *x509.Certificate // Device certificate (if available)
	ErrorMessage string            // Error description if authentication failed
}

// SignatureVerificationInput contains the data required to verify a digital signature
// against a public key.
type SignatureVerificationInput struct {
	Data      []byte      // Original data that was signed
	Signature []byte      // Digital signature to verify
	PublicKey interface{} // Public key for verification (RSA or ECDSA)
}
