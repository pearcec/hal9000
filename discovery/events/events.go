// Package events provides an event bus for decoupling HAL 9000 components.
// "I know I've made some very poor decisions recently, but I can give you
// my complete assurance that my work will be back to normal."
//
// This package enables event-driven communication between Floyd (watchers)
// and Bowman (storage), allowing components to operate independently.
package events

import "time"

// EventType identifies the kind of storage operation.
type EventType string

const (
	// EventStore requests storage of raw event data.
	EventStore EventType = "store"
	// EventDelete requests deletion of stored event data.
	EventDelete EventType = "delete"
)

// StorageEvent represents a request to store or delete data.
// Floyd emits these events; Bowman (via a subscriber) processes them.
type StorageEvent struct {
	Type      EventType              // store or delete
	Source    string                 // e.g., "google-calendar", "jira", "slack"
	EventID   string                 // Unique identifier for the event
	FetchedAt time.Time              // When the data was fetched
	Category  string                 // Storage category (e.g., "calendar", "jira", "slack")
	Data      map[string]interface{} // Raw event data (nil for delete operations)
}

// StorageResult is returned after processing a StorageEvent.
type StorageResult struct {
	Path  string // File path where data was stored (empty for deletes)
	Error error  // Any error that occurred
}
