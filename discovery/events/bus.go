package events

import (
	"log"
	"sync"
)

// Handler processes storage events.
type Handler func(event StorageEvent) StorageResult

// Bus is a simple in-process event bus using Go channels.
// It decouples event producers (Floyd) from consumers (Bowman).
type Bus struct {
	handlers []Handler
	events   chan StorageEvent
	results  chan StorageResult
	wg       sync.WaitGroup
	mu       sync.RWMutex
	closed   bool
}

// NewBus creates a new event bus with the specified buffer size.
func NewBus(bufferSize int) *Bus {
	if bufferSize < 1 {
		bufferSize = 100
	}
	b := &Bus{
		events:  make(chan StorageEvent, bufferSize),
		results: make(chan StorageResult, bufferSize),
	}
	b.start()
	return b
}

// Subscribe registers a handler to process storage events.
// Handlers are called synchronously in the order they were registered.
func (b *Bus) Subscribe(handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = append(b.handlers, handler)
}

// Publish sends a storage event to all subscribers.
// Returns the result from the last handler (typically the storage handler).
// This is synchronous to preserve the original Floyd behavior where
// storage errors are logged immediately.
func (b *Bus) Publish(event StorageEvent) StorageResult {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return StorageResult{Error: nil}
	}

	var result StorageResult
	for _, handler := range b.handlers {
		result = handler(event)
		if result.Error != nil {
			log.Printf("[events] Handler error: %v", result.Error)
		}
	}
	return result
}

// start begins processing events (for future async support).
func (b *Bus) start() {
	// Currently handlers are called synchronously in Publish().
	// This method exists for future async processing if needed.
}

// Close shuts down the event bus.
func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
}
