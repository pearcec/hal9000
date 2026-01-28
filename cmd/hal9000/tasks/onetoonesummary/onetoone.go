// Package onetoonesummary provides a task for summarizing 1:1 meeting transcripts.
// "I know I've made some very poor decisions recently, but I can give you
// my complete assurance that my work will be back to normal."
//
// This task processes meeting transcripts and updates People Profiles with
// summaries, action items, and interaction history.
package onetoonesummary

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pearcec/hal9000/cmd/hal9000/tasks"
	"github.com/pearcec/hal9000/discovery/bowman/transcript"
)

const (
	// Default library path for People Profiles
	defaultLibraryPath = "~/Documents/Google Drive/Claude"
	profilesSubdir     = "people"
)

// Task implements the 1:1 summary task.
type Task struct{}

// init registers the task with the registry.
func init() {
	tasks.Register(&Task{})
}

// Name returns the task identifier.
func (t *Task) Name() string {
	return "onetoone"
}

// Description returns a human-readable description.
func (t *Task) Description() string {
	return "Summarize 1:1 meeting transcripts and update People Profiles"
}

// PreferencesKey returns the preferences file name.
func (t *Task) PreferencesKey() string {
	return "onetoone"
}

// SetupQuestions returns questions for first-run setup.
func (t *Task) SetupQuestions() []tasks.SetupQuestion {
	return []tasks.SetupQuestion{
		{
			Key:      "summary_detail",
			Question: "How detailed should meeting summaries be?",
			Default:  "standard",
			Options:  []string{"brief", "standard", "detailed"},
			Section:  "Summary Settings",
			Type:     tasks.QuestionChoice,
		},
		{
			Key:      "extract_actions",
			Question: "Should I extract action items from meetings?",
			Default:  "yes",
			Section:  "Summary Settings",
			Type:     tasks.QuestionConfirm,
		},
		{
			Key:      "include_sentiment",
			Question: "Should I note the overall tone/sentiment of meetings?",
			Default:  "no",
			Section:  "Summary Settings",
			Type:     tasks.QuestionConfirm,
		},
	}
}

// Run executes the task with given options.
func (t *Task) Run(ctx context.Context, opts tasks.RunOptions) (*tasks.Result, error) {
	// Validate we have an event ID
	if len(opts.Args) == 0 {
		return nil, fmt.Errorf("event ID required: hal9000 onetoone run <event-id>")
	}
	eventID := opts.Args[0]

	// Create transcript fetcher
	fetcher, err := transcript.NewFetcher(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize transcript fetcher: %w", err)
	}

	// Fetch transcript for the event
	tr, err := fetcher.FetchForEvent(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transcript: %w", err)
	}

	// Parse transcript entries to identify speakers
	entries := transcript.ParseEntries(tr.Text)

	// Identify the other person (not self)
	otherPerson := identifyOtherPerson(tr.Speakers, entries)
	if otherPerson == "" {
		return nil, fmt.Errorf("could not identify the other person in the meeting")
	}

	// Load or create profile for the other person
	profile, err := loadOrCreateProfile(otherPerson)
	if err != nil {
		return nil, fmt.Errorf("failed to load profile: %w", err)
	}

	// Generate summary
	summary := generateSummary(tr, entries, opts)

	// Update profile with new interaction
	updateProfile(profile, tr, summary)

	// Save profile
	profilePath, err := saveProfile(profile)
	if err != nil {
		return nil, fmt.Errorf("failed to save profile: %w", err)
	}

	if opts.DryRun {
		return &tasks.Result{
			Success: true,
			Output:  formatSummary(summary),
			Message: fmt.Sprintf("Would update profile for %s at %s", otherPerson, profilePath),
		}, nil
	}

	return &tasks.Result{
		Success:    true,
		Output:     formatSummary(summary),
		OutputPath: profilePath,
		Message:    fmt.Sprintf("Updated profile for %s", otherPerson),
		Metadata: map[string]interface{}{
			"person":       otherPerson,
			"event_id":     eventID,
			"event_title":  tr.EventTitle,
			"profile_path": profilePath,
		},
	}, nil
}

// PeopleProfile represents a person's profile in the library.
type PeopleProfile struct {
	Name               string        `json:"name"`
	Email              string        `json:"email,omitempty"`
	Company            string        `json:"company,omitempty"`
	Role               string        `json:"role,omitempty"`
	RecentInteractions []Interaction `json:"recent_interactions"`
	OpenItems          []ActionItem  `json:"open_items"`
	Notes              string        `json:"notes,omitempty"`
	CreatedAt          time.Time     `json:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at"`
}

// Interaction represents a recorded interaction with a person.
type Interaction struct {
	Date       time.Time `json:"date"`
	Type       string    `json:"type"` // "1:1", "meeting", "email", etc.
	Title      string    `json:"title"`
	Summary    string    `json:"summary"`
	Topics     []string  `json:"topics,omitempty"`
	Decisions  []string  `json:"decisions,omitempty"`
	ActionItems []ActionItem `json:"action_items,omitempty"`
	Sentiment  string    `json:"sentiment,omitempty"`
	EventID    string    `json:"event_id,omitempty"`
}

// ActionItem represents a tracked action item.
type ActionItem struct {
	Description string    `json:"description"`
	Owner       string    `json:"owner"` // "me" or person's name
	DueDate     string    `json:"due_date,omitempty"`
	Status      string    `json:"status"` // "open", "completed"
	CreatedAt   time.Time `json:"created_at"`
	Source      string    `json:"source,omitempty"` // meeting title/date
}

// MeetingSummary contains the generated summary for a meeting.
type MeetingSummary struct {
	Topics      []string     `json:"topics"`
	Decisions   []string     `json:"decisions"`
	ActionItems []ActionItem `json:"action_items"`
	Summary     string       `json:"summary"`
	Sentiment   string       `json:"sentiment,omitempty"`
}

// identifyOtherPerson identifies the other participant in a 1:1 meeting.
func identifyOtherPerson(speakers []string, entries []transcript.TranscriptEntry) string {
	// For now, use a simple heuristic: the other speaker is anyone who isn't "me"
	// In practice, we'd match against the user's configured name/email
	// TODO: Load user's name from preferences/config

	// Count speaker turns to find the most active participant
	speakerCounts := make(map[string]int)
	for _, entry := range entries {
		if entry.Speaker != "" {
			speakerCounts[entry.Speaker]++
		}
	}

	// If we have speakers from the transcript metadata, use those
	if len(speakers) >= 2 {
		// Return the first speaker that isn't likely "me"
		// This is a simplification - real implementation would check against config
		return speakers[0]
	}

	// Otherwise use the most frequent speaker from entries
	var topSpeaker string
	var topCount int
	for speaker, count := range speakerCounts {
		if count > topCount {
			topCount = count
			topSpeaker = speaker
		}
	}

	return topSpeaker
}

// loadOrCreateProfile loads an existing profile or creates a new one.
func loadOrCreateProfile(name string) (*PeopleProfile, error) {
	profilePath := getProfilePath(name)

	data, err := os.ReadFile(profilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create new profile
			return &PeopleProfile{
				Name:               name,
				RecentInteractions: []Interaction{},
				OpenItems:          []ActionItem{},
				CreatedAt:          time.Now(),
				UpdatedAt:          time.Now(),
			}, nil
		}
		return nil, err
	}

	var profile PeopleProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("failed to parse profile: %w", err)
	}

	return &profile, nil
}

// saveProfile saves a profile to the library.
func saveProfile(profile *PeopleProfile) (string, error) {
	profile.UpdatedAt = time.Now()

	profilePath := getProfilePath(profile.Name)
	dir := filepath.Dir(profilePath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create profiles directory: %w", err)
	}

	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal profile: %w", err)
	}

	if err := os.WriteFile(profilePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write profile: %w", err)
	}

	return profilePath, nil
}

// getProfilePath returns the file path for a person's profile.
func getProfilePath(name string) string {
	basePath := expandPath(defaultLibraryPath)
	safeName := sanitizeName(name)
	return filepath.Join(basePath, profilesSubdir, safeName+".json")
}

// generateSummary generates a summary from the transcript.
// TODO: This should use an LLM for real summarization.
func generateSummary(tr *transcript.Transcript, entries []transcript.TranscriptEntry, opts tasks.RunOptions) *MeetingSummary {
	summary := &MeetingSummary{
		Topics:      []string{},
		Decisions:   []string{},
		ActionItems: []ActionItem{},
	}

	// Extract detail level from preferences/overrides
	detailLevel := "standard"
	if level, ok := opts.Overrides["summary_detail"]; ok {
		detailLevel = level
	}

	// For now, generate a basic summary from transcript metadata
	// Real implementation would call an LLM
	switch detailLevel {
	case "brief":
		summary.Summary = fmt.Sprintf("1:1 meeting on %s: %s",
			tr.EventTime.Format("Jan 2, 2006"), tr.EventTitle)
	case "detailed":
		summary.Summary = fmt.Sprintf("Detailed 1:1 meeting on %s regarding %s. "+
			"Discussion involved %d speakers with %d conversation turns.",
			tr.EventTime.Format("Jan 2, 2006 at 3:04 PM"),
			tr.EventTitle,
			len(tr.Speakers),
			len(entries))
	default: // standard
		summary.Summary = fmt.Sprintf("1:1 meeting on %s: %s with %d participants",
			tr.EventTime.Format("Jan 2, 2006"), tr.EventTitle, len(tr.Speakers))
	}

	// Extract basic topics from the first few entries
	if len(entries) > 0 {
		seenTopics := make(map[string]bool)
		for i, entry := range entries {
			if i >= 5 { // Limit to first 5 entries for basic topic extraction
				break
			}
			// Simple extraction - real implementation would use NLP/LLM
			words := strings.Fields(entry.Text)
			if len(words) > 3 {
				topic := strings.Join(words[:3], " ") + "..."
				if !seenTopics[topic] {
					seenTopics[topic] = true
					summary.Topics = append(summary.Topics, topic)
				}
			}
		}
	}

	return summary
}

// updateProfile updates a profile with a new interaction.
func updateProfile(profile *PeopleProfile, tr *transcript.Transcript, summary *MeetingSummary) {
	interaction := Interaction{
		Date:        tr.EventTime,
		Type:        "1:1",
		Title:       tr.EventTitle,
		Summary:     summary.Summary,
		Topics:      summary.Topics,
		Decisions:   summary.Decisions,
		ActionItems: summary.ActionItems,
		Sentiment:   summary.Sentiment,
		EventID:     tr.EventID,
	}

	// Prepend new interaction (most recent first)
	profile.RecentInteractions = append([]Interaction{interaction}, profile.RecentInteractions...)

	// Keep only last 20 interactions
	if len(profile.RecentInteractions) > 20 {
		profile.RecentInteractions = profile.RecentInteractions[:20]
	}

	// Add new action items to open items
	for _, item := range summary.ActionItems {
		profile.OpenItems = append(profile.OpenItems, item)
	}
}

// formatSummary formats a summary for output.
func formatSummary(summary *MeetingSummary) string {
	var sb strings.Builder

	sb.WriteString("## Meeting Summary\n\n")
	sb.WriteString(summary.Summary)
	sb.WriteString("\n\n")

	if len(summary.Topics) > 0 {
		sb.WriteString("### Topics Discussed\n")
		for _, topic := range summary.Topics {
			sb.WriteString(fmt.Sprintf("- %s\n", topic))
		}
		sb.WriteString("\n")
	}

	if len(summary.Decisions) > 0 {
		sb.WriteString("### Decisions Made\n")
		for _, decision := range summary.Decisions {
			sb.WriteString(fmt.Sprintf("- %s\n", decision))
		}
		sb.WriteString("\n")
	}

	if len(summary.ActionItems) > 0 {
		sb.WriteString("### Action Items\n")
		for _, item := range summary.ActionItems {
			owner := item.Owner
			if owner == "" {
				owner = "TBD"
			}
			sb.WriteString(fmt.Sprintf("- [ ] %s (%s)\n", item.Description, owner))
		}
		sb.WriteString("\n")
	}

	if summary.Sentiment != "" {
		sb.WriteString(fmt.Sprintf("**Sentiment**: %s\n", summary.Sentiment))
	}

	return sb.String()
}

// expandPath expands ~ to home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// sanitizeName makes a name safe for use in filenames.
func sanitizeName(name string) string {
	// Replace spaces with underscores and remove problematic characters
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "_")

	var result strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			result.WriteRune(c)
		}
	}
	return result.String()
}
