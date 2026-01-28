package events

import (
	"github.com/pearcec/hal9000/discovery/bowman"
)

// StorageHandler creates a handler that persists events using Bowman.
// This is the bridge between the event bus and the storage layer.
func StorageHandler(libraryPath string) Handler {
	return func(event StorageEvent) StorageResult {
		config := bowman.StoreConfig{
			LibraryPath: libraryPath,
			Category:    event.Category,
		}

		switch event.Type {
		case EventStore:
			rawEvent := bowman.RawEvent{
				Source:    event.Source,
				EventID:   event.EventID,
				FetchedAt: event.FetchedAt,
				Stage:     "raw",
				Data:      event.Data,
			}
			path, err := bowman.Store(config, rawEvent)
			return StorageResult{Path: path, Error: err}

		case EventDelete:
			err := bowman.Delete(config, event.EventID)
			return StorageResult{Error: err}

		default:
			return StorageResult{}
		}
	}
}
