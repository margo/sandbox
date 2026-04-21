// auth/onboarding.go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/margo/sandbox/poc/device/agent/database"
	"github.com/margo/sandbox/poc/device/agent/types"
	wfm "github.com/margo/sandbox/poc/wfm/cli"
	"github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
	"go.uber.org/zap"
)

type DeviceClientSettings struct {
	deviceClientId string
	// a temporary solution to simulate oem based device, later on once the onboarding story is clear
	// we can go ahead and implement something better over here
	deviceRootIdentity                              types.DeviceRootIdentity
	authEnabled                                     bool
	wfmEndpointsForClient                           []string
	oauthClientId, oAuthClientSecret, oauthTokenUrl string
	log                                             *zap.SugaredLogger
	apiClient                                       wfm.SBIAPIClientInterface
	db                                              database.DatabaseIfc
	canDeployHelm, canDeployCompose                 bool
}

type Option = func(auth *DeviceClientSettings)

func WithDeviceClientID(id string) Option {
	return func(auth *DeviceClientSettings) {
		auth.deviceClientId = id
	}
}

func WithEnableAuth(oauthClientId, oauthClientSecret, tokenUrl string) Option {
	return func(auth *DeviceClientSettings) {
		auth.authEnabled = true
		auth.oauthClientId = oauthClientId
		auth.oAuthClientSecret = oauthClientSecret
		auth.oauthTokenUrl = tokenUrl
	}
}

func WithEnableComposeDeployment() Option {
	return func(auth *DeviceClientSettings) {
		auth.canDeployCompose = true
	}
}

func WithEnableHelmDeployment() Option {
	return func(auth *DeviceClientSettings) {
		auth.canDeployHelm = true
	}
}

func WithDeviceRootIdentity(identity types.DeviceRootIdentity) Option {
	return func(settings *DeviceClientSettings) {
		settings.deviceRootIdentity = identity
	}
}

func NewDeviceSettings(client wfm.SBIAPIClientInterface, db database.DatabaseIfc, log *zap.SugaredLogger, opts ...Option) (*DeviceClientSettings, error) {
	existingRecord, err := db.GetDeviceSettings()
	if err != nil {
		return nil, fmt.Errorf("failed to get device settings from database, %s", err.Error())
	}

	canDeployHelm, canDeployCompose := false, false
	deviceClientId, deviceRootIdentity := "", types.DeviceRootIdentity{}
	authEnabled, oauthClientId, oauthClientSecret, oauthTokenUrl := false, "", "", ""
	if existingRecord != nil {
		deviceClientId = existingRecord.DeviceClientId
		deviceRootIdentity = existingRecord.DeviceRootIdentity
		authEnabled = existingRecord.AuthEnabled
		oauthClientId = existingRecord.OAuthClientId
		oauthClientSecret = existingRecord.OAuthClientSecret
		oauthTokenUrl = existingRecord.OAuthTokenEndpointUrl
		canDeployHelm = existingRecord.CanDeployHelm
		canDeployCompose = existingRecord.CanDeployCompose
	}

	settings := &DeviceClientSettings{
		deviceClientId:     deviceClientId,
		deviceRootIdentity: deviceRootIdentity,
		apiClient:          client,
		log:                log,
		db:                 db,
		authEnabled:        authEnabled,
		oauthClientId:      oauthClientId,
		oAuthClientSecret:  oauthClientSecret,
		oauthTokenUrl:      oauthTokenUrl,
		canDeployHelm:      canDeployHelm,
		canDeployCompose:   canDeployCompose,
	}

	for _, opt := range opts {
		opt(settings)
	}

	// NOTE: need to move this out of here, not a good pattern
	newDeviceRecord := database.DeviceSettingsRecord{}
	if existingRecord != nil {
		newDeviceRecord = *existingRecord
	}
	newDeviceRecord.CanDeployCompose = settings.canDeployCompose
	newDeviceRecord.CanDeployHelm = settings.canDeployHelm
	newDeviceRecord.AuthEnabled = settings.authEnabled
	newDeviceRecord.DeviceClientId = settings.deviceClientId
	newDeviceRecord.DeviceRootIdentity = settings.deviceRootIdentity
	newDeviceRecord.OAuthClientId = settings.oauthClientId
	newDeviceRecord.OAuthClientSecret = settings.oAuthClientSecret
	newDeviceRecord.OAuthTokenEndpointUrl = settings.oauthTokenUrl
	// s.State = settings.

	if err := db.SetDeviceSettings(newDeviceRecord); err != nil {
		return nil, err
	}

	return settings, nil
}

func (da *DeviceClientSettings) Onboard(ctx context.Context) (deviceClientId string, err error) {
	devicePubCert, err := da.deviceRootIdentity.PublicCertificatePEM()
	if err != nil {
		return "", err
	}

	da.log.Infow("Starting device onboarding", "hasValidDeviceSignature", len(devicePubCert) != 0)
	clientId, wfmEndpointsForClient, err := da.apiClient.OnboardDeviceClient(ctx, []byte(devicePubCert))
	if err != nil {
		return "", fmt.Errorf("failed to onboard device client: %s", err.Error())
	}

	da.deviceClientId = clientId
	da.wfmEndpointsForClient = wfmEndpointsForClient

	da.oauthClientId = ""
	da.oAuthClientSecret = ""
	da.oauthTokenUrl = ""
	da.log.Infow("Device onboarding successful", "deviceClientId", da.deviceClientId)

	err = da.db.SetDeviceSettings(database.DeviceSettingsRecord{
		DeviceClientId:        da.deviceClientId,
		DeviceRootIdentity:    da.deviceRootIdentity,
		State:                 types.DeviceOnboardStateOnboarded,
		OAuthClientId:         da.oauthClientId,
		OAuthClientSecret:     da.oAuthClientSecret,
		OAuthTokenEndpointUrl: da.oauthTokenUrl,
		AuthEnabled:           da.authEnabled,
		CanDeployHelm:         da.canDeployHelm,
		CanDeployCompose:      da.canDeployCompose,
	})

	return da.deviceClientId, err
}

func (da *DeviceClientSettings) OnboardWithRetries(ctx context.Context, retries uint8) (deviceClientId string, err error) {
	totalRetries := retries
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for retries > 0 {
		retries--

		// Wait for next tick or overall timeout
		<-ticker.C

		deviceClientId, err := da.Onboard(ctx)
		if err != nil {
			da.log.Infow("onboard operation failed", "tryCount", totalRetries-retries, "totalRetriesAllowed", totalRetries, "err", err.Error())
			continue
		}
		return deviceClientId, err
	}

	return "", fmt.Errorf("unable to onboard the device")
}

func (da *DeviceClientSettings) ReportCapabilities(ctx context.Context, capabilities sbi.DeviceCapabilitiesManifest) error {
	da.log.Infow("Starting capabilities reporting", "deviceClientId", da.deviceClientId)
	err := da.apiClient.ReportCapabilities(ctx, da.deviceClientId, capabilities)
	if err != nil {
		da.log.Errorw("Failed to report capabilities", "error", err, "deviceClientId", da.deviceClientId)
		return fmt.Errorf("failed to report capabilities: %w", err)
	}

	da.log.Infow("Capabilities reported successfully", "deviceClientId", da.deviceClientId)
	return nil
}

func (da *DeviceClientSettings) IsOnboarded() (bool, error) {
	_, isOnboarded, err := da.db.IsDeviceOnboarded()
	return isOnboarded, err
}
