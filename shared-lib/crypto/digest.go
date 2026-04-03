package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// GetDigestOfFile calculates the SHA256 digest of a file
func GetDigestOfFile(fPath string) (digest string, err error) {
	// Validate input
	if fPath == "" {
		return "", fmt.Errorf("filepath cannot be empty")
	}

	// Open the file
	file, err := os.Open(filepath.Clean(fPath))
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", fPath, err)
	}
	defer file.Close()

	// Create SHA256 hasher
	hasher := sha256.New()

	// Copy file content to hasher
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", fPath, err)
	}

	// Calculate digest
	hash := hasher.Sum(nil)
	digest = fmt.Sprintf("sha256:%s", hex.EncodeToString(hash))

	return digest, nil
}

// GetDigestOfContent calculates the SHA256 digest of byte content
// Note: The original signature had 'filepath string' but this should be content
func GetDigestOfContent(content []byte) (digest string, err error) {
	// Create SHA256 hasher
	hasher := sha256.New()

	// Write content to hasher
	if _, err := hasher.Write(content); err != nil {
		return "", fmt.Errorf("failed to hash content: %w", err)
	}

	// Calculate digest
	hash := hasher.Sum(nil)
	digest = fmt.Sprintf("sha256:%s", hex.EncodeToString(hash))

	return digest, nil
}

// Alternative implementation if you want to keep the original signature
// GetDigestOfContentFromFile reads content from file and calculates digest
func GetDigestOfContentFromFile(fPath string) (digest string, err error) {
	// Validate input
	if fPath == "" {
		return "", fmt.Errorf("filepath cannot be empty")
	}

	// Read file content
	content, err := os.ReadFile(filepath.Clean(fPath))
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", fPath, err)
	}

	// Calculate digest of content
	return GetDigestOfContent(content)
}

// GetDigestOfString calculates the SHA256 digest of a string
func GetDigestOfString(content string) (digest string, err error) {
	return GetDigestOfContent([]byte(content))
}

// GetDigestOfReader calculates the SHA256 digest from an io.Reader
func GetDigestOfReader(reader io.Reader) (digest string, err error) {
	if reader == nil {
		return "", fmt.Errorf("reader cannot be nil")
	}

	// Create SHA256 hasher
	hasher := sha256.New()

	// Copy reader content to hasher
	if _, err := io.Copy(hasher, reader); err != nil {
		return "", fmt.Errorf("failed to read from reader: %w", err)
	}

	// Calculate digest
	hash := hasher.Sum(nil)
	digest = fmt.Sprintf("sha256:%s", hex.EncodeToString(hash))

	return digest, nil
}

// VerifyFileDigest verifies if a file matches the expected digest
func VerifyFileDigest(filepath string, expectedDigest string) (bool, error) {
	actualDigest, err := GetDigestOfFile(filepath)
	if err != nil {
		return false, err
	}
	return actualDigest == expectedDigest, nil
}

// VerifyContentDigest verifies if content matches the expected digest
func VerifyContentDigest(content []byte, expectedDigest string) (bool, error) {
	actualDigest, err := GetDigestOfContent(content)
	if err != nil {
		return false, err
	}
	return actualDigest == expectedDigest, nil
}
