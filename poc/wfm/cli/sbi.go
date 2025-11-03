package wfm

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/margo/dev-repo/standard/generatedCode/wfm/sbi"
)

const (
	// southboundBaseURL is the default base URL path for the Northbound API
	southboundBaseURL = "margo/sbi/v1"

	// Default timeout for API requests
	sbiDefaultTimeout = 30 * time.Second
)

type HTTPApiClientRequestEditorOptions = sbi.RequestEditorFn
type HTTPApiClientOptions = sbi.ClientOption

// SbiHttpClient implementation
type SbiHttpClient struct {
	// You can add any dependencies needed for creating the client here, like auth info
	// clientId, clientSecret, tokenUrl string
	url     string
	client  sbi.ClientInterface
	options []HTTPApiClientOptions
}

func NewSbiHTTPClient(url string, options ...HTTPApiClientOptions) (*SbiHttpClient, error) {
	client, err := sbi.NewClient(url)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}
	for _, opt := range options {
		opt(client)
	}

	apiClient := &SbiHttpClient{
		url:     url,
		client:  client,
		options: options,
	}
	return apiClient, nil
}

func (self *SbiHttpClient) OnboardDeviceClient(ctx context.Context, deviceCertificate []byte, overrideOptions ...HTTPApiClientRequestEditorOptions) (clientId string, endpoints []string, err error) {
	cert := base64.StdEncoding.EncodeToString([]byte(deviceCertificate))

	onboardingReq := sbi.OnboardingRequest{
		PublicCertificate: &cert,
	}

	resp, err := self.client.PostOnboarding(ctx, onboardingReq, overrideOptions...)
	if err != nil {
		return "", nil, fmt.Errorf("onboarding failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", nil, fmt.Errorf("onboarding failed with status: %d", resp.StatusCode)
	}

	onboardingResp, err := sbi.ParsePostOnboardingResponse(resp)
	if err != nil {
		return "", nil, fmt.Errorf("onboarding device response parsing failed: %w", err)
	}

	if onboardingResp.JSON200.ClientId == "" {
		// or panic?
		return "", nil, fmt.Errorf("the clientid is empty in the onboarding response, this should never happen!")
	}

	var endpointsList []string

	if onboardingResp.JSON200.EndpointList != nil {
		endpointsList = *onboardingResp.JSON200.EndpointList
	}
	return onboardingResp.JSON200.ClientId, endpointsList, nil
}

func (self *SbiHttpClient) ReportCapabilities(ctx context.Context, deviceClientId string, capabilities sbi.DeviceCapabilities, overrideOptions ...HTTPApiClientRequestEditorOptions) error {
	resp, err := self.client.PostClientClientIdCapabilities(ctx, deviceClientId, capabilities)
	if err != nil {
		return fmt.Errorf("failed to report capabilities: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("capabilities reporting failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (self *SbiHttpClient) SyncState(ctx context.Context, deviceClientId string, currentStates sbi.CurrentAppStates, overrideOptions ...HTTPApiClientRequestEditorOptions) (desiredStates sbi.DesiredAppStates, err error) {
	resp, err := self.client.State(
		ctx,
		deviceClientId,
		currentStates,
		overrideOptions...,
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("an errornous status code returned by the server, %v", resp.Request.Response.StatusCode)
	}

	// Parse response
	desiredStateResp, err := sbi.ParseStateResponse(resp)
	if err != nil {
		return nil, err
	}
	if desiredStateResp.JSON200 == nil {
		return nil, fmt.Errorf("non okay status received from the server, %v", desiredStateResp.JSONDefault)
	}

	return *desiredStateResp.JSON200, nil
}

func (self *SbiHttpClient) ReportDeploymentStatus(ctx context.Context, deviceID, appID string, overallAppStatus sbi.OverallStatus, components []sbi.ComponentStatus) error {
	appUUID, err := uuid.Parse(appID)
	if err != nil {
		return err
	}

	deploymentStatus := sbi.DeploymentStatus{
		ApiVersion:   "deployment.margo/v1",
		Kind:         "DeploymentStatus",
		Components:   components,
		DeploymentId: appUUID,
		Status:       overallAppStatus,
	}

	resp, err := self.client.PostClientClientIdDeploymentDeploymentIdStatus(ctx, deviceID, appUUID, deploymentStatus)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
