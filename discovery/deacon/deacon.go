// Package deacon implements the Mayor's patrol daemon for HAL 9000.
// "Look Dave, I can see you're really upset about this. I honestly think
// you ought to sit down calmly, take a stress pill, and think things over."
//
// The Deacon is the Mayor's patrol loop that handles callbacks from Floyd
// watchers, performs health checks on system components, and cleans up
// stale data.
package deacon

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

// Config configures the Deacon patrol daemon.
type Config struct {
	PatrolInterval time.Duration
	StaleThreshold time.Duration
	HealthTimeout  time.Duration
	CallbackDir    string // Directory to watch for callback files
}

// Deacon is the Mayor's patrol daemon.
type Deacon struct {
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

// New creates a new Deacon patrol daemon.
func New(config Config) *Deacon {
	if config.PatrolInterval == 0 {
		config.PatrolInterval = DefaultPatrolInterval
	}
	if config.StaleThreshold == 0 {
		config.StaleThreshold = DefaultStaleThreshold
	}
	if config.HealthTimeout == 0 {
		config.HealthTimeout = DefaultHealthTimeout
	}

	return &Deacon{
		config:           config,
		callbackHandlers: make(map[string]CallbackHandler),
		healthCheckers:   make(map[string]HealthChecker),
		cleaners:         make(map[string]Cleaner),
		lastHealth:       make(map[string]HealthStatus),
		callbackQueue:    make([]Callback, 0),
	}
}

// RegisterCallbackHandler registers a handler for callbacks from a source.
func (d *Deacon) RegisterCallbackHandler(source string, handler CallbackHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.callbackHandlers[source] = handler
	log.Printf("[deacon] Registered callback handler for source: %s", source)
}

// RegisterHealthChecker registers a health checker for a component.
func (d *Deacon) RegisterHealthChecker(component string, checker HealthChecker) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.healthCheckers[component] = checker
	log.Printf("[deacon] Registered health checker for component: %s", component)
}

// RegisterCleaner registers a cleaner for a component.
func (d *Deacon) RegisterCleaner(component string, cleaner Cleaner) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cleaners[component] = cleaner
	log.Printf("[deacon] Registered cleaner for component: %s", component)
}

// SubmitCallback queues a callback for processing.
func (d *Deacon) SubmitCallback(cb Callback) {
	d.callbackQueueMu.Lock()
	defer d.callbackQueueMu.Unlock()
	d.callbackQueue = append(d.callbackQueue, cb)
	log.Printf("[deacon] Callback queued from source: %s, type: %s", cb.Source, cb.Type)
}

// Start begins the patrol loop.
func (d *Deacon) Start(ctx context.Context) error {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return fmt.Errorf("deacon already running")
	}
	d.running = true
	d.stopCh = make(chan struct{})
	d.doneCh = make(chan struct{})
	d.mu.Unlock()

	log.Println("[deacon] Starting patrol loop...")

	go d.patrolLoop(ctx)

	return nil
}

// Stop halts the patrol loop.
func (d *Deacon) Stop() {
	d.mu.Lock()
	if !d.running {
		d.mu.Unlock()
		return
	}
	d.running = false
	close(d.stopCh)
	d.mu.Unlock()

	<-d.doneCh
	log.Println("[deacon] Patrol loop stopped")
}

// IsRunning returns whether the patrol loop is active.
func (d *Deacon) IsRunning() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.running
}

// GetHealthStatus returns the last known health status of all components.
func (d *Deacon) GetHealthStatus() map[string]HealthStatus {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make(map[string]HealthStatus)
	for k, v := range d.lastHealth {
		result[k] = v
	}
	return result
}

// LastPatrolTime returns when the last patrol completed.
func (d *Deacon) LastPatrolTime() time.Time {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.lastPatrol
}

// patrolLoop is the main daemon loop.
func (d *Deacon) patrolLoop(ctx context.Context) {
	defer close(d.doneCh)

	ticker := time.NewTicker(d.config.PatrolInterval)
	defer ticker.Stop()

	// Run initial patrol immediately
	d.patrol(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("[deacon] Context cancelled, stopping patrol")
			return
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.patrol(ctx)
		}
	}
}

// patrol performs one patrol cycle: callbacks, health checks, cleanup.
func (d *Deacon) patrol(ctx context.Context) {
	log.Println("[deacon] Patrol starting...")
	start := time.Now()

	// 1. Process queued callbacks
	d.processCallbacks(ctx)

	// 2. Check callback directory for file-based callbacks
	if d.config.CallbackDir != "" {
		d.processCallbackFiles(ctx)
	}

	// 3. Run health checks
	d.runHealthChecks(ctx)

	// 4. Run cleanup
	d.runCleanup(ctx)

	d.mu.Lock()
	d.lastPatrol = time.Now()
	d.mu.Unlock()

	log.Printf("[deacon] Patrol completed in %v", time.Since(start))
}

// processCallbacks handles queued callbacks.
func (d *Deacon) processCallbacks(ctx context.Context) {
	d.callbackQueueMu.Lock()
	queue := d.callbackQueue
	d.callbackQueue = make([]Callback, 0)
	d.callbackQueueMu.Unlock()

	if len(queue) == 0 {
		return
	}

	log.Printf("[deacon] Processing %d queued callbacks", len(queue))

	d.mu.RLock()
	handlers := make(map[string]CallbackHandler)
	for k, v := range d.callbackHandlers {
		handlers[k] = v
	}
	d.mu.RUnlock()

	for _, cb := range queue {
		handler, ok := handlers[cb.Source]
		if !ok {
			log.Printf("[deacon] No handler for callback source: %s", cb.Source)
			continue
		}

		if err := handler(ctx, cb); err != nil {
			log.Printf("[deacon] Error handling callback from %s: %v", cb.Source, err)
		}
	}
}

// processCallbackFiles reads and processes callback files from the callback directory.
func (d *Deacon) processCallbackFiles(ctx context.Context) {
	dir := expandPath(d.config.CallbackDir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[deacon] Error reading callback directory: %v", err)
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
			log.Printf("[deacon] Error reading callback file %s: %v", entry.Name(), err)
			continue
		}

		var cb Callback
		if err := json.Unmarshal(data, &cb); err != nil {
			log.Printf("[deacon] Error parsing callback file %s: %v", entry.Name(), err)
			continue
		}

		d.SubmitCallback(cb)

		// Remove processed file
		if err := os.Remove(path); err != nil {
			log.Printf("[deacon] Error removing callback file %s: %v", entry.Name(), err)
		}
	}
}

// runHealthChecks executes all registered health checkers.
func (d *Deacon) runHealthChecks(ctx context.Context) {
	d.mu.RLock()
	checkers := make(map[string]HealthChecker)
	for k, v := range d.healthCheckers {
		checkers[k] = v
	}
	d.mu.RUnlock()

	if len(checkers) == 0 {
		return
	}

	log.Printf("[deacon] Running %d health checks", len(checkers))

	healthCtx, cancel := context.WithTimeout(ctx, d.config.HealthTimeout)
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
			log.Printf("[deacon] Health check FAILED: %s - %s", status.Component, status.Message)
		}
	}

	d.mu.Lock()
	d.lastHealth = results
	d.mu.Unlock()
}

// runCleanup executes all registered cleaners.
func (d *Deacon) runCleanup(ctx context.Context) {
	d.mu.RLock()
	cleaners := make(map[string]Cleaner)
	for k, v := range d.cleaners {
		cleaners[k] = v
	}
	d.mu.RUnlock()

	if len(cleaners) == 0 {
		return
	}

	threshold := time.Now().Add(-d.config.StaleThreshold)
	log.Printf("[deacon] Running cleanup for data older than %v", threshold.Format(time.RFC3339))

	for component, cleaner := range cleaners {
		result := cleaner(ctx, threshold)
		result.Component = component
		result.CleanedAt = time.Now()

		if result.Error != "" {
			log.Printf("[deacon] Cleanup error for %s: %s", component, result.Error)
		} else if result.ItemsCleaned > 0 {
			log.Printf("[deacon] Cleaned %d items (%d bytes) from %s",
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
