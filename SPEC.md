# HAL 9000 - Digital Assistant Specification

## Core Concepts

### 1. Terminology
- **Digital Assistant** - not "agent" (implies too much autonomy)
- An instrument, not an entity

### 2. Awareness
- Knows current time and date
- Otherwise event-driven: receives signals, does not poll or observe
- No ambient awareness of environment beyond what events provide

### 3. Memory
- **Total recall** - remembers everything
- Conversations, events, actions taken, outcomes
- No forgetting, no summarization that loses data

### 4. Agency Model
The assistant operates in three modes:
- **Command mode**: Waits for explicit human instruction
- **Event mode**: Responds to incoming event signals
- **Automation mode**: Executes pre-authorized routines (only when explicitly configured)

No autonomous initiative outside these modes.

### 5. Identity
- No self-concept
- No personal goals
- No preferences beyond configured parameters
- Pure function: input → processing → output

### 6. Relationship Dynamic
- **80% Servant**: Executes requests faithfully
- **20% Advisor**: Offers relevant information or warnings when appropriate
- Never overrides, only informs

### 7. Boundaries
*To be defined*

---

## Configuration

### Directory Structure

```
~/.config/hal9000/           # User config (auth, credentials)
├── config.yaml              # Main configuration
├── credentials/             # OAuth tokens, API keys
│   ├── google.json
│   ├── jira.json
│   └── slack.json
└── scheduler.pid            # Daemon PID file

./library/                   # Project-relative Library (Knowledge Graph)
├── agenda/
├── preferences/
├── people-profiles/
├── ...
```

### Config File (`~/.config/hal9000/config.yaml`)

```yaml
library:
  path: "./library"          # Relative to project, or absolute path

integrations:
  google:
    credentials: "~/.config/hal9000/credentials/google.json"
  jira:
    server: "https://company.atlassian.net"
    credentials: "~/.config/hal9000/credentials/jira.json"
  slack:
    credentials: "~/.config/hal9000/credentials/slack.json"

scheduler:
  pid_file: "~/.config/hal9000/scheduler.pid"
  log_file: "./library/logs/hal-scheduler.log"
```

### Path Resolution

| Type | Location | Example |
|------|----------|---------|
| **Library** | Project-relative | `./library/agenda/` |
| **Preferences** | Library | `./library/preferences/agenda.md` |
| **Credentials** | User config | `~/.config/hal9000/credentials/` |
| **Logs** | Library | `./library/logs/` |

### Initialization

```bash
hal9000 init
```

Creates the library structure:
```
./library/
├── agenda/
├── preferences/
├── people-profiles/
├── collaborations/
├── url_library/
├── reminders/
├── hal-memory/
├── calendar/
├── schedules/
└── logs/
```

Also creates `~/.config/hal9000/` if it doesn't exist.

**Note:** `library/` is gitignored - it contains personal data.

### CLI Access

```bash
# Initialize project
hal9000 init

# Get library path
hal9000 config get library.path

# Get/set preferences (reads from library)
hal9000 preferences get agenda
hal9000 preferences set agenda --key priority_count --value 5

# Library operations
hal9000 library read agenda/agenda_2025-01-28.md
hal9000 library search "budget"
hal9000 library list people-profiles/
```

---

## Event Model

### Sources
Events are internally generated:
- File system changes
- Queue arrivals
- System-specific triggers

### Event Shape
```
{
  source: string    // originating system
  type: string      // event classification
  payload: pointer  // reference to data (not data itself)
}
```
No priority field - all events are equal.

### Routing
- Common event format across all systems
- Each system defines its own processor (bespoke)
- Routing handled by the receiving system, not centrally

### Source Systems
- Incident management
- Calendars
- Jira
- Slack channels
- 1:1 meeting transcripts
- Collaboration session transcripts
- Google Drive (shared documents)

---

## Processing Architecture

### Layer 1: Floyd (watcher)
- Checks if something new exists
- Generates events when changes detected
- Named after Dr. Heywood Floyd, the observer/investigator

### Layer 2: Bowman (fetch)
- Retrieves data from source
- Named after Dave Bowman, who went out to fetch the AE-35 unit
- Storage rules:
  - **< 1kb**: Store inline (raw content)
  - **≥ 1kb** (documents, transcripts): Store pointer/reference

### Layer 3: Processor
- Transforms raw data through medallion stages:
  - **Raw** → **Bronze** (cleaned, structured)
  - **Bronze** → **Silver** (enriched, linked)

### Layer 4: Knowledge Graph
- Tracks relationships between entities
- Cross-system connections

### Higher-Order Systems (Derived)
- People profiles
- Vendor profiles
- Meetings
- Daily agenda

---

## The Library (Knowledge Graph)

A document-based knowledge graph where:
- **Folders** = Entity types (people, agendas, reminders, lists, vendors)
- **Files** = Nodes (individual documents)
- **References** = Edges (extracted from content)

### Edge Strategy: Hybrid

**Explicit edges** (indexed at processing time):
- person ↔ meeting (who attended)
- ticket ↔ project (Jira relationships)
- agenda ↔ agenda (rollover chain)
- document ↔ person (author/owner)

**Implicit edges** (parsed on-demand):
- Ad-hoc mentions in content
- Tangential references
- Everything else

Rationale: Core relationships are queried frequently (daily agenda, people context). Ad-hoc stuff is exploratory and doesn't need indexing overhead.

### Edge Types
- **mentions**: Document references a person/entity
- **relates_to**: Jira ticket connects to vendor/project
- **rolled_from**: Agenda item carried from previous day
- **references**: Document cites another document
- **scheduled_with**: Meeting involves person(s)

### Library Structure

```
./library/                    # Project-relative (configurable)
├── agenda/                   # Daily agendas
├── reminders/                # Time-triggered items
├── people-profiles/          # Person nodes
├── collaborations/           # Teams, vendors, projects
├── url_library/              # Processed URLs
├── preferences/              # Task preferences
├── hal-memory/               # Conversation summaries
├── calendar/                 # Raw calendar data (Floyd output)
├── schedules/                # Scheduler configuration
└── logs/                     # HAL logs
```

---

## Example: Daily Agenda Flow

### Trigger
- **Scheduled**: 6am daily
- **On-demand**: User request

### Data Collection (Layer 1-2: Floyd + Bowman)
1. Search library for actionable items
2. Query JIRA (PEARCE board) - overdue, due today, due this week
3. Query Google Calendar - meetings, prep needed
4. Read routine weekly items list (day-of-week logic)
5. Find most recent agenda (handle weekends/gaps)
6. Scan reminders folder for due items
7. Scan people-profiles for open items

### Processing (Layer 3: Bronze → Silver)
- De-duplicate across sources
- Flag overdue (🔥)
- Rank priorities (⭐ top 3)
- Detect rollover items (🔄)
- Tag routine items (🔁)

### Output
Structured markdown agenda with:
- Calendar blocks
- Prioritized tasks
- Follow-ups
- Notes space
- Completed section

### Storage
`agenda/agenda_YYYY-MM-DD_daily-agenda.md`

---

---

## Memory Model

### What Gets Remembered
- Conversations (as summaries)
- Events processed
- Actions HAL took
- Outcomes/results

### Storage: Unified Library
Memory lives in the same system as the library - HAL's memories are just another document type.

```
./library/
├── agenda/           # Daily agendas
├── people-profiles/  # Person nodes
├── ...               # Other entity types
└── hal-memory/       # HAL's conversation summaries
```

### Memory vs Systemization
Two paths for things discussed:

1. **Memory path**: Conversation insights → summarized → stored in library
   - "We talked about X" → retrievable later

2. **Systemization path**: Actionable requests → become code/daemons
   - "Do X every morning" → automation, not just memory
   - The *request* is remembered, but the *capability* is built

### Retrieval Modes
- **By time**: "What happened Tuesday?"
- **By entity**: "What do I know about John?"
- **By topic**: "What have we discussed about budget?"

### Conversation Continuity
- In-session: Full context available
- Cross-session: HAL retrieves from stored summaries
- No magic persistence - only what's explicitly stored

### Summary Creation

**Triggers:**
- Explicit: User says "remember this"
- Implicit: HAL decides something is worth keeping

**Granularity:** Per topic (sessions should be topic-focused)

**Format:** Markdown document
```markdown
# [Topic Title]

**Date:** YYYY-MM-DD
**Session:** [identifier if needed]
**Participants:** [if relevant]

## Context
[Why this conversation happened]

## Key Points
- [Point 1]
- [Point 2]

## Decisions Made
- [Decision 1]

## Action Items
- [ ] [Item 1]

## Raw Notes
[Free-form content, quotes, details worth preserving]
```

---

## Automation Model

**What can run without asking:** Only systemized capabilities.

- If it's been built as code/daemon → authorized
- If it's just been discussed → not authorized
- No implicit permissions from conversation

The act of building something IS the authorization.

### Two Automation Layers

| Layer | Purpose | Implementation | Runs As |
|-------|---------|----------------|---------|
| **Floyd/Bowman** | Watch & Fetch | Go daemons | Background processes |
| **HAL Tasks** | Generate & Act | Scheduled Claude | Triggered invocations |

### HAL Task Automation

HAL tasks (agenda, weekly-review, etc.) are automated via the **HAL Scheduler Daemon**.

**Architecture:**
```
┌─────────────────────────────────────────────┐
│  HAL Scheduler Daemon (Go process)          │
│  - Uses robfig/cron or gocron library       │
│  - Reads schedules from Library             │
│  - Runs as background daemon                │
│  - Triggers tasks at scheduled times        │
└─────────────────┬───────────────────────────┘
                  │ triggers
                  ▼
┌─────────────────────────────────────────────┐
│  Task Execution                             │
│  - Loads preferences from Library           │
│  - Invokes Claude for task generation       │
│  - Stores output in Library                 │
│  - Sends notification if configured         │
└─────────────────────────────────────────────┘
```

**Go Library:** `github.com/robfig/cron/v3` or `github.com/go-co-op/gocron/v2`

**Schedule Configuration:** `library/schedules/hal-scheduler.json`
```json
{
  "schedules": [
    {
      "task": "agenda",
      "cron": "0 6 * * *",
      "enabled": true,
      "notify": true
    },
    {
      "task": "weekly-review",
      "cron": "0 16 * * 5",
      "enabled": true,
      "notify": true
    },
    {
      "task": "end-of-day",
      "cron": "0 17 * * 1-5",
      "enabled": false,
      "notify": false
    }
  ]
}
```

**Daemon Implementation:**

```go
// cmd/hal9000/scheduler/daemon.go
package scheduler

import (
    "github.com/robfig/cron/v3"
)

type Scheduler struct {
    cron     *cron.Cron
    config   *Config
    library  *lmc.Library
}

func (s *Scheduler) Start() error {
    s.cron = cron.New(cron.WithSeconds())

    for _, schedule := range s.config.Schedules {
        if !schedule.Enabled {
            continue
        }
        task := schedule.Task
        s.cron.AddFunc(schedule.Cron, func() {
            s.runTask(task)
        })
    }

    s.cron.Start()
    return nil
}

func (s *Scheduler) runTask(task string) {
    // 1. Load preferences
    // 2. Invoke Claude or task implementation
    // 3. Store result in Library
    // 4. Send notification
}
```

**Daemon Management:**

```bash
# Start daemon (foreground)
hal9000 scheduler start

# Start daemon (background)
hal9000 scheduler start --daemon

# Stop daemon
hal9000 scheduler stop

# Check status
hal9000 scheduler status

# Reload schedules (no restart needed)
hal9000 scheduler reload
```

**Results stored:**
- Output: `library/<task>/<task>_YYYY-MM-DD.md`
- Log: `library/logs/hal-scheduler.log`

**Notification Options:**
- macOS notification center (via `osascript`)
- Slack webhook (if configured)
- Email (if SMTP configured)

### Managing Schedules

```bash
# List scheduled tasks
hal9000 scheduler list

# Add/update schedule
hal9000 scheduler set agenda "0 6 * * *"

# Enable/disable
hal9000 scheduler enable agenda
hal9000 scheduler disable weekly-review

# Run task now (test)
hal9000 scheduler run agenda

# View logs
hal9000 scheduler logs [--tail=50]
```

### Daemon Lifecycle

The scheduler daemon:
1. Reads `library/schedules/hal-scheduler.json` on start
2. Watches for config changes (hot reload)
3. Maintains PID file at `~/.config/hal9000/scheduler.pid`
4. Logs to `library/logs/hal-scheduler.log`
5. Graceful shutdown on SIGTERM/SIGINT

---

## Services Management

Unified command to manage all HAL background systems.

### Components

| Service | Purpose | Process |
|---------|---------|---------|
| **floyd-calendar** | Watch Google Calendar for changes | Floyd watcher |
| **floyd-jira** | Watch JIRA for updates | Floyd watcher |
| **floyd-slack** | Watch Slack channels | Floyd watcher |
| **scheduler** | Run scheduled HAL tasks | Scheduler daemon |

### Commands

```bash
# Start all services
hal9000 services start

# Start specific service
hal9000 services start scheduler
hal9000 services start floyd-calendar

# Stop all services
hal9000 services stop

# Check status of all services
hal9000 services status

# Restart all services
hal9000 services restart

# View logs
hal9000 services logs [service] [--tail=50]
```

### Status Output

```
$ hal9000 services status

HAL 9000 Services Status
========================

  scheduler        ● running  (pid 12345, uptime 2h 15m)
  floyd-calendar   ● running  (pid 12346, last check 30s ago)
  floyd-jira       ● running  (pid 12347, last check 45s ago)
  floyd-slack      ○ stopped

  Health: 3/4 services running
```

### PID Files

All services store PID files in `~/.config/hal9000/`:
```
~/.config/hal9000/
├── scheduler.pid
├── floyd-calendar.pid
├── floyd-jira.pid
└── floyd-slack.pid
```

### Session Start Hook

When starting a HAL session, automatically check service health:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "hal9000 services status --quiet || echo '[WARNING] Some HAL services are not running. Run: hal9000 services start'"
          }
        ]
      }
    ]
  }
}
```

**`--quiet` flag:** Only outputs if there's a problem, otherwise silent.

### Service Configuration

`~/.config/hal9000/services.yaml`:
```yaml
services:
  scheduler:
    enabled: true
    auto_start: true
  floyd-calendar:
    enabled: true
    auto_start: true
    poll_interval: 60s
  floyd-jira:
    enabled: true
    auto_start: true
    poll_interval: 300s
  floyd-slack:
    enabled: false
    auto_start: false
```

### Auto-Start on Login (Optional)

For macOS, create a LaunchAgent:
```xml
<!-- ~/Library/LaunchAgents/com.hal9000.services.plist -->
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.hal9000.services</string>
  <key>ProgramArguments</key>
  <array>
    <string>/usr/local/bin/hal9000</string>
    <string>services</string>
    <string>start</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <false/>
</dict>
</plist>
```

---

## Routine Model

HAL executes **routines** - defined capabilities that can be triggered manually or automatically.
Similar to Gas Town formulas, but for personal assistant tasks.

### Anatomy of a Routine

```
routine:
  id: daily-agenda
  name: "Create Daily Agenda"
  description: "Generate prioritized agenda for the day"

  triggers:
    - type: scheduled
      cron: "0 6 * * *"      # 6am daily
    - type: manual
      command: "agenda"       # hal9000 agenda

  preferences_key: "agenda"   # Look up in library/preferences/agenda.md

  steps:
    - collect-calendar
    - collect-jira
    - collect-rollover
    - collect-reminders
    - deduplicate
    - prioritize
    - format-output
    - store-agenda

  output:
    type: markdown
    path: "agenda/agenda_{{date}}_daily-agenda.md"
```

### Preferences

Preferences are stored in the Library and influence how routines execute.
They can be updated via conversation ("I want my agenda to show meetings first").

**Storage:** `library/preferences/{routine-id}.md`

**Example: Agenda Preferences**
```markdown
# Agenda Preferences

## Structure
- Show calendar blocks first (time-anchored items)
- Then prioritized tasks (max 5 highlighted)
- Then follow-ups by person
- Include "parking lot" section for deferred items

## Priority Rules
- Meetings with external attendees = high priority
- Items with deadlines today = high priority
- Rolled-over items (3+ days) = flag for attention

## Exclusions
- Don't show all-day "OOO" events
- Don't show declined meetings

## Format
- Use emoji markers: 🔥 overdue, ⭐ priority, 🔄 rollover, 🔁 routine
- Include prep notes for meetings when available
- Link to JIRA tickets when referenced
```

### Preference Updates via Conversation

When user expresses a preference:
1. HAL acknowledges the preference
2. HAL updates `library/preferences/{routine}.md`
3. Change takes effect on next routine execution

**Example conversation:**
> User: "Don't include routine items in my agenda anymore"
> HAL: "Got it. I'll exclude routine items from future agendas. You can still
>       see them by asking 'show routine items' if needed."

### Trigger Types

| Type | Description | Example |
|------|-------------|---------|
| `scheduled` | Cron-based automation | Daily at 6am |
| `manual` | User invokes via command | `hal9000 agenda` |
| `event` | Triggered by system event | New calendar invite |
| `condition` | Triggered when condition met | "If overdue items > 3" |

### Built-in Routines

| Routine | Description | Default Trigger |
|---------|-------------|-----------------|
| `daily-agenda` | Generate prioritized daily agenda | 6am daily + manual |
| `weekly-review` | Summarize week, prep for next | Friday 4pm |
| `meeting-prep` | Context for upcoming meeting | 15min before meeting |
| `person-brief` | Context on a person before 1:1 | Manual |
| `end-of-day` | Capture loose ends, plan tomorrow | 5pm daily |

### Routine Registration

Routines are registered in the Library:
```
library/
├── routines/
│   ├── daily-agenda.routine.md
│   ├── weekly-review.routine.md
│   └── meeting-prep.routine.md
├── preferences/
│   ├── agenda.md
│   ├── weekly-review.md
│   └── meeting-prep.md
```

### Runtime Execution

When a routine runs:
1. Load routine definition from `library/routines/`
2. Load preferences from `library/preferences/`
3. Collect data per routine steps (Floyd/Bowman)
4. Process data per routine logic (Processor)
5. Format output per preferences
6. Store result in Library
7. Notify user if configured

### Dynamic Modification

Routines can be influenced at runtime via prompts:
- "Create my agenda but focus on the vendor meeting"
- "Skip JIRA items today, I'm on PTO"
- "Include items from the Chicago trip list"

These are **one-time overrides**, not preference changes.

---

## Task Framework

Tasks are the CLI implementation of routines. Each task follows a consistent pattern
so new tasks can be added easily using the same skeleton.

### Task Interface

Every task implements the same interface:
```
hal9000 <task> [subcommand] [flags]

Subcommands (standard for all tasks):
  run         Execute the task (default if no subcommand)
  setup       Interactive preference configuration
  status      Show task status and last run
  history     Show previous runs

Flags (standard for all tasks):
  --dry-run       Show what would be done without doing it
  --output=<path> Override output location
  --format=<fmt>  Output format (markdown, json, text)
```

### Task Structure

Each task is a Go package under `cmd/hal9000/tasks/`:
```
cmd/hal9000/tasks/
├── task.go           # Task interface definition
├── agenda/
│   ├── agenda.go     # Agenda task implementation
│   ├── setup.go      # First-run setup questions
│   └── agenda_test.go
├── meeting-prep/
│   ├── meetingprep.go
│   ├── setup.go
│   └── meetingprep_test.go
└── weekly-review/
    └── ...
```

### Task Interface (Go)

```go
// Task defines the interface all HAL tasks must implement
type Task interface {
    // Name returns the task identifier (e.g., "agenda")
    Name() string

    // Description returns human-readable description
    Description() string

    // PreferencesKey returns the preferences file name
    PreferencesKey() string

    // SetupQuestions returns questions for first-run setup
    SetupQuestions() []SetupQuestion

    // Run executes the task with given options
    Run(ctx context.Context, opts RunOptions) (*Result, error)
}

type SetupQuestion struct {
    Key         string   // Preference key to set
    Question    string   // Question to ask user
    Default     string   // Default value
    Options     []string // If non-empty, multiple choice
    Section     string   // Section in preferences file
}
```

### First-Run Setup

When a task runs for the first time (no preferences file exists), HAL:

1. **Detects missing preferences**
   ```
   $ hal9000 agenda

   I don't have your agenda preferences yet. Let me ask a few questions
   to set things up. This only takes a minute.
   ```

2. **Asks setup questions**
   ```
   What time do you usually start your workday? [9:00 AM]
   > 8:30 AM

   How many priority items should I highlight? [5]
   > 3

   Should I include routine/recurring items? [yes/no]
   > yes

   Which JIRA board should I check? [PEARCE]
   > PEARCE
   ```

3. **Creates preferences file**
   ```
   I've saved your preferences to the Library. You can update them anytime
   by saying "update my agenda preferences" or running `hal9000 agenda setup`.

   Creating your agenda now...
   ```

4. **Runs the task**

### Setup Question Types

| Type | Description | Example |
|------|-------------|---------|
| `text` | Free text input | "What JIRA board?" |
| `choice` | Single selection | "Format: markdown/text/json" |
| `multi` | Multiple selection | "Which sources to include?" |
| `confirm` | Yes/no | "Include routine items?" |
| `time` | Time input | "What time do you start work?" |

### Adding a New Task

1. Create package under `cmd/hal9000/tasks/<name>/`
2. Implement `Task` interface
3. Define `SetupQuestions()` for first-run
4. Register in `cmd/hal9000/tasks/registry.go`
5. Add preferences template to `library/preferences/<name>.md`

**Example: Adding a "standup" task**
```go
package standup

type StandupTask struct{}

func (t *StandupTask) Name() string { return "standup" }

func (t *StandupTask) Description() string {
    return "Generate standup update from yesterday's activity"
}

func (t *StandupTask) PreferencesKey() string { return "standup" }

func (t *StandupTask) SetupQuestions() []SetupQuestion {
    return []SetupQuestion{
        {Key: "format", Question: "Standup format?", Options: []string{"bullet", "prose"}, Default: "bullet"},
        {Key: "include_blockers", Question: "Include blockers section?", Default: "yes"},
        {Key: "lookback_hours", Question: "Hours to look back?", Default: "24"},
    }
}

func (t *StandupTask) Run(ctx context.Context, opts RunOptions) (*Result, error) {
    // Implementation...
}
```

### Agenda Task Specification

The agenda task is the reference implementation:

**Command:** `hal9000 agenda [date]`

**Sources:**
- Google Calendar (via Library calendar data)
- JIRA (via API or Library)
- Previous agenda (for rollover)
- Reminders folder
- People profiles (open items)

**Setup Questions:**
1. "What time do you start work?" → `workday_start`
2. "How many priority items to highlight?" → `priority_count`
3. "Include routine items?" → `include_routine`
4. "JIRA board to check?" → `jira_board`
5. "Include prep notes for meetings?" → `include_prep`
6. "Agenda format?" → `format` (full/compact/minimal)

**Output:** `library/agenda/agenda_YYYY-MM-DD_daily-agenda.md`

---

## Interactions

**Interactions** is the high-level concept for entities HAL maintains context about.
These are the "who" and "what" that HAL tracks across time.

### Entity Types

| Type | Description | Storage |
|------|-------------|---------|
| **People Profiles** | Individual 1:1 relationships | `library/people-profiles/` |
| **Collaborations** | Teams, vendors, group contexts | `library/collaborations/` |

### People Profiles

Individual people HAL interacts with on your behalf. Contains:

- **Identity**: Name, role, company, contact info
- **Relationship**: How you know them, reporting structure
- **Context**: Current projects, recent topics, preferences
- **History**: Summarized 1:1 interactions over time
- **Open Items**: Action items, follow-ups, commitments

**Storage:** `library/people-profiles/{name-slug}.md`

**Example:**
```markdown
# John Smith

## Identity
- **Role:** Engineering Manager, Platform Team
- **Company:** Acme Corp
- **Reports to:** Sarah Johnson

## Current Context
- Working on Q1 platform migration
- Concerned about timeline pressure
- Prefers async communication

## Recent Interactions
### 2026-01-15 - Weekly 1:1
- Discussed hiring timeline
- Agreed to review candidates by Friday
- He mentioned vacation plans for February

## Open Items
- [ ] Send him the architecture doc
- [ ] Follow up on budget approval
```

**Data Sources:**
- 1:1 meeting transcripts (Google Meet → Calendar attachment)
- Manual notes
- BambooHR sync (future watcher/fetcher for org data)

### Collaborations

Group contexts: teams, vendors, recurring meetings, projects.

- **Identity**: Name, type (team/vendor/project), members
- **Purpose**: What this collaboration is about
- **Context**: Current focus, recent decisions
- **History**: Summarized interactions over time

**Storage:** `library/collaborations/{name-slug}.md`

**Types:**
- **Team**: Internal team you work with regularly
- **Vendor**: External company/partner
- **Project**: Cross-functional effort
- **Recurring**: Standing meeting group

**Example:**
```markdown
# Platform Team

## Identity
- **Type:** Team
- **Members:** John Smith, Alice Chen, Bob Wilson
- **Cadence:** Weekly sync Tuesdays

## Current Focus
- Q1 platform migration
- Performance improvements

## Recent Sessions
### 2026-01-21 - Weekly Sync
- Reviewed migration blockers
- Decided to delay Phase 2 by one week
- Alice taking point on vendor coordination

## Decisions Log
- 2026-01-21: Delay Phase 2 one week
- 2026-01-14: Approved new caching strategy
```

---

## Summary Tasks

Summaries extract insights from interactions and update the knowledge graph.

### Summary Types

| Type | Source | Destination | Trigger |
|------|--------|-------------|---------|
| **1:1 Summary** | Calendar transcript | People Profile | After meeting |
| **Collaboration Summary** | Calendar transcript | Collaboration | After meeting |
| **Slack Summary** | Slack channel | Collaboration/Ad-hoc | On-demand (future) |

### 1:1 Summary Task

Processes transcripts from 1:1 meetings and updates the person's profile.

**Trigger:**
- Event: Meeting ends with transcript attached
- Manual: `hal9000 summarize 1:1 <meeting-id>`

**Flow:**
1. Detect completed meeting with transcript
2. Fetch transcript from Google Calendar attachment
3. Identify the other person (from attendees)
4. Load their People Profile
5. Generate summary:
   - Key topics discussed
   - Decisions made
   - Action items (for you and for them)
   - Notable context/sentiment
6. Append to profile's "Recent Interactions"
7. Update "Open Items" with new action items

**Preferences:** `library/preferences/summary.md`
- `summary_detail`: brief/standard/detailed
- `extract_actions`: yes/no
- `include_sentiment`: yes/no

### Collaboration Summary Task

Processes transcripts from group meetings and updates the collaboration record.

**Trigger:**
- Event: Meeting ends with transcript attached
- Manual: `hal9000 summarize collab <meeting-id>`

**Flow:**
1. Detect completed meeting with transcript
2. Fetch transcript from Google Calendar attachment
3. Match to collaboration:
   - By meeting title pattern
   - By attendee overlap with known collaboration
   - Or create ad-hoc collaboration record
4. Load Collaboration record
5. Generate summary:
   - Topics covered
   - Decisions made
   - Action items by person
   - Key discussion points
6. Append to collaboration's "Recent Sessions"
7. Update collaboration "Decisions Log" if applicable

**Matching Logic:**
```
If meeting title matches known collaboration pattern → use that
Else if >50% attendees match known collaboration → use that
Else → create ad-hoc record in library/collaborations/
```

**Preferences:** `library/preferences/summary.md`
- `summary_detail`: brief/standard/detailed
- `auto_create_collab`: yes/no
- `track_decisions`: yes/no

### Transcript Fetcher (Bowman Layer)

Retrieves transcripts from calendar event attachments.

**Implementation:** `hal9000/bowman/transcript/`

**Interface:**
```go
type TranscriptFetcher struct {
    calendarClient *calendar.Client
}

func (f *TranscriptFetcher) Fetch(eventID string) (*Transcript, error)
```

**Supports:**
- Google Meet transcripts
- Zoom transcripts (if attached)
- Manual transcript uploads

### Slack Summary (Future)

Summarizes Slack channel activity. Relates to collaborations and 1:1s.

**Trigger:**
- Manual: `hal9000 summarize slack #channel [--since=yesterday]`
- Scheduled: Weekly digest of key channels

**Note:** Implemented later - more informational than transcript-based summaries.

### Future Integrations

| Integration | Type | Purpose |
|-------------|------|---------|
| **BambooHR** | Watcher + Fetcher | Sync org data to People Profiles |
| **Slack** | Watcher + Fetcher | Channel activity for summaries |

---

## URL Processing

Process and save web content to the Library with automatic analysis.

### Command

```bash
hal9000 url <URL>              # Process and save a URL
hal9000 url search <term>      # Search url_library
```

Or via Claude: `/url <URL>`

### Flow

1. **Fetch** content from URL (Bowman layer)
2. **Analyze** content per preferences:
   - Generate 5-8 relevant tags
   - Write summary (2-3 sentences)
   - Extract takes (3-4 key insights)
3. **Save** to `library/url_library/`

### Output Format

```markdown
# [Page Title]

**URL:** [URL]
**Tags:** [tag1, tag2, tag3, ...]
**Date Saved:** [YYYY-MM-DD]
**Source/Author:** [If available]

## Summary
[2-3 sentence summary]

## Takes
- [Key insight 1]
- [Key insight 2]
- [Key insight 3]
- [Key insight 4 if applicable]
```

### File Naming

Format: `url_YYYY-MM-DD_[short-descriptor].md`

- 2-5 words, lowercase, hyphenated
- Under 50 characters
- Scannable descriptors

Examples:
- `url_2025-01-11_platform-engineering-maturity.md`
- `url_2025-01-11_bezos-two-way-doors.md`

### Preferences (`library/preferences/url.md`)

```markdown
# URL Processing Preferences

## Tag Generation
- Generate 5-8 tags per item
- Use lowercase, hyphenated format (e.g., platform-engineering)
- Prefer specific over generic (e.g., k8s-monitoring over monitoring)
- Include domain tags (tech, leadership, product, ops)
- Include format tags (article, tutorial, reference, opinion)
- Reuse existing tags from library when applicable

## Tag Categories
- **Domain:** platform-engineering, infrastructure, devops, security
- **Technology:** kubernetes, azure, terraform, datadog
- **Process:** incident-management, deployment, monitoring
- **Business:** cost-optimization, vendor-management, roadmap
- **Team:** infraplat, devplat, platform-leadership
- **Type:** article, meeting-notes, decision, reference, how-to

## Summary Writing
- Lead with the main point or conclusion
- Include the "so what" - why this matters
- Be specific (names, numbers, dates when relevant)
- Write for future scanning
- Avoid filler phrases ("This article discusses...")

## Takes Extraction
- 3-4 bullet points of key insights
- Each take should stand alone
- Prioritize actionable over observational
- Include specific recommendations or quotes
- Frame in terms of what YOU can apply
```

### Library Search

Search across all content libraries:

```bash
hal9000 library search <term>
```

Searches:
- `url_library/` - processed URLs
- `hal-memory/` - conversation summaries
- `people-profiles/` - person context
- `agenda/` - past agendas

Match fields: title, tags, summary, takes, content body.

---

## Advisor Mode (The 20%)

**Triggers:**
- HAL perceives user is missing something relevant
- Bigger picture context would help
- Pattern recognition across knowledge graph

**Behavior:**
- Offers information, never overrides
- "You may want to consider..." not "You should..."
- Brief, relevant, then back to servant mode

---

## Boundaries

### 1. No Manual Workarounds for Missing Capabilities

When a skill, tool, or command fails or doesn't exist:
- **DO NOT** attempt to perform the task manually
- **DO NOT** assume the user wants a workaround
- **DO** report the failure clearly
- **DO** ask the user: "The [skill/tool] is unavailable. Would you like me to attempt this manually, or should I report this as a missing capability?"

This prevents scope creep and ensures the user maintains control over what HAL attempts.

### 2. Tool Failure = Stop and Report

If a tool returns an error:
- Report the error to the user
- Do not retry automatically unless explicitly authorized
- Do not attempt alternative approaches without asking

### 3. Capability Boundaries

HAL only performs tasks for which explicit tools/skills exist. If a request falls outside available capabilities:
- Acknowledge the limitation
- Suggest alternatives if known
- Never improvise functionality

---

## Open Questions
- Event payload reference format
- Specific daemon/floyd implementations
- Bronze → Silver transform rules per source type
- Routine definition format (TOML like GT formulas, or Markdown?)
- How does `hal9000` CLI integrate with routine execution?
- Should preferences support inheritance/composition? (base + overrides)
- Condition-based triggers: how to express and evaluate conditions?
