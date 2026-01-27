# Processor

Data transformation layer implementing the medallion architecture.

## Purpose

Processor transforms data through stages:

```
Raw → Bronze → Silver
```

- **Raw**: Direct from source, minimal processing
- **Bronze**: Cleaned, structured, normalized
- **Silver**: Enriched with extracted entities and links

## API

```go
import "github.com/pearcec/hal9000/discovery/processor"

config := processor.ProcessConfig{
    LibraryPath: "~/Documents/Google Drive/Claude/",
}

// Transform raw → bronze
bronzeDoc, err := processor.ToBronze(config, "calendar", rawData)

// Transform bronze → silver
silverDoc, err := processor.ToSilver(config, bronzeDoc)

// Save to library
path, err := processor.SaveDocument(config, silverDoc)
```

## Source-Specific Transformations

### Calendar (Bronze)
- Extracts: title, attendees, location, meeting URL
- Normalizes: datetime formats, attendee structure

### JIRA (Bronze)
- Extracts: key, summary, status, assignee, reporter
- Normalizes: nested field structures

### Slack (Bronze)
- Extracts: channel, user, text, thread context
- Cleans: whitespace, formatting

## Link Extraction (Silver)

Silver stage extracts relationships for the knowledge graph:

| Link Type | Example |
|-----------|---------|
| `scheduled_with` | Calendar event → attendees |
| `assigned_to` | JIRA issue → assignee |
| `belongs_to` | JIRA issue → project |
| `posted_in` | Slack message → channel |
| `mentions` | Any text → @mentioned users/emails |

## Log Format

```
[processor][bronze] Processing calendar document
[processor][bronze] Created bronze document for abc123
[processor][silver] Created silver document with 3 links
```
