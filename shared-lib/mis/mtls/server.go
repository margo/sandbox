package mtls

import (
	"crypto/tls"
	"fmt"

	"github.com/margo/sandbox/shared-lib/mis/validators"
)

// NewMTLSServerConfig returns a *tls.Config suitable for a server performing
// mutual TLS with SPIFFE X.509-SVID peer authentication.
//
// It enforces client certificate presentation via tls.RequireAnyClientCert and
// delegates full SPIFFE-compliant validation to the VerifyConnection hook, which
// enforces all four verifier rules from the spec plus an allow-list check.
//
// The caller is responsible for supplying the server's own certificate/key pair
// via tls.Certificate so the client can authenticate the server in return.
func NewMTLSServerConfig(serverCert tls.Certificate, cfg VerifierConfig) (*tls.Config, error) {
	if cfg.GetOwnTrustDomain() == "" {
		return nil, fmt.Errorf("OwnTrustDomain must not be empty")
	}
	if len(cfg.GetTrustBundleBytes()) == 0 {
		return nil, fmt.Errorf("TrustBundleBytes must not be empty")
	}
	if cfg.GetClientAllowList == nil {
		return nil, fmt.Errorf("GetClientAllowList must not be nil")
	}

	return &tls.Config{
		MinVersion:       tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256, tls.CurveP384},
		Certificates:     []tls.Certificate{serverCert},

		// RequireAnyClientCert mandates that the client presents a certificate
		// during the TLS handshake. The actual SPIFFE validation (trust domain,
		// chain, validity, leaf constraints, allow-list) is handled by
		// VerifyConnection below, not by Go's built-in chain verifier.
		ClientAuth: tls.RequireAnyClientCert,

		// VerifyConnection is called after the TLS handshake completes,
		// with access to the full ConnectionState including parsed peer certificates.
		VerifyConnection: buildVerifyConnection(cfg, validators.PrincipalWFMClient),
	}, nil
}
