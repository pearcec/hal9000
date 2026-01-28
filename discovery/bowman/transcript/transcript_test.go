package transcript

import (
	"testing"
	"time"
)

func TestParseVTT(t *testing.T) {
	input := `WEBVTT

00:00:00.000 --> 00:00:05.000
Hello, welcome to the meeting.

00:00:05.000 --> 00:00:10.000
Thank you for joining today.
`

	expected := "Hello, welcome to the meeting. Thank you for joining today."
	result := parseVTT(input)

	if result != expected {
		t.Errorf("parseVTT() = %q, want %q", result, expected)
	}
}

func TestParseVTTEmpty(t *testing.T) {
	input := `WEBVTT

`
	result := parseVTT(input)
	if result != "" {
		t.Errorf("parseVTT() for empty = %q, want empty string", result)
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected TranscriptFormat
	}{
		{
			name:     "google meet",
			content:  "Meeting transcript from Google Meet session on 2026-01-28",
			expected: FormatGoogleMeet,
		},
		{
			name:     "zoom webvtt",
			content:  "WEBVTT\n\n00:00:00.000 --> 00:00:05.000\nHello",
			expected: FormatZoom,
		},
		{
			name:     "zoom keyword",
			content:  "Zoom Meeting Transcript\nRecording from zoom.us",
			expected: FormatZoom,
		},
		{
			name:     "unknown",
			content:  "Just some meeting notes without any markers",
			expected: FormatUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectFormat(tt.content)
			if result != tt.expected {
				t.Errorf("detectFormat() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractSpeakers(t *testing.T) {
	input := `John Smith: Hello everyone, thanks for joining.
Jane Doe: Thank you, John.
John Smith: Let's get started with the agenda.
Bob Wilson: I have a question.
Jane Doe: Go ahead, Bob.`

	speakers := extractSpeakers(input)

	if len(speakers) != 3 {
		t.Errorf("extractSpeakers() found %d speakers, want 3", len(speakers))
	}

	expected := map[string]bool{
		"John Smith": true,
		"Jane Doe":   true,
		"Bob Wilson": true,
	}

	for _, speaker := range speakers {
		if !expected[speaker] {
			t.Errorf("extractSpeakers() found unexpected speaker: %q", speaker)
		}
	}
}

func TestExtractSpeakersEmpty(t *testing.T) {
	input := "No speakers in this text at all"
	speakers := extractSpeakers(input)

	if len(speakers) != 0 {
		t.Errorf("extractSpeakers() found %d speakers, want 0", len(speakers))
	}
}

func TestNormalizeTranscript(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "trim whitespace",
			input:    "  Hello world  ",
			expected: "Hello world",
		},
		{
			name:     "normalize line endings",
			input:    "Line1\r\nLine2\rLine3",
			expected: "Line1\nLine2\nLine3",
		},
		{
			name:     "remove excessive blank lines",
			input:    "Line1\n\n\n\n\nLine2",
			expected: "Line1\n\nLine2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeTranscript(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeTranscript() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestParseEntries(t *testing.T) {
	input := `John Smith: Hello everyone.
Jane Doe: Thanks for having us.
John Smith: Let's begin.`

	entries := ParseEntries(input)

	if len(entries) != 3 {
		t.Errorf("ParseEntries() found %d entries, want 3", len(entries))
	}

	if entries[0].Speaker != "John Smith" {
		t.Errorf("Entry 0 speaker = %q, want %q", entries[0].Speaker, "John Smith")
	}

	if entries[0].Text != "Hello everyone." {
		t.Errorf("Entry 0 text = %q, want %q", entries[0].Text, "Hello everyone.")
	}
}

func TestParseEntriesWithTimestamps(t *testing.T) {
	input := `10:30:00 John Smith: Meeting started.
10:31:15 Jane Doe: I have the report ready.`

	entries := ParseEntries(input)

	if len(entries) != 2 {
		t.Errorf("ParseEntries() found %d entries, want 2", len(entries))
	}

	// Check timestamp parsing
	expectedTime, _ := time.Parse("15:04:05", "10:30:00")
	if !entries[0].Timestamp.Equal(expectedTime) {
		t.Errorf("Entry 0 timestamp = %v, want %v", entries[0].Timestamp, expectedTime)
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantHome bool
	}{
		{
			name:     "tilde path",
			input:    "~/test/path",
			wantHome: true,
		},
		{
			name:     "absolute path",
			input:    "/absolute/path",
			wantHome: false,
		},
		{
			name:     "relative path",
			input:    "relative/path",
			wantHome: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)
			hasHome := !containsPrefix(result, "~/")

			if tt.wantHome && containsPrefix(result, "~/") {
				t.Errorf("expandPath(%q) = %q, expected ~ to be expanded", tt.input, result)
			}
			if !tt.wantHome && hasHome && tt.input[0] == '~' {
				t.Errorf("expandPath(%q) = %q, unexpected expansion", tt.input, result)
			}
		})
	}
}

func containsPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func TestTranscriptFormat(t *testing.T) {
	if FormatGoogleMeet != "google-meet" {
		t.Error("FormatGoogleMeet constant has wrong value")
	}
	if FormatZoom != "zoom" {
		t.Error("FormatZoom constant has wrong value")
	}
	if FormatUnknown != "unknown" {
		t.Error("FormatUnknown constant has wrong value")
	}
}
