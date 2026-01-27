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
