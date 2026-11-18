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
	wfmEndpointsForClient []string
	log                   *zap.SugaredLogger
	apiClient             wfm.SBIAPIClientInterface
	db                    database.DatabaseIfc
	miaf                  types.MIAFConfig
	parsedMiaf            *parser.ParsedMIAFConfig
}

type Option = func(auth *DeviceClientSettings)

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

	settings := &DeviceClientSettings{
		apiClient: client,
		log:       log,
		db:        db,
	}

	for _, opt := range opts {
		opt(settings)
	}

	newDeviceRecord := database.DeviceSettingsRecord{}
	if existingRecord != nil {
		newDeviceRecord = *existingRecord
	}
	newDeviceRecord.MIAF = settings.miaf
	if settings.parsedMiaf != nil {
		newDeviceRecord.ParsedMIAF = *settings.parsedMiaf
	}

	if err := db.SetDeviceSettings(newDeviceRecord); err != nil {
		return nil, err
	}

	return settings, nil
}

func (da *DeviceClientSettings) ReportDeviceProfile(
	ctx context.Context,
	deviceProfile *types.DeviceProfile,
) error {

	if deviceProfile == nil {
		return fmt.Errorf("device profile is nil, cannot report device profile")
	}

	da.log.Infow("Starting device profile reporting", "deviceClientId", deviceProfile.WorkloadTargetId)
	err := da.apiClient.ReportDeviceProfile(ctx, deviceProfile.WorkloadTargetId, deviceProfile.Profile)
	if err != nil {
		if pd, ok := sbi.AsProblemDetail(err); ok {
			da.log.Errorw(
				"WFM returned problem detail on device profile report",
				"workloadTargetId", deviceProfile.WorkloadTargetId,
				"type", pd.Type,
				"status", pd.Status,
				"detail", pd.Detail,
				"retryable", pd.IsRetryable(),
			)
			return fmt.Errorf("WFM rejected device profile [%d] %s: %w", pd.Status, pd.Title, err)
		}
		return fmt.Errorf("failed to report device profile: %w", err)
	}
	da.log.Infow("Device profile reported successfully", "workloadTargetId", deviceProfile.WorkloadTargetId)
	return nil
}
