package events

import (
	"testing"
	"time"
)

func TestBusPublishSubscribe(t *testing.T) {
	bus := NewBus(10)
	defer bus.Close()

	var received []StorageEvent

	// Subscribe a test handler
	bus.Subscribe(func(event StorageEvent) StorageResult {
		received = append(received, event)
		return StorageResult{Path: "/test/path"}
	})

	// Publish an event
	event := StorageEvent{
		Type:      EventStore,
		Source:    "test-source",
		EventID:   "test-123",
		FetchedAt: time.Now(),
		Category:  "test",
		Data:      map[string]interface{}{"key": "value"},
	}

	result := bus.Publish(event)

	// Verify handler was called
	if len(received) != 1 {
		t.Errorf("expected 1 event, got %d", len(received))
	}

	if received[0].EventID != "test-123" {
		t.Errorf("expected event ID 'test-123', got '%s'", received[0].EventID)
	}

	if result.Path != "/test/path" {
		t.Errorf("expected path '/test/path', got '%s'", result.Path)
	}
}

func TestBusMultipleHandlers(t *testing.T) {
	bus := NewBus(10)
	defer bus.Close()

	callOrder := []string{}

	bus.Subscribe(func(event StorageEvent) StorageResult {
		callOrder = append(callOrder, "first")
		return StorageResult{}
	})

	bus.Subscribe(func(event StorageEvent) StorageResult {
		callOrder = append(callOrder, "second")
		return StorageResult{Path: "/final"}
	})

	event := StorageEvent{Type: EventStore, EventID: "test"}
	result := bus.Publish(event)

	if len(callOrder) != 2 {
		t.Errorf("expected 2 handlers called, got %d", len(callOrder))
	}

	if callOrder[0] != "first" || callOrder[1] != "second" {
		t.Errorf("handlers called in wrong order: %v", callOrder)
	}

	// Last handler's result should be returned
	if result.Path != "/final" {
		t.Errorf("expected final result, got '%s'", result.Path)
	}
}

func TestBusClose(t *testing.T) {
	bus := NewBus(10)

	var callCount int
	bus.Subscribe(func(event StorageEvent) StorageResult {
		callCount++
		return StorageResult{}
	})

	// Publish before close
	bus.Publish(StorageEvent{Type: EventStore})
	if callCount != 1 {
		t.Errorf("expected 1 call before close, got %d", callCount)
	}

	// Close the bus
	bus.Close()

	// Publish after close should not call handlers
	bus.Publish(StorageEvent{Type: EventStore})
	if callCount != 1 {
		t.Errorf("expected still 1 call after close, got %d", callCount)
	}
}

func TestEventTypes(t *testing.T) {
	if EventStore != "store" {
		t.Errorf("EventStore should be 'store', got '%s'", EventStore)
	}
	if EventDelete != "delete" {
		t.Errorf("EventDelete should be 'delete', got '%s'", EventDelete)
	}
}
