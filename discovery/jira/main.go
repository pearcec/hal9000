// Package main implements the JIRA Floyd (watcher) dog for HAL 9000.
// "I'm sorry, Dave. I'm afraid I can't do that."
//
// Floyd watches for JIRA issue changes and emits events.
// Named after Dr. Heywood Floyd, the observer/investigator from 2001.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pearcec/hal9000/discovery/bowman"
)

const (
	configPath   = "~/.config/hal9000/jira-floyd-config.json"
	statePath    = "~/.config/hal9000/jira-floyd-state.json"
	eventsPath   = "~/.config/hal9000/jira-events.jsonl"
	libraryPath  = "~/Documents/Google Drive/Claude/"
	pollInterval = 5 * time.Minute
)

// Config holds JIRA connection settings.
type Config struct {
	BaseURL  string `json:"base_url"`  // e.g., "https://yourcompany.atlassian.net"
	Email    string `json:"email"`     // JIRA account email
	APIToken string `json:"api_token"` // JIRA API token
	JQL      string `json:"jql"`       // JQL query to watch (e.g., "project = MYPROJECT")
}

// Event represents a JIRA change event emitted by Floyd (watcher).
type Event struct {
	Source    string    `json:"source"`
	Type      string    `json:"type"`
	Payload   string    `json:"payload"` // Issue key
	Timestamp time.Time `json:"timestamp"`
}

// FloydState tracks known issues to detect changes.
type FloydState struct {
	Issues    map[string]string `json:"issues"`     // issueKey -> hash of issue data
	UpdatedAt time.Time         `json:"updated_at"`
}

// JIRASearchResponse is the JIRA search API response.
type JIRASearchResponse struct {
	Total  int          `json:"total"`
	Issues []JIRAIssue  `json:"issues"`
}

// JIRAIssue represents a JIRA issue from the API.
type JIRAIssue struct {
	ID     string                 `json:"id"`
	Key    string                 `json:"key"`
	Self   string                 `json:"self"`
	Fields map[string]interface{} `json:"fields"`
}

var bowmanConfig = bowman.StoreConfig{
	LibraryPath: libraryPath,
	Category:    "jira",
}

func main() {
	log.Println("[floyd][watcher] HAL 9000 JIRA Floyd initializing...")

	// Load config
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Unable to load config: %v", err)
	}

	log.Printf("[floyd][watcher] JIRA Floyd online. Watching: %s", config.JQL)

	// Load or initialize state
	state := loadState()

	// Run watch loop
	for {
		events, newState, err := watchJIRA(config, state)
		if err != nil {
			log.Printf("Error watching JIRA: %v", err)
		} else {
			for _, event := range events {
				emitEvent(event)
			}
			state = newState
			saveState(state)
		}

		time.Sleep(pollInterval)
	}
}

// expandPath expands ~ to home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// loadConfig loads JIRA configuration from file.
func loadConfig() (*Config, error) {
	path := expandPath(configPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read config file %s: %v\n\nCreate config with:\n%s", path, err, configExample())
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unable to parse config: %v", err)
	}

	if config.BaseURL == "" || config.Email == "" || config.APIToken == "" {
		return nil, fmt.Errorf("config missing required fields (base_url, email, api_token)")
	}

	return &config, nil
}

func configExample() string {
	return `{
  "base_url": "https://yourcompany.atlassian.net",
  "email": "you@company.com",
  "api_token": "your-api-token",
  "jql": "project = MYPROJECT AND updated >= -7d ORDER BY updated DESC"
}`
}

// watchJIRA checks for JIRA issue changes and returns events.
func watchJIRA(config *Config, state FloydState) ([]Event, FloydState, error) {
	var events []Event
	newState := FloydState{
		Issues:    make(map[string]string),
		UpdatedAt: time.Now(),
	}

	// Query JIRA
	issues, err := searchJIRA(config)
	if err != nil {
		return nil, state, err
	}

	// Build new state and detect changes
	currentKeys := make(map[string]bool)
	for _, issue := range issues {
		hash := hashIssue(issue)
		newState.Issues[issue.Key] = hash
		currentKeys[issue.Key] = true

		oldHash, existed := state.Issues[issue.Key]
		if !existed {
			// New issue - store via Bowman
			if err := storeJIRAIssue(issue); err != nil {
				log.Printf("Error storing new issue: %v", err)
			}
			events = append(events, Event{
				Source:    "jira",
				Type:      "issue.created",
				Payload:   issue.Key,
				Timestamp: time.Now(),
			})
			summary := getFieldString(issue.Fields, "summary")
			log.Printf("[floyd][watcher] New issue detected: %s - %s", issue.Key, summary)
		} else if oldHash != hash {
			// Modified issue - update via Bowman
			if err := storeJIRAIssue(issue); err != nil {
				log.Printf("Error updating issue: %v", err)
			}
			events = append(events, Event{
				Source:    "jira",
				Type:      "issue.modified",
				Payload:   issue.Key,
				Timestamp: time.Now(),
			})
			log.Printf("[floyd][watcher] Modified issue detected: %s", issue.Key)
		}
	}

	// Detect issues no longer in results (may be closed, moved out of query scope)
	for key := range state.Issues {
		if !currentKeys[key] {
			// Issue removed from results - note: don't delete, it may just be out of JQL scope
			events = append(events, Event{
				Source:    "jira",
				Type:      "issue.removed_from_watch",
				Payload:   key,
				Timestamp: time.Now(),
			})
			log.Printf("[floyd][watcher] Issue removed from watch scope: %s", key)
		}
	}

	return events, newState, nil
}

// searchJIRA queries JIRA with the configured JQL.
func searchJIRA(config *Config) ([]JIRAIssue, error) {
	url := fmt.Sprintf("%s/rest/api/3/search?jql=%s&maxResults=100",
		config.BaseURL,
		strings.ReplaceAll(config.JQL, " ", "%20"))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Basic auth with email:api_token
	auth := base64.StdEncoding.EncodeToString([]byte(config.Email + ":" + config.APIToken))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("JIRA request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("JIRA returned %d: %s", resp.StatusCode, string(body))
	}

	var result JIRASearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse JIRA response: %v", err)
	}

	return result.Issues, nil
}

// storeJIRAIssue stores a JIRA issue via Bowman.
func storeJIRAIssue(issue JIRAIssue) error {
	rawEvent := bowman.RawEvent{
		Source:    "jira",
		EventID:   issue.Key,
		FetchedAt: time.Now(),
		Stage:     "raw",
		Data: map[string]interface{}{
			"id":     issue.ID,
			"key":    issue.Key,
			"self":   issue.Self,
			"fields": issue.Fields,
		},
	}

	_, err := bowman.Store(bowmanConfig, rawEvent)
	return err
}

// hashIssue creates a hash of issue data to detect changes.
func hashIssue(issue JIRAIssue) string {
	summary := getFieldString(issue.Fields, "summary")
	status := getNestedString(issue.Fields, "status", "name")
	assignee := getNestedString(issue.Fields, "assignee", "displayName")
	updated := getFieldString(issue.Fields, "updated")

	return fmt.Sprintf("%s|%s|%s|%s|%s",
		issue.Key,
		summary,
		status,
		assignee,
		updated,
	)
}

// getFieldString safely gets a string field from the fields map.
func getFieldString(fields map[string]interface{}, key string) string {
	if val, ok := fields[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// getNestedString safely gets a nested string field (e.g., status.name).
func getNestedString(fields map[string]interface{}, key, subkey string) string {
	if val, ok := fields[key]; ok {
		if nested, ok := val.(map[string]interface{}); ok {
			if str, ok := nested[subkey].(string); ok {
				return str
			}
		}
	}
	return ""
}

// loadState loads Floyd state from disk.
func loadState() FloydState {
	path := expandPath(statePath)
	state := FloydState{Issues: make(map[string]string)}

	data, err := os.ReadFile(path)
	if err != nil {
		return state
	}
	json.Unmarshal(data, &state)
	return state
}

// saveState persists Floyd state to disk.
func saveState(state FloydState) {
	path := expandPath(statePath)
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(path, data, 0644)
}

// emitEvent outputs an event.
func emitEvent(event Event) {
	data, _ := json.Marshal(event)
	log.Printf("[floyd][watcher] EVENT: %s", string(data))

	// Write to events file
	path := expandPath(eventsPath)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Unable to write event: %v", err)
		return
	}
	defer f.Close()
	f.Write(append(data, '\n'))
}
