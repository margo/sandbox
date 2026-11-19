// Package client provides a simple wrapper around the generated MIS client.
package client

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/margo/sandbox/mis/pkg/standard/generatedCode"
	"github.com/margo/sandbox/shared-lib/mis/validators"
)

// TrustConfig holds the trust material used to authenticate the MIS HTTPS server.
// Exactly one of RootCAPEM or SPKIPins must be non-empty.
//
// RootCAPEM: one or more PEM-encoded CA certificates. The server chain is
// validated against these anchors with full RFC 5280 path validation and
// RFC 9525 name checks (handled by the standard TLS stack).
//
// SPKIPins: a set of SPKI SHA-256 pins in base64-encoded form per
// RFC 7469 §2.4. At least one certificate in the server's chain must
// match a pin. RFC 9525 name validation is still performed when a DNS
// name is present (see miaf2.md §5).
type TrustConfig struct {
	RootCAPEM []byte
	SPKIPins  []string // base64-encoded SHA-256 SPKI digests
}

// MISClient wraps the generated ClientWithResponses.
type MISClient struct {
	inner      *generatedCode.ClientWithResponses
	httpClient *http.Client
}

// New creates a MISClient using the provided endpoint and TrustConfig.
// in trust configuration, if both RootCAPEM & SPKIPins are provided, then RootCAPEM will be used.
// RootCAPEM takes precedence over SPKIPins
// endpoint example: "https://example.com:8443"
func New(endpoint string, trust TrustConfig) (*MISClient, error) {
	tlsCfg, err := buildTLSConfig(endpoint, trust)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
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

// buildTLSConfig constructs a tls.Config from the provided TrustConfig.
func buildTLSConfig(endpoint string, trust TrustConfig) (*tls.Config, error) {
	hasPEM := len(trust.RootCAPEM) > 0
	hasPins := len(trust.SPKIPins) > 0

	if !hasPEM && !hasPins {
		return nil, fmt.Errorf(
			"TrustConfig: at least one of RootCAPEM or SPKIPins must be provided",
		)
	}

	// Root CA takes precedence over
	if hasPEM {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(trust.RootCAPEM) {
			return nil, fmt.Errorf("failed to parse root CA certificate")
		}
		return &tls.Config{RootCAs: pool}, nil
	}

	// Pin mode: decode and validate all pins up front.
	pins, err := decodePins(trust.SPKIPins)
	if err != nil {
		return nil, fmt.Errorf("TrustConfig: invalid SPKI pin: %w", err)
	}

	// Extract hostname from endpoint URL at construction time
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint: %w", err)
	}
	serverName := parsedURL.Hostname()

	return &tls.Config{
		// InsecureSkipVerify disables default chain validation.
		// All verification (pin + hostname) is enforced in VerifyConnection,
		// which runs on both full handshakes and resumed sessions per miaf2.md §5.
		InsecureSkipVerify: true, //nolint:gosec // pin+hostname validation done in VerifyConnection
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return fmt.Errorf("tls: no peer certificates presented")
			}

			// Collect raw DER bytes from the verified chain for pin matching.
			rawCerts := make([][]byte, len(cs.PeerCertificates))
			for i, cert := range cs.PeerCertificates {
				rawCerts[i] = cert.Raw
			}

			// Pin match is always required (full handshake and resumed sessions).
			if err := verifyPins(rawCerts, pins); err != nil {
				return err
			}

			// RFC 9525 hostname verification: only for DNS names, not IP addresses.
			// Per miaf2.md §5, the pin establishes identity for IP-addressed endpoints.
			if net.ParseIP(serverName) == nil {
				if err := cs.PeerCertificates[0].VerifyHostname(serverName); err != nil {
					return fmt.Errorf("hostname verification: %w", err)
				}
			}

			return nil
		},
	}, nil
}

// decodePins parses base64-encoded SPKI SHA-256 pins (RFC 7469 §2.4).
func decodePins(encoded []string) ([][32]byte, error) {
	pins := make([][32]byte, 0, len(encoded))
	for _, s := range encoded {
		if err := validators.ValidateTrustPin(s); err != nil {
			return nil, err
		}
		b, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, "sha256/"))
		var pin [32]byte
		copy(pin[:], b)
		pins = append(pins, pin)
	}
	return pins, nil
}

// verifyPins checks that at least one certificate in the chain matches a pin.
// Per miaf2.md §5: pin the CA's SPKI, not the leaf's.
func verifyPins(rawCerts [][]byte, pins [][32]byte) error {
	for _, raw := range rawCerts {
		cert, err := x509.ParseCertificate(raw)
		if err != nil {
			return fmt.Errorf("pin verification: parsing certificate: %w", err)
		}
		spkiHash, err := spkiSHA256(cert)
		if err != nil {
			return fmt.Errorf("pin verification: computing SPKI hash: %w", err)
		}
		if slices.Contains(pins, spkiHash) {
			return nil // at least one cert in chain matches
		}
	}
	return fmt.Errorf("pin verification: no certificate in chain matched any configured SPKI pin")
}

// spkiSHA256 computes the SHA-256 digest of the certificate's
// SubjectPublicKeyInfo (DER-encoded), per RFC 7469 §2.4.
func spkiSHA256(cert *x509.Certificate) ([32]byte, error) {
	spkiDER, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return [32]byte{}, fmt.Errorf("marshalling SPKI: %w", err)
	}
	return sha256.Sum256(spkiDER), nil
}

// GetDiscoveryDocument calls GET /.well-known/margo and returns the parsed response.
// Pass a non-empty etag to send an If-None-Match header (conditional GET); pass "" to skip it.
// Callers should check rsp.HTTPResponse.StatusCode == http.StatusNotModified (304) to detect
// an unchanged document and reuse their cached copy.
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

	// 304 Not Modified: the generated client returns the raw response without
	// populating a JSON body field. Signal this to the caller via a sentinel
	// error so they know to reuse their cached document.
	if rsp.HTTPResponse.StatusCode == http.StatusNotModified {
		return rsp, nil // JSON200 will be nil; caller checks StatusCode or ETag header
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
// pass "" to skip it.
//
// Callers should check rsp.HTTPResponse.StatusCode == http.StatusNotModified (304)
// to detect an unchanged bundle and reuse their cached copy. In that case,
// JSON200 will be nil.
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
		// 304: generated client leaves JSON200 nil; caller checks StatusCode.
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

	case http.StatusNotModified:
		// 304 carries no body; JSON200 intentionally left nil.
		// The caller should reuse its cached bundle.

	case http.StatusNotFound:
		var dest generatedCode.ProblemDetail
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, fmt.Errorf("GetTrustBundle: parsing problem detail: %w", err)
		}
		response.ApplicationproblemJSON404 = &dest
	}

	return response, nil
}
