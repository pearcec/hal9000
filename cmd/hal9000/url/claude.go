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
func ClaudeAnalysis(url string, content *FetchResult, prefs *URLPreferences) (*Analysis, error) {
	prompt := buildAnalysisPrompt(url, content, prefs)

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

func buildAnalysisPrompt(url string, content *FetchResult, prefs *URLPreferences) string {
	var sb strings.Builder

	// Instructions from CLAUDE.md pattern
	sb.WriteString("You are HAL 9000, processing a URL for the library.\n\n")

	sb.WriteString("## Task\n")
	sb.WriteString("Analyze the following web content and provide:\n")
	sb.WriteString("1. **Tags**: 5-8 relevant topic tags (lowercase, single words or hyphenated)\n")
	sb.WriteString("2. **Summary**: 2-3 sentence summary of the content\n")
	sb.WriteString("3. **Takes**: 3-4 key insights or takeaways as bullet points\n\n")

	// Include user preferences if available
	if prefs != nil {
		sb.WriteString("## User Preferences\n")
		if len(prefs.TagCategories) > 0 {
			sb.WriteString(fmt.Sprintf("- Preferred tag categories: %s\n", strings.Join(prefs.TagCategories, ", ")))
		}
		if len(prefs.ExcludePatterns) > 0 {
			sb.WriteString(fmt.Sprintf("- Exclude patterns: %s\n", strings.Join(prefs.ExcludePatterns, ", ")))
		}
		if !prefs.GenerateTags {
			sb.WriteString("- Skip tags generation\n")
		}
		if !prefs.GenerateSummary {
			sb.WriteString("- Skip summary generation\n")
		}
		if !prefs.GenerateTakes {
			sb.WriteString("- Skip takes generation\n")
		}
		sb.WriteString("\n")
	}

	// URL and content
	sb.WriteString("## URL\n")
	sb.WriteString(url + "\n\n")

	sb.WriteString("## Title\n")
	sb.WriteString(content.Title + "\n\n")

	sb.WriteString("## Content\n")
	// Truncate content if too long
	body := content.Body
	maxLen := 8000
	if prefs != nil && prefs.MaxContentLength > 0 {
		maxLen = prefs.MaxContentLength
	}
	if len(body) > maxLen {
		body = body[:maxLen] + "\n[Content truncated...]"
	}
	sb.WriteString(body + "\n\n")

	// Output format instructions
	sb.WriteString("## Required Output Format\n")
	sb.WriteString("Respond with EXACTLY this format (no extra text):\n\n")
	sb.WriteString("```\n")
	sb.WriteString("TAGS: tag1, tag2, tag3, tag4, tag5\n")
	sb.WriteString("SUMMARY: Your 2-3 sentence summary here.\n")
	sb.WriteString("TAKES:\n")
	sb.WriteString("- First key insight\n")
	sb.WriteString("- Second key insight\n")
	sb.WriteString("- Third key insight\n")
	sb.WriteString("```\n")

	return sb.String()
}

func parseClaudeResponse(response string, fallbackTitle string) *Analysis {
	analysis := &Analysis{
		Title: fallbackTitle,
	}

	lines := strings.Split(response, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Handle various tag formats
		if strings.HasPrefix(line, "TAGS:") || strings.HasPrefix(line, "**Tags:**") || strings.HasPrefix(line, "Tags:") {
			tagsStr := line
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
		} else if strings.HasPrefix(line, "SUMMARY:") || strings.HasPrefix(line, "**Summary:**") || strings.HasPrefix(line, "Summary:") {
			summaryStr := line
			summaryStr = strings.TrimPrefix(summaryStr, "SUMMARY:")
			summaryStr = strings.TrimPrefix(summaryStr, "**Summary:**")
			summaryStr = strings.TrimPrefix(summaryStr, "Summary:")
			analysis.Summary = strings.TrimSpace(summaryStr)
		} else if strings.HasPrefix(line, "TAKES:") || strings.HasPrefix(line, "**Key Takes:**") || strings.HasPrefix(line, "Key Takes:") || strings.HasPrefix(line, "**Takes:**") {
			// Read subsequent lines starting with "-"
			for j := i + 1; j < len(lines); j++ {
				takeLine := strings.TrimSpace(lines[j])
				if strings.HasPrefix(takeLine, "- ") || strings.HasPrefix(takeLine, "* ") {
					take := strings.TrimPrefix(takeLine, "- ")
					take = strings.TrimPrefix(take, "* ")
					take = strings.TrimPrefix(take, "**")
					take = strings.TrimSuffix(take, "**")
					analysis.Takes = append(analysis.Takes, take)
				} else if takeLine == "```" || takeLine == "" {
					continue
				} else if !strings.HasPrefix(takeLine, "-") && !strings.HasPrefix(takeLine, "*") && takeLine != "" {
					break
				}
			}
		}
	}

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
func GenerateAndSaveWithClaude(url string, content *FetchResult, prefs *URLPreferences, dryRun bool) (string, error) {
	fmt.Println("Analyzing with Claude...")

	analysis, err := ClaudeAnalysis(url, content, prefs)
	if err != nil {
		return "", fmt.Errorf("Claude analysis failed: %w", err)
	}

	// Generate output markdown
	output := generateOutputMarkdown(url, content, analysis, prefs)

	if dryRun {
		fmt.Println("\n--- DRY RUN: Would save the following ---")
		fmt.Println(output)
		return "", nil
	}

	// Save to library
	return saveToLibraryPath(url, content, output)
}

func generateOutputMarkdown(url string, content *FetchResult, analysis *Analysis, prefs *URLPreferences) string {
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

func saveToLibraryPath(url string, content *FetchResult, output string) (string, error) {
	libPath := filepath.Join(config.GetLibraryPath(), "url_library")

	// Ensure directory exists
	if err := os.MkdirAll(libPath, 0755); err != nil {
		return "", err
	}

	// Generate filename: url_YYYY-MM-DD_{descriptor}.md
	date := time.Now().Format("2006-01-02")
	descriptor := generateDescriptorFromTitle(content.Title, url)
	filename := fmt.Sprintf("url_%s_%s.md", date, descriptor)
	fullPath := filepath.Join(libPath, filename)

	// Handle duplicate filenames
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
