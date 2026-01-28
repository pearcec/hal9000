package main

import (
	"strings"
	"testing"
)

func TestUpdateSection(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		sectionName string
		newValue    string
		wantFound   bool
		wantContain string
	}{
		{
			name: "update existing section",
			content: `# Title

## First Section

Some content here.

## Second Section

More content.
`,
			sectionName: "First Section",
			newValue:    "New content here.",
			wantFound:   true,
			wantContain: "New content here.",
		},
		{
			name: "section not found",
			content: `# Title

## First Section

Some content.
`,
			sectionName: "Missing Section",
			newValue:    "New content.",
			wantFound:   false,
		},
		{
			name: "case insensitive match",
			content: `# Title

## NOTES

Original notes.
`,
			sectionName: "notes",
			newValue:    "Updated notes.",
			wantFound:   true,
			wantContain: "Updated notes.",
		},
		{
			name: "update last section",
			content: `# Title

## First

Content.

## Last

Final content.
`,
			sectionName: "Last",
			newValue:    "New final content.",
			wantFound:   true,
			wantContain: "New final content.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, found := updateSection(tt.content, tt.sectionName, tt.newValue)

			if found != tt.wantFound {
				t.Errorf("updateSection() found = %v, want %v", found, tt.wantFound)
			}

			if tt.wantFound && tt.wantContain != "" {
				if !strings.Contains(result, tt.wantContain) {
					t.Errorf("updateSection() result does not contain %q, got:\n%s", tt.wantContain, result)
				}
			}
		})
	}
}
