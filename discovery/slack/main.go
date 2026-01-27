// Package main implements the Slack Floyd (watcher) dog for HAL 9000.
// "Look Dave, I can see you're really upset about this."
//
// Floyd watches Slack channels for new messages and emits events.
// Named after Dr. Heywood Floyd, the observer/investigator from 2001.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pearcec/hal9000/discovery/bowman"
)

const (
	configPath   = "~/.config/hal9000/slack-floyd-config.json"
	statePath    = "~/.config/hal9000/slack-floyd-state.json"
	eventsPath   = "~/.config/hal9000/slack-events.jsonl"
	libraryPath  = "~/Documents/Google Drive/Claude/"
	pollInterval = 2 * time.Minute // Slack rate limits are stricter
)

// Config holds Slack connection settings.
type Config struct {
	BotToken   string   `json:"bot_token"`   // xoxb-... token
	ChannelIDs []string `json:"channel_ids"` // Channel IDs to watch
}

// Event represents a Slack change event emitted by Floyd (watcher).
type Event struct {
	Source    string    `json:"source"`
	Type      string    `json:"type"`
	Payload   string    `json:"payload"` // Message timestamp (unique ID)
	Channel   string    `json:"channel"`
	Timestamp time.Time `json:"timestamp"`
}

// FloydState tracks last seen message per channel.
type FloydState struct {
	Channels  map[string]string `json:"channels"`   // channelID -> last message ts
	UpdatedAt time.Time         `json:"updated_at"`
}

// SlackHistoryResponse is the Slack conversations.history response.
type SlackHistoryResponse struct {
	OK       bool           `json:"ok"`
	Messages []SlackMessage `json:"messages"`
	Error    string         `json:"error,omitempty"`
}

// SlackMessage represents a Slack message.
type SlackMessage struct {
	Type      string `json:"type"`
	User      string `json:"user"`
	Text      string `json:"text"`
	TS        string `json:"ts"` // Timestamp (unique message ID)
	ThreadTS  string `json:"thread_ts,omitempty"`
	Channel   string `json:"channel,omitempty"`
}

var bowmanConfig = bowman.StoreConfig{
	LibraryPath: libraryPath,
	Category:    "slack",
}

func main() {
	log.Println("[floyd][watcher] HAL 9000 Slack Floyd initializing...")

	// Load config
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Unable to load config: %v", err)
	}

	log.Printf("[floyd][watcher] Slack Floyd online. Watching %d channels", len(config.ChannelIDs))

	// Load or initialize state
	state := loadState()

	// Run watch loop
	for {
		events, newState, err := watchSlack(config, state)
		if err != nil {
			log.Printf("Error watching Slack: %v", err)
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

// loadConfig loads Slack configuration from file.
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

	if config.BotToken == "" || len(config.ChannelIDs) == 0 {
		return nil, fmt.Errorf("config missing required fields (bot_token, channel_ids)")
	}

	return &config, nil
}

func configExample() string {
	return `{
  "bot_token": "xoxb-your-bot-token",
  "channel_ids": ["C1234567890", "C0987654321"]
}`
}

// watchSlack checks all configured channels for new messages.
func watchSlack(config *Config, state FloydState) ([]Event, FloydState, error) {
	var allEvents []Event
	newState := FloydState{
		Channels:  make(map[string]string),
		UpdatedAt: time.Now(),
	}

	// Copy existing state
	for k, v := range state.Channels {
		newState.Channels[k] = v
	}

	for _, channelID := range config.ChannelIDs {
		lastTS := state.Channels[channelID]

		messages, err := getChannelHistory(config.BotToken, channelID, lastTS)
		if err != nil {
			log.Printf("Error fetching channel %s: %v", channelID, err)
			continue
		}

		if len(messages) == 0 {
			continue
		}

		// Process messages (oldest first for proper ordering)
		for i := len(messages) - 1; i >= 0; i-- {
			msg := messages[i]
			msg.Channel = channelID

			// Store via Bowman
			if err := storeSlackMessage(msg); err != nil {
				log.Printf("Error storing message: %v", err)
			}

			allEvents = append(allEvents, Event{
				Source:    "slack",
				Type:      "message.new",
				Payload:   msg.TS,
				Channel:   channelID,
				Timestamp: time.Now(),
			})

			// Truncate for logging
			preview := msg.Text
			if len(preview) > 50 {
				preview = preview[:50] + "..."
			}
			log.Printf("[floyd][watcher] New message in %s: %s", channelID, preview)
		}

		// Update last seen timestamp (most recent message)
		newState.Channels[channelID] = messages[0].TS
	}

	return allEvents, newState, nil
}

// getChannelHistory fetches messages newer than oldestTS.
func getChannelHistory(token, channelID, oldestTS string) ([]SlackMessage, error) {
	apiURL := "https://slack.com/api/conversations.history"

	params := url.Values{}
	params.Set("channel", channelID)
	params.Set("limit", "100")
	if oldestTS != "" {
		params.Set("oldest", oldestTS)
	}

	req, err := http.NewRequest("GET", apiURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Slack request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result SlackHistoryResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Slack response: %v", err)
	}

	if !result.OK {
		return nil, fmt.Errorf("Slack API error: %s", result.Error)
	}

	return result.Messages, nil
}

// storeSlackMessage stores a Slack message via Bowman.
func storeSlackMessage(msg SlackMessage) error {
	// Parse timestamp for date
	fetchTime := time.Now()

	rawEvent := bowman.RawEvent{
		Source:    "slack",
		EventID:   fmt.Sprintf("%s_%s", msg.Channel, msg.TS),
		FetchedAt: fetchTime,
		Stage:     "raw",
		Data: map[string]interface{}{
			"type":      msg.Type,
			"user":      msg.User,
			"text":      msg.Text,
			"ts":        msg.TS,
			"thread_ts": msg.ThreadTS,
			"channel":   msg.Channel,
		},
	}

	_, err := bowman.Store(bowmanConfig, rawEvent)
	return err
}

// loadState loads Floyd state from disk.
func loadState() FloydState {
	path := expandPath(statePath)
	state := FloydState{Channels: make(map[string]string)}

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
