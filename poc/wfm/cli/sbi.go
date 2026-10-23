package wfm

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/margo/sandbox/shared-lib/cache"
	"github.com/margo/sandbox/shared-lib/pointers"
	"github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
)

type (
	HTTPApiClientRequestEditorOptions = sbi.RequestEditorFn
	HTTPApiClientOptions              = sbi.ClientOption
)

// SbiHttpClient implementation
type SbiHttpClient struct {
	url             string
	client          sbi.ClientInterface
	deviceId        string // This is not SPIFFE ID, this is capabilities.properties.id OR gatewayId
	options         []HTTPApiClientOptions
	bundleCache     *cache.BundleCache
	deploymentCache *cache.DeploymentCache
}

// Note: deviceId is capabilities.properties.id or gatewayId, not SpiffeId
func NewSbiHTTPClient(
	url string,
	deviceId string,
	options ...HTTPApiClientOptions,
) (*SbiHttpClient, error) {
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

func (sbiClient *SbiHttpClient) SyncState(
	ctx context.Context,
	etag string,
	overrideOptions ...HTTPApiClientRequestEditorOptions,
) (desiredStates *sbi.UnsignedAppStateManifest, err error) {
	// Prepare parameters
	params := &sbi.GetApiV1DeploymentsParams{
		Accept: pointers.Ptr("application/vnd.margo.manifest.v1+json"),
	}

	// Only set If-None-Match if etag is not empty
	if etag != "" && etag != `""` {
		params.IfNoneMatch = &etag
	}

	resp, err := sbiClient.client.GetApiV1Deployments(
		ctx,
		params,
		overrideOptions...,
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Parse response first
	desiredStateResp, err := sbi.ParseGetApiV1DeploymentsResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Handle status codes according to OpenAPI spec
	switch resp.StatusCode {
	case 200:
		if desiredStateResp.ApplicationvndMargoManifestV1JSON200 != nil {
			return desiredStateResp.ApplicationvndMargoManifestV1JSON200, nil
		}
		return nil, fmt.Errorf("SyncState: unexpected response structure for status 200")

	case 304:
		return nil, nil

	case 403:
		if desiredStateResp.ApplicationproblemJSON403 != nil {
			return nil, sbi.WrapProblemDetail(
				"SyncState",
				desiredStateResp.ApplicationproblemJSON403,
			)
		}
		return nil, fmt.Errorf("SyncState: not authorized: %d", resp.StatusCode)

	case 406:
		if desiredStateResp.ApplicationproblemJSON406 != nil {
			return nil, sbi.WrapProblemDetail(
				"SyncState",
				desiredStateResp.ApplicationproblemJSON406,
			)
		}
		return nil, fmt.Errorf("SyncState: server cannot generate response matching Accept header")

	default:
		if apiErr := sbi.ParseErrorResponseFromBytes(
			resp.StatusCode,
			resp.Header.Get("Content-Type"),
			desiredStateResp.Body,
		); apiErr != nil {
			return nil, fmt.Errorf("SyncState: %w", apiErr)
		}
		return nil, fmt.Errorf("SyncState: unexpected status code: %d", resp.StatusCode)
	}
}

// SyncStateWithResponse retrieves the desired state manifest and returns the HTTP response for
// header access
func (sbiClient *SbiHttpClient) SyncStateWithResponse(
	ctx context.Context,
	etag string,
	overrideOptions ...HTTPApiClientRequestEditorOptions,
) (desiredStates *sbi.UnsignedAppStateManifest, response *http.Response, err error) {
	// Prepare parameters
	params := &sbi.GetApiV1DeploymentsParams{
		Accept: pointers.Ptr("application/vnd.margo.manifest.v1+json"),
	}

	// Only set If-None-Match if etag is not empty
	if etag != "" && etag != `""` {
		params.IfNoneMatch = &etag
	}

	resp, err := sbiClient.client.GetApiV1Deployments(
		ctx,
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
	desiredStateResp, err := sbi.ParseGetApiV1DeploymentsResponse(resp)
	if err != nil {
		resp.Body.Close()
		return nil, nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Handle status codes according to OpenAPI spec
	switch resp.StatusCode {
	case 200:
		if desiredStateResp.ApplicationvndMargoManifestV1JSON200 != nil {
			return desiredStateResp.ApplicationvndMargoManifestV1JSON200, resp, nil
		}
		resp.Body.Close()
		return nil, nil, fmt.Errorf(
			"SyncStateWithResponse: unexpected response structure for status 200",
		)

	case 403:
		resp.Body.Close()
		if desiredStateResp.ApplicationproblemJSON403 != nil {
			return nil, nil, sbi.WrapProblemDetail(
				"SyncStateWithResponse",
				desiredStateResp.ApplicationproblemJSON403,
			)
		}
		return nil, nil, fmt.Errorf("SyncStateWithResponse: not authorized: %d", resp.StatusCode)

	case 406:
		resp.Body.Close()
		if desiredStateResp.ApplicationproblemJSON406 != nil {
			return nil, nil, sbi.WrapProblemDetail(
				"SyncStateWithResponse",
				desiredStateResp.ApplicationproblemJSON406,
			)
		}
		return nil, nil, fmt.Errorf(
			"SyncStateWithResponse: server cannot generate response matching Accept header",
		)

	default:
		resp.Body.Close()
		if apiErr := sbi.ParseErrorResponseFromBytes(
			resp.StatusCode,
			resp.Header.Get("Content-Type"),
			desiredStateResp.Body,
		); apiErr != nil {
			return nil, nil, fmt.Errorf("SyncStateWithResponse: %w", apiErr)
		}
		return nil, nil, fmt.Errorf(
			"SyncStateWithResponse: unexpected status code: %d",
			resp.StatusCode,
		)
	}
}

func (sbiClient *SbiHttpClient) ReportDeploymentStatus(
	ctx context.Context,
	appID string,
	adoptedManifestVersion uint64,
	overallAppStatus sbi.DeploymentStatusManifestStatusState,
	components []sbi.ComponentStatus,
	deploymentErr error,
) error {
	appUUID, err := uuid.Parse(appID)
	if err != nil {
		return err
	}

	// Build error struct only if there's an actual error
	var errorStruct *struct {
		Code    *string `json:"code,omitempty"`
		Message *string `json:"message,omitempty"`
		Source  *string `json:"source,omitempty"`
	}

	if deploymentErr != nil {
		errorStruct = &struct {
			Code    *string `json:"code,omitempty"`
			Message *string `json:"message,omitempty"`
			Source  *string `json:"source,omitempty"`
		}{
			Code:    pointers.Ptr("DEPLOYMENT_ERROR"),
			Message: pointers.Ptr(deploymentErr.Error()),
		}
	}

	deploymentStatus := sbi.DeploymentStatusManifest{
		Components:             components,
		DeploymentId:           appUUID.String(),
		AdoptedManifestVersion: sbi.ManifestVersion(adoptedManifestVersion),
		Status: struct {
			Error *struct {
				Code    *string "json:\"code,omitempty\""
				Message *string "json:\"message,omitempty\""
				Source  *string "json:\"source,omitempty\""
			} "json:\"error,omitempty\""
			State sbi.DeploymentStatusManifestStatusState "json:\"state\""
		}{
			Error: errorStruct,
			State: overallAppStatus,
		},
	}

	resp, err := sbiClient.client.PostApiV1DeploymentsDeploymentIdStatus(
		ctx,
		appUUID.String(),
		deploymentStatus,
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		return nil
	}

	parsedResp, err := sbi.ParsePostApiV1DeploymentsDeploymentIdStatusResponse(resp)
	if err != nil {
		return fmt.Errorf("ReportDeploymentStatus: failed to parse response: %w", err)
	}
	switch resp.StatusCode {
	case http.StatusBadRequest:
		if parsedResp.ApplicationproblemJSON400 != nil {
			return sbi.WrapProblemDetail(
				"ReportDeploymentStatus",
				parsedResp.ApplicationproblemJSON400,
			)
		}
	case http.StatusForbidden:
		if parsedResp.ApplicationproblemJSON403 != nil {
			return sbi.WrapProblemDetail(
				"ReportDeploymentStatus",
				parsedResp.ApplicationproblemJSON403,
			)
		}
	case http.StatusUnprocessableEntity:
		if parsedResp.ApplicationproblemJSON422 != nil {
			return sbi.WrapProblemDetail(
				"ReportDeploymentStatus",
				parsedResp.ApplicationproblemJSON422,
			)
		}
	}
	if apiErr := sbi.ParseErrorResponseFromBytes(
		resp.StatusCode,
		resp.Header.Get("Content-Type"),
		parsedResp.Body,
	); apiErr != nil {
		return fmt.Errorf("ReportDeploymentStatus: %w", apiErr)
	}
	return fmt.Errorf("ReportDeploymentStatus: unexpected status code: %d", resp.StatusCode)
}

// FetchDeploymentYAML with caching support and enhanced logging
func (sbiClient *SbiHttpClient) FetchDeploymentYAML(
	ctx context.Context,
	deploymentId, digest string,
	overrideOptions ...HTTPApiClientRequestEditorOptions,
) (yamlContent []byte, err error) {
	// Check if we have this deployment cached
	cachedDigest, cacheErr := sbiClient.deploymentCache.GetLastDeploymentDigest(deploymentId)

	params := &sbi.GetApiV1DeploymentsDeploymentIdDigestParams{}

	// Add If-None-Match header if we have a cached version
	if cacheErr == nil && cachedDigest == digest {
		etag := fmt.Sprintf("\"%s\"", digest)
		params.IfNoneMatch = &etag
		fmt.Printf("INFO: [Cache] Sending If-None-Match for deployment %s: %s\n",
			deploymentId[:8], etag)
	}

	resp, err := sbiClient.client.GetApiV1DeploymentsDeploymentIdDigest(
		ctx,
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
			deploymentId)

		cachedData, err := sbiClient.deploymentCache.GetDeployment(deploymentId, digest)
		if err != nil {
			return nil, fmt.Errorf("304 received but cache read failed: %w", err)
		}
		return cachedData, nil
	}

	if resp.StatusCode != 200 {
		parsedResp, parseErr := sbi.ParseGetApiV1DeploymentsDeploymentIdDigestResponse(resp)
		if parseErr == nil {
			if parsedResp.ApplicationproblemJSON403 != nil {
				return nil, sbi.WrapProblemDetail(
					"FetchDeploymentYAML",
					parsedResp.ApplicationproblemJSON403,
				)
			}
			if parsedResp.ApplicationproblemJSON404 != nil {
				return nil, sbi.WrapProblemDetail(
					"FetchDeploymentYAML",
					parsedResp.ApplicationproblemJSON404,
				)
			}
			if apiErr := sbi.ParseErrorResponseFromBytes(
				resp.StatusCode,
				resp.Header.Get("Content-Type"),
				parsedResp.Body,
			); apiErr != nil {
				return nil, fmt.Errorf("FetchDeploymentYAML: %w", apiErr)
			}
		}
		return nil, fmt.Errorf("FetchDeploymentYAML: unexpected status %d", resp.StatusCode)
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
	if err := sbiClient.deploymentCache.StoreDeployment(
		deploymentId,
		digest,
		yamlContent,
	); err != nil {
		fmt.Printf("WARNING: [Cache] Failed to cache deployment %s: %v\n", deploymentId[:8], err)
	} else {
		fmt.Printf("INFO: [Cache] Stored deployment %s (digest: %s...)\n",
			deploymentId[:8], digest[:16])
	}

	return yamlContent, nil
}

// DownloadBundle with caching support and enhanced logging
func (sbiClient *SbiHttpClient) DownloadBundle(
	ctx context.Context,
	digest string,
	overrideOptions ...HTTPApiClientRequestEditorOptions,
) (bundleData []byte, err error) {
	// Check if we have this bundle cached
	cachedDigest, cacheErr := sbiClient.bundleCache.GetLastBundleDigest(sbiClient.deviceId)

	params := &sbi.GetApiV1BundlesDigestParams{}

	// Add If-None-Match header if we have a cached version
	if cacheErr == nil && cachedDigest == digest {
		etag := fmt.Sprintf("\"%s\"", digest)
		params.IfNoneMatch = &etag
		fmt.Printf("INFO: [Cache] Sending If-None-Match for bundle (device: %s, digest: %s...)\n",
			sbiClient.deviceId, digest[:16])
	}

	resp, err := sbiClient.client.GetApiV1BundlesDigest(
		ctx,
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
		fmt.Printf(
			"INFO: [Cache HIT] Bundle not modified (304) - using cached version (device: %s)\n",
			sbiClient.deviceId,
		)

		cachedData, err := sbiClient.bundleCache.GetBundle(sbiClient.deviceId, digest)
		if err != nil {
			return nil, fmt.Errorf("304 received but cache read failed: %w", err)
		}

		fmt.Printf("INFO: [Cache] Retrieved bundle from cache (%d bytes)\n", len(cachedData))
		return cachedData, nil
	}

	if resp.StatusCode != 200 {
		parsedResp, parseErr := sbi.ParseGetApiV1BundlesDigestResponse(resp)
		if parseErr == nil {
			if parsedResp.ApplicationproblemJSON403 != nil {
				return nil, sbi.WrapProblemDetail(
					"DownloadBundle",
					parsedResp.ApplicationproblemJSON403,
				)
			}
			if parsedResp.ApplicationproblemJSON404 != nil {
				return nil, sbi.WrapProblemDetail(
					"DownloadBundle",
					parsedResp.ApplicationproblemJSON404,
				)
			}
			if apiErr := sbi.ParseErrorResponseFromBytes(
				resp.StatusCode,
				resp.Header.Get("Content-Type"),
				parsedResp.Body,
			); apiErr != nil {
				return nil, fmt.Errorf("DownloadBundle: %w", apiErr)
			}
		}
		return nil, fmt.Errorf("DownloadBundle: unexpected status %d", resp.StatusCode)
	}

	// Read bundle data
	bundleData, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read bundle: %w", err)
	}

	fmt.Printf("INFO: [Cache MISS] Downloaded bundle for device %s (%d bytes)\n",
		sbiClient.deviceId, len(bundleData))

	// Verify digest (Exact Bytes Rule)
	hash := sha256.Sum256(bundleData)
	actualDigest := fmt.Sprintf("sha256:%x", hash)

	if actualDigest != digest {
		return nil, fmt.Errorf("bundle digest mismatch: expected %s, got %s",
			digest, actualDigest)
	}

	// Store in cache (digest verification happens inside cache.Store)
	if err := sbiClient.bundleCache.StoreBundle(
		sbiClient.deviceId,
		digest,
		bundleData,
	); err != nil {
		fmt.Printf("WARNING: [Cache] Failed to cache bundle for device %s: %v\n",
			sbiClient.deviceId, err)
	} else {
		fmt.Printf("INFO: [Cache] Stored bundle for device %s (digest: %s...)\n",
			sbiClient.deviceId, digest[:16])
	}

	return bundleData, nil
}

func (sbiClient *SbiHttpClient) ReportCapabilities(
	ctx context.Context,
	deviceId string,
	capabilities sbi.DeviceCapabilitiesManifest,
	overrideOptions ...HTTPApiClientRequestEditorOptions,
) error {
	resp, err := sbiClient.client.PutApiV1CapabilitiesDeviceId(
		ctx,
		deviceId,
		capabilities,
		overrideOptions...,
	)
	if err != nil {
		return fmt.Errorf("failed to report capabilities: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		return nil
	}

	parsedResp, err := sbi.ParsePutApiV1CapabilitiesDeviceIdResponse(resp)
	if err != nil {
		return fmt.Errorf("ReportCapabilities: failed to parse response: %w", err)
	}
	switch resp.StatusCode {
	case http.StatusBadRequest:
		if parsedResp.ApplicationproblemJSON400 != nil {
			return sbi.WrapProblemDetail("ReportCapabilities", parsedResp.ApplicationproblemJSON400)
		}
	case http.StatusForbidden:
		if parsedResp.ApplicationproblemJSON403 != nil {
			return sbi.WrapProblemDetail("ReportCapabilities", parsedResp.ApplicationproblemJSON403)
		}
	case http.StatusNotFound:
		if parsedResp.ApplicationproblemJSON404 != nil {
			return sbi.WrapProblemDetail("ReportCapabilities", parsedResp.ApplicationproblemJSON404)
		}
	case http.StatusUnprocessableEntity:
		if parsedResp.ApplicationproblemJSON422 != nil {
			return sbi.WrapProblemDetail("ReportCapabilities", parsedResp.ApplicationproblemJSON422)
		}
	}
	if apiErr := sbi.ParseErrorResponseFromBytes(
		resp.StatusCode,
		resp.Header.Get("Content-Type"),
		parsedResp.Body,
	); apiErr != nil {
		return fmt.Errorf("ReportCapabilities: %w", apiErr)
	}
	return fmt.Errorf("ReportCapabilities: unexpected status code: %d", resp.StatusCode)
}
