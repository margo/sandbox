package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-playground/validator/v10"
	"github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
	"gopkg.in/yaml.v2"
)

// MIAFConfig holds the Margo Identity and Authorization Framework configuration.
// X509 contains the device's X.509-SVID paths issued by the Margo Identity Service (MIS).
type MIAFConfig struct {
	X509      MIAFX509Config `yaml:"x509"      validate:"required"`
	MIS       MISConfig      `yaml:"mis"       validate:"required"`
	AuthzPath string         `yaml:"authzPath" validate:"required"`
}

type MIAFX509Config struct {
	CertPath string `yaml:"certPath" validate:"required"`
	KeyPath  string `yaml:"keyPath"  validate:"required"`
}

type MISConfig struct {
	Endpoint      string `yaml:"endpoint"      validate:"required"`
	CacheInterval uint   `yaml:"cacheInterval"` // in seconds
	CAPath        string `yaml:"caPath"        validate:"required"`
}

type DeviceOnboardState string

const (
	DeviceOnboardStateOnboardInProgress DeviceOnboardState = "IN-PROGRESS"
	DeviceOnboardStateOnboarded         DeviceOnboardState = "ONBOARDED"
	DeviceOnboardStateOnboardFailed     DeviceOnboardState = "FAILED"
)

// Config struct
type Config struct {
	Logging      LoggingConfig               `yaml:"logging"      validate:"required"`
	Database     DatabaseConfig              `yaml:"database"     validate:"required"`
	MIAF         MIAFConfig                  `yaml:"miaf"         validate:"required"`
	Wfm          WFMConfig                   `yaml:"wfm"          validate:"required"`
	StateSeeking StateSeekingConfig          `yaml:"stateSeeking" validate:"required"`
	Capabilities CapabilitiesDiscoveryConfig `yaml:"capabilities" validate:"required"`
	Runtimes     []RuntimeInfo               `yaml:"runtimes"     validate:"required"`
}

type DatabaseConfig struct {
	DataDir string `yaml:"dataDir" validate:"required"`
}

type StateSeekingConfig struct {
	Interval uint16 `yaml:"interval" validate:"required"`
}

type WFMConfig struct {
	SbiURL string `yaml:"sbiUrl" validate:"required"`
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
	CertPath   *string `yaml:"certPath"   validate:"required"`
	KeyPath    *string `yaml:"keyPath"    validate:"required"`
}

type DockerConfig struct {
	Url                 string     `yaml:"url"                 validator:"url"`
	TLS                 *TLSConfig `yaml:"tls"`
	TLSSkipVerification *bool      `yaml:"tlsSkipVerification"`
}

type RuntimeInfo struct {
	Type       string            `yaml:"type"                 validate:"required"`
	Kubernetes *KubernetesConfig `yaml:"kubernetes,omitempty"`
	Docker     *DockerConfig     `yaml:"docker,omitempty"`
}

func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(filepath.Clean(configPath))
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, validateConfig(&config)
}

func LoadCapabilities(capabilitiesPath string) (*sbi.DeviceCapabilitiesManifest, error) {
	data, err := os.ReadFile(filepath.Clean(capabilitiesPath))
	if err != nil {
		return nil, fmt.Errorf("failed to read capabilities file: %w", err)
	}

	var capabilities sbi.DeviceCapabilitiesManifest
	if err := json.Unmarshal(data, &capabilities); err != nil {
		return nil, fmt.Errorf("failed to parse capabilities: %w", err)
	}

	return &capabilities, nil
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	v := validator.New()
	if err := v.Struct(config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	if config.Database.DataDir == "" {
		return errors.New("database.dataDir is required in configuration")
	}

	if config.Logging.Level == "" {
		return fmt.Errorf("logging.level is required in configuration")
	}

	if config.MIAF.X509.CertPath == "" {
		return fmt.Errorf("miaf.x509.certPath is required in configuration")
	}

	if config.MIAF.X509.KeyPath == "" {
		return fmt.Errorf("miaf.x509.keyPath is required in configuration")
	}

	if config.MIAF.MIS.Endpoint == "" {
		return fmt.Errorf("miaf.mis.endpoint is required in configuration")
	}

	if config.MIAF.MIS.CAPath == "" {
		return fmt.Errorf("miaf.mis.caPath is required in configuration")
	}

	if config.MIAF.AuthzPath == "" {
		return fmt.Errorf("miaf.authzPath is required in configuration")
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

	if config.MIAF.MIS.CacheInterval == 0 {
		config.MIAF.MIS.CacheInterval = 60 // setting By default cache interval to 60
	}

	return nil
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
