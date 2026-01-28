package main

import (
	"testing"
	"time"
)

func TestGetEventTime(t *testing.T) {
	tests := []struct {
		name     string
		event    CalendarEvent
		wantZero bool
	}{
		{
			name: "DateTime event",
			event: CalendarEvent{
				Start: EventTime{DateTime: "2026-01-28T10:00:00-05:00"},
			},
			wantZero: false,
		},
		{
			name: "Date-only event",
			event: CalendarEvent{
				Start: EventTime{Date: "2026-01-28"},
			},
			wantZero: false,
		},
		{
			name: "Empty event",
			event: CalendarEvent{
				Start: EventTime{},
			},
			wantZero: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getEventTime(tt.event)
			if tt.wantZero && !got.IsZero() {
				t.Errorf("getEventTime() = %v, want zero time", got)
			}
			if !tt.wantZero && got.IsZero() {
				t.Errorf("getEventTime() = zero time, want non-zero")
			}
		})
	}
}

func TestGetEventEndTime(t *testing.T) {
	tests := []struct {
		name     string
		event    CalendarEvent
		wantZero bool
	}{
		{
			name: "DateTime event",
			event: CalendarEvent{
				End: EventTime{DateTime: "2026-01-28T11:00:00-05:00"},
			},
			wantZero: false,
		},
		{
			name: "Date-only event",
			event: CalendarEvent{
				End: EventTime{Date: "2026-01-28"},
			},
			wantZero: false,
		},
		{
			name: "Empty event",
			event: CalendarEvent{
				End: EventTime{},
			},
			wantZero: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getEventEndTime(tt.event)
			if tt.wantZero && !got.IsZero() {
				t.Errorf("getEventEndTime() = %v, want zero time", got)
			}
			if !tt.wantZero && got.IsZero() {
				t.Errorf("getEventEndTime() = zero time, want non-zero")
			}
		})
	}
}

func TestFilterDeclined(t *testing.T) {
	events := []CalendarEvent{
		{
			Summary: "Accepted meeting",
			Attendees: []Attendee{
				{Self: true, ResponseStatus: "accepted"},
			},
		},
		{
			Summary: "Declined meeting",
			Attendees: []Attendee{
				{Self: true, ResponseStatus: "declined"},
			},
		},
		{
			Summary: "No self attendee",
			Attendees: []Attendee{
				{Self: false, ResponseStatus: "accepted"},
			},
		},
	}

	filtered := filterDeclined(events)

	if len(filtered) != 2 {
		t.Errorf("filterDeclined() returned %d events, want 2", len(filtered))
	}

	for _, event := range filtered {
		if event.Summary == "Declined meeting" {
			t.Errorf("filterDeclined() did not filter out declined meeting")
		}
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string // Just check prefix for home dir expansion
	}{
		{
			name: "Tilde path",
			path: "~/Documents",
			want: "/", // Should start with / after expansion
		},
		{
			name: "Absolute path",
			path: "/tmp/test",
			want: "/tmp/test",
		},
		{
			name: "Relative path",
			path: "relative/path",
			want: "relative/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandPath(tt.path)
			if tt.name == "Tilde path" {
				if got[0] != '/' {
					t.Errorf("expandPath(%q) = %q, want path starting with /", tt.path, got)
				}
			} else if got != tt.want {
				t.Errorf("expandPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestDateTimeLocalTimezone(t *testing.T) {
	// Test that all-day events are parsed in local timezone
	event := CalendarEvent{
		Start: EventTime{Date: "2026-01-28"},
	}

	eventTime := getEventTime(event)

	// The event should be at midnight local time
	expected := time.Date(2026, 1, 28, 0, 0, 0, 0, time.Local)
	if !eventTime.Equal(expected) {
		t.Errorf("getEventTime() = %v, want %v", eventTime, expected)
	}
}
