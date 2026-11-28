// Package trustbundle provides functionality for retrieving SPIFFE trust bundles
// from a Margo Identity Service (MIS), with fallback mechanisms for resilience.
package trustbundle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/margo/sandbox/shared-lib/mis/client"
	"github.com/margo/sandbox/shared-lib/mis/validators"
)

// ErrNotModified is returned by GetTrustBundle when the server responds with
// HTTP 304 Not Modified, indicating the caller's cached bundle is still current.
var ErrNotModified = errors.New("trust bundle not modified")

// Getter defines the interface for retrieving a SPIFFE trust bundle.
// Returns:
//   - trustDomain: the trust domain from the discovery document when discovery
//     succeeds. In every other case, the TrustDomain configured in
//     TrustBundleGetterConfig is returned.
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
	discoveryURL   string
	misRootCA      []byte
	misTrustPins   []string
	trustBundleURI string
	staticBundle   []byte
	trustDomain    string
}

// TrustBundleGetterConfig holds all inputs required to construct a Getter.
type TrustBundleGetterConfig struct {
	// DiscoveryURL is the full absolute HTTPS URL of the MIS discovery document,
	// for example "https://192.0.2.10/.well-known/margo".
	// Optional. When empty, discovery is skipped.
	DiscoveryURL string

	// MISRootCA is the PEM-encoded CA certificate passed to the MIS client.
	// The MIS client uses it for DNS-named MIS URLs.
	MISRootCA []byte

	// MISTrustPins is a list of base64-encoded SHA-256 SPKI pins (RFC 7469 §2.4).
	// Passed to the MIS client, which uses them for IP-addressed MIS URLs.
	MISTrustPins []string

	// TrustBundleURI is the full absolute HTTPS URL of the SPIFFE bundle.
	// Used as a fallback when discovery fails or DiscoveryURL is not configured.
	TrustBundleURI string

	// StaticBundle is an operator-supplied SPIFFE trust bundle in JWKS format.
	// Used as the final fallback.
	StaticBundle []byte

	// TrustDomain is the trust domain returned to the caller whenever the
	// bundle does not come from discovery (URI fallback, static fallback, or
	// a 304 from those sources). When discovery succeeds, the trust domain from
	// the discovery document is returned instead.
	// Required when StaticBundle is configured.
	TrustDomain string
}

// New validates the provided configuration and returns a Getter backed by
// trustBundleGetterImpl.
//
// StaticBundle and TrustDomain are always required. A network source
// (DiscoveryURL and/or TrustBundleURI) is optional, but when one is set a
// trust anchor (MISRootCA or MISTrustPins) is also required.
//
// Valid configurations:
//   - Discovery mode:     DiscoveryURL + trust anchor + StaticBundle + TrustDomain
//   - URI mode:           TrustBundleURI + trust anchor + StaticBundle + TrustDomain
//   - Discovery and URI:  DiscoveryURL + TrustBundleURI + trust anchor + StaticBundle + TrustDomain
//   - Static bundle mode: StaticBundle + TrustDomain (no network configuration)
//
// Network sources (DiscoveryURL and/or TrustBundleURI) are tried first, with
// StaticBundle as the final fallback. Both MISRootCA and MISTrustPins are passed
// to the MIS client unchanged, which selects the trust mode per URL.
func New(cfg TrustBundleGetterConfig) (Getter, error) {
	if err := validateNewInputs(cfg); err != nil {
		return nil, err
	}
	return &trustBundleGetterImpl{
		discoveryURL:   cfg.DiscoveryURL,
		misRootCA:      cfg.MISRootCA,
		misTrustPins:   cfg.MISTrustPins,
		trustBundleURI: cfg.TrustBundleURI,
		staticBundle:   cfg.StaticBundle,
		trustDomain:    cfg.TrustDomain,
	}, nil
}

// GetTrustBundle retrieves the SPIFFE trust bundle using the following strategy:
//
//  1. Discovery: if DiscoveryURL is configured, fetches the discovery document
//     and then fetches the bundle from its trustBundleUri. On success, returns
//     the trust domain from the discovery document together with the bundle.
//
//  2. URI Fallback: if discovery fails, or DiscoveryURL is not configured, and
//     TrustBundleURI is configured, fetches the bundle directly from
//     TrustBundleURI. The configured TrustDomain is returned.
//
//  3. Static Fallback: if the previous steps failed or could not be performed
//     (no DiscoveryURL and no TrustBundleURI, or a fetch/validation failure),
//     returns the operator-supplied static bundle. The configured TrustDomain
//     is returned.
//
// When the server returns HTTP 304 (ETag match), GetTrustBundle returns
// (trustDomain, nil, "", ErrNotModified). The trust domain is the discovered
// one when the 304 came from the discovered bundle, and the configured one
// otherwise. The static fallback is NOT attempted on 304, since the cached
// network bundle is authoritative.
func (t *trustBundleGetterImpl) GetTrustBundle(
	ctx context.Context, ietag string,
) (string, []byte, string, error) {
	c, err := t.newClient()
	if err != nil {
		return t.staticFallback(fmt.Errorf("building MIS client: %w", err))
	}

	// ── Step 1: Discovery ────────────────────────────────────────────────────
	if t.discoveryURL == "" {
		return t.fallbackToURIOrStatic(
			ctx, c, ietag,
			errors.New("discovery skipped: no DiscoveryURL configured"),
		)
	}

	discoveryResp, err := c.GetDiscoveryDocument(ctx, t.discoveryURL, "")
	if err != nil {
		return t.fallbackToURIOrStatic(
			ctx, c, ietag,
			fmt.Errorf("discovery request to %q failed: %w", t.discoveryURL, err),
		)
	}

	if discoveryResp.JSON200 == nil {
		return t.fallbackToURIOrStatic(
			ctx, c, ietag,
			fmt.Errorf(
				"discovery at %q returned non-200 status %d",
				t.discoveryURL,
				discoveryResp.HTTPResponse.StatusCode,
			),
		)
	}

	discoveryDoc := discoveryResp.JSON200

	discoveredBundleURL := discoveryDoc.TrustBundleUri
	if discoveredBundleURL == "" {
		return t.fallbackToURIOrStatic(
			ctx, c, ietag,
			errors.New("discovery document contains no trustBundleUri"),
		)
	}

	// ── Step 2: Fetch bundle from discovered URL ─────────────────────────────
	bundleBytes, etag, err := t.fetchBundleFromURL(ctx, c, ietag, discoveredBundleURL)
	if err != nil {
		// 304: the cached bundle is still valid. Surface ErrNotModified directly
		// and do NOT fall back to static, as the network source is authoritative.
		if errors.Is(err, ErrNotModified) {
			return discoveryDoc.TrustDomain, nil, "", ErrNotModified
		}
		return t.fallbackToURIOrStatic(
			ctx, c, ietag,
			fmt.Errorf("fetching bundle from discovered URL %q: %w", discoveredBundleURL, err),
		)
	}

	if err := validators.ValidateSpiffeTrustBundle(bundleBytes); err != nil {
		return t.fallbackToURIOrStatic(
			ctx,
			c,
			ietag,
			fmt.Errorf(
				"bundle from discovered URL %q failed validation: %w",
				discoveredBundleURL,
				err,
			),
		)
	}

	return discoveryDoc.TrustDomain, bundleBytes, etag, nil
}

// newClient builds an MIS client from the configured trust material.
// Both RootCAPEM and SPKIPins are passed through unchanged. The MIS client
// selects the trust mode per URL and returns any resulting error.
func (t *trustBundleGetterImpl) newClient() (*client.MISClient, error) {
	return client.New(client.TrustConfig{
		RootCAPEM: t.misRootCA,
		SPKIPins:  t.misTrustPins,
	})
}

// fallbackToURIOrStatic implements step 2 and step 3 of the retrieval strategy.
// precedingErr describes why the discovery step did not produce a bundle.
// The configured TrustDomain is returned on every path.
func (t *trustBundleGetterImpl) fallbackToURIOrStatic(
	ctx context.Context,
	c *client.MISClient,
	ietag string,
	precedingErr error,
) (string, []byte, string, error) {
	// ── Step 2: URI Fallback ─────────────────────────────────────────────────
	if t.trustBundleURI == "" {
		return t.staticFallback(precedingErr)
	}

	bundleBytes, etag, err := t.fetchBundleFromURL(ctx, c, ietag, t.trustBundleURI)
	if err != nil {
		// 304 from the URI fallback: cached bundle is still valid.
		if errors.Is(err, ErrNotModified) {
			return t.trustDomain, nil, "", ErrNotModified
		}
		return t.staticFallback(fmt.Errorf(
			"%w; URI fallback to %q failed: %v",
			precedingErr, t.trustBundleURI, err,
		))
	}

	if err := validators.ValidateSpiffeTrustBundle(bundleBytes); err != nil {
		return t.staticFallback(fmt.Errorf(
			"%w; URI fallback bundle from %q failed validation: %v",
			precedingErr, t.trustBundleURI, err,
		))
	}

	return t.trustDomain, bundleBytes, etag, nil
}

// staticFallback implements step 3: return the operator-supplied static bundle
// together with the configured TrustDomain.
func (t *trustBundleGetterImpl) staticFallback(precedingErr error) (string, []byte, string, error) {
	if len(t.staticBundle) == 0 {
		return "", nil, "", fmt.Errorf(
			"%w; no static trust bundle configured as fallback",
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
	case http.StatusOK:
		// handled below
	case http.StatusNotModified:
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

// validateNewInputs orchestrates all input validation for New, delegating each
// concern to a focused helper. Per-URL trust material checks are left to the
// MIS client, which returns the error at request time.
func validateNewInputs(cfg TrustBundleGetterConfig) error {
	if err := validateModeRequirements(cfg); err != nil {
		return err
	}

	if err := validateTrustAnchors(cfg); err != nil {
		return err
	}

	if cfg.DiscoveryURL != "" {
		if err := validateHTTPSURL(cfg.DiscoveryURL); err != nil {
			return fmt.Errorf("invalid DiscoveryURL: %w", err)
		}
	}

	if cfg.TrustBundleURI != "" {
		if err := validateHTTPSURL(cfg.TrustBundleURI); err != nil {
			return fmt.Errorf("invalid TrustBundleURI: %w", err)
		}
	}
	if len(cfg.StaticBundle) == 0 {
		return errors.New("static trust bundle is required")
	}

	if err := validators.ValidateSpiffeTrustBundle(cfg.StaticBundle); err != nil {
		return fmt.Errorf("invalid static trust bundle: %w", err)
	}

	if !validators.ValidateTrustDomain(cfg.TrustDomain) {
		return errors.New("valid TrustDomain is required when StaticBundle is configured")
	}

	return nil
}

// validateModeRequirements enforces the top-level structural rules:
//
//   - At least one source must be configured: DiscoveryURL, TrustBundleURI,
//     or StaticBundle.
//   - Any network source (DiscoveryURL or TrustBundleURI) requires a trust anchor.
//   - A trust anchor without a network source is a misconfiguration.
func validateModeRequirements(cfg TrustBundleGetterConfig) error {
	hasNetwork := cfg.DiscoveryURL != "" || cfg.TrustBundleURI != ""
	hasTrustAnchor := len(cfg.MISRootCA) > 0 || len(cfg.MISTrustPins) > 0
	hasStaticBundle := len(cfg.StaticBundle) > 0

	if !hasNetwork && !hasStaticBundle {
		return errors.New(
			"either DiscoveryURL, TrustBundleURI, or StaticBundle must be provided",
		)
	}

	if hasNetwork && !hasTrustAnchor {
		return errors.New(
			"DiscoveryURL or TrustBundleURI requires a trust anchor (MISRootCA or MISTrustPins)",
		)
	}

	if hasTrustAnchor && !hasNetwork {
		return errors.New(
			"a trust anchor (MISRootCA or MISTrustPins) requires DiscoveryURL or TrustBundleURI",
		)
	}

	return nil
}

// validateTrustAnchors validates the root CA and SPKI pins when provided.
func validateTrustAnchors(cfg TrustBundleGetterConfig) error {
	if len(cfg.MISRootCA) > 0 {
		if _, err := validators.ValidateRootCACertificate(cfg.MISRootCA); err != nil {
			return fmt.Errorf("invalid MIS root CA: %w", err)
		}
	}

	for _, tp := range cfg.MISTrustPins {
		if err := validators.ValidateTrustPin(tp); err != nil {
			return fmt.Errorf("invalid MIS trust pin: %w", err)
		}
	}

	return nil
}

// validateHTTPSURL ensures the value is a non-empty, absolute HTTPS URL with a host.
func validateHTTPSURL(rawURL string) error {
	if rawURL == "" {
		return errors.New("must not be empty")
	}
	parsed, err := url.ParseRequestURI(rawURL)
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
