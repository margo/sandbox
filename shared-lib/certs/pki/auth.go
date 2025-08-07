// Package pki provides Public Key Infrastructure (PKI) authentication capabilities
// for device onboarding and identity verification. It implements certificate-based
// authentication using digital signatures and challenge-response protocols.
package pki

import (
	"fmt"
)

// PKIAuthenticator orchestrates PKI-based authentication workflows.
// It coordinates certificate management, challenge generation, and signature
// verification to provide secure device authentication using X.509 certificates.
//
// The authenticator follows a challenge-response pattern where:
//  1. Device presents its certificate
//  2. Server generates a cryptographic challenge
//  3. Device signs the challenge with its private key
//  4. Server verifies the signature using the device's public key
type PKIAuthenticator struct {
	certManager       *CertificateManager // Handles certificate parsing, validation, and chain verification
	challengeGen      *ChallengeGenerator // Creates and validates cryptographic challenges
	signatureVerifier *SignatureVerifier  // Verifies digital signatures against public keys
}

// NewPKIAuthenticator returns a new PKI authenticator with the given dependencies.
func NewPKIAuthenticator(
	certManager *CertificateManager,
	challengeGen *ChallengeGenerator,
	signatureVerifier *SignatureVerifier,
) *PKIAuthenticator {
	return &PKIAuthenticator{
		certManager:       certManager,
		challengeGen:      challengeGen,
		signatureVerifier: signatureVerifier,
	}
}

// CreateDeviceIdentity parses a PEM certificate and returns the device identity.
// The device ID is extracted from the certificate's Subject or Subject Alternative Name.
func (auth *PKIAuthenticator) CreateDeviceIdentity(certPEM []byte) (*DeviceIdentity, error) {
	cert, err := auth.certManager.ParseDeviceCertificate(certPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	deviceID, err := auth.certManager.ExtractDeviceID(cert)
	if err != nil {
		return nil, fmt.Errorf("failed to extract device ID: %w", err)
	}

	return &DeviceIdentity{
		DeviceID:    deviceID,
		Certificate: cert,
		PublicKey:   cert.PublicKey, // Extract public key for signature verification
	}, nil
}

// ValidateDeviceIdentity validates the device's certificate chain and expiry.
func (auth *PKIAuthenticator) ValidateDeviceIdentity(identity *DeviceIdentity) error {
	// Verify the certificate chain to ensure it's signed by a trusted CA
	if err := auth.certManager.VerifyCertificateChain(identity.Certificate); err != nil {
		return fmt.Errorf("certificate chain validation failed: %w", err)
	}

	// Check certificate hasn't expired (and optionally not yet valid)
	if err := auth.certManager.ValidateCertificateExpiry(identity.Certificate); err != nil {
		return fmt.Errorf("certificate expiry validation failed: %w", err)
	}

	return nil
}

// GenerateAuthenticationChallenge creates a time-bound cryptographic challenge for the device.
func (auth *PKIAuthenticator) GenerateAuthenticationChallenge(deviceID string) (*Challenge, error) {
	return auth.challengeGen.GenerateChallenge(deviceID)
}

// VerifyAuthenticationResponse verifies the device's signature against the challenge.
// It returns an AuthenticationResult with success status and error details.
func (auth *PKIAuthenticator) VerifyAuthenticationResponse(
	identity *DeviceIdentity,
	challenge *Challenge,
	signature []byte,
) *AuthenticationResult {
	result := &AuthenticationResult{
		DeviceID:    identity.DeviceID,
		Certificate: identity.Certificate,
	}

	// Validate challenge hasn't expired and is still usable
	if !auth.challengeGen.IsValidChallenge(challenge) {
		result.ErrorMessage = "challenge has expired"
		return result
	}

	// Ensure challenge was issued for this specific device (prevents challenge reuse)
	if challenge.DeviceID != identity.DeviceID {
		result.ErrorMessage = "challenge device ID mismatch"
		return result
	}

	// Prepare signature verification input with challenge data and device's public key
	verificationInput := SignatureVerificationInput{
		Data:      challenge.Value,    // The original challenge data that was signed
		Signature: signature,          // The signature provided by the device
		PublicKey: identity.PublicKey, // Public key from the device's certificate
	}

	// Verify the signature using the appropriate algorithm (RSA, ECDSA, etc.)
	if err := auth.signatureVerifier.VerifySignature(verificationInput); err != nil {
		result.ErrorMessage = fmt.Sprintf("signature verification failed: %v", err)
		return result
	}

	result.Success = true
	return result
}

// PerformFullAuthentication performs complete PKI authentication including
// certificate validation and challenge-response verification.
func (auth *PKIAuthenticator) PerformFullAuthentication(
	certPEM []byte,
	challenge *Challenge,
	signature []byte,
) *AuthenticationResult {
	// Step 1: Create device identity from certificate
	identity, err := auth.CreateDeviceIdentity(certPEM)
	if err != nil {
		return &AuthenticationResult{
			ErrorMessage: fmt.Sprintf("failed to create device identity: %v", err),
		}
	}

	// Step 2: Validate the device's certificate
	if err := auth.ValidateDeviceIdentity(identity); err != nil {
		return &AuthenticationResult{
			DeviceID:     identity.DeviceID,
			Certificate:  identity.Certificate,
			ErrorMessage: fmt.Sprintf("certificate validation failed: %v", err),
		}
	}

	// Step 3: Verify the authentication response
	return auth.VerifyAuthenticationResponse(identity, challenge, signature)
}
