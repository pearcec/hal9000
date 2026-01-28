package collabsummary

import (
	"context"
	"testing"
	"time"

	"github.com/pearcec/hal9000/cmd/hal9000/tasks"
	"github.com/pearcec/hal9000/discovery/bowman/transcript"
)

func TestTaskInterface(t *testing.T) {
	task := &Task{}

	// Verify it implements the Task interface
	var _ tasks.Task = task

	if task.Name() != "collab" {
		t.Errorf("expected name 'collab', got %q", task.Name())
	}

	if task.PreferencesKey() != "collab" {
		t.Errorf("expected preferences key 'collab', got %q", task.PreferencesKey())
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

	// Check auto_create_collab question
	found = false
	for _, q := range questions {
		if q.Key == "auto_create_collab" {
			found = true
			if q.Type != tasks.QuestionConfirm {
				t.Errorf("auto_create_collab should be QuestionConfirm, got %v", q.Type)
			}
		}
	}
	if !found {
		t.Error("auto_create_collab question not found")
	}

	// Check track_decisions question
	found = false
	for _, q := range questions {
		if q.Key == "track_decisions" {
			found = true
			if q.Type != tasks.QuestionConfirm {
				t.Errorf("track_decisions should be QuestionConfirm, got %v", q.Type)
			}
		}
	}
	if !found {
		t.Error("track_decisions question not found")
	}
}

func TestMatchesTitlePattern(t *testing.T) {
	tests := []struct {
		title    string
		patterns []string
		want     bool
	}{
		{
			title:    "Weekly Team Sync",
			patterns: []string{"Team Sync"},
			want:     true,
		},
		{
			title:    "Project Alpha Standup",
			patterns: []string{"Project Alpha"},
			want:     true,
		},
		{
			title:    "Random Meeting",
			patterns: []string{"Team Sync", "Project Alpha"},
			want:     false,
		},
		{
			title:    "team sync",
			patterns: []string{"Team Sync"},
			want:     true, // case insensitive
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := matchesTitlePattern(tt.title, tt.patterns)
			if got != tt.want {
				t.Errorf("matchesTitlePattern(%q, %v) = %v, want %v", tt.title, tt.patterns, got, tt.want)
			}
		})
	}
}

func TestHasSignificantOverlap(t *testing.T) {
	tests := []struct {
		name         string
		speakers     []string
		participants []Participant
		want         bool
	}{
		{
			name:     "100% overlap",
			speakers: []string{"Alice", "Bob"},
			participants: []Participant{
				{Name: "Alice"},
				{Name: "Bob"},
			},
			want: true,
		},
		{
			name:     "50% overlap - not significant",
			speakers: []string{"Alice", "Bob"},
			participants: []Participant{
				{Name: "Alice"},
				{Name: "Charlie"},
			},
			want: false,
		},
		{
			name:     "66% overlap - significant",
			speakers: []string{"Alice", "Bob", "Charlie"},
			participants: []Participant{
				{Name: "Alice"},
				{Name: "Bob"},
			},
			want: true,
		},
		{
			name:         "empty speakers",
			speakers:     []string{},
			participants: []Participant{{Name: "Alice"}},
			want:         false,
		},
		{
			name:         "empty participants",
			speakers:     []string{"Alice"},
			participants: []Participant{},
			want:         false,
		},
		{
			name:     "case insensitive",
			speakers: []string{"alice", "BOB"},
			participants: []Participant{
				{Name: "Alice"},
				{Name: "Bob"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasSignificantOverlap(tt.speakers, tt.participants)
			if got != tt.want {
				t.Errorf("hasSignificantOverlap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeriveCollabName(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"Weekly Team Sync", "Team"},
		{"Daily Standup", "Standup"},
		{"Project Alpha Meeting", "Project Alpha"},
		{"Bi-weekly Review Meeting", "Review"},
		{"Just a Meeting", "Just a"},
		{"", "Ad-hoc Meeting"},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := deriveCollabName(tt.title)
			if got != tt.want {
				t.Errorf("deriveCollabName(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

func TestSpeakersToParticipants(t *testing.T) {
	speakers := []string{"Alice", "Bob", "Charlie"}
	participants := speakersToParticipants(speakers)

	if len(participants) != 3 {
		t.Errorf("expected 3 participants, got %d", len(participants))
	}

	for i, p := range participants {
		if p.Name != speakers[i] {
			t.Errorf("participant %d name = %q, want %q", i, p.Name, speakers[i])
		}
	}
}

func TestGenerateSummary(t *testing.T) {
	tr := &transcript.Transcript{
		EventID:    "test-event",
		EventTitle: "Team Sync",
		EventTime:  time.Date(2026, 1, 28, 10, 0, 0, 0, time.UTC),
		Speakers:   []string{"Alice", "Bob", "Charlie"},
	}

	entries := []transcript.TranscriptEntry{
		{Speaker: "Alice", Text: "Let's discuss the project status today"},
		{Speaker: "Bob", Text: "Sure, I've made good progress on the API"},
		{Speaker: "Charlie", Text: "I can help with the frontend work"},
	}

	tests := []struct {
		name        string
		detailLevel string
	}{
		{"brief summary", "brief"},
		{"standard summary", "standard"},
		{"detailed summary", "detailed"},
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

			if len(summary.Participants) != 3 {
				t.Errorf("expected 3 participants, got %d", len(summary.Participants))
			}
		})
	}
}

func TestUpdateCollaboration(t *testing.T) {
	collab := &Collaboration{
		Name:           "Test Collab",
		Participants:   []Participant{{Name: "Alice"}},
		RecentSessions: []Session{},
		DecisionsLog:   []Decision{},
		OpenActions:    []ActionItem{},
		CreatedAt:      time.Now(),
	}

	tr := &transcript.Transcript{
		EventID:    "event-123",
		EventTitle: "Team Sync",
		EventTime:  time.Date(2026, 1, 28, 10, 0, 0, 0, time.UTC),
		Speakers:   []string{"Alice", "Bob"},
	}

	summary := &MeetingSummary{
		Summary:      "Test meeting summary",
		Topics:       []string{"Project updates", "Timeline"},
		Participants: []string{"Alice", "Bob"},
		Decisions: []Decision{
			{Description: "Use REST API"},
		},
		ActionItems: []ActionItem{
			{Description: "Review PR", Owner: "Bob"},
		},
	}

	updateCollaboration(collab, tr, summary)

	if len(collab.RecentSessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(collab.RecentSessions))
	}

	session := collab.RecentSessions[0]
	if session.Title != "Team Sync" {
		t.Errorf("expected title 'Team Sync', got %q", session.Title)
	}
	if session.EventID != "event-123" {
		t.Errorf("expected event_id 'event-123', got %q", session.EventID)
	}

	if len(collab.DecisionsLog) != 1 {
		t.Errorf("expected 1 decision, got %d", len(collab.DecisionsLog))
	}

	if len(collab.OpenActions) != 1 {
		t.Errorf("expected 1 open action, got %d", len(collab.OpenActions))
	}

	// Check that Bob was added as a participant
	if len(collab.Participants) != 2 {
		t.Errorf("expected 2 participants (Alice + Bob), got %d", len(collab.Participants))
	}
}

func TestUpdateCollaborationKeepsMax30Sessions(t *testing.T) {
	collab := &Collaboration{
		Name:           "Test Collab",
		Participants:   []Participant{},
		RecentSessions: make([]Session, 30),
		DecisionsLog:   []Decision{},
		OpenActions:    []ActionItem{},
	}

	for i := range collab.RecentSessions {
		collab.RecentSessions[i] = Session{Title: "Old session"}
	}

	tr := &transcript.Transcript{
		EventID:    "new-event",
		EventTitle: "New Session",
		EventTime:  time.Now(),
	}

	summary := &MeetingSummary{
		Summary:      "New session",
		Participants: []string{},
	}

	updateCollaboration(collab, tr, summary)

	if len(collab.RecentSessions) != 30 {
		t.Errorf("expected 30 sessions (max), got %d", len(collab.RecentSessions))
	}

	// Newest should be first
	if collab.RecentSessions[0].Title != "New Session" {
		t.Errorf("newest session should be first, got %q", collab.RecentSessions[0].Title)
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Team Alpha", "team_alpha"},
		{"Project-Beta", "project-beta"},
		{"Test 123", "test_123"},
		{"A/B Testing", "ab_testing"},
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
		Summary:      "Test team meeting about project status",
		Participants: []string{"Alice", "Bob", "Charlie"},
		Topics:       []string{"API design", "Timeline"},
		KeyPoints:    []string{"Need to prioritize API"},
		Decisions: []Decision{
			{Description: "Use REST API", MadeBy: "Alice"},
		},
		ActionItems: []ActionItem{
			{Description: "Review PR", Owner: "Bob"},
			{Description: "Update docs", Owner: "Charlie"},
		},
	}

	output := formatSummary(summary)

	// Check all sections are present
	if !contains(output, "## Meeting Summary") {
		t.Error("output should contain summary header")
	}
	if !contains(output, "Test team meeting") {
		t.Error("output should contain summary text")
	}
	if !contains(output, "### Participants") {
		t.Error("output should contain participants section")
	}
	if !contains(output, "Alice, Bob, Charlie") {
		t.Error("output should contain participant names")
	}
	if !contains(output, "### Topics Discussed") {
		t.Error("output should contain topics section")
	}
	if !contains(output, "### Decisions Made") {
		t.Error("output should contain decisions section")
	}
	if !contains(output, "(by Alice)") {
		t.Error("output should contain decision maker")
	}
	if !contains(output, "### Action Items") {
		t.Error("output should contain action items section")
	}
	if !contains(output, "@Bob") {
		t.Error("output should contain action item owner")
	}
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
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
