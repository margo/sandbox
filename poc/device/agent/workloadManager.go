package main

import (
	"context"
	"fmt"

	"github.com/margo/dev-repo/poc/device/agent/database"
	workloads "github.com/margo/dev-repo/shared-lib/workloads"
	"github.com/margo/dev-repo/standard/generatedCode/wfm/sbi"
	"github.com/margo/dev-repo/standard/pkg"
	"go.uber.org/zap"
)

// WorkloadManager interface defines the contract for managing application workloads
type WorkloadManager interface {
	// Start initializes the workload manager and begins listening for database events
	Start() error

	// Stop gracefully shuts down the workload manager
	Stop() error

	// ExplicitlyTriggerDeploy manually triggers deployment of an application
	ExplicitlyTriggerDeploy(app sbi.AppState) error

	// ExplicitlyTriggerUpdate manually triggers update of an application
	ExplicitlyTriggerUpdate(app sbi.AppState) error

	// ExplicitlyTriggerRemove manually triggers removal of an application
	ExplicitlyTriggerRemove(appID string) error

	// DatabaseSubscriber interface methods for event-driven operations
	database.WorkloadDatabaseSubscriber
}

// workloadManager implements the WorkloadManager interface with event-driven architecture
type workloadManager struct {
	ctx                 context.Context
	operationStopper    context.CancelFunc
	log                 *zap.SugaredLogger
	database            database.AgentDatabase
	helmClient          *workloads.HelmClient
	dockerComposeClient *workloads.DockerComposeClient
}

// NewWorkloadManager creates a new WorkloadManager instance
func NewWorkloadManager(ctx context.Context, log *zap.SugaredLogger, database database.AgentDatabase, helmClient *workloads.HelmClient, dockerComposeClient *workloads.DockerComposeClient) WorkloadManager {
	localCtx, localCanceller := context.WithCancel(ctx)
	return &workloadManager{
		log:                 log,
		ctx:                 localCtx,
		operationStopper:    localCanceller,
		database:            database,
		helmClient:          helmClient,
		dockerComposeClient: dockerComposeClient,
	}
}

func (wm *workloadManager) Start() error {
	// Subscribe to database events for reactive workload management
	if err := wm.database.Subscribe(wm); err != nil {
		return fmt.Errorf("failed to subscribe to database events: %w", err)
	}
	return nil
}

// Stop gracefully shuts down the workload manager
func (wm *workloadManager) Stop() error {
	return nil
}

// GetSubscriberID returns a unique identifier for this database subscriber
func (wm *workloadManager) GetSubscriberID() string {
	return "workload-manager"
}

// OnDatabaseEvent handles database events and triggers appropriate workload operations
func (wm *workloadManager) OnDatabaseEvent(event database.WorkloadDatabaseEvent) error {
	wm.log.Debugw("Received database event",
		"type", event.Type,
		"appId", event.AppID,
		"timestamp", event.Timestamp)

	switch event.Type {
	case database.EventAppAdded:
		if event.NewState != nil {
			wm.log.Infow("Handling new app deployment", "appId", event.AppID)
			return wm.deploy(*event.NewState)
		}
	case database.EventAppUpdated:
		if event.NewState != nil {
			wm.log.Infow("Handling app update", "appId", event.AppID)
			return wm.update(*event.NewState)
		}
	case database.EventAppDeleted:
		wm.log.Infow("Handling app removal", "appId", event.AppID)
		return wm.remove(event.AppID)
	default:
		wm.log.Debugw("Ignoring unhandled event type", "type", event.Type)
	}

	return nil
}

// ExplicitlyTriggerDeploy manually triggers deployment of an application
func (wm *workloadManager) ExplicitlyTriggerDeploy(app sbi.AppState) error {
	wm.log.Infow("Explicitly triggering deployment", "appId", app.AppId)
	return wm.deploy(app)
}

// ExplicitlyTriggerUpdate manually triggers update of an application
func (wm *workloadManager) ExplicitlyTriggerUpdate(app sbi.AppState) error {
	wm.log.Infow("Explicitly triggering update", "appId", app.AppId)
	return wm.update(app)
}

// ExplicitlyTriggerRemove manually triggers removal of an application
func (wm *workloadManager) ExplicitlyTriggerRemove(appID string) error {
	wm.log.Infow("Explicitly triggering removal", "appId", appID)
	return wm.remove(appID)
}

// deploy performs the actual deployment of an application workload
func (wm *workloadManager) deploy(app sbi.AppState) error {
	wm.log.Infow("Deploying workload", "appId", app.AppId)

	// Convert AppState to AppDeployment
	appDeployment, err := pkg.ConvertAppStateToAppDeployment(app)
	if err != nil {
		return fmt.Errorf("failed to convert AppState to AppDeployment: %w", err)
	}

	// Validate components
	if len(appDeployment.Spec.DeploymentProfile.Components) == 0 {
		return fmt.Errorf("no component specified inside the deployment profile")
	}

	// Determine the workload type and deploy
	switch appDeployment.Spec.DeploymentProfile.Type {
	case "helm.v3":
		return wm.deployHelm(appDeployment)
	case "compose":
		return fmt.Errorf("unsupported deployment profile type: %s", appDeployment.Spec.DeploymentProfile.Type)
	default:
		return fmt.Errorf("unsupported deployment profile type: %s", appDeployment.Spec.DeploymentProfile.Type)
	}
}

// update performs the actual update of an application workload
func (wm *workloadManager) update(app sbi.AppState) error {
	wm.log.Infow("Updating workload", "appId", app.AppId)
	// TODO: Implement update logic
	return fmt.Errorf("update not implemented")
}

// remove performs the actual removal of an application workload
func (wm *workloadManager) remove(appID string) error {
	wm.log.Infow("Removing workload", "appId", appID)
	// TODO: Implement removal logic
	return fmt.Errorf("remove not implemented")
}

// deployHelm handles Helm-based deployments
func (wm *workloadManager) deployHelm(deployment sbi.AppDeployment) error {
	componentValues, err := pkg.ConvertAllAppDeploymentParamsToValues(*deployment.Spec.Parameters)
	if err != nil {
		return fmt.Errorf("failed to parse the parameters for value override in helm chart deployment: %s", err.Error())
	}

	// Assuming there will be one single chart in the profile
	component := deployment.Spec.DeploymentProfile.Components[0]
	componentAsHelm, err := component.AsHelmApplicationDeploymentProfileComponent()
	if err != nil {
		return fmt.Errorf("the deployment profile info is not helm application deployment profile, %s", err.Error())
	}

	repository := componentAsHelm.Properties.Repository
	if err := wm.helmClient.AddRepository("", repository, workloads.HelmRepoAuth{}); err != nil {
		return fmt.Errorf("failed to add repository: %w", err)
	}

	namespace := ""                     // Let us not set the namespace
	releaseName := componentAsHelm.Name // We are using component name as release name
	revision := componentAsHelm.Properties.Revision
	wait := componentAsHelm.Properties.Wait

	// Extract the overriding values for this component
	overrides := componentValues[componentAsHelm.Name]

	if err := wm.helmClient.InstallChart(releaseName, repository, namespace, *revision, *wait, overrides); err != nil {
		return err
	}

	return nil
}
