package crypto

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
)

// ComputeKeyIDFromPrivateKeyPEM derives a deterministic key identifier from the
// provided private key PEM. It extracts the public key, marshals it to PKIX
// DER and returns the SHA-256 hex thumbprint. This is suitable as a stable
// keyid for use in HTTP Message Signatures.
func ComputeKeyIDFromPrivateKeyPEM(privateKeyPEM string) (string, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block")
	}

	var priv interface{}
	var err error
	switch block.Type {
	case "RSA PRIVATE KEY":
		priv, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return "", fmt.Errorf("failed to parse RSA private key: %w", err)
		}
	case "EC PRIVATE KEY":
		priv, err = x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return "", fmt.Errorf("failed to parse EC private key: %w", err)
		}
	default:
		// Try PKCS#8 as a fallback
		priv, err = x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return "", fmt.Errorf(
				"unsupported or invalid private key PEM (type=%s): %w",
				block.Type,
				err,
			)
		}
	}

	var pub interface{}
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		pub = &k.PublicKey
	case *ecdsa.PrivateKey:
		pub = &k.PublicKey
	default:
		return "", fmt.Errorf("unsupported private key type: %T", priv)
	}

	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", fmt.Errorf("failed to marshal public key: %w", err)
	}

	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:]), nil
}
