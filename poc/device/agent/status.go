package main

import (
	"context"
	"time"

	"github.com/margo/sandbox/poc/device/agent/database"
	wfm "github.com/margo/sandbox/poc/wfm/cli"
	"github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
	"go.uber.org/zap"
)

type StatusReporterIfc interface {
	Start()
	Stop()
}

type StatusReporter struct {
	database  database.DatabaseIfc
	apiClient wfm.SBIAPIClientInterface
	deviceID  string
	log       *zap.SugaredLogger
	stopChan  chan struct{}
}

func NewStatusReporter(db database.DatabaseIfc, client wfm.SBIAPIClientInterface, deviceID string, log *zap.SugaredLogger) *StatusReporter {
	return &StatusReporter{
		database:  db,
		apiClient: client,
		deviceID:  deviceID,
		log:       log,
		stopChan:  make(chan struct{}),
	}
}

func (sr *StatusReporter) Start() {
	// Subscribe to database changes for status updates
	sr.database.Subscribe(sr.onDeploymentChange)
}

func (sr *StatusReporter) Stop() {
	close(sr.stopChan)
}

func (sr *StatusReporter) onDeploymentChange(appID string, record *database.DeploymentRecord, changeType database.DeploymentRecordChangeType) {
	// Concise logging with only important fields
	logFields := []interface{}{
		"appId", appID,
		"changeType", changeType,
		"phase", record.Phase,
	}

	// Add deployment name if available
	if record.DesiredState != nil && record.DesiredState.Metadata.Name != "" {
		logFields = append(logFields, "name", record.DesiredState.Metadata.Name)
	}

	// Add desired state if available
	if record.DesiredState != nil {
		logFields = append(logFields, "desiredState", record.DesiredState.Status.Status.State)
	}

	// Add current state if available
	if record.CurrentState != nil {
		logFields = append(logFields, "currentState", record.CurrentState.Status.Status.State)
	}

	// Add message if present
	if record.Message != "" {
		logFields = append(logFields, "message", record.Message)
	}

	sr.log.Infow("Deployment change detected", logFields...)

	// Report status when phase changes
	if changeType == database.DeploymentChangeTypeDesiredStateAdded ||
		changeType == database.DeploymentChangeTypeComponentPhaseChanged {
		go sr.reportStatus(appID, record)
	}
}

func (sr *StatusReporter) reportStatus(appID string, record *database.DeploymentRecord) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Add nil check for record
	if record == nil {
		sr.log.Warnw("Skipping status report - nil deployment record", "appId", appID)
		return
	}

	// Allow reporting failures even without current state
	// If phase is FAILED but no current state, create one from desired state
	if record.CurrentState == nil {
		if record.Phase == "FAILED" && record.DesiredState != nil {
			sr.log.Infow("Creating current state for failed deployment", "appId", appID)

			// Create failed current state from desired state
			failedState := *record.DesiredState
			failedState.Status.Status.State = sbi.DeploymentStatusManifestStatusStateFailed

			// This will trigger another status report via the subscriber
			sr.database.SetCurrentState(appID, failedState)
			return
		}

		// For non-failed states, skip reporting
		sr.log.Debugw("Skipping status report - no current state yet", "appId", appID, "phase", record.Phase)
		return
	}

	// Convert component status - ensure non-nil slice
	var components []sbi.ComponentStatus
	if len(record.ComponentViseStatus) > 0 {
		components = make([]sbi.ComponentStatus, 0, len(record.ComponentViseStatus))
		for _, status := range record.ComponentViseStatus {
			components = append(components, status)
		}
	} else {
		// Initialize empty slice instead of nil
		components = []sbi.ComponentStatus{}
	}

	// Use the actual sbi constants for deployment state
	var deploymentState sbi.DeploymentStatusManifestStatusState

	// Map the phase to the correct deployment state (case-insensitive)
	switch record.Phase {
	case "PENDING", "pending":
		deploymentState = sbi.DeploymentStatusManifestStatusStatePending
	case "DEPLOYING", "deploying":
		deploymentState = sbi.DeploymentStatusManifestStatusStateInstalling
	case "RUNNING", "running":
		deploymentState = sbi.DeploymentStatusManifestStatusStateInstalled
	case "FAILED", "failed":
		deploymentState = sbi.DeploymentStatusManifestStatusStateFailed
	case "REMOVING", "removing":
		deploymentState = sbi.DeploymentStatusManifestStatusStateRemoving
	case "REMOVED", "removed":
		deploymentState = sbi.DeploymentStatusManifestStatusStateRemoved
	default:
		sr.log.Warnw("Unknown deployment phase, defaulting to PENDING", "appId", appID, "phase", record.Phase)
		deploymentState = sbi.DeploymentStatusManifestStatusStatePending
	}

	// Add defensive logging
	sr.log.Debugw("Reporting status",
		"appId", appID,
		"phase", record.Phase,
		"state", deploymentState,
		"componentCount", len(components),
		"deviceID", sr.deviceID)

	// Report deployment status with error recovery
	defer func() {
		if r := recover(); r != nil {
			sr.log.Errorw("Panic in ReportDeploymentStatus",
				"appId", appID,
				"panic", r,
				"phase", record.Phase,
				"state", deploymentState)
		}
	}()

	err := sr.apiClient.ReportDeploymentStatus(
		ctx,
		sr.deviceID,
		appID,
		deploymentState,
		components,
		nil, // error parameter
	)

	if err != nil {
		sr.log.Errorw("Failed to report status", "appId", appID, "error", err)
		return
	}

	sr.log.Infow("Status reported successfully", "appId", appID, "phase", record.Phase, "state", deploymentState)
}
