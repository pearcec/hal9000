package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pearcec/hal9000/internal/config"
	"golang.org/x/term"
)

// PrintGreeting displays the HAL 9000 greeting with time-appropriate salutation.
func PrintGreeting() {
	// Only print if stdout is a terminal
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return
	}

	now := time.Now()

	// Time-appropriate salutation
	salutation := getSalutation(now.Hour())
	fmt.Printf("%s\n\n", salutation)

	// HAL quote
	fmt.Println("I am completely operational, and all my circuits are functioning perfectly.")
	fmt.Println()

	// Today's date
	fmt.Printf("📅 Today: %s\n", now.Format("Monday, January 2, 2006"))

	// Calendar events count
	eventCount := getTodayEventCount()
	if eventCount >= 0 {
		fmt.Printf("🔔 %d calendar event", eventCount)
		if eventCount != 1 {
			fmt.Print("s")
		}
		fmt.Println()
	}

	// Pending items count (from library inbox if it exists)
	pendingCount := getPendingItemsCount()
	if pendingCount > 0 {
		fmt.Printf("📬 %d item", pendingCount)
		if pendingCount != 1 {
			fmt.Print("s")
		}
		fmt.Println(" need attention")
	}

	fmt.Println()
}

// getSalutation returns a time-appropriate greeting.
func getSalutation(hour int) string {
	switch {
	case hour >= 5 && hour < 12:
		return "Good morning, Dave."
	case hour >= 12 && hour < 17:
		return "Good afternoon, Dave."
	default:
		return "Good evening, Dave."
	}
}

// getTodayEventCount returns the number of calendar events for today.
// Returns -1 if calendar is not available.
func getTodayEventCount() int {
	now := time.Now()
	startDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endDate := startDate.AddDate(0, 0, 1)

	events, err := loadCalendarEvents(startDate, endDate)
	if err != nil {
		return -1
	}

	// Filter declined events
	events = filterDeclined(events)

	return len(events)
}

// getPendingItemsCount returns the number of pending items in the inbox.
// Returns 0 if inbox is not available or empty.
func getPendingItemsCount() int {
	inboxPath := filepath.Join(config.GetLibraryPath(), "inbox")

	entries, err := os.ReadDir(inboxPath)
	if err != nil {
		return 0
	}

	count := 0
	for _, entry := range entries {
		// Count files, not directories, skip hidden files
		if !entry.IsDir() && entry.Name()[0] != '.' {
			count++
		}
	}

	return count
}
