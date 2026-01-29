// Package main implements the Poole service for HAL 9000.
// "I'm completely operational, and all my circuits are functioning perfectly."
//
// Poole is the event dispatcher that connects Floyd (watchers) and Bowman
// (fetchers) to Claude actions. It monitors event files from Floyd and
// dispatches appropriate actions based on the action registry.
package main

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pearcec/hal9000/discovery/config"
	"github.com/pearcec/hal9000/discovery/events"
	"github.com/pearcec/hal9000/discovery/poole"
)

const (
	// pollInterval is how often to check for new events.
	pollInterval = 5 * time.Second
)

// FloydEvent matches the event structure emitted by Floyd watchers.
type FloydEvent struct {
	Source    string    `json:"source"`
	Type      string    `json:"type"`
	Payload   string    `json:"payload"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {
	log.Println("[poole] HAL 9000 Poole dispatcher initializing...")

	// Load configuration
	cfg, err := poole.LoadConfig()
	if err != nil {
		log.Printf("[poole] Warning: failed to load config, using defaults: %v", err)
		cfg = poole.DefaultConfig()
	}

	if !cfg.Enabled {
		log.Println("[poole] Poole is disabled in configuration. Exiting.")
		return
	}

	// Create registry
	registry := poole.NewRegistry()

	// Add prompt paths (defaults first, then user overrides)
	registry.AddPromptPath(cfg.DefaultPromptsPath)
	registry.AddPromptPath(cfg.UserPromptsPath)

	// Load prompts
	if err := registry.LoadPrompts(); err != nil {
		log.Printf("[poole] Warning: failed to load prompts: %v", err)
	}

	// Load actions if config exists
	if _, err := os.Stat(cfg.ActionsPath); err == nil {
		if err := registry.LoadActions(cfg.ActionsPath); err != nil {
			log.Fatalf("[poole] Failed to load actions: %v", err)
		}
		log.Printf("[poole] Loaded %d actions", len(registry.ListActions()))
	} else {
		log.Println("[poole] No actions.yaml found, running with empty action registry")
	}

	// Create scheduler
	scheduler := poole.NewScheduler()
	scheduler.Start()
	defer scheduler.Stop()

	// Create dispatcher
	dispatcher := poole.NewDispatcher(registry, scheduler)

	// Create event bus
	bus := events.NewBus(100)
	defer bus.Close()

	// Connect dispatcher to bus
	dispatcher.Connect(bus)

	log.Println("[poole] Poole online. Monitoring Floyd events...")

	// Track file positions for incremental reading
	positions := make(map[string]int64)

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Main event loop
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sigCh:
			log.Println("[poole] Received shutdown signal")
			dispatcher.Stop()
			return
		case <-ticker.C:
			processEventFiles(bus, positions)
		}
	}
}

// processEventFiles reads new events from Floyd event files.
func processEventFiles(bus *events.Bus, positions map[string]int64) {
	runtimeDir := config.GetRuntimeDir()

	// Look for Floyd event files
	eventFiles := []string{
		filepath.Join(runtimeDir, "calendar-events.jsonl"),
		filepath.Join(runtimeDir, "jira-events.jsonl"),
		filepath.Join(runtimeDir, "slack-events.jsonl"),
	}

	for _, path := range eventFiles {
		readNewEvents(path, bus, positions)
	}
}

// readNewEvents reads new events from a JSONL file starting from the last position.
func readNewEvents(path string, bus *events.Bus, positions map[string]int64) {
	file, err := os.Open(path)
	if err != nil {
		// File doesn't exist yet, that's fine
		return
	}
	defer file.Close()

	// Get current file size
	info, err := file.Stat()
	if err != nil {
		return
	}

	// Get last read position
	lastPos := positions[path]
	currentSize := info.Size()

	// No new data
	if currentSize <= lastPos {
		return
	}

	// Seek to last position
	if lastPos > 0 {
		_, err = file.Seek(lastPos, 0)
		if err != nil {
			log.Printf("[poole] Error seeking in %s: %v", path, err)
			return
		}
	}

	// Read new lines
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var floydEvent FloydEvent
		if err := json.Unmarshal(line, &floydEvent); err != nil {
			log.Printf("[poole] Error parsing event: %v", err)
			continue
		}

		// Convert to storage event for the dispatcher
		storageEvent := events.StorageEvent{
			Type:      events.EventStore,
			Source:    floydEvent.Source,
			EventID:   floydEvent.Payload,
			FetchedAt: floydEvent.Timestamp,
			Category:  floydEvent.Source,
			Data: map[string]interface{}{
				"event_type": floydEvent.Type,
				"payload":    floydEvent.Payload,
			},
		}

		log.Printf("[poole] Received Floyd event: %s:%s (%s)", floydEvent.Source, floydEvent.Type, floydEvent.Payload)

		// Publish to bus (dispatcher will handle it)
		bus.Publish(storageEvent)
	}

	// Update position
	newPos, _ := file.Seek(0, 1) // Get current position
	positions[path] = newPos
}
