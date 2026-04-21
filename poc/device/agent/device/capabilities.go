package device

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"os"

// 	"github.com/margo/sandbox/poc/device/agent/types"
// 	wfm "github.com/margo/sandbox/poc/wfm/cli"
// 	"github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
// 	"go.uber.org/zap"
// )

// type CapabilitiesManager interface {
// 	Start() error
// 	Stop() error

// 	GetCapabilities(ctx context.Context) (sbi.Properties, error)
// 	ReportCapabilities(ctx context.Context) error
// }

// // manualCapabilitiesManager struct
// type manualCapabilitiesManager struct {
// 	config           *types.Config
// 	log              *zap.SugaredLogger
// 	apiClientFactory wfm.SBIAPIClientInterface

// 	// Lifecycle management
// 	started  bool
// 	stopChan chan struct{}
// }

// // NewManualCapabilitiesManager creates a new CapabilitiesManager
// func NewManualCapabilitiesManager(log *zap.SugaredLogger, config *types.Config, apiClientFactory
// wfm.SBIAPIClientInterface) CapabilitiesManager {
// 	return &manualCapabilitiesManager{
// 		config:           config,
// 		log:              log,
// 		apiClientFactory: apiClientFactory,
// 		stopChan:         make(chan struct{}),
// 	}
// }

// // Start initializes the capabilities manager
// func (cm *manualCapabilitiesManager) Start() error {
// 	cm.log.Info("Starting CapabilitiesManager")

// 	// Validate configuration
// 	if cm.config.Capabilities.ReadFromFile == "" {
// 		return fmt.Errorf("capabilities file path is required")
// 	}

// 	// Check if capabilities file exists
// 	if _, err := os.Stat(cm.config.Capabilities.ReadFromFile); os.IsNotExist(err) {
// 		return fmt.Errorf("capabilities file does not exist: %s", cm.config.Capabilities.ReadFromFile)
// 	}

// 	cm.started = true
// 	cm.log.Info("CapabilitiesManager started successfully")
// 	return nil
// }

// // Stop gracefully shuts down the capabilities manager
// func (cm *manualCapabilitiesManager) Stop() error {
// 	cm.log.Info("Stopping CapabilitiesManager")

// 	if !cm.started {
// 		cm.log.Warn("CapabilitiesManager not started, hence no need for stop operation")
// 		return nil
// 	}

// 	close(cm.stopChan)
// 	cm.started = false
// 	cm.log.Info("CapabilitiesManager stopped successfully")
// 	return nil
// }

// // GetCapabilities discovers and returns the device's capabilities
// func (cm *manualCapabilitiesManager) GetCapabilities(ctx context.Context) (sbi.Properties, error)
// {
// 	cm.log.Info("Getting device capabilities...")

// 	// Check if manager is started
// 	if !cm.started {
// 		return sbi.Properties{}, fmt.Errorf("capabilities manager not started")
// 	}

// 	deviceCapabilities, err := cm.discoverCapabilities(ctx)
// 	if err != nil {
// 		return sbi.Properties{}, err
// 	}

// 	return *deviceCapabilities, nil
// }

// // ReportCapabilities discovers and reports the device's capabilities to the WFM
// func (cm *manualCapabilitiesManager) ReportCapabilities(ctx context.Context) error {
// 	cm.log.Info("Reporting device capabilities...")

// 	// Check if manager is started
// 	if !cm.started {
// 		return fmt.Errorf("capabilities manager not started")
// 	}

// 	// 1. Discover Device Capabilities
// 	deviceCapabilities, err := cm.discoverCapabilities(ctx)
// 	if err != nil {
// 		cm.log.Errorw("Failed to discover device capabilities", "error", err)
// 		return fmt.Errorf("failed to discover device capabilities: %w", err)
// 	}

// 	// 2. Create API Client
// 	err = cm.apiClientFactory.ReportCapabilities(ctx, deviceCapabilities.Id, deviceCapabilities)
// 	if err != nil {
// 		cm.log.Errorw("Failed to create API client", "error", err)
// 		return fmt.Errorf("failed to create API client: %w", err)
// 	}

// 	// 3. Send Capabilities to WFM
// 	requestBody := sbi.DeviceCapabilities{
// 		ApiVersion: "device.margo/v1",
// 		Kind:       "DeviceCapabilities",
// 		Properties: *deviceCapabilities,
// 	}

// 	cm.log.Debug("Reporting the following capabilities", "cap", requestBody)

// 	resp, err := client.PostDeviceDeviceIdCapabilities(ctx, deviceCapabilities.Id, requestBody)
// 	if err != nil {
// 		cm.log.Errorw("Failed to send device capabilities", "error", err)
// 		return fmt.Errorf("failed to send device capabilities: %w", err)
// 	}
// 	defer resp.Body.Close()

// 	// 4. Handle Response
// 	if resp.StatusCode != 200 {
// 		cm.log.Errorw("Failed to report device capabilities", "statusCode", resp.StatusCode)
// 		return fmt.Errorf("failed to report device capabilities: status code %d", resp.StatusCode)
// 	}

// 	cm.log.Info("Device capabilities reported successfully")
// 	return nil
// }

// // discoverCapabilities reads the device capabilities from the specified file
// func (cm *manualCapabilitiesManager) discoverCapabilities(ctx context.Context) (*sbi.Properties,
// error) { 	cm.log.Infow("Reading device capabilities from file", "file",
// cm.config.Capabilities.ReadFromFile)

// 	// 1. Read the file
// 	data, err := os.ReadFile(cm.config.Capabilities.ReadFromFile)
// 	if err != nil {
// 		cm.log.Errorw("Failed to read capabilities file", "file", cm.config.Capabilities.ReadFromFile,
// "error", err)
// 		return nil, fmt.Errorf("failed to read capabilities file: %w", err)
// 	}

// 	capabilities := sbi.DeviceCapabilities{}
// 	// 2. Unmarshal the JSON data
// 	err = json.Unmarshal(data, &capabilities)
// 	if err != nil {
// 		cm.log.Errorw("Failed to unmarshal capabilities data", "file",
// cm.config.Capabilities.ReadFromFile, "error", err)
// 		return nil, fmt.Errorf("failed to unmarshal capabilities data: %w", err)
// 	}
// 	var properties sbi.Properties = capabilities.Properties
// 	cm.log.Debugw("Successfully read capabilities from file", "capabilities", properties)
// 	return &properties, nil
// }
