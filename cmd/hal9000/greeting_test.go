package main

import "testing"

func TestGetSalutation(t *testing.T) {
	tests := []struct {
		name     string
		hour     int
		expected string
	}{
		// Morning: 5am-12pm
		{"5am morning start", 5, "Good morning, Dave."},
		{"8am mid morning", 8, "Good morning, Dave."},
		{"11am late morning", 11, "Good morning, Dave."},

		// Afternoon: 12pm-5pm
		{"12pm afternoon start", 12, "Good afternoon, Dave."},
		{"14pm mid afternoon", 14, "Good afternoon, Dave."},
		{"16pm late afternoon", 16, "Good afternoon, Dave."},

		// Evening: 5pm-5am
		{"17pm evening start", 17, "Good evening, Dave."},
		{"20pm night", 20, "Good evening, Dave."},
		{"23pm late night", 23, "Good evening, Dave."},
		{"0am midnight", 0, "Good evening, Dave."},
		{"3am early morning", 3, "Good evening, Dave."},
		{"4am pre-dawn", 4, "Good evening, Dave."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getSalutation(tt.hour)
			if result != tt.expected {
				t.Errorf("getSalutation(%d) = %q, want %q", tt.hour, result, tt.expected)
			}
		})
	}
}
