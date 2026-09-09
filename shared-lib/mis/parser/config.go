package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/margo/sandbox/shared-lib/mis/validators"
)

// MIAFx509Input holds paths to the device's X.509-SVID certificate and private key.
type MIAFx509Input struct {
	CertPath string
	KeyPath  string
}

// TrustBundleInput holds the trust bundle URI (used as-is) and an optional local path to read from.
type TrustBundleInput struct {
	URI  string
	Path string
}

// MISInput holds the Margo Identity Service connection parameters.
type MISInput struct {
	Endpoint    string
	CAPath      string
	TrustDomain string
	TrustBundle *TrustBundleInput
}

// MIAFInput is a self-contained input struct for ParseMIAFConfig,
// mirroring MIAFConfig without depending on the types package.
type MIAFInput struct {
	X509      MIAFx509Input
	MIS       MISInput
	AuthzPath string
}

// ParsedMIAFx509 holds the loaded X.509 certificate and private key bytes.
type ParsedMIAFx509 struct {
	// CertPEM is the raw PEM-encoded X.509-SVID certificate, or nil if CertPath was empty.
	CertPEM []byte
	// KeyPEM is the raw PEM-encoded private key, or nil if KeyPath was empty.
	KeyPEM []byte
}

// ParsedTrustBundle holds the resolved trust bundle data.
type ParsedTrustBundle struct {
	// URI is passed through unchanged from the input.
	URI string
	// BundleJSON is the raw JWKS trust bundle bytes read from Path, or nil if Path was empty.
	BundleJSON []byte
}

// ParsedMIS holds the resolved Margo Identity Service configuration.
type ParsedMIS struct {
	// Endpoint is passed through unchanged from the input.
	Endpoint string
	// CAPEM is the raw PEM-encoded CA certificate bytes, or nil if CAPath was empty.
	CAPEM []byte
	// TrustDomain is passed through unchanged from the input.
	TrustDomain string
	// TrustBundle holds the resolved trust bundle, or nil if no trust bundle was configured.
	TrustBundle *ParsedTrustBundle
}

// ParsedMIAFConfig is the fully resolved MIAF configuration returned by ParseMIAFConfig.
// All file-backed fields have been read into memory; string-only fields are passed through.
type ParsedMIAFConfig struct {
	X509 ParsedMIAFx509
	MIS  ParsedMIS
	// AuthorizedSPIFFEIDs is the list of SPIFFE IDs loaded from AuthzPath.
	// Must contain at least one entry.
	AuthorizedSPIFFEIDs []string
}

// readFile is a small helper that reads and returns the contents of a file,
// returning a descriptive error that includes the field name and path.
func readFile(fieldName, path string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("%s: failed to read file %q: %w", fieldName, path, err)
	}
	return data, nil
}

// ParseMIAFConfig resolves a MIAFInput into a ParsedMIAFConfig by:
//   - Reading file-backed fields (x509 cert/key, CA cert, trust bundle, authz list) into memory.
//   - Passing string-only fields (endpoint, trust domain, trust bundle URI) through unchanged.
//   - Validating that the authorized SPIFFE ID list is non-empty.
//   - Principal should be wfm or wfm-client (for which config is being validated)
//
// Returns an error if any required file cannot be read or if the authz file contains no entries.
func ParseMIAFConfig(input MIAFInput, principal string) (*ParsedMIAFConfig, error) {
	out := &ParsedMIAFConfig{}

	// --- X.509 certificate ---
	if input.X509.CertPath != "" {
		cert, err := readFile("miaf.x509.certPath", input.X509.CertPath)
		if err != nil {
			return nil, err
		}
		if len(bytes.TrimSpace(cert)) == 0 {
			return nil, fmt.Errorf(
				"miaf.x509.certPath: file %q must not be empty",
				input.X509.CertPath,
			)
		}
		if ok, err := validators.ValidateX509SVID(cert, principal); !ok {
			return nil, fmt.Errorf(
				"miaf.x509.certPath: file %q not a valid x509 SVID certificate, err: %s",
				input.X509.CertPath, err.Error(),
			)
		}
		out.X509.CertPEM = cert
	}

	// --- X.509 private key ---
	if input.X509.KeyPath != "" {
		key, err := readFile("miaf.x509.keyPath", input.X509.KeyPath)
		if err != nil {
			return nil, err
		}
		if len(bytes.TrimSpace(key)) == 0 {
			return nil, fmt.Errorf(
				"miaf.x509.keyPath: file %q must not be empty",
				input.X509.KeyPath,
			)
		}

		if err := validators.ValidatePrivateKey(key); err != nil {
			return nil, fmt.Errorf(
				"miaf.x509.keyPath: file %q not a valid x509 SVID certificate key, err: %s",
				input.X509.KeyPath, err.Error(),
			)
		}
		out.X509.KeyPEM = key
	}

	// --- MIS endpoint (pass-through) ---
	out.MIS.Endpoint = input.MIS.Endpoint

	// --- MIS CA certificate ---
	if input.MIS.CAPath != "" {
		ca, err := readFile("miaf.mis.caPath", input.MIS.CAPath)
		if err != nil {
			return nil, err
		}
		if len(bytes.TrimSpace(ca)) == 0 {
			return nil, fmt.Errorf("miaf.mis.caPath: file %q must not be empty", input.MIS.CAPath)
		}
		if _, err = validators.ValidateRootCACertificate(ca); err != nil {
			return nil, fmt.Errorf(
				"miaf.mis.caPath: file %q must be a valid ca certificate, err: %s",
				input.MIS.CAPath, err.Error(),
			)
		}

		out.MIS.CAPEM = ca
	}

	// --- MIS trust domain (pass-through after basic check) ---
	if ok := validators.ValidateTrustDomain(input.MIS.TrustDomain); !ok {
		return nil, fmt.Errorf(
			"miaf.mis.trustDomain: trust domain %q must be valid",
			input.MIS.TrustBundle.Path,
		)
	}

	out.MIS.TrustDomain = input.MIS.TrustDomain

	// --- Trust bundle ---
	if input.MIS.TrustBundle != nil {
		parsed := &ParsedTrustBundle{
			URI: input.MIS.TrustBundle.URI, // pass-through
		}

		if input.MIS.TrustBundle.Path != "" {
			bundle, err := readFile("miaf.mis.trustBundle.path", input.MIS.TrustBundle.Path)
			if err != nil {
				return nil, err
			}
			if len(bytes.TrimSpace(bundle)) == 0 {
				return nil, fmt.Errorf(
					"miaf.mis.trustBundle.path: file %q must not be empty",
					input.MIS.TrustBundle.Path,
				)
			}

			if err := validators.ValidateSpiffeTrustBundle(
				bundle,
				input.MIS.TrustDomain,
			); err != nil {
				return nil, fmt.Errorf(
					"miaf.mis.trustBundle.path: file %q must be a valid SPIFFE trust bundle, err: %s",
					input.MIS.TrustBundle.Path,
					err.Error(),
				)
			}
			parsed.BundleJSON = bundle
		}

		out.MIS.TrustBundle = parsed
	}

	// --- Authorized SPIFFE IDs ---
	if input.AuthzPath == "" {
		return nil, fmt.Errorf("miaf.authzPath must not be empty")
	}

	authzData, err := readFile("miaf.authzPath", input.AuthzPath)
	if err != nil {
		return nil, err
	}

	var spiffeIDs []string
	if err := json.Unmarshal(authzData, &spiffeIDs); err != nil {
		return nil, fmt.Errorf(
			"miaf.authzPath: failed to parse JSON array from %q: %w",
			input.AuthzPath,
			err,
		)
	}

	// Filter out blank entries and validate at least one valid SPIFFE ID exists.
	var validIDs []string
	for _, id := range spiffeIDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			validIDs = append(validIDs, trimmed)
		}
	}
	if len(validIDs) == 0 {
		return nil, fmt.Errorf(
			"miaf.authzPath: %q must contain at least one SPIFFE ID, but the list is empty",
			input.AuthzPath,
		)
	}

	out.AuthorizedSPIFFEIDs = validIDs

	return out, nil
}
