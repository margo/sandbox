package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/margo/sandbox/shared-lib/http/auth"
)

// NewGetRequest creates a new GET HTTP request with authentication and query parameters
func NewGetRequest(
	ctx context.Context,
	url string,
	auth *auth.AuthConfig,
	queryParams map[string]interface{},
) (*http.Request, error) {
	finalURL, err := buildURLWithParams(url, queryParams)
	if err != nil {
		return nil, fmt.Errorf("failed to build URL with parameters: %w", err)
	}

	// Use NewRequestWithContext
	req, err := http.NewRequestWithContext(ctx, "GET", finalURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GET request: %w", err)
	}

	if err := applyAuthentication(req, auth); err != nil {
		return nil, fmt.Errorf("failed to apply authentication: %w", err)
	}

	setDefaultHeaders(req)
	return req, nil
}

// NewPostRequest creates a new POST HTTP request with authentication and body
func NewPostRequest(
	ctx context.Context,
	url string,
	auth *auth.AuthConfig,
	body interface{},
	contentType string,
) (*http.Request, error) {
	var bodyReader io.Reader
	var err error

	if body != nil {
		bodyReader, err = prepareRequestBody(body, contentType)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare request body: %w", err)
		}
	}

	// Use NewRequestWithContext
	req, err := http.NewRequestWithContext(ctx, "POST", url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create POST request: %w", err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	} else if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if err := applyAuthentication(req, auth); err != nil {
		return nil, fmt.Errorf("failed to apply authentication: %w", err)
	}

	setDefaultHeaders(req)
	return req, nil
}

// NewPutRequest creates a new PUT HTTP request with authentication and body
func NewPutRequest(
	ctx context.Context,
	url string,
	auth *auth.AuthConfig,
	body interface{},
	contentType string,
) (*http.Request, error) {
	var bodyReader io.Reader
	var err error

	if body != nil {
		bodyReader, err = prepareRequestBody(body, contentType)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare request body: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create PUT request: %w", err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	} else if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if err := applyAuthentication(req, auth); err != nil {
		return nil, fmt.Errorf("failed to apply authentication: %w", err)
	}

	setDefaultHeaders(req)
	return req, nil
}

// NewPatchRequest creates a new PATCH HTTP request with authentication and body
func NewPatchRequest(
	ctx context.Context,
	url string,
	auth *auth.AuthConfig,
	body interface{},
	contentType string,
) (*http.Request, error) {
	var bodyReader io.Reader
	var err error

	if body != nil {
		bodyReader, err = prepareRequestBody(body, contentType)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare request body: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create PATCH request: %w", err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	} else if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if err := applyAuthentication(req, auth); err != nil {
		return nil, fmt.Errorf("failed to apply authentication: %w", err)
	}

	setDefaultHeaders(req)
	return req, nil
}

// NewDeleteRequest creates a new DELETE HTTP request with authentication and optional query
// parameters
func NewDeleteRequest(
	ctx context.Context,
	url string,
	auth *auth.AuthConfig,
	queryParams map[string]interface{},
) (*http.Request, error) {
	finalURL, err := buildURLWithParams(url, queryParams)
	if err != nil {
		return nil, fmt.Errorf("failed to build URL with parameters: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "DELETE", finalURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create DELETE request: %w", err)
	}

	if err := applyAuthentication(req, auth); err != nil {
		return nil, fmt.Errorf("failed to apply authentication: %w", err)
	}

	setDefaultHeaders(req)
	return req, nil
}

// NewHeadRequest creates a new HEAD HTTP request with authentication and query parameters
func NewHeadRequest(
	ctx context.Context,
	url string,
	auth *auth.AuthConfig,
	queryParams map[string]interface{},
) (*http.Request, error) {
	finalURL, err := buildURLWithParams(url, queryParams)
	if err != nil {
		return nil, fmt.Errorf("failed to build URL with parameters: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "HEAD", finalURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HEAD request: %w", err)
	}

	if err := applyAuthentication(req, auth); err != nil {
		return nil, fmt.Errorf("failed to apply authentication: %w", err)
	}

	setDefaultHeaders(req)
	return req, nil
}

// NewOptionsRequest creates a new OPTIONS HTTP request with authentication
func NewOptionsRequest(
	ctx context.Context,
	url string,
	auth *auth.AuthConfig,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "OPTIONS", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create OPTIONS request: %w", err)
	}

	if err := applyAuthentication(req, auth); err != nil {
		return nil, fmt.Errorf("failed to apply authentication: %w", err)
	}

	setDefaultHeaders(req)
	return req, nil
}

// Helper function to build URL with query parameters
func buildURLWithParams(baseURL string, queryParams map[string]interface{}) (string, error) {
	if len(queryParams) == 0 {
		return baseURL, nil
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	query := parsedURL.Query()
	for key, value := range queryParams {
		if value != nil {
			query.Add(key, fmt.Sprintf("%v", value))
		}
	}

	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

// Helper function to prepare request body based on content type
func prepareRequestBody(body interface{}, contentType string) (io.Reader, error) {
	switch {
	case strings.Contains(contentType, "application/json") || contentType == "":
		// JSON body
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON body: %w", err)
		}
		return bytes.NewReader(jsonData), nil

	case strings.Contains(contentType, "application/x-www-form-urlencoded"):
		// Form data
		if formData, ok := body.(map[string]string); ok {
			values := url.Values{}
			for key, value := range formData {
				values.Set(key, value)
			}
			return strings.NewReader(values.Encode()), nil
		}
		return nil, fmt.Errorf("form data must be map[string]string")

	case strings.Contains(contentType, "text/plain"):
		// Plain text
		if text, ok := body.(string); ok {
			return strings.NewReader(text), nil
		}
		return nil, fmt.Errorf("plain text body must be string")

	case strings.Contains(contentType, "application/xml"):
		// XML body (assuming body is already XML string)
		if xmlData, ok := body.(string); ok {
			return strings.NewReader(xmlData), nil
		}
		return nil, fmt.Errorf("XML body must be string")

	default:
		// Try to handle as bytes or string
		switch v := body.(type) {
		case []byte:
			return bytes.NewReader(v), nil
		case string:
			return strings.NewReader(v), nil
		case io.Reader:
			return v, nil
		default:
			// Fallback to JSON
			jsonData, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal body: %w", err)
			}
			return bytes.NewReader(jsonData), nil
		}
	}
}

// Helper function to apply authentication to request
func applyAuthentication(req *http.Request, authReq *auth.AuthConfig) error {
	if authReq == nil {
		return nil
	}

	switch authReq.Type {
	case auth.AuthTypeNone:
		// No authentication
		return nil

	case auth.AuthTypeBasic:
		if authReq.Username == "" || authReq.Password == "" {
			return fmt.Errorf("username and password required for basic authentication")
		}
		req.SetBasicAuth(authReq.Username, authReq.Password)

	case auth.AuthTypeBearer:
		if authReq.Token == "" {
			return fmt.Errorf("token required for bearer authentication")
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authReq.Token))

	case auth.AuthTypeAPIKey:
		if authReq.APIKey == "" {
			return fmt.Errorf("API key required for API key authentication")
		}
		req.Header.Set("X-API-Key", authReq.APIKey)

	case auth.AuthTypeCustom:
		if len(authReq.Headers) == 0 {
			return fmt.Errorf("custom headers required for custom authentication")
		}
		for key, value := range authReq.Headers {
			if key != "" && value != "" {
				req.Header.Set(key, value)
			}
		}

	default:
		return fmt.Errorf("unsupported authentication type: %s", authReq.Type)
	}

	return nil
}

// Helper function to set default headers
func setDefaultHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "margo-device-agent/1.0")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	// Set Accept-Encoding for compression support
	req.Header.Set("Accept-Encoding", "gzip, deflate")

	// Set Connection header for keep-alive
	req.Header.Set("Connection", "keep-alive")
}
