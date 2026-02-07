package oneononesummary

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

	if task.Name() != "oneonone" {
		t.Errorf("Name() = %q, want %q", task.Name(), "oneonone")
	}

	if task.PreferencesKey() != "oneonone" {
		t.Errorf("PreferencesKey() = %q, want %q", task.PreferencesKey(), "oneonone")
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
		"summary_detail":    false,
		"extract_actions":   false,
		"include_sentiment": false,
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

func TestIdentifyOtherPerson(t *testing.T) {
	task := &Task{}

	tests := []struct {
		name     string
		speakers []string
		expected string
	}{
		{
			name:     "two speakers",
			speakers: []string{"Alice", "Bob"},
			expected: "Bob",
		},
		{
			name:     "single speaker",
			speakers: []string{"Alice"},
			expected: "Alice",
		},
		{
			name:     "no speakers",
			speakers: []string{},
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := task.identifyOtherPerson(tt.speakers)
			if result != tt.expected {
				t.Errorf("identifyOtherPerson() = %q, want %q", result, tt.expected)
			}
		})
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
			name:     "let's talk about",
			text:     "Let's talk about the project timeline.",
			expected: 1,
		},
		{
			name:     "how's question",
			text:     "How's the new feature coming along?",
			expected: 1,
		},
		{
			name:     "no topics",
			text:     "Hello, how are you doing today?",
			expected: 0,
		},
		{
			name:     "multiple topics",
			text:     "Let's talk about budget. Regarding the timeline, we need to discuss.",
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topics := task.extractTopics(tt.text)
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
			text:     "We decided to move forward with the plan.",
			expected: 1,
		},
		{
			name:     "lets go with",
			text:     "Let's go with option A for the design.",
			expected: 1,
		},
		{
			name:     "no decisions",
			text:     "I'm not sure what we should do yet.",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decisions := task.extractDecisions(tt.text)
			if len(decisions) != tt.expected {
				t.Errorf("extractDecisions() = %d decisions, want %d", len(decisions), tt.expected)
			}
		})
	}
}

func TestExtractActionItems(t *testing.T) {
	task := &Task{}

	tests := []struct {
		name              string
		text              string
		speakers          []string
		expectedMyActions int
		expectedTheirs    int
	}{
		{
			name:              "I will action",
			text:              "I'll send you the document tomorrow.",
			speakers:          []string{"Alice", "Bob"},
			expectedMyActions: 1,
			expectedTheirs:    0,
		},
		{
			name:              "you should action",
			text:              "you should review the proposal by Friday.",
			speakers:          []string{"Alice", "Bob"},
			expectedMyActions: 0,
			expectedTheirs:    1,
		},
		{
			name:              "named person action",
			text:              "Bob will prepare the presentation.",
			speakers:          []string{"Alice", "Bob"},
			expectedMyActions: 0,
			expectedTheirs:    1,
		},
		{
			name:              "no actions",
			text:              "That was a good meeting.",
			speakers:          []string{"Alice", "Bob"},
			expectedMyActions: 0,
			expectedTheirs:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			myActions, theirActions := task.extractActionItems(tt.text, nil, tt.speakers)
			if len(myActions) != tt.expectedMyActions {
				t.Errorf("my actions = %d, want %d", len(myActions), tt.expectedMyActions)
			}
			if len(theirActions) != tt.expectedTheirs {
				t.Errorf("their actions = %d, want %d", len(theirActions), tt.expectedTheirs)
			}
		})
	}
}

func TestExtractSentiment(t *testing.T) {
	task := &Task{}

	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{
			name:     "positive",
			text:     "This is great news! I'm really excited about this project. It's going to be awesome!",
			expected: "positive",
		},
		{
			name:     "concerned",
			text:     "I'm worried about the timeline. This is a difficult problem and I'm frustrated with the delays.",
			expected: "concerned",
		},
		{
			name:     "neutral",
			text:     "The meeting went fine. Everything is okay and proceeding normally.",
			expected: "neutral",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sentiment := task.extractSentiment(tt.text)
			if sentiment != tt.expected {
				t.Errorf("extractSentiment() = %q, want %q", sentiment, tt.expected)
			}
		})
	}
}

func TestGenerateSummary(t *testing.T) {
	task := &Task{}

	tr := &transcript.Transcript{
		EventTitle: "Weekly 1:1",
		EventTime:  time.Now(),
		Speakers:   []string{"Alice", "Bob"},
		Text: `Alice: Let's talk about the project status.
Bob: We decided to move forward with plan A.
Alice: I'll send you the updated timeline.
Bob: You should review the budget proposal.`,
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
					"summary_detail":  tt.detailLevel,
					"extract_actions": "yes",
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
		})
	}
}

func TestFormatOutput(t *testing.T) {
	task := &Task{}

	tr := &transcript.Transcript{
		EventTitle: "Test 1:1",
		EventTime:  time.Now(),
		Speakers:   []string{"Alice", "Bob"},
	}

	summary := &Summary{
		Summary:      "Test summary",
		Topics:       []string{"Topic 1"},
		Decisions:    []Decision{{Description: "Decision 1"}},
		MyActions:    []ActionItem{{Task: "My task"}},
		TheirActions: []ActionItem{{Task: "Their task"}},
	}

	tests := []struct {
		name     string
		format   string
		contains string
	}{
		{"markdown", "markdown", "# 1:1 Summary"},
		{"text", "text", "1:1 SUMMARY"},
		{"json", "json", `"with"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := task.formatOutput(summary, tr, "Bob", tt.format)
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
			name:      "override summary_detail",
			key:       "summary_detail",
			overrides: map[string]string{"summary_detail": "detailed"},
			expected:  "detailed",
		},
		{
			name:      "default extract_actions",
			key:       "extract_actions",
			overrides: nil,
			expected:  "yes",
		},
		{
			name:      "default include_sentiment",
			key:       "include_sentiment",
			overrides: nil,
			expected:  "no",
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

func TestDeduplicateStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected int
	}{
		{
			name:     "no duplicates",
			input:    []string{"a", "b", "c"},
			expected: 3,
		},
		{
			name:     "with duplicates",
			input:    []string{"a", "b", "A", "B"},
			expected: 2,
		},
		{
			name:     "all duplicates",
			input:    []string{"a", "a", "a"},
			expected: 1,
		},
		{
			name:     "empty",
			input:    []string{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deduplicateStrings(tt.input)
			if len(result) != tt.expected {
				t.Errorf("deduplicateStrings() = %d items, want %d", len(result), tt.expected)
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
