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

### Library Locations (Current)
```
/Users/cpearce/Documents/Google Drive/Claude/
├── agenda/           # Daily agendas
├── reminders/        # Time-triggered items
├── people-profiles/  # Person nodes
├── lists/            # Reference lists (routines, etc.)
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
/library/
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

*To be defined through usage*

---

## Open Questions
- Event payload reference format
- Specific daemon/floyd implementations
- Bronze → Silver transform rules per source type
- Routine definition format (TOML like GT formulas, or Markdown?)
- How does `hal9000` CLI integrate with routine execution?
- Should preferences support inheritance/composition? (base + overrides)
- Condition-based triggers: how to express and evaluate conditions?
