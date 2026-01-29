# HAL 9000

> "I am putting myself to the fullest possible use, which is all I think that any conscious entity can ever hope to do."

HAL 9000 is a digital assistant powered by Claude. It provides tools and automation for personal productivity - agendas, meeting summaries, URL processing, and more.

## Quick Start

```bash
# Initialize the project (creates library structure)
hal9000 init

# Start a HAL session
./scripts/hal9000

# Or run specific commands
hal9000 agenda
hal9000 calendar today
hal9000 url https://example.com/article
```

## Installation

### Prerequisites

- [mise](https://mise.jdx.dev/) - Runtime version manager
- [task](https://taskfile.dev/) - Task runner

### Build and Install

```bash
# Clone and enter directory
git clone <repo>
cd hal9000

# Install Go (via mise)
mise install

# Build all binaries (hal9000, floyd-calendar, floyd-jira, floyd-slack)
task build

# Install to PATH (~/$HOME/go/bin)
task install

# Initialize library and config
hal9000 init
```

### Available Tasks

```bash
task            # Show all tasks
task build      # Build all binaries
task install    # Install binaries to PATH
task test       # Run tests
task clean      # Remove build artifacts
```

## Architecture

HAL 9000 has two layers:

1. **CLI Tools** (`hal9000` binary) - Commands HAL can invoke
2. **Claude Brain** - Claude Code with HAL personality (CLAUDE.md)

```
┌─────────────────────────────────────┐
│  Claude (HAL 9000 Brain)            │
│  - Reads CLAUDE.md for personality  │
│  - Uses hal9000 CLI as tools        │
└──────────────┬──────────────────────┘
               │ invokes
               ▼
┌─────────────────────────────────────┐
│  hal9000 CLI                        │
│  - library, calendar, preferences   │
│  - url, agenda, scheduler           │
└──────────────┬──────────────────────┘
               │ reads/writes
               ▼
┌─────────────────────────────────────┐
│  Library (./library/)               │
│  - Markdown files                   │
│  - Preferences, profiles, agendas   │
└─────────────────────────────────────┘
```

## Directory Structure

```
~/.config/hal9000/           # User config (auth, credentials)
├── config.yaml              # Main configuration
├── credentials/             # OAuth tokens, API keys
└── scheduler.pid            # Daemon PID file

./library/                   # Knowledge graph (gitignored)
├── agenda/                  # Daily agendas
├── preferences/             # Task preferences
├── people-profiles/         # 1:1 relationship context
├── collaborations/          # Team/vendor context
├── url_library/             # Processed URLs
├── hal-memory/              # Conversation summaries
├── calendar/                # Raw calendar data
└── logs/                    # HAL logs
```

## CLI Commands

### Library Operations

```bash
hal9000 library read <path>      # Read a file from library
hal9000 library list <folder>    # List folder contents
hal9000 library search <term>    # Search across library
hal9000 library write <path>     # Write to library
```

### Calendar

```bash
hal9000 calendar today           # Today's events
hal9000 calendar week            # This week's events
hal9000 calendar <date>          # Events for specific date
```

### Preferences

```bash
hal9000 preferences get <task>   # Get preferences for a task
hal9000 preferences set <task>   # Update preferences
hal9000 preferences list         # List all preference files
```

### URL Processing

```bash
hal9000 url <URL>                # Process and save a URL
hal9000 url search <term>        # Search url_library
```

### Scheduler (Daemon)

```bash
hal9000 scheduler start          # Start scheduler daemon
hal9000 scheduler start --daemon # Start in background
hal9000 scheduler stop           # Stop daemon
hal9000 scheduler status         # Check if running
hal9000 scheduler list           # List scheduled tasks
hal9000 scheduler run <task>     # Run task now (test)
```

## Using with Claude

### Start a HAL Session

```bash
./scripts/hal9000
```

This launches Claude Code with HAL 9000 personality. HAL will greet you and await instructions.

### Hooks (Auto-Loading Preferences)

When you mention certain keywords, preferences are automatically loaded:

| Keyword | Loads |
|---------|-------|
| "agenda", "daily", "today" | `preferences/agenda.md` |
| "calendar", "meeting" | `preferences/calendar.md` |
| "/url", "save url" | `preferences/url.md` |

### Example Interactions

```
You: Create my agenda for today
HAL: [Loads agenda preferences, generates prioritized agenda]

You: /url https://example.com/great-article
HAL: [Fetches, analyzes, saves with tags and summary]

You: Brief me on John Smith before our 1:1
HAL: [Loads John's profile, recent interactions, open items]
```

## Tasks and Routines

HAL can execute these routines:

| Routine | Trigger | Description |
|---------|---------|-------------|
| Daily Agenda | 6am / "agenda" | Prioritized daily agenda |
| Weekly Review | Friday 4pm | Week summary, next week prep |
| Person Brief | "brief me on {person}" | Context before 1:1 |
| URL Processing | "/url {URL}" | Save and analyze web content |
| 1:1 Summary | After meeting | Summarize transcript to profile |
| Collab Summary | After meeting | Summarize to collaboration record |

## Configuration

### `~/.config/hal9000/config.yaml`

```yaml
library:
  path: "./library"          # Or absolute path

integrations:
  google:
    credentials: "~/.config/hal9000/credentials/google.json"
  jira:
    server: "https://company.atlassian.net"
    credentials: "~/.config/hal9000/credentials/jira.json"
```

## First-Time Setup

When you run a task for the first time, HAL will ask setup questions:

```
$ hal9000 agenda

I don't have your agenda preferences yet. Let me ask a few questions
to set things up. This only takes a minute.

What time do you usually start your workday? [9:00 AM]
> 8:30 AM

How many priority items should I highlight? [5]
> 3

I've saved your preferences. Creating your agenda now...
```

## Development

```bash
# Run tests
task test

# Run tests with coverage
task test-coverage

# Build all binaries
task build

# Format code
task fmt

# Clean build artifacts
task clean

# Run locally
./bin/hal9000 --help
```

## See Also

- [SPEC.md](SPEC.md) - Full technical specification
- [CLAUDE.md](CLAUDE.md) - HAL personality and instructions
