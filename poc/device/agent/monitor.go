// monitor/helm_monitor.go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/margo/sandbox/poc/device/agent/database"
	"github.com/margo/sandbox/shared-lib/workloads"
	"github.com/margo/sandbox/standard/generatedCode/wfm/sbi"

	//	"github.com/margo/sandbox/standard/pkg"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/release"
)

type DeploymentMonitorIfc interface {
	Start()
	Stop()
}

type DeploymentMonitor struct {
	database      database.DatabaseIfc
	helmClient    *workloads.HelmClient
	composeClient *workloads.DockerComposeCliClient
	log           *zap.SugaredLogger
	stopChan      chan struct{}
}

func NewDeploymentMonitor(db database.DatabaseIfc, helmClient *workloads.HelmClient, composeClient *workloads.DockerComposeCliClient, log *zap.SugaredLogger) *DeploymentMonitor {
	return &DeploymentMonitor{
		database:      db,
		helmClient:    helmClient,
		composeClient: composeClient,
		log:           log,
		stopChan:      make(chan struct{}),
	}
}

func (hm *DeploymentMonitor) Start() {
	go hm.monitorLoop()
}

func (hm *DeploymentMonitor) Stop() {
	close(hm.stopChan)
}

func (hm *DeploymentMonitor) monitorLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hm.checkAllDeployments()
		case <-hm.stopChan:
			return
		}
	}
}

func (hm *DeploymentMonitor) checkAllDeployments() {
	deployments := hm.database.ListDeployments()

	for _, deployment := range deployments {
		if deployment.Phase == "running" || deployment.Phase == "deploying" {
			go hm.checkDeployment(deployment.AppID)
		}
	}
}

func (hm *DeploymentMonitor) checkDeployment(appID string) {
	record, err := hm.database.GetDeployment(appID)
	if err != nil || record.CurrentState == nil {
		return
	}

	// Get the app deployment manifest directly
	appDeployment := record.CurrentState.AppDeploymentManifest

	if len(appDeployment.Spec.DeploymentProfile.Components) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, component := range appDeployment.Spec.DeploymentProfile.Components {
		helmComp, err := component.AsHelmApplicationDeploymentProfileComponent()
		if err != nil {
			hm.log.Warnw("Failed to convert component to Helm component", "appID", appID, "error", err)
			continue
		}

		releaseName := fmt.Sprintf("%s-%s", helmComp.Name, appID[:8])

		status, err := hm.helmClient.GetReleaseStatus(ctx, releaseName, "")
		if err != nil {
			errMsg := err.Error()
			componentStatus := sbi.ComponentStatus{
				Name:  helmComp.Name,
				State: sbi.ComponentStatusStateFailed,
				Error: &struct {
					Code    *string `json:"code,omitempty"`
					Message *string `json:"message,omitempty"`
				}{
					Code:    strPtr("HELM_STATUS_ERROR"),
					Message: &errMsg,
				},
			}
			hm.database.SetComponentStatus(appID, helmComp.Name, componentStatus)
			continue
		}

		componentState := hm.convertHelmStatus(status.Status)
		componentStatus := sbi.ComponentStatus{
			Name:  helmComp.Name,
			State: componentState,
			Error: nil,
		}

		hm.database.SetComponentStatus(appID, helmComp.Name, componentStatus)
	}
}

func (hm *DeploymentMonitor) convertHelmStatus(status release.Status) sbi.ComponentStatusState {
	switch status {
	case release.StatusDeployed:
		return sbi.ComponentStatusStateInstalled
	case release.StatusFailed:
		return sbi.ComponentStatusStateFailed
	case release.StatusPendingInstall, release.StatusPendingUpgrade:
		return sbi.ComponentStatusStateInstalling
	case release.StatusUninstalling:
		return sbi.ComponentStatusStateRemoving
	default:
		return sbi.ComponentStatusStateFailed
	}
}
