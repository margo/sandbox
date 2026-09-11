// auth/onboarding.go
package main

import (
	"context"
	"fmt"

	"github.com/margo/sandbox/poc/device/agent/database"
	"github.com/margo/sandbox/poc/device/agent/types"
	wfm "github.com/margo/sandbox/poc/wfm/cli"
	"github.com/margo/sandbox/shared-lib/mis/parser"
	"github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
	"go.uber.org/zap"
)

// onboarding.go
type DeviceClientSettings struct {
	wfmEndpointsForClient    []string
	log                      *zap.SugaredLogger
	apiClient                wfm.SBIAPIClientInterface
	db                       database.DatabaseIfc
	supportedDeploymentTypes []sbi.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypes
	supportedRuntimes        []sbi.DeviceCapabilitiesManifestPropertiesSupportedRuntimes
	miaf                     types.MIAFConfig
	parsedMiaf               *parser.ParsedMIAFConfig
}

type Option = func(auth *DeviceClientSettings)

func WithEnableComposeDeployment() Option {
	return func(auth *DeviceClientSettings) {
		auth.supportedDeploymentTypes = append(
			auth.supportedDeploymentTypes,
			sbi.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypesCompose,
		)
	}
}

func WithMIAFConfig(cfg types.MIAFConfig) Option {
	return func(auth *DeviceClientSettings) {
		auth.miaf = cfg
	}
}

// WithParsedMIAFConfig sets the pre-parsed MIAF configuration on DeviceClientSettings.
func WithParsedMIAFConfig(cfg *parser.ParsedMIAFConfig) Option {
	return func(auth *DeviceClientSettings) {
		auth.parsedMiaf = cfg
	}
}

func WithEnableHelmDeployment() Option {
	return func(auth *DeviceClientSettings) {
		auth.supportedDeploymentTypes = append(
			auth.supportedDeploymentTypes,
			sbi.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypesHelm,
		)

		auth.supportedRuntimes = append(
			auth.supportedRuntimes,
			sbi.DeviceCapabilitiesManifestPropertiesSupportedRuntimesOci,
		)
	}
}

func NewDeviceSettings(
	client wfm.SBIAPIClientInterface,
	db database.DatabaseIfc,
	log *zap.SugaredLogger,
	opts ...Option,
) (*DeviceClientSettings, error) {
	existingRecord, err := db.GetDeviceSettings()
	if err != nil {
		return nil, fmt.Errorf("failed to get device settings from database, %s", err.Error())
	}

	var supportedDeploymentTypes []sbi.DeviceCapabilitiesManifestPropertiesSupportedDeploymentTypes
	var supportedRuntimes []sbi.DeviceCapabilitiesManifestPropertiesSupportedRuntimes

	if existingRecord != nil {
		supportedDeploymentTypes = existingRecord.SupportedDeploymentTypes
		supportedRuntimes = existingRecord.SupportedRuntimes
	}

	settings := &DeviceClientSettings{
		apiClient:                client,
		log:                      log,
		db:                       db,
		supportedDeploymentTypes: supportedDeploymentTypes,
		supportedRuntimes:        supportedRuntimes,
	}

	for _, opt := range opts {
		opt(settings)
	}

	newDeviceRecord := database.DeviceSettingsRecord{}
	if existingRecord != nil {
		newDeviceRecord = *existingRecord
	}
	newDeviceRecord.SupportedDeploymentTypes = settings.supportedDeploymentTypes
	newDeviceRecord.SupportedRuntimes = settings.supportedRuntimes

	if err := db.SetDeviceSettings(newDeviceRecord); err != nil {
		return nil, err
	}

	return settings, nil
}

func (da *DeviceClientSettings) ReportCapabilities(
	ctx context.Context,
	capabilities sbi.DeviceCapabilitiesManifest,
) error {
	da.log.Infow("Starting capabilities reporting")
	err := da.apiClient.ReportCapabilities(ctx, capabilities.Properties.Id, capabilities)
	if err != nil {
		da.log.Errorw(
			"Failed to report capabilities",
			"error",
			err,
		)
		return fmt.Errorf("failed to report capabilities: %w", err)
	}

	da.log.Infow("Capabilities reported successfully", "deviceClientId", capabilities.Properties.Id)
	return nil
}
