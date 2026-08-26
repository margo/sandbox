package conf

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/margo/sandbox/mis/pkg/log"
)

type Config struct {
	TrustDomain    string       `json:"trustDomain"`
	Log            *LogConfig   `json:"log"`
	TrustBundleURI string       `json:"trustBundleURI"`
	CA             *CAConfig    `json:"ca"`
	HTTPS          *HTTPSConfig `json:"https"`
}

type LogConfig struct {
	Level string `json:"level"`
}

type CAConfig struct {
	Cert string `json:"cert"`
	Key  string `json:"key"`
}

type HTTPSConfig struct {
	Addr string `json:"addr"`
	CA   string `json:"ca"`
	Cert string `json:"cert"`
	Key  string `json:"key"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error in reading config file, err : %s", err.Error())
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("error in unmarshalling config, err : %s", err.Error())
	}

	err = validateConfig(&cfg)
	if err != nil {
		return nil, fmt.Errorf("invalid config: %s", err.Error())
	}

	return &cfg, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func validateConfig(cfg *Config) error {
	var errs []error

	// 1. trustDomain cannot be empty
	if cfg.TrustDomain == "" {
		errs = append(errs, errors.New("trustDomain cannot be empty"))
	}

	// 2. trustBundleURI default
	if cfg.TrustBundleURI == "" {
		cfg.TrustBundleURI = ".well-known/spiffe/bundle.json"
	}

	// 3. log.level default and validation
	if cfg.Log == nil {
		cfg.Log = &LogConfig{Level: "info"}
	} else if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	} else if _, ok := log.ValidLogLevels[cfg.Log.Level]; !ok {
		errs = append(
			errs,
			fmt.Errorf(
				"log.level %q is invalid: accepted values are error, warn, info, debug",
				cfg.Log.Level,
			),
		)
	}

	// 4 & 5. ca.cert and ca.key
	if cfg.CA == nil {
		errs = append(errs, errors.New("ca configuration is required"))
	} else {
		if cfg.CA.Cert == "" {
			errs = append(errs, errors.New("ca.cert cannot be empty"))
		} else if !fileExists(cfg.CA.Cert) {
			errs = append(errs, fmt.Errorf("ca.cert file does not exist: %s", cfg.CA.Cert))
		}

		if cfg.CA.Key == "" {
			errs = append(errs, errors.New("ca.key cannot be empty"))
		} else if !fileExists(cfg.CA.Key) {
			errs = append(errs, fmt.Errorf("ca.key file does not exist: %s", cfg.CA.Key))
		}
	}

	// 6. https.addr default
	if cfg.HTTPS == nil {
		cfg.HTTPS = &HTTPSConfig{Addr: ":8443"}
	} else {
		if cfg.HTTPS.Addr == "" {
			cfg.HTTPS.Addr = ":8443"
		}

		// 7. https.ca, https.cert, https.key
		if cfg.HTTPS.CA == "" {
			errs = append(errs, errors.New("https.ca cannot be empty"))
		} else if !fileExists(cfg.HTTPS.CA) {
			errs = append(errs, fmt.Errorf("https.ca file does not exist: %s", cfg.HTTPS.CA))
		}

		if cfg.HTTPS.Cert == "" {
			errs = append(errs, errors.New("https.cert cannot be empty"))
		} else if !fileExists(cfg.HTTPS.Cert) {
			errs = append(errs, fmt.Errorf("https.cert file does not exist: %s", cfg.HTTPS.Cert))
		}

		if cfg.HTTPS.Key == "" {
			errs = append(errs, errors.New("https.key cannot be empty"))
		} else if !fileExists(cfg.HTTPS.Key) {
			errs = append(errs, fmt.Errorf("https.key file does not exist: %s", cfg.HTTPS.Key))
		}
	}

	return errors.Join(errs...)
}
