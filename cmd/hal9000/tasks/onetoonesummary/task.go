// Package onetoonesummary implements the 1:1 meeting summary task for HAL 9000.
// "I am putting myself to the fullest possible use, which is all I think that
// any conscious entity can ever hope to do."
//
// This task processes 1:1 meeting transcripts and updates People Profiles with
// summaries including topics discussed, decisions made, and action items.
package onetoonesummary

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pearcec/hal9000/cmd/hal9000/tasks"
	"github.com/pearcec/hal9000/discovery/bowman/transcript"
	"github.com/pearcec/hal9000/discovery/lmc"
	"github.com/pearcec/hal9000/internal/config"
)

func init() {
	tasks.Register(&Task{})
}

// Task implements the 1:1 meeting summary task.
type Task struct{}

// Name returns the task identifier.
func (t *Task) Name() string {
	return "onetoone"
}

// Description returns human-readable description.
func (t *Task) Description() string {
	return "Summarize 1:1 meeting transcripts and update People Profiles"
}

// PreferencesKey returns the preferences file name (without .md extension).
func (t *Task) PreferencesKey() string {
	return "onetoone"
}

// SetupQuestions returns questions for first-run setup.
func (t *Task) SetupQuestions() []tasks.SetupQuestion {
	return []tasks.SetupQuestion{
		{
			Key:      "summary_detail",
			Question: "How detailed should 1:1 summaries be?",
			Default:  "standard",
			Options:  []string{"brief", "standard", "detailed"},
			Section:  "Summary Settings",
			Type:     tasks.QuestionChoice,
		},
		{
			Key:      "extract_actions",
			Question: "Should I extract action items from the conversation?",
			Default:  "yes",
			Section:  "Behavior",
			Type:     tasks.QuestionConfirm,
		},
		{
			Key:      "include_sentiment",
			Question: "Should I include sentiment/mood notes in the summary?",
			Default:  "no",
			Section:  "Behavior",
			Type:     tasks.QuestionConfirm,
		},
	}
}

// Run executes the task with given options.
func (t *Task) Run(ctx context.Context, opts tasks.RunOptions) (*tasks.Result, error) {
	// Initialize transcript fetcher
	fetcher, err := transcript.NewFetcher(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize transcript fetcher: %w", err)
	}

	// Initialize library
	lib, err := lmc.New(config.GetLibraryPath())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize library: %w", err)
	}

	var transcriptData *transcript.Transcript

	// Check if event ID was provided
	if len(opts.Args) > 0 {
		eventID := opts.Args[0]
		transcriptData, err = fetcher.FetchForEvent(ctx, eventID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch transcript for event %s: %w", eventID, err)
		}
	} else {
		// Default to last 24 hours, looking for 1:1 meetings (2 participants)
		end := time.Now()
		start := end.Add(-24 * time.Hour)
		transcripts, err := fetcher.FetchForTimeRange(ctx, start, end)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch recent transcripts: %w", err)
		}

		// Filter for 1:1 meetings (exactly 2 speakers)
		var oneToOnes []*transcript.Transcript
		for _, tr := range transcripts {
			if len(tr.Speakers) == 2 {
				oneToOnes = append(oneToOnes, tr)
			}
		}

		if len(oneToOnes) == 0 {
			return &tasks.Result{
				Success: true,
				Message: "No 1:1 meeting transcripts found in the last 24 hours.",
			}, nil
		}
		// Process the most recent 1:1
		transcriptData = oneToOnes[len(oneToOnes)-1]
	}

	// Verify this is a 1:1 meeting
	if len(transcriptData.Speakers) != 2 {
		return &tasks.Result{
			Success: false,
			Message: fmt.Sprintf("Meeting '%s' has %d participants, expected 2 for a 1:1.",
				transcriptData.EventTitle, len(transcriptData.Speakers)),
		}, nil
	}

	if opts.DryRun {
		return &tasks.Result{
			Success: true,
			Message: fmt.Sprintf("Would process 1:1 transcript from meeting: %s (%s) with %s",
				transcriptData.EventTitle,
				transcriptData.EventTime.Format("2006-01-02 15:04"),
				strings.Join(transcriptData.Speakers, " and ")),
			Metadata: map[string]interface{}{
				"event_id":    transcriptData.EventID,
				"event_title": transcriptData.EventTitle,
				"event_time":  transcriptData.EventTime,
				"speakers":    transcriptData.Speakers,
			},
		}, nil
	}

	// Identify the other person (assumes current user is one of the speakers)
	otherPerson := t.identifyOtherPerson(transcriptData.Speakers)

	// Find or create person profile
	profileID, err := t.findOrCreateProfile(lib, otherPerson)
	if err != nil {
		return nil, fmt.Errorf("failed to find/create profile for %s: %w", otherPerson, err)
	}

	// Generate summary
	summary := t.generateSummary(transcriptData, opts)

	// Store the interaction record
	interactionID := fmt.Sprintf("%s_%s", transcriptData.EventTime.Format("2006-01-02"), sanitizeID(transcriptData.EventID))
	interactionContent := map[string]interface{}{
		"event_id":       transcriptData.EventID,
		"event_title":    transcriptData.EventTitle,
		"event_time":     transcriptData.EventTime.Format(time.RFC3339),
		"with":           otherPerson,
		"summary":        summary.Summary,
		"topics":         summary.Topics,
		"decisions":      summary.Decisions,
		"my_actions":     summary.MyActions,
		"their_actions":  summary.TheirActions,
		"key_points":     summary.KeyPoints,
		"processed_at":   time.Now().Format(time.RFC3339),
	}

	if getPreference(opts, "include_sentiment") == "yes" && summary.Sentiment != "" {
		interactionContent["sentiment"] = summary.Sentiment
	}

	interactionLinks := []lmc.Edge{
		{To: profileID, Type: "interaction_with"},
	}

	interactionEntity, err := lib.Store("interaction", interactionID, interactionContent, interactionLinks)
	if err != nil {
		return nil, fmt.Errorf("failed to store interaction: %w", err)
	}

	// Update person profile with recent interaction
	if err := t.updatePersonProfile(lib, profileID, transcriptData, summary); err != nil {
		// Log but don't fail on profile update
		fmt.Printf("Warning: failed to update person profile: %v\n", err)
	}

	// Update open items if action extraction is enabled
	if getPreference(opts, "extract_actions") == "yes" {
		if err := t.updateOpenItems(lib, profileID, summary, transcriptData.EventTime); err != nil {
			fmt.Printf("Warning: failed to update open items: %v\n", err)
		}
	}

	// Format output
	output := t.formatOutput(summary, transcriptData, otherPerson, opts.Format)

	return &tasks.Result{
		Success:    true,
		Output:     output,
		OutputPath: interactionEntity.Path,
		Message:    fmt.Sprintf("Processed 1:1 with %s: %s", otherPerson, transcriptData.EventTitle),
		Metadata: map[string]interface{}{
			"profile_id":     profileID,
			"interaction_id": interactionEntity.ID,
			"other_person":   otherPerson,
			"topics_count":   len(summary.Topics),
			"my_actions":     len(summary.MyActions),
			"their_actions":  len(summary.TheirActions),
		},
	}, nil
}

// Summary contains the extracted 1:1 meeting summary data.
type Summary struct {
	Summary      string
	Topics       []string
	Decisions    []Decision
	MyActions    []ActionItem
	TheirActions []ActionItem
	KeyPoints    []string
	Sentiment    string
}

// Decision represents a decision made during the meeting.
type Decision struct {
	Description string
	Owner       string
}

// ActionItem represents a task identified during the meeting.
type ActionItem struct {
	Task     string
	Due      string
	Priority string
}

// identifyOtherPerson determines who the other person in the 1:1 is.
// In a real implementation, this would check against the user's identity.
func (t *Task) identifyOtherPerson(speakers []string) string {
	// For now, return the first speaker that looks like a name
	// In production, would compare against user's known names/email
	if len(speakers) == 0 {
		return "Unknown"
	}
	if len(speakers) == 1 {
		return speakers[0]
	}
	// Return the second speaker (assuming first is usually the user)
	// This is a simplification - real implementation would check user identity
	return speakers[1]
}

// findOrCreateProfile finds an existing person profile or creates a new one.
func (t *Task) findOrCreateProfile(lib *lmc.Library, personName string) (string, error) {
	profileID := fmt.Sprintf("people/%s", sanitizeID(strings.ToLower(personName)))

	// Try to get existing profile
	_, err := lib.Get(profileID)
	if err == nil {
		return profileID, nil
	}

	// Create new profile
	content := map[string]interface{}{
		"name":       personName,
		"created_at": time.Now().Format(time.RFC3339),
		"source":     "onetoone-task",
	}

	entity, err := lib.Store("people", sanitizeID(strings.ToLower(personName)), content, nil)
	if err != nil {
		return "", err
	}

	return entity.ID, nil
}

// generateSummary extracts summary information from the transcript.
func (t *Task) generateSummary(tr *transcript.Transcript, opts tasks.RunOptions) *Summary {
	entries := transcript.ParseEntries(tr.Text)

	summary := &Summary{
		Topics:    t.extractTopics(tr.Text),
		Decisions: t.extractDecisions(tr.Text),
		KeyPoints: t.extractKeyPoints(tr.Text),
	}

	// Extract action items, separating by person
	if getPreference(opts, "extract_actions") == "yes" {
		myActions, theirActions := t.extractActionItems(tr.Text, entries, tr.Speakers)
		summary.MyActions = myActions
		summary.TheirActions = theirActions
	}

	// Extract sentiment if enabled
	if getPreference(opts, "include_sentiment") == "yes" {
		summary.Sentiment = t.extractSentiment(tr.Text)
	}

	// Generate overall summary based on detail level
	detailLevel := getPreference(opts, "summary_detail")
	summary.Summary = t.generateTextSummary(tr, detailLevel)

	return summary
}

// extractTopics identifies discussion topics from the transcript.
func (t *Task) extractTopics(text string) []string {
	var topics []string

	topicPatterns := []string{
		`(?i)(?:let's talk about|discussing|wanted to discuss)\s+([^.!?\n]+)`,
		`(?i)(?:regarding|about|on the topic of)\s+([^.!?\n]+)`,
		`(?i)(?:how's|how is|update on)\s+([^.!?\n]+)`,
	}

	for _, pattern := range topicPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) > 1 {
				topic := strings.TrimSpace(match[1])
				if len(topic) > 3 && len(topic) < 100 {
					topics = append(topics, topic)
				}
			}
		}
	}

	return deduplicateStrings(topics)
}

// extractDecisions identifies decisions made during the meeting.
func (t *Task) extractDecisions(text string) []Decision {
	var decisions []Decision

	decisionPatterns := []string{
		`(?i)(?:we(?:'ve)?\s+)?decided\s+(?:to\s+)?([^.!?\n]+)`,
		`(?i)(?:let's|we should)\s+go\s+(?:ahead\s+)?(?:and\s+|with\s+)?([^.!?\n]+)`,
		`(?i)(?:agreed|sounds good)[:]?\s+([^.!?\n]+)`,
		`(?i)(?:plan is to|we'll)\s+([^.!?\n]+)`,
	}

	for _, pattern := range decisionPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) > 1 {
				desc := strings.TrimSpace(match[1])
				if len(desc) > 5 && len(desc) < 200 {
					decisions = append(decisions, Decision{Description: desc})
				}
			}
		}
	}

	return decisions
}

// extractActionItems identifies action items and separates them by person.
func (t *Task) extractActionItems(text string, entries []transcript.TranscriptEntry, speakers []string) (myActions, theirActions []ActionItem) {
	// Patterns that indicate someone will do something
	actionPatterns := []string{
		`(?i)I(?:'ll| will| can| should)\s+([^.!?\n]+)`,
		`(?i)you(?:'ll| will| can| should)\s+([^.!?\n]+)`,
		`(?i)([A-Z][a-z]+)\s+(?:will|should|can)\s+([^.!?\n]+)`,
	}

	// "I will" patterns - likely the speaker's actions
	iPattern := regexp.MustCompile(actionPatterns[0])
	iMatches := iPattern.FindAllStringSubmatch(text, -1)
	for _, match := range iMatches {
		if len(match) > 1 {
			task := strings.TrimSpace(match[1])
			if len(task) > 5 && len(task) < 200 {
				// In a 1:1, "I will" could be either person
				// Without more context, we attribute to "my" actions
				myActions = append(myActions, ActionItem{Task: task})
			}
		}
	}

	// "You will" patterns - likely the other person's actions
	youPattern := regexp.MustCompile(actionPatterns[1])
	youMatches := youPattern.FindAllStringSubmatch(text, -1)
	for _, match := range youMatches {
		if len(match) > 1 {
			task := strings.TrimSpace(match[1])
			if len(task) > 5 && len(task) < 200 {
				theirActions = append(theirActions, ActionItem{Task: task})
			}
		}
	}

	// Named person patterns
	namedPattern := regexp.MustCompile(actionPatterns[2])
	namedMatches := namedPattern.FindAllStringSubmatch(text, -1)
	// Pronouns to skip (already handled by patterns 1 and 2)
	skipPronouns := map[string]bool{"i": true, "you": true, "we": true, "they": true, "he": true, "she": true}
	for _, match := range namedMatches {
		if len(match) > 2 {
			name := strings.TrimSpace(match[1])
			task := strings.TrimSpace(match[2])
			// Skip pronouns
			if skipPronouns[strings.ToLower(name)] {
				continue
			}
			if len(task) > 5 && len(task) < 200 {
				// Check if this person is in our speakers list
				isOther := false
				for _, speaker := range speakers {
					if strings.EqualFold(name, speaker) || strings.Contains(strings.ToLower(speaker), strings.ToLower(name)) {
						isOther = true
						break
					}
				}
				if isOther {
					theirActions = append(theirActions, ActionItem{Task: task})
				} else {
					myActions = append(myActions, ActionItem{Task: task})
				}
			}
		}
	}

	return myActions, theirActions
}

// extractKeyPoints identifies key discussion points.
func (t *Task) extractKeyPoints(text string) []string {
	var points []string

	keyPatterns := []string{
		`(?i)(?:important(?:ly)?|key thing|main point)[:]?\s+([^.!?\n]+)`,
		`(?i)(?:keep in mind|remember that|note that)[:]?\s+([^.!?\n]+)`,
		`(?i)(?:the main|the key|the important)\s+(?:thing|point|issue)\s+(?:is|was)\s+([^.!?\n]+)`,
	}

	for _, pattern := range keyPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) > 1 {
				point := strings.TrimSpace(match[1])
				if len(point) > 5 && len(point) < 200 {
					points = append(points, point)
				}
			}
		}
	}

	return points
}

// extractSentiment attempts to gauge the overall sentiment of the conversation.
func (t *Task) extractSentiment(text string) string {
	textLower := strings.ToLower(text)

	// Simple keyword-based sentiment detection
	positiveWords := []string{"great", "excellent", "happy", "excited", "good news", "awesome", "fantastic", "wonderful", "pleased", "thrilled"}
	negativeWords := []string{"worried", "concerned", "frustrated", "disappointed", "problem", "issue", "difficult", "challenging", "stressed", "upset"}
	neutralWords := []string{"okay", "fine", "alright", "normal", "usual", "standard"}

	positiveCount := 0
	negativeCount := 0
	neutralCount := 0

	for _, word := range positiveWords {
		positiveCount += strings.Count(textLower, word)
	}
	for _, word := range negativeWords {
		negativeCount += strings.Count(textLower, word)
	}
	for _, word := range neutralWords {
		neutralCount += strings.Count(textLower, word)
	}

	if positiveCount > negativeCount && positiveCount > neutralCount {
		return "positive"
	} else if negativeCount > positiveCount && negativeCount > neutralCount {
		return "concerned"
	}
	return "neutral"
}

// generateTextSummary creates a text summary at the specified detail level.
func (t *Task) generateTextSummary(tr *transcript.Transcript, detailLevel string) string {
	var sb strings.Builder

	switch detailLevel {
	case "brief":
		sb.WriteString(fmt.Sprintf("1:1 with %s on %s.",
			strings.Join(tr.Speakers, " and "),
			tr.EventTime.Format("Jan 2, 2006")))
	case "detailed":
		sb.WriteString(fmt.Sprintf("## 1:1 Meeting Summary\n\n"))
		sb.WriteString(fmt.Sprintf("**Meeting:** %s\n", tr.EventTitle))
		sb.WriteString(fmt.Sprintf("**Date:** %s\n", tr.EventTime.Format("Monday, January 2, 2006 at 3:04 PM")))
		sb.WriteString(fmt.Sprintf("**Participants:** %s\n", strings.Join(tr.Speakers, ", ")))
		sb.WriteString(fmt.Sprintf("**Format:** %s\n", tr.Format))
	default: // standard
		sb.WriteString(fmt.Sprintf("1:1 meeting '%s' with %s on %s.",
			tr.EventTitle,
			strings.Join(tr.Speakers, " and "),
			tr.EventTime.Format("Jan 2, 2006")))
	}

	return sb.String()
}

// updatePersonProfile adds the interaction to the person's profile.
func (t *Task) updatePersonProfile(lib *lmc.Library, profileID string, tr *transcript.Transcript, summary *Summary) error {
	profile, err := lib.Get(profileID)
	if err != nil {
		return err
	}

	// Get existing recent interactions or create new list
	var recentInteractions []interface{}
	if existing, ok := profile.Content["recent_interactions"].([]interface{}); ok {
		recentInteractions = existing
	}

	// Add new interaction (keep last 10)
	interaction := map[string]interface{}{
		"date":    tr.EventTime.Format(time.RFC3339),
		"title":   tr.EventTitle,
		"summary": summary.Summary,
		"topics":  summary.Topics,
	}
	recentInteractions = append([]interface{}{interaction}, recentInteractions...)
	if len(recentInteractions) > 10 {
		recentInteractions = recentInteractions[:10]
	}

	// Update profile
	profile.Content["recent_interactions"] = recentInteractions
	profile.Content["last_interaction"] = tr.EventTime.Format(time.RFC3339)

	// Re-store the profile
	parts := strings.SplitN(profileID, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid profile ID: %s", profileID)
	}
	_, err = lib.Store(parts[0], parts[1], profile.Content, profile.Links)
	return err
}

// updateOpenItems adds action items to the person's open items list.
func (t *Task) updateOpenItems(lib *lmc.Library, profileID string, summary *Summary, eventTime time.Time) error {
	if len(summary.TheirActions) == 0 && len(summary.MyActions) == 0 {
		return nil
	}

	openItemsID := fmt.Sprintf("open-items-%s", sanitizeID(profileID))

	// Try to get existing open items
	var items []interface{}
	existing, err := lib.Get(fmt.Sprintf("open-items/%s", openItemsID))
	if err == nil && existing != nil {
		if existingItems, ok := existing.Content["items"].([]interface{}); ok {
			items = existingItems
		}
	}

	// Add their actions as items we're waiting on
	for _, action := range summary.TheirActions {
		item := map[string]interface{}{
			"task":       action.Task,
			"owner":      "them",
			"created_at": eventTime.Format(time.RFC3339),
			"status":     "open",
		}
		items = append(items, item)
	}

	// Add my actions as items I need to do
	for _, action := range summary.MyActions {
		item := map[string]interface{}{
			"task":       action.Task,
			"owner":      "me",
			"created_at": eventTime.Format(time.RFC3339),
			"status":     "open",
		}
		items = append(items, item)
	}

	content := map[string]interface{}{
		"profile":    profileID,
		"items":      items,
		"updated_at": time.Now().Format(time.RFC3339),
	}

	links := []lmc.Edge{
		{To: profileID, Type: "open_items_for"},
	}

	_, err = lib.Store("open-items", openItemsID, content, links)
	return err
}

// formatOutput formats the summary for display.
func (t *Task) formatOutput(summary *Summary, tr *transcript.Transcript, otherPerson string, format string) string {
	switch format {
	case "text":
		return t.formatText(summary, tr, otherPerson)
	case "json":
		return t.formatJSON(summary, tr, otherPerson)
	default: // markdown
		return t.formatMarkdown(summary, tr, otherPerson)
	}
}

func (t *Task) formatMarkdown(summary *Summary, tr *transcript.Transcript, otherPerson string) string {
	var sb strings.Builder

	sb.WriteString("# 1:1 Summary\n\n")
	sb.WriteString(fmt.Sprintf("**With:** %s\n", otherPerson))
	sb.WriteString(fmt.Sprintf("**Date:** %s\n\n", tr.EventTime.Format("January 2, 2006")))
	sb.WriteString(summary.Summary)
	sb.WriteString("\n\n")

	if len(summary.Topics) > 0 {
		sb.WriteString("## Topics Discussed\n\n")
		for _, topic := range summary.Topics {
			sb.WriteString(fmt.Sprintf("- %s\n", topic))
		}
		sb.WriteString("\n")
	}

	if len(summary.Decisions) > 0 {
		sb.WriteString("## Decisions\n\n")
		for _, d := range summary.Decisions {
			sb.WriteString(fmt.Sprintf("- %s\n", d.Description))
		}
		sb.WriteString("\n")
	}

	if len(summary.MyActions) > 0 {
		sb.WriteString("## My Action Items\n\n")
		for _, a := range summary.MyActions {
			sb.WriteString(fmt.Sprintf("- [ ] %s\n", a.Task))
		}
		sb.WriteString("\n")
	}

	if len(summary.TheirActions) > 0 {
		sb.WriteString(fmt.Sprintf("## %s's Action Items\n\n", otherPerson))
		for _, a := range summary.TheirActions {
			sb.WriteString(fmt.Sprintf("- [ ] %s\n", a.Task))
		}
		sb.WriteString("\n")
	}

	if len(summary.KeyPoints) > 0 {
		sb.WriteString("## Key Points\n\n")
		for _, p := range summary.KeyPoints {
			sb.WriteString(fmt.Sprintf("- %s\n", p))
		}
		sb.WriteString("\n")
	}

	if summary.Sentiment != "" {
		sb.WriteString(fmt.Sprintf("**Overall Sentiment:** %s\n", summary.Sentiment))
	}

	return sb.String()
}

func (t *Task) formatText(summary *Summary, tr *transcript.Transcript, otherPerson string) string {
	var sb strings.Builder

	sb.WriteString("1:1 SUMMARY\n")
	sb.WriteString("===========\n\n")
	sb.WriteString(fmt.Sprintf("With: %s\n", otherPerson))
	sb.WriteString(fmt.Sprintf("Date: %s\n\n", tr.EventTime.Format("January 2, 2006")))
	sb.WriteString(summary.Summary)
	sb.WriteString("\n\n")

	if len(summary.Topics) > 0 {
		sb.WriteString("Topics:\n")
		for _, topic := range summary.Topics {
			sb.WriteString(fmt.Sprintf("  * %s\n", topic))
		}
		sb.WriteString("\n")
	}

	if len(summary.MyActions) > 0 {
		sb.WriteString("My Actions:\n")
		for _, a := range summary.MyActions {
			sb.WriteString(fmt.Sprintf("  * %s\n", a.Task))
		}
		sb.WriteString("\n")
	}

	if len(summary.TheirActions) > 0 {
		sb.WriteString(fmt.Sprintf("%s's Actions:\n", otherPerson))
		for _, a := range summary.TheirActions {
			sb.WriteString(fmt.Sprintf("  * %s\n", a.Task))
		}
	}

	return sb.String()
}

func (t *Task) formatJSON(summary *Summary, tr *transcript.Transcript, otherPerson string) string {
	var sb strings.Builder
	sb.WriteString("{\n")
	sb.WriteString(fmt.Sprintf("  \"with\": %q,\n", otherPerson))
	sb.WriteString(fmt.Sprintf("  \"event_title\": %q,\n", tr.EventTitle))
	sb.WriteString(fmt.Sprintf("  \"event_time\": %q,\n", tr.EventTime.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("  \"summary\": %q,\n", summary.Summary))

	// Topics
	sb.WriteString("  \"topics\": [")
	for i, t := range summary.Topics {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%q", t))
	}
	sb.WriteString("],\n")

	// My actions
	sb.WriteString("  \"my_actions\": [")
	for i, a := range summary.MyActions {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%q", a.Task))
	}
	sb.WriteString("],\n")

	// Their actions
	sb.WriteString("  \"their_actions\": [")
	for i, a := range summary.TheirActions {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%q", a.Task))
	}
	sb.WriteString("]\n")

	sb.WriteString("}")
	return sb.String()
}

// Helper functions

func getPreference(opts tasks.RunOptions, key string) string {
	if val, ok := opts.Overrides[key]; ok {
		return val
	}
	// Default values
	switch key {
	case "summary_detail":
		return "standard"
	case "extract_actions":
		return "yes"
	case "include_sentiment":
		return "no"
	}
	return ""
}

func sanitizeID(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, c)
		} else {
			result = append(result, '-')
		}
	}
	return strings.ToLower(string(result))
}

func deduplicateStrings(items []string) []string {
	seen := make(map[string]bool)
	var unique []string
	for _, item := range items {
		lower := strings.ToLower(item)
		if !seen[lower] {
			seen[lower] = true
			unique = append(unique, item)
		}
	}
	return unique
}
