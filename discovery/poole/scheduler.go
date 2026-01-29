package poole

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/pearcec/hal9000/discovery/events"
)

// ScheduledAction represents an action waiting to be executed.
type ScheduledAction struct {
	Event     events.StorageEvent
	Action    *Action
	Handler   ActionHandler
	ScheduledAt time.Time
	ExecuteAt   time.Time
}

// Scheduler manages action execution, including delayed and batched actions.
// It handles invoking Claude via the CLI and coordinating with Bowman fetchers.
type Scheduler struct {
	queue     []*ScheduledAction
	batches   map[string][]*ScheduledAction // batchKey -> actions
	mu        sync.Mutex
	running   bool
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// NewScheduler creates a new action scheduler.
func NewScheduler() *Scheduler {
	return &Scheduler{
		queue:   make([]*ScheduledAction, 0),
		batches: make(map[string][]*ScheduledAction),
		stopCh:  make(chan struct{}),
	}
}

// Start begins the scheduler's background processing.
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.processLoop()
	log.Println("[poole][scheduler] Started")
}

// Stop shuts down the scheduler.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopCh)
	s.wg.Wait()
	log.Println("[poole][scheduler] Stopped")
}

// processLoop runs the scheduler's main loop.
func (s *Scheduler) processLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.processQueue()
			s.processBatches()
		}
	}
}

// processQueue executes delayed actions that are due.
func (s *Scheduler) processQueue() {
	s.mu.Lock()
	now := time.Now()

	var remaining []*ScheduledAction
	var ready []*ScheduledAction

	for _, sa := range s.queue {
		if now.After(sa.ExecuteAt) || now.Equal(sa.ExecuteAt) {
			ready = append(ready, sa)
		} else {
			remaining = append(remaining, sa)
		}
	}

	s.queue = remaining
	s.mu.Unlock()

	// Execute ready actions outside the lock
	for _, sa := range ready {
		result := sa.Handler(sa.Event, sa.Action)
		if result.Error != nil {
			log.Printf("[poole][scheduler] Delayed action '%s' failed: %v", sa.Action.Name, result.Error)
		} else {
			log.Printf("[poole][scheduler] Delayed action '%s' completed", sa.Action.Name)
		}
	}
}

// processBatches executes batched actions that have accumulated.
func (s *Scheduler) processBatches() {
	s.mu.Lock()
	if len(s.batches) == 0 {
		s.mu.Unlock()
		return
	}

	// Copy and clear batches
	batches := s.batches
	s.batches = make(map[string][]*ScheduledAction)
	s.mu.Unlock()

	// Process each batch
	for batchKey, actions := range batches {
		if len(actions) == 0 {
			continue
		}

		// Use the first action's handler for the batch
		firstAction := actions[0]
		log.Printf("[poole][scheduler] Processing batch '%s' with %d actions", batchKey, len(actions))

		// Combine events for batch processing
		combinedEvent := combineBatchEvents(actions)
		result := firstAction.Handler(combinedEvent, firstAction.Action)
		if result.Error != nil {
			log.Printf("[poole][scheduler] Batch '%s' failed: %v", batchKey, result.Error)
		} else {
			log.Printf("[poole][scheduler] Batch '%s' completed", batchKey)
		}
	}
}

// combineBatchEvents merges multiple events into one for batch processing.
func combineBatchEvents(actions []*ScheduledAction) events.StorageEvent {
	if len(actions) == 0 {
		return events.StorageEvent{}
	}

	first := actions[0].Event

	// Combine event IDs
	var eventIDs []string
	for _, a := range actions {
		eventIDs = append(eventIDs, a.Event.EventID)
	}

	return events.StorageEvent{
		Type:      first.Type,
		Source:    first.Source,
		EventID:   strings.Join(eventIDs, ","),
		FetchedAt: time.Now(),
		Category:  first.Category,
		Data: map[string]interface{}{
			"batch_count": len(actions),
			"event_ids":   eventIDs,
		},
	}
}

// Schedule adds an action to be executed later.
func (s *Scheduler) Schedule(event events.StorageEvent, action *Action, handler ActionHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sa := &ScheduledAction{
		Event:       event,
		Action:      action,
		Handler:     handler,
		ScheduledAt: time.Now(),
	}

	switch action.ActionType {
	case ActionTypeDelayed:
		// Default delay: 1 minute
		delay := time.Minute
		if delayStr, ok := action.Metadata["delay"]; ok {
			if d, err := time.ParseDuration(delayStr); err == nil {
				delay = d
			}
		}
		sa.ExecuteAt = time.Now().Add(delay)
		s.queue = append(s.queue, sa)
		log.Printf("[poole][scheduler] Scheduled delayed action '%s' for %v", action.Name, sa.ExecuteAt)

	case ActionTypeBatched:
		// Group by action name
		batchKey := action.Name
		s.batches[batchKey] = append(s.batches[batchKey], sa)
		log.Printf("[poole][scheduler] Added to batch '%s' (now %d items)", batchKey, len(s.batches[batchKey]))

	default:
		// Immediate - execute now (shouldn't reach here normally)
		go func() {
			result := handler(event, action)
			if result.Error != nil {
				log.Printf("[poole][scheduler] Action '%s' failed: %v", action.Name, result.Error)
			}
		}()
	}
}

// Execute runs an action immediately and returns the result.
// This invokes Claude with the given prompt and event context.
func (s *Scheduler) Execute(event events.StorageEvent, action *Action, promptTemplate string) (string, error) {
	// Build context from event data
	vars := make(map[string]string)
	vars["event_id"] = event.EventID
	vars["source"] = event.Source
	vars["category"] = event.Category
	vars["fetched_at"] = event.FetchedAt.Format(time.RFC3339)

	// Add event data as JSON
	if event.Data != nil {
		dataJSON, err := json.MarshalIndent(event.Data, "", "  ")
		if err == nil {
			vars["event_data"] = string(dataJSON)
		}
	}

	// Expand the prompt template
	prompt := ExpandPrompt(promptTemplate, vars)

	// Invoke Claude CLI
	output, err := invokeClaude(prompt)
	if err != nil {
		return "", fmt.Errorf("Claude invocation failed: %w", err)
	}

	return output, nil
}

// invokeClaude calls the Claude CLI with the given prompt.
// Uses the -p flag for non-interactive mode.
func invokeClaude(prompt string) (string, error) {
	cmd := exec.Command("claude", "-p", prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("claude error: %s", stderr.String())
		}
		return "", fmt.Errorf("claude execution failed: %w", err)
	}

	return stdout.String(), nil
}

// QueueLength returns the number of delayed actions waiting.
func (s *Scheduler) QueueLength() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.queue)
}

// BatchCount returns the number of action batches waiting.
func (s *Scheduler) BatchCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.batches)
}
