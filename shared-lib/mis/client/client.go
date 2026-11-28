// Package client provides a simple wrapper for fetching MIS discovery documents
// and SPIFFE trust bundles from full, caller-supplied URLs.
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

// TrustConfig holds the trust material used to authenticate MIS HTTPS servers.
// At least one of RootCAPEM or SPKIPins must be non-empty.
//
// The trust mode is selected per request from the URL host:
//   - IP address host: SPKIPins are required. At least one certificate in the
//     server's chain must match a pin (RFC 7469 §2.4, miaf2.md §5). Hostname
//     verification is not performed because the pin establishes identity.
//   - DNS name host: RootCAPEM is required. The server chain is validated
//     against these anchors with full RFC 5280 path validation and RFC 9525
//     name checks (handled by the standard TLS stack).
//
// RootCAPEM: one or more PEM-encoded CA certificates.
//
// SPKIPins: base64-encoded SHA-256 SPKI digests in the form "sha256/<base64>".
type TrustConfig struct {
	RootCAPEM []byte
	SPKIPins  []string // "sha256/<base64-encoded SHA-256 SPKI digest>"
}

// MISClient fetches MIS documents from caller-supplied URLs using the trust
// material provided at construction time.
type MISClient struct {
	trust TrustConfig
}

// New creates a MISClient using the provided TrustConfig.
// The TrustConfig is validated up front; the trust mode for each request is
// selected later from the URL passed to GetDiscoveryDocument or GetTrustBundle.
func New(trust TrustConfig) (*MISClient, error) {
	hasPEM := len(trust.RootCAPEM) > 0
	hasPins := len(trust.SPKIPins) > 0

	if !hasPEM && !hasPins {
		return nil, fmt.Errorf(
			"TrustConfig: at least one of RootCAPEM or SPKIPins must be provided",
		)
	}

	if hasPEM {
		if _, err := parseRootCAPool(trust.RootCAPEM); err != nil {
			return nil, err
		}
	}

	if hasPins {
		if _, err := decodePins(trust.SPKIPins); err != nil {
			return nil, fmt.Errorf("TrustConfig: invalid SPKI pin: %w", err)
		}
	}

	return &MISClient{trust: trust}, nil
}

// parseRootCAPool parses PEM-encoded CA certificates into a cert pool.
func parseRootCAPool(rootCAPEM []byte) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(rootCAPEM) {
		return nil, fmt.Errorf("failed to parse root CA certificate")
	}
	return pool, nil
}

// buildTLSConfig validates the URL and constructs a tls.Config for it using
// the trust mode that matches the URL host (IP: pins, DNS name: root CA).
func buildTLSConfig(rawURL string, trust TrustConfig) (*tls.Config, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	if parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("URL %q: scheme must be https", rawURL)
	}

	host := parsedURL.Hostname()
	if host == "" {
		return nil, fmt.Errorf("URL %q: host must not be empty", rawURL)
	}

	// IP-addressed URL: SPKI pin mode.
	if net.ParseIP(host) != nil {
		if len(trust.SPKIPins) == 0 {
			return nil, fmt.Errorf("URL %q: IP address host requires SPKIPins in TrustConfig", rawURL)
		}

		pins, err := decodePins(trust.SPKIPins)
		if err != nil {
			return nil, fmt.Errorf("TrustConfig: invalid SPKI pin: %w", err)
		}

		return &tls.Config{
			// InsecureSkipVerify disables default chain validation.
			// All verification (pin) is enforced in VerifyConnection,
			// which runs on both full handshakes and resumed sessions per miaf2.md §5.
			InsecureSkipVerify: true, //nolint:gosec // pin validation done in VerifyConnection
			VerifyConnection: func(cs tls.ConnectionState) error {
				if len(cs.PeerCertificates) == 0 {
					return fmt.Errorf("tls: no peer certificates presented")
				}

				// Collect raw DER bytes from the presented chain for pin matching.
				rawCerts := make([][]byte, len(cs.PeerCertificates))
				for i, cert := range cs.PeerCertificates {
					rawCerts[i] = cert.Raw
				}

				// Pin match is always required (full handshake and resumed sessions).
				return verifyPins(rawCerts, pins)
			},
		}, nil
	}

	// DNS-named URL: root CA mode with standard chain and name validation.
	if len(trust.RootCAPEM) == 0 {
		return nil, fmt.Errorf("URL %q: DNS name host requires RootCAPEM in TrustConfig", rawURL)
	}

	pool, err := parseRootCAPool(trust.RootCAPEM)
	if err != nil {
		return nil, err
	}

	return &tls.Config{RootCAs: pool}, nil
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

// doGet validates the URL and trust material, performs a GET request to the
// full URL (with optional If-None-Match), and returns the response along with
// its fully-read body.
func (c *MISClient) doGet(ctx context.Context, rawURL, etag string) (*http.Response, []byte, error) {
	tlsCfg, err := buildTLSConfig(rawURL, c.trust)
	if err != nil {
		return nil, nil, err
	}

	transport := &http.Transport{TLSClientConfig: tlsCfg}
	defer transport.CloseIdleConnections()
	httpClient := &http.Client{Transport: transport}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("building request: %w", err)
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	httpRsp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("executing request: %w", err)
	}
	defer httpRsp.Body.Close()

	bodyBytes, err := io.ReadAll(httpRsp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("reading response body: %w", err)
	}

	return httpRsp, bodyBytes, nil
}

// etagFromResponse returns the ETag header value, or nil if absent.
func etagFromResponse(rsp *http.Response) *string {
	v := rsp.Header.Get("ETag")
	if v == "" {
		return nil
	}
	return &v
}

// GetDiscoveryDocument calls the supplied discovery document URL and returns the parsed response.
// discoveryURL: full absolute HTTPS URL of the discovery document (e.g. https://192.0.2.10/.well-known/margo).
// Pass a non-empty etag to send an If-None-Match header (conditional GET); pass "" to skip it.
// Callers should check rsp.HTTPResponse.StatusCode == http.StatusNotModified (304) to detect
// an unchanged document and reuse their cached copy. In that case, JSON200 will be nil.
func (c *MISClient) GetDiscoveryDocument(
	ctx context.Context,
	discoveryURL string,
	etag string,
) (*generatedCode.GetWellKnownMargoResponse, error) {
	if discoveryURL == "" {
		return nil, fmt.Errorf("GetDiscoveryDocument: discoveryURL must not be empty")
	}

	httpRsp, bodyBytes, err := c.doGet(ctx, discoveryURL, etag)
	if err != nil {
		return nil, fmt.Errorf("GetDiscoveryDocument: %w", err)
	}

	response := &generatedCode.GetWellKnownMargoResponse{
		Body:         bodyBytes,
		HTTPResponse: httpRsp,
	}

	switch httpRsp.StatusCode {
	case http.StatusOK:
		var dest generatedCode.DiscoveryDocument
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, fmt.Errorf("GetDiscoveryDocument: parsing discovery document: %w", err)
		}
		response.JSON200 = &dest
		response.Headers200 = &generatedCode.GetWellKnownMargoResponse200Headers{
			ETag: etagFromResponse(httpRsp),
		}

	case http.StatusNotModified:
		// 304 carries no body; JSON200 intentionally left nil.
		// The caller should reuse its cached discovery document.

	case http.StatusNotFound:
		var dest generatedCode.ProblemDetail
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, fmt.Errorf("GetDiscoveryDocument: parsing problem detail: %w", err)
		}
		response.ApplicationproblemJSON404 = &dest
	}

	return response, nil
}

// GetTrustBundle calls the supplied SPIFFE bundle URL and returns the parsed response.
// trustBundleURL: full absolute HTTPS URL of the bundle, typically the trustBundleUri
// value from the discovery document or operator configuration.
// Pass a non-empty etag to send an If-None-Match header (conditional GET); pass "" to skip it.
// Callers should check rsp.HTTPResponse.StatusCode == http.StatusNotModified (304)
// to detect an unchanged bundle and reuse their cached copy. In that case,
// JSON200 will be nil.
func (c *MISClient) GetTrustBundle(
	ctx context.Context,
	trustBundleURL string,
	etag string,
) (*generatedCode.GetWellKnownSpiffeBundleJsonResponse, error) {
	if trustBundleURL == "" {
		return nil, fmt.Errorf("GetTrustBundle: trustBundleURL must not be empty")
	}

	httpRsp, bodyBytes, err := c.doGet(ctx, trustBundleURL, etag)
	if err != nil {
		return nil, fmt.Errorf("GetTrustBundle: %w", err)
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
		response.Headers200 = &generatedCode.GetWellKnownSpiffeBundleJsonResponse200Headers{
			ETag: etagFromResponse(httpRsp),
		}

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
