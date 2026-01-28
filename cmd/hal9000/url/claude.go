// Package url implements the hal9000 url command for processing and saving URLs.
package url

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pearcec/hal9000/internal/config"
)

// ClaudeAnalysis invokes Claude to analyze URL content
func ClaudeAnalysis(url string, content *FetchResult, rawPreferences string) (*Analysis, error) {
	prompt := buildAnalysisPrompt(url, content, rawPreferences)

	// Invoke claude CLI with -p flag for non-interactive output
	// The prompt is passed as a positional argument after -p
	cmd := exec.Command("claude", "-p", prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// If claude command fails, return error with stderr
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("claude analysis failed: %s", stderr.String())
		}
		return nil, fmt.Errorf("claude analysis failed: %w", err)
	}

	response := stdout.String()

	// Parse Claude's response into Analysis struct
	analysis := parseClaudeResponse(response, content.Title)

	// If parsing found nothing, Claude might have responded conversationally
	// Log the response for debugging
	if len(analysis.Tags) == 0 && analysis.Summary == "" && len(analysis.Takes) == 0 {
		// Try to extract any useful content from a conversational response
		analysis = parseConversationalResponse(response, content.Title)
	}

	return analysis, nil
}

func buildAnalysisPrompt(url string, content *FetchResult, rawPreferences string) string {
	var sb strings.Builder

	sb.WriteString("You are HAL 9000, processing a URL for the knowledge library.\n\n")

	sb.WriteString("## Task\n")
	sb.WriteString("Analyze the web content below and generate a markdown document for the library.\n\n")

	sb.WriteString("## Default Output Sections\n")
	sb.WriteString("Unless preferences specify otherwise, include:\n")
	sb.WriteString("- **Tags**: 5-8 relevant topic tags (lowercase, hyphenated)\n")
	sb.WriteString("- **Summary**: 2-3 sentence summary\n")
	sb.WriteString("- **Key Takes**: 3-4 key insights as bullet points\n\n")

	// Include raw user preferences - Claude interprets them
	if rawPreferences != "" {
		sb.WriteString("## User Preferences\n")
		sb.WriteString("IMPORTANT: Follow these preferences exactly. They override defaults.\n\n")
		sb.WriteString(rawPreferences)
		sb.WriteString("\n\n")
	}

	// URL and content
	sb.WriteString("## URL Being Processed\n")
	sb.WriteString(url + "\n\n")

	sb.WriteString("## Page Title\n")
	sb.WriteString(content.Title + "\n\n")

	sb.WriteString("## Page Content\n")
	body := content.Body
	if len(body) > 8000 {
		body = body[:8000] + "\n[Content truncated...]"
	}
	sb.WriteString(body + "\n\n")

	// Output format - flexible markdown
	sb.WriteString("## Output Format\n")
	sb.WriteString("Generate clean markdown. Start with:\n")
	sb.WriteString("```\n")
	sb.WriteString("TAGS: tag1, tag2, tag3, ...\n")
	sb.WriteString("\n")
	sb.WriteString("## Summary\n")
	sb.WriteString("Your summary here.\n")
	sb.WriteString("\n")
	sb.WriteString("## Key Takes\n")
	sb.WriteString("- Insight one\n")
	sb.WriteString("- Insight two\n")
	sb.WriteString("```\n\n")
	sb.WriteString("Add any additional sections the user preferences request (e.g., Manager Notes).\n")
	sb.WriteString("Output ONLY the markdown content, no explanations.\n")

	return sb.String()
}

func parseClaudeResponse(response string, fallbackTitle string) *Analysis {
	analysis := &Analysis{
		Title: fallbackTitle,
	}

	lines := strings.Split(response, "\n")
	var extraSections []string
	inExtraSection := false
	currentSection := ""
	takesEndLine := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Handle tags
		if strings.HasPrefix(trimmed, "TAGS:") || strings.HasPrefix(trimmed, "**Tags:**") || strings.HasPrefix(trimmed, "Tags:") {
			tagsStr := trimmed
			tagsStr = strings.TrimPrefix(tagsStr, "TAGS:")
			tagsStr = strings.TrimPrefix(tagsStr, "**Tags:**")
			tagsStr = strings.TrimPrefix(tagsStr, "Tags:")
			tagsStr = strings.TrimSpace(tagsStr)
			for _, tag := range strings.Split(tagsStr, ",") {
				tag = strings.TrimSpace(tag)
				tag = strings.Trim(tag, "`")
				if tag != "" {
					analysis.Tags = append(analysis.Tags, tag)
				}
			}
			continue
		}

		// Handle summary section
		if strings.HasPrefix(trimmed, "## Summary") || strings.HasPrefix(trimmed, "SUMMARY:") {
			currentSection = "summary"
			if strings.HasPrefix(trimmed, "SUMMARY:") {
				analysis.Summary = strings.TrimSpace(strings.TrimPrefix(trimmed, "SUMMARY:"))
			}
			continue
		}

		// Handle takes/key takes section
		if strings.HasPrefix(trimmed, "## Key Takes") || strings.HasPrefix(trimmed, "TAKES:") || strings.HasPrefix(trimmed, "## Takes") {
			currentSection = "takes"
			continue
		}

		// Detect other ## sections (manager notes, etc.)
		if strings.HasPrefix(trimmed, "## ") && currentSection != "" {
			header := strings.TrimPrefix(trimmed, "## ")
			headerLower := strings.ToLower(header)
			if headerLower != "summary" && headerLower != "key takes" && headerLower != "takes" && headerLower != "content" {
				// This is an extra section Claude generated
				currentSection = "extra"
				inExtraSection = true
				takesEndLine = i
				extraSections = append(extraSections, line)
				continue
			}
			if headerLower == "content" {
				// Stop processing - content section is added separately
				break
			}
		}

		// Accumulate content based on current section
		switch currentSection {
		case "summary":
			if trimmed != "" && !strings.HasPrefix(trimmed, "##") && trimmed != "```" {
				if analysis.Summary != "" {
					analysis.Summary += " "
				}
				analysis.Summary += trimmed
			}
		case "takes":
			if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
				take := strings.TrimPrefix(trimmed, "- ")
				take = strings.TrimPrefix(take, "* ")
				analysis.Takes = append(analysis.Takes, take)
			}
		case "extra":
			if inExtraSection {
				extraSections = append(extraSections, line)
			}
		}
	}

	// Combine extra sections
	if len(extraSections) > 0 {
		analysis.RawExtra = strings.Join(extraSections, "\n")
	}

	// If we didn't find anything, mark where takes ended for debugging
	_ = takesEndLine

	return analysis
}

// parseConversationalResponse attempts to extract content from a more free-form Claude response
func parseConversationalResponse(response string, fallbackTitle string) *Analysis {
	analysis := &Analysis{
		Title: fallbackTitle,
	}

	lines := strings.Split(response, "\n")
	inSection := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Detect section headers (## Tags, ## Summary, etc.)
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
			header := strings.TrimPrefix(line, "## ")
			header = strings.TrimPrefix(header, "### ")
			header = strings.ToLower(header)
			if strings.Contains(header, "tag") {
				inSection = "tags"
			} else if strings.Contains(header, "summary") {
				inSection = "summary"
			} else if strings.Contains(header, "take") || strings.Contains(header, "insight") || strings.Contains(header, "key point") {
				inSection = "takes"
			} else {
				inSection = ""
			}
			continue
		}

		// Process content based on current section
		switch inSection {
		case "tags":
			// Tags might be comma-separated or bullet points
			if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
				tag := strings.TrimPrefix(line, "- ")
				tag = strings.TrimPrefix(tag, "* ")
				tag = strings.TrimSpace(tag)
				if tag != "" {
					analysis.Tags = append(analysis.Tags, tag)
				}
			} else if line != "" && !strings.HasPrefix(line, "#") {
				// Might be comma-separated on one line
				for _, tag := range strings.Split(line, ",") {
					tag = strings.TrimSpace(tag)
					tag = strings.Trim(tag, "`")
					if tag != "" && len(tag) < 50 {
						analysis.Tags = append(analysis.Tags, tag)
					}
				}
			}
		case "summary":
			if line != "" && !strings.HasPrefix(line, "#") {
				if analysis.Summary != "" {
					analysis.Summary += " "
				}
				analysis.Summary += line
			}
		case "takes":
			if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "1.") || strings.HasPrefix(line, "2.") || strings.HasPrefix(line, "3.") {
				take := line
				take = strings.TrimPrefix(take, "- ")
				take = strings.TrimPrefix(take, "* ")
				// Remove numbered list prefixes
				for i := 1; i <= 9; i++ {
					take = strings.TrimPrefix(take, fmt.Sprintf("%d. ", i))
				}
				take = strings.TrimSpace(take)
				if take != "" {
					analysis.Takes = append(analysis.Takes, take)
				}
			}
		}
	}

	return analysis
}

// GenerateAndSaveWithClaude performs full URL processing with Claude
func GenerateAndSaveWithClaude(url string, content *FetchResult, rawPreferences string, dryRun bool) (string, error) {
	fmt.Println("Analyzing with Claude...")

	analysis, err := ClaudeAnalysis(url, content, rawPreferences)
	if err != nil {
		return "", fmt.Errorf("Claude analysis failed: %w", err)
	}

	// Generate output markdown using Claude's analysis
	output := generateOutputFromAnalysis(url, content, analysis)

	if dryRun {
		fmt.Println("\n--- DRY RUN: Would save the following ---")
		fmt.Println(output)
		return "", nil
	}

	// Save to library (with duplicate detection)
	return saveToLibraryPath(url, content, output)
}

func generateOutputFromAnalysis(url string, content *FetchResult, analysis *Analysis) string {
	var sb strings.Builder

	title := analysis.Title
	if title == "" {
		title = content.Title
	}
	if title == "" {
		title = "Untitled"
	}

	// Header
	sb.WriteString(fmt.Sprintf("# %s\n\n", title))
	sb.WriteString(fmt.Sprintf("**URL:** %s\n", url))
	sb.WriteString(fmt.Sprintf("**Date:** %s\n", time.Now().Format("2006-01-02")))

	// Tags from analysis
	if len(analysis.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("**Tags:** %s\n", strings.Join(analysis.Tags, ", ")))
	}

	sb.WriteString("\n")

	// Claude-generated summary
	if analysis.Summary != "" {
		sb.WriteString("## Summary\n\n")
		sb.WriteString(analysis.Summary)
		sb.WriteString("\n\n")
	}

	// Claude-generated takes
	if len(analysis.Takes) > 0 {
		sb.WriteString("## Key Takes\n\n")
		for _, take := range analysis.Takes {
			sb.WriteString(fmt.Sprintf("- %s\n", take))
		}
		sb.WriteString("\n")
	}

	// Any extra content Claude generated (manager notes, etc.) is in RawExtra
	if analysis.RawExtra != "" {
		sb.WriteString(analysis.RawExtra)
		sb.WriteString("\n")
	}

	// Content excerpt
	bodyExcerpt := content.Body
	if len(bodyExcerpt) > 5000 {
		bodyExcerpt = bodyExcerpt[:5000] + "\n\n[Content truncated...]"
	}

	sb.WriteString("## Content\n\n")
	sb.WriteString(bodyExcerpt)
	sb.WriteString("\n")

	return sb.String()
}

func saveToLibraryPath(url string, content *FetchResult, output string) (string, error) {
	libPath := filepath.Join(config.GetLibraryPath(), "url_library")

	// Ensure directory exists
	if err := os.MkdirAll(libPath, 0755); err != nil {
		return "", err
	}

	// Check if this URL already exists in the library
	existingFile := findExistingURLInPath(libPath, url)
	if existingFile != "" {
		// Update existing file
		fmt.Printf("Updating existing entry: %s\n", filepath.Base(existingFile))
		if err := os.WriteFile(existingFile, []byte(output), 0644); err != nil {
			return "", err
		}
		return existingFile, nil
	}

	// Generate filename: url_YYYY-MM-DD_{descriptor}.md
	date := time.Now().Format("2006-01-02")
	descriptor := generateDescriptorFromTitle(content.Title, url)
	filename := fmt.Sprintf("url_%s_%s.md", date, descriptor)
	fullPath := filepath.Join(libPath, filename)

	// Handle duplicate filenames (different URLs with same title/date)
	if _, err := os.Stat(fullPath); err == nil {
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

// findExistingURLInPath searches the library for an existing entry with the same URL
func findExistingURLInPath(libPath, targetURL string) string {
	entries, err := os.ReadDir(libPath)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(libPath, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		// Look for **URL:** line and compare
		for _, line := range strings.Split(string(content), "\n") {
			if strings.HasPrefix(line, "**URL:**") {
				fileURL := strings.TrimSpace(strings.TrimPrefix(line, "**URL:**"))
				if fileURL == targetURL {
					return filePath
				}
				break
			}
		}
	}

	return ""
}

func generateDescriptorFromTitle(title, url string) string {
	source := title
	if source == "" {
		source = url
		if idx := strings.Index(source, "://"); idx != -1 {
			source = source[idx+3:]
		}
		if idx := strings.Index(source, "/"); idx != -1 {
			source = source[:idx]
		}
	}

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

	if len(s) > 50 {
		s = s[:50]
	}
	if s == "" {
		s = "untitled"
	}

	return s
}
