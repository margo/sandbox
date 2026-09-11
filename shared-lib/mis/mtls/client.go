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
		MinVersion:       tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256, tls.CurveP384},
		Certificates:     []tls.Certificate{clientCert},

		// Disable Go's built-in hostname/DNS verification.
		// SPIFFE identity is carried in the URI SAN, not the DNS SAN or CN.
		// Rule 1 explicitly states DNS hostname matching MUST NOT apply.
		InsecureSkipVerify: true, //nolint:gosec // intentional; replaced by VerifyConnection

		// VerifyConnection is called after the TLS handshake completes,
		// with access to the full ConnectionState including parsed peer certificates.
		VerifyConnection: buildVerifyConnection(cfg),
	}

	return tlsCfg, nil
}

// buildVerifyConnection returns the VerifyConnection callback that enforces
// the four SPIFFE verifier rules, followed by an allow-list check.
func buildVerifyConnection(cfg VerifierConfig) func(tls.ConnectionState) error {
	return func(cs tls.ConnectionState) error {
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
		if ok, err := validators.ValidateX509SVID(leaf.Raw, validators.PrincipalWFM); !ok {
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

		if slices.Contains(allowList, spiffeID) {
			return nil
		}

		return fmt.Errorf(
			"allow-list check: peer SPIFFE ID %q is not in the configured allow list",
			spiffeID,
		)
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
