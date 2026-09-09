// Package trustbundle provides functionality for retrieving SPIFFE trust bundles
// from a Margo Infrastructure Service (MIS) endpoint, with fallback mechanisms
// for resilience.
package trustbundle

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"

	"github.com/margo/sandbox/shared-lib/mis/client"
)

// ErrNotModified is returned by GetTrustBundle when the server responds with
// HTTP 304 Not Modified, indicating the caller's cached bundle is still current.
var ErrNotModified = errors.New("trust bundle not modified")

// Getter defines the interface for retrieving a SPIFFE trust bundle.
// Returns:
//   - trustDomain: the trust domain string (e.g. "margo.org")
//   - bundle:      the raw SPIFFE bundle in JWKS JSON format
//   - etag:        the ETag value from the MIS server response, or "" if not present
//   - err:         non-nil if the bundle could not be retrieved from any source.
//     err == ErrNotModified when the server confirmed the cached bundle
//     is still valid (HTTP 304); in that case bundle and etag are empty.
type Getter interface {
	GetTrustBundle(
		ctx context.Context,
		ietag string,
	) (trustDomain string, bundle []byte, etag string, err error)
}

// trustBundleGetterImpl is the unexported implementation of Getter.
type trustBundleGetterImpl struct {
	misEndpoint    string
	misRootCA      []byte
	trustBundleURI string
	staticBundle   []byte
	trustDomain    string
}

// New validates the provided configuration and returns a Getter backed by
// trustBundleGetterImpl.
func New(
	misEndpoint string,
	misRootCA []byte,
	trustBundleURI string,
	staticBundle []byte,
	trustDomain string,
) (Getter, error) {
	if err := validateMISEndpoint(misEndpoint); err != nil {
		return nil, fmt.Errorf("invalid MIS endpoint: %w", err)
	}

	if err := validateRootCA(misRootCA); err != nil {
		return nil, fmt.Errorf("invalid MIS root CA: %w", err)
	}

	if trustBundleURI != "" {
		if err := validateTrustBundleURI(trustBundleURI); err != nil {
			return nil, fmt.Errorf("invalid trust bundle URI: %w", err)
		}
	}

	if len(staticBundle) > 0 {
		if err := validateSPIFFEBundle(staticBundle); err != nil {
			return nil, fmt.Errorf("invalid static trust bundle: %w", err)
		}
	}

	return &trustBundleGetterImpl{
		misEndpoint:    misEndpoint,
		misRootCA:      misRootCA,
		trustBundleURI: trustBundleURI,
		staticBundle:   staticBundle,
		trustDomain:    trustDomain,
	}, nil
}

// GetTrustBundle retrieves the SPIFFE trust bundle using the following strategy:
//
//  1. Discovery: calls GET /.well-known/margo on the MIS endpoint to obtain the
//     canonical trust bundle URL and trust domain from the discovery document.
//     On success, fetches the bundle from the discovered URL and returns it.
//
//  2. URI Fallback: if discovery fails and a trustBundleURI is configured,
//     constructs the full bundle URL (misEndpoint + trustBundleURI) and fetches
//     the bundle directly.
//
//  3. Static Fallback: returns the operator-supplied static bundle and trust domain.
//
// When the server returns HTTP 304 (ETag match), GetTrustBundle returns
// (trustDomain, nil, "", ErrNotModified). The caller should continue using its
// cached bundle. The static fallback is NOT attempted on 304, since the cached
// network bundle is authoritative.
func (t *trustBundleGetterImpl) GetTrustBundle(
	ctx context.Context, ietag string,
) (string, []byte, string, error) {
	c, err := client.New(t.misEndpoint, t.misRootCA)
	if err != nil {
		return t.fallbackToURIOrStatic(ctx, ietag, fmt.Errorf("building MIS client: %w", err))
	}

	// ── Step 1: Discovery ────────────────────────────────────────────────────
	discoveryResp, err := c.GetDiscoveryDocument(ctx, "")
	if err != nil {
		return t.fallbackToURIOrStatic(ctx, ietag, fmt.Errorf("discovery request failed: %w", err))
	}

	if discoveryResp.JSON200 == nil {
		return t.fallbackToURIOrStatic(
			ctx,
			ietag,
			fmt.Errorf(
				"discovery returned non-200 status %d",
				discoveryResp.HTTPResponse.StatusCode,
			),
		)
	}

	discoveryDoc := discoveryResp.JSON200

	discoveredBundleURL := discoveryDoc.TrustBundleUri
	if discoveredBundleURL == "" {
		return t.fallbackToURIOrStatic(
			ctx, ietag,
			errors.New("discovery document contains no trustBundleUri"),
		)
	}

	discoveredTrustDomain := discoveryDoc.TrustDomain

	// ── Step 2: Fetch bundle from discovered URL ──────────────────────────────
	bundleBytes, etag, err := t.fetchBundleFromURL(ctx, c, ietag, discoveredBundleURL)
	if err != nil {
		// 304: the cached bundle is still valid — surface ErrNotModified directly.
		// Do NOT fall back to static, as the network source is authoritative.
		if errors.Is(err, ErrNotModified) {
			return discoveredTrustDomain, nil, "", ErrNotModified
		}
		return t.staticFallback(
			fmt.Errorf("fetching bundle from discovered URL %q: %w", discoveredBundleURL, err),
		)
	}

	return discoveredTrustDomain, bundleBytes, etag, nil
}

// fallbackToURIOrStatic implements steps 2 and 3 of the retrieval strategy.
func (t *trustBundleGetterImpl) fallbackToURIOrStatic(
	ctx context.Context,
	ietag string,
	discoveryErr error,
) (string, []byte, string, error) {
	// ── Step 2: URI Fallback ─────────────────────────────────────────────────
	if t.trustBundleURI != "" {
		fullBundleURL := t.misEndpoint + t.trustBundleURI

		c, clientErr := client.New(t.misEndpoint, t.misRootCA)
		if clientErr != nil {
			return t.staticFallback(fmt.Errorf(
				"discovery failed (%w); also failed to build MIS client for URI fallback: %v",
				discoveryErr, clientErr,
			))
		}

		bundleBytes, etag, fetchErr := t.fetchBundleFromURL(ctx, c, ietag, fullBundleURL)
		if fetchErr != nil {
			// 304 from the URI fallback: cached bundle is still valid.
			if errors.Is(fetchErr, ErrNotModified) {
				return t.trustDomain, nil, "", ErrNotModified
			}
			return t.staticFallback(fmt.Errorf(
				"discovery failed (%w); URI fallback to %q also failed: %v",
				discoveryErr, fullBundleURL, fetchErr,
			))
		}

		return t.trustDomain, bundleBytes, etag, nil
	}

	return t.staticFallback(discoveryErr)
}

// staticFallback implements step 3: return the operator-supplied static bundle.
func (t *trustBundleGetterImpl) staticFallback(precedingErr error) (string, []byte, string, error) {
	if len(t.staticBundle) == 0 && t.trustDomain == "" {
		return "", nil, "", fmt.Errorf(
			"%w; no static trust bundle or trust domain configured as fallback",
			precedingErr,
		)
	}

	if len(t.staticBundle) == 0 {
		return "", nil, "", fmt.Errorf(
			"%w; static fallback failed: static trust bundle is empty",
			precedingErr,
		)
	}

	if t.trustDomain == "" {
		return "", nil, "", fmt.Errorf(
			"%w; static fallback failed: trust domain is empty",
			precedingErr,
		)
	}

	return t.trustDomain, t.staticBundle, "", nil
}

// fetchBundleFromURL calls client.GetTrustBundle with the given full URL and
// returns the raw bundle bytes on HTTP 200.
//
// On HTTP 304 (Not Modified) it returns (nil, "", ErrNotModified), signalling
// that the caller's cached bundle is still current. No static fallback should
// be attempted in that case.
func (t *trustBundleGetterImpl) fetchBundleFromURL(
	ctx context.Context,
	c *client.MISClient,
	ietag string,
	fullURL string,
) ([]byte, string, error) {
	resp, err := c.GetTrustBundle(ctx, fullURL, ietag)
	if err != nil {
		return nil, "", fmt.Errorf("GetTrustBundle request: %w", err)
	}

	if resp.HTTPResponse == nil {
		return nil, "", errors.New("nil HTTP response")
	}

	switch resp.HTTPResponse.StatusCode {
	case 200:
		// handled below
	case 304:
		// ETag matched: the cached bundle is still valid.
		return nil, "", ErrNotModified
	default:
		return nil, "", fmt.Errorf("unexpected HTTP status %d", resp.HTTPResponse.StatusCode)
	}

	if resp.JSON200 == nil {
		return nil, "", errors.New("HTTP 200 response contained no parsed bundle body")
	}

	etag := resp.HTTPResponse.Header.Get("ETag")

	bundleBytes, err := json.Marshal(resp.JSON200)
	if err != nil {
		return nil, "", fmt.Errorf("re-marshalling SPIFFE bundle: %w", err)
	}

	return bundleBytes, etag, nil
}

// ── Validation helpers ────────────────────────────────────────────────────────

// validateMISEndpoint ensures the endpoint is a non-empty, parseable HTTPS URL.
func validateMISEndpoint(endpoint string) error {
	if endpoint == "" {
		return errors.New("must not be empty")
	}
	parsed, err := url.ParseRequestURI(endpoint)
	if err != nil {
		return fmt.Errorf("not a valid URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("scheme must be \"https\", got %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return errors.New("URL must include a host")
	}
	return nil
}

// validateRootCA ensures the bytes contain at least one valid PEM-encoded X.509
// CA certificate that can be parsed by the standard library.
func validateRootCA(rootCA []byte) error {
	if len(rootCA) == 0 {
		return errors.New("must not be empty")
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(rootCA) {
		return errors.New("no valid PEM-encoded X.509 certificate found")
	}
	block, _ := pem.Decode(rootCA)
	if block == nil {
		return errors.New("no PEM block found")
	}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return fmt.Errorf("parsing certificate: %w", err)
	}
	return nil
}

// validateTrustBundleURI ensures the URI is a non-empty path starting with "/".
func validateTrustBundleURI(uri string) error {
	if uri == "" {
		return errors.New("must not be empty when provided")
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		return fmt.Errorf("not a valid URI: %w", err)
	}
	if parsed.Scheme != "" || parsed.Host != "" {
		return errors.New(
			"must be a URI path (e.g. \"/.well-known/spiffe/bundle.json\"), not a full URL",
		)
	}
	if len(parsed.Path) == 0 || parsed.Path[0] != '/' {
		return errors.New("path must start with \"/\"")
	}
	return nil
}

// validateSPIFFEBundle performs a lightweight structural check to confirm the
// bytes represent a JSON object containing a "keys" array, as required by the
// SPIFFE JWKS bundle format.
func validateSPIFFEBundle(bundle []byte) error {
	if len(bundle) == 0 {
		return errors.New("must not be empty")
	}
	var doc struct {
		Keys []json.RawMessage `json:"keys"`
	}
	if err := json.Unmarshal(bundle, &doc); err != nil {
		return fmt.Errorf("not valid JSON: %w", err)
	}
	if doc.Keys == nil {
		return errors.New("SPIFFE bundle must contain a \"keys\" array")
	}
	return nil
}
