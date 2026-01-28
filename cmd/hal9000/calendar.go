package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pearcec/hal9000/internal/config"
	"github.com/spf13/cobra"
)

// getLibraryCalendarPath returns the path to the calendar directory in the library.
func getLibraryCalendarPath() string {
	return filepath.Join(config.GetLibraryPath(), "calendar")
}

// CalendarEvent represents a calendar event from the library.
type CalendarEvent struct {
	Meta        EventMeta              `json:"_meta"`
	Summary     string                 `json:"summary"`
	Description string                 `json:"description"`
	Location    string                 `json:"location"`
	Start       EventTime              `json:"start"`
	End         EventTime              `json:"end"`
	Attendees   []Attendee             `json:"attendees"`
	Organizer   Organizer              `json:"organizer"`
	Status      string                 `json:"status"`
	HTMLLink    string                 `json:"htmlLink"`
	HangoutLink string                 `json:"hangoutLink"`
	Raw         map[string]interface{} `json:"-"` // For JSON output
}

// EventMeta contains metadata about the event.
type EventMeta struct {
	Source    string `json:"source"`
	FetchedAt string `json:"fetched_at"`
	EventID   string `json:"event_id"`
	Stage     string `json:"stage"`
}

// EventTime represents start/end time with optional date-only format.
type EventTime struct {
	DateTime string `json:"dateTime"`
	Date     string `json:"date"`
	TimeZone string `json:"timeZone"`
}

// Attendee represents a meeting attendee.
type Attendee struct {
	Email          string `json:"email"`
	DisplayName    string `json:"displayName"`
	ResponseStatus string `json:"responseStatus"`
	Self           bool   `json:"self"`
}

// Organizer represents the meeting organizer.
type Organizer struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Self        bool   `json:"self"`
}

var (
	calendarJSON            bool
	calendarIncludeDeclined bool
	calendarDate            string
	calendarDays            int
)

var calendarCmd = &cobra.Command{
	Use:   "calendar",
	Short: "View calendar events",
	Long: `I can help you review your calendar events.

Use subcommands to view events for different time periods:
  today   - Events for today
  week    - Events for this week
  list    - Events for a specific date range`,
}

var calendarListCmd = &cobra.Command{
	Use:   "list",
	Short: "List calendar events",
	Long:  `List calendar events for a specific date and number of days.`,
	RunE:  runCalendarList,
}

var calendarTodayCmd = &cobra.Command{
	Use:   "today",
	Short: "Show today's events",
	Long:  `I'll show you what's on your calendar for today.`,
	RunE:  runCalendarToday,
}

var calendarWeekCmd = &cobra.Command{
	Use:   "week",
	Short: "Show this week's events",
	Long:  `I'll show you what's on your calendar for this week.`,
	RunE:  runCalendarWeek,
}

func init() {
	// Add flags to calendar commands
	calendarCmd.PersistentFlags().BoolVar(&calendarJSON, "json", false, "Output as JSON")
	calendarCmd.PersistentFlags().BoolVar(&calendarIncludeDeclined, "include-declined", false, "Include declined meetings")

	// List-specific flags
	calendarListCmd.Flags().StringVar(&calendarDate, "date", "", "Start date (YYYY-MM-DD), defaults to today")
	calendarListCmd.Flags().IntVar(&calendarDays, "days", 1, "Number of days to show")

	// Build command hierarchy
	calendarCmd.AddCommand(calendarListCmd)
	calendarCmd.AddCommand(calendarTodayCmd)
	calendarCmd.AddCommand(calendarWeekCmd)

	rootCmd.AddCommand(calendarCmd)
}

func runCalendarList(cmd *cobra.Command, args []string) error {
	startDate := time.Now()
	if calendarDate != "" {
		parsed, err := time.Parse("2006-01-02", calendarDate)
		if err != nil {
			return fmt.Errorf("invalid date format (use YYYY-MM-DD): %v", err)
		}
		startDate = parsed
	}

	// Normalize to start of day
	startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())
	endDate := startDate.AddDate(0, 0, calendarDays)

	return displayEvents(startDate, endDate)
}

func runCalendarToday(cmd *cobra.Command, args []string) error {
	now := time.Now()
	startDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endDate := startDate.AddDate(0, 0, 1)

	return displayEvents(startDate, endDate)
}

func runCalendarWeek(cmd *cobra.Command, args []string) error {
	now := time.Now()
	startDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Find the start of the current week (Sunday)
	daysSinceSunday := int(startDate.Weekday())
	startOfWeek := startDate.AddDate(0, 0, -daysSinceSunday)
	endOfWeek := startOfWeek.AddDate(0, 0, 7)

	return displayEvents(startOfWeek, endOfWeek)
}

func displayEvents(startDate, endDate time.Time) error {
	events, err := loadCalendarEvents(startDate, endDate)
	if err != nil {
		return err
	}

	// Filter declined if not requested
	if !calendarIncludeDeclined {
		events = filterDeclined(events)
	}

	// Sort by start time
	sort.Slice(events, func(i, j int) bool {
		return getEventTime(events[i]).Before(getEventTime(events[j]))
	})

	if calendarJSON {
		return outputJSON(events)
	}

	return outputText(events, startDate, endDate)
}

func loadCalendarEvents(startDate, endDate time.Time) ([]CalendarEvent, error) {
	calPath := getLibraryCalendarPath()

	// Check if directory exists
	if _, err := os.Stat(calPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("calendar library not found at %s", calPath)
	}

	var events []CalendarEvent

	err := filepath.Walk(calPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		event, err := loadEventFile(path)
		if err != nil {
			return nil // Skip invalid files
		}

		eventTime := getEventTime(event)

		// Filter by date range
		if eventTime.Before(startDate) || !eventTime.Before(endDate) {
			return nil
		}

		events = append(events, event)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error reading calendar: %v", err)
	}

	return events, nil
}

func loadEventFile(path string) (CalendarEvent, error) {
	var event CalendarEvent

	data, err := os.ReadFile(path)
	if err != nil {
		return event, err
	}

	// First unmarshal to raw map to preserve all fields
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return event, err
	}

	// Then unmarshal to struct
	if err := json.Unmarshal(data, &event); err != nil {
		return event, err
	}

	event.Raw = raw
	return event, nil
}

func getEventTime(event CalendarEvent) time.Time {
	if event.Start.DateTime != "" {
		t, err := time.Parse(time.RFC3339, event.Start.DateTime)
		if err == nil {
			return t
		}
	}
	if event.Start.Date != "" {
		// Parse all-day events in local timezone
		t, err := time.ParseInLocation("2006-01-02", event.Start.Date, time.Local)
		if err == nil {
			return t
		}
	}
	return time.Time{}
}

func getEventEndTime(event CalendarEvent) time.Time {
	if event.End.DateTime != "" {
		t, err := time.Parse(time.RFC3339, event.End.DateTime)
		if err == nil {
			return t
		}
	}
	if event.End.Date != "" {
		// Parse all-day events in local timezone
		t, err := time.ParseInLocation("2006-01-02", event.End.Date, time.Local)
		if err == nil {
			return t
		}
	}
	return time.Time{}
}

func filterDeclined(events []CalendarEvent) []CalendarEvent {
	var filtered []CalendarEvent
	for _, event := range events {
		declined := false
		for _, attendee := range event.Attendees {
			if attendee.Self && attendee.ResponseStatus == "declined" {
				declined = true
				break
			}
		}
		if !declined {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func outputJSON(events []CalendarEvent) error {
	// Output raw data for full fidelity
	var rawEvents []map[string]interface{}
	for _, event := range events {
		rawEvents = append(rawEvents, event.Raw)
	}

	data, err := json.MarshalIndent(rawEvents, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func outputText(events []CalendarEvent, startDate, endDate time.Time) error {
	if len(events) == 0 {
		fmt.Printf("No events found between %s and %s.\n",
			startDate.Format("Mon Jan 2"),
			endDate.AddDate(0, 0, -1).Format("Mon Jan 2"))
		fmt.Println("\nI am completely operational, and all my circuits are functioning perfectly.")
		return nil
	}

	// Group events by day (in local timezone)
	eventsByDay := make(map[string][]CalendarEvent)
	for _, event := range events {
		day := getEventTime(event).Local().Format("2006-01-02")
		eventsByDay[day] = append(eventsByDay[day], event)
	}

	// Get sorted days
	var days []string
	for day := range eventsByDay {
		days = append(days, day)
	}
	sort.Strings(days)

	fmt.Printf("Calendar events from %s to %s:\n\n",
		startDate.Format("Mon Jan 2"),
		endDate.AddDate(0, 0, -1).Format("Mon Jan 2"))

	for _, day := range days {
		dayTime, _ := time.Parse("2006-01-02", day)
		fmt.Printf("=== %s ===\n\n", dayTime.Format("Monday, January 2, 2006"))

		for _, event := range eventsByDay[day] {
			printEvent(event)
		}
	}

	return nil
}

func printEvent(event CalendarEvent) {
	startTime := getEventTime(event)
	endTime := getEventEndTime(event)

	// Format time range (in local timezone)
	var timeStr string
	if event.Start.Date != "" && event.Start.DateTime == "" {
		// All-day event
		timeStr = "All day"
	} else {
		timeStr = fmt.Sprintf("%s - %s",
			startTime.Local().Format("3:04 PM"),
			endTime.Local().Format("3:04 PM"))
	}

	fmt.Printf("  %s\n", timeStr)
	fmt.Printf("  %s\n", event.Summary)

	if event.Location != "" {
		fmt.Printf("  Location: %s\n", event.Location)
	}

	if event.HangoutLink != "" {
		fmt.Printf("  Meet: %s\n", event.HangoutLink)
	}

	// Show attendee count if more than just self
	attendeeCount := 0
	for _, a := range event.Attendees {
		if !a.Self {
			attendeeCount++
		}
	}
	if attendeeCount > 0 {
		fmt.Printf("  Attendees: %d\n", attendeeCount)
	}

	fmt.Println()
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
