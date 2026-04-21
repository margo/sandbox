// sync/state_syncer.go
package main

import (
	"context"
	"crypto"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/margo/sandbox/poc/device/agent/database"
	wfm "github.com/margo/sandbox/poc/wfm/cli"
	"github.com/margo/sandbox/shared-lib/archive"
	"github.com/margo/sandbox/shared-lib/http/auth"
	"github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

type StateSyncerIfc interface {
	Start()
	Stop()
}

type StateSyncer struct {
	database                  *database.Database
	apiClient                 wfm.SBIAPIClientInterface
	requestSigner             crypto.Signer
	deviceID                  string
	log                       *zap.SugaredLogger
	stopChan                  chan struct{}
	stateSyncingIntervalInSec uint16
}

func NewStateSyncer(
	db *database.Database,
	client wfm.SBIAPIClientInterface,
	deviceID string,
	stateSeekingIntervalInSec uint16,
	log *zap.SugaredLogger) *StateSyncer {
	return &StateSyncer{
		database:                  db,
		apiClient:                 client,
		deviceID:                  deviceID,
		log:                       log,
		stopChan:                  make(chan struct{}),
		stateSyncingIntervalInSec: stateSeekingIntervalInSec,
	}
}

func (ss *StateSyncer) Start() {
	go ss.syncLoop()
}

func (ss *StateSyncer) Stop() {
	close(ss.stopChan)
}

func (ss *StateSyncer) syncLoop() {
	ticker := time.NewTicker(
		time.Duration(ss.stateSyncingIntervalInSec) * time.Second,
	)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ss.performSync()
		case <-ss.stopChan:
			return
		}
	}
}

func (ss *StateSyncer) performSync() {
	ss.log.Debugf("Performing sync....")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get device settings
	device, err := ss.database.GetDeviceSettings()
	if err != nil {
		ss.log.Errorw(
			"Sync failed",
			"err",
			err.Error(),
			"msg",
			"failed to fetch device settings",
		)
		return
	}

	// Calculate current ETag for If-None-Match header
	currentETag := ss.getLastSyncedETag()

	// Use the existing SyncState method with proper parameters
	var desiredStateManifest *sbi.UnsignedAppStateManifest
	var response *http.Response

	if device.AuthEnabled {
		desiredStateManifest, response, err = ss.apiClient.SyncStateWithResponse(
			ctx,
			device.DeviceClientId,
			currentETag,
			auth.WithOAuth(
				ctx,
				device.OAuthClientId,
				device.OAuthClientSecret,
				device.OAuthTokenEndpointUrl,
			),
		)
	} else {
		desiredStateManifest, response, err = ss.apiClient.SyncStateWithResponse(
			ctx,
			device.DeviceClientId,
			currentETag,
		)
	}

	if err != nil {
		ss.log.Errorw(
			"Sync failed",
			"err",
			err.Error(),
			"deviceId",
			device.DeviceClientId,
		)
		return
	}

	// Handle 304 Not Modified
	if response != nil && response.StatusCode == http.StatusNotModified {
		ss.log.Infow(
			"Sync completed",
			"msg",
			"No change in desired and current states (304 Not Modified)",
		)
		return
	}

	if desiredStateManifest == nil {
		ss.log.Infow(
			"Sync completed",
			"msg",
			"No change in desired and current states",
		)
		return
	}

	ss.log.Infow("Received manifest details",
		"version", desiredStateManifest.ManifestVersion,
		"deployments", len(desiredStateManifest.Deployments),
		"bundleDigest", func() string {
			if desiredStateManifest.Bundle != nil &&
				desiredStateManifest.Bundle.Digest != nil {
				return *desiredStateManifest.Bundle.Digest
			}
			return "none"
		}())

	// Security and Version Checks according to specification
	if err := ss.validateManifest(desiredStateManifest); err != nil {
		ss.log.Errorw("Manifest validation failed", "error", err)
		return
	}

	// Process deployments from the manifest
	ss.log.Debugf("Setting desired states....")

	ss.detectRemovedDeployments(desiredStateManifest.Deployments)

	if len(desiredStateManifest.Deployments) > 0 {
		// Decide: bundle download vs individual fetch
		if ss.shouldDownloadBundle(desiredStateManifest) {
			// Download and extract bundle
			bundleYAMLs, err := ss.downloadAndExtractBundle(
				ctx,
				desiredStateManifest.Bundle,
			)
			if err != nil {
				ss.log.Errorw(
					"Failed to download bundle, falling back to individual fetch",
					"error",
					err,
				)
				// Fall back to individual fetch
				ss.processDeploymentsIndividually(
					ctx,
					desiredStateManifest.Deployments,
				)
			} else {
				// Process deployments from bundle
				ss.processDeploymentsFromBundle(
					ctx,
					desiredStateManifest.Deployments,
					bundleYAMLs,
				)
			}
		} else {
			// Fetch deployments individually
			ss.processDeploymentsIndividually(
				ctx,
				desiredStateManifest.Deployments,
			)
		}
	}

	// Store the new manifest metadata (including ETag from response)
	if err := ss.persistManifestMetadata(
		desiredStateManifest,
		response,
	); err != nil {
		ss.log.Errorw("Failed to persist manifest metadata", "error", err)
	}

	deploymentCount := len(desiredStateManifest.Deployments)
	ss.log.Debugw("Sync completed", "desiredStates", deploymentCount)
}

func (ss *StateSyncer) detectRemovedDeployments(
	desiredDeployments []sbi.DeploymentManifestRef,
) {
	currentDeployments := ss.database.ListDeployments()

	desiredIDs := make(map[string]bool)
	for _, dep := range desiredDeployments {
		desiredIDs[dep.DeploymentId] = true
	}

	for _, current := range currentDeployments {
		if current.DesiredState == nil {
			continue
		}

		if !desiredIDs[current.DeploymentID] {
			ss.log.Infow("Deployment removed from server, marking for removal",
				"deploymentId", current.DeploymentID,
				"name", current.DesiredState.Metadata.Name)

			removingState := *current.DesiredState
			removingState.Status.Status.State = sbi.DeploymentStatusManifestStatusStateRemoving

			if err := ss.database.SetDesiredState(
				current.DeploymentID,
				removingState,
			); err != nil {
				ss.log.Errorw("Failed to mark deployment for removal",
					"deploymentId", current.DeploymentID,
					"error", err)
			}
		}
	}
}

// validateManifest performs security and version checks according to specification
func (ss *StateSyncer) validateManifest(
	manifest *sbi.UnsignedAppStateManifest,
) error {
	if manifest.ManifestVersion == 0 {
		return fmt.Errorf("manifest version is required")
	}

	// CAST: float32 to uint64 for comparison
	newVersionInt := uint64(manifest.ManifestVersion)
	currentVersionInt, _ := ss.database.GetLastSyncedManifestVersion()

	// If we have a previous version, ensure new version is not less than current
	// Allow equal versions for unchanged manifests (especially empty ones)
	if currentVersionInt > 0 && newVersionInt < currentVersionInt {
		return fmt.Errorf(
			"potential rollback attack: new version %d < current version %d",
			newVersionInt,
			currentVersionInt,
		)
	}

	// Log when receiving same version (normal for unchanged manifests)
	if currentVersionInt > 0 && newVersionInt == currentVersionInt {
		ss.log.Debugw("Received manifest with same version",
			"version", newVersionInt,
			"deployments", len(manifest.Deployments))
	}

	return nil
}

// getLastSyncedETag retrieves the ETag from the last successful sync
func (ss *StateSyncer) getLastSyncedETag() string {
	etag, err := ss.database.GetLastSyncedETag()
	if err != nil {
		ss.log.Debugw("No previous ETag found", "error", err)
		return ""
	}
	return etag
}

// persistManifestMetadata stores manifest metadata according to specification
func (ss *StateSyncer) persistManifestMetadata(
	manifest *sbi.UnsignedAppStateManifest,
	response *http.Response,
) error {
	// Store manifest version for rollback protection

	manifestVersionInt := uint64(manifest.ManifestVersion)
	if manifestVersionInt != 0 {
		if err := ss.database.SetLastSyncedManifestVersion(
			manifestVersionInt,
		); err != nil {
			return fmt.Errorf("failed to store manifest version: %w", err)
		}
	}

	// SPEC-COMPLIANT: Extract ETag from HTTP response header
	var etag string
	if response != nil {
		etag = response.Header.Get("ETag")
		ss.log.Debugw("Extracted ETag from response header", "etag", etag)
	}

	// Fallback: Construct ETag if not in response (shouldn't happen with compliant server)
	if etag == "" {
		if manifest.Bundle != nil && manifest.Bundle.Digest != nil {
			// Bundle with deployments: Use bundle digest
			etag = fmt.Sprintf("\"%s\"", *manifest.Bundle.Digest)

			// Store bundle digest
			if err := ss.database.SetLastSyncedBundleDigest(
				*manifest.Bundle.Digest,
			); err != nil {
				return fmt.Errorf("failed to store bundle digest: %w", err)
			}
		} else {
			// Empty bundle: Compute digest of manifest JSON
			manifestJSON, err := json.Marshal(manifest)
			if err != nil {
				return fmt.Errorf(
					"failed to marshal manifest for digest: %w",
					err,
				)
			}
			hash := sha256.Sum256(manifestJSON)
			etag = fmt.Sprintf("\"sha256:%x\"", hash)
		}
		ss.log.Warnw(
			"ETag not in response header, computed fallback",
			"etag",
			etag,
		)
	}

	// Store ETag for HTTP caching (enables 304 Not Modified responses)
	if err := ss.database.SetLastSyncedETag(etag); err != nil {
		return fmt.Errorf("failed to store ETag: %w", err)
	}

	ss.log.Debugw("Stored manifest metadata",
		"version", manifestVersionInt,
		"etag", etag,
		"hasBundle", manifest.Bundle != nil,
		"deployments", len(manifest.Deployments))

	return nil
}

func (ss *StateSyncer) fetchDeploymentYAML(
	ctx context.Context,
	deploymentRef sbi.DeploymentManifestRef,
) (*sbi.AppDeploymentManifest, error) {
	ss.log.Infow("Fetching deployment YAML",
		"deploymentId", deploymentRef.DeploymentId,
		"digest", deploymentRef.Digest)

	device, err := ss.database.GetDeviceSettings()
	if err != nil {
		return nil, fmt.Errorf("failed to get device settings: %w", err)
	}

	var yamlContent []byte

	if device.AuthEnabled {
		yamlContent, err = ss.apiClient.FetchDeploymentYAML(
			ctx,
			device.DeviceClientId,
			deploymentRef.DeploymentId,
			deploymentRef.Digest,
			auth.WithOAuth(
				ctx,
				device.OAuthClientId,
				device.OAuthClientSecret,
				device.OAuthTokenEndpointUrl,
			),
		)
	} else {
		yamlContent, err = ss.apiClient.FetchDeploymentYAML(
			ctx,
			device.DeviceClientId,
			deploymentRef.DeploymentId,
			deploymentRef.Digest,
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to fetch deployment: %w", err)
	}

	// Parse YAML:  YAML-to-JSON-to-Struct conversion
	var yamlInterface interface{}
	if err := yaml.Unmarshal(yamlContent, &yamlInterface); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	// Convert YAML maps to JSON-compatible format
	jsonCompatible := convertYAMLToJSON(yamlInterface)

	jsonData, err := json.Marshal(jsonCompatible)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to JSON: %w", err)
	}

	var deployment sbi.AppDeploymentManifest
	if err := json.Unmarshal(jsonData, &deployment); err != nil {
		return nil, fmt.Errorf("failed to parse deployment: %w", err)
	}

	ss.log.Infow("Successfully fetched and verified deployment",
		"deploymentId", deploymentRef.DeploymentId)

	return &deployment, nil
}

// downloadAndExtractBundle downloads the bundle and extracts deployment YAMLs
func (ss *StateSyncer) downloadAndExtractBundle(
	ctx context.Context,
	bundleRef *sbi.DeploymentBundleRef,
) (map[string][]byte, error) {
	if bundleRef == nil || bundleRef.Digest == nil {
		return nil, fmt.Errorf("invalid bundle reference")
	}

	ss.log.Infow("Downloading bundle", "digest", *bundleRef.Digest)

	device, err := ss.database.GetDeviceSettings()
	if err != nil {
		return nil, fmt.Errorf("failed to get device settings: %w", err)
	}

	// Download bundle
	var bundleData []byte
	if device.AuthEnabled {
		bundleData, err = ss.apiClient.DownloadBundle(
			ctx,
			device.DeviceClientId,
			*bundleRef.Digest,
			auth.WithOAuth(
				ctx,
				device.OAuthClientId,
				device.OAuthClientSecret,
				device.OAuthTokenEndpointUrl,
			),
		)
	} else {
		bundleData, err = ss.apiClient.DownloadBundle(
			ctx,
			device.DeviceClientId,
			*bundleRef.Digest,
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to download bundle: %w", err)
	}

	ss.log.Infow("Bundle downloaded successfully",
		"digest", *bundleRef.Digest,
		"sizeBytes", len(bundleData))

	// Use generic extractor from shared-lib
	extractor := archive.NewExtractor(bundleData)

	// Verify bundle digest
	if err := extractor.VerifyBundleDigest(*bundleRef.Digest); err != nil {
		return nil, fmt.Errorf("bundle digest verification failed: %w", err)
	}

	// Extract deployments
	deploymentYAMLs, err := extractor.Extract()
	if err != nil {
		return nil, fmt.Errorf("failed to extract bundle: %w", err)
	}

	ss.log.Infow("Extracted deployments from bundle",
		"count", len(deploymentYAMLs))

	return deploymentYAMLs, nil
}

// shouldDownloadBundle determines if we should download the bundle or individual deployments
func (ss *StateSyncer) shouldDownloadBundle(
	manifest *sbi.UnsignedAppStateManifest,
) bool {
	// If no bundle available, must fetch individually
	if manifest.Bundle == nil || manifest.Bundle.Digest == nil {
		return false
	}

	// Heuristic: If more than 2 deployments, use bundle for efficiency
	if len(manifest.Deployments) > 2 {
		ss.log.Infow("Using bundle download (many deployments)",
			"deploymentCount", len(manifest.Deployments))
		return true
	}

	// Heuristic: If bundle size is reasonable (< 50MB), use bundle
	if manifest.Bundle.SizeBytes != nil &&
		*manifest.Bundle.SizeBytes < 50*1024*1024 {
		ss.log.Infow("Using bundle download (reasonable size)",
			"sizeBytes", *manifest.Bundle.SizeBytes)
		return true
	}

	// Default: fetch individually for small number of deployments
	ss.log.Infow("Using individual deployment fetch",
		"deploymentCount", len(manifest.Deployments))
	return false
}

// processDeploymentsIndividually fetches and stores each deployment individually
func (ss *StateSyncer) processDeploymentsIndividually(
	ctx context.Context,
	deploymentRefs []sbi.DeploymentManifestRef,
) {
	for _, deploymentRef := range deploymentRefs {
		if deploymentRef.DeploymentId == "" {
			ss.log.Warnw("Skipping deployment with empty DeploymentId")
			continue
		}

		deploymentId := deploymentRef.DeploymentId

		// Fetch the actual deployment YAML
		deploymentYAML, err := ss.fetchDeploymentYAML(ctx, deploymentRef)
		if err != nil {
			ss.log.Errorw("Failed to fetch deployment YAML",
				"deploymentId", deploymentId,
				"error", err)
			ss.database.SetPhase(deploymentId, "FAILED",
				fmt.Sprintf("Failed to fetch deployment: %v", err))
			continue
		}

		// Store deployment
		ss.storeDeployment(deploymentId, deploymentRef, deploymentYAML)
	}
}

// processDeploymentsFromBundle processes deployments extracted from bundle

func (ss *StateSyncer) processDeploymentsFromBundle(
	_ context.Context,
	deploymentRefs []sbi.DeploymentManifestRef,
	bundleYAMLs map[string][]byte,
) {
	for _, deploymentRef := range deploymentRefs {
		if deploymentRef.DeploymentId == "" {
			ss.log.Warnw("Skipping deployment with empty DeploymentId")
			continue
		}

		deploymentId := deploymentRef.DeploymentId

		// Find YAML in bundle (filename is typically deploymentId.yaml)
		yamlFilename := fmt.Sprintf("%s.yaml", deploymentId)
		yamlContent, found := bundleYAMLs[yamlFilename]
		if !found {
			ss.log.Errorw("Deployment YAML not found in bundle",
				"deploymentId", deploymentId,
				"expectedFilename", yamlFilename)
			ss.database.SetPhase(deploymentId, "FAILED",
				"Deployment YAML not found in bundle")
			continue
		}

		// Verify digest
		hash := sha256.Sum256(yamlContent)
		actualDigest := fmt.Sprintf("sha256:%x", hash)
		if actualDigest != deploymentRef.Digest {
			ss.log.Errorw("Deployment digest mismatch",
				"deploymentId", deploymentId,
				"expected", deploymentRef.Digest,
				"actual", actualDigest)
			ss.database.SetPhase(deploymentId, "FAILED",
				"Deployment digest verification failed")
			continue
		}

		// Parse YAML

		var yamlInterface interface{}
		if err := yaml.Unmarshal(yamlContent, &yamlInterface); err != nil {
			ss.log.Errorw("Failed to unmarshal YAML to interface",
				"deploymentId", deploymentId,
				"error", err)
			ss.database.SetPhase(deploymentId, "FAILED",
				fmt.Sprintf("Failed to parse YAML: %v", err))
			continue
		}

		// Convert YAML maps to JSON-compatible format
		jsonCompatible := convertYAMLToJSON(yamlInterface)

		// Convert to JSON (which will be properly unmarshaled by UnmarshalJSON())
		jsonData, err := json.Marshal(jsonCompatible)
		if err != nil {
			ss.log.Errorw("Failed to marshal to JSON",
				"deploymentId", deploymentId,
				"error", err)
			ss.database.SetPhase(deploymentId, "FAILED",
				fmt.Sprintf("Failed to convert to JSON: %v", err))
			continue
		}

		// Unmarshal JSON to struct (calls custom UnmarshalJSON() for components)
		var deployment sbi.AppDeploymentManifest
		if err := json.Unmarshal(jsonData, &deployment); err != nil {
			ss.log.Errorw("Failed to unmarshal JSON to deployment",
				"deploymentId", deploymentId,
				"error", err)
			ss.database.SetPhase(deploymentId, "FAILED",
				fmt.Sprintf("Failed to parse deployment: %v", err))
			continue
		}

		// Store deployment
		ss.storeDeployment(deploymentId, deploymentRef, &deployment)
	}
}

// storeDeployment stores a deployment in the database
func (ss *StateSyncer) storeDeployment(
	deploymentId string,
	deploymentRef sbi.DeploymentManifestRef,
	deploymentYAML *sbi.AppDeploymentManifest,
) {
	desiredState := database.AppDeploymentState{
		AppDeploymentManifest: *deploymentYAML,
		Status: sbi.DeploymentStatusManifest{
			ApiVersion:   "deployment.margo.org/v1alpha1",
			Kind:         sbi.DeploymentStatusManifestKindDeploymentStatusManifest,
			DeploymentId: deploymentId,
			Status: struct {
				Error *struct {
					Code    *string `json:"code,omitempty"`
					Message *string `json:"message,omitempty"`
				} `json:"error,omitempty"`
				State sbi.DeploymentStatusManifestStatusState `json:"state"`
			}{
				State: sbi.DeploymentStatusManifestStatusStatePending,
			},
		},
		AppId:       deploymentId,
		State:       "PENDING",
		LastUpdated: time.Now(),
		Digest:      &deploymentRef.Digest,
		URL:         &deploymentRef.Url,
	}

	err := ss.database.SetDesiredState(deploymentId, desiredState)
	if err != nil {
		ss.log.Errorw("Failed to set desired state",
			"deploymentId", deploymentId,
			"error", err.Error())
		ss.database.SetPhase(deploymentId, "FAILED",
			fmt.Sprintf("Failed to set desired state: %v", err))
		return
	}

	ss.log.Infow("Set desired state for deployment",
		"deploymentId", deploymentId,
		"digest", deploymentRef.Digest)
}

// convertYAMLToJSON converts YAML-style maps (interface{} keys) to JSON-compatible maps (string
// keys)
func convertYAMLToJSON(i interface{}) interface{} {
	switch x := i.(type) {
	case map[interface{}]interface{}:
		m2 := map[string]interface{}{}
		for k, v := range x {
			m2[fmt.Sprintf("%v", k)] = convertYAMLToJSON(v)
		}
		return m2
	case []interface{}:
		for i, v := range x {
			x[i] = convertYAMLToJSON(v)
		}
	}
	return i
}
