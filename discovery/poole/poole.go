// Package poole provides an event dispatcher and action registry for HAL 9000.
// "Look Dave, I can see you're really upset about this. I honestly think you
// ought to sit down calmly, take a stress pill, and think things over."
//
// Poole dispatches events from Floyd (watchers) to registered action handlers,
// coordinates with Bowman (fetchers) to gather context, and invokes Claude for
// intelligent responses. Named after Frank Poole, HAL's first victim who was
// dispatched during a spacewalk.
package poole

import (
	"fmt"
	"log"
	"sync"

	"github.com/pearcec/hal9000/discovery/events"
)

// ActionType identifies the type of action to take.
type ActionType string

const (
	// ActionTypeImmediate executes the action immediately.
	ActionTypeImmediate ActionType = "immediate"
	// ActionTypeDelayed schedules the action for later execution.
	ActionTypeDelayed ActionType = "delayed"
	// ActionTypeBatched collects events and processes them together.
	ActionTypeBatched ActionType = "batched"
)

// Action defines an action to take when an event is received.
type Action struct {
	Name       string              // Unique action identifier
	EventType  string              // Event type to match (e.g., "jira:issue.created")
	Enabled    bool                // Whether this action is active
	Fetchers   []string            // Bowman fetchers to invoke for context
	Prompt     string              // Prompt template name from prompt registry
	ActionType ActionType          // immediate, delayed, or batched
	Metadata   map[string]string   // Additional action metadata
}

// ActionResult is returned after executing an action.
type ActionResult struct {
	ActionName string      // Name of the action that was executed
	Success    bool        // Whether the action completed successfully
	Output     string      // Output from the action (Claude's response)
	Error      error       // Any error that occurred
	Metadata   map[string]interface{} // Additional result data
}

// ActionHandler processes an event and returns a result.
type ActionHandler func(event events.StorageEvent, action *Action) ActionResult

// Dispatcher routes events from Floyd to registered action handlers.
// It integrates with the event bus to receive events and triggers
// appropriate actions based on the action registry.
type Dispatcher struct {
	registry    *Registry
	scheduler   *Scheduler
	bus         *events.Bus
	handlers    map[string]ActionHandler
	mu          sync.RWMutex
	running     bool
}

// NewDispatcher creates a new event dispatcher.
func NewDispatcher(registry *Registry, scheduler *Scheduler) *Dispatcher {
	return &Dispatcher{
		registry:  registry,
		scheduler: scheduler,
		handlers:  make(map[string]ActionHandler),
	}
}

// RegisterHandler registers a custom handler for a specific action.
// If no custom handler is registered, the default handler is used.
func (d *Dispatcher) RegisterHandler(actionName string, handler ActionHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[actionName] = handler
}

// Connect attaches the dispatcher to an event bus.
// Events published to the bus will be routed to appropriate actions.
func (d *Dispatcher) Connect(bus *events.Bus) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.bus = bus
	d.running = true

	// Subscribe to the event bus
	bus.Subscribe(d.handleEvent)
	log.Println("[poole] Dispatcher connected to event bus")
}

// handleEvent processes incoming events from the bus.
func (d *Dispatcher) handleEvent(event events.StorageEvent) events.StorageResult {
	d.mu.RLock()
	if !d.running {
		d.mu.RUnlock()
		return events.StorageResult{}
	}
	d.mu.RUnlock()

	// Build the event type key: source:type (e.g., "jira:issue.created")
	// Note: Floyd events use the Event type, but storage events come through
	// after storage. We dispatch based on the source.
	eventKey := fmt.Sprintf("%s:*", event.Source)

	// Find matching actions
	actions := d.registry.GetActionsForEvent(eventKey)
	if len(actions) == 0 {
		// Try exact source match
		actions = d.registry.GetActionsForEvent(event.Source)
	}

	if len(actions) == 0 {
		log.Printf("[poole] No actions registered for event: %s", eventKey)
		return events.StorageResult{}
	}

	// Dispatch each matching action
	for _, action := range actions {
		if !action.Enabled {
			continue
		}

		go d.dispatchAction(event, action)
	}

	return events.StorageResult{}
}

// dispatchAction executes a single action for an event.
func (d *Dispatcher) dispatchAction(event events.StorageEvent, action *Action) {
	log.Printf("[poole] Dispatching action '%s' for event from %s", action.Name, event.Source)

	d.mu.RLock()
	handler, hasCustom := d.handlers[action.Name]
	d.mu.RUnlock()

	if !hasCustom {
		handler = d.defaultHandler
	}

	switch action.ActionType {
	case ActionTypeImmediate:
		result := handler(event, action)
		if result.Error != nil {
			log.Printf("[poole] Action '%s' failed: %v", action.Name, result.Error)
		} else {
			log.Printf("[poole] Action '%s' completed successfully", action.Name)
		}

	case ActionTypeDelayed, ActionTypeBatched:
		// Schedule for later execution
		d.scheduler.Schedule(event, action, handler)
	}
}

// defaultHandler is the built-in action handler that uses Claude.
func (d *Dispatcher) defaultHandler(event events.StorageEvent, action *Action) ActionResult {
	result := ActionResult{
		ActionName: action.Name,
		Metadata:   make(map[string]interface{}),
	}

	// Get the prompt template
	prompt, err := d.registry.GetPrompt(action.Prompt)
	if err != nil {
		result.Error = fmt.Errorf("failed to load prompt '%s': %w", action.Prompt, err)
		return result
	}

	// Execute via scheduler (which handles Claude invocation)
	output, err := d.scheduler.Execute(event, action, prompt)
	if err != nil {
		result.Error = err
		return result
	}

	result.Success = true
	result.Output = output
	return result
}

// Stop shuts down the dispatcher.
func (d *Dispatcher) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.running = false
	log.Println("[poole] Dispatcher stopped")
}
