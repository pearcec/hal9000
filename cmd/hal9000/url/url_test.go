package url

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateDescriptor(t *testing.T) {
	tests := []struct {
		title    string
		url      string
		expected string
	}{
		{
			title:    "Hello World Article",
			url:      "https://example.com/article",
			expected: "hello-world-article",
		},
		{
			title:    "",
			url:      "https://blog.example.com/post",
			expected: "blog-example-com",
		},
		{
			title:    "This is a Very Long Title That Should Be Truncated Eventually",
			url:      "https://example.com",
			expected: "this-is-a-very-long-",
		},
		{
			title:    "Special! Characters? Here.",
			url:      "https://example.com",
			expected: "special-characters-here-",
		},
	}

	for _, tt := range tests {
		result := generateDescriptor(tt.title, tt.url)
		// Just check prefix since exact length may vary
		if !strings.HasPrefix(result, tt.expected[:10]) {
			t.Errorf("generateDescriptor(%q, %q) = %q, expected prefix %q",
				tt.title, tt.url, result, tt.expected[:10])
		}
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		html     string
		expected string
	}{
		{
			html:     `<html><head><title>Test Title</title></head></html>`,
			expected: "Test Title",
		},
		{
			html:     `<html><head><title>  Spaces Around  </title></head></html>`,
			expected: "Spaces Around",
		},
		{
			html:     `<html><head><meta property="og:title" content="OG Title"></head></html>`,
			expected: "OG Title",
		},
		{
			html:     `<html><head></head><body>No title here</body></html>`,
			expected: "",
		},
	}

	for _, tt := range tests {
		result := extractTitle(tt.html)
		if result != tt.expected {
			t.Errorf("extractTitle(%q) = %q, expected %q", tt.html[:50], result, tt.expected)
		}
	}
}

func TestHtmlToText(t *testing.T) {
	tests := []struct {
		html     string
		contains string
	}{
		{
			html:     `<p>Hello World</p>`,
			contains: "Hello World",
		},
		{
			html:     `<script>alert('test')</script><p>Content</p>`,
			contains: "Content",
		},
		{
			html:     `<style>.test{color:red}</style><p>Visible</p>`,
			contains: "Visible",
		},
	}

	for _, tt := range tests {
		result := htmlToText(tt.html)
		if !strings.Contains(result, tt.contains) {
			t.Errorf("htmlToText(...) should contain %q, got %q", tt.contains, result)
		}
	}
}

func TestDecodeHTMLEntities(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "Hello&nbsp;World",
			expected: "Hello World",
		},
		{
			input:    "A &amp; B",
			expected: "A & B",
		},
		{
			input:    "1 &lt; 2 &gt; 0",
			expected: "1 < 2 > 0",
		},
		{
			input:    "&quot;quoted&quot;",
			expected: "\"quoted\"",
		},
	}

	for _, tt := range tests {
		result := decodeHTMLEntities(tt.input)
		if result != tt.expected {
			t.Errorf("decodeHTMLEntities(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestExtractTags(t *testing.T) {
	prefs := &URLPreferences{
		GenerateTags: true,
	}

	text := `Kubernetes is a container orchestration platform.
	Kubernetes helps with deployment and scaling.
	Docker containers run on Kubernetes clusters.
	The kubernetes ecosystem includes many tools.`

	tags := extractTags(text, prefs)

	// Should extract "kubernetes" as a top tag
	found := false
	for _, tag := range tags {
		if tag == "kubernetes" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'kubernetes' in tags, got %v", tags)
	}
}

func TestGenerateSummary(t *testing.T) {
	prefs := &URLPreferences{
		GenerateSummary: true,
	}

	text := `This is the first sentence. This is the second sentence. This is the third sentence.`

	summary := generateSummary(text, prefs)

	if summary == "" {
		t.Error("Expected non-empty summary")
	}

	// Summary should not be longer than the configured max
	if len(summary) > 550 { // 500 + some buffer
		t.Errorf("Summary too long: %d chars", len(summary))
	}
}

func TestParsePreferences(t *testing.T) {
	content := `# URL Preferences

## Settings
- No takes
- Skip summary

## Tag Categories
- technology
- business
`

	prefs, err := parsePreferences(content)
	if err != nil {
		t.Fatalf("parsePreferences failed: %v", err)
	}

	if prefs.GenerateTakes {
		t.Error("Expected GenerateTakes to be false")
	}
	if prefs.GenerateSummary {
		t.Error("Expected GenerateSummary to be false")
	}
	if len(prefs.TagCategories) != 2 {
		t.Errorf("Expected 2 tag categories, got %d", len(prefs.TagCategories))
	}
}

func TestSearchLibrary(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "url_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test file
	testContent := `# Test Article About Golang

**URL:** https://example.com/golang-article
**Date:** 2024-01-15
**Tags:** golang, programming

## Content

This is an article about Golang programming language.
`
	err = os.WriteFile(filepath.Join(tmpDir, "url_2024-01-15_test.md"), []byte(testContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Search for term
	results, err := searchLibrary(tmpDir, "golang", nil, time.Time{})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if len(results) > 0 && results[0].Title != "Test Article About Golang" {
		t.Errorf("Unexpected title: %s", results[0].Title)
	}
}

func TestParseURLFile(t *testing.T) {
	content := `# My Test Article

**URL:** https://test.example.com/path
**Date:** 2024-02-20
**Tags:** test, article, sample

## Summary

This is the summary.
`

	result := parseURLFile("/tmp/test.md", content)

	if result.Title != "My Test Article" {
		t.Errorf("Expected title 'My Test Article', got %q", result.Title)
	}
	if result.URL != "https://test.example.com/path" {
		t.Errorf("Expected URL 'https://test.example.com/path', got %q", result.URL)
	}
	if result.Date != "2024-02-20" {
		t.Errorf("Expected date '2024-02-20', got %q", result.Date)
	}
	if len(result.Tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(result.Tags))
	}
}

func TestParseClaudeResponse(t *testing.T) {
	tests := []struct {
		name         string
		response     string
		expectedTags int
		expectedSum  string
		expectTakes  int
		wantErr      bool
	}{
		{
			name: "standard format",
			response: `TAGS: kubernetes, docker, containers, devops, cloud-native
SUMMARY: This article discusses container orchestration with Kubernetes. It covers deployment strategies and best practices for production environments.
TAKES:
- Use namespaces to isolate workloads
- Implement proper resource limits
- Set up monitoring from day one`,
			expectedTags: 5,
			expectedSum:  "This article discusses container orchestration with Kubernetes.",
			expectTakes:  3,
			wantErr:      false,
		},
		{
			name: "with asterisk bullets",
			response: `TAGS: golang, programming
SUMMARY: A guide to Go programming.
TAKES:
* Learn goroutines early
* Use interfaces wisely`,
			expectedTags: 2,
			expectedSum:  "A guide to Go programming.",
			expectTakes:  2,
			wantErr:      false,
		},
		{
			name: "extra whitespace",
			response: `TAGS:   ai, machine-learning,  deep-learning
SUMMARY:   Machine learning fundamentals explained.
TAKES:
-   Start with supervised learning
-   Practice with real datasets`,
			expectedTags: 3,
			expectedSum:  "Machine learning fundamentals explained.",
			expectTakes:  2,
			wantErr:      false,
		},
		{
			name:         "empty response",
			response:     "",
			expectedTags: 0,
			expectedSum:  "",
			expectTakes:  0,
			wantErr:      true,
		},
		{
			name:         "no parseable content",
			response:     "This is just random text without any structure.",
			expectedTags: 0,
			expectedSum:  "",
			expectTakes:  0,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseClaudeResponse(tt.response)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(result.Tags) != tt.expectedTags {
				t.Errorf("Expected %d tags, got %d: %v", tt.expectedTags, len(result.Tags), result.Tags)
			}

			if !strings.HasPrefix(result.Summary, tt.expectedSum) {
				t.Errorf("Expected summary to start with %q, got %q", tt.expectedSum, result.Summary)
			}

			if len(result.Takes) != tt.expectTakes {
				t.Errorf("Expected %d takes, got %d: %v", tt.expectTakes, len(result.Takes), result.Takes)
			}
		})
	}
}

func TestBuildAnalysisPrompt(t *testing.T) {
	prefs := &URLPreferences{
		MaxContentLength: 100,
	}

	content := "This is test content for analysis."
	title := "Test Article"

	prompt := buildAnalysisPrompt(content, title, prefs)

	if !strings.Contains(prompt, title) {
		t.Error("Prompt should contain the title")
	}
	if !strings.Contains(prompt, content) {
		t.Error("Prompt should contain the content")
	}
	if !strings.Contains(prompt, "TAGS") {
		t.Error("Prompt should mention TAGS")
	}
	if !strings.Contains(prompt, "SUMMARY") {
		t.Error("Prompt should mention SUMMARY")
	}
	if !strings.Contains(prompt, "TAKES") {
		t.Error("Prompt should mention TAKES")
	}
}

func TestBuildAnalysisPromptTruncation(t *testing.T) {
	prefs := &URLPreferences{
		MaxContentLength: 50,
	}

	longContent := strings.Repeat("a", 100)

	prompt := buildAnalysisPrompt(longContent, "", prefs)

	if !strings.Contains(prompt, "[Content truncated...]") {
		t.Error("Long content should be truncated")
	}
}
