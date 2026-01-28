// Package url implements the hal9000 url command for processing and saving URLs.
package url

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	libraryPath string
	dryRun      bool
	noAnalyze   bool
)

// Cmd is the url command
var Cmd = &cobra.Command{
	Use:   "url <URL>",
	Short: "Process and save a URL to the library",
	Long: `I can process URLs and save them to your knowledge library.

When you provide a URL, I will:
1. Fetch the content from the URL
2. Analyze it (extract title, generate tags, summary, and takes)
3. Save it to library/url_library/ with a dated filename

The saved document includes:
- Original URL
- Title
- Date processed
- Tags (auto-generated based on content)
- Summary
- Key takes/insights
- Raw content excerpt

Examples:
  hal9000 url https://example.com/article
  hal9000 url https://blog.example.com/post --dry-run
  hal9000 url https://news.example.com/story --no-analyze`,
	Args: cobra.ExactArgs(1),
	RunE: runURL,
}

func init() {
	Cmd.Flags().StringVar(&libraryPath, "library-path", "", "Override default library location")
	Cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be saved without saving")
	Cmd.Flags().BoolVar(&noAnalyze, "no-analyze", false, "Skip content analysis, save raw content only")

	Cmd.AddCommand(searchCmd)
}

func runURL(cmd *cobra.Command, args []string) error {
	url := args[0]

	// Validate URL
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("invalid URL: must start with http:// or https://")
	}

	fmt.Printf("Fetching: %s\n", url)

	// Fetch content
	content, err := Fetch(url)
	if err != nil {
		return fmt.Errorf("failed to fetch URL: %w", err)
	}

	fmt.Printf("Fetched %d bytes, title: %s\n", len(content.Body), content.Title)

	// Load preferences
	prefs, err := loadPreferences()
	if err != nil {
		// Preferences are optional, continue with defaults
		fmt.Printf("Note: No URL preferences found, using defaults\n")
		prefs = &URLPreferences{
			MaxContentLength: 5000,
			GenerateTags:     true,
			GenerateSummary:  true,
			GenerateTakes:    true,
		}
	}

	// Analyze content (unless --no-analyze)
	var analysis *Analysis
	if !noAnalyze {
		fmt.Println("Analyzing content...")
		analysis, err = Analyze(content, prefs)
		if err != nil {
			fmt.Printf("Warning: Analysis failed, saving raw content: %v\n", err)
			analysis = &Analysis{
				Title:   content.Title,
				Tags:    []string{},
				Summary: "",
				Takes:   []string{},
			}
		}
	} else {
		analysis = &Analysis{
			Title:   content.Title,
			Tags:    []string{},
			Summary: "",
			Takes:   []string{},
		}
	}

	// Generate output
	output := generateOutput(url, content, analysis, prefs)

	if dryRun {
		fmt.Println("\n--- DRY RUN: Would save the following ---")
		fmt.Println(output)
		return nil
	}

	// Save to library
	filename, err := saveToLibrary(url, content, output)
	if err != nil {
		return fmt.Errorf("failed to save to library: %w", err)
	}

	fmt.Printf("\nSaved to: %s\n", filename)
	fmt.Println("I am completely operational, and the URL has been processed.")
	return nil
}

func loadPreferences() (*URLPreferences, error) {
	prefsPath := getPreferencesPath()
	content, err := os.ReadFile(prefsPath)
	if err != nil {
		return nil, err
	}

	return parsePreferences(string(content))
}

func getPreferencesPath() string {
	base := libraryPath
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, "Documents", "Google Drive", "Claude")
	}
	return filepath.Join(base, "preferences", "url.md")
}

func getLibraryPath() string {
	base := libraryPath
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, "Documents", "Google Drive", "Claude")
	}
	return filepath.Join(base, "url_library")
}

func generateOutput(url string, content *FetchResult, analysis *Analysis, prefs *URLPreferences) string {
	var sb strings.Builder

	title := analysis.Title
	if title == "" {
		title = content.Title
	}
	if title == "" {
		title = "Untitled"
	}

	sb.WriteString(fmt.Sprintf("# %s\n\n", title))
	sb.WriteString(fmt.Sprintf("**URL:** %s\n", url))
	sb.WriteString(fmt.Sprintf("**Date:** %s\n", time.Now().Format("2006-01-02")))

	if len(analysis.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("**Tags:** %s\n", strings.Join(analysis.Tags, ", ")))
	}

	sb.WriteString("\n")

	if analysis.Summary != "" {
		sb.WriteString("## Summary\n\n")
		sb.WriteString(analysis.Summary)
		sb.WriteString("\n\n")
	}

	if len(analysis.Takes) > 0 {
		sb.WriteString("## Key Takes\n\n")
		for _, take := range analysis.Takes {
			sb.WriteString(fmt.Sprintf("- %s\n", take))
		}
		sb.WriteString("\n")
	}

	// Add content excerpt
	maxLen := 5000
	if prefs != nil && prefs.MaxContentLength > 0 {
		maxLen = prefs.MaxContentLength
	}

	bodyExcerpt := content.Body
	if len(bodyExcerpt) > maxLen {
		bodyExcerpt = bodyExcerpt[:maxLen] + "\n\n[Content truncated...]"
	}

	sb.WriteString("## Content\n\n")
	sb.WriteString(bodyExcerpt)
	sb.WriteString("\n")

	return sb.String()
}

func saveToLibrary(url string, content *FetchResult, output string) (string, error) {
	libPath := getLibraryPath()

	// Ensure directory exists
	if err := os.MkdirAll(libPath, 0755); err != nil {
		return "", err
	}

	// Generate filename: url_YYYY-MM-DD_{descriptor}.md
	date := time.Now().Format("2006-01-02")
	descriptor := generateDescriptor(content.Title, url)
	filename := fmt.Sprintf("url_%s_%s.md", date, descriptor)
	fullPath := filepath.Join(libPath, filename)

	// Handle duplicate filenames
	if _, err := os.Stat(fullPath); err == nil {
		// File exists, add a counter
		for i := 2; i < 100; i++ {
			filename = fmt.Sprintf("url_%s_%s_%d.md", date, descriptor, i)
			fullPath = filepath.Join(libPath, filename)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				break
			}
		}
	}

	if err := os.WriteFile(fullPath, []byte(output), 0644); err != nil {
		return "", err
	}

	return fullPath, nil
}

func generateDescriptor(title, url string) string {
	// Use title if available, otherwise use URL domain
	source := title
	if source == "" {
		// Extract domain from URL
		source = url
		if idx := strings.Index(source, "://"); idx != -1 {
			source = source[idx+3:]
		}
		if idx := strings.Index(source, "/"); idx != -1 {
			source = source[:idx]
		}
	}

	// Clean up for filename
	source = strings.ToLower(source)
	var result strings.Builder
	wordCount := 0
	inWord := false

	for _, r := range source {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
			inWord = true
		} else if inWord {
			result.WriteRune('-')
			inWord = false
			wordCount++
			if wordCount >= 5 {
				break
			}
		}
	}

	s := result.String()
	s = strings.Trim(s, "-")

	// Ensure reasonable length
	if len(s) > 50 {
		s = s[:50]
	}
	if s == "" {
		s = "untitled"
	}

	return s
}
