// Package transcript handles fetching meeting transcripts from Google Calendar attachments.
// "I know I've made some very poor decisions recently, but I can give you
// my complete assurance that my work will be back to normal."
//
// Bowman retrieves transcript data from calendar event attachments.
// Named after Dave Bowman, who went out to fetch the AE-35 unit.
package transcript

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const (
	credentialsPath = "~/.config/hal9000/calendar-floyd-credentials.json"
	tokenPath       = "~/.config/hal9000/calendar-floyd-token.json"
)

// TranscriptFormat identifies the source of a transcript.
type TranscriptFormat string

const (
	FormatGoogleMeet TranscriptFormat = "google-meet"
	FormatZoom       TranscriptFormat = "zoom"
	FormatUnknown    TranscriptFormat = "unknown"
)

// Transcript represents a fetched meeting transcript.
type Transcript struct {
	EventID     string           `json:"event_id"`
	EventTitle  string           `json:"event_title"`
	EventTime   time.Time        `json:"event_time"`
	Format      TranscriptFormat `json:"format"`
	Text        string           `json:"text"`
	Speakers    []string         `json:"speakers,omitempty"`
	FetchedAt   time.Time        `json:"fetched_at"`
	SourceFile  string           `json:"source_file,omitempty"`
	AttachmentID string          `json:"attachment_id,omitempty"`
}

// TranscriptEntry represents a single speaker turn in a transcript.
type TranscriptEntry struct {
	Speaker   string    `json:"speaker"`
	Timestamp time.Time `json:"timestamp,omitempty"`
	Text      string    `json:"text"`
}

// Fetcher retrieves transcripts from Google Calendar attachments.
type Fetcher struct {
	calendarService *calendar.Service
	driveService    *drive.Service
	httpClient      *http.Client
}

// NewFetcher creates a new transcript fetcher with authenticated clients.
func NewFetcher(ctx context.Context) (*Fetcher, error) {
	config, err := loadOAuthConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load OAuth config: %w", err)
	}

	client, err := getClient(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to get authenticated client: %w", err)
	}

	calSrv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create calendar service: %w", err)
	}

	drvSrv, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create drive service: %w", err)
	}

	return &Fetcher{
		calendarService: calSrv,
		driveService:    drvSrv,
		httpClient:      client,
	}, nil
}

// FetchForEvent retrieves transcript(s) for a specific calendar event.
func (f *Fetcher) FetchForEvent(ctx context.Context, eventID string) (*Transcript, error) {
	event, err := f.calendarService.Events.Get("primary", eventID).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	return f.fetchTranscriptFromEvent(ctx, event)
}

// FetchForTimeRange retrieves transcripts for events in a time range.
func (f *Fetcher) FetchForTimeRange(ctx context.Context, start, end time.Time) ([]*Transcript, error) {
	events, err := f.calendarService.Events.List("primary").
		TimeMin(start.Format(time.RFC3339)).
		TimeMax(end.Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime").
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}

	var transcripts []*Transcript
	for _, event := range events.Items {
		transcript, err := f.fetchTranscriptFromEvent(ctx, event)
		if err != nil {
			continue // Skip events without transcripts
		}
		if transcript != nil {
			transcripts = append(transcripts, transcript)
		}
	}

	return transcripts, nil
}

// fetchTranscriptFromEvent extracts transcript from a calendar event.
func (f *Fetcher) fetchTranscriptFromEvent(ctx context.Context, event *calendar.Event) (*Transcript, error) {
	// Check for attachments
	if len(event.Attachments) == 0 {
		return nil, fmt.Errorf("no attachments found for event %s", event.Id)
	}

	// Look for transcript attachment
	for _, attachment := range event.Attachments {
		if isTranscriptAttachment(attachment) {
			return f.fetchAttachmentTranscript(ctx, event, attachment)
		}
	}

	return nil, fmt.Errorf("no transcript attachment found for event %s", event.Id)
}

// isTranscriptAttachment checks if an attachment is likely a transcript.
func isTranscriptAttachment(attachment *calendar.EventAttachment) bool {
	title := strings.ToLower(attachment.Title)
	mimeType := attachment.MimeType

	// Google Meet transcripts are typically Google Docs
	if strings.Contains(title, "transcript") {
		return true
	}

	// Check for common transcript file patterns
	if strings.Contains(title, "meeting notes") && mimeType == "application/vnd.google-apps.document" {
		return true
	}

	// Zoom transcript files
	if strings.HasSuffix(title, ".vtt") || strings.HasSuffix(title, ".txt") {
		if strings.Contains(title, "transcript") || strings.Contains(title, "recording") {
			return true
		}
	}

	return false
}

// fetchAttachmentTranscript downloads and parses a transcript attachment.
func (f *Fetcher) fetchAttachmentTranscript(ctx context.Context, event *calendar.Event, attachment *calendar.EventAttachment) (*Transcript, error) {
	var text string
	var format TranscriptFormat
	var err error

	switch attachment.MimeType {
	case "application/vnd.google-apps.document":
		// Google Doc - export as plain text
		text, err = f.exportGoogleDoc(ctx, attachment.FileId)
		format = FormatGoogleMeet
	case "text/vtt":
		// WebVTT format (Zoom, etc.)
		text, err = f.downloadFile(ctx, attachment.FileId)
		format = FormatZoom
		if err == nil {
			text = parseVTT(text)
		}
	case "text/plain":
		// Plain text transcript
		text, err = f.downloadFile(ctx, attachment.FileId)
		format = detectFormat(text)
	default:
		// Try to download and detect format
		text, err = f.downloadFile(ctx, attachment.FileId)
		format = detectFormat(text)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to fetch attachment content: %w", err)
	}

	eventTime := parseEventTime(event)

	transcript := &Transcript{
		EventID:      event.Id,
		EventTitle:   event.Summary,
		EventTime:    eventTime,
		Format:       format,
		Text:         normalizeTranscript(text),
		Speakers:     extractSpeakers(text),
		FetchedAt:    time.Now(),
		SourceFile:   attachment.Title,
		AttachmentID: attachment.FileId,
	}

	return transcript, nil
}

// exportGoogleDoc exports a Google Doc as plain text.
func (f *Fetcher) exportGoogleDoc(ctx context.Context, fileID string) (string, error) {
	resp, err := f.driveService.Files.Export(fileID, "text/plain").Context(ctx).Download()
	if err != nil {
		return "", fmt.Errorf("failed to export doc: %w", err)
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read doc content: %w", err)
	}

	return string(content), nil
}

// downloadFile downloads a file from Google Drive.
func (f *Fetcher) downloadFile(ctx context.Context, fileID string) (string, error) {
	resp, err := f.driveService.Files.Get(fileID).Context(ctx).Download()
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read file content: %w", err)
	}

	return string(content), nil
}

// parseEventTime extracts the start time from a calendar event.
func parseEventTime(event *calendar.Event) time.Time {
	if event.Start.DateTime != "" {
		t, err := time.Parse(time.RFC3339, event.Start.DateTime)
		if err == nil {
			return t
		}
	}
	if event.Start.Date != "" {
		t, err := time.Parse("2006-01-02", event.Start.Date)
		if err == nil {
			return t
		}
	}
	return time.Time{}
}

// parseVTT parses WebVTT format transcript into plain text.
func parseVTT(content string) string {
	var result strings.Builder
	lines := strings.Split(content, "\n")

	// Skip header
	inCue := false
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip timestamps and empty lines
		if line == "" || line == "WEBVTT" {
			continue
		}
		if strings.Contains(line, "-->") {
			inCue = true
			continue
		}
		if inCue && line != "" {
			result.WriteString(line)
			result.WriteString(" ")
		}
	}

	return strings.TrimSpace(result.String())
}

// detectFormat attempts to identify the transcript format from content.
func detectFormat(content string) TranscriptFormat {
	lower := strings.ToLower(content)

	// Google Meet format markers
	if strings.Contains(lower, "google meet") || strings.Contains(lower, "meet.google.com") {
		return FormatGoogleMeet
	}

	// Zoom format markers
	if strings.Contains(lower, "zoom") || strings.Contains(content, "WEBVTT") {
		return FormatZoom
	}

	return FormatUnknown
}

// extractSpeakers extracts unique speaker names from transcript text.
func extractSpeakers(text string) []string {
	// Common patterns: "Speaker Name:" or "Name:" at start of line
	speakerPattern := regexp.MustCompile(`(?m)^([A-Z][a-zA-Z\s]+):\s`)
	matches := speakerPattern.FindAllStringSubmatch(text, -1)

	seen := make(map[string]bool)
	var speakers []string

	for _, match := range matches {
		if len(match) > 1 {
			speaker := strings.TrimSpace(match[1])
			if !seen[speaker] && len(speaker) > 1 && len(speaker) < 50 {
				seen[speaker] = true
				speakers = append(speakers, speaker)
			}
		}
	}

	return speakers
}

// normalizeTranscript cleans up transcript text for processing.
func normalizeTranscript(text string) string {
	// Remove excessive whitespace
	text = strings.TrimSpace(text)

	// Normalize line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// Remove multiple consecutive blank lines
	multiBlank := regexp.MustCompile(`\n{3,}`)
	text = multiBlank.ReplaceAllString(text, "\n\n")

	return text
}

// ParseEntries parses transcript text into individual speaker entries.
func ParseEntries(text string) []TranscriptEntry {
	var entries []TranscriptEntry

	// Pattern: "Speaker Name: text" or "HH:MM:SS Speaker Name: text"
	entryPattern := regexp.MustCompile(`(?m)^(?:(\d{1,2}:\d{2}(?::\d{2})?)\s+)?([A-Z][a-zA-Z\s]+):\s*(.+?)(?:\n|$)`)
	matches := entryPattern.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		entry := TranscriptEntry{
			Text: strings.TrimSpace(match[3]),
		}

		if len(match) > 2 {
			entry.Speaker = strings.TrimSpace(match[2])
		}

		if len(match) > 1 && match[1] != "" {
			// Parse timestamp if present
			entry.Timestamp, _ = time.Parse("15:04:05", match[1])
		}

		entries = append(entries, entry)
	}

	return entries
}

// OAuth helper functions (reused from Floyd)

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func loadOAuthConfig() (*oauth2.Config, error) {
	path := expandPath(credentialsPath)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials file: %w", err)
	}

	config, err := google.ConfigFromJSON(b,
		calendar.CalendarReadonlyScope,
		drive.DriveReadonlyScope,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials: %w", err)
	}

	return config, nil
}

func getClient(ctx context.Context, config *oauth2.Config) (*http.Client, error) {
	tokPath := expandPath(tokenPath)
	tok, err := loadToken(tokPath)
	if err != nil {
		return nil, fmt.Errorf("no valid token found (run calendar-floyd first): %w", err)
	}
	return config.Client(ctx, tok), nil
}

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
