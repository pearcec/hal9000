package url

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// FetchResult contains the result of fetching a URL
type FetchResult struct {
	URL         string
	Title       string
	Body        string
	ContentType string
	StatusCode  int
}

// Fetch retrieves content from a URL
func Fetch(url string) (*FetchResult, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set a reasonable User-Agent
	req.Header.Set("User-Agent", "HAL9000/1.0 (Personal Knowledge Assistant)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Read body with size limit (10MB)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	bodyStr := string(body)

	// Extract title from HTML
	title := extractTitle(bodyStr)

	// Convert HTML to plain text if needed
	if strings.Contains(contentType, "text/html") {
		bodyStr = htmlToText(bodyStr)
	}

	return &FetchResult{
		URL:         url,
		Title:       title,
		Body:        bodyStr,
		ContentType: contentType,
		StatusCode:  resp.StatusCode,
	}, nil
}

func extractTitle(html string) string {
	// Try to extract title from <title> tag
	titleRegex := regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)
	matches := titleRegex.FindStringSubmatch(html)
	if len(matches) > 1 {
		title := strings.TrimSpace(matches[1])
		// Decode common HTML entities
		title = decodeHTMLEntities(title)
		return title
	}

	// Try og:title meta tag
	ogTitleRegex := regexp.MustCompile(`(?i)<meta[^>]*property=["']og:title["'][^>]*content=["']([^"']+)["']`)
	matches = ogTitleRegex.FindStringSubmatch(html)
	if len(matches) > 1 {
		return decodeHTMLEntities(strings.TrimSpace(matches[1]))
	}

	// Try reverse order meta tag
	ogTitleRegex2 := regexp.MustCompile(`(?i)<meta[^>]*content=["']([^"']+)["'][^>]*property=["']og:title["']`)
	matches = ogTitleRegex2.FindStringSubmatch(html)
	if len(matches) > 1 {
		return decodeHTMLEntities(strings.TrimSpace(matches[1]))
	}

	return ""
}

func htmlToText(html string) string {
	// Remove script and style elements
	scriptRegex := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	html = scriptRegex.ReplaceAllString(html, "")

	styleRegex := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = styleRegex.ReplaceAllString(html, "")

	// Remove HTML comments
	commentRegex := regexp.MustCompile(`(?s)<!--.*?-->`)
	html = commentRegex.ReplaceAllString(html, "")

	// Convert common block elements to newlines
	blockRegex := regexp.MustCompile(`(?i)</(p|div|h[1-6]|li|tr|br|hr)[^>]*>`)
	html = blockRegex.ReplaceAllString(html, "\n")

	brRegex := regexp.MustCompile(`(?i)<br[^>]*>`)
	html = brRegex.ReplaceAllString(html, "\n")

	// Remove all remaining HTML tags
	tagRegex := regexp.MustCompile(`<[^>]+>`)
	text := tagRegex.ReplaceAllString(html, "")

	// Decode HTML entities
	text = decodeHTMLEntities(text)

	// Normalize whitespace
	text = normalizeWhitespace(text)

	return text
}

func decodeHTMLEntities(s string) string {
	replacements := map[string]string{
		"&nbsp;":  " ",
		"&amp;":   "&",
		"&lt;":    "<",
		"&gt;":    ">",
		"&quot;":  "\"",
		"&#39;":   "'",
		"&apos;":  "'",
		"&mdash;": "-",
		"&ndash;": "-",
		"&bull;":  "*",
		"&copy;":  "(c)",
		"&reg;":   "(R)",
		"&trade;": "(TM)",
		"&hellip;": "...",
		"&rsquo;": "'",
		"&lsquo;": "'",
		"&rdquo;": "\"",
		"&ldquo;": "\"",
	}

	for entity, replacement := range replacements {
		s = strings.ReplaceAll(s, entity, replacement)
	}

	// Handle numeric entities
	numericRegex := regexp.MustCompile(`&#(\d+);`)
	s = numericRegex.ReplaceAllStringFunc(s, func(match string) string {
		var num int
		fmt.Sscanf(match, "&#%d;", &num)
		if num > 0 && num < 128 {
			return string(rune(num))
		}
		return match
	})

	return s
}

func normalizeWhitespace(s string) string {
	// Replace multiple spaces with single space
	spaceRegex := regexp.MustCompile(`[ \t]+`)
	s = spaceRegex.ReplaceAllString(s, " ")

	// Replace multiple newlines with double newline
	newlineRegex := regexp.MustCompile(`\n{3,}`)
	s = newlineRegex.ReplaceAllString(s, "\n\n")

	// Trim lines
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	s = strings.Join(lines, "\n")

	return strings.TrimSpace(s)
}
