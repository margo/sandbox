package wfm

import (
	"context"
	"net/http"

	"github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
)

// SBIAPIClient interface
type SBIAPIClientInterface interface {
     // OnboardDeviceClient removed — onboarding endpoint no longer in SBI spec.
    // Will be re-implemented as part of MIAF SUP (operator pre-provisioning via mTLS/X.509-SVID).
	SyncState(
		ctx context.Context,
		deviceClientId string,
		etag string,
		overrideOptions ...HTTPApiClientRequestEditorOptions,
	) (desiredStates *sbi.UnsignedAppStateManifest, err error)
	SyncStateWithResponse(
		ctx context.Context,
		deviceClientId string,
		etag string,
		overrideOptions ...HTTPApiClientRequestEditorOptions,
	) (desiredStates *sbi.UnsignedAppStateManifest, response *http.Response, err error)
	FetchDeploymentYAML(
		ctx context.Context,
		deviceClientId, deploymentId, digest string,
		overrideOptions ...HTTPApiClientRequestEditorOptions,
	) (yamlContent []byte, err error)
	DownloadBundle(
		ctx context.Context,
		deviceClientId, digest string,
		overrideOptions ...HTTPApiClientRequestEditorOptions,
	) (bundleData []byte, err error)
	ReportCapabilities(
		ctx context.Context,
		deviceId string,
		capabilities sbi.DeviceCapabilitiesManifest,
		overrideOptions ...HTTPApiClientRequestEditorOptions,
	) error
	ReportDeploymentStatus(
		ctx context.Context,
		deviceID, appID string,
		overallAppStatus sbi.DeploymentStatusManifestStatusState,
		components []sbi.ComponentStatus,
		err error,
	) error
	// DeboardDeviceClient(ctx context.Context, clientId string, overrideOptions
	// ...HTTPApiClientOptions) error
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
