package validators

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

func ValidatePrivateKey(keyBytes []byte) error {
	block, _ := pem.Decode(keyBytes)
	if block == nil {
		return errors.New("failed to decode PEM block: invalid or missing PEM data")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("invalid RSA private key: %w", err)
		}
		if err := key.Validate(); err != nil {
			return fmt.Errorf("RSA private key validation failed: %w", err)
		}

	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("invalid ECDSA private key: %w", err)
		}
		if err := validateECDSAKey(key); err != nil {
			return err
		}

	case "PRIVATE KEY": // PKCS#8
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("invalid PKCS#8 private key: %w", err)
		}
		switch k := key.(type) {
		case *rsa.PrivateKey:
			if err := k.Validate(); err != nil {
				return fmt.Errorf("RSA private key validation failed: %w", err)
			}
		case *ecdsa.PrivateKey:
			if err := validateECDSAKey(k); err != nil {
				return err
			}
			// ed25519.PrivateKey is valid if parsed successfully
		}

	default:
		return fmt.Errorf("unsupported PEM block type: %s", block.Type)
	}

	return nil
}

// validateECDSAKey validates an ECDSA key using crypto/ecdh (replaces deprecated IsOnCurve).
func validateECDSAKey(key *ecdsa.PrivateKey) error {
	ecdhCurve, err := curveToECDH(key.Curve)
	if err != nil {
		return err
	}

	// Marshal the public key and attempt to parse it via ecdh — this performs the on-curve check.
	pubKeyBytes := elliptic.Marshal(key.Curve, key.PublicKey.X, key.PublicKey.Y)
	if _, err := ecdhCurve.NewPublicKey(pubKeyBytes); err != nil {
		return fmt.Errorf("ECDSA public key is invalid: %w", err)
	}

	return nil
}

// curveToECDH maps an elliptic.Curve to its crypto/ecdh equivalent.
func curveToECDH(curve elliptic.Curve) (ecdh.Curve, error) {
	switch curve {
	case elliptic.P256():
		return ecdh.P256(), nil
	case elliptic.P384():
		return ecdh.P384(), nil
	case elliptic.P521():
		return ecdh.P521(), nil
	default:
		return nil, fmt.Errorf("unsupported elliptic curve: %s", curve.Params().Name)
	}
}
