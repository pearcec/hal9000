package processor

import (
	"os"
	"testing"
	"time"
)

func TestToBronzeCalendar(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "processor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := ProcessConfig{LibraryPath: tmpDir}

	rawDoc := map[string]interface{}{
		"_meta": map[string]interface{}{
			"source":   "google-calendar",
			"event_id": "test-123",
		},
		"summary":     "Team Standup",
		"description": "Daily standup meeting",
		"location":    "Conference Room A",
		"start": map[string]interface{}{
			"dateTime": "2026-01-27T09:00:00-05:00",
		},
		"end": map[string]interface{}{
			"dateTime": "2026-01-27T09:30:00-05:00",
		},
		"attendees": []interface{}{
			map[string]interface{}{
				"email":          "alice@example.com",
				"displayName":    "Alice",
				"responseStatus": "accepted",
			},
		},
	}

	doc, err := ToBronze(config, "calendar", rawDoc)
	if err != nil {
		t.Fatalf("ToBronze failed: %v", err)
	}

	if doc.Meta.Stage != StageBronze {
		t.Errorf("Stage = %q, want %q", doc.Meta.Stage, StageBronze)
	}

	if doc.Content["title"] != "Team Standup" {
		t.Errorf("title = %v, want %q", doc.Content["title"], "Team Standup")
	}

	if doc.Content["location"] != "Conference Room A" {
		t.Errorf("location = %v, want %q", doc.Content["location"], "Conference Room A")
	}
}

func TestToBronzeJIRA(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "processor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := ProcessConfig{LibraryPath: tmpDir}

	rawDoc := map[string]interface{}{
		"_meta": map[string]interface{}{
			"source":   "jira",
			"event_id": "PROJ-123",
		},
		"key": "PROJ-123",
		"fields": map[string]interface{}{
			"summary":     "Fix the bug",
			"description": "There is a bug that needs fixing",
			"status": map[string]interface{}{
				"name": "In Progress",
			},
			"priority": map[string]interface{}{
				"name": "High",
			},
			"assignee": map[string]interface{}{
				"displayName":  "Bob",
				"emailAddress": "bob@example.com",
			},
			"project": map[string]interface{}{
				"key": "PROJ",
			},
			"issuetype": map[string]interface{}{
				"name": "Bug",
			},
		},
	}

	doc, err := ToBronze(config, "jira", rawDoc)
	if err != nil {
		t.Fatalf("ToBronze failed: %v", err)
	}

	if doc.Content["key"] != "PROJ-123" {
		t.Errorf("key = %v, want %q", doc.Content["key"], "PROJ-123")
	}

	if doc.Content["status"] != "In Progress" {
		t.Errorf("status = %v, want %q", doc.Content["status"], "In Progress")
	}

	if doc.Content["project"] != "PROJ" {
		t.Errorf("project = %v, want %q", doc.Content["project"], "PROJ")
	}
}

func TestToBronzeSlack(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "processor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := ProcessConfig{LibraryPath: tmpDir}

	rawDoc := map[string]interface{}{
		"_meta": map[string]interface{}{
			"source":   "slack",
			"event_id": "C123_1234567890",
		},
		"channel": "C123",
		"user":    "U456",
		"text":    "Hello team!  Let's meet.",
		"ts":      "1234567890.123456",
	}

	doc, err := ToBronze(config, "slack", rawDoc)
	if err != nil {
		t.Fatalf("ToBronze failed: %v", err)
	}

	if doc.Content["channel"] != "C123" {
		t.Errorf("channel = %v, want %q", doc.Content["channel"], "C123")
	}

	if doc.Content["user"] != "U456" {
		t.Errorf("user = %v, want %q", doc.Content["user"], "U456")
	}

	// Text should be cleaned (whitespace normalized)
	text := doc.Content["text"].(string)
	if text != "Hello team! Let's meet." {
		t.Errorf("text = %q, want cleaned text", text)
	}
}

func TestToSilver(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "processor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := ProcessConfig{LibraryPath: tmpDir}

	bronzeDoc := &Document{
		Meta: DocumentMeta{
			Source:      "google-calendar",
			EventID:     "test-123",
			Stage:       StageBronze,
			ProcessedAt: time.Now(),
		},
		Content: map[string]interface{}{
			"title": "Meeting",
			"attendees": []map[string]interface{}{
				{"email": "alice@example.com", "name": "Alice"},
				{"email": "bob@example.com", "name": "Bob"},
			},
		},
	}

	doc, err := ToSilver(config, bronzeDoc)
	if err != nil {
		t.Fatalf("ToSilver failed: %v", err)
	}

	if doc.Meta.Stage != StageSilver {
		t.Errorf("Stage = %q, want %q", doc.Meta.Stage, StageSilver)
	}

	// Should have extracted links
	if len(doc.Links) == 0 {
		t.Error("Expected links to be extracted")
	}

	// Check for scheduled_with links
	hasScheduledWith := false
	for _, link := range doc.Links {
		if link.Type == "scheduled_with" {
			hasScheduledWith = true
			break
		}
	}
	if !hasScheduledWith {
		t.Error("Expected scheduled_with links for attendees")
	}
}

func TestExtractMentions(t *testing.T) {
	content := map[string]interface{}{
		"text":        "Hey @alice, can you check with bob@example.com?",
		"description": "Contact support@company.org for help",
		"nested": map[string]interface{}{
			"field": "Also cc: manager@example.com",
		},
	}

	mentions := extractMentions(content)

	// Should find emails
	found := make(map[string]bool)
	for _, m := range mentions {
		found[m] = true
	}

	expectedEmails := []string{"bob@example.com", "support@company.org", "manager@example.com"}
	for _, email := range expectedEmails {
		if !found[email] {
			t.Errorf("Expected to find mention: %s", email)
		}
	}
}

func TestCleanText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  spaces  ", "spaces"},
		{"multiple   spaces", "multiple spaces"},
		{"tabs\t\there", "tabs here"},
		{"newlines\n\nhere", "newlines here"},
		{"normal text", "normal text"},
	}

	for _, tt := range tests {
		result := cleanText(tt.input)
		if result != tt.expected {
			t.Errorf("cleanText(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestSaveDocument(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "processor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := ProcessConfig{LibraryPath: tmpDir}

	doc := &Document{
		Meta: DocumentMeta{
			Source:      "test",
			EventID:     "save-test-123",
			Stage:       StageBronze,
			ProcessedAt: time.Now(),
		},
		Content: map[string]interface{}{
			"key": "value",
		},
	}

	path, err := SaveDocument(config, doc)
	if err != nil {
		t.Fatalf("SaveDocument failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("Expected file to exist at %s", path)
	}

	// Verify content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}

	content := string(data)
	if !containsString(content, "save-test-123") {
		t.Error("Expected event_id in saved document")
	}
	if !containsString(content, "bronze") {
		t.Error("Expected stage in saved document")
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
