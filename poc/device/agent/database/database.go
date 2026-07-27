package database

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/margo/sandbox/poc/device/agent/types"
	"github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
)

type AppDeploymentState struct {
	sbi.AppDeploymentManifest
	Status sbi.DeploymentStatusManifest

	// Added these fields for sync state management
	AppId       string    `json:"appId"`
	State       string    `json:"state"`
	LastUpdated time.Time `json:"lastUpdated"`
	Digest      *string   `json:"digest,omitempty"`
	URL         *string   `json:"url,omitempty"`
}

type DeploymentRecord struct {
	AppID               string
	DeploymentID        string
	Digest              string
	Path                string
	URL                 string
	DesiredState        *AppDeploymentState
	CurrentState        *AppDeploymentState
	ComponentViseStatus map[string]sbi.ComponentStatus
	Phase               string // "deploying", "running", "failed", "removing", "removed"
	Message             string
	LastUpdated         time.Time
}

type DeploymentBundleRecord struct {
	DeviceClientId string
	Manifest       sbi.UnsignedAppStateManifest
	ArchivePath    string
	UpdatedAt      time.Time
}

type DeploymentRecordChangeType string

const (
	DeploymentChangeTypeRecordAdded           DeploymentRecordChangeType = "RECORD-ADDED"
	DeploymentChangeTypeRecordDeleted         DeploymentRecordChangeType = "RECORD-DELETED"
	DeploymentChangeTypeComponentPhaseChanged DeploymentRecordChangeType = "COMPONENT-PHASE-CHANGED"
	DeploymentChangeTypeDesiredStateAdded     DeploymentRecordChangeType = "DESIRED-STATE-ADDED"
	DeploymentChangeTypeCurrentStateAdded     DeploymentRecordChangeType = "CURRENT-STATE-ADDED"
)

type DeviceSettingsRecord struct {
	DeviceClientId     string                   `json:"deviceClientId"`
	DeviceRootIdentity types.DeviceRootIdentity `json:"deviceRootIdentity"`
	State              types.DeviceOnboardState `json:"state"`
	AuthEnabled        bool                     `json:"authEnabled"`
	// OAuthClientId The client ID for OAuth 2.0 authentication.
	OAuthClientId string `json:"clientId"`
	// OAuthClientSecret The client secret for OAuth 2.0 authentication.
	OAuthClientSecret string `json:"clientSecret"`
	// OAuthTokenEndpointUrl The URL for the OAuth 2.0 token endpoint.
	OAuthTokenEndpointUrl string `json:"tokenEndpointUrl"`
	// the applications that the device can deploy
	CanDeployHelm    bool
	CanDeployCompose bool

	// Added these new fields for sync state management
	LastSyncedETag            string `json:"lastSyncedETag"`
	LastSyncedManifestVersion uint64 `json:"lastSyncedManifestVersion"`
	LastSyncedBundleDigest    string `json:"lastSyncedBundleDigest"`
}

type DatabaseIfc interface {
	// if your database engine already has persistence, then just keep the implementation empty
	// we added an in-memory database implementation for this margo poc, hence needed this one
	TriggerDataPersist()
	Subscribe(callback func(string, *DeploymentRecord, DeploymentRecordChangeType))
	SetDesiredState(deploymentId string, state AppDeploymentState) error
	SetCurrentState(deploymentId string, state AppDeploymentState)
	SetPhase(deploymentId, phase, message string)
	SetComponentStatus(deploymentId, componentName string, status sbi.ComponentStatus)
	GetDeployment(deploymentId string) (*DeploymentRecord, error)
	ListDeployments() []*DeploymentRecord
	RemoveDeployment(deploymentId string)
	NeedsReconciliation(deploymentId string) bool
	GetDeviceSettings() (*DeviceSettingsRecord, error)
	SetDeviceSettings(settings DeviceSettingsRecord) error
	IsDeviceOnboarded() (*DeviceSettingsRecord, bool, error)

	GetLastSyncedETag() (string, error)
	SetLastSyncedETag(etag string) error
	GetLastSyncedManifestVersion() (uint64, error)
	SetLastSyncedManifestVersion(version uint64) error
	GetLastSyncedBundleDigest() (string, error)
	SetLastSyncedBundleDigest(digest string) error
}

type Database struct {
	deviceSettings *DeviceSettingsRecord
	deployments    map[string]*DeploymentRecord
	subscribers    []func(string, *DeploymentRecord, DeploymentRecordChangeType) // appID, record
	mu             sync.RWMutex
	subscriberMu   sync.RWMutex

	// for persistence
	dataDir     string
	persistChan chan struct{}
	stopPersist chan struct{}
}

// ETag management for efficient polling
func (db *Database) GetLastSyncedETag() (string, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.deviceSettings.LastSyncedETag == "" {
		return "", fmt.Errorf("no previous ETag found")
	}
	return db.deviceSettings.LastSyncedETag, nil
}

func (db *Database) SetLastSyncedETag(etag string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.deviceSettings.LastSyncedETag = etag
	db.TriggerDataPersist()
	return nil
}

// Manifest version management for rollback protection
func (db *Database) GetLastSyncedManifestVersion() (uint64, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.deviceSettings.LastSyncedManifestVersion == 0 {
		return 0, fmt.Errorf("no previous manifest version found")
	}
	return db.deviceSettings.LastSyncedManifestVersion, nil
}

func (db *Database) SetLastSyncedManifestVersion(version uint64) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.deviceSettings.LastSyncedManifestVersion = version
	db.TriggerDataPersist()
	return nil
}

// Bundle digest management
func (db *Database) GetLastSyncedBundleDigest() (string, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.deviceSettings.LastSyncedBundleDigest == "" {
		return "", fmt.Errorf("no previous bundle digest found")
	}
	return db.deviceSettings.LastSyncedBundleDigest, nil
}

func (db *Database) SetLastSyncedBundleDigest(digest string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.deviceSettings.LastSyncedBundleDigest = digest
	db.TriggerDataPersist()
	return nil
}

func NewDatabase(dataDir string) *Database {
	db := &Database{
		deployments:    make(map[string]*DeploymentRecord),
		deviceSettings: &DeviceSettingsRecord{},
		subscribers:    make([]func(string, *DeploymentRecord, DeploymentRecordChangeType), 0),
		dataDir:        dataDir,
		persistChan:    make(chan struct{}, 1),
		stopPersist:    make(chan struct{}),
	}

	// Load from disk
	db.load()

	// Start persistence goroutine
	go db.persistenceLoop()

	return db
}

func (db *Database) TriggerDataPersist() {
	select {
	case db.persistChan <- struct{}{}:
	default: // Already queued
	}
}

func (db *Database) persistenceLoop() {
	ticker := time.NewTicker(30 * time.Second) // Periodic saves
	defer ticker.Stop()

	for {
		select {
		case <-db.persistChan:
			db.save()
		case <-ticker.C:
			db.save()
		case <-db.stopPersist:
			db.save() // Final save
			return
		}
	}
}

func (db *Database) save() {
	db.mu.RLock()
	var dump = struct {
		Deployments    map[string]*DeploymentRecord `json:"deployments"`
		DeviceSettings *DeviceSettingsRecord        `json:"deviceSettings"`
	}{
		Deployments:    db.deployments,
		DeviceSettings: db.deviceSettings,
	}

	data, err := json.MarshalIndent(dump, "", "  ")
	db.mu.RUnlock()

	if err != nil {
		return
	}

	if err := os.MkdirAll(db.dataDir, 0750); err != nil {
		return
	}

	tempFile := filepath.Join(db.dataDir, "agent.database.json.tmp")
	finalFile := filepath.Join(db.dataDir, "agent.database.json")

	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		return
	}

	if err := os.Rename(tempFile, finalFile); err != nil {
		return
	}
}

func (db *Database) load() {
	file := filepath.Join(db.dataDir, "agent.database.json")
	data, err := os.ReadFile(filepath.Clean(file))
	if err != nil {
		return // File doesn't exist, start fresh
	}

	var dump = struct {
		Deployments    map[string]*DeploymentRecord `json:"deployments"`
		DeviceSettings *DeviceSettingsRecord        `json:"deviceSettings"`
	}{}
	if err := json.Unmarshal(data, &dump); err != nil {
		return
	}
	db.deployments = dump.Deployments
	db.deviceSettings = dump.DeviceSettings
}

func (db *Database) Subscribe(
	callback func(string, *DeploymentRecord, DeploymentRecordChangeType),
) {
	db.subscriberMu.Lock()
	defer db.subscriberMu.Unlock()
	db.subscribers = append(db.subscribers, callback)
}

func (db *Database) notify(
	appID string,
	record *DeploymentRecord,
	changeType DeploymentRecordChangeType,
) {
	db.subscriberMu.RLock()
	defer db.subscriberMu.RUnlock()
	subscribers := make(
		[]func(string, *DeploymentRecord, DeploymentRecordChangeType),
		len(db.subscribers),
	)
	copy(subscribers, db.subscribers)

	for _, callback := range subscribers {
		go callback(appID, record, changeType)
	}
}

func (db *Database) SetDesiredState(deploymentId string, state AppDeploymentState) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	record, exists := db.deployments[deploymentId]
	if !exists {
		// initialise a record object
		record = &DeploymentRecord{
			AppID:               deploymentId,
			DeploymentID:        deploymentId,
			ComponentViseStatus: make(map[string]sbi.ComponentStatus),
			Phase:               "pending",
			LastUpdated:         time.Now(),
		}
		// db.notify(deploymentId, record, DeploymentChangeTypeDesiredStateAdded)
	}

	// Only update if actually different
	// if record.DesiredState == nil || record.DesiredState.AppDeploymentYAMLHash !=
	// state.AppDeploymentYAMLHash {
	record.DesiredState = &state
	record.LastUpdated = time.Now()
	// Store the digest and URL from the state
	if state.Digest != nil {
		record.Digest = *state.Digest
	}
	if state.URL != nil {
		record.URL = *state.URL
	}
	// record should be updated in db after assigning all fields in record
	db.deployments[deploymentId] = record

	db.notify(deploymentId, record, DeploymentChangeTypeDesiredStateAdded)

	db.TriggerDataPersist()

	return nil
}

func (db *Database) SetCurrentState(deploymentId string, state AppDeploymentState) {
	db.mu.Lock()
	defer db.mu.Unlock()

	record, exists := db.deployments[deploymentId]
	if !exists {
		return
	}

	record.CurrentState = &state
	record.LastUpdated = time.Now()
}

func (db *Database) SetPhase(deploymentId, phase, message string) {
	db.mu.Lock()
	defer db.mu.Unlock()

	record, exists := db.deployments[deploymentId]
	if !exists {
		return
	}

	record.Phase = phase
	record.Message = message
	record.LastUpdated = time.Now()
	db.notify(deploymentId, record, DeploymentChangeTypeComponentPhaseChanged)
}

func (db *Database) SetComponentStatus(
	deploymentId, componentName string,
	status sbi.ComponentStatus,
) {
	db.mu.Lock()
	defer db.mu.Unlock()

	record, exists := db.deployments[deploymentId]
	if !exists {
		return
	}

	record.ComponentViseStatus[componentName] = status
	record.LastUpdated = time.Now()

	// Update overall phase based on component status
	switch status.State {
	case sbi.ComponentStatusStateInstalled:
		record.Phase = "running"
	case sbi.ComponentStatusStateFailed:
		record.Phase = "failed"
	}

	// Notify subscribers so StatusReporter can re-report with updated component status
	db.notify(deploymentId, record, DeploymentChangeTypeComponentPhaseChanged)
}

func (db *Database) GetDeployment(deploymentId string) (*DeploymentRecord, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	record, exists := db.deployments[deploymentId]
	if !exists {
		return nil, fmt.Errorf("deployment %s not found", deploymentId)
	}

	// Return a copy
	copy := *record
	return &copy, nil
}

func (db *Database) ListDeployments() []*DeploymentRecord {
	db.mu.RLock()
	defer db.mu.RUnlock()

	records := make([]*DeploymentRecord, 0, len(db.deployments))
	for _, record := range db.deployments {
		copy := *record
		records = append(records, &copy)
	}
	return records
}

func (db *Database) RemoveDeployment(deploymentId string) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if record, exists := db.deployments[deploymentId]; exists {
		delete(db.deployments, deploymentId)
		db.notify(deploymentId, record, DeploymentChangeTypeRecordDeleted)
		db.TriggerDataPersist()
	}
}

func (db *Database) NeedsReconciliation(deploymentId string) bool {
	db.mu.RLock()
	defer db.mu.RUnlock()

	record, exists := db.deployments[deploymentId]
	if !exists || record.DesiredState == nil {
		return false
	}

	if record.DesiredState.Status.Status.State == "REMOVED" {
		return false
	}

	// Check if desired and current states differ
	if record.CurrentState == nil {
		return true
	}

	// Compare the deployment status
	if record.CurrentState.Status.Status.State != record.DesiredState.Status.Status.State {
		return true
	}

	// Compare the embedded AppDeploymentManifest specs by marshaling to JSON
	currentSpecBytes, err1 := json.Marshal(record.CurrentState.Spec)
	desiredSpecBytes, err2 := json.Marshal(record.DesiredState.Spec)

	if err1 != nil || err2 != nil {
		// If marshaling fails, assume reconciliation is needed
		return true
	}

	// If specs are different, reconciliation is needed
	return string(currentSpecBytes) != string(desiredSpecBytes)
}

func (db *Database) GetDeviceSettings() (*DeviceSettingsRecord, error) {
	return db.deviceSettings, nil
}

func (db *Database) SetDeviceSettings(settings DeviceSettingsRecord) error {
	db.deviceSettings = &settings
	return nil
}

func (db *Database) SetDeviceOnboardState(state types.DeviceOnboardState) error {
	db.deviceSettings.State = state
	return nil
}

func (db *Database) IsDeviceOnboarded() (*DeviceSettingsRecord, bool, error) {
	return db.deviceSettings, db.deviceSettings.State == types.DeviceOnboardStateOnboarded, nil
}

func (db *Database) SetDeviceCanDeployHelm(deployable bool) {
	db.deviceSettings.CanDeployHelm = deployable
}

func (db *Database) SetDeviceCanDeployCompose(deployable bool) {
	db.deviceSettings.CanDeployCompose = deployable
}

func (db *Database) CanDeployAppProfile(profileType string) bool {
	return (strings.ToLower(profileType) == "helm" && db.deviceSettings.CanDeployHelm) ||
		(strings.ToLower(profileType) == "compose" && db.deviceSettings.CanDeployCompose)
}
