// deploy/manager.go
package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/kr/pretty"
	"github.com/margo/sandbox/poc/device/agent/database"
	"github.com/margo/sandbox/shared-lib/workloads"
	"github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
	"github.com/margo/sandbox/standard/pkg"
	"go.uber.org/zap"
)

type DeploymentManagerIfc interface {
	Start()
	Stop()
}

type DeploymentManager struct {
	database      database.DatabaseIfc
	helmClient    *workloads.HelmClient
	composeClient *workloads.DockerComposeCliClient
	log           *zap.SugaredLogger
	stopChan      chan struct{}
	//  Mutex to prevent concurrent reconciliation
	reconcileLocks sync.Map // map[deploymentId]bool
}

func NewDeploymentManager(db database.DatabaseIfc, helmClient *workloads.HelmClient, composeClient *workloads.DockerComposeCliClient, log *zap.SugaredLogger) *DeploymentManager {
	return &DeploymentManager{
		database:       db,
		helmClient:     helmClient,
		composeClient:  composeClient,
		log:            log,
		stopChan:       make(chan struct{}),
		reconcileLocks: sync.Map{},
	}
}

func (dm *DeploymentManager) Start() {
	// Subscribe to database changes
	dm.database.Subscribe(dm.onDeploymentChange)

	// Start reconciliation loop
	go dm.reconcileLoop()
}

func (dm *DeploymentManager) Stop() {
	close(dm.stopChan)
}

func (dm *DeploymentManager) onDeploymentChange(deploymentId string, record *database.DeploymentRecord, changeType database.DeploymentRecordChangeType) {
	if changeType == database.DeploymentChangeTypeDesiredStateAdded {
		if dm.database.NeedsReconciliation(deploymentId) {
			dm.log.Infow("Deployment needs reconciliation", "appId", deploymentId)
			go dm.reconcileDeployment(deploymentId)
		}
	}
}

func (dm *DeploymentManager) reconcileLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			dm.reconcileAll()
		case <-dm.stopChan:
			return
		}
	}
}

func (dm *DeploymentManager) reconcileAll() {
	deployments := dm.database.ListDeployments()
	for _, deployment := range deployments {
		if dm.database.NeedsReconciliation(deployment.DeploymentID) {
			go dm.reconcileDeployment(deployment.DeploymentID)
		}
	}
}

func (dm *DeploymentManager) reconcileDeployment(deploymentId string) {
	//  Prevent concurrent reconciliation of the same deployment
	if _, loaded := dm.reconcileLocks.LoadOrStore(deploymentId, true); loaded {
		dm.log.Debugw("Reconciliation already in progress, skipping", "deploymentId", deploymentId)
		return
	}
	defer dm.reconcileLocks.Delete(deploymentId)

	record, err := dm.database.GetDeployment(deploymentId)
	if err != nil {
		dm.log.Errorw("Failed to get deployment", "deploymentId", deploymentId, "error", err)
		return
	}

	if record.DesiredState == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Get the desired state from the manifest
	desiredState := record.DesiredState.Status.Status.State

	// Get current state (what's actually deployed)
	var currentState sbi.DeploymentStatusManifestStatusState
	if record.CurrentState != nil {
		currentState = record.CurrentState.Status.Status.State
	} else {
		currentState = sbi.DeploymentStatusManifestStatusStatePending
	}

	dm.log.Debugw("Reconciling deployment",
		"deploymentId", deploymentId,
		"desiredState", desiredState,
		"currentState", currentState)

	// Only reconcile if states don't match
	switch desiredState {
	case sbi.DeploymentStatusManifestStatusStatePending:
		// Only deploy if not already installed
		if currentState != sbi.DeploymentStatusManifestStatusStateInstalled {
			dm.log.Debugw("deploying pending deployment", "deploymentId", deploymentId)
			dm.deployOrUpdate(ctx, deploymentId, *record.DesiredState)
		} else {
			dm.log.Debugw("deployment already installed, skipping", "deploymentId", deploymentId)
		}

	case sbi.DeploymentStatusManifestStatusStateInstalling:
		// Only deploy if not already installed
		if currentState != sbi.DeploymentStatusManifestStatusStateInstalled {
			dm.log.Debugw("deploying or updating the deployment", "deploymentId", deploymentId)
			dm.deployOrUpdate(ctx, deploymentId, *record.DesiredState)
		} else {
			dm.log.Debugw("deployment already installed, skipping", "deploymentId", deploymentId)
		}

	case sbi.DeploymentStatusManifestStatusStateRemoving:
		// Only remove if not already removed
		if currentState != sbi.DeploymentStatusManifestStatusStateRemoved {
			dm.log.Debugw("removing the deployment", "deploymentId", deploymentId)
			dm.remove(ctx, deploymentId)
		} else {
			dm.log.Debugw("deployment already removed, skipping", "deploymentId", deploymentId)
		}

	case sbi.DeploymentStatusManifestStatusStateRemoved:
		dm.log.Debugw("deployment already removed", "deploymentId", deploymentId)
		return

	case sbi.DeploymentStatusManifestStatusStateInstalled:
		// Check if current state matches
		if currentState != sbi.DeploymentStatusManifestStatusStateInstalled {
			dm.log.Debugw("current state doesn't match desired, reconciling", "deploymentId", deploymentId)
			dm.deployOrUpdate(ctx, deploymentId, *record.DesiredState)
		} else {
			dm.log.Debugw("deployment already installed and matches desired state", "deploymentId", deploymentId)
		}

	case sbi.DeploymentStatusManifestStatusStateFailed:
		dm.log.Warnw("deployment in failed state", "deploymentId", deploymentId)
		return

	default:
		dm.log.Warnw("unknown deployment state", "deploymentId", deploymentId, "state", desiredState)
	}
}

func (dm *DeploymentManager) deployOrUpdate(ctx context.Context, deploymentId string, desiredState database.AppDeploymentState) {
	// Use the AppDeploymentManifest directly instead of converting
	appDeployment := desiredState.AppDeploymentManifest

	// Get component
	if len(appDeployment.Spec.DeploymentProfile.Components) == 0 {
		// Set current state even on failure
		failedState := desiredState
		failedState.Status.Status.State = sbi.DeploymentStatusManifestStatusStateFailed
		dm.database.SetCurrentState(deploymentId, failedState)
		dm.database.SetPhase(deploymentId, "FAILED", "No components found")
		return
	}

	// Initialize per-component status for ALL components before starting deployment.
	// This ensures the status report always contains one entry per component (spec requirement).
	componentNames := dm.extractComponentNames(appDeployment)
	for _, name := range componentNames {
		dm.database.SetComponentStatus(deploymentId, name, sbi.ComponentStatus{
			Name:  name,
			State: sbi.ComponentStatusStateInstalling,
		})
	}

	dm.database.SetPhase(deploymentId, "DEPLOYING", "Starting deployment")

	profileType := appDeployment.Spec.DeploymentProfile.Type
	var err error

	switch profileType {
	case sbi.HelmV3:
		//  Check if Helm client is available
		if dm.helmClient == nil {
			err = fmt.Errorf("Helm client not initialized (device may not support Helm deployments)")
		} else {
			err = dm.deployOrUpdateHelm(ctx, deploymentId, appDeployment)
		}

	case sbi.Compose:
		// Check if Compose client is available
		if dm.composeClient == nil {
			err = fmt.Errorf("Docker Compose client not initialized (device may not support Compose deployments)")
		} else {
			err = dm.deployOrUpdateCompose(ctx, deploymentId, appDeployment)
		}

	default:
		// Set current state on unsupported type
		for _, name := range componentNames {
			dm.database.SetComponentStatus(deploymentId, name, sbi.ComponentStatus{
				Name:  name,
				State: sbi.ComponentStatusStateFailed,
			})
		}
		failedState := desiredState
		failedState.Status.Status.State = sbi.DeploymentStatusManifestStatusStateFailed
		dm.database.SetCurrentState(deploymentId, failedState)
		dm.database.SetPhase(deploymentId, "FAILED", fmt.Sprintf("Unsupported deployment type: %s", profileType))
		return
	}

	// Handle deployment errors
	if err != nil {
		for _, name := range componentNames {
			errMsg := err.Error()
			dm.database.SetComponentStatus(deploymentId, name, sbi.ComponentStatus{
				Name:  name,
				State: sbi.ComponentStatusStateFailed,
				Error: &struct {
					Code    *string `json:"code,omitempty"`
					Message *string `json:"message,omitempty"`
				}{
					Code:    strPtr("DEPLOYMENT_ERROR"),
					Message: &errMsg,
				},
			})
		}
		failedState := desiredState
		failedState.Status.Status.State = sbi.DeploymentStatusManifestStatusStateFailed
		dm.database.SetCurrentState(deploymentId, failedState)
		dm.database.SetPhase(deploymentId, "FAILED", fmt.Sprintf("%s operation failed: %v", profileType, err))
		return
	}

	// Success - update all component states to installed
	for _, name := range componentNames {
		dm.database.SetComponentStatus(deploymentId, name, sbi.ComponentStatus{
			Name:  name,
			State: sbi.ComponentStatusStateInstalled,
		})
	}
	currentState := desiredState
	currentState.Status.Status.State = sbi.DeploymentStatusManifestStatusStateInstalled
	dm.database.SetCurrentState(deploymentId, currentState)
	dm.database.SetPhase(deploymentId, "RUNNING", "Deployment successful")
	dm.log.Infow("Deployment successful", "appId", deploymentId)
}

func (dm *DeploymentManager) deployOrUpdateHelm(ctx context.Context, deploymentId string, appDeployment sbi.AppDeploymentManifest) error {
	component := appDeployment.Spec.DeploymentProfile.Components[0]
	helmComp, err := component.AsHelmApplicationDeploymentProfileComponent()
	if err != nil {
		return fmt.Errorf("invalid helm component: %v", err)
	}

	// Generate release name
	releaseName := fmt.Sprintf("%s-%s", helmComp.Name, deploymentId[:8])

	values := map[string]interface{}{}
	if appDeployment.Spec.Parameters != nil {
		componentValues, err := pkg.ConvertAllAppDeploymentParamsToValues(*appDeployment.Spec.Parameters)
		if err != nil {
			return fmt.Errorf("failed to convert deployment profiles: %w", err)
		}
		if v, exists := componentValues[helmComp.Name]; exists {
			values = v
		}
	}

	values["fullnameOverride"] = releaseName // Makes all K8s resources unique

	dm.log.Infow("Deploying with unique resource names",
		"releaseName", releaseName,
		"fullnameOverride", releaseName)

	// Deploy/Update
	release, err := dm.helmClient.GetReleaseStatus(ctx, releaseName, "")
	if err != nil {
		dm.log.Infow("failed to check whether a release exists or not, assuming that it doesn't exist, will proceed with installation", "releaseName", releaseName, "deploymentId", deploymentId, "err", err.Error())

	}

	if release != nil {
		// Release exists, update it
		dm.log.Infow("Updating existing Helm release", "releaseName", releaseName, "deploymentId", deploymentId)
		err = dm.helmClient.UpdateChart(ctx, releaseName, helmComp.Properties.Repository, "", values)
		if err != nil {
			return fmt.Errorf("failed to upgrade existing release: %v", err)
		}
		return nil
	}

	// New deployment
	dm.log.Infow("Installing new Helm release", "releaseName", releaseName, "deploymentId", deploymentId)
	revision := "latest"
	if helmComp.Properties.Revision != nil {
		revision = *helmComp.Properties.Revision
	}
	wait := helmComp.Properties.Wait != nil && *helmComp.Properties.Wait
	err = dm.helmClient.InstallChart(ctx, releaseName, helmComp.Properties.Repository, "", revision, wait, values)
	if err != nil {
		return err
	}
	dm.log.Infow("Helm deployment successful", "appId", deploymentId, "releaseName", releaseName)
	return nil
}

func (dm *DeploymentManager) deployOrUpdateCompose(ctx context.Context, deploymentId string, appDeployment sbi.AppDeploymentManifest) error {
	component := appDeployment.Spec.DeploymentProfile.Components[0]
	composeComp, err := component.AsComposeApplicationDeploymentProfileComponent()
	if err != nil {
		return fmt.Errorf("invalid compose component %v", err)
	}
	// Get compose content from package location
	dm.log.Infow("view of the compose component", "composecomp", pretty.Sprint(composeComp))

	// Generate project name (must be valid Docker Compose project name)
	projectName := fmt.Sprintf("%s-%s", strings.ToLower(composeComp.Name), deploymentId[:8])
	projectName = strings.ReplaceAll(projectName, "_", "-")

	values := map[string]interface{}{}
	if appDeployment.Spec.Parameters != nil {
		componentValues, err := pkg.ConvertAllAppDeploymentParamsToValues(*appDeployment.Spec.Parameters)
		if err != nil {
			return fmt.Errorf("failed to parse compose parameters: %w", err)
		}
		if v, exists := componentValues[composeComp.Name]; exists {
			values = v
		}
	}

	composeFilename, err := dm.composeClient.DownloadCompose(ctx, composeComp.Properties.PackageLocation, composeComp.Properties.KeyLocation, projectName)
	if err != nil {
		return fmt.Errorf("failed to get compose content: %v", err)
	}
	dm.log.Debugw("preview of the compose file", "composeFilename", composeFilename)

	// Convert parameters to environment variables
	envVars := dm.convertParametersToEnvVars(values, composeComp.Name)

	// Check if project already exists
	exists, err := dm.composeClient.ComposeExists(ctx, composeFilename, projectName)
	if err != nil {
		return fmt.Errorf("failed to check compose project existence: %v", err)
	}
	if exists {
		// Update existing deployment
		dm.log.Infow("Updating existing Docker Compose project", "projectName", projectName, "deploymentId", deploymentId, "composeFilename", composeFilename)
		err = dm.composeClient.UpdateCompose(ctx, projectName, composeFilename, envVars)
	} else {
		// New deployment
		dm.log.Infow("Deploying new Docker Compose project", "projectName", projectName, "deploymentId", deploymentId, "composeFilename", composeFilename)
		err = dm.composeClient.DeployCompose(ctx, projectName, composeFilename, envVars)
	}

	if err != nil {
		return fmt.Errorf("docker compose operation failed: %v", err)
	}

	dm.log.Infow("Docker Compose deployment successful", "appId", deploymentId, "projectName", projectName)
	return nil
}

func (dm *DeploymentManager) remove(ctx context.Context, deploymentId string) {
	record, err := dm.database.GetDeployment(deploymentId)
	if err != nil {
		dm.log.Warnw("Deployment not found for removal", "deploymentId", deploymentId)
		return
	}

	if record.CurrentState == nil {
		dm.log.Infow("No current state found, proceeding with complete removal", "deploymentId", deploymentId)

		// Update desired state to REMOVED before deleting
		if record.DesiredState != nil {
			componentNames := dm.extractComponentNames(record.DesiredState.AppDeploymentManifest)
			for _, name := range componentNames {
				dm.database.SetComponentStatus(deploymentId, name, sbi.ComponentStatus{
					Name:  name,
					State: sbi.ComponentStatusStateRemoved,
				})
			}

			removedState := *record.DesiredState
			removedState.Status.Status.State = sbi.DeploymentStatusManifestStatusStateRemoved
			dm.database.SetCurrentState(deploymentId, removedState)
		}

		dm.database.SetPhase(deploymentId, "REMOVED", "Removal Complete")
		dm.database.RemoveDeployment(deploymentId)
		return
	}

	// Use the AppDeploymentManifest directly
	appDeployment := record.CurrentState.AppDeploymentManifest

	// Initialize per-component status to "removing"
	componentNames := dm.extractComponentNames(appDeployment)
	for _, name := range componentNames {
		dm.database.SetComponentStatus(deploymentId, name, sbi.ComponentStatus{
			Name:  name,
			State: sbi.ComponentStatusStateRemoving,
		})
	}

	//  Set current state to REMOVING
	currentState := *record.CurrentState
	currentState.Status.Status.State = sbi.DeploymentStatusManifestStatusStateRemoving
	dm.database.SetCurrentState(deploymentId, currentState)
	dm.database.SetPhase(deploymentId, "REMOVING", "Starting removal")

	if len(appDeployment.Spec.DeploymentProfile.Components) == 0 {
		dm.log.Warnw("No components to remove", "deploymentId", deploymentId)

		// Update state to REMOVED
		removedState := currentState
		removedState.Status.Status.State = sbi.DeploymentStatusManifestStatusStateRemoved
		dm.database.SetCurrentState(deploymentId, removedState)

		dm.database.SetPhase(deploymentId, "REMOVED", "No components to remove")
		dm.database.RemoveDeployment(deploymentId)
		return
	}

	// Route removal based on deployment type
	profileType := appDeployment.Spec.DeploymentProfile.Type

	var removeErr error
	switch profileType {
	case sbi.HelmV3:
		removeErr = dm.removeHelm(ctx, deploymentId, appDeployment)
	case sbi.Compose:
		removeErr = dm.removeCompose(ctx, deploymentId, appDeployment)
	default:
		dm.log.Warnw("Unknown deployment type for removal", "type", profileType, "deploymentId", deploymentId)
	}

	// Update per-component status to "removed" (or "failed")
	for _, name := range componentNames {
		if removeErr != nil {
			dm.database.SetComponentStatus(deploymentId, name, sbi.ComponentStatus{
				Name:  name,
				State: sbi.ComponentStatusStateFailed,
				Error: &struct {
					Code    *string `json:"code,omitempty"`
					Message *string `json:"message,omitempty"`
				}{
					Code:    strPtr("REMOVAL_ERROR"),
					Message: strPtr(removeErr.Error()),
				},
			})
		} else {
			dm.database.SetComponentStatus(deploymentId, name, sbi.ComponentStatus{
				Name:  name,
				State: sbi.ComponentStatusStateRemoved,
			})
		}
	}

	// Update current state to REMOVED (even if removal failed)
	removedState := currentState
	removedState.Status.Status.State = sbi.DeploymentStatusManifestStatusStateRemoved
	dm.database.SetCurrentState(deploymentId, removedState)

	if removeErr != nil {
		dm.log.Errorw("Removal failed but marking as removed",
			"deploymentId", deploymentId,
			"error", removeErr)
		dm.database.SetPhase(deploymentId, "REMOVED", fmt.Sprintf("Removal completed with errors: %v", removeErr))
	} else {
		dm.database.SetPhase(deploymentId, "REMOVED", "Removal Complete")
	}

	// Remove from local database (triggers status report via subscriber)
	dm.database.RemoveDeployment(deploymentId)

	dm.log.Infow("Removal completed", "appId", deploymentId)
}

func (dm *DeploymentManager) removeHelm(ctx context.Context, deploymentId string, appDeployment sbi.AppDeploymentManifest) error {
	// Check if Helm client is available
	if dm.helmClient == nil {
		dm.log.Warnw("Helm client not initialized, skipping Helm removal", "deploymentId", deploymentId)
		return nil // Return nil to allow cleanup to continue
	}

	component := appDeployment.Spec.DeploymentProfile.Components[0]
	if helmComp, err := component.AsHelmApplicationDeploymentProfileComponent(); err == nil {
		releaseName := fmt.Sprintf("%s-%s", helmComp.Name, deploymentId[:8])
		dm.log.Infow("Removing Helm release", "releaseName", releaseName, "deploymentId", deploymentId)

		if err := dm.helmClient.UninstallChart(ctx, releaseName, ""); err != nil {
			dm.log.Warnw("Failed to uninstall Helm chart", "releaseName", releaseName, "error", err)
			return err
		}
	}

	return nil
}

func (dm *DeploymentManager) removeCompose(ctx context.Context, deploymentId string, appDeployment sbi.AppDeploymentManifest) error {
	// Check if Compose client is available
	if dm.composeClient == nil {
		dm.log.Warnw("Docker Compose client not initialized, skipping Compose removal", "deploymentId", deploymentId)
		return nil // Return nil to allow cleanup to continue
	}

	component := appDeployment.Spec.DeploymentProfile.Components[0]
	if composeComp, err := component.AsComposeApplicationDeploymentProfileComponent(); err == nil {
		projectName := fmt.Sprintf("%s-%s", strings.ToLower(composeComp.Name), deploymentId[:8])
		projectName = strings.ReplaceAll(projectName, "_", "-")

		dm.log.Infow("Removing Docker Compose project", "projectName", projectName, "deploymentId", deploymentId)

		if err := dm.composeClient.RemoveCompose(ctx, projectName); err != nil {
			dm.log.Warnw("Failed to remove Docker Compose project", "projectName", projectName, "error", err)
			return err
		}
	}

	return nil
}

// extractComponentNames returns the name of every component in an AppDeploymentManifest,
// regardless of the deployment profile type (Helm, Compose, etc.).
func (dm *DeploymentManager) extractComponentNames(appDeployment sbi.AppDeploymentManifest) []string {
	names := make([]string, 0, len(appDeployment.Spec.DeploymentProfile.Components))
	for _, comp := range appDeployment.Spec.DeploymentProfile.Components {
		if helmComp, err := comp.AsHelmApplicationDeploymentProfileComponent(); err == nil {
			names = append(names, helmComp.Name)
		} else if composeComp, err := comp.AsComposeApplicationDeploymentProfileComponent(); err == nil {
			names = append(names, composeComp.Name)
		}
	}
	return names
}

func strPtr(s string) *string {
	return &s
}

// Helper function to convert parameters to environment variables
func (dm *DeploymentManager) convertParametersToEnvVars(params map[string]interface{}, componentName string) map[string]string {
	envVars := make(map[string]string)

	// Convert component-specific parameters
	if componentParams, exists := params[componentName]; exists {
		if paramMap, ok := componentParams.(map[string]interface{}); ok {
			for key, value := range paramMap {
				envVars[strings.ToUpper(key)] = fmt.Sprintf("%v", value)
			}
		}
	}

	// Convert global parameters
	for key, value := range params {
		if key != componentName { // Skip component-specific params already processed
			envVars[strings.ToUpper(key)] = fmt.Sprintf("%v", value)
		}
	}

	return envVars
}
