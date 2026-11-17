package operations

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/margo/sandbox/mis/pkg/helpers"
	"github.com/margo/sandbox/mis/pkg/types"
)

// SVIDGenerationResult holds the result of an SVID generation operation.
// GenerationType is either "installed" (key + cert generated) or "enrolled" (cert only, via CSR).
// Cert is the PEM-encoded X.509 SVID certificate.
// Key is the PEM-encoded EC private key; populated only when GenerationType is "installed".
type SVIDGenerationResult struct {
	GenerationType string
	Cert           []byte
	Key            []byte
}

const (
	SVIDGenerationTypeInstalled string = "installed"
	SVIDGenerationTypeEnrolled  string = "enrolled"
)

// loadCA reads and parses the CA certificate and EC private key from disk.
// Returns the parsed certificate and private key, or an error.
func (mo *MintOperations) loadCA(caFilePath, caKeyPath string) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	logger := mo.logger.With("operation", "loadCA")

	// --- Load CA certificate ---
	logger.Debug("reading CA certificate from disk", "path", caFilePath)
	caFilePath = filepath.Clean(caFilePath)
	caCertPEMBytes, err := os.ReadFile(caFilePath)
	if err != nil {
		logger.Error("failed to read CA certificate file", "path", caFilePath, "error", err)
		return nil, nil, fmt.Errorf("reading CA cert file: %w", err)
	}

	caCertBlock, _ := pem.Decode(caCertPEMBytes)
	if caCertBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode PEM block from CA certificate file: %s", caFilePath)
	}

	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		logger.Error("failed to parse CA certificate", "error", err)
		return nil, nil, fmt.Errorf("parsing CA certificate: %w", err)
	}
	logger.Debug("CA certificate loaded successfully")

	// --- Load CA private key ---
	logger.Debug("reading CA private key from disk", "path", caKeyPath)
	caKeyPath = filepath.Clean(caKeyPath)
	caKeyPEMBytes, err := os.ReadFile(caKeyPath)
	if err != nil {
		logger.Error("failed to read CA key file", "path", caKeyPath, "error", err)
		return nil, nil, fmt.Errorf("reading CA key file: %w", err)
	}

	caKeyBlock, _ := pem.Decode(caKeyPEMBytes)
	if caKeyBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode PEM block from CA key file: %s", caKeyPath)
	}

	caPrivKey, err := x509.ParseECPrivateKey(caKeyBlock.Bytes)
	if err != nil {
		logger.Error("failed to parse CA EC private key", "error", err)
		return nil, nil, fmt.Errorf("parsing CA private key: %w", err)
	}
	logger.Debug("CA private key loaded successfully")

	return caCert, caPrivKey, nil
}

// buildCertTemplate constructs the x509.Certificate template from the request.
// spiffeURI is the parsed SPIFFE ID URI, always sourced from req.SpiffeID.
func buildCertTemplate(req *types.MintSVIDRequest, spiffeURI *url.URL) *x509.Certificate {
	now := time.Now()
	ttl := req.ResolvedTTL()

	return &x509.Certificate{
		SerialNumber: helpers.NewSerialNumber(),
		Subject: pkix.Name{
			Organization: []string{spiffeURI.Host},
		},
		NotBefore: now,
		NotAfter:  now.Add(ttl),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		BasicConstraintsValid: true,
		URIs:                  []*url.URL{spiffeURI},
		DNSNames:              req.DNS,
	}
}

// GenerateSVID generates an X.509 SVID certificate signed by the provided CA.
//
// Behaviour is driven by req.CSR:
//   - Empty CSR  → "installed" mode: a new ECDSA P-256 key-pair is generated internally.
//     Both the certificate and the private key are returned in the result.
//   - Non-empty CSR → "enrolled" mode: the CSR is validated and its public key is used
//     to sign the certificate. No private key is generated or returned.
//     If the CSR is non-empty but invalid, the call errors out immediately; it does NOT
//     fall back to "installed" mode.
//
// The SPIFFE ID is always taken from req.SpiffeID. When a CSR is provided, its URI SAN
// is extracted purely for informational comparison; a warning is logged if the two differ.
//
// caFilePath: path to the PEM-encoded CA certificate on disk.
// caKeyPath:  path to the PEM-encoded CA private key on disk.
func (mo *MintOperations) GenerateSVID(
	req *types.MintSVIDRequest,
	caFilePath string,
	caKeyPath string,
) (*SVIDGenerationResult, error) {
	logger := mo.logger.With("operation", "GenerateSVID")

	// --- Load CA (shared for both modes) ---
	caCert, caPrivKey, err := mo.loadCA(caFilePath, caKeyPath)
	if err != nil {
		// loadCA already logs the specific error; wrap for caller context.
		return nil, fmt.Errorf("loading CA material: %w", err)
	}

	// --- Parse SPIFFE ID from request (authoritative source, always used) ---
	spiffeURI, err := url.Parse(req.SpiffeID)
	if err != nil {
		logger.Error("failed to parse SPIFFE ID as URI", "spiffe_id", req.SpiffeID, "error", err)
		return nil, fmt.Errorf("parsing SPIFFE ID: %w", err)
	}
	logger.Debug("SPIFFE ID parsed from request", "spiffe_id", req.SpiffeID, "trust_domain", spiffeURI.Host)

	// --- Build certificate template (shared for both modes) ---
	ttl := req.ResolvedTTL()
	logger.Debug(
		"building X.509 certificate template",
		"not_before", time.Now(),
		"not_after", time.Now().Add(ttl),
		"ttl", ttl,
		"dns_sans", req.DNS,
	)
	template := buildCertTemplate(req, spiffeURI)

	// --- Mode selection: CSR present → "enrolled", CSR absent → "installed" ---
	if req.CSR != "" {
		// =========================================================
		// ENROLLED MODE — validate CSR, use its public key to sign.
		// A non-empty but invalid CSR is a hard error; no fallback.
		// =========================================================
		logger.Debug("CSR provided, entering enrolled mode")

		csrBlock, _ := pem.Decode([]byte(req.CSR))
		if csrBlock == nil {
			// CSR field is non-empty but not valid PEM — hard error.
			return nil, fmt.Errorf("enrolled mode: failed to decode PEM block from CSR")
		}

		csr, err := x509.ParseCertificateRequest(csrBlock.Bytes)
		if err != nil {
			logger.Error("failed to parse CSR", "error", err)
			return nil, fmt.Errorf("enrolled mode: parsing CSR: %w", err)
		}

		// Validate CSR signature to ensure the caller holds the private key.
		if err := csr.CheckSignature(); err != nil {
			logger.Error("CSR signature validation failed", "error", err)
			return nil, fmt.Errorf("enrolled mode: CSR signature invalid: %w", err)
		}

		// Informational: CSR should carry exactly one URI SAN (the SPIFFE ID).
		if len(csr.URIs) == 1 {
			// Informational: compare CSR URI SAN with req.SpiffeID; warn if they differ.
			csrSpiffeID := csr.URIs[0].String()
			if csrSpiffeID != req.SpiffeID {
				logger.Warn(
					"CSR URI SAN differs from req.SpiffeID; using req.SpiffeID as authoritative value",
					"req_spiffe_id", req.SpiffeID,
					"csr_spiffe_id", csrSpiffeID,
				)
			} else {
				logger.Debug(
					"CSR URI SAN matches req.SpiffeID",
					"spiffe_id", req.SpiffeID,
					"csr_spiffe_id", csrSpiffeID,
				)
			}
		} else {
			logger.Warn(
				"enrolled mode: CSR should contain exactly one URI SAN",
				"got", len(csr.URIs),
			)
		}

		// --- Sign certificate using CSR's public key ---
		logger.Debug("signing X.509 certificate with CA using CSR public key")
		certDER, err := x509.CreateCertificate(
			rand.Reader,
			template,
			caCert,
			csr.PublicKey, // public key from the caller's CSR
			caPrivKey,
		)
		if err != nil {
			logger.Error("failed to create X.509 certificate", "error", err)
			return nil, fmt.Errorf("enrolled mode: creating X.509 certificate: %w", err)
		}

		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

		logger.Info(
			"X.509 SVID generated successfully (enrolled mode)",
			"spiffe_id", req.SpiffeID,
			"trust_domain", spiffeURI.Host,
			"ttl", ttl,
			"dns_sans", req.DNS,
		)

		return &SVIDGenerationResult{
			GenerationType: SVIDGenerationTypeEnrolled,
			Cert:           certPEM,
			Key:            nil, // private key never generated or held in enrolled mode
		}, nil
	}

	// =========================================================
	// INSTALLED MODE — generate a new ECDSA P-256 key-pair and
	// sign the certificate with it.
	// =========================================================
	logger.Debug("no CSR provided, entering installed mode")

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		logger.Error("failed to generate ECDSA private key", "error", err)
		return nil, fmt.Errorf("installed mode: generating ECDSA private key: %w", err)
	}
	logger.Debug("ECDSA P-256 private key generated successfully")

	// --- Sign certificate using the freshly generated public key ---
	logger.Debug("signing X.509 certificate with CA using generated public key")
	certDER, err := x509.CreateCertificate(
		rand.Reader,
		template,
		caCert,
		&privKey.PublicKey,
		caPrivKey,
	)
	if err != nil {
		logger.Error("failed to create X.509 certificate", "error", err)
		return nil, fmt.Errorf("installed mode: creating X.509 certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	// Marshal the private key to DER, then PEM-encode it.
	logger.Debug("marshalling EC private key to DER")
	privKeyDER, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		logger.Error("failed to marshal EC private key", "error", err)
		return nil, fmt.Errorf("installed mode: marshalling EC private key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privKeyDER})

	logger.Info(
		"X.509 SVID generated successfully (installed mode)",
		"spiffe_id", req.SpiffeID,
		"trust_domain", spiffeURI.Host,
		"ttl", ttl,
		"dns_sans", req.DNS,
	)

	return &SVIDGenerationResult{
		GenerationType: SVIDGenerationTypeInstalled,
		Cert:           certPEM,
		Key:            keyPEM,
	}, nil
}
