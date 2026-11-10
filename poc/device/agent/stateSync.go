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

	"github.com/margo/sandbox/poc/device/agent/constant"
	"github.com/margo/sandbox/poc/device/agent/database"
	"github.com/margo/sandbox/poc/device/agent/types"
	wfm "github.com/margo/sandbox/poc/wfm/cli"
	"github.com/margo/sandbox/shared-lib/archive"
	"github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

const bytesPerMB = 1024 * 1024

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
	bundleThreshold           int
	bundleSizeThresholdBytes  float64
}

func NewStateSyncer(
	db *database.Database,
	client wfm.SBIAPIClientInterface,
	stateSeekingIntervalInSec uint16,
	bundleCfg types.BundleConfig,
	log *zap.SugaredLogger,
) *StateSyncer {
	return &StateSyncer{
		database:                  db,
		apiClient:                 client,
		log:                       log,
		stopChan:                  make(chan struct{}),
		stateSyncingIntervalInSec: stateSeekingIntervalInSec,
		bundleThreshold:           bundleCfg.DeploymentThreshold,
		bundleSizeThresholdBytes:  float64(bundleCfg.SizeThresholdMB) * bytesPerMB,
	}
}

func (ss *StateSyncer) logMemoryRequirementsByComponent(deploymentID string, components []any, source string) {
	for componentIndex, item := range components {
		component, ok := item.(map[string]any)
		if !ok {
			continue
		}
		componentName, _ := component["name"].(string)
		var resources map[string]any
		if props, ok := component["properties"].(map[string]any); ok {
			if dc, ok := props["deviceConstraints"].(map[string]any); ok {
				resources, _ = dc["capacityRequirements"].(map[string]any)
				if resources == nil {
					resources = dc
				}
			}
		}
		if resources == nil {
			if dc, ok := component["deviceConstraints"].(map[string]any); ok {
				resources, _ = dc["capacityRequirements"].(map[string]any)
				if resources == nil {
					resources = dc
				}
			}
		}
		memory, ok := resources["memory"].(map[string]any)
		if !ok {
			continue
		}
		ss.log.Infow("Parsed deployment memory requirement", "deploymentId", deploymentID, "source", source,
			"component", componentName, "componentIndex", componentIndex, "size", memory["size"],
			"bandwidthAllocation", memory["bandwidthAllocation"])
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

	// Calculate current ETag for If-None-Match header
	currentETag := ss.getLastSyncedETag()

	// Use the existing SyncState method with proper parameters
	var desiredStateManifest *sbi.UnsignedAppStateManifest
	var response *http.Response

	desiredStateManifest, response, err := ss.apiClient.SyncStateWithResponse(
		ctx,
		currentETag,
	)
	if err != nil {
		if pd, ok := sbi.AsProblemDetail(err); ok {
			if pd.Status == http.StatusNotModified {
				// 304 — expected cache hit, not an error
				ss.log.Infow("no change in desired and current states (304 Not Modified)",
					"status", pd.Status)
			} else {
				// 4xx/5xx — genuine WFM error
				ss.log.Errorw("WFM returned error response",
					"type", pd.Type,
					"status", pd.Status,
					"title", pd.Title,
					"detail", pd.Detail,
					"retryable", pd.IsRetryable(),
					"backoff", pd.BackoffStrategy,
				)
			}
		} else {
			ss.log.Errorw("Sync failed", "err", err.Error())
		}
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

	manifestVersion := uint64(desiredStateManifest.ManifestVersion)
	ss.detectRemovedDeployments(desiredStateManifest.Deployments, manifestVersion)

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
					manifestVersion,
				)
			} else {
				// Process deployments from bundle
				ss.processDeploymentsFromBundle(
					ctx,
					desiredStateManifest.Deployments,
					bundleYAMLs,
					manifestVersion,
				)
			}
		} else {
			// Fetch deployments individually
			ss.processDeploymentsIndividually(
				ctx,
				desiredStateManifest.Deployments,
				manifestVersion,
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
	manifestVersion uint64,
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

			if err := ss.database.SetAdoptedManifestVersion(
				current.DeploymentID,
				manifestVersion,
			); err != nil {
				ss.log.Warnw("Failed to update adopted manifest version on removal",
					"deploymentId", current.DeploymentID,
					"manifestVersion", manifestVersion,
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

	var yamlContent []byte

	yamlContent, err := ss.apiClient.FetchDeploymentYAML(
		ctx,
		deploymentRef.DeploymentId,
		deploymentRef.Digest,
	)
	if err != nil {
		if pd, ok := sbi.AsProblemDetail(err); ok {
			ss.log.Errorw("WFM returned problem detail fetching deployment YAML",
				"deploymentId", deploymentRef.DeploymentId,
				"type", pd.Type,
				"status", pd.Status,
				"detail", pd.Detail,
				"retryable", pd.IsRetryable(),
			)
			return nil, fmt.Errorf("WFM error [%d] %s: %w", pd.Status, pd.Title, err)
		}
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

	ss.logDeploymentProfileCPURequirements(
		deploymentRef.DeploymentId,
		deployment.Spec.DeploymentProfile.Components,
	)
	ss.logDeploymentProfileCacheRequirements(
		deploymentRef.DeploymentId,
		deployment.Spec.DeploymentProfile.Components,
	)
	ss.logDeploymentProfileMemoryRequirements(deploymentRef.DeploymentId, deployment.Spec.DeploymentProfile)

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

	// Download bundle
	var bundleData []byte

	bundleData, err := ss.apiClient.DownloadBundle(
		ctx,
		*bundleRef.Digest,
	)
	if err != nil {
		if pd, ok := sbi.AsProblemDetail(err); ok {
			ss.log.Errorw("WFM returned problem detail downloading bundle",
				"digest", *bundleRef.Digest,
				"type", pd.Type,
				"status", pd.Status,
				"detail", pd.Detail,
				"retryable", pd.IsRetryable(),
			)
			return nil, fmt.Errorf("WFM error [%d] %s: %w", pd.Status, pd.Title, err)
		}
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

	// Download the bundle when deployments exceed the threshold (default: 5)
	// to reduce request overhead
	if len(manifest.Deployments) > ss.bundleThreshold {
		ss.log.Infow("Using bundle download (deployment count above threshold)",
			"deploymentCount", len(manifest.Deployments),
			"threshold", ss.bundleThreshold)
		return true
	}

	// Download the bundle if it exceeds the size threshold (default: 50 MB).
	if manifest.Bundle.SizeBytes != nil &&
		float64(*manifest.Bundle.SizeBytes) > ss.bundleSizeThresholdBytes {
		ss.log.Infow("Using bundle download (bundle size above threshold)",
			"sizeBytes", *manifest.Bundle.SizeBytes,
			"thresholdBytes", ss.bundleSizeThresholdBytes)
		return true
	}

	// Default: both values are at or below their thresholds, so fetch
	// each deployment individually.
	ss.log.Infow("Using individual deployment fetch",
		"deploymentCount", len(manifest.Deployments),
		"threshold", ss.bundleThreshold)
	return false
}

// processDeploymentsIndividually fetches and stores each deployment individually
func (ss *StateSyncer) processDeploymentsIndividually(
	ctx context.Context,
	deploymentRefs []sbi.DeploymentManifestRef,
	manifestVersion uint64,
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
			ss.database.SetPhase(deploymentId, constant.PhaseFailed,
				fmt.Sprintf("failed to fetch deployment: %v", err))
			continue
		}

		// Store deployment
		ss.storeDeployment(
			deploymentRef.DeploymentId,
			deploymentRef,
			deploymentYAML,
			manifestVersion,
		)
	}
}

// processDeploymentsFromBundle processes deployments extracted from bundle

func (ss *StateSyncer) processDeploymentsFromBundle(
	_ context.Context,
	deploymentRefs []sbi.DeploymentManifestRef,
	bundleYAMLs map[string][]byte,
	manifestVersion uint64,
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
			ss.database.SetPhase(deploymentId, constant.PhaseFailed,
				"deployment YAML not found in bundle")
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
			ss.database.SetPhase(deploymentId, constant.PhaseFailed,
				"deployment digest verification failed")
			continue
		}

		// Parse YAML

		var yamlInterface interface{}
		if err := yaml.Unmarshal(yamlContent, &yamlInterface); err != nil {
			ss.log.Errorw("Failed to unmarshal YAML to interface",
				"deploymentId", deploymentId,
				"error", err)
			ss.database.SetPhase(deploymentId, constant.PhaseFailed,
				fmt.Sprintf("failed to parse YAML: %v", err))
			continue
		}

		// Convert YAML maps to JSON-compatible format
		jsonCompatible := convertYAMLToJSON(yamlInterface)
		ss.logDeploymentCPURequirementsFromRawYAML(deploymentId, jsonCompatible)
		ss.logDeploymentCacheRequirementsFromRawYAML(deploymentId, jsonCompatible)
		ss.logDeploymentMemoryRequirementsFromRawYAML(deploymentId, jsonCompatible)

		// Convert to JSON (which will be properly unmarshaled by UnmarshalJSON())
		jsonData, err := json.Marshal(jsonCompatible)
		if err != nil {
			ss.log.Errorw("Failed to marshal to JSON",
				"deploymentId", deploymentId,
				"error", err)
			ss.database.SetPhase(deploymentId, constant.PhaseFailed,
				fmt.Sprintf("failed to convert to JSON: %v", err))
			continue
		}

		// Unmarshal JSON to struct (calls custom UnmarshalJSON() for components)
		var deployment sbi.AppDeploymentManifest
		if err := json.Unmarshal(jsonData, &deployment); err != nil {
			ss.log.Errorw("Failed to unmarshal JSON to deployment",
				"deploymentId", deploymentId,
				"error", err)
			ss.database.SetPhase(deploymentId, constant.PhaseFailed,
				fmt.Sprintf("Failed to parse deployment: %v", err))
			continue
		}

		// Store deployment
		ss.logDeploymentProfileCacheRequirements(
			deploymentId,
			deployment.Spec.DeploymentProfile.Components,
		)
		ss.logDeploymentProfileMemoryRequirements(deploymentId, deployment.Spec.DeploymentProfile)
		ss.storeDeployment(deploymentId, deploymentRef, &deployment, manifestVersion)
	}
}

// storeDeployment stores a deployment in the database
func (ss *StateSyncer) storeDeployment(
	deploymentId string,
	deploymentRef sbi.DeploymentManifestRef,
	deploymentYAML *sbi.AppDeploymentManifest,
	manifestVersion uint64,
) {
	desiredState := database.AppDeploymentState{
		AppDeploymentManifest: *deploymentYAML,
		Status: sbi.DeploymentStatusManifest{
			DeploymentId: deploymentId,
			Status: struct {
				Error *struct {
					Code    *string `json:"code,omitempty"`
					Message *string `json:"message,omitempty"`
					Source  *string `json:"source,omitempty"`
				} `json:"error,omitempty"`
				State sbi.DeploymentStatusManifestStatusState `json:"state"`
			}{
				State: sbi.DeploymentStatusManifestStatusStatePending,
			},
		},
		AppId:       deploymentId,
		State:       constant.AppDeploymentStateStatePending,
		LastUpdated: time.Now(),
		Digest:      &deploymentRef.Digest,
		URL:         &deploymentRef.Url,
	}

	err := ss.database.SetDesiredState(deploymentId, desiredState)
	if err != nil {
		ss.log.Errorw("Failed to set desired state",
			"deploymentId", deploymentId,
			"error", err.Error())
		ss.database.SetPhase(deploymentId, constant.PhaseFailed,
			fmt.Sprintf("Failed to set desired state: %v", err))
		return
	}

	if err := ss.database.SetAdoptedManifestVersion(deploymentId, manifestVersion); err != nil {
		ss.log.Warnw("Failed to set adopted manifest version",
			"deploymentId", deploymentId,
			"manifestVersion", manifestVersion,
			"error", err)
	}

	ss.log.Infow("Set desired state for deployment",
		"deploymentId", deploymentId,
		"digest", deploymentRef.Digest,
		"adoptedManifestVersion", manifestVersion)
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

// TODO: these are helper function to log the rt related data model changes. they can be
// removed after the PR is approved
func (ss *StateSyncer) logDeploymentProfileCPURequirements(
	deploymentID string,
	components []sbi.ApplicationDeploymentProfileComponent,
) {
	for componentIndex, component := range components {
		componentName := component.Name
		dc := component.DeviceConstraints
		if dc == nil || dc.CapacityRequirements == nil || dc.CapacityRequirements.Cpu == nil {
			continue
		}

		for cpuIndex, cpu := range *dc.CapacityRequirements.Cpu {
			cpuName := ""
			if cpu.Name != nil && *cpu.Name != "" {
				cpuName = *cpu.Name
			} else {
				ss.log.Warnw("Deployment CPU requirement missing name",
					"deploymentId", deploymentID,
					"component", componentName,
					"cpuIndex", cpuIndex)
			}

			var cpuClass, cpuType any
			if cpu.Class != nil {
				cpuClass = *cpu.Class
			}
			if cpu.Type != nil {
				cpuType = *cpu.Type
			}

			ss.log.Infow("Parsed deployment CPU requirement",
				"deploymentId", deploymentID,
				"source", "typed profile",
				"component", componentName,
				"componentIndex", componentIndex,
				"name", cpuName,
				"cpuIndex", cpuIndex,
				"cores", cpu.Cores,
				"class", cpuClass,
				"type", cpuType)
		}
	}
}

// TODO: method used to log/debug newly added rt capabilities data model. Can be removed after
func (ss *StateSyncer) logDeploymentProfileCacheRequirements(
	deploymentID string,
	components []sbi.ApplicationDeploymentProfileComponent,
) {
	componentJSON, err := json.Marshal(components)
	if err != nil {
		ss.log.Errorw("Failed to marshal deployment profile components for cache logging",
			"deploymentId", deploymentID,
			"error", err)
		return
	}

	var rawComponents []any
	if err := json.Unmarshal(componentJSON, &rawComponents); err != nil {
		ss.log.Errorw("Failed to parse deployment profile components for cache logging",
			"deploymentId", deploymentID,
			"error", err)
		return
	}

	ss.logCacheRequirementsByComponent(deploymentID, rawComponents, "typed profile")
}

func (ss *StateSyncer) logDeploymentProfileMemoryRequirements(
	deploymentID string,
	profile sbi.AppDeploymentProfile,
) {
	if profile.DeviceConstraints != nil &&
		profile.DeviceConstraints.CapacityRequirements != nil &&
		profile.DeviceConstraints.CapacityRequirements.Memory != nil {
		memoryJSON, err := json.Marshal(profile.DeviceConstraints.CapacityRequirements.Memory)
		if err != nil {
			ss.log.Errorw("Failed to marshal deployment profile memory requirement", "deploymentId", deploymentID, "error", err)
		} else {
			ss.log.Infow("Parsed deployment memory requirement", "deploymentId", deploymentID, "source", "typed profile", "memory", string(memoryJSON))
		}
	}
	rawComponents, err := json.Marshal(profile.Components)
	if err == nil {
		var components []any
		if json.Unmarshal(rawComponents, &components) == nil {
			ss.logMemoryRequirementsByComponent(deploymentID, components, "typed profile")
		}
	}
}

func (ss *StateSyncer) logDeploymentCPURequirementsFromRawYAML(
	deploymentID string,
	rawManifest any,
) {
	manifestObj, ok := rawManifest.(map[string]any)
	if !ok {
		ss.log.Debugw("Raw deployment manifest is not an object",
			"deploymentId", deploymentID,
			"type", fmt.Sprintf("%T", rawManifest))
		return
	}

	specObj, ok := manifestObj["spec"].(map[string]any)
	if !ok {
		ss.log.Debugw("No spec found in raw deployment manifest",
			"deploymentId", deploymentID)
		return
	}

	profileObj, ok := specObj["deploymentProfile"].(map[string]any)
	if !ok {
		ss.log.Debugw("No deploymentProfile found in raw deployment manifest",
			"deploymentId", deploymentID)
		return
	}

	components, ok := profileObj["components"].([]any)
	if !ok {
		ss.log.Debugw("No deployment profile components found in raw deployment",
			"deploymentId", deploymentID)
		return
	}

	ss.logCPURequirementsByComponent(deploymentID, components, "raw YAML")
}

// TODO: method used to log/debug newly added rt capabilities data model. Can be removed after
func (ss *StateSyncer) logDeploymentCacheRequirementsFromRawYAML(
	deploymentID string,
	rawManifest any,
) {
	manifestObj, ok := rawManifest.(map[string]any)
	if !ok {
		ss.log.Debugw("Raw deployment manifest is not an object",
			"deploymentId", deploymentID,
			"type", fmt.Sprintf("%T", rawManifest))
		return
	}

	specObj, ok := manifestObj["spec"].(map[string]any)
	if !ok {
		ss.log.Debugw("No spec found in raw deployment manifest",
			"deploymentId", deploymentID)
		return
	}

	profileObj, ok := specObj["deploymentProfile"].(map[string]any)
	if !ok {
		ss.log.Debugw("No deploymentProfile found in raw deployment manifest",
			"deploymentId", deploymentID)
		return
	}

	components, ok := profileObj["components"].([]any)
	if !ok {
		ss.log.Debugw("No deployment profile components found in raw deployment",
			"deploymentId", deploymentID)
		return
	}

	ss.logCacheRequirementsByComponent(deploymentID, components, "raw YAML")
}

func (ss *StateSyncer) logDeploymentMemoryRequirementsFromRawYAML(deploymentID string, rawManifest any) {
	manifestObj, ok := rawManifest.(map[string]any)
	if !ok {
		return
	}
	specObj, ok := manifestObj["spec"].(map[string]any)
	if !ok {
		return
	}
	profileObj, ok := specObj["deploymentProfile"].(map[string]any)
	if !ok {
		return
	}
	var resources map[string]any
	if dc, ok := profileObj["deviceConstraints"].(map[string]any); ok {
		resources, _ = dc["capacityRequirements"].(map[string]any)
		if resources == nil {
			resources = dc
		}
	}
	if memoryObj, ok := resources["memory"].(map[string]any); ok {
		ss.log.Infow("Parsed deployment memory requirement", "deploymentId", deploymentID, "source", "raw YAML",
			"size", memoryObj["size"], "bandwidthAllocation", memoryObj["bandwidthAllocation"])
	}
	if components, ok := profileObj["components"].([]any); ok {
		ss.logMemoryRequirementsByComponent(deploymentID, components, "raw YAML")
	}
}

func (ss *StateSyncer) logCPURequirementsByComponent(
	deploymentID string,
	components []any,
	source string,
) {
	for componentIndex, componentItem := range components {
		component, ok := componentItem.(map[string]any)
		if !ok {
			continue
		}

		componentName, _ := component["name"].(string)

		var resources map[string]any
		if props, ok := component["properties"].(map[string]any); ok {
			if dc, ok := props["deviceConstraints"].(map[string]any); ok {
				if capReqs, ok := dc["capacityRequirements"].(map[string]any); ok {
					resources = capReqs
				} else {
					resources = dc
				}
			}
		}

		if resources == nil {
			if dc, ok := component["deviceConstraints"].(map[string]any); ok {
				if capReqs, ok := dc["capacityRequirements"].(map[string]any); ok {
					resources = capReqs
				} else {
					resources = dc
				}
			}
		}

		if resources == nil {
			continue
		}

		var cpuItems []any
		if items, ok := resources["cpu"].([]any); ok {
			cpuItems = items
		} else if item, ok := resources["cpu"].(map[string]any); ok {
			cpuItems = []any{item}
		}

		if len(cpuItems) == 0 {
			continue
		}

		for cpuIndex, cpuItem := range cpuItems {
			cpuObj, ok := cpuItem.(map[string]any)
			if !ok {
				continue
			}

			cpuName := ""
			rawName, hasCPUName := cpuObj["name"].(string)
			if hasCPUName && rawName != "" {
				cpuName = rawName
			} else {
				ss.log.Warnw("Deployment CPU requirement missing name",
					"deploymentId", deploymentID,
					"component", componentName,
					"cpuIndex", cpuIndex)
			}

			ss.log.Infow("Parsed deployment CPU requirement",
				"deploymentId", deploymentID,
				"source", source,
				"component", componentName,
				"componentIndex", componentIndex,
				"name", cpuName,
				"cpuIndex", cpuIndex,
				"cores", cpuObj["cores"],
				"class", cpuObj["class"],
				"type", cpuObj["type"])
		}
	}
}

// TODO: method used to log/debug newly added rt capabilities data model. Can be removed after
func (ss *StateSyncer) logCacheRequirementsByComponent(
	deploymentID string,
	components []any,
	source string,
) {
	for componentIndex, componentItem := range components {
		component, ok := componentItem.(map[string]any)
		if !ok {
			continue
		}

		componentName, _ := component["name"].(string)

		var resources map[string]any
		if props, ok := component["properties"].(map[string]any); ok {
			if dc, ok := props["deviceConstraints"].(map[string]any); ok {
				if capReqs, ok := dc["capacityRequirements"].(map[string]any); ok {
					resources = capReqs
				} else {
					resources = dc
				}
			}
		}

		if resources == nil {
			if dc, ok := component["deviceConstraints"].(map[string]any); ok {
				if capReqs, ok := dc["capacityRequirements"].(map[string]any); ok {
					resources = capReqs
				} else {
					resources = dc
				}
			}
		}

		if resources == nil {
			continue
		}

		var cacheItems []any
		if items, ok := resources["cache"].([]any); ok {
			cacheItems = items
		} else if item, ok := resources["cache"].(map[string]any); ok {
			cacheItems = []any{item}
		}

		if len(cacheItems) == 0 {
			continue
		}

		for cacheIndex, cacheItem := range cacheItems {
			cacheObj, ok := cacheItem.(map[string]any)
			if !ok {
				continue
			}

			ss.log.Infow("Parsed deployment cache requirement",
				"deploymentId", deploymentID,
				"source", source,
				"component", componentName,
				"componentIndex", componentIndex,
				"cacheIndex", cacheIndex,
				"level", cacheObj["level"],
				"allocation", cacheObj["allocation"],
				"size", cacheObj["size"])
		}
	}
}
