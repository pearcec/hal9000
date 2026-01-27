# Discovery

HAL 9000's service modules, named after the Discovery One spacecraft from 2001: A Space Odyssey.

## Structure

```
discovery/
├── floyd/      # Watchers - detect changes in external systems
├── bowman/     # Fetch & store - retrieve and persist data
├── processor/  # Transform data through medallion stages
└── lmc/        # Logic Memory Center - knowledge graph
```

## Data Flow

```
External Systems (Calendar, JIRA, Slack)
         │
         ▼
    ┌─────────┐
    │  Floyd  │  Watchers detect changes
    └────┬────┘
         │ events
         ▼
    ┌─────────┐
    │ Bowman  │  Fetch and store raw data
    └────┬────┘
         │ raw
         ▼
    ┌───────────┐
    │ Processor │  Raw → Bronze → Silver
    └─────┬─────┘
          │ enriched
          ▼
    ┌─────────┐
    │   LMC   │  Knowledge graph storage
    └─────────┘
```

## 2001: A Space Odyssey References

- **Discovery** - The spacecraft carrying HAL 9000
- **Floyd** - Dr. Heywood Floyd, the observer/investigator
- **Bowman** - Dave Bowman, who went out to fetch the AE-35 unit
- **LMC** - Logic Memory Center, HAL's memory modules
