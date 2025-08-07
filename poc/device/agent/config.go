package main

import (
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

// Config struct
type Config struct {
	DeviceID         string      `mapstructure:"deviceId" validate:"required"`
	WfmSbiUrl        string      `mapstructure:"wfmSbiUrl" validate:"required,url"`
	CapabilitiesFile string      `mapstructure:"capabilitiesFile" validate:"required"`
	RuntimeInfo      RuntimeInfo `mapstructure:"runtimeInfo"`
	Auth             AuthConfig  `mapstructure:"auth"`
}

type RuntimeInfo struct {
	Type string                 `mapstructure:"type"`
	Info map[string]interface{} `mapstructure:"info"`
}

type AuthConfig struct {
	ClientId     string `mapstructure:"clientId"`
	ClientSecret string `mapstructure:"clientSecret"`
	TokenUrl     string `mapstructure:"tokenUrl"`
}

// ConfigManager interface
type ConfigManager interface {
	LoadAndValidateConfig() (*Config, error)
}

// configManager implementation
type configManager struct {
	validator      *validator.Validate
	configFilePath string
}

// NewConfigManager creates a new ConfigManager
func NewConfigManager(completeFilePath string) ConfigManager {
	return &configManager{
		validator:      validator.New(),
		configFilePath: completeFilePath,
	}
}

// LoadConfig loads the configuration
func (cm *configManager) LoadAndValidateConfig() (*Config, error) {
	viper.SetConfigFile(cm.configFilePath)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate Config
	err := cm.validateConfig(&config)
	if err != nil {
		return nil, fmt.Errorf("Failed to validate config: %v", err)
	}

	return &config, nil
}

// validateConfig validates the configuration
func (cm *configManager) validateConfig(config *Config) error {
	err := cm.validator.Struct(config)
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	return nil
}
