package auth

func BearerAuthCredentials(token string) map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + token,
	}
}

// BearerAuthCredentials creates a map containing the Authorization header
// with a Bearer token for authentication.
// It is used to authenticate API requests that require a Bearer token.
// The token is typically a JWT or other token format that the server recognizes.
// The returned map can be used in HTTP requests to set the Authorization header.
// Example usage:
// ```go
// headers := auth.BearerAuthCredentials("your_token_here")
