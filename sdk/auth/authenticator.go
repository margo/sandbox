package auth

import (
	"context"
	"net/http"
)

// AuthType represents the type of authentication
type AuthType string

const (
	NoAuth      AuthType = "none"
	BasicAuth   AuthType = "basic"
	BearerToken AuthType = "bearer"
	APIKey      AuthType = "apikey"
	// you are allowed to ingest your own authenticator implementation as well
	Custom AuthType = "custom"
)

// Authenticator defines the interface all auth plugins must implement
type Authenticator interface {
	// Authenticate applies authentication to the HTTP request
	Authenticate(ctx context.Context, req *http.Request) error

	// Type returns the authentication type this plugin handles
	Type() AuthType

	// IsValid checks if the authenticator is properly configured
	IsValid() bool

	// Refresh refreshes tokens/credentials if needed
	Refresh(ctx context.Context) error
}

// AuthPlugin defines the plugin interface for creating authenticators
type AuthPlugin interface {
	// Type returns the auth type this plugin supports
	Type() AuthType

	// Create creates a new authenticator instance from config
	Create(config map[string]interface{}) (Authenticator, error)

	// Validate validates the configuration before creating
	Validate(config map[string]interface{}) error
}
