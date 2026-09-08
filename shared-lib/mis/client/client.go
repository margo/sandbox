// Package client provides a simple wrapper around the generated MIS client.
package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/margo/sandbox/mis/pkg/standard/generatedCode"
)

// MISClient wraps the generated ClientWithResponses.
type MISClient struct {
	inner      *generatedCode.ClientWithResponses
	httpClient *http.Client
}

// New creates a MISClient using the provided endpoint and Root CA PEM bytes.
// endpoint example: "https://example.com:8443"
func New(endpoint string, rootCAPEM []byte) (*MISClient, error) {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(rootCAPEM) {
		return nil, fmt.Errorf("failed to parse root CA certificate")
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: pool,
			},
		},
	}

	inner, err := generatedCode.NewClientWithResponses(
		endpoint,
		generatedCode.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}

	return &MISClient{inner: inner, httpClient: httpClient}, nil
}

// GetDiscoveryDocument calls GET /.well-known/margo and returns the parsed response.
// Pass a non-empty etag to send an If-None-Match header (conditional GET); pass "" to skip it.
func (c *MISClient) GetDiscoveryDocument(
	ctx context.Context,
	etag string,
) (*generatedCode.GetWellKnownMargoResponse, error) {
	params := &generatedCode.GetWellKnownMargoParams{}
	if etag != "" {
		params.IfNoneMatch = &etag
	}

	rsp, err := c.inner.GetWellKnownMargoWithResponse(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("GetDiscoveryDocument: %w", err)
	}
	return rsp, nil
}

// GetTrustBundle calls the SPIFFE bundle endpoint and returns the parsed response.
//
// trustBundleURL: optional override for the bundle URL (e.g. from the discovery
// document's trustBundleUri field). When empty, the default
// /.well-known/spiffe/bundle.json path on the configured server is used.
//
// etag: pass a non-empty value to send an If-None-Match header (conditional GET);
// pass "" to skip it. A 304 response will have a nil JSON200 body.
func (c *MISClient) GetTrustBundle(
	ctx context.Context,
	trustBundleURL string,
	etag string,
) (*generatedCode.GetWellKnownSpiffeBundleJsonResponse, error) {
	// Use default path via generated client when no URL override is provided.
	if trustBundleURL == "" {
		params := &generatedCode.GetWellKnownSpiffeBundleJsonParams{}
		if etag != "" {
			params.IfNoneMatch = &etag
		}
		rsp, err := c.inner.GetWellKnownSpiffeBundleJsonWithResponse(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("GetTrustBundle: %w", err)
		}
		return rsp, nil
	}

	// Custom URL path: validate, build and execute the request manually,
	// then parse into the same response type so callers have a uniform API.
	if _, err := url.Parse(trustBundleURL); err != nil {
		return nil, fmt.Errorf("GetTrustBundle: invalid trustBundleURL %q: %w", trustBundleURL, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, trustBundleURL, nil)
	if err != nil {
		return nil, fmt.Errorf("GetTrustBundle: building request: %w", err)
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	httpRsp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GetTrustBundle: executing request: %w", err)
	}
	defer httpRsp.Body.Close()

	bodyBytes, err := io.ReadAll(httpRsp.Body)
	if err != nil {
		return nil, fmt.Errorf("GetTrustBundle: reading response body: %w", err)
	}

	response := &generatedCode.GetWellKnownSpiffeBundleJsonResponse{
		Body:         bodyBytes,
		HTTPResponse: httpRsp,
	}

	switch httpRsp.StatusCode {
	case http.StatusOK:
		var dest generatedCode.SpiffeBundle
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, fmt.Errorf("GetTrustBundle: parsing bundle: %w", err)
		}
		response.JSON200 = &dest
	case http.StatusNotFound:
		var dest generatedCode.ProblemDetail
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, fmt.Errorf("GetTrustBundle: parsing problem detail: %w", err)
		}
		response.ApplicationproblemJSON404 = &dest
	}

	return response, nil
}
