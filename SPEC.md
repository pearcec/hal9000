# Deacon: Personal AI Assistant System

> A daemon-based personal assistant with knowledge graph memory, designed for sharing and extensibility.

## Vision

A personal digital assistant that:
- Runs as persistent background services (not cron jobs)
- Maintains a knowledge graph of your life, work, and interests
- Executes automations based on events, schedules, and triggers
- Understands tasks via a structured specification language
- Can be packaged and shared with others

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         DEACON                                   │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │ Watchers │  │ Workers  │  │  Graph   │  │   API    │        │
│  │          │  │          │  │          │  │          │        │
│  │ - Time   │  │ - Tasks  │  │ - Nodes  │  │ - REST   │        │
│  │ - Files  │  │ - Scripts│  │ - Edges  │  │ - WebSocket│      │
│  │ - Web    │  │ - Plugins│  │ - Query  │  │ - CLI    │        │
│  │ - Events │  │          │  │          │  │          │        │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘        │
│       │             │             │             │               │
│  ┌────┴─────────────┴─────────────┴─────────────┴────┐         │
│  │                    Core Daemon                     │         │
│  │  - Event bus (pub/sub)                            │         │
│  │  - Task queue                                      │         │
│  │  - Plugin lifecycle                                │         │
│  │  - State management                                │         │
│  └───────────────────────────────────────────────────┘         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Plugins                                   │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐   │
│  │Calendar │ │  Email  │ │  Notes  │ │   Web   │ │  Home   │   │
│  │         │ │         │ │         │ │ Research│ │  Auto   │   │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Components

### 1. Core Daemon

The central orchestrator that:
- Manages plugin lifecycle (start, stop, health checks)
- Routes events between components
- Maintains task queue with priorities
- Handles graceful shutdown and recovery

**Implementation**: Single Go binary, low memory footprint (~20-50MB)

### 2. Knowledge Graph

Stores entities, relationships, and context:

```
Entities:
- People (contacts, relationships)
- Projects (tasks, deadlines, status)
- Notes (content, tags, links)
- Events (calendar, reminders)
- Locations (places, addresses)
- Topics (interests, research areas)

Relationships:
- person WORKS_ON project
- note RELATES_TO topic
- event INVOLVES person
- task BLOCKS task
- topic SUBTOPIC_OF topic
```

**Implementation Options**:
| Option | Pros | Cons |
|--------|------|------|
| SQLite + custom | Simple, single file, fast | Manual graph traversal |
| SurrealDB | Graph + document, embedded | Newer, less tooling |
| Neo4j | Mature, great queries | Heavy, requires server |
| EdgeDB | Graph + relational | Requires server |

**Recommendation**: Start with SQLite + custom graph layer, migrate if needed.

### 3. Watchers

Event sources that trigger actions:

| Watcher | Events |
|---------|--------|
| Time | Cron-like schedules, intervals, specific times |
| File | Created, modified, deleted in watched directories |
| Web | Webhooks, RSS feeds, page changes |
| System | Startup, shutdown, network changes |
| Graph | Entity created, relationship changed, query match |

### 4. Workers

Task executors:

- **Script Runner** - Execute shell scripts, Python, etc.
- **HTTP Client** - API calls, webhooks
- **LLM Interface** - Claude API for reasoning tasks
- **Plugin Caller** - Invoke plugin capabilities

### 5. Plugins

Modular integrations:

```yaml
# plugin.yaml
name: calendar
version: 1.0.0
capabilities:
  - list_events
  - create_event
  - update_event
  - delete_event
triggers:
  - event_reminder
  - daily_agenda
config:
  provider: google | apple | caldav
  credentials_path: ~/.deacon/calendar_creds.json
```

**Plugin Communication**: gRPC or stdin/stdout (like MCP)

## Task Specification Language (DSL)

A structured way to define tasks that Claude can parse and execute:

```yaml
# task.deacon.yaml
task: daily_briefing
description: Morning summary of agenda and priorities
trigger:
  type: schedule
  cron: "0 7 * * *"  # 7 AM daily
steps:
  - action: plugin.calendar.list_events
    params:
      range: today
    output: $events

  - action: plugin.email.unread_count
    output: $email_count

  - action: graph.query
    params:
      query: "MATCH (t:Task {status: 'active'}) RETURN t ORDER BY t.priority"
    output: $tasks

  - action: llm.summarize
    params:
      prompt: |
        Create a morning briefing with:
        - Today's events: {{$events}}
        - Unread emails: {{$email_count}}
        - Active tasks: {{$tasks}}
    output: $briefing

  - action: notify
    params:
      title: "Morning Briefing"
      body: "{{$briefing}}"
```

### Natural Language Interface

For conversational requests, Claude translates to the DSL:

```
User: "Remind me to call mom every Sunday at 2pm"

Claude generates:
---
task: call_mom_reminder
trigger:
  type: schedule
  cron: "0 14 * * 0"
steps:
  - action: notify
    params:
      title: "Call Mom"
      body: "Time to call mom!"
---
```

## Deployment

### Docker Compose

```yaml
version: '3.8'
services:
  deacon:
    image: deacon:latest
    container_name: deacon
    restart: unless-stopped
    volumes:
      - ~/.deacon:/data
      - ~/.deacon/plugins:/plugins
    ports:
      - "8420:8420"  # API
    environment:
      - DEACON_LOG_LEVEL=info
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
    healthcheck:
      test: ["CMD", "deacon", "health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

### Directory Structure

```
~/.deacon/
├── config.yaml          # Main configuration
├── deacon.db            # SQLite knowledge graph
├── tasks/               # Task definitions
│   ├── daily_briefing.yaml
│   └── weekly_review.yaml
├── plugins/             # Installed plugins
│   ├── calendar/
│   └── email/
├── logs/                # Log files
└── state/               # Runtime state
```

## CLI Interface

```bash
# Daemon control
deacon start                    # Start daemon
deacon stop                     # Stop daemon
deacon status                   # Check health

# Tasks
deacon task list                # List all tasks
deacon task run <name>          # Run task manually
deacon task create              # Interactive task creation
deacon task enable/disable      # Toggle task

# Graph
deacon graph query "..."        # Query knowledge graph
deacon graph add entity         # Add entity
deacon graph link A B relation  # Create relationship

# Plugins
deacon plugin list              # List plugins
deacon plugin install <name>    # Install plugin
deacon plugin config <name>     # Configure plugin

# Interactive
deacon chat                     # Chat with Claude about your graph/tasks
deacon ask "..."                # One-off question using context
```

## Integration with Claude/Mayor

The system is designed so Claude (as mayor or assistant) can:

1. **Read** the knowledge graph for context
2. **Write** new tasks via the DSL
3. **Query** current state (calendar, tasks, etc.)
4. **Execute** one-off automations
5. **Learn** patterns and suggest automations

```
User: "What do I have going on this week?"

Claude:
1. Queries graph for active projects
2. Calls calendar plugin for events
3. Checks task queue
4. Synthesizes response with full context
```

## Implementation Phases

### Phase 1: Foundation
- [ ] Core daemon with event bus
- [ ] SQLite knowledge graph with basic schema
- [ ] CLI for daemon control
- [ ] File watcher + time scheduler
- [ ] Docker deployment

### Phase 2: Intelligence
- [ ] Task DSL parser and executor
- [ ] Claude API integration
- [ ] Natural language → DSL translation
- [ ] Basic notification system

### Phase 3: Integrations
- [ ] Calendar plugin (Google/Apple)
- [ ] Email plugin (IMAP/Gmail)
- [ ] Notes plugin (filesystem/Obsidian)
- [ ] Web research plugin

### Phase 4: Polish
- [ ] Web UI dashboard
- [ ] Mobile notifications
- [ ] Plugin marketplace/sharing
- [ ] Multi-device sync

## Tech Stack

| Component | Technology | Rationale |
|-----------|------------|-----------|
| Core Daemon | Go | Low memory, single binary, good concurrency |
| Knowledge Graph | SQLite + custom | Simple, embedded, portable |
| Task DSL | YAML | Human readable, Claude-friendly |
| Plugin Protocol | gRPC | Fast, typed, language-agnostic |
| API | REST + WebSocket | Standard, easy to integrate |
| Deployment | Docker | Portable, easy monitoring |
| Config | YAML | Consistent with tasks |

## Open Questions

1. **Graph schema**: How detailed? Start minimal or comprehensive?
2. **Plugin isolation**: Sandboxed containers or trusted processes?
3. **Multi-user**: Single user first, or design for sharing early?
4. **Cloud sync**: Local-only or optional cloud backup?
5. **Name**: Deacon? Something else?

## Name: Deacon

"Deacon" fits:
- Assistant/helper connotation
- Matches Gas Town naming (daemon, mayor)
- Short, memorable
- Available as package name

---

*This spec is a living document. Update as the project evolves.*
