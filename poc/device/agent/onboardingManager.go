package main

import (
	"context"
	"fmt"
	"time"

	"github.com/margo/dev-repo/non-standard/pkg/utils"
	"github.com/margo/dev-repo/poc/device/agent/database"
	"github.com/margo/dev-repo/standard/generatedCode/wfm/sbi"
	"go.uber.org/zap"
)

type OnboardingManager interface {
	Onboard() (string, error)
	KeepTryingOnboardingIfFailed(ctx context.Context) (string, error)
	IsOnboarded() error
	GetDeviceId() string
}

type oauthBasedOnboardingManager struct {
	config           *Config
	ctx              context.Context
	operationStopper context.CancelFunc
	database         database.AgentDatabase
	log              *zap.SugaredLogger
	apiClientFactory APIClientInterface
}

func NewOAuthBasedOnboardingManager(ctx context.Context, log *zap.SugaredLogger, config *Config, database database.AgentDatabase, apiClientFactory APIClientInterface) OnboardingManager {
	localCtx, localCanceller := context.WithCancel(ctx)
	return &oauthBasedOnboardingManager{
		config:           config,
		ctx:              localCtx,
		operationStopper: localCanceller,
		database:         database,
		log:              log,
		apiClientFactory: apiClientFactory,
	}
}

func (da *oauthBasedOnboardingManager) KeepTryingOnboardingIfFailed(ctx context.Context) (string, error) {
	deviceId := ""
	retryCount := 0
	maxRetries := 10
	onboardingTicker := time.NewTicker(time.Second * 5)
	defer onboardingTicker.Stop() // Ensure ticker is always stopped

	// Onboarding loop with proper exit conditions
	onboardingComplete := false
	for !onboardingComplete {
		select {
		case <-onboardingTicker.C:
			var err error
			deviceId, err := da.Onboard()
			if err != nil {
				retryCount++
				da.log.Warnw("Device onboarding failed, will retry",
					"error", err.Error(),
					"attempt", retryCount,
					"maxRetries", maxRetries,
					"retryDelay", "5s")

				if retryCount >= maxRetries {
					da.log.Errorw("Device onboarding failed after maximum retries",
						"attempts", retryCount,
						"lastError", err.Error())
					return "", fmt.Errorf("device onboarding failed after %d attempts: %w", maxRetries, err)
				}
				// Continue to next retry
				continue
			}

			// Onboarding successful
			da.log.Infow("Device onboarding completed successfully",
				"deviceId", deviceId,
				"attempts", retryCount+1)
			onboardingComplete = true

		case <-da.ctx.Done():
			da.log.Info("Device agent startup cancelled during onboarding")
			return "", da.ctx.Err()
		}
	}
	return deviceId, nil
}

func (da *oauthBasedOnboardingManager) Onboard() (string, error) {
	deviceId := utils.GenerateDeviceId()
	da.log.Info("Trying device onboarding...")

	// Fetch device info from database
	da.log.Debug("Fetching device information from database...")
	device, err := da.database.GetDevice()
	if err != nil {
		da.log.Errorw("Failed to fetch device details from database", "error", err)
		return "", fmt.Errorf("failed to fetch device details from database: %w", err)
	}

	// Check if device is already onboarded
	if device != nil && device.DeviceAuth != nil {
		da.log.Infow("Device already onboarded, skipping onboarding process",
			"deviceId", device.DeviceProperties.Id)
		return device.DeviceProperties.Id, nil
	}

	da.log.Info("Device not onboarded, proceeding with onboarding...")

	// Create onboarding request
	da.log.Debug("Creating onboarding request...")
	onboardingReq := sbi.PostOnboardingDeviceJSONRequestBody{}

	// Create API client
	da.log.Debug("Creating SBI API client...")
	client, err := da.apiClientFactory.NewSBIClient()
	if err != nil {
		da.log.Errorw("Failed to create SBI API client", "error", err)
		return "", fmt.Errorf("failed to create SBI API client: %w", err)
	}

	// Send onboarding request
	da.log.Info("Sending device onboarding request to orchestrator...")
	resp, err := client.PostOnboardingDevice(da.ctx, onboardingReq)
	if err != nil {
		da.log.Errorw("Failed to send onboarding request", "error", err)
		return "", fmt.Errorf("failed to send onboarding request: %w", err)
	}
	defer resp.Body.Close()

	da.log.Debugw("Received onboarding response", "statusCode", resp.StatusCode)

	// Parse onboarding response
	da.log.Debug("Parsing onboarding response...")
	onboardResp, err := sbi.ParsePostOnboardingDeviceResponse(resp)
	if err != nil {
		da.log.Errorw("Failed to parse onboarding response", "error", err)
		return "", fmt.Errorf("failed to parse onboarding response: %w", err)
	}

	if onboardResp.JSON200 == nil {
		da.log.Errorw("Invalid onboarding response: missing device auth data",
			"statusCode", resp.StatusCode)
		return "", fmt.Errorf("invalid onboarding response: missing device auth data")
	}

	// Store device auth information
	da.log.Debug("Storing device authentication information...")
	if err := da.database.UpsertDeviceAuth(onboardResp.JSON200); err != nil {
		da.log.Errorw("Failed to store device auth information", "error", err)
		return "", fmt.Errorf("failed to store device auth information: %w", err)
	}
	da.log.Info("Device authentication information stored successfully")

	return deviceId, nil
}

func (manager *oauthBasedOnboardingManager) IsOnboarded() error {
	return nil
}

func (manager *oauthBasedOnboardingManager) GetDeviceId() string {
	return ""
}
