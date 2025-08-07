package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/margo/dev-repo/standard/generatedCode/wfm/sbi"
)

type AgentDatabase interface {
	Start() error
	Stop() error
	Clear() error
	GetAllWorkloads() (map[string]sbi.AppState, error)
	GetWorkload(workloadId string) (sbi.AppState, error)
	AddWorkload(workload sbi.AppState) error
	Remove(workloadId string) error
	UpdateWorkload(workload sbi.AppState) error
	UpsertWorkload(workload sbi.AppState) error
	WorkloadExists(workloadId string) bool
	GetWorkloadCount() int
	GetDeviceProperties() *sbi.Properties
	GetDeviceAuth() *sbi.OnboardingResponse
	GetDevice() (*DeviceModel, error)
	UpsertDevice(device *DeviceModel) error
	UpsertDeviceCapabilities(capabilities *sbi.DeviceCapabilities) error
	UpsertDeviceAuth(auth *sbi.OnboardingResponse) error
	Subscribe(subscriber WorkloadDatabaseSubscriber) error
}

// DeviceStatus represents device operational status.
type DeviceStatus string

const (
	DeviceStatusOffline DeviceStatus = "offline" // Device is offline
	DeviceStatusOnline  DeviceStatus = "online"  // Device is online
	DeviceStatusError   DeviceStatus = "error"   // Device is in an error state
	DeviceStatusBooting DeviceStatus = "booting" // Device is in booting state
)

type DeviceModel struct {
	Id               string
	DeviceProperties *sbi.Properties
	DeviceAuth       *sbi.OnboardingResponse
	Status           DeviceStatus
	LastSeen         time.Time
}

type DatabaseDump struct {
	Workloads map[string]sbi.AppState
	Device    DeviceModel
}

// AgentInMemoryDatabase is an in-memory database that stores application workloads
// and provides event-driven notifications to subscribers when data changes.
// It combines in-memory storage for fast access with disk persistence for durability.
type AgentInMemoryDatabase struct {
	// bkpDataDir is the directory where database dumps are stored for persistence
	bkpDataDir string

	// device object
	device DeviceModel

	// workloads maps workload ID to app state - this is the source of truth
	workloads map[string]sbi.AppState

	// workloadChangeSubscribers is a list of components that want to be notified of database changes
	// Using slice instead of map for simplicity, assuming small number of workloadChangeSubscribers
	workloadChangeSubscribers []WorkloadDatabaseSubscriber

	// workloadEventChan is a buffered channel for async event publishing to avoid blocking database operations
	workloadEventChan chan WorkloadDatabaseEvent

	// mu protects concurrent access to subscribers slice and currentWorkloads map
	// Using RWMutex to allow concurrent reads while protecting writes
	mu sync.RWMutex

	// ctx is the context for controlling the lifecycle of background goroutines
	ctx context.Context

	// operationStopper is used to gracefully shutdown background operations
	// Separate from ctx to allow controlled shutdown sequence
	operationStopper context.CancelFunc
}

// NewAgentInMemoryDatabase creates a new in-memory database instance.
// It sets up the database with event publishing capabilities and prepares for persistence.
//
// Parameters:
//   - ctx: Context for controlling database lifecycle
//   - dataDir: Directory for storing database dumps (defaults to "./data" if empty)
func NewAgentInMemoryDatabase(ctx context.Context, dataDir string) *AgentInMemoryDatabase {
	if dataDir == "" {
		dataDir = "./data"
	}

	// Design Note: We create a separate context with cancel function to ensure
	// we can control the shutdown sequence independently of the parent context.
	localCtx, canceller := context.WithCancel(ctx)

	return &AgentInMemoryDatabase{
		device: DeviceModel{
			Id:               "",
			DeviceProperties: nil,
			DeviceAuth:       nil,
			Status:           DeviceStatusOnline,
			LastSeen:         time.Now().UTC(),
		},
		workloads:                 make(map[string]sbi.AppState),
		bkpDataDir:                dataDir,
		workloadChangeSubscribers: make([]WorkloadDatabaseSubscriber, 0),
		workloadEventChan:         make(chan WorkloadDatabaseEvent, 100), // Buffered channel to prevent blocking
		ctx:                       localCtx,
		operationStopper:          canceller,
	}
}

// Start initializes the database by restoring from disk and starting background processes.
// This method should be called before using the database.
func (db *AgentInMemoryDatabase) Start() error {
	// Restore data from disk first to ensure we have the latest database state
	if err := db.restore(); err != nil {
		return fmt.Errorf("failed to restore database: %w", err)
	}

	if db.ctx == nil {
		panic("line 114")
	}

	// Start event publisher in background goroutine
	go db.eventPublisher(db.ctx)

	return nil
}

// Stop gracefully shuts down the database by stopping background operations
// and persisting current state to disk.
func (db *AgentInMemoryDatabase) Stop() error {
	// Stop all background operations first
	db.operationStopper()

	// TODO: Wait a moment for goroutines to finish (could be improved with sync.WaitGroup)
	time.Sleep(100 * time.Millisecond)

	// Dump current state to disk for persistence
	if err := db.dump(); err != nil {
		return fmt.Errorf("failed to dump database: %w", err)
	}

	return nil
}

func (db *AgentInMemoryDatabase) UpsertDevice(device *DeviceModel) error {
	db.device = *device
	return nil
}

func (db *AgentInMemoryDatabase) UpsertDeviceCapabilities(capabilities *sbi.DeviceCapabilities) error {
	db.device.DeviceProperties = &capabilities.Properties
	return nil
}

func (db *AgentInMemoryDatabase) UpsertDeviceAuth(auth *sbi.OnboardingResponse) error {
	db.device.DeviceAuth = auth
	return nil
}

func (db *AgentInMemoryDatabase) GetDevice() (*DeviceModel, error) {
	return &db.device, nil
}

// dump persists the current in-memory state to disk as JSON.
// Uses atomic write (write to temp file, then rename) to prevent corruption.
func (db *AgentInMemoryDatabase) dump() error {
	// Ensure data directory exists before writing
	if err := os.MkdirAll(db.bkpDataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}
	filename := filepath.Join(db.bkpDataDir, "agent.dump.json")

	dump := DatabaseDump{
		Workloads: db.workloads,
		Device:    db.device,
	}

	// Marshal with indentation for human readability (could be optimized for size)
	data, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal workloads: %w", err)
	}

	// Atomic write: write to temporary file first, then rename
	// This prevents corruption if the process is killed during write
	tempFile := filename + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write dump file: %w", err)
	}

	// Rename is atomic on most filesystems, ensuring consistency
	if err := os.Rename(tempFile, filename); err != nil {
		return fmt.Errorf("failed to rename dump file: %w", err)
	}

	return nil
}

// restore loads the database state from disk if a dump file exists.
// If no dump exists, initializes with empty state.
func (db *AgentInMemoryDatabase) restore() error {
	filename := filepath.Join(db.bkpDataDir, "agent.dump.json")

	// Check if dump file exists - missing file is not an error
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		// No dump file exists, start with empty database
		db.workloads = make(map[string]sbi.AppState)
		return nil
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read dump file: %w", err)
	}

	dump := DatabaseDump{}

	if err := json.Unmarshal(data, &dump); err != nil {
		return fmt.Errorf("failed to unmarshal database dump: %w", err)
	}

	db.workloads = dump.Workloads
	// let us initialize the map if it is nil
	if db.workloads == nil {
		db.workloads = make(map[string]sbi.AppState)
	}

	db.device = dump.Device
	return nil
}

// GetAllWorkloads returns a copy of all workloads in the database.
// Returns a copy to prevent external modifications to internal state.
func (db *AgentInMemoryDatabase) GetAllWorkloads() (map[string]sbi.AppState, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	// Return a copy to prevent external modifications
	workloads := make(map[string]sbi.AppState, len(db.workloads))
	for k, v := range db.workloads {
		workloads[k] = v
	}
	return workloads, nil
}

// GetWorkload retrieves a specific workload by ID.
// Returns an error if the workload doesn't exist.
func (db *AgentInMemoryDatabase) GetWorkload(workloadId string) (sbi.AppState, error) {
	if workloadId == "" {
		return sbi.AppState{}, fmt.Errorf("workload ID cannot be empty")
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	workload, exists := db.workloads[workloadId]
	if !exists {
		return sbi.AppState{}, fmt.Errorf("workload with ID %s not found", workloadId)
	}

	return workload, nil
}

// AddWorkload adds a new workload to the database.
// Returns an error if a workload with the same ID already exists.
func (db *AgentInMemoryDatabase) AddWorkload(workload sbi.AppState) error {
	if workload.AppId == "" {
		return fmt.Errorf("workload AppId cannot be empty")
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	// Check if workload already exists
	if _, exists := db.workloads[workload.AppId]; exists {
		return fmt.Errorf("workload with ID %s already exists", workload.AppId)
	}

	db.workloads[workload.AppId] = workload

	// Determine event type based on whether workload existed
	eventType := EventAppAdded

	// Publish event to notify subscribers of the change
	// Note: publishEvent is non-blocking to avoid deadlocks
	db.publishEvent(WorkloadDatabaseEvent{
		Type:      eventType,
		AppID:     workload.AppId,
		OldState:  nil,
		NewState:  &workload,
		Timestamp: time.Now(),
	})
	return nil
}

// Remove deletes a workload from the database.
// Returns an error if the workload doesn't exist.
func (db *AgentInMemoryDatabase) Remove(workloadId string) error {
	if workloadId == "" {
		return fmt.Errorf("workload ID cannot be empty")
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	existingAppState, exists := db.workloads[workloadId]
	if !exists {
		return fmt.Errorf("workload with ID %s not found", workloadId)
	}

	delete(db.workloads, workloadId)

	db.publishEvent(WorkloadDatabaseEvent{
		Type:      EventAppDeleted,
		AppID:     workloadId,
		Timestamp: time.Now(),
		OldState:  &existingAppState,
	})

	return nil
}

// UpdateWorkload updates an existing workload in the database.
// Returns an error if the workload doesn't exist.
func (db *AgentInMemoryDatabase) UpdateWorkload(workload sbi.AppState) error {
	if workload.AppId == "" {
		return fmt.Errorf("workload AppId cannot be empty")
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	// Check if workload exists
	existingWorkload, exists := db.workloads[workload.AppId]
	if !exists {
		return fmt.Errorf("workload with ID %s not found", workload.AppId)
	}

	db.workloads[workload.AppId] = workload

	db.publishEvent(WorkloadDatabaseEvent{
		Type:      EventAppDeleted,
		AppID:     workload.AppId,
		Timestamp: time.Now(),
		OldState:  &existingWorkload,
		NewState:  &workload,
	})
	return nil
}

// UpsertWorkload inserts or updates a workload and publishes appropriate events.
func (db *AgentInMemoryDatabase) UpsertWorkload(workload sbi.AppState) error {
	if workload.AppId == "" {
		return fmt.Errorf("workload AppId cannot be empty")
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	// Capture old state for event publishing
	oldState, exists := db.workloads[workload.AppId]
	db.workloads[workload.AppId] = workload

	// Determine event type based on whether workload existed
	eventType := EventAppAdded
	if exists {
		eventType = EventAppUpdated
	}

	// Publish event to notify subscribers of the change
	// Note: publishEvent is non-blocking to avoid deadlocks
	db.publishEvent(WorkloadDatabaseEvent{
		Type:      eventType,
		AppID:     workload.AppId,
		OldState:  &oldState,
		NewState:  &workload,
		Timestamp: time.Now(),
	})

	return nil
}

// WorkloadExists checks if a workload with the given ID exists.
func (db *AgentInMemoryDatabase) WorkloadExists(workloadId string) bool {
	db.mu.RLock()
	defer db.mu.RUnlock()

	_, exists := db.workloads[workloadId]
	return exists
}

// GetWorkloadCount returns the total number of workloads in the database.
func (db *AgentInMemoryDatabase) GetWorkloadCount() int {
	db.mu.RLock()
	defer db.mu.RUnlock()

	return len(db.workloads)
}

// GetDeviceProperties returns error if unable to update device properties.
func (db *AgentInMemoryDatabase) GetDeviceProperties() *sbi.Properties {
	return db.device.DeviceProperties
}

// GetDeviceAuth returns error if unable to get device auth.
func (db *AgentInMemoryDatabase) GetDeviceAuth() *sbi.OnboardingResponse {
	return db.device.DeviceAuth
}

// Clear removes all workloads from the database.
// This primarily be used for testing or reset scenarios
func (db *AgentInMemoryDatabase) Clear() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	eventType := EventAppDeleted
	for _, workload := range db.workloads {
		db.publishEvent(WorkloadDatabaseEvent{
			Type:      eventType,
			AppID:     workload.AppId,
			OldState:  &workload,
			NewState:  nil,
			Timestamp: time.Now(),
		})
	}

	db.workloads = make(map[string]sbi.AppState)
	return nil
}

// eventPublisher runs in a background goroutine and distributes database events
// to all registered subscribers.
//
// Note: This runs in a separate goroutine to avoid blocking database operations.
// Each subscriber is notified in its own goroutine to prevent slow subscribers
// from blocking others.
func (db *AgentInMemoryDatabase) eventPublisher(ctx context.Context) {
	if ctx == nil {
		panic("context cannot be nil")
	}

	for {
		select {
		case event := <-db.workloadEventChan:
			// Get a snapshot of current subscribers under read lock
			db.mu.RLock()
			subscribers := make([]WorkloadDatabaseSubscriber, len(db.workloadChangeSubscribers))
			copy(subscribers, db.workloadChangeSubscribers)
			db.mu.RUnlock()

			// Notify all subscribers in parallel to avoid blocking
			// Each subscriber gets its own goroutine to prevent slow subscribers
			// from affecting others
			for _, subscriber := range subscribers {
				go func(sub WorkloadDatabaseSubscriber) {
					if err := sub.OnDatabaseEvent(event); err != nil {
						// Log error but don't block other subscribers
						// In production, this should use structured logging
						log.Printf("Subscriber %s failed to handle event: %v",
							sub.GetSubscriberID(), err)

						// TODO: implement a dead letter queue or retry mechanism
						// for failed event deliveries ????
					}
				}(subscriber)
			}

		case <-ctx.Done():
			// shutdown
			log.Print("shutting down the database...")
			return
		}
	}
}

// Subscribe registers a subscriber to receive database events.
// Subscribers will be notified of all workload changes via the OnDatabaseEvent method.
func (db *AgentInMemoryDatabase) Subscribe(subscriber WorkloadDatabaseSubscriber) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// TODO: Consider checking for duplicate subscribers based on ID
	db.workloadChangeSubscribers = append(db.workloadChangeSubscribers, subscriber)
	return nil
}

// publishEvent sends an event to the event channel for distribution to subscribers.
// This method is non-blocking - if the channel is full, the event is dropped.
//
// Note: Non-blocking publish prevents database operations from hanging
// if the event system is overwhelmed.
func (db *AgentInMemoryDatabase) publishEvent(event WorkloadDatabaseEvent) {
	select {
	case db.workloadEventChan <- event:
		// Event successfully queued
	default:
		// TODO: Event channel full, log this
		log.Printf("Warning: Event channel full, dropping event for app %s", event.AppID)
	}
}
