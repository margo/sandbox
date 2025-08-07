package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kr/pretty"
	"github.com/margo/dev-repo/poc/device/agent/database"
	"github.com/margo/dev-repo/standard/generatedCode/wfm/sbi"
	"github.com/margo/dev-repo/standard/pkg"
	"go.uber.org/zap"
)

const (
	appStateSeekingInterval = time.Second * 10
)

// AppStateSeeker interface
type AppStateSeeker interface {
	Start() error
	Stop() error
	ExplicityTriggerSync() error
}

// appStateSeeker struct
type appStateSeeker struct {
	config           *Config
	ctx              context.Context
	operationStopper context.CancelFunc
	database         database.AgentDatabase
	log              *zap.SugaredLogger
	apiClientFactory APIClientInterface
}

// NewAppStateSeeker creates a new StateSyncer
func NewAppStateSeeker(ctx context.Context, log *zap.SugaredLogger, config *Config, database database.AgentDatabase, apiClientFactory APIClientInterface) AppStateSeeker {
	localCtx, localCanceller := context.WithCancel(ctx)
	return &appStateSeeker{
		config:           config,
		ctx:              localCtx,
		operationStopper: localCanceller,
		database:         database,
		log:              log,
		apiClientFactory: apiClientFactory,
	}
}

func (ss *appStateSeeker) Start() error {
	go ss.syncAppStates(ss.ctx)
	return nil
}

func (ss *appStateSeeker) Stop() error {
	ss.operationStopper()
	return nil
}

func (ss *appStateSeeker) ExplicityTriggerSync() error {
	return ss.syncAppStates(ss.ctx)
}

// syncAppStates runs a continuous loop that periodically syncs application states
// with the orchestration service. It sends current states and receives desired states.
func (ss *appStateSeeker) syncAppStates(ctx context.Context) error {
	ss.log.Info("Starting app state sync loop")

	// Create ticker for consistent intervals
	ticker := time.NewTicker(appStateSeekingInterval)
	defer ticker.Stop() // Important: always stop ticker to prevent goroutine leak

	// Perform initial sync before starting the loop
	// This ensures we sync immediately on startup rather than waiting for first tick
	if err := ss.performStateSync(ctx); err != nil {
		ss.log.Errorw("Initial state sync failed", "error", err)
		// Don't return error here - continue with periodic sync
	}

	for {
		select {
		case <-ticker.C:
			// Perform periodic state synchronization
			if err := ss.performStateSync(ctx); err != nil {
				ss.log.Errorw("Failed to sync app states", "error", err)
				// Continue loop even on error - don't break the sync cycle
				// Individual sync failures shouldn't stop the entire sync process
			}

		case <-ctx.Done():
			// Graceful shutdown requested
			ss.log.Info("App state sync loop shutting down")
			return nil
		}
	}
}

// performStateSync handles the actual state synchronization logic.
func (ss *appStateSeeker) performStateSync(ctx context.Context) error {
	ss.log.Debug("Syncing app states...")

	// Fetch current workloads from local database
	currentWorkloads, err := ss.database.GetAllWorkloads()
	if err != nil {
		return fmt.Errorf("failed to fetch workloads from database: %w", err)
	}

	// Convert workloads map to slice for API request
	currentAppStates := make(sbi.StateJSONRequestBody, 0, len(currentWorkloads))
	for _, appState := range currentWorkloads {
		currentAppStates = append(currentAppStates, appState)
	}

	ss.log.Debugw("Sending current states to orchestrator", "count", len(currentAppStates))

	// Create API client for communication with orchestration service
	client, err := ss.apiClientFactory.NewSBIClient()
	if err != nil {
		ss.log.Errorw("Failed to create API client", "error", err)
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Send current states and receive desired states
	// Using the provided context to allow cancellation
	resp, err := client.State(ctx, currentAppStates)
	if err != nil {
		ss.log.Errorw("Failed to send state sync request", "error", err)
		return fmt.Errorf("failed to send state sync request: %w", err)
	}
	defer resp.Body.Close() // Always close response body to prevent resource leaks

	// Parse the response from orchestration service
	desiredStateResp, err := sbi.ParseStateResponse(resp)
	if err != nil {
		ss.log.Errorw("Failed to parse app state response", "error", err)
		return fmt.Errorf("failed to parse app state response: %w", err)
	}

	// Handle different response status codes
	switch desiredStateResp.StatusCode() {
	case http.StatusOK, http.StatusAccepted:
		ss.log.Debugw("Received desired state API response",
			"statusCode", desiredStateResp.StatusCode(),
			"hasData", desiredStateResp.JSON200 != nil)

		// Process desired states if provided
		if desiredStateResp.JSON200 != nil {
			ss.log.Debugw("Processing desired states", "response", pretty.Sprint(desiredStateResp.JSON200))

			if err := ss.mergeAppStates(*desiredStateResp.JSON200); err != nil {
				ss.log.Errorw("Failed to merge app states", "error", err)
				return fmt.Errorf("failed to merge app states: %w", err)
			}
		} else {
			ss.log.Debug("No desired state changes received")
		}

		ss.log.Debug("App states synced successfully")
		return nil

	default:
		// Handle error responses from orchestration service
		ss.log.Errorw("Received error response from orchestrator",
			"statusCode", desiredStateResp.StatusCode())
		return ss.handleErrorResponse(desiredStateResp.Body, desiredStateResp.StatusCode(), "sync app state")
	}
}

// handleErrorResponse processes error responses consistently.
func (ss *appStateSeeker) handleErrorResponse(errBody []byte, statusCode int, operation string) error {
	body, err := io.ReadAll(bytes.NewReader(errBody))
	if err != nil {
		ss.log.Errorw("Failed to read response body", "operation", operation, "statusCode", statusCode, "error", err)
		return fmt.Errorf("%s request failed with status %d (could not read response body): %w", operation, statusCode, err)
	}

	ss.log.Errorw("Request failed", "operation", operation, "statusCode", statusCode, "body", string(body))
	return fmt.Errorf("%s request failed with status %d: %s", operation, statusCode, string(body))
}

// mergeAppStates merges the desired app states with the current app states.
func (ss *appStateSeeker) mergeAppStates(states sbi.DesiredAppStates) error {
	ss.log.Debugw("Merging desired app states", "desiredStates", pretty.Sprint(states))

	for _, state := range states {
		if state.AppDeploymentYAML == nil {
			ss.log.Warnw("Received state with nil AppDeploymentYAML, skipping", "appId", state.AppId)
			continue
		}

		appDeployment, err := pkg.ParseAppDeploymentFromBase64(*state.AppDeploymentYAML)
		if err != nil {
			return fmt.Errorf("failed to parse the deployment yaml from the base64 encoded string, err: %s", err.Error())
		}

		if appDeployment.Metadata.Id == nil {
			ss.log.Warnw("Received AppDeployment with nil ID in the metadata field, skipping", "appId", state.AppId)
			continue
		}
		ss.log.Debugw("Successfully processed AppDeployment", "name", appDeployment.Metadata.Name, "id", *appDeployment.Metadata.Id)

		switch state.AppState {
		case sbi.REMOVING:
			ss.log.Infow("Removing app", "appId", *appDeployment.Metadata.Id)
			ss.database.Remove(*appDeployment.Metadata.Id)

		case sbi.RUNNING, sbi.UPDATING:
			ss.log.Infow("Adding/Updating app", "appId", *appDeployment.Metadata.Id, "state", state.AppState)
			newAppState, err := pkg.ConvertAppDeploymentToAppState(appDeployment, *appDeployment.Metadata.Id, "v1", "RUNNING")
			if err != nil {
				ss.log.Errorw("Failed to convert AppDeployment to AppState", "appId", *appDeployment.Metadata.Id, "error", err)
				return err
			}

			if state.AppState == sbi.RUNNING {
				ss.database.AddWorkload(newAppState)
			} else {
				ss.database.UpdateWorkload(newAppState)
			}

		default:
			ss.log.Warnw("Unknown app state, skipping", "appId", *appDeployment.Metadata.Id, "state", state.AppState)
		}
	}

	ss.log.Debug("App states merged successfully")
	return nil
}
