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
	SVIDWithIntermediatePath string
	KeyPath                  string
}

// TrustBundleInput holds the trust bundle URI (used as-is) and an optional local path to read from.
type TrustBundleInput struct {
	URI        string
	StaticPath string
}

// MISInput holds the Margo Identity Service connection parameters.
type MISInput struct {
	DiscoveryDocumentURL string
	TrustAnchorPath      string
	TrustPins            []string
	TrustBundle          *TrustBundleInput
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
	// CertPEM is the raw PEM-encoded leaf X.509-SVID certificate, or nil if CertPath was empty.
	CertPEM []byte
	// ChainPEM is the raw PEM-encoded intermediate CA certificate(s) that follow the leaf
	// in the input file, or nil if no intermediates were present.
	ChainPEM []byte
	// KeyPEM is the raw PEM-encoded private key, or nil if KeyPath was empty.
	KeyPEM []byte
	// SpiffeID extracted & validated from certificate provided.
	// Will be populated only if SVID certificate field is populated in input
	SpiffeID string
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
	// DiscoveryDocumentURL is passed through unchanged from the input.
	DiscoveryDocumentURL string
	// TrustAnchorPEM is the raw PEM-encoded CA certificate bytes, or nil if CAPath was empty.
	TrustAnchorPEM []byte
	// TrustPins holds SPKI SHA-256 pins (RFC 7469 §2.4) for authenticating the MIS HTTPS server.
	TrustPins []string
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

// ParseMIAFConfig resolves a MIAFInput into a fully populated ParsedMIAFConfig.
//
// # Overview
//
// It reads all file-backed fields into memory, passes string-only fields through
// unchanged, validates cryptographic material, and enforces principal-specific rules.
// The function is safe to call with partially populated inputs. Optional fields are
// skipped when their path is empty.
//
// # Principal
//
// The principal argument identifies the MIAF component being configured.
// Accepted values are defined in the validators package (e.g. validators.PrincipalWFM,
// validators.PrincipalWFMClient). It is used in two ways:
//   - Passed to validators.ValidateX509SVID to verify the leaf certificate's SPIFFE URI
//     encodes the correct principal path segment.
//   - Gates the authz block: authorized SPIFFE ID loading and validation only
//     runs when principal == validators.PrincipalWFM.
//
// # Input → Output Field Mapping
//
// The table below describes, for each input field, what output field is populated
// when the input is non-empty and no error is encountered.
//
//	Input field                          Output field(s)                          Notes
//	─────────────────────────────────────────────────────────────────────────────────────────────
//	input.X509.SVIDWithIntermediatePath  → out.X509.CertPEM                       Leaf X.509-SVID PEM, the first
//	  (non-empty)                                                                 certificate in the file.
//	                                     → out.X509.ChainPEM                      Intermediate CA PEM(s) that
//	                                                                              follow the leaf. Nil if none.
//	                                     → out.X509.SpiffeID                      SPIFFE ID validated from the leaf.
//	                                     → out.MIS.TrustDomain                    Trust domain parsed from the
//	                                                                              SPIFFE ID (e.g.
//	                                                                              "spiffe://example.org/..."
//	                                                                              → "example.org").
//
//	input.X509.KeyPath                   → out.X509.KeyPEM                        Raw PEM private key bytes.
//	  (non-empty)
//
//	input.MIS.DiscoveryDocumentURL       → out.MIS.DiscoveryDocumentURL           Passed through unchanged
//	  (any value)                                                                 (no validation).
//
//	input.MIS.TrustPins                  → out.MIS.TrustPins                      Copied as-is after every pin
//	  (non-empty)                                                                 is validated as a base64-encoded
//	                                                                              SPKI SHA-256 hash (RFC 7469 §2.4).
//	                                                                              Any invalid pin returns an error.
//
//	input.MIS.TrustAnchorPath            → out.MIS.TrustAnchorPEM                 Raw PEM root CA bytes.
//	  (non-empty)
//
//	input.MIS.TrustBundle                → out.MIS.TrustBundle (non-nil struct)   Set whenever input is non-nil.
//	  (non-nil)
//	  └─ .URI            (any value)     out.MIS.TrustBundle.URI                  Passed through unchanged.
//	  └─ .StaticPath     (non-empty)     out.MIS.TrustBundle.BundleJSON           Raw JWKS bytes, validated as a
//	                                                                              SPIFFE trust bundle. Nil when
//	                                                                              StaticPath is empty.
//
//	input.AuthzPath                      → out.AuthorizedSPIFFEIDs                JSON string array read from the
//	  (non-empty, principal==WFM)                                                 file. Blank/whitespace-only
//	                                                                              entries are removed. At least one
//	                                                                              entry must remain.
//
// # Optional vs Required Fields
//
//   - input.X509.SVIDWithIntermediatePath and input.X509.KeyPath are individually optional.
//     When both are provided, the leaf certificate and key are additionally validated
//     as a matching pair via validators.ValidateCertKeyPair.
//   - input.X509.SVIDWithIntermediatePath is the source of out.X509.SpiffeID and
//     out.MIS.TrustDomain. Both are empty when it is omitted.
//   - input.MIS.TrustAnchorPath is optional; omitting it leaves out.MIS.TrustAnchorPEM nil.
//   - input.MIS.TrustBundle is optional; a nil pointer leaves out.MIS.TrustBundle nil.
//     A non-nil pointer with an empty StaticPath leaves out.MIS.TrustBundle.BundleJSON nil
//     while still populating out.MIS.TrustBundle.URI.
//   - input.MIS.TrustPins is optional; an empty slice leaves out.MIS.TrustPins nil.
//   - input.AuthzPath is required only when principal == validators.PrincipalWFM.
//     For all other principals the field is ignored and out.AuthorizedSPIFFEIDs remains nil.
//
// # Errors
//
// An error is returned (and a nil config) when any of the following occur:
//   - A non-empty path cannot be read from disk.
//   - A file's content is blank/whitespace-only.
//   - The certificate file contains no valid PEM block.
//   - The leaf certificate fails SVID validation or does not encode the expected principal.
//   - The SPIFFE ID in the leaf certificate cannot be parsed into a trust domain.
//   - The intermediate CA chain is invalid.
//   - The private key fails PEM/key validation.
//   - The certificate and private key do not form a valid pair.
//   - Any trust pin is not a valid base64-encoded SHA-256 SPKI hash.
//   - The trust anchor is not a valid root CA certificate.
//   - The static trust bundle file is not valid SPIFFE JWKS JSON.
//   - principal == validators.PrincipalWFM and input.AuthzPath is empty.
//   - The authz file cannot be unmarshalled as a JSON string array.
//   - The authz file contains no non-blank SPIFFE IDs after filtering.
func ParseMIAFConfig(input MIAFInput, principal string) (*ParsedMIAFConfig, error) {
	out := &ParsedMIAFConfig{}

	// --- X.509 certificate with optional intermediate chain ---
	if input.X509.SVIDWithIntermediatePath != "" {
		raw, err := readFile("miaf.x509.certPath", input.X509.SVIDWithIntermediatePath)
		if err != nil {
			return nil, err
		}
		if len(bytes.TrimSpace(raw)) == 0 {
			return nil, fmt.Errorf(
				"miaf.x509.certPath: file %q must not be empty",
				input.X509.SVIDWithIntermediatePath,
			)
		}

		leafPEM, chainPEM := SplitLeafAndChain(raw)
		if leafPEM == nil {
			return nil, fmt.Errorf(
				"miaf.x509.certPath: file %q contains no valid PEM block",
				input.X509.SVIDWithIntermediatePath,
			)
		}

		// Validate the leaf as an X.509 SVID for the given principal.
		spiffeID, err := validators.ValidateX509SVID(leafPEM, principal)
		if err != nil {
			return nil, fmt.Errorf(
				"miaf.x509.certPath: file %q not a valid x509 SVID certificate, err: %s",
				input.X509.SVIDWithIntermediatePath, err.Error(),
			)
		}
		out.X509.CertPEM = leafPEM
		out.X509.SpiffeID = spiffeID

		td, err := ParseTrustDomain(spiffeID)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to extract trust domain from spiffeId, err: %s",
				err.Error(),
			)
		}
		out.MIS.TrustDomain = td

		// Validate and store the intermediate chain if present.
		if len(chainPEM) > 0 {
			if err := validators.ValidateIntermediateCACertificates(chainPEM); err != nil {
				return nil, fmt.Errorf(
					"miaf.x509.certPath: file %q contains an invalid intermediate CA chain, err: %s",
					input.X509.SVIDWithIntermediatePath,
					err.Error(),
				)
			}
			out.X509.ChainPEM = chainPEM
		}
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

	// Verify leaf certificate against the key
	if len(out.X509.CertPEM) != 0 && len(out.X509.KeyPEM) != 0 {
		err := validators.ValidateCertKeyPair(out.X509.CertPEM, out.X509.KeyPEM)
		if err != nil {
			return nil, err
		}
	}

	// --- MIS endpoint (pass-through) ---
	out.MIS.DiscoveryDocumentURL = input.MIS.DiscoveryDocumentURL

	// Validate Trust Pins
	if len(input.MIS.TrustPins) != 0 {
		for _, tp := range input.MIS.TrustPins {
			if err := validators.ValidateTrustPin(tp); err != nil {
				return nil, fmt.Errorf(
					"miaf.mis.trustPins: must be valid SPKI SHA-256 pins (RFC 7469 §2.4)",
				)
			}
		}
		// If validated, then assign to parsed trust pins
		out.MIS.TrustPins = input.MIS.TrustPins
	}

	// --- MIS CA certificate ---
	if input.MIS.TrustAnchorPath != "" {
		ca, err := readFile("miaf.mis.caPath", input.MIS.TrustAnchorPath)
		if err != nil {
			return nil, err
		}
		if len(bytes.TrimSpace(ca)) == 0 {
			return nil, fmt.Errorf(
				"miaf.mis.caPath: file %q must not be empty",
				input.MIS.TrustAnchorPath,
			)
		}
		if _, err = validators.ValidateRootCACertificate(ca); err != nil {
			return nil, fmt.Errorf(
				"miaf.mis.caPath: file %q must be a valid ca certificate, err: %s",
				input.MIS.TrustAnchorPath, err.Error(),
			)
		}

		out.MIS.TrustAnchorPEM = ca
	}

	// --- Trust bundle ---
	if input.MIS.TrustBundle != nil {
		parsed := &ParsedTrustBundle{
			URI: input.MIS.TrustBundle.URI, // pass-through
		}

		if input.MIS.TrustBundle.StaticPath != "" {
			bundle, err := readFile("miaf.mis.trustBundle.path", input.MIS.TrustBundle.StaticPath)
			if err != nil {
				return nil, err
			}
			if len(bytes.TrimSpace(bundle)) == 0 {
				return nil, fmt.Errorf(
					"miaf.mis.trustBundle.path: file %q must not be empty",
					input.MIS.TrustBundle.StaticPath,
				)
			}

			if err := validators.ValidateSpiffeTrustBundle(
				bundle,
			); err != nil {
				return nil, fmt.Errorf(
					"miaf.mis.trustBundle.path: file %q must be a valid SPIFFE trust bundle, err: %s",
					input.MIS.TrustBundle.StaticPath,
					err.Error(),
				)
			}
			parsed.BundleJSON = bundle
		}

		out.MIS.TrustBundle = parsed
	}

	// --- Authorized SPIFFE IDs (In case of Principal WFM) ---
	if principal == validators.PrincipalWFM {

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
	}

	return out, nil
}
