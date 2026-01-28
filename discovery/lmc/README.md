# LMC - Logic Memory Center

HAL 9000's knowledge graph storage.

Named after HAL's Logic Memory Center - the modules Dave Bowman disconnects one by one while HAL sings "Daisy Bell."

## Purpose

The LMC is a document-based knowledge graph where:

- **Folders** = Entity types (people, calendar, jira, slack)
- **Files** = Nodes (individual JSON documents)
- **Links** = Edges (relationships between entities)

## API

```go
import (
    "github.com/pearcec/hal9000/discovery/config"
    "github.com/pearcec/hal9000/discovery/lmc"
)

// Initialize using config-based path
lib, err := lmc.New(config.GetLibraryPath())

// Store an entity
entity, err := lib.Store("people", "john@example.com", content, links)

// Get by ID
entity, err := lib.Get("people/john@example.com")

// Query
results, err := lib.Query(lmc.QueryOptions{
    Type:     "calendar",
    Since:    time.Now().AddDate(0, 0, -7),
    Contains: "standup",
    Limit:    10,
})

// Get linked entities
related, err := lib.GetLinked("people/john@example.com", "both")

// Delete
err := lib.Delete("calendar/event123")

// List entity types
types, err := lib.ListTypes()
```

## Edge Types

| Type | Description |
|------|-------------|
| `scheduled_with` | Person attending a meeting |
| `assigned_to` | Issue assigned to person |
| `belongs_to` | Entity belongs to container |
| `mentions` | Text mentions person/entity |
| `relates_to` | Generic relationship |

## Storage Structure

```
./library/    # Default, or path from ~/.config/hal9000/config.yaml
├── people/
│   └── john_example_com.json
├── calendar/
│   └── 2026-01-27_meeting123.json
├── jira/
│   └── 2026-01-27_PROJ-123.json
└── slack/
    └── 2026-01-27_C123_1234567890.json
```

## Log Format

```
[lmc] Initialized at /path/to/library/
[lmc] Stored entity: people/john@example.com
[lmc] Rebuilding edge index...
```

## Current Limitations

- File-based storage (no database)
- In-memory edge index (rebuilt on startup)
- No complex graph traversals (shortest path, etc.)

Future: May migrate to SQLite, DuckDB, or a proper graph database.
