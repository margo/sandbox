package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
)

type BasicAuthenticator struct {
	username string
	password string
}

func NewBasicAuthenticator(username, password string) *BasicAuthenticator {
	return &BasicAuthenticator{
		username: username,
		password: password,
	}
}

func (b *BasicAuthenticator) Authenticate(ctx context.Context, req *http.Request) error {
	if !b.IsValid() {
		return fmt.Errorf("basic auth credentials not configured")
	}

	credentials := base64.StdEncoding.EncodeToString(
		[]byte(fmt.Sprintf("%s:%s", b.username, b.password)),
	)

	req.Header.Set("Authorization", "Basic "+credentials)
	return nil
}

func (b *BasicAuthenticator) Type() AuthType {
	return BasicAuth
}

func (b *BasicAuthenticator) IsValid() bool {
	return b.username != "" && b.password != ""
}

func (b *BasicAuthenticator) Refresh(ctx context.Context) error {
	return nil // Basic auth doesn't need refresh
}
