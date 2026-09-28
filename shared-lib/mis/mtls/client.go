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
// enforces all four verifier rules from the spec, plus wfm-id cross-validation (Rule 5).
//
// The caller is responsible for supplying the client's own certificate/key pair
// via tls.Certificate so the server can authenticate the client in return.
func NewMTLSClientConfig(clientCert tls.Certificate, cfg VerifierConfig) (*tls.Config, error) {
	tlsCfg := &tls.Config{
		MinVersion:       tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256, tls.CurveP384},
		Certificates:     []tls.Certificate{clientCert},

		// Disable Go's built-in hostname/DNS verification.
		// SPIFFE identity is carried in the URI SAN, not the DNS SAN or CN.
		// Rule 1 explicitly states DNS hostname matching MUST NOT apply.
		InsecureSkipVerify: true, //nolint:gosec // intentional; replaced by VerifyConnection

		// VerifyConnection is called after the TLS handshake completes,
		// with access to the full ConnectionState including parsed peer certificates.
		VerifyConnection: buildVerifyConnection(cfg, clientCert, validators.PrincipalWFM),
	}

	return tlsCfg, nil
}

// buildVerifyConnection returns the VerifyConnection callback that enforces
// the four SPIFFE verifier rules, followed by an allow-list check, and a
// wfm-id cross-validation rule (Rule 5):
//
//   - Rule 1: Peer trust domain must match the verifier's own trust domain.
//   - Rule 2: Peer certificate must chain to the configured SPIFFE trust bundle.
//   - Rule 3: Peer SVID must be within its validity period.
//   - Rule 4: Peer SVID must satisfy X.509-SVID leaf constraints.
//   - Allow-list: Peer SPIFFE ID must appear in the configured allow list.
//   - Rule 5: The <wfm-id> in the peer's SPIFFE ID must match the <wfm-id>
//     in the verifier's own SPIFFE ID, regardless of which side is wfm or wfm-client.
//
// principal is the expected principal of the peer (PrincipalWFM or PrincipalWFMClient).
func buildVerifyConnection(
	cfg VerifierConfig,
	ownCert tls.Certificate,
	peerPrincipal string,
) func(tls.ConnectionState) error {
	return func(cs tls.ConnectionState) error {
		if cfg.GetOwnTrustDomain() == "" {
			return fmt.Errorf("trustdomain is unavailable")
		}
		if len(cfg.GetTrustBundleBytes()) == 0 {
			return fmt.Errorf("trustbundle is unavailable")
		}
		if cfg.GetClientAllowList == nil {
			return fmt.Errorf("connections are not allowed")
		}

		if len(cs.PeerCertificates) == 0 {
			return fmt.Errorf("peer presented no certificates")
		}

		// cs.PeerCertificates[0] is always the leaf/SVID.
		leaf := cs.PeerCertificates[0]

		// ----------------------------------------------------------------
		// Rule 1 — Trust Domain check.
		// ----------------------------------------------------------------
		spiffeID, err := parser.ParseSpiffeIdFromX509Svid(leaf.Raw)
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
		// ----------------------------------------------------------------
		rawIntermediates := make([][]byte, 0, len(cs.PeerCertificates)-1)
		for _, cert := range cs.PeerCertificates[1:] {
			rawIntermediates = append(rawIntermediates, cert.Raw)
		}

		if err := verifyChainAgainstBundle(
			leaf,
			rawIntermediates,
			cfg.GetTrustBundleBytes(),
		); err != nil {
			return fmt.Errorf("rule 2: %w", err)
		}

		// ----------------------------------------------------------------
		// Rule 3 — Validity period.
		// ----------------------------------------------------------------
		if ok, err := validators.ValidateX509SVID(leaf.Raw, peerPrincipal); !ok {
			return fmt.Errorf("rule 3/4: SVID validation failed: %w", err)
		}

		// ----------------------------------------------------------------
		// Rule 4 — SVID leaf constraints.
		// ----------------------------------------------------------------
		if err := validators.ValidateSVIDLeafConstraints(leaf); err != nil {
			return fmt.Errorf("rule 4: %w", err)
		}

		// ----------------------------------------------------------------
		// Allow-list check — SPIFFE ID authorisation.
		// ----------------------------------------------------------------
		allowList := cfg.GetClientAllowList()
		if len(allowList) == 0 {
			return fmt.Errorf(
				"allow-list check: peer SPIFFE ID %q rejected — allow list is empty (deny all)",
				spiffeID,
			)
		}

		if !slices.Contains(allowList, spiffeID) {
			return fmt.Errorf(
				"allow-list check: peer SPIFFE ID %q is not in the configured allow list",
				spiffeID,
			)
		}

		// ----------------------------------------------------------------
		// Rule 5 — wfm-id cross-validation.
		// The <wfm-id> segment must match between peer and self SPIFFE IDs,
		// ensuring a wfm-client only connects to its own wfm, and vice versa.
		// ----------------------------------------------------------------

		ownX509, err := x509.ParseCertificate(ownCert.Certificate[0])
		if err != nil {
			return fmt.Errorf("rule 5: failed to parse own certificate: %w", err)
		}

		ownSpiffeID, err := parser.ParseSpiffeIdFromX509Svid(ownX509.Raw)
		if err != nil {
			return fmt.Errorf("rule 5: failed to extract SPIFFE ID from own SVID: %w", err)
		}

		peerWFMID, err := parser.ParseWFMID(spiffeID)
		if err != nil {
			return fmt.Errorf("rule 5: failed to extract wfm-id from peer SPIFFE ID: %w", err)
		}

		ownWFMID, err := parser.ParseWFMID(ownSpiffeID)
		if err != nil {
			return fmt.Errorf("rule 5: failed to extract wfm-id from own SPIFFE ID: %w", err)
		}

		if peerWFMID != ownWFMID {
			return fmt.Errorf(
				"rule 5: wfm-id mismatch — peer wfm-id %q does not match own wfm-id %q",
				peerWFMID, ownWFMID,
			)
		}

		return nil
	}
}

// verifyChainAgainstBundle and verifyWithIntermediates remain unchanged.

func verifyChainAgainstBundle(
	leaf *x509.Certificate,
	rawIntermediates [][]byte,
	trustBundle []byte,
) error {
	leafPEM := encoders.CertToPEM(leaf)

	intermediatePool := x509.NewCertPool()
	for i, raw := range rawIntermediates {
		intermediate, err := x509.ParseCertificate(raw)
		if err != nil {
			return fmt.Errorf("failed to parse intermediate certificate [%d]: %w", i, err)
		}
		intermediatePool.AddCert(intermediate)
	}

	if len(rawIntermediates) == 0 {
		ok, err := validators.ValidateX509SVIDAgainstTrustBundle(leafPEM, trustBundle)
		if !ok {
			return err
		}
		return nil
	}

	return verifyWithIntermediates(leaf, intermediatePool, trustBundle)
}

func verifyWithIntermediates(
	leaf *x509.Certificate,
	intermediates *x509.CertPool,
	trustBundle []byte,
) error {
	leafPEM := encoders.CertToPEM(leaf)
	if ok, _ := validators.ValidateX509SVIDAgainstTrustBundle(leafPEM, trustBundle); ok {
		return nil
	}

	roots, err := validators.GetTrustBundleFromJWK(trustBundle)
	if err != nil {
		return fmt.Errorf("failed to build root pool from trust bundle: %w", err)
	}

	opts := x509.VerifyOptions{
		Roots:         encoders.CertsToPool(roots),
		Intermediates: intermediates,
		CurrentTime:   leaf.NotBefore.Add(1),
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}

	if _, err := leaf.Verify(opts); err != nil {
		return fmt.Errorf("certificate does not chain to a trusted SPIFFE bundle: %w", err)
	}
	return nil
}
