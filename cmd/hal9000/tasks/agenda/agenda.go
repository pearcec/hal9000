// Package agenda provides the daily agenda task for HAL 9000.
//
// The agenda task generates a daily agenda by collecting calendar events,
// checking for rollover items from previous agendas, and gathering reminders.
package agenda

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pearcec/hal9000/cmd/hal9000/tasks"
	"github.com/pearcec/hal9000/internal/config"
)

func init() {
	tasks.Register(&AgendaTask{})
}

// AgendaTask implements the daily agenda generation task.
type AgendaTask struct{}

// Name returns the task identifier.
func (t *AgendaTask) Name() string {
	return "agenda"
}

// Description returns human-readable description.
func (t *AgendaTask) Description() string {
	return "Generate your daily agenda from calendar and tasks"
}

// PreferencesKey returns the preferences file name.
func (t *AgendaTask) PreferencesKey() string {
	return "agenda"
}

// SetupQuestions returns questions for first-run setup.
func (t *AgendaTask) SetupQuestions() []tasks.SetupQuestion {
	return []tasks.SetupQuestion{
		{
			Key:      "workday_start",
			Question: "What time do you typically start work?",
			Default:  "09:00",
			Type:     tasks.QuestionTime,
			Section:  "Schedule",
		},
		{
			Key:      "priority_count",
			Question: "How many priority items should I highlight?",
			Default:  "3",
			Type:     tasks.QuestionText,
			Section:  "Display",
		},
		{
			Key:      "include_routine",
			Question: "Include routine items in the agenda?",
			Default:  "yes",
			Type:     tasks.QuestionConfirm,
			Section:  "Display",
		},
		{
			Key:      "jira_board",
			Question: "Which JIRA board should I check for your tasks? (leave blank to skip)",
			Default:  "",
			Type:     tasks.QuestionText,
			Section:  "Integrations",
		},
		{
			Key:      "include_prep",
			Question: "Include prep notes for meetings?",
			Default:  "yes",
			Type:     tasks.QuestionConfirm,
			Section:  "Display",
		},
		{
			Key:      "format",
			Question: "Which agenda format do you prefer?",
			Default:  "full",
			Options:  []string{"full", "compact", "minimal"},
			Type:     tasks.QuestionChoice,
			Section:  "Display",
		},
	}
}

// Run executes the agenda task.
func (t *AgendaTask) Run(ctx context.Context, opts tasks.RunOptions) (*tasks.Result, error) {
	today := time.Now()
	dateStr := today.Format("2006-01-02")

	// Load calendar events for today
	events, err := loadTodayEvents(today)
	if err != nil {
		return nil, fmt.Errorf("failed to load calendar events: %w", err)
	}

	// Load rollover items from previous agenda
	rollovers, err := loadRolloverItems(today)
	if err != nil {
		// Non-fatal - just log and continue
		rollovers = nil
	}

	// Load reminders
	reminders, err := loadReminders()
	if err != nil {
		// Non-fatal - just continue
		reminders = nil
	}

	// Generate agenda content
	agenda := generateAgenda(today, events, rollovers, reminders, opts)

	// Determine output path
	outputPath := opts.Output
	if outputPath == "" {
		libraryPath := config.GetLibraryPath()
		outputPath = filepath.Join(libraryPath, "agenda", fmt.Sprintf("agenda_%s_daily-agenda.md", dateStr))
	}

	if opts.DryRun {
		return &tasks.Result{
			Success:    true,
			Output:     agenda,
			OutputPath: outputPath,
			Message:    fmt.Sprintf("Would write agenda to: %s", outputPath),
		}, nil
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write agenda file
	if err := os.WriteFile(outputPath, []byte(agenda), 0644); err != nil {
		return nil, fmt.Errorf("failed to write agenda: %w", err)
	}

	return &tasks.Result{
		Success:    true,
		Output:     agenda,
		OutputPath: outputPath,
		Message:    fmt.Sprintf("Agenda written to: %s", outputPath),
		Metadata: map[string]interface{}{
			"events":    len(events),
			"rollovers": len(rollovers),
			"reminders": len(reminders),
		},
	}, nil
}

// CalendarEvent represents a calendar event from the library.
type CalendarEvent struct {
	Summary     string    `json:"summary"`
	Description string    `json:"description"`
	Location    string    `json:"location"`
	Start       EventTime `json:"start"`
	End         EventTime `json:"end"`
	Attendees   []struct {
		Email          string `json:"email"`
		DisplayName    string `json:"displayName"`
		ResponseStatus string `json:"responseStatus"`
		Self           bool   `json:"self"`
	} `json:"attendees"`
	HangoutLink string `json:"hangoutLink"`
	HTMLLink    string `json:"htmlLink"`
}

// EventTime represents start/end time.
type EventTime struct {
	DateTime string `json:"dateTime"`
	Date     string `json:"date"`
}

// RolloverItem represents an incomplete item from a previous agenda.
type RolloverItem struct {
	Text     string
	Priority int
}

// Reminder represents a reminder item.
type Reminder struct {
	Text    string
	DueDate time.Time
}

func loadTodayEvents(today time.Time) ([]CalendarEvent, error) {
	return loadTodayEventsFromPath(today, config.GetLibraryPath())
}

func loadTodayEventsFromPath(today time.Time, libraryPath string) ([]CalendarEvent, error) {
	calendarPath := filepath.Join(libraryPath, "calendar")

	if _, err := os.Stat(calendarPath); os.IsNotExist(err) {
		return nil, nil // No calendar directory - not an error
	}

	startOfDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	endOfDay := startOfDay.AddDate(0, 0, 1)

	var events []CalendarEvent

	err := filepath.Walk(calendarPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var event CalendarEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return nil
		}

		eventTime := parseEventTime(event.Start)
		if eventTime.IsZero() {
			return nil
		}

		// Filter to today's events
		if eventTime.Before(startOfDay) || !eventTime.Before(endOfDay) {
			return nil
		}

		// Skip declined events
		for _, a := range event.Attendees {
			if a.Self && a.ResponseStatus == "declined" {
				return nil
			}
		}

		events = append(events, event)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort by start time
	sort.Slice(events, func(i, j int) bool {
		return parseEventTime(events[i].Start).Before(parseEventTime(events[j].Start))
	})

	return events, nil
}

func parseEventTime(et EventTime) time.Time {
	if et.DateTime != "" {
		t, err := time.Parse(time.RFC3339, et.DateTime)
		if err == nil {
			return t
		}
	}
	if et.Date != "" {
		t, err := time.ParseInLocation("2006-01-02", et.Date, time.Local)
		if err == nil {
			return t
		}
	}
	return time.Time{}
}

func loadRolloverItems(today time.Time) ([]RolloverItem, error) {
	return loadRolloverItemsFromPath(today, config.GetLibraryPath())
}

func loadRolloverItemsFromPath(today time.Time, libraryPath string) ([]RolloverItem, error) {
	agendaPath := filepath.Join(libraryPath, "agenda")

	if _, err := os.Stat(agendaPath); os.IsNotExist(err) {
		return nil, nil
	}

	// Look for yesterday's agenda
	yesterday := today.AddDate(0, 0, -1)
	yesterdayFile := filepath.Join(agendaPath, fmt.Sprintf("agenda_%s_daily-agenda.md", yesterday.Format("2006-01-02")))

	data, err := os.ReadFile(yesterdayFile)
	if err != nil {
		return nil, nil // No previous agenda - not an error
	}

	// Parse for unchecked items (- [ ] format)
	var rollovers []RolloverItem
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- [ ]") {
			item := strings.TrimPrefix(line, "- [ ]")
			item = strings.TrimSpace(item)
			if item != "" {
				rollovers = append(rollovers, RolloverItem{Text: item, Priority: 1})
			}
		}
	}

	return rollovers, nil
}

func loadReminders() ([]Reminder, error) {
	return loadRemindersFromPath(config.GetLibraryPath())
}

func loadRemindersFromPath(libraryPath string) ([]Reminder, error) {
	remindersPath := filepath.Join(libraryPath, "reminders")

	if _, err := os.Stat(remindersPath); os.IsNotExist(err) {
		return nil, nil
	}

	var reminders []Reminder

	err := filepath.Walk(remindersPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip non-markdown files
		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Simple parsing - treat first line as reminder text
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				reminders = append(reminders, Reminder{Text: line})
				break
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return reminders, nil
}

func generateAgenda(today time.Time, events []CalendarEvent, rollovers []RolloverItem, reminders []Reminder, opts tasks.RunOptions) string {
	var sb strings.Builder

	dateStr := today.Format("Monday, January 2, 2006")

	sb.WriteString(fmt.Sprintf("# Daily Agenda: %s\n\n", dateStr))
	sb.WriteString(fmt.Sprintf("Generated at %s\n\n", today.Format("3:04 PM")))

	// Rollover items (high priority)
	if len(rollovers) > 0 {
		sb.WriteString("## Rollover Items\n\n")
		sb.WriteString("These items were not completed yesterday:\n\n")
		for _, item := range rollovers {
			sb.WriteString(fmt.Sprintf("- [ ] %s\n", item.Text))
		}
		sb.WriteString("\n")
	}

	// Calendar events
	sb.WriteString("## Today's Schedule\n\n")
	if len(events) == 0 {
		sb.WriteString("No meetings scheduled for today.\n\n")
	} else {
		for _, event := range events {
			writeEvent(&sb, event)
		}
	}

	// Reminders
	if len(reminders) > 0 {
		sb.WriteString("## Reminders\n\n")
		for _, r := range reminders {
			sb.WriteString(fmt.Sprintf("- %s\n", r.Text))
		}
		sb.WriteString("\n")
	}

	// Quick actions section
	sb.WriteString("## Quick Actions\n\n")
	sb.WriteString("- [ ] Review and prioritize tasks\n")
	sb.WriteString("- [ ] Check messages and emails\n")
	sb.WriteString("- [ ] Plan deep work blocks\n")
	sb.WriteString("\n")

	// Footer
	sb.WriteString("---\n")
	sb.WriteString("*Generated by HAL 9000. I am completely operational.*\n")

	return sb.String()
}

func writeEvent(sb *strings.Builder, event CalendarEvent) {
	startTime := parseEventTime(event.Start)
	endTime := parseEventTime(event.End)

	var timeStr string
	if event.Start.Date != "" && event.Start.DateTime == "" {
		timeStr = "All day"
	} else {
		timeStr = fmt.Sprintf("%s - %s",
			startTime.Local().Format("3:04 PM"),
			endTime.Local().Format("3:04 PM"))
	}

	sb.WriteString(fmt.Sprintf("### %s\n", event.Summary))
	sb.WriteString(fmt.Sprintf("**Time:** %s\n", timeStr))

	if event.Location != "" {
		sb.WriteString(fmt.Sprintf("**Location:** %s\n", event.Location))
	}

	if event.HangoutLink != "" {
		sb.WriteString(fmt.Sprintf("**Meet:** %s\n", event.HangoutLink))
	}

	// Count non-self attendees
	attendeeCount := 0
	for _, a := range event.Attendees {
		if !a.Self {
			attendeeCount++
		}
	}
	if attendeeCount > 0 {
		sb.WriteString(fmt.Sprintf("**Attendees:** %d\n", attendeeCount))
	}

	sb.WriteString("\n")
}
