# Bowman

Fetch and store layer for HAL 9000.

Named after Dave Bowman, who went out to fetch the AE-35 unit in 2001: A Space Odyssey.

## Purpose

Bowman handles data retrieval and storage. When Floyd watchers detect changes, they call Bowman to:

1. Fetch the full data from the source
2. Store it in the library with proper metadata
3. Handle storage rules (inline vs pointer for large documents)

## API

```go
import (
    "github.com/pearcec/hal9000/discovery/bowman"
    "github.com/pearcec/hal9000/discovery/config"
)

// Configure storage location using config
cfg := bowman.StoreConfig{
    LibraryPath: config.GetLibraryPath(),
    Category:    "calendar",
}

// Store a raw event
event := bowman.RawEvent{
    Source:    "google-calendar",
    EventID:   "abc123",
    FetchedAt: time.Now(),
    Stage:     "raw",
    Data:      map[string]interface{}{...},
}
path, err := bowman.Store(config, event)

// Delete an event
err := bowman.Delete(config, "abc123")
```

## Storage Rules (from SPEC.md)

- **< 1kb**: Store inline (full content in JSON)
- **≥ 1kb**: Store as pointer/reference (TBD)

## Log Format

```
[bowman][fetch] Stored raw event: calendar_2026-01-27_abc123.json
[bowman][fetch] Large event (2048 bytes), storing full document
```
