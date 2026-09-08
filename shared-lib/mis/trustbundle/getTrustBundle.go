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

// Getter defines the interface for retrieving a SPIFFE trust bundle.
// Returns:
//   - trustDomain: the trust domain string (e.g. "margo.org")
//   - bundle:      the raw SPIFFE bundle in JWKS JSON format
//   - err:         non-nil if the bundle could not be retrieved from any source
type Getter interface {
	GetTrustBundle(ctx context.Context) (trustDomain string, bundle []byte, err error)
}

// trustBundleGetterImpl is the unexported implementation of Getter.
// It holds all configuration needed to retrieve a SPIFFE trust bundle,
// with multiple fallback strategies.
type trustBundleGetterImpl struct {
	// misEndpoint is the base URL of the MIS server (e.g. "https://mis.margo.org:9443").
	misEndpoint string

	// misRootCA is the DER- or PEM-encoded X.509 CA certificate used to
	// establish TLS trust with the MIS server.
	misRootCA []byte

	// trustBundleURI is the well-known URI path for the SPIFFE bundle endpoint
	// (e.g. "/.well-known/spiffe/bundle.json"). When non-empty it is combined
	// with misEndpoint to form the full bundle URL used as a fallback.
	trustBundleURI string

	// staticBundle is an operator-supplied SPIFFE bundle in JWKS JSON format,
	// read from a .json file. Used as the last-resort fallback when all
	// network-based retrieval attempts fail.
	staticBundle []byte

	// trustDomain is the SPIFFE trust domain (e.g. "margo.org") associated
	// with the bundle. Returned alongside the bundle bytes.
	trustDomain string
}

// New validates the provided configuration and returns a Getter backed by
// trustBundleGetterImpl.
//
// Parameters:
//   - misEndpoint:     base URL of the MIS server, must be a valid HTTPS URL.
//   - misRootCA:       PEM-encoded X.509 CA certificate for TLS verification.
//   - trustBundleURI:  optional URI path for the SPIFFE bundle (may be empty).
//   - staticBundle:    optional operator-supplied SPIFFE bundle JSON (may be nil/empty).
//   - trustDomain:     SPIFFE trust domain string (may be empty if not yet known).
//
// Returns an error if any supplied value fails validation.
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
//     the bundle directly. Returns the bundle with the configured trust domain.
//
//  3. Static Fallback: if step 2 is skipped or also fails, returns the
//     operator-supplied static bundle and trust domain. Both must be non-empty
//     for this step to succeed.
//
// Returns the trust domain string, raw SPIFFE bundle bytes, and any error.
func (t *trustBundleGetterImpl) GetTrustBundle(ctx context.Context) (string, []byte, error) {
	client, err := client.New(t.misEndpoint, t.misRootCA)
	if err != nil {
		// Client construction failure is non-fatal; we fall through to URI/static fallbacks.
		return t.fallbackToURIOrStatic(ctx, fmt.Errorf("building MIS client: %w", err))
	}

	// ── Step 1: Discovery ────────────────────────────────────────────────────
	discoveryResp, err := client.GetDiscoveryDocument(ctx, "")
	if err != nil {
		return t.fallbackToURIOrStatic(ctx, fmt.Errorf("discovery request failed: %w", err))
	}

	if discoveryResp.JSON200 == nil {
		return t.fallbackToURIOrStatic(
			ctx,
			fmt.Errorf(
				"discovery returned non-200 status %d",
				discoveryResp.HTTPResponse.StatusCode,
			),
		)
	}

	discoveryDoc := discoveryResp.JSON200

	// Extract the trust bundle URL advertised by the discovery document.
	discoveredBundleURL := discoveryDoc.TrustBundleUri
	if discoveredBundleURL == "" {
		return t.fallbackToURIOrStatic(
			ctx,
			errors.New("discovery document contains no trustBundleUri"),
		)
	}

	// Extract the trust domain advertised by the discovery document.
	discoveredTrustDomain := discoveryDoc.TrustDomain

	// ── Step 4: Fetch bundle from discovered URL ──────────────────────────────
	bundleBytes, err := t.fetchBundleFromURL(ctx, client, discoveredBundleURL)
	if err != nil {
		// Step 5: Discovery succeeded but bundle fetch failed — fall back to static.
		return t.staticFallback(
			fmt.Errorf("fetching bundle from discovered URL %q: %w", discoveredBundleURL, err),
		)
	}

	return discoveredTrustDomain, bundleBytes, nil
}

// fallbackToURIOrStatic implements steps 2 and 3 of the retrieval strategy.
// It is called whenever the discovery step (step 1) fails for any reason.
func (t *trustBundleGetterImpl) fallbackToURIOrStatic(
	ctx context.Context,
	discoveryErr error,
) (string, []byte, error) {
	// ── Step 2: URI Fallback ─────────────────────────────────────────────────
	if t.trustBundleURI != "" {
		fullBundleURL := t.misEndpoint + t.trustBundleURI

		client, clientErr := client.New(t.misEndpoint, t.misRootCA)
		if clientErr != nil {
			// Cannot build client; skip to static fallback.
			return t.staticFallback(fmt.Errorf(
				"discovery failed (%w); also failed to build MIS client for URI fallback: %v",
				discoveryErr, clientErr,
			))
		}

		bundleBytes, fetchErr := t.fetchBundleFromURL(ctx, client, fullBundleURL)
		if fetchErr != nil {
			return t.staticFallback(fmt.Errorf(
				"discovery failed (%w); URI fallback to %q also failed: %v",
				discoveryErr, fullBundleURL, fetchErr,
			))
		}

		return t.trustDomain, bundleBytes, nil
	}

	// trustBundleURI is empty — skip step 2 and proceed directly to step 3.
	return t.staticFallback(discoveryErr)
}

// staticFallback implements step 3: return the operator-supplied static bundle.
// Both staticBundle and trustDomain must be non-empty; otherwise an error is returned.
func (t *trustBundleGetterImpl) staticFallback(precedingErr error) (string, []byte, error) {
	if len(t.staticBundle) == 0 && t.trustDomain == "" {
		return "", nil, fmt.Errorf(
			"%w; no static trust bundle or trust domain configured as fallback",
			precedingErr,
		)
	}

	if len(t.staticBundle) == 0 {
		return "", nil, fmt.Errorf(
			"%w; static fallback failed: static trust bundle is empty",
			precedingErr,
		)
	}

	if t.trustDomain == "" {
		return "", nil, fmt.Errorf(
			"%w; static fallback failed: trust domain is empty",
			precedingErr,
		)
	}

	return t.trustDomain, t.staticBundle, nil
}

// fetchBundleFromURL calls client.GetTrustBundle with the given full URL
// and returns the raw bundle bytes on HTTP 200.
func (t *trustBundleGetterImpl) fetchBundleFromURL(
	ctx context.Context,
	client *client.MISClient,
	fullURL string,
) ([]byte, error) {
	resp, err := client.GetTrustBundle(ctx, fullURL, "")
	if err != nil {
		return nil, fmt.Errorf("GetTrustBundle request: %w", err)
	}

	if resp.HTTPResponse == nil {
		return nil, errors.New("nil HTTP response")
	}

	if resp.HTTPResponse.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected HTTP status %d", resp.HTTPResponse.StatusCode)
	}

	if resp.JSON200 == nil {
		return nil, errors.New("HTTP 200 response contained no parsed bundle body")
	}

	// Re-marshal the parsed SpiffeBundle back to JSON so callers receive
	// a stable, canonical byte representation.
	bundleBytes, err := json.Marshal(resp.JSON200)
	if err != nil {
		return nil, fmt.Errorf("re-marshalling SPIFFE bundle: %w", err)
	}

	return bundleBytes, nil
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

	// Verify at least one block is actually parseable.
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

	// A URI-path-only value must not carry a scheme or host.
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

// stringFromPtr safely dereferences a *string, returning "" for nil pointers.
func stringFromPtr(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
