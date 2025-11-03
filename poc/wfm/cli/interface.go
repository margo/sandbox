package wfm

import (
	"context"

	"github.com/margo/dev-repo/standard/generatedCode/wfm/sbi"
)

// SBIAPIClient interface
type SBIAPIClientInterface interface {
	OnboardDeviceClient(ctx context.Context, deviceSignature []byte, overrideOptions ...HTTPApiClientRequestEditorOptions) (clientId string, endpoints []string, err error)
	SyncState(ctx context.Context, deviceClientId string, currentStates sbi.CurrentAppStates, overrideOptions ...HTTPApiClientRequestEditorOptions) (desiredStates sbi.DesiredAppStates, err error)
	ReportCapabilities(ctx context.Context, deviceId string, capabilities sbi.DeviceCapabilities, overrideOptions ...HTTPApiClientRequestEditorOptions) error
	ReportDeploymentStatus(ctx context.Context, deviceID, appID string, overallAppStatus sbi.OverallStatus, components []sbi.ComponentStatus) error
	// DeboardDeviceClient(ctx context.Context, clientId string, overrideOptions ...HTTPApiClientOptions) error
}

type NBIAPIClientInterface interface {
	OnboardAppPkg(params AppPkgOnboardingReq) (*AppPkgOnboardingResp, error)
	GetAppPkg(pkgId string) (*AppPkgSummary, error)
	ListAppPkgs(params ListAppPkgsParams) (*ListAppPkgsResp, error)
	DeleteAppPkg(pkgId string) error
	CreateDeployment(params DeploymentReq) (*DeploymentResp, error)
	GetDeployment(deploymentId string) (*DeploymentResp, error)
	ListDeployments(params DeploymentListParams)
	DeleteDeployment(deploymentId string) error
	ListDevices() (*DeviceListResp, error)
}
