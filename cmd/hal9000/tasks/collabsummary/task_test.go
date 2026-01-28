package collabsummary

import (
	"testing"
	"time"

	"github.com/pearcec/hal9000/cmd/hal9000/tasks"
	"github.com/pearcec/hal9000/discovery/bowman/transcript"
)

func TestTaskInterface(t *testing.T) {
	task := &Task{}

	// Verify interface compliance
	var _ tasks.Task = task

	if task.Name() != "collabsummary" {
		t.Errorf("Name() = %q, want %q", task.Name(), "collabsummary")
	}

	if task.PreferencesKey() != "collaboration" {
		t.Errorf("PreferencesKey() = %q, want %q", task.PreferencesKey(), "collaboration")
	}

	if task.Description() == "" {
		t.Error("Description() should not be empty")
	}

	questions := task.SetupQuestions()
	if len(questions) != 3 {
		t.Errorf("SetupQuestions() returned %d questions, want 3", len(questions))
	}

	// Check setup question keys
	expectedKeys := map[string]bool{
		"summary_detail":     false,
		"auto_create_collab": false,
		"track_decisions":    false,
	}

	for _, q := range questions {
		if _, ok := expectedKeys[q.Key]; ok {
			expectedKeys[q.Key] = true
		}
	}

	for key, found := range expectedKeys {
		if !found {
			t.Errorf("SetupQuestions() missing expected key: %s", key)
		}
	}
}

func TestExtractTopics(t *testing.T) {
	task := &Task{}

	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "topic indicator",
			text:     "Let's talk about the quarterly review.",
			expected: 1,
		},
		{
			name:     "multiple topics",
			text:     "Let's talk about budget. Moving on to hiring plans.",
			expected: 2,
		},
		{
			name:     "no topics",
			text:     "Hello everyone, welcome to the meeting.",
			expected: 0,
		},
		{
			name:     "agenda item",
			text:     "Agenda item: Performance review process",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topics := task.extractTopics(tt.text, nil)
			if len(topics) != tt.expected {
				t.Errorf("extractTopics() = %d topics, want %d", len(topics), tt.expected)
			}
		})
	}
}

func TestExtractDecisions(t *testing.T) {
	task := &Task{}

	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "decided statement",
			text:     "We decided to postpone the launch.",
			expected: 1,
		},
		{
			name:     "going with",
			text:     "We're going with option A for the redesign.",
			expected: 1,
		},
		{
			name:     "agreed statement",
			text:     "Agreed: we'll use the new framework.",
			expected: 1,
		},
		{
			name:     "no decisions",
			text:     "We need to discuss this further.",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decisions := task.extractDecisions(tt.text, nil)
			if len(decisions) != tt.expected {
				t.Errorf("extractDecisions() = %d decisions, want %d", len(decisions), tt.expected)
			}
		})
	}
}

func TestExtractActionItems(t *testing.T) {
	task := &Task{}

	tests := []struct {
		name          string
		text          string
		expectedCount int
		hasAssignee   bool
	}{
		{
			name:          "will action",
			text:          "Dave will update the documentation.",
			expectedCount: 1,
			hasAssignee:   true,
		},
		{
			name:          "action item label",
			text:          "Action item: Review the proposal.",
			expectedCount: 1,
			hasAssignee:   false,
		},
		{
			name:          "follow-up",
			text:          "Follow-up: Schedule a meeting with the team.",
			expectedCount: 1,
			hasAssignee:   false,
		},
		{
			name:          "no actions",
			text:          "The meeting went well.",
			expectedCount: 0,
			hasAssignee:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := task.extractActionItems(tt.text, nil)
			if len(actions) != tt.expectedCount {
				t.Errorf("extractActionItems() = %d actions, want %d", len(actions), tt.expectedCount)
			}
			if len(actions) > 0 && tt.hasAssignee && actions[0].Assignee == "" {
				t.Error("expected action to have an assignee")
			}
		})
	}
}

func TestGenerateSummary(t *testing.T) {
	task := &Task{}

	tr := &transcript.Transcript{
		EventTitle: "Team Standup",
		EventTime:  time.Now(),
		Speakers:   []string{"Alice", "Bob", "Charlie"},
		Text: `Alice: Let's talk about the sprint goals.
Bob: We decided to focus on the mobile app.
Charlie will update the backlog.
Action item: Review the PRs.`,
	}

	tests := []struct {
		name        string
		detailLevel string
	}{
		{"brief", "brief"},
		{"standard", "standard"},
		{"detailed", "detailed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tasks.RunOptions{
				Overrides: map[string]string{
					"summary_detail": tt.detailLevel,
				},
			}
			summary := task.generateSummary(tr, opts)

			if summary.Summary == "" {
				t.Error("Summary should not be empty")
			}

			// Should find at least one topic
			if len(summary.Topics) < 1 {
				t.Error("Should extract at least one topic")
			}

			// Should find at least one decision
			if len(summary.Decisions) < 1 {
				t.Error("Should extract at least one decision")
			}

			// Should find at least one action item
			if len(summary.ActionItems) < 1 {
				t.Error("Should extract at least one action item")
			}
		})
	}
}

func TestFormatOutput(t *testing.T) {
	task := &Task{}

	tr := &transcript.Transcript{
		EventTitle: "Test Meeting",
		EventTime:  time.Now(),
		Speakers:   []string{"Alice"},
	}

	summary := &Summary{
		Summary:     "Test summary",
		Topics:      []string{"Topic 1"},
		Decisions:   []Decision{{Description: "Decision 1"}},
		ActionItems: []ActionItem{{Task: "Task 1", Assignee: "Alice"}},
	}

	tests := []struct {
		name     string
		format   string
		contains string
	}{
		{"markdown", "markdown", "# Collaboration Summary"},
		{"text", "text", "COLLABORATION SUMMARY"},
		{"json", "json", `"event_title"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := task.formatOutput(summary, tr, tt.format)
			if output == "" {
				t.Error("formatOutput should return non-empty string")
			}
			if !contains(output, tt.contains) {
				t.Errorf("formatOutput() should contain %q", tt.contains)
			}
		})
	}
}

func TestSanitizeID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with spaces", "with-spaces"},
		{"With-Dashes_and_underscores", "with-dashes_and_underscores"},
		{"Special@#$chars", "special---chars"},
		{"UPPERCASE", "uppercase"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeID(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetPreference(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		overrides map[string]string
		expected  string
	}{
		{
			name:      "default summary_detail",
			key:       "summary_detail",
			overrides: nil,
			expected:  "standard",
		},
		{
			name:     "override summary_detail",
			key:      "summary_detail",
			overrides: map[string]string{"summary_detail": "detailed"},
			expected: "detailed",
		},
		{
			name:      "default auto_create_collab",
			key:       "auto_create_collab",
			overrides: nil,
			expected:  "yes",
		},
		{
			name:      "default track_decisions",
			key:       "track_decisions",
			overrides: nil,
			expected:  "yes",
		},
		{
			name:      "unknown key",
			key:       "unknown",
			overrides: nil,
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tasks.RunOptions{Overrides: tt.overrides}
			result := getPreference(opts, tt.key)
			if result != tt.expected {
				t.Errorf("getPreference(%q) = %q, want %q", tt.key, result, tt.expected)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && searchSubstring(s, substr)))
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
