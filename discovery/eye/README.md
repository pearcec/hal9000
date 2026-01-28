# Eye

The Mayor's patrol daemon for HAL 9000.

## Overview

The Eye is a supervisory daemon that performs periodic "patrols" to:

1. **Handle Callbacks** - Process events/callbacks from Floyd watchers
2. **Health Checks** - Monitor the health of system components
3. **Cleanup** - Remove stale data older than a configured threshold

## Usage

```go
package main

import (
    "context"
    "time"

    "github.com/pearcec/hal9000/discovery/eye"
)

func main() {
    // Create eye with custom config
    e := eye.New(eye.Config{
        PatrolInterval: 1 * time.Minute,
        StaleThreshold: 7 * 24 * time.Hour,
        HealthTimeout:  5 * time.Second,
        CallbackDir:    "~/.config/hal9000/callbacks",
    })

    // Register callback handler for Floyd watchers
    e.RegisterCallbackHandler("google-calendar", func(ctx context.Context, cb eye.Callback) error {
        // Process calendar callback
        return nil
    })

    // Register health checker
    e.RegisterHealthChecker("lmc", func(ctx context.Context) eye.HealthStatus {
        // Check LMC health
        return eye.HealthStatus{Healthy: true}
    })

    // Register cleaner
    e.RegisterCleaner("events", func(ctx context.Context, threshold time.Time) eye.CleanupResult {
        // Clean up old events
        return eye.CleanupResult{ItemsCleaned: 10}
    })

    // Start patrol loop
    ctx := context.Background()
    e.Start(ctx)
    defer e.Stop()

    // ... wait for shutdown signal
}
```

## Configuration

| Option | Default | Description |
|--------|---------|-------------|
| `PatrolInterval` | 1 minute | How often the patrol loop runs |
| `StaleThreshold` | 7 days | How old data must be before cleanup |
| `HealthTimeout` | 5 seconds | Timeout for health checks |
| `CallbackDir` | "" | Directory to watch for callback files |

## Callbacks

Callbacks can be submitted programmatically:

```go
e.SubmitCallback(eye.Callback{
    Source:    "google-calendar",
    Type:      "event.created",
    Payload:   map[string]interface{}{"event_id": "abc123"},
    Timestamp: time.Now(),
})
```

Or via JSON files in the callback directory:

```json
{
    "source": "google-calendar",
    "type": "event.created",
    "payload": {"event_id": "abc123"},
    "timestamp": "2026-01-27T10:00:00Z"
}
```

Files are automatically removed after processing.

## 2001: A Space Odyssey Reference

The Eye represents HAL's iconic all-seeing red eye - the perfect name for a watchdog/health monitor that keeps constant vigil over all system components. "The Eye is watching all systems."
