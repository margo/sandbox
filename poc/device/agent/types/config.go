package types

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-playground/validator/v10"
	"github.com/margo/dev-repo/standard/generatedCode/wfm/sbi"
	"gopkg.in/yaml.v2"
)

// DeviceRootIdentity represents the device's root identity/attestation used for onboarding.
type DeviceRootIdentity struct {
	IdentityType string            `yaml:"identityType" validate:"required"`
	Attestation  DeviceAttestation `yaml:"attestation" validate:"required"`
}

type DeviceAttestation struct {
	Random *RandomAttestation `yaml:"random,omitempty"`
	PKI    *PKIAttestation    `yaml:"pki,omitempty"`
}

type RandomAttestation struct {
	Value string `yaml:"value" validate:"required"`
}

type PKIAttestation struct {
	PubCertPath string `yaml:"pubCertPath" validate:"required"`
	Issuer      string `yaml:"issuer,omitempty"`
}

// Note: Key references and signer configuration are intentionally not part of
// PKIAttestation to keep the public identity (certificate) decoupled from
// how the device performs request signing. Request-signing key references are
// configured under the RequestSignerConfig below.

type DeviceOnboardState string

const (
	DeviceOnboardStateOnboardInProgress DeviceOnboardState = "IN-PROGRESS"
	DeviceOnboardStateOnboarded         DeviceOnboardState = "ONBOARDED"
	DeviceOnboardStateOnboardFailed     DeviceOnboardState = "FAILED"
)

// Config struct
type Config struct {
	Logging            LoggingConfig               `yaml:"logging" validate:"required"`
	DeviceRootIdentity DeviceRootIdentity          `yaml:"deviceRootIdentity" validate:"required"`
	Wfm                WFMConfig                   `yaml:"wfm" validate:"required"`
	StateSeeking       StateSeekingConfig          `yaml:"stateSeeking" validate:"required"`
	Capabilities       CapabilitiesDiscoveryConfig `yaml:"capabilities" validate:"required"`
	Runtimes           []RuntimeInfo               `yaml:"runtimes" validate:"required"`
}

type StateSeekingConfig struct {
	Interval uint16 `yaml:"interval" validate:"required"`
}

type WFMConfig struct {
	SbiURL        string              `yaml:"sbiUrl" validate:"required"`
	ClientPlugins ClientPluginsConfig `yaml:"clientPlugins,omitempty"`
}

type ClientPluginsConfig struct {
	RequestSigner *RequestSignerConfig `yaml:"requestSigner,omitempty"`
	AuthHelper    *AuthHelperConfig    `yaml:"authHelper,omitempty"`
	TLSHelper     *TLSHelperConfig     `yaml:"tlsHelper,omitempty"`
}

type RequestSignerConfig struct {
	Enabled         bool   `yaml:"enabled"`
	SignatureAlgo   string `yaml:"signatureAlgo" validate:"required"`
	HashAlgo        string `yaml:"hashAlgo" validate:"required"`
	SignatureFormat string `yaml:"signatureFormat" validate:"required"`
	// KeyRef describes where the private key used for request signing is located.
	KeyRef *KeyRef `yaml:"keyRef,omitempty"`
}

type AuthHelperConfig struct {
	Enabled  bool       `yaml:"enabled"`
	AuthType string     `yaml:"authType"`
	JWT      *JWTConfig `yaml:"jwt"`
}

type TLSHelperConfig struct {
	Enabled        bool    `yaml:"enabled"`
	ServerCAKeyRef *KeyRef `yaml:"caKeyRef,omitempty"`
	// you can support the following to enable client side tls as well
	// ClientCertPath string `yaml:"certPath"`
	// ClientKeyPath  string `yaml:"keyPath"`
}

type JWTConfig struct {
	ClientId     string `yaml:"clientId,omitempty"`
	ClientSecret string `yaml:"clientSecret,omitempty"`
	TokenUrl     string `yaml:"tokenUrl,omitempty"`
}

type CapabilitiesDiscoveryConfig struct {
	ReadFromFile string `yaml:"readFromFile" validate:"required"`
}

type LoggingConfig struct {
	Level string `yaml:"level" validate:"required"`
}

type KubernetesConfig struct {
	KubeconfigPath string `yaml:"kubeconfigPath" validate:"required"`
}

type TLSConfig struct {
	CacertPath *string `yaml:"cacertPath" validate:"required"`
	CertPath   *string `yaml:"certPath" validate:"required"`
	KeyPath    *string `yaml:"keyPath" validate:"required"`
}

type DockerConfig struct {
	Url                 string     `yaml:"url" validator:"url"`
	TLS                 *TLSConfig `yaml:"tls"`
	TLSSkipVerification *bool      `yaml:"tlsSkipVerification"`
}

type RuntimeInfo struct {
	Type       string            `yaml:"type" validate:"required"`
	Kubernetes *KubernetesConfig `yaml:"kubernetes,omitempty"`
	Docker     *DockerConfig     `yaml:"docker,omitempty"`
}

func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	// fmt.Println("read config", string(data))

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// fmt.Println("parsed config", pretty.Sprint(config))
	return &config, validateConfig(&config)
}

func LoadCapabilities(capabilitiesPath string) (*sbi.DeviceCapabilities, error) {
	data, err := os.ReadFile(capabilitiesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read capabilities file: %w", err)
	}

	var capabilities sbi.DeviceCapabilities
	if err := json.Unmarshal(data, &capabilities); err != nil {
		return nil, fmt.Errorf("failed to parse capabilities: %w", err)
	}

	return &capabilities, nil
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	v := validator.New()
	if err := v.Struct(config); err != nil {
		// Return the validator error directly so caller can inspect validation failures
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// logging.level must be present
	if config.Logging.Level == "" {
		return fmt.Errorf("logging.level is required in configuration")
	}
	// If request signer plugin is enabled, require a KeyRef for signing (explicitly decoupled from deviceRootIdentity)
	if config.Wfm.ClientPlugins.RequestSigner != nil && config.Wfm.ClientPlugins.RequestSigner.Enabled {
		if config.Wfm.ClientPlugins.RequestSigner.KeyRef == nil {
			return fmt.Errorf("wfm.clientPlugins.requestSigner.keyRef is required when request signer is enabled")
		}
	}

	if config.Wfm.SbiURL == "" {
		return fmt.Errorf("wfm.sbiUrl is required in configuration")
	}

	if len(config.Runtimes) == 0 {
		return fmt.Errorf("there are no runtimes defined in agent configuration")
	}

	if config.Capabilities.ReadFromFile == "" {
		return fmt.Errorf("capabilities.readFromFile is required in configuration")
	}

	// Basic checks for client plugins (no strict validation here; plugin-specific validation should exist in plugin)
	return nil
}

// PublicCertificatePEM returns the public certificate PEM content if available for PKI attestation.
func (d DeviceRootIdentity) PublicCertificatePEM() (string, error) {
	if d.Attestation.PKI != nil && d.Attestation.PKI.PubCertPath != "" {
		certBytes, err := os.ReadFile(d.Attestation.PKI.PubCertPath)
		if err != nil {
			return "", fmt.Errorf("failed to read certificate file %s: %w", d.Attestation.PKI.PubCertPath, err)
		}
		return string(certBytes), nil
	}
	return "", nil
}

// PublicCertificatePath returns the public certificate file path if available for PKI attestation.
func (d DeviceRootIdentity) PublicCertificatePath() string {
	if d.Attestation.PKI != nil {
		return d.Attestation.PKI.PubCertPath
	}
	return ""
}

// HasCertificateReference returns true if a certificate reference is present.
func (d DeviceRootIdentity) HasCertificateReference() bool {
	if d.Attestation.PKI != nil && d.Attestation.PKI.PubCertPath != "" {
		return true
	}
	return false
}

// KeyRef describes where the private key used for signing can be found.
type KeyRef struct {
	Path string `yaml:"path"` // for type=file
}

type PKCS11Config struct {
	Library string `yaml:"library,omitempty"`
	Token   string `yaml:"token,omitempty"`
	Label   string `yaml:"label,omitempty"`
	PinRef  string `yaml:"pinRef,omitempty"` // reference to secret storage; do not store PIN raw
}

type TPMConfig struct {
	KeyHandle string `yaml:"keyHandle,omitempty"`
}

type KMSConfig struct {
	Provider string `yaml:"provider,omitempty"` // e.g., aws|gcp|azure
	KeyID    string `yaml:"keyId,omitempty"`
}
