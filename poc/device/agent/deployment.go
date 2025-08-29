// deploy/manager.go
package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kr/pretty"
	"github.com/margo/dev-repo/poc/device/agent/database"
	"github.com/margo/dev-repo/shared-lib/workloads"
	"github.com/margo/dev-repo/standard/generatedCode/wfm/sbi"
	"github.com/margo/dev-repo/standard/pkg"
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
}

func NewDeploymentManager(db database.DatabaseIfc, helmClient *workloads.HelmClient, composeClient *workloads.DockerComposeCliClient, log *zap.SugaredLogger) *DeploymentManager {
	return &DeploymentManager{
		database:      db,
		helmClient:    helmClient,
		composeClient: composeClient,
		log:           log,
		stopChan:      make(chan struct{}),
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

func (dm *DeploymentManager) onDeploymentChange(deploymentId string, record *database.DeploymentRecord, changeType database.DeploymentChangeType) {
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

	switch record.DesiredState.AppState {
	case sbi.RUNNING:
		dm.log.Debugw("deploying or updating the deployment", "deploymentId", deploymentId)
		dm.deployOrUpdate(ctx, deploymentId, *record.DesiredState)
	case sbi.REMOVING:
		dm.log.Debugw("removing the deployment", "deploymentId", deploymentId)
		dm.remove(ctx, deploymentId)
	case "REMOVED": // TODO: remove this one later on
		return
	}
}

func (dm *DeploymentManager) deployOrUpdate(ctx context.Context, deploymentId string, desiredState sbi.AppState) {
	dm.database.SetPhase(deploymentId, "DEPLOYING", "Starting deployment")

	// Convert to AppDeployment
	appDeployment, err := pkg.ConvertAppStateToAppDeployment(desiredState)
	if err != nil {
		dm.database.SetPhase(deploymentId, "FAILED", fmt.Sprintf("Conversion failed: %v", err))
		return
	}

	// Get component
	if len(appDeployment.Spec.DeploymentProfile.Components) == 0 {
		dm.database.SetPhase(deploymentId, "FAILED", "No components found")
		return
	}

	// Determine deployment type and route accordingly
	profileType := appDeployment.Spec.DeploymentProfile.Type
	switch profileType {
	case sbi.HelmV3:
		err = dm.deployOrUpdateHelm(ctx, deploymentId, appDeployment)
	case sbi.Compose:
		err = dm.deployOrUpdateCompose(ctx, deploymentId, appDeployment)
	default:
		dm.database.SetPhase(deploymentId, "FAILED", fmt.Sprintf("Unsupported deployment type: %s", profileType))
	}
	if err != nil {
		dm.database.SetPhase(deploymentId, "FAILED", fmt.Sprintf("operation failed: %v", err))
		return
	}

	// Update current state
	currentState := desiredState
	currentState.AppState = sbi.RUNNING
	dm.database.SetCurrentState(deploymentId, currentState)
	dm.database.SetPhase(deploymentId, "RUNNING", "Deployment successful")
	dm.log.Infow("Deployment successful", "appId", deploymentId)
}

func (dm *DeploymentManager) deployOrUpdateHelm(ctx context.Context, deploymentId string, appDeployment sbi.AppDeployment) error {
	component := appDeployment.Spec.DeploymentProfile.Components[0]
	helmComp, err := component.AsHelmApplicationDeploymentProfileComponent()
	if err != nil {
		return fmt.Errorf("invalid helm component: %v", err)
	}

	// Generate release name
	releaseName := fmt.Sprintf("%s-%s", helmComp.Name, deploymentId[:8])

	// Get values
	componentValues, _ := pkg.ConvertAllAppDeploymentParamsToValues(*appDeployment.Spec.Parameters)
	values := componentValues[helmComp.Name]

	// Deploy/Update
	release, err := dm.helmClient.GetReleaseStatus(ctx, releaseName, "")
	if err != nil {
		return fmt.Errorf("failed to check existing release info: %v", err)
	}

	if release != nil {
		// Release exists, update it
		dm.log.Infow("Updating existing Helm release", "releaseName", releaseName, "deploymentId", deploymentId)
		err = dm.helmClient.UpdateChart(ctx, releaseName, helmComp.Properties.Repository, "", values)
		if err != nil {
			return fmt.Errorf("failed to upgrade existing release: %v", err)
		}

		// release upgrade is successful
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

	dm.log.Infow("Helm deployment successful", "appId", deploymentId, "releaseName", releaseName)
	return err
}

func (dm *DeploymentManager) deployOrUpdateCompose(ctx context.Context, deploymentId string, appDeployment sbi.AppDeployment) error {
	component := appDeployment.Spec.DeploymentProfile.Components[0]
	composeComp, err := component.AsComposeApplicationDeploymentProfileComponent()
	if err != nil {
		return fmt.Errorf("invalid compose component %v", err)
	}

	// Generate project name (must be valid Docker Compose project name)
	projectName := fmt.Sprintf("%s-%s", strings.ToLower(composeComp.Name), deploymentId[:8])
	projectName = strings.ReplaceAll(projectName, "_", "-")

	componentValues, _ := pkg.ConvertAllAppDeploymentParamsToValues(*appDeployment.Spec.Parameters)
	values := componentValues[composeComp.Name]

	// Get compose content from package location
	dm.log.Infow("view of the compose component", "composecomp", pretty.Sprint(composeComp))

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
	dm.database.SetPhase(deploymentId, "REMOVING", "Starting removal")

	record, err := dm.database.GetDeployment(deploymentId)
	if err != nil {
		return
	}

	if record.CurrentState == nil {
		dm.log.Infof("there was no current state found against the deployment, proceeding with complete removal", "expected", "complete removal", "currentState", "null")
		dm.database.SetPhase(deploymentId, "REMOVED", "Removal Complete")
		dm.database.RemoveDeployment(deploymentId)
		return
	}

	currentState := *record.CurrentState
	currentState.AppState = sbi.REMOVING
	dm.database.SetCurrentState(deploymentId, currentState)

	// Convert and get release name
	appDeployment, err := pkg.ConvertAppStateToAppDeployment(*record.CurrentState)
	if err != nil {
		dm.database.SetPhase(deploymentId, "FAILED", fmt.Sprintf("Conversion failed: %v", err))
		return
	}

	if len(appDeployment.Spec.DeploymentProfile.Components) == 0 {
		dm.database.SetPhase(deploymentId, "REMOVED", "No components to remove")
		dm.database.RemoveDeployment(deploymentId)
		return
	}

	// Route removal based on deployment type
	profileType := appDeployment.Spec.DeploymentProfile.Type

	switch profileType {
	case sbi.HelmV3:
		dm.removeHelm(ctx, deploymentId, appDeployment)
	case sbi.Compose:
		dm.removeCompose(ctx, deploymentId, appDeployment)
	default:
		dm.log.Warnw("Unknown deployment type for removal", "type", profileType, "deploymentId", deploymentId)
	}

	dm.database.SetPhase(deploymentId, "REMOVED", "Removal Complete")
	dm.database.RemoveDeployment(deploymentId)
	dm.log.Infow("Removal completed", "appId", deploymentId)
}

func (dm *DeploymentManager) removeHelm(ctx context.Context, deploymentId string, appDeployment sbi.AppDeployment) error {
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

func (dm *DeploymentManager) removeCompose(ctx context.Context, deploymentId string, appDeployment sbi.AppDeployment) error {
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
