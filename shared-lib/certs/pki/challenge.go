// Package pki provides PKI-based device authentication using X.509 certificates.
package pki

import (
	"crypto/rand"
	"fmt"
	"time"
)

// ChallengeGenerator generates time-bound cryptographic challenges for device authentication.
// It creates random byte sequences that devices must sign to prove private key possession.
type ChallengeGenerator struct {
	challengeSize int           // Size of challenge in bytes
	defaultTTL    time.Duration // Time-to-live for generated challenges
}

// NewChallengeGenerator creates a new challenge generator with the specified parameters.
// If challengeSize is <= 0, defaults to 32 bytes. If defaultTTL is <= 0, defaults to 5 minutes.
func NewChallengeGenerator(challengeSize int, defaultTTL time.Duration) *ChallengeGenerator {
	if challengeSize <= 0 {
		challengeSize = 32 // Default 32 bytes
	}
	if defaultTTL <= 0 {
		defaultTTL = 5 * time.Minute // Default 5 minutes
	}

	return &ChallengeGenerator{
		challengeSize: challengeSize,
		defaultTTL:    defaultTTL,
	}
}

// GenerateChallenge creates a new cryptographic challenge for the specified device.
// The challenge contains random bytes that expire after the configured TTL.
func (cg *ChallengeGenerator) GenerateChallenge(deviceID string) (*Challenge, error) {
	challengeBytes := make([]byte, cg.challengeSize)
	if _, err := rand.Read(challengeBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random challenge: %w", err)
	}

	now := time.Now()
	return &Challenge{
		Value:     challengeBytes,
		DeviceID:  deviceID,
		CreatedAt: now,
		ExpiresAt: now.Add(cg.defaultTTL),
	}, nil
}

// IsValidChallenge returns true if the challenge has not expired.
func (cg *ChallengeGenerator) IsValidChallenge(challenge *Challenge) bool {
	return time.Now().Before(challenge.ExpiresAt)
}
