package auth

import (
	"context"
	"fmt"
	"net/http"
)

type KeyLocation string

const (
	HeaderLocation KeyLocation = "header"
	QueryLocation  KeyLocation = "query"
	CookieLocation KeyLocation = "cookie"
)

type APIKeyAuthenticator struct {
	apiKey   string
	keyName  string
	location KeyLocation
}

func NewAPIKeyAuthenticator(apiKey, keyName string, location KeyLocation) *APIKeyAuthenticator {
	return &APIKeyAuthenticator{
		apiKey:   apiKey,
		keyName:  keyName,
		location: location,
	}
}

func (ak *APIKeyAuthenticator) Authenticate(ctx context.Context, req *http.Request) error {
	if !ak.IsValid() {
		return fmt.Errorf("API key not configured")
	}

	switch ak.location {
	case HeaderLocation:
		req.Header.Set(ak.keyName, ak.apiKey)
	case QueryLocation:
		q := req.URL.Query()
		q.Set(ak.keyName, ak.apiKey)
		req.URL.RawQuery = q.Encode()
	case CookieLocation:
		req.AddCookie(&http.Cookie{
			Name:  ak.keyName,
			Value: ak.apiKey,
		})
	default:
		return fmt.Errorf("unsupported API key location: %s", ak.location)
	}

	return nil
}

func (ak *APIKeyAuthenticator) Type() AuthType {
	return APIKey
}

func (ak *APIKeyAuthenticator) IsValid() bool {
	return ak.apiKey != "" && ak.keyName != ""
}

func (ak *APIKeyAuthenticator) Refresh(ctx context.Context) error {
	return nil
}
