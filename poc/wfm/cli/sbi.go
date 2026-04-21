package wfm

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/margo/sandbox/shared-lib/cache"
	"github.com/margo/sandbox/shared-lib/pointers"
	"github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
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
	url             string
	client          sbi.ClientInterface
	options         []HTTPApiClientOptions
	bundleCache     *cache.BundleCache
	deploymentCache *cache.DeploymentCache
}

func NewSbiHTTPClient(url string, options ...HTTPApiClientOptions) (*SbiHttpClient, error) {
	client, err := sbi.NewClient(url)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}
	for _, opt := range options {
		if err := opt(client); err != nil {
			return nil, err
		}
	}

	// Initialize caches
	bundleCache, err := cache.NewBundleCache("data/cache")
	if err != nil {
		return nil, fmt.Errorf("failed to create bundle cache: %w", err)
	}

	deploymentCache, err := cache.NewDeploymentCache("data/cache")
	if err != nil {
		return nil, fmt.Errorf("failed to create deployment cache: %w", err)
	}

	apiClient := &SbiHttpClient{
		url:             url,
		client:          client,
		options:         options,
		bundleCache:     bundleCache,
		deploymentCache: deploymentCache,
	}
	return apiClient, nil
}

func (sbiClient *SbiHttpClient) OnboardDeviceClient(ctx context.Context, deviceCertificate []byte, overrideOptions ...HTTPApiClientRequestEditorOptions) (clientId string, endpoints []string, err error) {
	cert := base64.StdEncoding.EncodeToString(deviceCertificate)

	onboardingReq := sbi.PostApiV1OnboardingJSONRequestBody{
		ApiVersion:  "onboarding.margo.org/v1alpha1",
		Kind:        sbi.OnboardingRequest,
		Certificate: cert,
	}

	resp, err := sbiClient.client.PostApiV1Onboarding(ctx, onboardingReq, overrideOptions...)
	if err != nil {
		return "", nil, fmt.Errorf("onboarding failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return "", nil, fmt.Errorf("onboarding failed with status: %d", resp.StatusCode)
	}

	onboardingResp, err := sbi.ParsePostApiV1OnboardingResponse(resp)
	if err != nil {
		return "", nil, fmt.Errorf("onboarding device response parsing failed: %w", err)
	}

	if onboardingResp.JSON201 == nil {
		return "", nil, fmt.Errorf("unexpected response format: JSON201 is nil")
	}

	if onboardingResp.JSON201.ClientId == nil {
		return "", nil, fmt.Errorf("clientId is nil in the onboarding response")
	}

	if *onboardingResp.JSON201.ClientId == "" {
		return "", nil, fmt.Errorf("the clientid is empty in the onboarding response, this should never happen")
	}

	var endpointsList []string
	return *onboardingResp.JSON201.ClientId, endpointsList, nil
}

func (sbiClient *SbiHttpClient) ReportCapabilities(ctx context.Context, deviceClientId string, capabilities sbi.DeviceCapabilitiesManifest, overrideOptions ...HTTPApiClientRequestEditorOptions) error {
	resp, err := sbiClient.client.PostApiV1ClientsClientIdCapabilities(ctx, deviceClientId, capabilities)
	if err != nil {
		return fmt.Errorf("failed to report capabilities: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return fmt.Errorf("capabilities reporting failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (sbiClient *SbiHttpClient) SyncState(ctx context.Context, deviceClientId string, etag string, overrideOptions ...HTTPApiClientRequestEditorOptions) (desiredStates *sbi.UnsignedAppStateManifest, err error) {
	// Prepare parameters
	params := &sbi.GetApiV1ClientsClientIdDeploymentsParams{
		Accept: pointers.Ptr("application/vnd.margo.manifest.v1+json"),
	}

	// Only set If-None-Match if etag is not empty
	if etag != "" && etag != `""` {
		params.IfNoneMatch = &etag
	}

	resp, err := sbiClient.client.GetApiV1ClientsClientIdDeployments(
		ctx,
		deviceClientId,
		params,
		overrideOptions...,
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Parse response first
	desiredStateResp, err := sbi.ParseGetApiV1ClientsClientIdDeploymentsResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Handle status codes according to OpenAPI spec
	switch resp.StatusCode {
	case 200:
		// OK - new data available
		if desiredStateResp.ApplicationvndMargoManifestV1JSON200 != nil {
			return desiredStateResp.ApplicationvndMargoManifestV1JSON200, nil
		}
		return nil, fmt.Errorf("unexpected response structure for status 200")

	case 304:
		// Not Modified - no new data
		return nil, nil

	case 406:
		// Not Acceptable - server cannot generate response matching Accept header
		return nil, fmt.Errorf("server cannot generate response matching Accept header")

	default:
		return nil, fmt.Errorf("unexpected status code returned by server: %d", resp.StatusCode)
	}
}

// SyncStateWithResponse retrieves the desired state manifest and returns the HTTP response for header access
func (sbiClient *SbiHttpClient) SyncStateWithResponse(ctx context.Context, deviceClientId string, etag string, overrideOptions ...HTTPApiClientRequestEditorOptions) (desiredStates *sbi.UnsignedAppStateManifest, response *http.Response, err error) {
	// Prepare parameters
	params := &sbi.GetApiV1ClientsClientIdDeploymentsParams{
		Accept: pointers.Ptr("application/vnd.margo.manifest.v1+json"),
	}

	// Only set If-None-Match if etag is not empty
	if etag != "" && etag != `""` {
		params.IfNoneMatch = &etag
	}

	resp, err := sbiClient.client.GetApiV1ClientsClientIdDeployments(
		ctx,
		deviceClientId,
		params,
		overrideOptions...,
	)
	if err != nil {
		return nil, nil, err
	}

	// Check status code BEFORE parsing response
	// 304 Not Modified has no body, so don't try to parse it
	if resp.StatusCode == 304 {
		return nil, resp, nil
	}

	// Only parse response for status codes that have a body
	desiredStateResp, err := sbi.ParseGetApiV1ClientsClientIdDeploymentsResponse(resp)
	if err != nil {
		resp.Body.Close()
		return nil, nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Handle status codes according to OpenAPI spec
	switch resp.StatusCode {
	case 200:
		// OK - new data available
		if desiredStateResp.ApplicationvndMargoManifestV1JSON200 != nil {
			return desiredStateResp.ApplicationvndMargoManifestV1JSON200, resp, nil
		}
		resp.Body.Close()
		return nil, nil, fmt.Errorf("unexpected response structure for status 200")

	case 406:
		// Not Acceptable
		resp.Body.Close()
		return nil, nil, fmt.Errorf("server cannot generate response matching Accept header")

	default:
		resp.Body.Close()
		return nil, nil, fmt.Errorf("unexpected status code returned by server: %d", resp.StatusCode)
	}
}

func (sbiClient *SbiHttpClient) ReportDeploymentStatus(ctx context.Context, deviceID, appID string, overallAppStatus sbi.DeploymentStatusManifestStatusState, components []sbi.ComponentStatus, deploymentErr error) error {
	appUUID, err := uuid.Parse(appID)
	if err != nil {
		return err
	}

	// Build error struct only if there's an actual error
	var errorStruct *struct {
		Code    *string `json:"code,omitempty"`
		Message *string `json:"message,omitempty"`
	}

	if deploymentErr != nil {
		errorStruct = &struct {
			Code    *string `json:"code,omitempty"`
			Message *string `json:"message,omitempty"`
		}{
			Code:    pointers.Ptr("DEPLOYMENT_ERROR"),
			Message: pointers.Ptr(deploymentErr.Error()),
		}
	}

	deploymentStatus := sbi.DeploymentStatusManifest{
		ApiVersion:   "deployment.margo.org/v1alpha1",
		Kind:         sbi.DeploymentStatusManifestKindDeploymentStatusManifest,
		Components:   components,
		DeploymentId: appUUID.String(),
		Status: struct {
			Error *struct {
				Code    *string "json:\"code,omitempty\""
				Message *string "json:\"message,omitempty\""
			} "json:\"error,omitempty\""
			State sbi.DeploymentStatusManifestStatusState "json:\"state\""
		}{
			Error: errorStruct,
			State: overallAppStatus,
		},
	}

	resp, err := sbiClient.client.PostApiV1ClientsClientIdDeploymentsDeploymentIdStatus(ctx, deviceID, appUUID.String(), deploymentStatus)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// FetchDeploymentYAML with caching support and enhanced logging
func (sbiClient *SbiHttpClient) FetchDeploymentYAML(ctx context.Context, deviceClientId, deploymentId, digest string, overrideOptions ...HTTPApiClientRequestEditorOptions) (yamlContent []byte, err error) {
	// Check if we have this deployment cached
	cachedDigest, cacheErr := sbiClient.deploymentCache.GetLastDeploymentDigest(deploymentId)

	params := &sbi.GetApiV1ClientsClientIdDeploymentsDeploymentIdDigestParams{}

	// Add If-None-Match header if we have a cached version
	if cacheErr == nil && cachedDigest == digest {
		etag := fmt.Sprintf("\"%s\"", digest)
		params.IfNoneMatch = &etag
		fmt.Printf("INFO: [Cache] Sending If-None-Match for deployment %s: %s\n",
			deploymentId[:8], etag)
	}

	resp, err := sbiClient.client.GetApiV1ClientsClientIdDeploymentsDeploymentIdDigest(
		ctx,
		deviceClientId,
		deploymentId,
		digest,
		params,
		overrideOptions...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch deployment YAML: %w", err)
	}
	defer resp.Body.Close()

	// Handle 304 Not Modified
	if resp.StatusCode == http.StatusNotModified {
		fmt.Printf("INFO: [Cache HIT] Deployment %s not modified (304) - using cached version\n",
			deploymentId[:8])

		cachedData, err := sbiClient.deploymentCache.GetDeployment(deploymentId, digest)
		if err != nil {
			return nil, fmt.Errorf("304 received but cache read failed: %w", err)
		}
		return cachedData, nil
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("deployment fetch failed with status: %d", resp.StatusCode)
	}

	// Read YAML content
	yamlContent, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read deployment YAML: %w", err)
	}

	fmt.Printf("INFO: [Cache MISS] Downloaded deployment %s (%d bytes)\n",
		deploymentId[:8], len(yamlContent))

	// CRITICAL: Verify digest (Exact Bytes Rule)
	hash := sha256.Sum256(yamlContent)
	actualDigest := fmt.Sprintf("sha256:%x", hash)

	if actualDigest != digest {
		return nil, fmt.Errorf("deployment digest mismatch: expected %s, got %s",
			digest, actualDigest)
	}

	// Store in cache (digest verification happens inside cache.Store)
	if err := sbiClient.deploymentCache.StoreDeployment(deploymentId, digest, yamlContent); err != nil {
		fmt.Printf("WARNING: [Cache] Failed to cache deployment %s: %v\n", deploymentId[:8], err)
	} else {
		fmt.Printf("INFO: [Cache] Stored deployment %s (digest: %s...)\n",
			deploymentId[:8], digest[:16])
	}

	return yamlContent, nil
}

// DownloadBundle with caching support and enhanced logging
func (sbiClient *SbiHttpClient) DownloadBundle(ctx context.Context, deviceClientId, digest string, overrideOptions ...HTTPApiClientRequestEditorOptions) (bundleData []byte, err error) {
	// Check if we have this bundle cached
	cachedDigest, cacheErr := sbiClient.bundleCache.GetLastBundleDigest(deviceClientId)

	params := &sbi.GetApiV1ClientsClientIdBundlesDigestParams{}

	// Add If-None-Match header if we have a cached version
	if cacheErr == nil && cachedDigest == digest {
		etag := fmt.Sprintf("\"%s\"", digest)
		params.IfNoneMatch = &etag
		fmt.Printf("INFO: [Cache] Sending If-None-Match for bundle (device: %s, digest: %s...)\n",
			deviceClientId[:8], digest[:16])
	}

	resp, err := sbiClient.client.GetApiV1ClientsClientIdBundlesDigest(
		ctx,
		deviceClientId,
		digest,
		params,
		overrideOptions...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to download bundle: %w", err)
	}
	defer resp.Body.Close()

	// Handle 304 Not Modified
	if resp.StatusCode == http.StatusNotModified {
		fmt.Printf("INFO: [Cache HIT] Bundle not modified (304) - using cached version (device: %s)\n",
			deviceClientId[:8])

		cachedData, err := sbiClient.bundleCache.GetBundle(deviceClientId, digest)
		if err != nil {
			return nil, fmt.Errorf("304 received but cache read failed: %w", err)
		}

		fmt.Printf("INFO: [Cache] Retrieved bundle from cache (%d bytes)\n", len(cachedData))
		return cachedData, nil
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("bundle download failed with status: %d", resp.StatusCode)
	}

	// Read bundle data
	bundleData, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read bundle: %w", err)
	}

	fmt.Printf("INFO: [Cache MISS] Downloaded bundle for device %s (%d bytes)\n",
		deviceClientId[:8], len(bundleData))

	// Verify digest (Exact Bytes Rule)
	hash := sha256.Sum256(bundleData)
	actualDigest := fmt.Sprintf("sha256:%x", hash)

	if actualDigest != digest {
		return nil, fmt.Errorf("bundle digest mismatch: expected %s, got %s",
			digest, actualDigest)
	}

	// Store in cache (digest verification happens inside cache.Store)
	if err := sbiClient.bundleCache.StoreBundle(deviceClientId, digest, bundleData); err != nil {
		fmt.Printf("WARNING: [Cache] Failed to cache bundle for device %s: %v\n",
			deviceClientId[:8], err)
	} else {
		fmt.Printf("INFO: [Cache] Stored bundle for device %s (digest: %s...)\n",
			deviceClientId[:8], digest[:16])
	}

	return bundleData, nil
}
