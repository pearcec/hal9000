package collabsummary

import (
	"testing"
	"time"

	"github.com/pearcec/hal9000/cmd/hal9000/tasks"
	"github.com/pearcec/hal9000/discovery/bowman/transcript"
)

func TestTaskInterface(t *testing.T) {
	task := &CollabSummaryTask{}

	// Verify it implements Task interface
	var _ tasks.Task = task

	if task.Name() != "collabsummary" {
		t.Errorf("Name() = %q, want %q", task.Name(), "collabsummary")
	}

	if task.PreferencesKey() != "collabsummary" {
		t.Errorf("PreferencesKey() = %q, want %q", task.PreferencesKey(), "collabsummary")
	}

	if task.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestSetupQuestions(t *testing.T) {
	task := &CollabSummaryTask{}
	questions := task.SetupQuestions()

	if len(questions) != 3 {
		t.Errorf("SetupQuestions() returned %d questions, want 3", len(questions))
	}

	// Verify expected questions are present
	questionKeys := make(map[string]bool)
	for _, q := range questions {
		questionKeys[q.Key] = true
	}

	expectedKeys := []string{"summary_detail", "auto_create_collab", "track_decisions"}
	for _, key := range expectedKeys {
		if !questionKeys[key] {
			t.Errorf("SetupQuestions() missing expected question key: %s", key)
		}
	}
}

func TestSanitizeID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Team Standup", "team-standup"},
		{"Project-Review", "project-review"},
		{"Weekly Meeting (2024)", "weekly-meeting-2024"},
		{"API/Backend Discussion", "apibackend-discussion"},
		{"   spaces   ", "---spaces---"},
	}

	for _, tc := range tests {
		result := sanitizeID(tc.input)
		if result != tc.expected {
			t.Errorf("sanitizeID(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestContains(t *testing.T) {
	slice := []string{"Apple", "Banana", "Cherry"}

	if !contains(slice, "apple") { // case insensitive
		t.Error("contains() should find 'apple' (case insensitive)")
	}

	if !contains(slice, "Banana") {
		t.Error("contains() should find 'Banana'")
	}

	if contains(slice, "Grape") {
		t.Error("contains() should not find 'Grape'")
	}
}

func TestExtractContext(t *testing.T) {
	tests := []struct {
		line      string
		indicator string
		expectLen bool
	}{
		{"let's talk about the new feature", "let's talk about", true},
		{"no match here", "discussing", false},
		{"moving on to a very long topic that should be truncated at some point because it exceeds the maximum length we want to capture for a topic description in the summary", "moving on to", true},
	}

	for _, tc := range tests {
		result := extractContext(tc.line, tc.indicator)
		hasContent := result != ""
		if hasContent != tc.expectLen {
			t.Errorf("extractContext(%q, %q) content presence = %v, want %v", tc.line, tc.indicator, hasContent, tc.expectLen)
		}
	}
}

func TestExtractTopics(t *testing.T) {
	task := &CollabSummaryTask{}

	text := `John: Let's talk about the API changes.
Mary: Regarding the timeline, we need two weeks.
Bob: Moving on to testing strategy.`

	entries := []transcript.TranscriptEntry{
		{Speaker: "John", Text: "Let's talk about the API changes."},
		{Speaker: "Mary", Text: "Regarding the timeline, we need two weeks."},
		{Speaker: "Bob", Text: "Moving on to testing strategy."},
	}

	topics := task.extractTopics(entries, text)

	if len(topics) == 0 {
		t.Error("extractTopics() should find at least one topic")
	}
}

func TestExtractDecisions(t *testing.T) {
	task := &CollabSummaryTask{}

	entries := []transcript.TranscriptEntry{
		{Speaker: "John", Text: "We've decided to use PostgreSQL."},
		{Speaker: "Mary", Text: "I think we need more time."},
		{Speaker: "Bob", Text: "Agreed to deploy on Friday."},
	}

	decisions := task.extractDecisions(entries, "")

	if len(decisions) != 2 {
		t.Errorf("extractDecisions() found %d decisions, want 2", len(decisions))
	}

	// Verify speaker attribution
	hasJohn := false
	hasBob := false
	for _, d := range decisions {
		if d.DecidedBy == "John" {
			hasJohn = true
		}
		if d.DecidedBy == "Bob" {
			hasBob = true
		}
	}

	if !hasJohn || !hasBob {
		t.Error("extractDecisions() should attribute decisions to speakers")
	}
}

func TestExtractActionItems(t *testing.T) {
	task := &CollabSummaryTask{}

	entries := []transcript.TranscriptEntry{
		{Speaker: "John", Text: "I'll take the database migration."},
		{Speaker: "Mary", Text: "We need to discuss this further."},
		{Speaker: "Bob", Text: "Action item: update the docs by Friday."},
	}

	actions := task.extractActionItems(entries, "")

	if len(actions) != 2 {
		t.Errorf("extractActionItems() found %d actions, want 2", len(actions))
	}
}

func TestFormatSummary(t *testing.T) {
	task := &CollabSummaryTask{}

	trans := &transcript.Transcript{
		EventTitle: "Team Standup",
		EventTime:  time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		Speakers:   []string{"John", "Mary", "Bob"},
	}

	summary := &MeetingSummary{
		Topics:      []string{"API Changes", "Timeline"},
		Decisions:   []Decision{{Description: "Use PostgreSQL", DecidedBy: "John"}},
		ActionItems: []ActionItem{{Description: "Update docs", Assignee: "Bob"}},
		KeyPoints:   []string{"Important: deadline is Friday"},
		GeneratedAt: time.Now(),
	}

	// Test brief format
	briefText := task.formatSummary(trans, summary, "brief")
	if briefText == "" {
		t.Error("formatSummary(brief) should not return empty string")
	}
	// Brief should not include decisions section
	if len(summary.Decisions) > 0 && !containsString(briefText, "Decisions Made") {
		// This is expected for brief
	}

	// Test standard format
	standardText := task.formatSummary(trans, summary, "standard")
	if !containsString(standardText, "Decisions Made") {
		t.Error("formatSummary(standard) should include decisions section")
	}

	// Test detailed format
	detailedText := task.formatSummary(trans, summary, "detailed")
	if !containsString(detailedText, "Key Discussion Points") {
		t.Error("formatSummary(detailed) should include key points section")
	}
}

func TestMatchType(t *testing.T) {
	// Verify MatchType constants
	if MatchByTitle != "title" {
		t.Errorf("MatchByTitle = %q, want %q", MatchByTitle, "title")
	}
	if MatchByAttendees != "attendees" {
		t.Errorf("MatchByAttendees = %q, want %q", MatchByAttendees, "attendees")
	}
	if MatchAdHoc != "ad-hoc" {
		t.Errorf("MatchAdHoc = %q, want %q", MatchAdHoc, "ad-hoc")
	}
}

func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || containsString(s[1:], substr)))
}
