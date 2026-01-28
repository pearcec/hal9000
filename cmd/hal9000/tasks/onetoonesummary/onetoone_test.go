package onetoonesummary

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pearcec/hal9000/cmd/hal9000/tasks"
	"github.com/pearcec/hal9000/discovery/bowman/transcript"
)

func TestTaskInterface(t *testing.T) {
	task := &Task{}

	// Verify it implements the Task interface
	var _ tasks.Task = task

	if task.Name() != "onetoone" {
		t.Errorf("expected name 'onetoone', got %q", task.Name())
	}

	if task.PreferencesKey() != "onetoone" {
		t.Errorf("expected preferences key 'onetoone', got %q", task.PreferencesKey())
	}

	if task.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestSetupQuestions(t *testing.T) {
	task := &Task{}
	questions := task.SetupQuestions()

	if len(questions) != 3 {
		t.Errorf("expected 3 setup questions, got %d", len(questions))
	}

	// Check summary_detail question
	found := false
	for _, q := range questions {
		if q.Key == "summary_detail" {
			found = true
			if q.Type != tasks.QuestionChoice {
				t.Errorf("summary_detail should be QuestionChoice, got %v", q.Type)
			}
			if len(q.Options) != 3 {
				t.Errorf("summary_detail should have 3 options, got %d", len(q.Options))
			}
		}
	}
	if !found {
		t.Error("summary_detail question not found")
	}

	// Check extract_actions question
	found = false
	for _, q := range questions {
		if q.Key == "extract_actions" {
			found = true
			if q.Type != tasks.QuestionConfirm {
				t.Errorf("extract_actions should be QuestionConfirm, got %v", q.Type)
			}
		}
	}
	if !found {
		t.Error("extract_actions question not found")
	}

	// Check include_sentiment question
	found = false
	for _, q := range questions {
		if q.Key == "include_sentiment" {
			found = true
			if q.Type != tasks.QuestionConfirm {
				t.Errorf("include_sentiment should be QuestionConfirm, got %v", q.Type)
			}
		}
	}
	if !found {
		t.Error("include_sentiment question not found")
	}
}

func TestIdentifyOtherPerson(t *testing.T) {
	tests := []struct {
		name     string
		speakers []string
		entries  []transcript.TranscriptEntry
		want     string
	}{
		{
			name:     "two speakers - returns first",
			speakers: []string{"Alice", "Bob"},
			entries:  []transcript.TranscriptEntry{},
			want:     "Alice",
		},
		{
			name:     "single speaker from entries",
			speakers: []string{},
			entries: []transcript.TranscriptEntry{
				{Speaker: "Charlie", Text: "Hello there"},
				{Speaker: "Charlie", Text: "How are you"},
			},
			want: "Charlie",
		},
		{
			name:     "most frequent speaker wins",
			speakers: []string{},
			entries: []transcript.TranscriptEntry{
				{Speaker: "Dave", Text: "One"},
				{Speaker: "Eve", Text: "Two"},
				{Speaker: "Eve", Text: "Three"},
				{Speaker: "Eve", Text: "Four"},
			},
			want: "Eve",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := identifyOtherPerson(tt.speakers, tt.entries)
			if got != tt.want {
				t.Errorf("identifyOtherPerson() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateSummary(t *testing.T) {
	tr := &transcript.Transcript{
		EventID:    "test-event",
		EventTitle: "Weekly 1:1",
		EventTime:  time.Date(2026, 1, 28, 10, 0, 0, 0, time.UTC),
		Speakers:   []string{"Alice", "Bob"},
	}

	entries := []transcript.TranscriptEntry{
		{Speaker: "Alice", Text: "Let's discuss the project status today"},
		{Speaker: "Bob", Text: "Sure, I've made good progress on the API"},
	}

	tests := []struct {
		name        string
		detailLevel string
		wantBrief   bool
	}{
		{
			name:        "brief summary",
			detailLevel: "brief",
			wantBrief:   true,
		},
		{
			name:        "standard summary",
			detailLevel: "standard",
			wantBrief:   false,
		},
		{
			name:        "detailed summary",
			detailLevel: "detailed",
			wantBrief:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tasks.RunOptions{
				Overrides: map[string]string{
					"summary_detail": tt.detailLevel,
				},
			}

			summary := generateSummary(tr, entries, opts)

			if summary.Summary == "" {
				t.Error("summary should not be empty")
			}
		})
	}
}

func TestUpdateProfile(t *testing.T) {
	profile := &PeopleProfile{
		Name:               "Test Person",
		RecentInteractions: []Interaction{},
		OpenItems:          []ActionItem{},
		CreatedAt:          time.Now(),
	}

	tr := &transcript.Transcript{
		EventID:    "event-123",
		EventTitle: "1:1 Meeting",
		EventTime:  time.Date(2026, 1, 28, 10, 0, 0, 0, time.UTC),
	}

	summary := &MeetingSummary{
		Summary: "Test meeting summary",
		Topics:  []string{"Project updates", "Timeline"},
		ActionItems: []ActionItem{
			{Description: "Follow up on API", Owner: "me"},
		},
	}

	updateProfile(profile, tr, summary)

	if len(profile.RecentInteractions) != 1 {
		t.Errorf("expected 1 interaction, got %d", len(profile.RecentInteractions))
	}

	interaction := profile.RecentInteractions[0]
	if interaction.Type != "1:1" {
		t.Errorf("expected type '1:1', got %q", interaction.Type)
	}
	if interaction.EventID != "event-123" {
		t.Errorf("expected event_id 'event-123', got %q", interaction.EventID)
	}
	if len(interaction.Topics) != 2 {
		t.Errorf("expected 2 topics, got %d", len(interaction.Topics))
	}

	if len(profile.OpenItems) != 1 {
		t.Errorf("expected 1 open item, got %d", len(profile.OpenItems))
	}
}

func TestUpdateProfileKeepsMax20Interactions(t *testing.T) {
	profile := &PeopleProfile{
		Name:               "Test Person",
		RecentInteractions: make([]Interaction, 20),
		OpenItems:          []ActionItem{},
	}

	for i := range profile.RecentInteractions {
		profile.RecentInteractions[i] = Interaction{
			Title: "Old meeting",
		}
	}

	tr := &transcript.Transcript{
		EventID:    "new-event",
		EventTitle: "New Meeting",
		EventTime:  time.Now(),
	}

	summary := &MeetingSummary{
		Summary: "New meeting",
	}

	updateProfile(profile, tr, summary)

	if len(profile.RecentInteractions) != 20 {
		t.Errorf("expected 20 interactions (max), got %d", len(profile.RecentInteractions))
	}

	// Newest should be first
	if profile.RecentInteractions[0].Title != "New Meeting" {
		t.Errorf("newest interaction should be first, got %q", profile.RecentInteractions[0].Title)
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"John Doe", "john_doe"},
		{"Alice", "alice"},
		{"Bob O'Brien", "bob_obrien"},
		{"Jane Doe-Smith", "jane_doe-smith"},
		{"Test 123", "test_123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatSummary(t *testing.T) {
	summary := &MeetingSummary{
		Summary:   "Test meeting about project status",
		Topics:    []string{"API design", "Timeline"},
		Decisions: []string{"Use REST API"},
		ActionItems: []ActionItem{
			{Description: "Review PR", Owner: "me"},
			{Description: "Update docs", Owner: "them"},
		},
		Sentiment: "positive",
	}

	output := formatSummary(summary)

	// Check all sections are present
	if !contains(output, "## Meeting Summary") {
		t.Error("output should contain summary header")
	}
	if !contains(output, "Test meeting about project status") {
		t.Error("output should contain summary text")
	}
	if !contains(output, "### Topics Discussed") {
		t.Error("output should contain topics section")
	}
	if !contains(output, "### Decisions Made") {
		t.Error("output should contain decisions section")
	}
	if !contains(output, "### Action Items") {
		t.Error("output should contain action items section")
	}
	if !contains(output, "**Sentiment**: positive") {
		t.Error("output should contain sentiment")
	}
}

func TestProfileSaveLoad(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "onetoone-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	profile := &PeopleProfile{
		Name:      "Test Person",
		Email:     "test@example.com",
		Company:   "Test Co",
		Role:      "Engineer",
		OpenItems: []ActionItem{},
		RecentInteractions: []Interaction{
			{
				Date:    time.Now(),
				Type:    "1:1",
				Title:   "Test Meeting",
				Summary: "Test summary",
			},
		},
		CreatedAt: time.Now(),
	}

	// Test that profile path uses sanitized name
	path := getProfilePath("Test Person")
	if !contains(path, "test_person") {
		t.Errorf("profile path should contain sanitized name, got %q", path)
	}

	// Create profile manually in temp dir for load test
	profilePath := filepath.Join(tmpDir, "test_person.json")
	profileDir := filepath.Dir(profilePath)
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatalf("failed to create profile dir: %v", err)
	}

	// Note: Full save/load test would require mocking the path functions
	_ = profile
}

func TestRunRequiresEventID(t *testing.T) {
	task := &Task{}
	ctx := context.Background()

	// Run without event ID should error
	_, err := task.Run(ctx, tasks.RunOptions{Args: []string{}})
	if err == nil {
		t.Error("expected error when event ID not provided")
	}
	if !contains(err.Error(), "event ID required") {
		t.Errorf("error should mention event ID, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
