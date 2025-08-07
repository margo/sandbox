package main

import (
	"context"
	"fmt"
	"time"

	"github.com/margo/dev-repo/poc/device/agent/database"
	workloads "github.com/margo/dev-repo/shared-lib/workloads"
	"go.uber.org/zap"
)

// DeviceAgent represents the main device agent.
type DeviceAgent struct {
	log                 *zap.SugaredLogger
	ctx                 context.Context    // Context for managing agent lifecycle
	cancelFunc          context.CancelFunc // Function to cancel the context
	config              *Config
	database            database.AgentDatabase
	apiClientFactory    APIClientInterface
	stateSyncer         AppStateSeeker
	workloadManager     WorkloadManager
	workloadWatcher     WorkloadWatcher
	capabilitiesManager CapabilitiesManager
	onboardingManager   OnboardingManager
}

// NewDeviceAgent creates a new device agent instance.
func NewDeviceAgent(
	config *Config,
	logger *zap.SugaredLogger,
	apiClientFactory APIClientInterface,
) (*DeviceAgent, error) {
	logger.Debug("Creating new DeviceAgent instance")

	if config == nil {
		logger.Error("Configuration is nil, cannot create DeviceAgent")
		return nil, fmt.Errorf("config cannot be nil")
	}

	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	// if database == nil {
	// 	logger.Error("Database is nil, cannot create DeviceAgent")
	// 	return nil, fmt.Errorf("database cannot be nil")
	// }

	if apiClientFactory == nil {
		logger.Error("API client factory is nil, cannot create DeviceAgent")
		return nil, fmt.Errorf("apiClientFactory cannot be nil")
	}

	ctx, cancel := context.WithCancel(context.Background())

	helmClient := &workloads.HelmClient{}
	dockerComposeClient := &workloads.DockerComposeClient{}
	var database database.AgentDatabase = database.NewAgentInMemoryDatabase(ctx, "./data")
	var stateSeeker AppStateSeeker = NewAppStateSeeker(ctx, logger, config, database, apiClientFactory)
	var workloadManager WorkloadManager = NewWorkloadManager(ctx, logger, database, helmClient, dockerComposeClient)
	var capabilityManager CapabilitiesManager = NewManualCapabilitiesManager(ctx, logger, config, apiClientFactory)
	var workloadWatcher WorkloadWatcher = NewWorkloadWatcher(ctx, logger, database, helmClient, dockerComposeClient)
	var onboardingManager OnboardingManager = NewOAuthBasedOnboardingManager(ctx, logger, config, database, apiClientFactory)
	logger.Debug("Created context for DeviceAgent lifecycle management")

	agent := &DeviceAgent{
		config:              config,
		ctx:                 ctx,
		cancelFunc:          cancel,
		apiClientFactory:    apiClientFactory,
		database:            database,
		stateSyncer:         stateSeeker,
		workloadManager:     workloadManager,
		capabilitiesManager: capabilityManager,
		workloadWatcher:     workloadWatcher,
		onboardingManager:   onboardingManager,
		log:                 logger,
	}

	logger.Infow("DeviceAgent instance created successfully",
		"isOAuthBasedOnboardingManagerEnabled", onboardingManager != nil,
		"isStateSeekerEnabled", stateSeeker != nil,
		"isWorkloadManagerEnabled", workloadManager != nil,
		"isWorkloadWatcherEnabled", workloadWatcher != nil,
		"isCapabilitiesManagerEnabled", capabilityManager != nil)

	return agent, nil
}

// Start initializes and starts the device agent.
func (da *DeviceAgent) Start() error {
	da.log.Info("Starting device agent...")

	// Start database first
	da.log.Debug("Starting database...")
	if err := da.database.Start(); err != nil {
		da.log.Errorw("Failed to start database", "error", err)
		return fmt.Errorf("failed to start database: %w", err)
	}
	da.log.Info("Database started successfully")

	// Onboard device with retry logic
	da.log.Info("Starting device onboarding process...")
	onboardingTimeout, _ := context.WithDeadline(da.ctx, time.Now().Add(time.Second*30))
	deviceId, err := da.onboardingManager.KeepTryingOnboardingIfFailed(onboardingTimeout)
	if err != nil {
		da.log.Errorw("failed to onboard the device", "error", err)
		return fmt.Errorf("failed to onboard the device: %w", err)
	}

	// Report Device Capabilities
	da.log.Info("Reporting device capabilities...")
	if err := da.capabilitiesManager.ReportCapabilities(); err != nil {
		da.log.Errorw("Failed to report device capabilities", "error", err)
		// Note: Not returning error here as per original code comment
	} else {
		da.log.Info("Device capabilities reported successfully")
	}

	// Start all components with context checking
	da.log.Info("Starting device agent components...")

	da.log.Debug("Starting state syncer...")
	if err := da.stateSyncer.Start(); err != nil {
		da.log.Errorw("Failed to start state syncer", "error", err)
		return fmt.Errorf("failed to start state syncer: %w", err)
	}
	da.log.Info("State syncer started successfully")

	da.log.Debug("Starting workload manager...")
	if err := da.workloadManager.Start(); err != nil {
		da.log.Errorw("Failed to start workload manager", "error", err)
		return fmt.Errorf("failed to start workload manager: %w", err)
	}
	da.log.Info("Workload manager started successfully")

	da.log.Debug("Starting workload watcher...")
	if err := da.workloadWatcher.Start(); err != nil {
		da.log.Errorw("Failed to start workload watcher", "error", err)
		return fmt.Errorf("failed to start workload watcher: %w", err)
	}
	da.log.Info("Workload watcher started successfully")

	da.log.Infow("Device agent started successfully",
		"deviceId", deviceId,
		"components", []string{"database", "stateSyncer", "workloadManager", "workloadWatcher"})

	return nil
}

// Stop gracefully shuts down the device agent.
func (da *DeviceAgent) Stop() error {
	da.log.Info("Initiating device agent shutdown...")

	var shutdownErrors []error

	// Stop components in reverse order of startup
	da.log.Debug("Stopping workload watcher...")
	if err := da.workloadWatcher.Stop(); err != nil {
		da.log.Errorw("Failed to stop workload watcher", "error", err)
		shutdownErrors = append(shutdownErrors, fmt.Errorf("workload watcher: %w", err))
	} else {
		da.log.Info("Workload watcher stopped successfully")
	}

	da.log.Debug("Stopping workload manager...")
	if err := da.workloadManager.Stop(); err != nil {
		da.log.Errorw("Failed to stop workload manager", "error", err)
		shutdownErrors = append(shutdownErrors, fmt.Errorf("workload manager: %w", err))
	} else {
		da.log.Info("Workload manager stopped successfully")
	}

	da.log.Debug("Stopping state syncer...")
	if err := da.stateSyncer.Stop(); err != nil {
		da.log.Errorw("Failed to stop state syncer", "error", err)
		shutdownErrors = append(shutdownErrors, fmt.Errorf("state syncer: %w", err))
	} else {
		da.log.Info("State syncer stopped successfully")
	}

	da.log.Debug("Stopping database...")
	if err := da.database.Stop(); err != nil {
		da.log.Errorw("Failed to stop database", "error", err)
		shutdownErrors = append(shutdownErrors, fmt.Errorf("database: %w", err))
	} else {
		da.log.Info("Database stopped successfully")
	}

	// Cancel context
	da.log.Debug("Cancelling agent context...")
	da.cancelFunc()

	// Report shutdown status
	if len(shutdownErrors) > 0 {
		da.log.Errorw("Device agent shutdown completed with errors",
			"errorCount", len(shutdownErrors),
			"errors", shutdownErrors)
		return fmt.Errorf("shutdown completed with %d errors: %v", len(shutdownErrors), shutdownErrors)
	}

	da.log.Info("Device agent shutdown completed successfully")
	return nil
}
