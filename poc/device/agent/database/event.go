package database

import (
	"time"

	"github.com/margo/dev-repo/standard/generatedCode/wfm/sbi"
)

// Event system for database changes
type WorkloadDatabaseEvent struct {
	Type      EventType
	AppID     string
	OldState  *sbi.AppState
	NewState  *sbi.AppState
	Timestamp time.Time
}

type EventType string

const (
	EventAppAdded     EventType = "APP_ADDED"
	EventAppUpdated   EventType = "APP_UPDATED"
	EventAppDeleted   EventType = "APP_DELETED"
	EventStateChanged EventType = "STATE_CHANGED"
)

type WorkloadDatabaseSubscriber interface {
	OnDatabaseEvent(event WorkloadDatabaseEvent) error
	GetSubscriberID() string
}
