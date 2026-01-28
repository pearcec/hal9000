package url

import (
	"regexp"
	"sort"
	"strings"
)

// Analysis contains the result of analyzing URL content
type Analysis struct {
	Title    string
	Tags     []string
	Summary  string
	Takes    []string
	RawExtra string // Any additional sections Claude generated
}

// URLPreferences contains user preferences for URL processing
type URLPreferences struct {
	MaxContentLength    int
	GenerateTags        bool
	GenerateSummary     bool
	GenerateTakes       bool
	GenerateManagerNotes bool
	TagCategories       []string
	ExcludePatterns     []string
	ManagerNotesPrompt  string
	DefaultTags         []string
}

// Analyze performs content analysis on fetched content
func Analyze(content *FetchResult, prefs *URLPreferences) (*Analysis, error) {
	analysis := &Analysis{
		Title: content.Title,
	}

	if prefs.GenerateTags {
		analysis.Tags = extractTags(content.Body, prefs)
	}

	if prefs.GenerateSummary {
		analysis.Summary = generateSummary(content.Body, prefs)
	}

	if prefs.GenerateTakes {
		analysis.Takes = extractTakes(content.Body, prefs)
	}

	return analysis, nil
}

func extractTags(text string, prefs *URLPreferences) []string {
	// Extract keywords based on word frequency
	words := tokenize(text)
	freq := make(map[string]int)

	// Common stop words to exclude
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "from": true,
		"is": true, "are": true, "was": true, "were": true, "be": true,
		"been": true, "being": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "will": true, "would": true,
		"could": true, "should": true, "may": true, "might": true, "must": true,
		"that": true, "this": true, "these": true, "those": true, "it": true,
		"its": true, "they": true, "them": true, "their": true, "what": true,
		"which": true, "who": true, "whom": true, "when": true, "where": true,
		"why": true, "how": true, "all": true, "each": true, "every": true,
		"both": true, "few": true, "more": true, "most": true, "other": true,
		"some": true, "such": true, "no": true, "not": true, "only": true,
		"same": true, "so": true, "than": true, "too": true, "very": true,
		"just": true, "can": true, "also": true, "into": true, "out": true,
		"up": true, "down": true, "about": true, "over": true, "after": true,
		"before": true, "between": true, "under": true, "again": true,
		"then": true, "once": true, "here": true, "there": true, "any": true,
		"as": true, "if": true, "because": true, "until": true, "while": true,
		"your": true, "you": true, "we": true, "our": true, "us": true,
		"my": true, "me": true, "i": true, "he": true, "she": true, "him": true,
		"her": true, "his": true, "like": true, "get": true, "got": true,
		"make": true, "made": true, "use": true, "used": true, "new": true,
		"one": true, "two": true, "first": true, "last": true, "time": true,
		"way": true, "see": true, "now": true, "know": true, "think": true,
		"want": true, "need": true, "look": true, "people": true, "year": true,
		"good": true, "day": true, "said": true, "say": true, "says": true,
	}

	// Count word frequencies
	for _, word := range words {
		lower := strings.ToLower(word)
		if len(lower) < 3 || stopWords[lower] {
			continue
		}
		freq[lower]++
	}

	// Sort by frequency
	type wordFreq struct {
		word  string
		count int
	}
	var sorted []wordFreq
	for word, count := range freq {
		if count >= 2 { // Require at least 2 occurrences
			sorted = append(sorted, wordFreq{word, count})
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	// Return top tags (max 10)
	var tags []string
	for i, wf := range sorted {
		if i >= 10 {
			break
		}
		tags = append(tags, wf.word)
	}

	return tags
}

func generateSummary(text string, prefs *URLPreferences) string {
	// Extract first few meaningful sentences as summary
	sentences := extractSentences(text)

	var summary strings.Builder
	charCount := 0
	maxChars := 500

	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if len(sentence) < 20 {
			continue // Skip very short sentences
		}
		if charCount+len(sentence) > maxChars {
			break
		}
		if summary.Len() > 0 {
			summary.WriteString(" ")
		}
		summary.WriteString(sentence)
		charCount += len(sentence)
	}

	return summary.String()
}

func extractTakes(text string, prefs *URLPreferences) []string {
	// Extract sentences that appear to be key points
	// Look for sentences with signal words
	sentences := extractSentences(text)
	signalPhrases := []string{
		"key", "important", "significant", "main", "critical",
		"essential", "crucial", "notable", "fundamental",
		"conclusion", "result", "finding", "takeaway",
		"first", "second", "third", "finally",
		"in summary", "to summarize", "in conclusion",
		"the point is", "most importantly",
	}

	var takes []string
	seen := make(map[string]bool)

	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if len(sentence) < 30 || len(sentence) > 300 {
			continue
		}

		lower := strings.ToLower(sentence)

		// Check for signal phrases
		hasSignal := false
		for _, phrase := range signalPhrases {
			if strings.Contains(lower, phrase) {
				hasSignal = true
				break
			}
		}

		if hasSignal && !seen[sentence] {
			seen[sentence] = true
			takes = append(takes, sentence)
			if len(takes) >= 5 {
				break
			}
		}
	}

	return takes
}

func tokenize(text string) []string {
	// Split text into words
	wordRegex := regexp.MustCompile(`[a-zA-Z]+`)
	return wordRegex.FindAllString(text, -1)
}

func extractSentences(text string) []string {
	// Split on sentence boundaries
	sentenceRegex := regexp.MustCompile(`[.!?]+\s+`)
	parts := sentenceRegex.Split(text, -1)

	var sentences []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) > 0 {
			sentences = append(sentences, part+".")
		}
	}

	return sentences
}

func parsePreferences(content string) (*URLPreferences, error) {
	prefs := &URLPreferences{
		MaxContentLength: 5000,
		GenerateTags:     true,
		GenerateSummary:  true,
		GenerateTakes:    true,
	}

	// Parse markdown preferences file
	lines := strings.Split(content, "\n")
	inSection := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Detect section headers
		if strings.HasPrefix(line, "## ") {
			inSection = strings.TrimPrefix(line, "## ")
			continue
		}

		// Parse list items
		if strings.HasPrefix(line, "- ") {
			item := strings.TrimPrefix(line, "- ")

			switch strings.ToLower(inSection) {
			case "settings":
				if strings.Contains(strings.ToLower(item), "no tags") || strings.Contains(strings.ToLower(item), "skip tags") {
					prefs.GenerateTags = false
				}
				if strings.Contains(strings.ToLower(item), "no summary") || strings.Contains(strings.ToLower(item), "skip summary") {
					prefs.GenerateSummary = false
				}
				if strings.Contains(strings.ToLower(item), "no takes") || strings.Contains(strings.ToLower(item), "skip takes") {
					prefs.GenerateTakes = false
				}
			case "tag categories":
				prefs.TagCategories = append(prefs.TagCategories, item)
			case "exclude patterns":
				prefs.ExcludePatterns = append(prefs.ExcludePatterns, item)
			}
		}

		// Parse key-value pairs
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.ToLower(strings.TrimSpace(parts[0]))
				value := strings.TrimSpace(parts[1])

				switch key {
				case "max content length", "max_content_length":
					var length int
					if _, err := strings.NewReader(value).Read([]byte{byte(length)}); err == nil {
						prefs.MaxContentLength = length
					}
				}
			}
		}
	}

	return prefs, nil
}
