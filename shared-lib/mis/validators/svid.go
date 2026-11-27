package validators

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// jwkSet represents a SPIFFE trust bundle in JWK Set format
type jwkSet struct {
	Keys []jwkKey `json:"keys"`
}

// jwkKey represents a single JWK entry
type jwkKey struct {
	Use string   `json:"use"` // "x509-svid" for SPIFFE X.509 trust anchors
	X5C []string `json:"x5c"` // Base64-encoded DER certificate chain (NOT base64url)
	Kty string   `json:"kty"`
}

// ValidateX509SVID validates an x509 SVID provided as a PEM or DER encoded []byte.
// Returns the SPIFFE ID string if valid, or an error describing the reason for invalidity.
//
// Note: This function only checks the validity of SVID, does not check the validity of SVID against any trust bundle.
func ValidateX509SVID(svidBytes []byte, principal string) (string, error) {
	// 1. Parse the certificate — try PEM first, then DER
	cert, err := parseCertificate(svidBytes)
	if err != nil {
		return "", fmt.Errorf("invalid x.509 certificate: %w", err)
	}

	// 2. Validate SPIFFE ID: must contain exactly one URI SAN that is a valid SPIFFE ID
	if err := validateSPIFFEID(cert, principal); err != nil {
		return "", err
	}

	// 3. Enforce SPIFFE X.509-SVID leaf constraints
	if err := ValidateSVIDLeafConstraints(cert); err != nil {
		return "", err
	}

	// 4. Validate certificate time validity
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return "", fmt.Errorf("certificate is not yet valid: NotBefore is %s", cert.NotBefore)
	}
	if now.After(cert.NotAfter) {
		return "", fmt.Errorf("certificate has expired: NotAfter was %s", cert.NotAfter)
	}

	// 5. Extract and return the SPIFFE ID from the URI SAN
	// At this point validateSPIFFEID has already guaranteed exactly one valid SPIFFE URI SAN exists
	return cert.URIs[0].String(), nil
}

// ValidateSVIDLeafConstraints enforces SPIFFE X.509-SVID leaf certificate requirements:
//   - basicConstraints cA MUST be false
//   - keyCertSign and cRLSign MUST NOT be set in key usage
//   - the SPIFFE ID MUST have a non-root path (i.e. path must not be empty or "/")
//   - the certificate MUST carry exactly one URI SAN
func ValidateSVIDLeafConstraints(cert *x509.Certificate) error {
	// cA MUST be false
	if cert.IsCA {
		return fmt.Errorf("SVID leaf certificate must not be a CA (basicConstraints cA is true)")
	}

	// keyCertSign and cRLSign MUST NOT be set
	if cert.KeyUsage&x509.KeyUsageCertSign != 0 {
		return fmt.Errorf("SVID leaf certificate must not have keyCertSign key usage set")
	}
	if cert.KeyUsage&x509.KeyUsageCRLSign != 0 {
		return fmt.Errorf("SVID leaf certificate must not have cRLSign key usage set")
	}

	// Exactly one URI SAN (redundant with validateSPIFFEID but enforced explicitly here)
	if len(cert.URIs) != 1 {
		return fmt.Errorf(
			"SVID leaf certificate must carry exactly one URI SAN, found %d",
			len(cert.URIs),
		)
	}

	// SPIFFE ID MUST use the spiffe scheme with a non-root path
	spiffeID := cert.URIs[0]
	if spiffeID.Scheme != "spiffe" {
		return fmt.Errorf("URI SAN scheme must be 'spiffe', got %q", spiffeID.Scheme)
	}
	if spiffeID.Path == "" || spiffeID.Path == "/" {
		return fmt.Errorf(
			"SPIFFE ID %q must have a non-root path (path is %q)",
			spiffeID.String(), spiffeID.Path,
		)
	}

	return nil
}

// ValidateX509SVIDAgainstTrustBundle verifies that the given X.509 SVID certificate
// chains to a trusted CA in the provided SPIFFE trust bundle (in JWK Set format).
//
// Parameters:
//   - svidBytes:        PEM or DER encoded X.509 SVID leaf certificate.
//   - chainPEM:         Optional PEM bundle of intermediate CA certificates ordered
//     from the issuing intermediate up to the topmost intermediate.
//     Pass nil or empty if no intermediates are present.
//   - trustBundleBytes: SPIFFE trust bundle in JWK Set (JSON) format.
//
// Returns:
//   - (true, nil)    if the certificate chains to the trust bundle successfully.
//   - (false, error) describing why validation failed.
func ValidateX509SVIDAgainstTrustBundle(
	svidBytes []byte,
	chainPEM []byte,
	trustBundleBytes []byte,
) (bool, error) {
	cert, err := parseCertificate(svidBytes)
	if err != nil {
		return false, fmt.Errorf("failed to parse SVID certificate: %w", err)
	}

	if err := validateAgainstTrustBundle(cert, trustBundleBytes, chainPEM); err != nil {
		return false, err
	}

	return true, nil
}

// validateAgainstTrustBundle verifies the certificate chains to the SPIFFE trust bundle.
// chainPEM is optional; pass nil when no intermediates are available.
func validateAgainstTrustBundle(cert *x509.Certificate, trustBundle []byte, chainPEM []byte) error {
	if cert.IsCA {
		return fmt.Errorf("certificate is a CA certificate; SVID must be a leaf certificate")
	}

	trustBundleCerts, err := GetTrustBundleFromJWK(trustBundle)
	if err != nil {
		return fmt.Errorf("failed to obtain trust bundle: %w", err)
	}
	if len(trustBundleCerts) == 0 {
		return fmt.Errorf("trust bundle is empty; cannot validate certificate chain")
	}

	roots := x509.NewCertPool()
	for _, ca := range trustBundleCerts {
		roots.AddCert(ca)
	}

	interPool := x509.NewCertPool()
	if len(chainPEM) > 0 {
		intermediates, err := parseCertificateChain(chainPEM)
		if err != nil {
			return fmt.Errorf("failed to parse intermediate chain: %w", err)
		}
		for _, c := range intermediates {
			interPool.AddCert(c)
		}
	}

	opts := x509.VerifyOptions{
		Roots:         roots,
		Intermediates: interPool,
		CurrentTime:   cert.NotBefore.Add(time.Second),
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}

	if _, err := cert.Verify(opts); err != nil {
		return fmt.Errorf("certificate does not chain to a trusted SPIFFE bundle: %w", err)
	}

	return nil
}

// parseCertificate attempts to decode the input as PEM, falling back to raw DER.
func parseCertificate(data []byte) (*x509.Certificate, error) {
	// Try PEM decoding first
	block, _ := pem.Decode(data)
	if block != nil {
		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("PEM block is not a certificate (got type: %s)", block.Type)
		}
		return x509.ParseCertificate(block.Bytes)
	}

	// Fall back to raw DER
	cert, err := x509.ParseCertificate(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse as PEM or DER: %w", err)
	}
	return cert, nil
}

// validateSPIFFEID ensures the certificate has exactly one URI SAN and it is a valid SPIFFE ID.
func validateSPIFFEID(cert *x509.Certificate, principal string) error {
	uris := cert.URIs
	if len(uris) == 0 {
		return fmt.Errorf("certificate contains no URI SANs; a SPIFFE ID is required")
	}
	if len(uris) > 1 {
		return fmt.Errorf(
			"certificate contains %d URI SANs; exactly one SPIFFE ID is required",
			len(uris),
		)
	}

	spiffeID := uris[0]
	if err := validateSPIFFEIDFormat(spiffeID); err != nil {
		return fmt.Errorf("invalid SPIFFE ID %q: %w", spiffeID.String(), err)
	}

	// Validate based on principal as well
	if err := ValidateSpiffeID(spiffeID.String(), principal); err != nil {
		return fmt.Errorf("invalid SPIFFE ID %q: %w", spiffeID.String(), err)
	}
	return nil
}

// validateSPIFFEIDFormat checks that a URI conforms to the SPIFFE ID format:
// spiffe://<trust-domain>/<path>
func validateSPIFFEIDFormat(u *url.URL) error {
	if u.Scheme != "spiffe" {
		return fmt.Errorf("scheme must be 'spiffe', got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("trust domain (host) must not be empty")
	}
	if strings.Contains(u.Host, ":") {
		return fmt.Errorf("trust domain must not contain a port")
	}
	if u.User != nil {
		return fmt.Errorf("SPIFFE ID must not contain user info")
	}
	if u.Fragment != "" {
		return fmt.Errorf("SPIFFE ID must not contain a fragment")
	}
	if u.RawQuery != "" {
		return fmt.Errorf("SPIFFE ID must not contain a query string")
	}
	return nil
}

// GetTrustBundleFromJWK parses a SPIFFE trust bundle in JWK Set format
// and returns the X.509 trust anchor certificates.
// Only keys with use="x509-svid" are considered.
func GetTrustBundleFromJWK(jwkBytes []byte) ([]*x509.Certificate, error) {
	var keySet jwkSet
	if err := json.Unmarshal(jwkBytes, &keySet); err != nil {
		return nil, fmt.Errorf("failed to parse JWK Set: %w", err)
	}

	var certs []*x509.Certificate
	for i, key := range keySet.Keys {
		if key.Use != "x509-svid" {
			continue // skip JWT SVIDs and other key types
		}
		if len(key.X5C) == 0 {
			return nil, fmt.Errorf("key %d has use=x509-svid but missing x5c field", i)
		}

		// Only the first entry in x5c is the trust anchor;
		// remaining entries are intermediates (if present)
		for j, certB64 := range key.X5C {
			derBytes, err := base64.StdEncoding.DecodeString(certB64)
			if err != nil {
				return nil, fmt.Errorf("key %d, x5c[%d]: failed to base64 decode: %w", i, j, err)
			}
			cert, err := x509.ParseCertificate(derBytes)
			if err != nil {
				return nil, fmt.Errorf(
					"key %d, x5c[%d]: failed to parse certificate: %w",
					i,
					j,
					err,
				)
			}
			certs = append(certs, cert)
		}
	}

	if len(certs) == 0 {
		return nil, fmt.Errorf("no x509-svid trust anchors found in JWK Set")
	}

	return certs, nil
}

// ValidateCertKeyPair verifies that the given PEM-encoded certificate and
// private key are cryptographically paired (i.e., the public key in the
// certificate matches the private key).
// Supports RSA, ECDSA, and Ed25519 key types.
func ValidateCertKeyPair(certPEM, keyPEM []byte) error {
	// Decode certificate
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return fmt.Errorf("failed to decode certificate PEM block")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Decode private key — try PKCS8 first, then SEC1 (EC), then PKCS1 (RSA)
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return fmt.Errorf("failed to decode private key PEM block")
	}

	var privateKey crypto.PrivateKey
	switch keyBlock.Type {
	case "PRIVATE KEY": // PKCS#8
		privateKey, err = x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	case "EC PRIVATE KEY": // SEC1
		privateKey, err = x509.ParseECPrivateKey(keyBlock.Bytes)
	case "RSA PRIVATE KEY": // PKCS#1
		privateKey, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	default:
		return fmt.Errorf("unsupported private key type: %q", keyBlock.Type)
	}
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// Extract public key from private key and compare with cert's public key
	type publicKeyer interface {
		Public() crypto.PublicKey
	}
	keyer, ok := privateKey.(publicKeyer)
	if !ok {
		return fmt.Errorf("private key does not expose a public key")
	}

	certPubKey := cert.PublicKey
	derivedPub := keyer.Public()

	switch pub := certPubKey.(type) {
	case *rsa.PublicKey:
		derived, ok := derivedPub.(*rsa.PublicKey)
		if !ok || pub.N.Cmp(derived.N) != 0 || pub.E != derived.E {
			return fmt.Errorf("certificate and private key do not match: RSA public key mismatch")
		}
	case *ecdsa.PublicKey:
		derived, ok := derivedPub.(*ecdsa.PublicKey)
		if !ok || pub.X.Cmp(derived.X) != 0 || pub.Y.Cmp(derived.Y) != 0 {
			return fmt.Errorf("certificate and private key do not match: ECDSA public key mismatch")
		}
	case ed25519.PublicKey:
		derived, ok := derivedPub.(ed25519.PublicKey)
		if !ok || !bytes.Equal(pub, derived) {
			return fmt.Errorf(
				"certificate and private key do not match: Ed25519 public key mismatch",
			)
		}
	default:
		return fmt.Errorf("unsupported public key type in certificate: %T", certPubKey)
	}

	return nil
}

// ValidateX509SVIDChain verifies that the given X.509 SVID leaf certificate
// forms a valid chain with the provided intermediate CA certificates.
//
// This validates the path from the leaf up to the topmost intermediate — it does
// NOT require or check a trust anchor (root CA). Use ValidateX509SVIDAgainstTrustBundle
// to perform full chain validation including the trust anchor.
//
// Parameters:
//   - svidBytes: PEM or DER encoded X.509 SVID leaf certificate.
//   - chainPEM:  PEM bundle containing one or more intermediate CA certificates,
//     ordered from the issuing intermediate (closest to the leaf) up to
//     the topmost intermediate. Must not be empty.
//
// Returns an error if:
//   - The leaf or any intermediate certificate cannot be parsed.
//   - The leaf does not chain to the provided intermediates.
//   - chainPEM is empty or contains no valid PEM blocks.
func ValidateX509SVIDChain(svidBytes []byte, chainPEM []byte) error {
	leaf, err := parseCertificate(svidBytes)
	if err != nil {
		return fmt.Errorf("failed to parse SVID leaf certificate: %w", err)
	}

	if leaf.IsCA {
		return fmt.Errorf("svidBytes must be a leaf certificate, not a CA")
	}

	intermediates, err := parseCertificateChain(chainPEM)
	if err != nil {
		return fmt.Errorf("failed to parse intermediate chain: %w", err)
	}
	if len(intermediates) == 0 {
		return fmt.Errorf("intermediate chain must contain at least one certificate")
	}

	// Build an intermediates pool from all but the topmost cert.
	// The topmost intermediate acts as a temporary root so that
	// x509.Certificate.Verify can walk the path without a real trust anchor.
	topmostIntermediate := intermediates[len(intermediates)-1]

	roots := x509.NewCertPool()
	roots.AddCert(topmostIntermediate)

	interPool := x509.NewCertPool()
	for _, c := range intermediates[:len(intermediates)-1] {
		interPool.AddCert(c)
	}

	opts := x509.VerifyOptions{
		Roots:         roots,
		Intermediates: interPool,
		// Anchor time to the leaf's own validity window to avoid spurious
		// expiry failures during chain-only validation.
		CurrentTime: leaf.NotBefore.Add(time.Second),
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}

	if _, err := leaf.Verify(opts); err != nil {
		return fmt.Errorf("SVID does not chain to the provided intermediates: %w", err)
	}

	return nil
}

// parseCertificateChain decodes all PEM blocks in pemData and parses each as
// an X.509 certificate. Returns an error if any block fails to parse.
// The returned slice preserves the order of the input PEM blocks.
func parseCertificateChain(pemData []byte) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	rest := pemData
	i := 0
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("block %d: expected CERTIFICATE, got %q", i+1, block.Type)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("block %d: failed to parse certificate: %w", i+1, err)
		}
		certs = append(certs, cert)
		i++
	}
	return certs, nil
}
