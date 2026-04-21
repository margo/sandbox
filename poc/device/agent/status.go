package main

import (
	"context"
	"fmt"
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

func NewStatusReporter(
	db database.DatabaseIfc,
	client wfm.SBIAPIClientInterface,
	deviceID string,
	log *zap.SugaredLogger,
) *StatusReporter {
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

func (sr *StatusReporter) onDeploymentChange(
	appID string,
	record *database.DeploymentRecord,
	changeType database.DeploymentRecordChangeType,
) {
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

	// Report status when phase changes or current state is updated
	if changeType == database.DeploymentChangeTypeDesiredStateAdded ||
		changeType == database.DeploymentChangeTypeComponentPhaseChanged ||
		changeType == database.DeploymentChangeTypeCurrentStateAdded {
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
		sr.log.Debugw(
			"Skipping status report - no current state yet",
			"appId",
			appID,
			"phase",
			record.Phase,
		)
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

	// Derive overall deployment state from component states per the Margo spec.
	// Precedence: failed > removing > installing > pending > removed > installed
	deploymentState := deriveOverallState(record.ComponentViseStatus)

	// If no components have been tracked yet, fall back to the internal phase
	if len(record.ComponentViseStatus) == 0 {
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
			sr.log.Warnw(
				"Unknown deployment phase, defaulting to PENDING",
				"appId",
				appID,
				"phase",
				record.Phase,
			)
			deploymentState = sbi.DeploymentStatusManifestStatusStatePending
		}
	}

	// Propagate error information when the deployment has failed
	var deploymentErr error
	if deploymentState == sbi.DeploymentStatusManifestStatusStateFailed && record.Message != "" {
		deploymentErr = fmt.Errorf("%s", record.Message)
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
		deploymentErr,
	)

	if err != nil {
		sr.log.Errorw("Failed to report status", "appId", appID, "error", err)
		return
	}

	sr.log.Infow(
		"Status reported successfully",
		"appId",
		appID,
		"phase",
		record.Phase,
		"state",
		deploymentState,
	)
}

// deriveOverallState computes the overall deployment state from component states
// using the Margo spec precedence: failed > removing > installing > pending > removed > installed.
func deriveOverallState(
	components map[string]sbi.ComponentStatus,
) sbi.DeploymentStatusManifestStatusState {
	if len(components) == 0 {
		return sbi.DeploymentStatusManifestStatusStatePending
	}

	// Precedence order (highest severity first)
	precedence := map[sbi.ComponentStatusState]int{
		sbi.ComponentStatusStateFailed:     6,
		sbi.ComponentStatusStateRemoving:   5,
		sbi.ComponentStatusStateInstalling: 4,
		sbi.ComponentStatusStatePending:    3,
		sbi.ComponentStatusStateRemoved:    2,
		sbi.ComponentStatusStateInstalled:  1,
	}

	// Map component states to deployment-level states
	toDeploymentState := map[sbi.ComponentStatusState]sbi.DeploymentStatusManifestStatusState{
		sbi.ComponentStatusStateFailed:     sbi.DeploymentStatusManifestStatusStateFailed,
		sbi.ComponentStatusStateRemoving:   sbi.DeploymentStatusManifestStatusStateRemoving,
		sbi.ComponentStatusStateInstalling: sbi.DeploymentStatusManifestStatusStateInstalling,
		sbi.ComponentStatusStatePending:    sbi.DeploymentStatusManifestStatusStatePending,
		sbi.ComponentStatusStateRemoved:    sbi.DeploymentStatusManifestStatusStateRemoved,
		sbi.ComponentStatusStateInstalled:  sbi.DeploymentStatusManifestStatusStateInstalled,
	}

	var mostSevere sbi.ComponentStatusState
	highestPrecedence := 0

	for _, cs := range components {
		p := precedence[cs.State]
		if p > highestPrecedence {
			highestPrecedence = p
			mostSevere = cs.State
		}
	}

	if ds, ok := toDeploymentState[mostSevere]; ok {
		return ds
	}
	return sbi.DeploymentStatusManifestStatusStatePending
}
