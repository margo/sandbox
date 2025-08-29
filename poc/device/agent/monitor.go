// monitor/helm_monitor.go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/margo/dev-repo/poc/device/agent/database"
	"github.com/margo/dev-repo/shared-lib/workloads"
	"github.com/margo/dev-repo/standard/generatedCode/wfm/sbi"
	"github.com/margo/dev-repo/standard/pkg"
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

	// Convert to get component info
	appDeployment, err := pkg.ConvertAppStateToAppDeployment(*record.CurrentState)
	if err != nil {
		return
	}

	if len(appDeployment.Spec.DeploymentProfile.Components) == 0 {
		return
	}

	component := appDeployment.Spec.DeploymentProfile.Components[0]
	helmComp, err := component.AsHelmApplicationDeploymentProfileComponent()
	if err != nil {
		return
	}

	releaseName := fmt.Sprintf("%s-%s", helmComp.Name, appID[:8])

	// Get Helm status
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status, err := hm.helmClient.GetReleaseStatus(ctx, releaseName, "")
	if err != nil {
		// Release not found or error
		componentStatus := sbi.ComponentStatus{
			Name:  helmComp.Name,
			State: sbi.ComponentStatusStateFailed,
			Error: &sbi.Error{Message: &[]string{err.Error()}[0]},
		}
		hm.database.SetComponentStatus(appID, helmComp.Name, componentStatus)
		return
	}

	// Convert Helm status to component status
	componentState := hm.convertHelmStatus(status.Status)
	componentStatus := sbi.ComponentStatus{
		Name:  helmComp.Name,
		State: componentState,
		Error: nil,
	}

	hm.database.SetComponentStatus(appID, helmComp.Name, componentStatus)
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
		return "uninstalling" // TODO: define uninstalling state in standard package
	default:
		return sbi.ComponentStatusStateFailed
	}
}
