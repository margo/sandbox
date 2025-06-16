package auth

import (
	"context"
	"fmt"
	"net/http"
)

type BearerTokenAuthenticator struct {
	token string
}

func NewBearerTokenAuthenticator(token string) *BearerTokenAuthenticator {
	return &BearerTokenAuthenticator{token: token}
}

func (bt *BearerTokenAuthenticator) Authenticate(ctx context.Context, req *http.Request) error {
	if !bt.IsValid() {
		return fmt.Errorf("bearer token not configured")
	}

	req.Header.Set("Authorization", "Bearer "+bt.token)
	return nil
}

func (bt *BearerTokenAuthenticator) Type() AuthType {
	return BearerToken
}

func (bt *BearerTokenAuthenticator) IsValid() bool {
	return bt.token != ""
}

func (bt *BearerTokenAuthenticator) Refresh(ctx context.Context) error {
	return nil // Static token doesn't need refresh
}
