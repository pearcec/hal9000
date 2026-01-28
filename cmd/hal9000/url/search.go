package url

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	searchLimit int
	searchTags  string
	searchSince string
)

var searchCmd = &cobra.Command{
	Use:   "search <term>",
	Short: "Search the URL library",
	Long: `I can search through your saved URLs in the library.

Search matches against:
- Title
- URL
- Tags
- Content

Examples:
  hal9000 url search golang
  hal9000 url search "machine learning" --limit=5
  hal9000 url search api --tags=technology
  hal9000 url search kubernetes --since=2024-01-01`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().IntVar(&searchLimit, "limit", 10, "Maximum number of results")
	searchCmd.Flags().StringVar(&searchTags, "tags", "", "Filter by tags (comma-separated)")
	searchCmd.Flags().StringVar(&searchSince, "since", "", "Only show URLs saved since date (YYYY-MM-DD)")
}

// SearchResult represents a search match
type SearchResult struct {
	Filename string
	Title    string
	URL      string
	Date     string
	Tags     []string
	Excerpt  string
	Score    int
}

func runSearch(cmd *cobra.Command, args []string) error {
	term := args[0]
	libPath := getLibraryPath()

	// Check if library exists
	if _, err := os.Stat(libPath); os.IsNotExist(err) {
		fmt.Println("I'm sorry, Dave. The URL library does not exist yet.")
		fmt.Println("Use 'hal9000 url <URL>' to add URLs to the library.")
		return nil
	}

	// Parse since date if provided
	var sinceDate time.Time
	if searchSince != "" {
		var err error
		sinceDate, err = time.Parse("2006-01-02", searchSince)
		if err != nil {
			return fmt.Errorf("invalid date format for --since, expected YYYY-MM-DD: %w", err)
		}
	}

	// Parse tags filter
	var tagFilter []string
	if searchTags != "" {
		for _, tag := range strings.Split(searchTags, ",") {
			tagFilter = append(tagFilter, strings.TrimSpace(strings.ToLower(tag)))
		}
	}

	// Search files
	results, err := searchLibrary(libPath, term, tagFilter, sinceDate)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Printf("No results found for '%s'\n", term)
		return nil
	}

	// Apply limit
	if searchLimit > 0 && len(results) > searchLimit {
		results = results[:searchLimit]
	}

	// Display results
	fmt.Printf("Found %d results for '%s':\n\n", len(results), term)
	for i, r := range results {
		fmt.Printf("%d. %s\n", i+1, r.Title)
		fmt.Printf("   URL:  %s\n", r.URL)
		fmt.Printf("   Date: %s\n", r.Date)
		if len(r.Tags) > 0 {
			fmt.Printf("   Tags: %s\n", strings.Join(r.Tags, ", "))
		}
		if r.Excerpt != "" {
			fmt.Printf("   ...%s...\n", r.Excerpt)
		}
		fmt.Printf("   File: %s\n", r.Filename)
		fmt.Println()
	}

	return nil
}

func searchLibrary(libPath, term string, tagFilter []string, sinceDate time.Time) ([]SearchResult, error) {
	var results []SearchResult
	termLower := strings.ToLower(term)
	termRegex := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(term))

	err := filepath.Walk(libPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		// Read file
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		contentStr := string(content)
		contentLower := strings.ToLower(contentStr)

		// Check if term matches
		if !strings.Contains(contentLower, termLower) {
			return nil
		}

		// Parse the file
		result := parseURLFile(path, contentStr)

		// Apply date filter
		if !sinceDate.IsZero() {
			fileDate, err := time.Parse("2006-01-02", result.Date)
			if err == nil && fileDate.Before(sinceDate) {
				return nil
			}
		}

		// Apply tag filter
		if len(tagFilter) > 0 {
			hasTag := false
			for _, filterTag := range tagFilter {
				for _, fileTag := range result.Tags {
					if strings.ToLower(fileTag) == filterTag {
						hasTag = true
						break
					}
				}
				if hasTag {
					break
				}
			}
			if !hasTag {
				return nil
			}
		}

		// Calculate relevance score
		score := 0
		// Higher score for title matches
		if strings.Contains(strings.ToLower(result.Title), termLower) {
			score += 10
		}
		// Higher score for URL matches
		if strings.Contains(strings.ToLower(result.URL), termLower) {
			score += 5
		}
		// Count occurrences in content
		score += strings.Count(contentLower, termLower)
		result.Score = score

		// Extract excerpt around match
		result.Excerpt = extractExcerpt(contentStr, termRegex)

		results = append(results, result)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, nil
}

func parseURLFile(path, content string) SearchResult {
	result := SearchResult{
		Filename: filepath.Base(path),
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Extract title (first H1)
		if strings.HasPrefix(line, "# ") && result.Title == "" {
			result.Title = strings.TrimPrefix(line, "# ")
			continue
		}

		// Extract URL
		if strings.HasPrefix(line, "**URL:**") {
			result.URL = strings.TrimSpace(strings.TrimPrefix(line, "**URL:**"))
			continue
		}

		// Extract date
		if strings.HasPrefix(line, "**Date:**") {
			result.Date = strings.TrimSpace(strings.TrimPrefix(line, "**Date:**"))
			continue
		}

		// Extract tags
		if strings.HasPrefix(line, "**Tags:**") {
			tagsStr := strings.TrimSpace(strings.TrimPrefix(line, "**Tags:**"))
			for _, tag := range strings.Split(tagsStr, ",") {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					result.Tags = append(result.Tags, tag)
				}
			}
			continue
		}
	}

	// Default title from filename if not found
	if result.Title == "" {
		result.Title = result.Filename
	}

	return result
}

func extractExcerpt(content string, termRegex *regexp.Regexp) string {
	// Find first match location
	loc := termRegex.FindStringIndex(content)
	if loc == nil {
		return ""
	}

	// Extract context around match
	start := loc[0] - 50
	if start < 0 {
		start = 0
	}
	end := loc[1] + 50
	if end > len(content) {
		end = len(content)
	}

	excerpt := content[start:end]

	// Clean up excerpt
	excerpt = strings.ReplaceAll(excerpt, "\n", " ")
	excerpt = strings.TrimSpace(excerpt)

	// Truncate to reasonable length
	if len(excerpt) > 100 {
		excerpt = excerpt[:100]
	}

	return excerpt
}
