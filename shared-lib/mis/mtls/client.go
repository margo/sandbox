package mtls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"slices"

	"github.com/margo/sandbox/shared-lib/mis/encoders"
	"github.com/margo/sandbox/shared-lib/mis/parser"
	"github.com/margo/sandbox/shared-lib/mis/validators"
)

// VerifierConfig holds the configuration required by the SPIFFE peer verifier.
type VerifierConfig struct {
	// GetOwnTrustDomain is a function that returns the verifier's own SPIFFE trust domain (e.g. "example.org").
	// Only SVIDs belonging to this trust domain are accepted (Rule 1).
	GetOwnTrustDomain func() string

	// GetTrustBundleBytes is a function that returns the SPIFFE trust bundle in JWK Set (JSON) format
	// used to validate the presented certificate chain (Rule 2).
	GetTrustBundleBytes func() []byte

	// GetClientAllowList is a function that returns the list of SPIFFE IDs permitted to connect.
	// The peer's SPIFFE ID must appear in this list for the connection to be accepted.
	// An empty list means no peer is allowed — all connections will be rejected.
	GetClientAllowList func() []string
}

// NewMTLSClientConfig returns a *tls.Config suitable for a client performing
// mutual TLS with SPIFFE X.509-SVID peer authentication.
//
// It disables Go's default hostname/DNS verification (InsecureSkipVerify = true)
// and replaces it with a fully SPIFFE-compliant VerifyPeerCertificates hook that
// enforces all four verifier rules from the spec.
//
// The caller is responsible for supplying the client's own certificate/key pair
// via tls.Certificate so the server can authenticate the client in return.
func NewMTLSClientConfig(clientCert tls.Certificate, cfg VerifierConfig) (*tls.Config, error) {
	if cfg.GetOwnTrustDomain() == "" {
		return nil, fmt.Errorf("OwnTrustDomain must not be empty")
	}
	if len(cfg.GetTrustBundleBytes()) == 0 {
		return nil, fmt.Errorf("TrustBundleBytes must not be empty")
	}
	if cfg.GetClientAllowList == nil {
		return nil, fmt.Errorf("GetClientAllowList must not be nil")
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{clientCert},

		// Disable Go's built-in hostname/DNS verification.
		// SPIFFE identity is carried in the URI SAN, not the DNS SAN or CN.
		// Rule 1 explicitly states DNS hostname matching MUST NOT apply.
		InsecureSkipVerify: true, //nolint:gosec // intentional; replaced by VerifyPeerCertificates

		// VerifyPeerCertificates is called by the TLS stack after the handshake
		// with the raw DER-encoded certificate chain presented by the peer.
		// rawCerts[0] is always the leaf (SVID); rawCerts[1:] are intermediates.
		// verifiedChains is nil because InsecureSkipVerify suppresses Go's own
		// chain building — we build and verify the chain ourselves below.
		VerifyPeerCertificate: buildVerifyPeerCertificates(cfg),
	}

	return tlsCfg, nil
}

// buildVerifyPeerCertificates returns the VerifyPeerCertificates callback that
// enforces the four SPIFFE verifier rules, followed by an allow-list check.
func buildVerifyPeerCertificates(
	cfg VerifierConfig,
) func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			return fmt.Errorf("peer presented no certificates")
		}

		// ----------------------------------------------------------------
		// Parse the leaf certificate (index 0 is always the leaf/SVID).
		// ----------------------------------------------------------------
		leaf, err := x509.ParseCertificate(rawCerts[0])
		if err != nil {
			return fmt.Errorf("failed to parse peer leaf certificate: %w", err)
		}

		// ----------------------------------------------------------------
		// Rule 1 — Trust Domain check.
		//
		// Read the SPIFFE ID from the leaf's URI SAN and verify that its
		// trust domain matches the verifier's own trust domain.
		// DNS hostname matching MUST NOT be used for SVID identity.
		// ----------------------------------------------------------------
		spiffeID, err := parser.ParseSpiffeIdFromX509Svid(rawCerts[0])
		if err != nil {
			return fmt.Errorf("rule 1: failed to extract SPIFFE ID from peer SVID: %w", err)
		}

		peerTrustDomain, err := parser.ParseTrustDomainFromSpiffeID(spiffeID)
		if err != nil {
			return fmt.Errorf("rule 1: %w", err)
		}

		if peerTrustDomain != cfg.GetOwnTrustDomain() {
			return fmt.Errorf(
				"rule 1: trust domain mismatch — peer trust domain %q does not match verifier's own trust domain %q",
				peerTrustDomain,
				cfg.GetOwnTrustDomain(),
			)
		}

		// ----------------------------------------------------------------
		// Rule 2 — Chain validation against the Trust Bundle.
		//
		// Build a certificate pool from the trust bundle and verify that
		// the presented chain (leaf + any intermediates) chains to an
		// anchor in the bundle.  Every anchor in the bundle is equally
		// authoritative (supports rotation overlap).
		//
		// Intermediates supplied by the presenter are loaded into an
		// intermediate pool; no AIA fetching is performed (Rule 3 / spec).
		// ----------------------------------------------------------------
		if err := verifyChainAgainstBundle(
			leaf,
			rawCerts[1:],
			cfg.GetTrustBundleBytes(),
		); err != nil {
			return fmt.Errorf("rule 2: %w", err)
		}

		// ----------------------------------------------------------------
		// Rule 3 — Validity period.
		//
		// ValidateX509SVID already checks NotBefore / NotAfter against
		// time.Now().  We pass the raw DER bytes (leaf only) and the
		// principal as wfm as client will be wfm-client so it can also re-validate the SPIFFE ID format.
		// ----------------------------------------------------------------
		if ok, err := validators.ValidateX509SVID(rawCerts[0], validators.PrincipalWFM); !ok {
			return fmt.Errorf("rule 3/4: SVID validation failed: %w", err)
		}

		// ----------------------------------------------------------------
		// Rule 4 — SVID leaf constraints.
		//
		// (a) basicConstraints cA MUST be false.
		// (b) keyCertSign and cRLSign MUST NOT be set in key usage.
		// (c) SPIFFE ID MUST use the spiffe scheme with a non-root path.
		// (d) Certificate MUST carry exactly one URI SAN.
		//
		// (c) and (d) are already enforced by ValidateX509SVID above via
		// validateSPIFFEID / validateSPIFFEIDFormat.  We enforce (a) and
		// (b) explicitly here for clarity and defence-in-depth.
		// ----------------------------------------------------------------
		if err := validators.ValidateSVIDLeafConstraints(leaf); err != nil {
			return fmt.Errorf("rule 4: %w", err)
		}

		// ----------------------------------------------------------------
		// Allow-list check — SPIFFE ID authorisation.
		//
		// After all cryptographic and structural checks pass, verify that
		// the peer's SPIFFE ID is explicitly present in the configured
		// allow list.  This enforces coarse-grained service-to-service
		// authorisation at the TLS layer.
		//
		// An empty allow list is treated as "deny all": if no SPIFFE IDs
		// are configured, every peer is rejected.  This fail-closed
		// behaviour prevents accidental open access when the allow list
		// has not been populated yet.
		// ----------------------------------------------------------------
		allowList := cfg.GetClientAllowList()
		if len(allowList) == 0 {
			return fmt.Errorf(
				"allow-list check: peer SPIFFE ID %q rejected — allow list is empty (deny all)",
				spiffeID,
			)
		}

		if slices.Contains(allowList, spiffeID) {
			// Peer is explicitly authorised; allow the connection.
			return nil
		}

		return fmt.Errorf(
			"allow-list check: peer SPIFFE ID %q is not in the configured allow list",
			spiffeID,
		)
	}
}

// verifyChainAgainstBundle builds root and intermediate pools and verifies
// that the leaf chains to a trust anchor in the SPIFFE trust bundle.
//
// rawIntermediates contains the DER-encoded intermediate certificates
// supplied by the presenter (rawCerts[1:] from the TLS handshake).
// No out-of-band retrieval (AIA) is performed.
func verifyChainAgainstBundle(
	leaf *x509.Certificate,
	rawIntermediates [][]byte,
	trustBundle []byte,
) error {
	// Validate the leaf against the trust bundle using the existing validator.
	// ValidateX509SVIDAgainstTrustBundle parses the JWK Set, builds a root pool,
	// and calls cert.Verify — it does NOT perform AIA fetching.
	//
	// We pass the leaf in PEM form so the existing helper can re-parse it.
	leafPEM := encoders.CertToPEM(leaf)

	// Load presenter-supplied intermediates into a pool so that
	// x509.Certificate.Verify can use them when building the chain.
	// This satisfies the spec requirement that the presenter supplies
	// all intermediates the chain needs.
	intermediatePool := x509.NewCertPool()
	for i, raw := range rawIntermediates {
		intermediate, err := x509.ParseCertificate(raw)
		if err != nil {
			return fmt.Errorf("failed to parse intermediate certificate [%d]: %w", i, err)
		}
		intermediatePool.AddCert(intermediate)
	}

	// ValidateX509SVIDAgainstTrustBundle internally builds its own root pool
	// from the JWK Set and calls cert.Verify with Roots only.
	// For chains that require intermediates we need to call Verify ourselves
	// so we can supply the Intermediates pool.
	if len(rawIntermediates) == 0 {
		// Simple case: leaf chains directly to a root anchor.
		ok, err := validators.ValidateX509SVIDAgainstTrustBundle(leafPEM, trustBundle)
		if !ok {
			return err
		}
		return nil
	}

	// Complex case: chain includes intermediates.
	// Re-use getTrustBundleFromJWK indirectly by delegating to the validator
	// for root extraction, then run Verify with both pools ourselves.
	return verifyWithIntermediates(leaf, intermediatePool, trustBundle)
}

// verifyWithIntermediates performs x509 chain verification when the presenter
// has supplied intermediate certificates.  It extracts trust anchors from the
// JWK Set trust bundle and runs x509.Certificate.Verify with both the root
// pool and the intermediate pool populated.
func verifyWithIntermediates(
	leaf *x509.Certificate,
	intermediates *x509.CertPool,
	trustBundle []byte,
) error {
	// We need the root pool from the trust bundle.  The cleanest way to obtain
	// it without duplicating getTrustBundleFromJWK is to call
	// ValidateX509SVIDAgainstTrustBundle on one of the intermediates to confirm
	// the bundle is parseable, then build the pool ourselves via a thin wrapper.
	//
	// Since getTrustBundleFromJWK is unexported in validators, we build the
	// root pool by verifying the leaf with CurrentTime set to its NotBefore
	// (matching the approach in validateAgainstTrustBundle) but supplying
	// the Intermediates pool.
	//
	// To avoid duplicating JWK parsing logic, we call the exported validator
	// on the leaf PEM first (which will fail if the leaf doesn't chain even
	// with intermediates), then fall back to a manual Verify call.
	//
	// NOTE: If your project makes getTrustBundleFromJWK exported in the future,
	// replace this block with a direct call to build the root pool.

	// Attempt direct validation first (covers the case where the leaf chains
	// to a root without needing the intermediates pool in the validator).
	leafPEM := encoders.CertToPEM(leaf)
	if ok, _ := validators.ValidateX509SVIDAgainstTrustBundle(leafPEM, trustBundle); ok {
		return nil
	}

	// The leaf requires intermediates.  We must build the root pool ourselves.
	roots, err := validators.GetTrustBundleFromJWK(trustBundle)
	if err != nil {
		return fmt.Errorf("failed to build root pool from trust bundle: %w", err)
	}

	opts := x509.VerifyOptions{
		Roots:         encoders.CertsToPool(roots),
		Intermediates: intermediates,
		CurrentTime:   leaf.NotBefore.Add(1), // matches validateAgainstTrustBundle approach
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}

	if _, err := leaf.Verify(opts); err != nil {
		return fmt.Errorf("certificate does not chain to a trusted SPIFFE bundle: %w", err)
	}
	return nil
}
