package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/margo/dev-repo/standard/generatedCode/wfm/sbi"
	"go.uber.org/zap"
)

type CapabilitiesManager interface {
	GetCapabilities() (sbi.Properties, error)
	ReportCapabilities() error
}

// manualCapabilitiesManager struct
type manualCapabilitiesManager struct {
	config           *Config
	log              *zap.SugaredLogger
	apiClientFactory APIClientInterface
	// device           *database.DeviceModel // Access to device ID
	ctx context.Context
}

// NewManualCapabilitiesManager creates a new CapabilitiesManager
func NewManualCapabilitiesManager(ctx context.Context, log *zap.SugaredLogger, config *Config, apiClientFactory APIClientInterface) *manualCapabilitiesManager {
	return &manualCapabilitiesManager{
		config:           config,
		log:              log,
		apiClientFactory: apiClientFactory,
		// device:           device,
		ctx: ctx,
	}
}

// GetCapabilities discovers and reports the device's capabilities to the WFM.
func (cm *manualCapabilitiesManager) GetCapabilities() (sbi.Properties, error) {
	cm.log.Info("Reporting device capabilities...")
	deviceCapabilities, err := cm.discoverCapabilities()
	return *deviceCapabilities, err
}

// ReportCapabilities discovers and reports the device's capabilities to the WFM.
func (cm *manualCapabilitiesManager) ReportCapabilities() error {
	cm.log.Info("Reporting device capabilities...")

	// 1. Discover Device Capabilities
	deviceCapabilities, err := cm.discoverCapabilities()
	if err != nil {
		cm.log.Errorw("Failed to discover device capabilities", "error", err)
		return fmt.Errorf("failed to discover device capabilities: %w", err)
	}

	// 2. Create API Client
	client, err := cm.apiClientFactory.NewSBIClient()
	if err != nil {
		cm.log.Errorw("Failed to create API client", "error", err)
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// 3. Send Capabilities to WFM
	// Construct the request body
	requestBody := sbi.DeviceCapabilities{
		ApiVersion: "device.margo/v1", // Replace with your actual API version
		Kind:       "DeviceCapabilities",
		Properties: *deviceCapabilities,
	}

	resp, err := client.PostDeviceDeviceIdCapabilities(cm.ctx, deviceCapabilities.Id, requestBody)
	if err != nil {
		cm.log.Errorw("Failed to send device capabilities", "error", err)
		return fmt.Errorf("failed to send device capabilities: %w", err)
	}
	defer resp.Body.Close()

	// 4. Handle Response
	if resp.StatusCode != 201 {
		cm.log.Errorw("Failed to report device capabilities", "statusCode", resp.StatusCode)
		return fmt.Errorf("failed to report device capabilities: status code %d", resp.StatusCode)
	}

	cm.log.Info("Device capabilities reported successfully")
	return nil
}

// discoverCapabilities reads the device capabilities from the specified file.
func (cm *manualCapabilitiesManager) discoverCapabilities() (*sbi.Properties, error) {
	cm.log.Infow("Reading device capabilities from file", "file", cm.config.CapabilitiesFile)

	// 1. Read the file
	data, err := ioutil.ReadFile(cm.config.CapabilitiesFile)
	if err != nil {
		cm.log.Errorw("Failed to read capabilities file", "file", cm.config.CapabilitiesFile, "error", err)
		return nil, fmt.Errorf("failed to read capabilities file: %w", err)
	}

	// 2. Unmarshal the JSON data
	var properties sbi.Properties
	err = json.Unmarshal(data, &properties)
	if err != nil {
		cm.log.Errorw("Failed to unmarshal capabilities data", "file", cm.config.CapabilitiesFile, "error", err)
		return nil, fmt.Errorf("failed to unmarshal capabilities data: %w", err)
	}

	cm.log.Debugw("Successfully read capabilities from file", "capabilities", properties)
	return &properties, nil
}
