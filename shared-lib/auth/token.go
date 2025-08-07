package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuthTokenResponse represents the structure of the token response from the OAuth 2.0 server.
type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"` // Optional, depending on grant type
}

// GetOAuthToken retrieves an OAuth 2.0 access token using the client credentials grant type.
func GetOAuthToken(ctx context.Context, clientID, clientSecret, tokenURL string) (*OAuthTokenResponse, error) {
	// 1. Prepare the request body.
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)

	// 2. Create the HTTP request.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// 3. Make the HTTP request.
	client := &http.Client{
		Timeout: 10 * time.Second, // Set a reasonable timeout
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// 4. Check the response status code.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// 5. Decode the response body.
	var tokenResponse OAuthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	return &tokenResponse, nil
}
