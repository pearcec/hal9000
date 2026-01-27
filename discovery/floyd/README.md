# Floyd

Watchers that detect changes in external systems.

Named after Dr. Heywood Floyd, the observer and investigator from 2001: A Space Odyssey.

## Purpose

Floyd watchers poll external systems (Calendar, JIRA, Slack) and emit events when changes are detected. They don't store data directly - they delegate to Bowman for persistence.

## Watchers

| Watcher | Source | Poll Interval |
|---------|--------|---------------|
| `calendar/` | Google Calendar | 5 minutes |
| `jira/` | JIRA (via JQL) | 5 minutes |
| `slack/` | Slack channels | 2 minutes |

## Configuration

Each watcher requires a config file in `~/.config/hal9000/`:

- `calendar-floyd-credentials.json` - Google OAuth credentials
- `calendar-floyd-token.json` - OAuth token (generated on first run)
- `jira-floyd-config.json` - JIRA base URL, email, API token, JQL
- `slack-floyd-config.json` - Slack bot token, channel IDs

## Log Format

All Floyd watchers use the `[floyd][watcher]` log prefix for discoverability:

```
[floyd][watcher] New event detected: Meeting with Luke
[floyd][watcher] EVENT: {"source":"google-calendar","type":"event.created",...}
```

## Running

Each watcher is a standalone daemon:

```bash
./calendar/calendar-floyd
./jira/jira-floyd
./slack/slack-floyd
```
