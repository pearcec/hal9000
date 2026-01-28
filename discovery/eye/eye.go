// Package eye implements the Mayor's patrol daemon for HAL 9000.
// "Look Dave, I can see you're really upset about this. I honestly think
// you ought to sit down calmly, take a stress pill, and think things over."
//
// The Eye is the Mayor's patrol loop that handles callbacks from Floyd
// watchers, performs health checks on system components, and cleans up
// stale data.
package eye

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultPatrolInterval is how often the patrol loop runs
	DefaultPatrolInterval = 1 * time.Minute

	// DefaultStaleThreshold is how old data must be before cleanup
	DefaultStaleThreshold = 7 * 24 * time.Hour

	// DefaultHealthTimeout is how long to wait for health checks
	DefaultHealthTimeout = 5 * time.Second
)

// Callback represents an event callback from a Floyd watcher.
type Callback struct {
	Source    string                 `json:"source"`
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
	Timestamp time.Time              `json:"timestamp"`
}

// CallbackHandler processes callbacks from Floyd watchers.
type CallbackHandler func(ctx context.Context, cb Callback) error

// HealthStatus represents the health of a component.
type HealthStatus struct {
	Component string    `json:"component"`
	Healthy   bool      `json:"healthy"`
	Message   string    `json:"message,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

// HealthChecker checks the health of a component.
type HealthChecker func(ctx context.Context) HealthStatus

// CleanupResult reports what was cleaned up.
type CleanupResult struct {
	Component    string    `json:"component"`
	ItemsCleaned int       `json:"items_cleaned"`
	BytesFreed   int64     `json:"bytes_freed"`
	CleanedAt    time.Time `json:"cleaned_at"`
	Error        string    `json:"error,omitempty"`
}

// Cleaner performs cleanup for a component.
type Cleaner func(ctx context.Context, threshold time.Time) CleanupResult

// Config configures the Eye patrol daemon.
type Config struct {
	PatrolInterval time.Duration
	StaleThreshold time.Duration
	HealthTimeout  time.Duration
	CallbackDir    string // Directory to watch for callback files
}

// Eye is the Mayor's patrol daemon.
type Eye struct {
	config Config

	// Registered handlers
	callbackHandlers map[string]CallbackHandler // source -> handler
	healthCheckers   map[string]HealthChecker   // component -> checker
	cleaners         map[string]Cleaner         // component -> cleaner

	// State
	mu             sync.RWMutex
	running        bool
	lastPatrol     time.Time
	lastHealth     map[string]HealthStatus
	callbackQueue  []Callback
	callbackQueueMu sync.Mutex

	// Control
	stopCh chan struct{}
	doneCh chan struct{}
}

// New creates a new Eye patrol daemon.
func New(config Config) *Eye {
	if config.PatrolInterval == 0 {
		config.PatrolInterval = DefaultPatrolInterval
	}
	if config.StaleThreshold == 0 {
		config.StaleThreshold = DefaultStaleThreshold
	}
	if config.HealthTimeout == 0 {
		config.HealthTimeout = DefaultHealthTimeout
	}

	return &Eye{
		config:           config,
		callbackHandlers: make(map[string]CallbackHandler),
		healthCheckers:   make(map[string]HealthChecker),
		cleaners:         make(map[string]Cleaner),
		lastHealth:       make(map[string]HealthStatus),
		callbackQueue:    make([]Callback, 0),
	}
}

// RegisterCallbackHandler registers a handler for callbacks from a source.
func (e *Eye) RegisterCallbackHandler(source string, handler CallbackHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.callbackHandlers[source] = handler
	log.Printf("[eye] Registered callback handler for source: %s", source)
}

// RegisterHealthChecker registers a health checker for a component.
func (e *Eye) RegisterHealthChecker(component string, checker HealthChecker) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.healthCheckers[component] = checker
	log.Printf("[eye] Registered health checker for component: %s", component)
}

// RegisterCleaner registers a cleaner for a component.
func (e *Eye) RegisterCleaner(component string, cleaner Cleaner) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cleaners[component] = cleaner
	log.Printf("[eye] Registered cleaner for component: %s", component)
}

// SubmitCallback queues a callback for processing.
func (e *Eye) SubmitCallback(cb Callback) {
	e.callbackQueueMu.Lock()
	defer e.callbackQueueMu.Unlock()
	e.callbackQueue = append(e.callbackQueue, cb)
	log.Printf("[eye] Callback queued from source: %s, type: %s", cb.Source, cb.Type)
}

// Start begins the patrol loop.
func (e *Eye) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return fmt.Errorf("eye already running")
	}
	e.running = true
	e.stopCh = make(chan struct{})
	e.doneCh = make(chan struct{})
	e.mu.Unlock()

	log.Println("[eye] Starting patrol loop...")

	go e.patrolLoop(ctx)

	return nil
}

// Stop halts the patrol loop.
func (e *Eye) Stop() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return
	}
	e.running = false
	close(e.stopCh)
	e.mu.Unlock()

	<-e.doneCh
	log.Println("[eye] Patrol loop stopped")
}

// IsRunning returns whether the patrol loop is active.
func (e *Eye) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// GetHealthStatus returns the last known health status of all components.
func (e *Eye) GetHealthStatus() map[string]HealthStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make(map[string]HealthStatus)
	for k, v := range e.lastHealth {
		result[k] = v
	}
	return result
}

// LastPatrolTime returns when the last patrol completed.
func (e *Eye) LastPatrolTime() time.Time {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastPatrol
}

// patrolLoop is the main daemon loop.
func (e *Eye) patrolLoop(ctx context.Context) {
	defer close(e.doneCh)

	ticker := time.NewTicker(e.config.PatrolInterval)
	defer ticker.Stop()

	// Run initial patrol immediately
	e.patrol(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("[eye] Context cancelled, stopping patrol")
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.patrol(ctx)
		}
	}
}

// patrol performs one patrol cycle: callbacks, health checks, cleanup.
func (e *Eye) patrol(ctx context.Context) {
	log.Println("[eye] Patrol starting...")
	start := time.Now()

	// 1. Process queued callbacks
	e.processCallbacks(ctx)

	// 2. Check callback directory for file-based callbacks
	if e.config.CallbackDir != "" {
		e.processCallbackFiles(ctx)
	}

	// 3. Run health checks
	e.runHealthChecks(ctx)

	// 4. Run cleanup
	e.runCleanup(ctx)

	e.mu.Lock()
	e.lastPatrol = time.Now()
	e.mu.Unlock()

	log.Printf("[eye] Patrol completed in %v", time.Since(start))
}

// processCallbacks handles queued callbacks.
func (e *Eye) processCallbacks(ctx context.Context) {
	e.callbackQueueMu.Lock()
	queue := e.callbackQueue
	e.callbackQueue = make([]Callback, 0)
	e.callbackQueueMu.Unlock()

	if len(queue) == 0 {
		return
	}

	log.Printf("[eye] Processing %d queued callbacks", len(queue))

	e.mu.RLock()
	handlers := make(map[string]CallbackHandler)
	for k, v := range e.callbackHandlers {
		handlers[k] = v
	}
	e.mu.RUnlock()

	for _, cb := range queue {
		handler, ok := handlers[cb.Source]
		if !ok {
			log.Printf("[eye] No handler for callback source: %s", cb.Source)
			continue
		}

		if err := handler(ctx, cb); err != nil {
			log.Printf("[eye] Error handling callback from %s: %v", cb.Source, err)
		}
	}
}

// processCallbackFiles reads and processes callback files from the callback directory.
func (e *Eye) processCallbackFiles(ctx context.Context) {
	dir := expandPath(e.config.CallbackDir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[eye] Error reading callback directory: %v", err)
		}
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("[eye] Error reading callback file %s: %v", entry.Name(), err)
			continue
		}

		var cb Callback
		if err := json.Unmarshal(data, &cb); err != nil {
			log.Printf("[eye] Error parsing callback file %s: %v", entry.Name(), err)
			continue
		}

		e.SubmitCallback(cb)

		// Remove processed file
		if err := os.Remove(path); err != nil {
			log.Printf("[eye] Error removing callback file %s: %v", entry.Name(), err)
		}
	}
}

// runHealthChecks executes all registered health checkers.
func (e *Eye) runHealthChecks(ctx context.Context) {
	e.mu.RLock()
	checkers := make(map[string]HealthChecker)
	for k, v := range e.healthCheckers {
		checkers[k] = v
	}
	e.mu.RUnlock()

	if len(checkers) == 0 {
		return
	}

	log.Printf("[eye] Running %d health checks", len(checkers))

	healthCtx, cancel := context.WithTimeout(ctx, e.config.HealthTimeout)
	defer cancel()

	results := make(map[string]HealthStatus)
	var wg sync.WaitGroup

	resultCh := make(chan HealthStatus, len(checkers))

	for component, checker := range checkers {
		wg.Add(1)
		go func(comp string, check HealthChecker) {
			defer wg.Done()
			status := check(healthCtx)
			status.Component = comp
			status.CheckedAt = time.Now()
			resultCh <- status
		}(component, checker)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	for status := range resultCh {
		results[status.Component] = status
		if !status.Healthy {
			log.Printf("[eye] Health check FAILED: %s - %s", status.Component, status.Message)
		}
	}

	e.mu.Lock()
	e.lastHealth = results
	e.mu.Unlock()
}

// runCleanup executes all registered cleaners.
func (e *Eye) runCleanup(ctx context.Context) {
	e.mu.RLock()
	cleaners := make(map[string]Cleaner)
	for k, v := range e.cleaners {
		cleaners[k] = v
	}
	e.mu.RUnlock()

	if len(cleaners) == 0 {
		return
	}

	threshold := time.Now().Add(-e.config.StaleThreshold)
	log.Printf("[eye] Running cleanup for data older than %v", threshold.Format(time.RFC3339))

	for component, cleaner := range cleaners {
		result := cleaner(ctx, threshold)
		result.Component = component
		result.CleanedAt = time.Now()

		if result.Error != "" {
			log.Printf("[eye] Cleanup error for %s: %s", component, result.Error)
		} else if result.ItemsCleaned > 0 {
			log.Printf("[eye] Cleaned %d items (%d bytes) from %s",
				result.ItemsCleaned, result.BytesFreed, component)
		}
	}
}

// Helper functions

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
