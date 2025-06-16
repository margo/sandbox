package auth

import (
	"context"
	"net/http"
)

type NoAuthenticator struct{}

func NewNoAuthenticator() *NoAuthenticator {
	return &NoAuthenticator{}
}

func (n *NoAuthenticator) Authenticate(ctx context.Context, req *http.Request) error {
	// No authentication required
	return nil
}

func (n *NoAuthenticator) Type() AuthType {
	return NoAuth
}

func (n *NoAuthenticator) IsValid() bool {
	return true
}

func (n *NoAuthenticator) Refresh(ctx context.Context) error {
	return nil
}
