package agenda

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pearcec/hal9000/cmd/hal9000/tasks"
)

func TestAgendaTask_Interface(t *testing.T) {
	task := &AgendaTask{}

	// Verify interface implementation
	var _ tasks.Task = task

	if task.Name() != "agenda" {
		t.Errorf("Name() = %q, want %q", task.Name(), "agenda")
	}

	if task.PreferencesKey() != "agenda" {
		t.Errorf("PreferencesKey() = %q, want %q", task.PreferencesKey(), "agenda")
	}

	if task.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestAgendaTask_SetupQuestions(t *testing.T) {
	task := &AgendaTask{}
	questions := task.SetupQuestions()

	if len(questions) != 6 {
		t.Errorf("SetupQuestions() returned %d questions, want 6", len(questions))
	}

	// Verify required questions exist
	expectedKeys := []string{"workday_start", "priority_count", "include_routine", "jira_board", "include_prep", "format"}
	for _, key := range expectedKeys {
		found := false
		for _, q := range questions {
			if q.Key == key {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Missing setup question with key %q", key)
		}
	}

	// Verify format question has correct options
	for _, q := range questions {
		if q.Key == "format" {
			if len(q.Options) != 3 {
				t.Errorf("format question has %d options, want 3", len(q.Options))
			}
			expectedOptions := []string{"full", "compact", "minimal"}
			for _, opt := range expectedOptions {
				found := false
				for _, o := range q.Options {
					if o == opt {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("format question missing option %q", opt)
				}
			}
		}
	}
}

func TestAgendaTask_Run_DryRun(t *testing.T) {
	// Create a temporary library directory
	tmpDir, err := os.MkdirTemp("", "agenda-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	task := &AgendaTask{}
	opts := tasks.RunOptions{
		DryRun: true,
		Output: filepath.Join(tmpDir, "test-agenda.md"),
	}

	result, err := task.Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !result.Success {
		t.Error("Run() result.Success = false, want true")
	}

	if result.Output == "" {
		t.Error("Run() result.Output should not be empty")
	}

	if !strings.Contains(result.Message, "Would write") {
		t.Errorf("Run() result.Message = %q, should contain 'Would write'", result.Message)
	}

	// Verify file was NOT written in dry run
	if _, err := os.Stat(opts.Output); !os.IsNotExist(err) {
		t.Error("Dry run should not create file")
	}
}

func TestLoadTodayEventsFromPath(t *testing.T) {
	// Create a temporary library directory structure
	tmpDir, err := os.MkdirTemp("", "agenda-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create calendar directory
	calendarDir := filepath.Join(tmpDir, "calendar")
	if err := os.MkdirAll(calendarDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a test event for today
	now := time.Now()
	event := map[string]interface{}{
		"summary":     "Team Meeting",
		"description": "Weekly sync",
		"location":    "Conference Room A",
		"start": map[string]string{
			"dateTime": now.Add(2 * time.Hour).Format(time.RFC3339),
		},
		"end": map[string]string{
			"dateTime": now.Add(3 * time.Hour).Format(time.RFC3339),
		},
		"attendees": []map[string]interface{}{
			{"email": "me@example.com", "self": true, "responseStatus": "accepted"},
			{"email": "other@example.com", "displayName": "Other Person", "responseStatus": "accepted"},
		},
		"hangoutLink": "https://meet.google.com/abc-defg-hij",
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}

	eventFile := filepath.Join(calendarDir, "meeting.json")
	if err := os.WriteFile(eventFile, eventData, 0644); err != nil {
		t.Fatal(err)
	}

	events, err := loadTodayEventsFromPath(now, tmpDir)
	if err != nil {
		t.Fatalf("loadTodayEventsFromPath() error = %v", err)
	}

	if len(events) != 1 {
		t.Errorf("loadTodayEventsFromPath() returned %d events, want 1", len(events))
	}

	if len(events) > 0 && events[0].Summary != "Team Meeting" {
		t.Errorf("Event summary = %q, want %q", events[0].Summary, "Team Meeting")
	}
}

func TestLoadTodayEventsFromPath_NoCalendarDir(t *testing.T) {
	// Create a temporary empty library directory
	tmpDir, err := os.MkdirTemp("", "agenda-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	events, err := loadTodayEventsFromPath(time.Now(), tmpDir)
	if err != nil {
		t.Errorf("loadTodayEventsFromPath() error = %v, want nil", err)
	}

	if events != nil && len(events) > 0 {
		t.Errorf("loadTodayEventsFromPath() returned %d events, want 0", len(events))
	}
}

func TestLoadTodayEventsFromPath_SkipsDeclinedEvents(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agenda-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	calendarDir := filepath.Join(tmpDir, "calendar")
	if err := os.MkdirAll(calendarDir, 0755); err != nil {
		t.Fatal(err)
	}

	now := time.Now()

	// Create a declined event
	declinedEvent := map[string]interface{}{
		"summary": "Declined Meeting",
		"start": map[string]string{
			"dateTime": now.Add(1 * time.Hour).Format(time.RFC3339),
		},
		"end": map[string]string{
			"dateTime": now.Add(2 * time.Hour).Format(time.RFC3339),
		},
		"attendees": []map[string]interface{}{
			{"email": "me@example.com", "self": true, "responseStatus": "declined"},
		},
	}

	eventData, _ := json.Marshal(declinedEvent)
	if err := os.WriteFile(filepath.Join(calendarDir, "declined.json"), eventData, 0644); err != nil {
		t.Fatal(err)
	}

	events, err := loadTodayEventsFromPath(now, tmpDir)
	if err != nil {
		t.Fatalf("loadTodayEventsFromPath() error = %v", err)
	}

	if len(events) != 0 {
		t.Errorf("loadTodayEventsFromPath() returned %d events, want 0 (declined should be filtered)", len(events))
	}
}

func TestLoadRolloverItemsFromPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agenda-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create agenda directory with yesterday's agenda
	agendaDir := filepath.Join(tmpDir, "agenda")
	if err := os.MkdirAll(agendaDir, 0755); err != nil {
		t.Fatal(err)
	}

	yesterday := time.Now().AddDate(0, 0, -1)
	yesterdayFile := filepath.Join(agendaDir, "agenda_"+yesterday.Format("2006-01-02")+"_daily-agenda.md")
	yesterdayContent := `# Daily Agenda

## Tasks
- [x] Completed task
- [ ] Incomplete task that should roll over
- [ ] Another incomplete item
`
	if err := os.WriteFile(yesterdayFile, []byte(yesterdayContent), 0644); err != nil {
		t.Fatal(err)
	}

	rollovers, err := loadRolloverItemsFromPath(time.Now(), tmpDir)
	if err != nil {
		t.Fatalf("loadRolloverItemsFromPath() error = %v", err)
	}

	if len(rollovers) != 2 {
		t.Errorf("loadRolloverItemsFromPath() returned %d items, want 2", len(rollovers))
	}

	// Verify first rollover item
	if len(rollovers) > 0 && rollovers[0].Text != "Incomplete task that should roll over" {
		t.Errorf("First rollover text = %q, want %q", rollovers[0].Text, "Incomplete task that should roll over")
	}
}

func TestLoadRolloverItemsFromPath_NoPreviousAgenda(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agenda-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	rollovers, err := loadRolloverItemsFromPath(time.Now(), tmpDir)
	if err != nil {
		t.Errorf("loadRolloverItemsFromPath() error = %v, want nil", err)
	}

	if rollovers != nil && len(rollovers) > 0 {
		t.Errorf("loadRolloverItemsFromPath() returned %d items, want 0", len(rollovers))
	}
}

func TestLoadRemindersFromPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agenda-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create reminders directory
	remindersDir := filepath.Join(tmpDir, "reminders")
	if err := os.MkdirAll(remindersDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a reminder file
	reminderContent := `# Reminder
Follow up with team about project status
`
	if err := os.WriteFile(filepath.Join(remindersDir, "followup.md"), []byte(reminderContent), 0644); err != nil {
		t.Fatal(err)
	}

	reminders, err := loadRemindersFromPath(tmpDir)
	if err != nil {
		t.Fatalf("loadRemindersFromPath() error = %v", err)
	}

	if len(reminders) != 1 {
		t.Errorf("loadRemindersFromPath() returned %d reminders, want 1", len(reminders))
	}

	if len(reminders) > 0 && reminders[0].Text != "Follow up with team about project status" {
		t.Errorf("Reminder text = %q, want %q", reminders[0].Text, "Follow up with team about project status")
	}
}

func TestLoadRemindersFromPath_NoRemindersDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agenda-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	reminders, err := loadRemindersFromPath(tmpDir)
	if err != nil {
		t.Errorf("loadRemindersFromPath() error = %v, want nil", err)
	}

	if reminders != nil && len(reminders) > 0 {
		t.Errorf("loadRemindersFromPath() returned %d reminders, want 0", len(reminders))
	}
}

func TestParseEventTime(t *testing.T) {
	tests := []struct {
		name     string
		input    EventTime
		wantZero bool
	}{
		{
			name:     "empty",
			input:    EventTime{},
			wantZero: true,
		},
		{
			name: "with dateTime",
			input: EventTime{
				DateTime: "2024-01-15T10:00:00-08:00",
			},
			wantZero: false,
		},
		{
			name: "with date only",
			input: EventTime{
				Date: "2024-01-15",
			},
			wantZero: false,
		},
		{
			name: "invalid dateTime",
			input: EventTime{
				DateTime: "invalid",
			},
			wantZero: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseEventTime(tt.input)
			if tt.wantZero && !result.IsZero() {
				t.Errorf("parseEventTime() = %v, want zero time", result)
			}
			if !tt.wantZero && result.IsZero() {
				t.Error("parseEventTime() = zero time, want non-zero")
			}
		})
	}
}

func TestGenerateAgenda(t *testing.T) {
	today := time.Date(2024, 1, 15, 10, 0, 0, 0, time.Local)

	events := []CalendarEvent{
		{
			Summary:  "Test Meeting",
			Location: "Room 1",
			Start:    EventTime{DateTime: "2024-01-15T14:00:00-08:00"},
			End:      EventTime{DateTime: "2024-01-15T15:00:00-08:00"},
		},
	}

	rollovers := []RolloverItem{
		{Text: "Rollover task", Priority: 1},
	}

	reminders := []Reminder{
		{Text: "Remember to follow up"},
	}

	agenda := generateAgenda(today, events, rollovers, reminders, tasks.RunOptions{})

	// Verify sections exist
	if !strings.Contains(agenda, "# Daily Agenda") {
		t.Error("Agenda should contain title")
	}

	if !strings.Contains(agenda, "Rollover Items") {
		t.Error("Agenda should contain rollover section")
	}

	if !strings.Contains(agenda, "Today's Schedule") {
		t.Error("Agenda should contain schedule section")
	}

	if !strings.Contains(agenda, "Test Meeting") {
		t.Error("Agenda should contain event")
	}

	if !strings.Contains(agenda, "Reminders") {
		t.Error("Agenda should contain reminders section")
	}

	if !strings.Contains(agenda, "Generated by HAL 9000") {
		t.Error("Agenda should contain HAL 9000 footer")
	}
}

func TestGenerateAgenda_NoEvents(t *testing.T) {
	today := time.Date(2024, 1, 15, 10, 0, 0, 0, time.Local)

	agenda := generateAgenda(today, nil, nil, nil, tasks.RunOptions{})

	if !strings.Contains(agenda, "No meetings scheduled for today") {
		t.Error("Agenda should show no meetings message when no events")
	}

	// Should not contain rollover section when no rollovers
	if strings.Contains(agenda, "Rollover Items") {
		t.Error("Agenda should not contain rollover section when no rollovers")
	}

	// Should not contain reminders section when no reminders
	if strings.Contains(agenda, "## Reminders") {
		t.Error("Agenda should not contain reminders section when no reminders")
	}
}

func TestWriteEvent(t *testing.T) {
	var sb strings.Builder

	event := CalendarEvent{
		Summary:     "Important Meeting",
		Location:    "Conference Room B",
		HangoutLink: "https://meet.google.com/xyz",
		Start:       EventTime{DateTime: "2024-01-15T14:00:00-08:00"},
		End:         EventTime{DateTime: "2024-01-15T15:00:00-08:00"},
		Attendees: []struct {
			Email          string `json:"email"`
			DisplayName    string `json:"displayName"`
			ResponseStatus string `json:"responseStatus"`
			Self           bool   `json:"self"`
		}{
			{Email: "me@example.com", Self: true},
			{Email: "other@example.com", Self: false},
			{Email: "another@example.com", Self: false},
		},
	}

	writeEvent(&sb, event)
	output := sb.String()

	if !strings.Contains(output, "### Important Meeting") {
		t.Error("Event output should contain summary as heading")
	}

	if !strings.Contains(output, "Conference Room B") {
		t.Error("Event output should contain location")
	}

	if !strings.Contains(output, "https://meet.google.com/xyz") {
		t.Error("Event output should contain hangout link")
	}

	if !strings.Contains(output, "Attendees:** 2") {
		t.Error("Event output should show 2 non-self attendees")
	}
}

func TestWriteEvent_AllDay(t *testing.T) {
	var sb strings.Builder

	event := CalendarEvent{
		Summary: "All Day Event",
		Start:   EventTime{Date: "2024-01-15"},
		End:     EventTime{Date: "2024-01-16"},
	}

	writeEvent(&sb, event)
	output := sb.String()

	if !strings.Contains(output, "All day") {
		t.Error("All-day event should show 'All day' time")
	}
}
