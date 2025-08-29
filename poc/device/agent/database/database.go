package database

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/margo/dev-repo/standard/generatedCode/wfm/sbi"
)

type DeploymentRecord struct {
	AppID           string
	DeploymentID    string
	DesiredState    *sbi.AppState
	CurrentState    *sbi.AppState
	ComponentStatus map[string]sbi.ComponentStatus
	Phase           string // "deploying", "running", "failed", "removing", "removed"
	Message         string
	LastUpdated     time.Time
}

type DeploymentChangeType string

const (
	DeploymentChangeTypeRecordAdded           DeploymentChangeType = "record-added"
	DeploymentChangeTypeRecordDeleted         DeploymentChangeType = "record-deleted"
	DeploymentChangeTypeComponentPhaseChanged DeploymentChangeType = "component-phase-changed"
	DeploymentChangeTypeDesiredStateAdded     DeploymentChangeType = "desired-state-added"
	DeploymentChangeTypeCurrentStateAdded     DeploymentChangeType = "current-state-added"
)

type DatabaseIfc interface {
	// if your database engine already has persistence, then just keep the implementation empty
	// we added an in-memory database implementation for this margo poc, hence needed this one
	TriggerDataPersist()
	Subscribe(callback func(string, *DeploymentRecord, DeploymentChangeType))
	SetDesiredState(deploymentId string, state sbi.AppState)
	SetCurrentState(deploymentId string, state sbi.AppState)
	SetPhase(deploymentId, phase, message string)
	SetComponentStatus(deploymentId, componentName string, status sbi.ComponentStatus)
	GetDeployment(deploymentId string) (*DeploymentRecord, error)
	ListDeployments() []*DeploymentRecord
	RemoveDeployment(deploymentId string)
	NeedsReconciliation(deploymentId string) bool
}

type Database struct {
	deployments  map[string]*DeploymentRecord
	subscribers  []func(string, *DeploymentRecord, DeploymentChangeType) // appID, record
	mu           sync.RWMutex
	subscriberMu sync.RWMutex

	// for persistence
	dataDir     string
	persistChan chan struct{}
	stopPersist chan struct{}
}

func NewDatabase(dataDir string) *Database {
	db := &Database{
		deployments: make(map[string]*DeploymentRecord),
		subscribers: make([]func(string, *DeploymentRecord, DeploymentChangeType), 0),
		dataDir:     dataDir,
		persistChan: make(chan struct{}, 1),
		stopPersist: make(chan struct{}),
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
	data, err := json.MarshalIndent(db.deployments, "", "  ")
	db.mu.RUnlock()

	if err != nil {
		return
	}

	os.MkdirAll(db.dataDir, 0755)
	tempFile := filepath.Join(db.dataDir, "deployments.json.tmp")
	finalFile := filepath.Join(db.dataDir, "deployments.json")

	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return
	}

	os.Rename(tempFile, finalFile) // Atomic
}

func (db *Database) load() {
	file := filepath.Join(db.dataDir, "agent.database.json")
	data, err := os.ReadFile(file)
	if err != nil {
		return // File doesn't exist, start fresh
	}

	var deployments map[string]*DeploymentRecord
	if err := json.Unmarshal(data, &deployments); err != nil {
		return
	}

	db.deployments = deployments
}

func (db *Database) Subscribe(callback func(string, *DeploymentRecord, DeploymentChangeType)) {
	db.subscriberMu.Lock()
	defer db.subscriberMu.Unlock()
	db.subscribers = append(db.subscribers, callback)
}

func (db *Database) notify(appID string, record *DeploymentRecord, changeType DeploymentChangeType) {
	db.subscriberMu.RLock()
	defer db.subscriberMu.RUnlock()
	subscribers := make([]func(string, *DeploymentRecord, DeploymentChangeType), len(db.subscribers))
	copy(subscribers, db.subscribers)

	for _, callback := range subscribers {
		go callback(appID, record, changeType)
	}
}

func (db *Database) SetDesiredState(deploymentId string, state sbi.AppState) {
	db.mu.Lock()
	defer db.mu.Unlock()

	record, exists := db.deployments[deploymentId]
	if !exists {
		record = &DeploymentRecord{
			AppID:           deploymentId,
			ComponentStatus: make(map[string]sbi.ComponentStatus),
			Phase:           "pending",
			LastUpdated:     time.Now(),
		}
		db.deployments[deploymentId] = record
	}

	// Only update if actually different
	// if record.DesiredState == nil || record.DesiredState.AppDeploymentYAMLHash != state.AppDeploymentYAMLHash {
	record.DesiredState = &state
	record.LastUpdated = time.Now()
	db.notify(deploymentId, record, DeploymentChangeTypeDesiredStateAdded)
	// }
}

func (db *Database) SetCurrentState(deploymentId string, state sbi.AppState) {
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

func (db *Database) SetComponentStatus(deploymentId, componentName string, status sbi.ComponentStatus) {
	db.mu.Lock()
	defer db.mu.Unlock()

	record, exists := db.deployments[deploymentId]
	if !exists {
		return
	}

	record.ComponentStatus[componentName] = status
	record.LastUpdated = time.Now()

	// Update overall phase based on component status
	if status.State == sbi.ComponentStatusStateInstalled {
		record.Phase = "running"
	} else if status.State == sbi.ComponentStatusStateFailed {
		record.Phase = "failed"
	}
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
	delete(db.deployments, deploymentId)
}

func (db *Database) NeedsReconciliation(deploymentId string) bool {
	db.mu.RLock()
	defer db.mu.RUnlock()

	record, exists := db.deployments[deploymentId]
	if !exists || record.DesiredState == nil {
		return false
	}

	if record.DesiredState.AppState == "REMOVED" {
		// temporarily added
		return false
	}

	// Check if desired and current states differ
	if record.CurrentState == nil {
		return true
	}

	if record.CurrentState.AppState != record.DesiredState.AppState {
		return true
	}

	return record.DesiredState.AppDeploymentYAMLHash != record.CurrentState.AppDeploymentYAMLHash ||
		record.DesiredState.AppState != record.CurrentState.AppState
}
