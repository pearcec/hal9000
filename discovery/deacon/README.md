# Deacon

The Mayor's patrol daemon for HAL 9000.

## Overview

The Deacon is a supervisory daemon that performs periodic "patrols" to:

1. **Handle Callbacks** - Process events/callbacks from Floyd watchers
2. **Health Checks** - Monitor the health of system components
3. **Cleanup** - Remove stale data older than a configured threshold

## Usage

```go
package main

import (
    "context"
    "time"

    "github.com/pearcec/hal9000/discovery/deacon"
)

func main() {
    // Create deacon with custom config
    d := deacon.New(deacon.Config{
        PatrolInterval: 1 * time.Minute,
        StaleThreshold: 7 * 24 * time.Hour,
        HealthTimeout:  5 * time.Second,
        CallbackDir:    "~/.config/hal9000/callbacks",
    })

    // Register callback handler for Floyd watchers
    d.RegisterCallbackHandler("google-calendar", func(ctx context.Context, cb deacon.Callback) error {
        // Process calendar callback
        return nil
    })

    // Register health checker
    d.RegisterHealthChecker("lmc", func(ctx context.Context) deacon.HealthStatus {
        // Check LMC health
        return deacon.HealthStatus{Healthy: true}
    })

    // Register cleaner
    d.RegisterCleaner("events", func(ctx context.Context, threshold time.Time) deacon.CleanupResult {
        // Clean up old events
        return deacon.CleanupResult{ItemsCleaned: 10}
    })

    // Start patrol loop
    ctx := context.Background()
    d.Start(ctx)
    defer d.Stop()

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
d.SubmitCallback(deacon.Callback{
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

The name "Deacon" is fitting for a patrol/maintenance daemon - keeping watch over the system like a diligent officer making their rounds on the Discovery One spacecraft.
