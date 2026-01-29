# HAL 9000 Prompts

This directory contains prompt templates for Poole, the event dispatcher.

## Directory Structure

```
prompts/
├── defaults/          # Built-in prompts (don't modify)
│   ├── bamboohr-inbox-triage.md
│   └── email-triage.md
└── README.md
```

## User Overrides

To customize prompts, create your own versions in:
- `.hal9000/prompts/` (project-level)
- `~/.hal9000/prompts/` (user-level)

User prompts with the same filename as defaults will override them.

## Template Variables

Prompts support variable substitution using `{{variable_name}}` syntax:

| Variable | Description |
|----------|-------------|
| `{{event_id}}` | Unique identifier for the event |
| `{{source}}` | Event source (e.g., "jira", "slack") |
| `{{category}}` | Event category for storage |
| `{{fetched_at}}` | ISO 8601 timestamp when event was received |
| `{{event_data}}` | JSON representation of the full event data |

## Creating Custom Prompts

1. Create a `.md` file in your prompts directory
2. Use a descriptive name (e.g., `jira-epic-analysis.md`)
3. Reference it in your `actions.yaml`:

```yaml
actions:
  analyze-epics:
    enabled: true
    event_type: "jira:issue.created"
    prompt: jira-epic-analysis
```

## Best Practices

- Keep prompts focused on a single task
- Include clear output format specifications
- Use structured output (YAML, JSON, or labeled sections)
- Test prompts with sample data before deploying
