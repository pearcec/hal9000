// Package main implements the Calendar Floyd (watcher) dog for HAL 9000.
// "I am putting myself to the fullest possible use, which is all I think
// that any conscious entity can ever hope to do."
//
// Floyd watches for calendar changes and emits events.
// Named after Dr. Heywood Floyd, the observer/investigator from 2001.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

const (
	credentialsPath = "~/.config/hal9000/calendar-floyd-credentials.json"
	tokenPath       = "~/.config/hal9000/calendar-floyd-token.json"
	libraryPath     = "~/Documents/Google Drive/Claude/calendar/" // Raw event storage
	watchWindow     = 7 * 24 * time.Hour                          // One week ahead
	pollInterval    = 5 * time.Minute
)

// Event represents a calendar change event emitted by Floyd (watcher).
type Event struct {
	Source    string    `json:"source"`
	Type      string    `json:"type"`
	Payload   string    `json:"payload"`
	Timestamp time.Time `json:"timestamp"`
}

// FloydState tracks known events to detect changes (watcher state).
type FloydState struct {
	Events    map[string]string `json:"events"`    // eventID -> hash of event data
	UpdatedAt time.Time         `json:"updated_at"`
}

func main() {
	log.Println("[floyd][watcher] HAL 9000 Calendar Floyd initializing...")

	ctx := context.Background()

	// Load OAuth2 config
	config, err := loadOAuthConfig()
	if err != nil {
		log.Fatalf("Unable to load OAuth config: %v", err)
	}

	// Get authenticated client
	client, err := getClient(ctx, config)
	if err != nil {
		log.Fatalf("Unable to get authenticated client: %v", err)
	}

	// Create calendar service
	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to create Calendar service: %v", err)
	}

	log.Println("[floyd][watcher] Calendar Floyd online. Monitoring primary calendar.")

	// Load or initialize state
	state := loadState()

	// Run watch loop
	for {
		events, newState, err := watchCalendar(srv, state)
		if err != nil {
			log.Printf("Error watching calendar: %v", err)
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
	if path[:2] == "~/" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// loadOAuthConfig loads OAuth2 configuration from credentials file.
func loadOAuthConfig() (*oauth2.Config, error) {
	path := expandPath(credentialsPath)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials: %v", err)
	}

	return config, nil
}

// getClient retrieves an authenticated HTTP client.
func getClient(ctx context.Context, config *oauth2.Config) (*http.Client, error) {
	tokPath := expandPath(tokenPath)
	tok, err := loadToken(tokPath)
	if err != nil {
		tok, err = getTokenFromWeb(ctx, config)
		if err != nil {
			return nil, err
		}
		saveToken(tokPath, tok)
	}
	return config.Client(ctx, tok), nil
}

// loadToken loads a token from file.
func loadToken(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// saveToken saves a token to file.
func saveToken(path string, token *oauth2.Token) {
	f, err := os.Create(path)
	if err != nil {
		log.Printf("Unable to save token: %v", err)
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
	log.Printf("Token saved to %s", path)
}

// getTokenFromWeb uses OAuth2 flow to get a token.
func getTokenFromWeb(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	// Use localhost callback for desktop app
	config.RedirectURL = "http://localhost:8089/callback"

	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("\nOpen the following URL in your browser:\n\n%s\n\n", authURL)

	// Start local server to receive callback
	codeChan := make(chan string)
	errChan := make(chan error)

	server := &http.Server{Addr: ":8089"}
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no code in callback")
			return
		}
		fmt.Fprintf(w, "Authorization successful. You may close this window.")
		codeChan <- code
	})

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	var code string
	select {
	case code = <-codeChan:
	case err := <-errChan:
		return nil, err
	case <-time.After(2 * time.Minute):
		return nil, fmt.Errorf("timeout waiting for authorization")
	}

	server.Shutdown(ctx)

	tok, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("unable to exchange code for token: %v", err)
	}

	return tok, nil
}

// watchCalendar checks for calendar changes and returns events.
func watchCalendar(srv *calendar.Service, state FloydState) ([]Event, FloydState, error) {
	var events []Event
	newState := FloydState{
		Events:    make(map[string]string),
		UpdatedAt: time.Now(),
	}

	now := time.Now()
	timeMin := now.Format(time.RFC3339)
	timeMax := now.Add(watchWindow).Format(time.RFC3339)

	calEvents, err := srv.Events.List("primary").
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(timeMin).
		TimeMax(timeMax).
		OrderBy("startTime").
		Do()
	if err != nil {
		return nil, state, fmt.Errorf("unable to retrieve events: %v", err)
	}

	// Build new state and detect changes
	currentIDs := make(map[string]bool)
	for _, item := range calEvents.Items {
		hash := hashEvent(item)
		newState.Events[item.Id] = hash
		currentIDs[item.Id] = true

		oldHash, existed := state.Events[item.Id]
		if !existed {
			// New event - store raw data
			if err := storeRawEvent(item); err != nil {
				log.Printf("Error storing new event: %v", err)
			}
			events = append(events, Event{
				Source:    "google-calendar",
				Type:      "event.created",
				Payload:   item.Id,
				Timestamp: time.Now(),
			})
			log.Printf("[floyd][watcher] New event detected: %s - %s", item.Summary, item.Start.DateTime)
		} else if oldHash != hash {
			// Modified event - update stored data
			if err := storeRawEvent(item); err != nil {
				log.Printf("Error updating event: %v", err)
			}
			events = append(events, Event{
				Source:    "google-calendar",
				Type:      "event.modified",
				Payload:   item.Id,
				Timestamp: time.Now(),
			})
			log.Printf("[floyd][watcher] Modified event detected: %s", item.Summary)
		}
	}

	// Detect deleted events
	for id := range state.Events {
		if !currentIDs[id] {
			// Delete stored data
			if err := deleteRawEvent(id); err != nil {
				log.Printf("Error deleting event file: %v", err)
			}
			events = append(events, Event{
				Source:    "google-calendar",
				Type:      "event.deleted",
				Payload:   id,
				Timestamp: time.Now(),
			})
			log.Printf("[floyd][watcher] Deleted event detected: %s", id)
		}
	}

	return events, newState, nil
}

// hashEvent creates a simple hash of event data to detect changes.
func hashEvent(e *calendar.Event) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s",
		e.Summary,
		e.Start.DateTime,
		e.End.DateTime,
		e.Location,
		e.Description,
	)
}

// loadState loads Floyd (watcher) state from disk.
func loadState() FloydState {
	path := expandPath("~/.config/hal9000/calendar-floyd-state.json")
	state := FloydState{Events: make(map[string]string)}

	data, err := os.ReadFile(path)
	if err != nil {
		return state
	}
	json.Unmarshal(data, &state)
	return state
}

// saveState persists Floyd (watcher) state to disk.
func saveState(state FloydState) {
	path := expandPath("~/.config/hal9000/calendar-floyd-state.json")
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(path, data, 0644)
}

// emitEvent outputs an event (for now, just logs it).
// TODO: Write to event queue or bus.
func emitEvent(event Event) {
	data, _ := json.Marshal(event)
	log.Printf("[floyd][watcher] EVENT: %s", string(data))

	// Also write to events file for Bowman (fetch) to pick up
	path := expandPath("~/.config/hal9000/calendar-events.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Unable to write event: %v", err)
		return
	}
	defer f.Close()
	f.Write(append(data, '\n'))
}

// storeRawEvent stores the full calendar event data in the library.
// This is the Bowman (fetch) function - stores raw data for later processing.
func storeRawEvent(item *calendar.Event) error {
	// Ensure library directory exists
	libPath := expandPath(libraryPath)
	if err := os.MkdirAll(libPath, 0755); err != nil {
		return fmt.Errorf("unable to create library directory: %v", err)
	}

	// Extract date from event for filename
	var eventDate string
	if item.Start.DateTime != "" {
		t, _ := time.Parse(time.RFC3339, item.Start.DateTime)
		eventDate = t.Format("2006-01-02")
	} else if item.Start.Date != "" {
		eventDate = item.Start.Date
	} else {
		eventDate = time.Now().Format("2006-01-02")
	}

	// Create filename: calendar_YYYY-MM-DD_eventid.json
	// Sanitize event ID for filename (replace problematic chars)
	safeID := sanitizeFilename(item.Id)
	filename := fmt.Sprintf("calendar_%s_%s.json", eventDate, safeID)
	filepath := filepath.Join(libPath, filename)

	// Create raw event structure with metadata
	rawEvent := map[string]interface{}{
		"_meta": map[string]interface{}{
			"source":     "google-calendar",
			"fetched_at": time.Now().Format(time.RFC3339),
			"event_id":   item.Id,
			"stage":      "raw",
		},
		"summary":     item.Summary,
		"description": item.Description,
		"location":    item.Location,
		"start":       item.Start,
		"end":         item.End,
		"attendees":   item.Attendees,
		"organizer":   item.Organizer,
		"created":     item.Created,
		"updated":     item.Updated,
		"status":      item.Status,
		"htmlLink":    item.HtmlLink,
		"hangoutLink": item.HangoutLink,
		"conferenceData": item.ConferenceData,
	}

	data, err := json.MarshalIndent(rawEvent, "", "  ")
	if err != nil {
		return fmt.Errorf("unable to marshal event: %v", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("unable to write event file: %v", err)
	}

	log.Printf("[bowman][fetch] Stored raw event: %s", filename)
	return nil
}

// deleteRawEvent removes the stored event file when an event is deleted.
func deleteRawEvent(eventID string) error {
	libPath := expandPath(libraryPath)
	safeID := sanitizeFilename(eventID)

	// Find and delete matching file(s) - we need to glob since we don't know the date
	pattern := filepath.Join(libPath, fmt.Sprintf("calendar_*_%s.json", safeID))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, match := range matches {
		if err := os.Remove(match); err != nil {
			log.Printf("Unable to delete %s: %v", match, err)
		} else {
			log.Printf("Deleted event file: %s", filepath.Base(match))
		}
	}
	return nil
}

// sanitizeFilename makes a string safe for use in filenames.
func sanitizeFilename(s string) string {
	// Replace characters that are problematic in filenames
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
