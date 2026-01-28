// Package bowman handles data fetching and storage for HAL 9000.
// "Open the pod bay doors, HAL."
//
// Bowman retrieves data from sources and stores it in the library.
// Named after Dave Bowman, who went out to fetch the AE-35 unit.
package bowman

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pearcec/hal9000/discovery/config"
)

const (
	// InlineThreshold - data smaller than this is stored inline (per SPEC.md)
	InlineThreshold = 1024 // 1kb
)

// RawEvent represents data fetched from a source, ready for storage.
type RawEvent struct {
	Source    string                 `json:"source"`
	EventID   string                 `json:"event_id"`
	FetchedAt time.Time              `json:"fetched_at"`
	Stage     string                 `json:"stage"` // always "raw" from bowman
	Data      map[string]interface{} `json:"data"`
}

// StoreConfig configures where Bowman stores data.
type StoreConfig struct {
	LibraryPath string // Base path for the library (from config.GetLibraryPath())
	Category    string // Subfolder category (e.g., "calendar", "jira", "slack")
}

// Store saves a raw event to the library.
// Returns the path where the event was stored.
func Store(config StoreConfig, event RawEvent) (string, error) {
	// Expand ~ in path
	libPath := expandPath(config.LibraryPath)
	categoryPath := filepath.Join(libPath, config.Category)

	// Ensure directory exists
	if err := os.MkdirAll(categoryPath, 0755); err != nil {
		return "", fmt.Errorf("[bowman][fetch] unable to create library directory: %v", err)
	}

	// Build filename: {category}_{date}_{eventid}.json
	dateStr := event.FetchedAt.Format("2006-01-02")
	safeID := sanitizeFilename(event.EventID)
	filename := fmt.Sprintf("%s_%s_%s.json", config.Category, dateStr, safeID)
	fullPath := filepath.Join(categoryPath, filename)

	// Build storage structure with metadata
	storageDoc := map[string]interface{}{
		"_meta": map[string]interface{}{
			"source":     event.Source,
			"fetched_at": event.FetchedAt.Format(time.RFC3339),
			"event_id":   event.EventID,
			"stage":      "raw",
		},
	}

	// Merge in the data
	for k, v := range event.Data {
		storageDoc[k] = v
	}

	// Marshal and check size
	data, err := json.MarshalIndent(storageDoc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("[bowman][fetch] unable to marshal event: %v", err)
	}

	// SPEC.md storage rules: <1kb inline, ≥1kb store pointer
	// For now, we always store the full document (pointer logic TBD for large docs)
	if len(data) >= InlineThreshold {
		log.Printf("[bowman][fetch] Large event (%d bytes), storing full document: %s", len(data), filename)
	}

	// Write to file
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return "", fmt.Errorf("[bowman][fetch] unable to write event file: %v", err)
	}

	log.Printf("[bowman][fetch] Stored raw event: %s", filename)
	return fullPath, nil
}

// Delete removes a stored event from the library.
func Delete(config StoreConfig, eventID string) error {
	libPath := expandPath(config.LibraryPath)
	categoryPath := filepath.Join(libPath, config.Category)
	safeID := sanitizeFilename(eventID)

	// Find and delete matching file(s) - glob since we don't know the date
	pattern := filepath.Join(categoryPath, fmt.Sprintf("%s_*_%s.json", config.Category, safeID))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, match := range matches {
		if err := os.Remove(match); err != nil {
			log.Printf("[bowman][fetch] Unable to delete %s: %v", match, err)
		} else {
			log.Printf("[bowman][fetch] Deleted event file: %s", filepath.Base(match))
		}
	}
	return nil
}

// expandPath expands ~ to home directory and resolves relative paths.
// Relative paths are resolved from the executable's directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	// For relative paths, resolve from executable directory
	if !filepath.IsAbs(path) {
		return filepath.Join(config.GetExecutableDir(), path)
	}
	return path
}

// sanitizeFilename makes a string safe for use in filenames.
func sanitizeFilename(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, c)
		} else {
			result = append(result, '_')
		}
	}
	return string(result)
}
