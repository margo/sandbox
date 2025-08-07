// Package pkg provides utility functions for generating unique identifiers
// and other common operations used throughout the application.
package utils

import "github.com/google/uuid"

// GenerateAppPkgId generates a unique identifier for application packages.
//
// Returns:
//   - string: A UUID v4 string in the format "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx"
//
// Example:
//
//	pkgId := GenerateAppPkgId()
//	// Output: "550e8400-e29b-41d4-a716-446655440000"
//
// Note: Each call generates a new, cryptographically random UUID.
func GenerateAppPkgId() string {
	return generateUUID()
}

// GenerateAppDeploymentId generates a unique identifier for application instances.
//
// Returns:
//   - string: A UUID v4 string in the format "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx"
//
// Example:
//
//	instanceId := GenerateAppDeploymentId()
//	// Output: "6ba7b811-9dad-11d1-80b4-00c04fd430c8"
//
// Note: Each call generates a new, cryptographically random UUID.
func GenerateAppDeploymentId() string {
	return generateUUID()
}

// GenerateDeviceId generates a unique identifier for devices.
//
// Returns:
//   - string: A UUID v4 string in the format "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx"
//
// Example:
//
//	deviceId := GenerateDeviceId()
//	// Output: "6ba7b812-9dad-11d1-80b4-00c04fd430c8"
//
// Note: Each call generates a new, cryptographically random UUID.
func GenerateDeviceId() string {
	return generateUUID()
}

// generateUUID is a helper function that generates a UUID v4 string.
//
// Returns:
//   - string: A UUID v4 string in the format "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx"
//
// Note: This function is not exported and is intended for internal use only.
// Use the specific Generate*Id functions for different entity types instead.
func generateUUID() string {
	return uuid.New().String()
}
