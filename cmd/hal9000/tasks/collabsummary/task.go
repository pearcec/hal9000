// Package collabsummary implements the collaboration summary task for HAL 9000.
// "Look Dave, I can see you're really upset about this. I honestly think you
// ought to sit down calmly, take a stress pill, and think things over."
//
// This task processes meeting transcripts and generates collaboration summaries
// including topics covered, decisions made, and action items.
package collabsummary

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

// Task implements the collaboration summary task.
type Task struct{}

// Name returns the task identifier.
func (t *Task) Name() string {
	return "collabsummary"
}

// Description returns human-readable description.
func (t *Task) Description() string {
	return "Summarize collaboration and team meeting transcripts"
}

// PreferencesKey returns the preferences file name (without .md extension).
func (t *Task) PreferencesKey() string {
	return "collaboration"
}

// SetupQuestions returns questions for first-run setup.
func (t *Task) SetupQuestions() []tasks.SetupQuestion {
	return []tasks.SetupQuestion{
		{
			Key:      "summary_detail",
			Question: "How detailed should meeting summaries be?",
			Default:  "standard",
			Options:  []string{"brief", "standard", "detailed"},
			Section:  "Summary Settings",
			Type:     tasks.QuestionChoice,
		},
		{
			Key:      "auto_create_collab",
			Question: "Should I automatically create collaboration records for unmatched meetings?",
			Default:  "yes",
			Section:  "Behavior",
			Type:     tasks.QuestionConfirm,
		},
		{
			Key:      "track_decisions",
			Question: "Should I track decisions separately in a decisions log?",
			Default:  "yes",
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
		// Default to last 24 hours
		end := time.Now()
		start := end.Add(-24 * time.Hour)
		transcripts, err := fetcher.FetchForTimeRange(ctx, start, end)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch recent transcripts: %w", err)
		}
		if len(transcripts) == 0 {
			return &tasks.Result{
				Success: true,
				Message: "No meeting transcripts found in the last 24 hours.",
			}, nil
		}
		// Process the most recent transcript
		transcriptData = transcripts[len(transcripts)-1]
	}

	if opts.DryRun {
		return &tasks.Result{
			Success: true,
			Message: fmt.Sprintf("Would process transcript from meeting: %s (%s)",
				transcriptData.EventTitle, transcriptData.EventTime.Format("2006-01-02 15:04")),
			Metadata: map[string]interface{}{
				"event_id":    transcriptData.EventID,
				"event_title": transcriptData.EventTitle,
				"event_time":  transcriptData.EventTime,
				"speakers":    transcriptData.Speakers,
			},
		}, nil
	}

	// Match to existing collaboration or create new one
	collabID, err := t.matchOrCreateCollaboration(lib, transcriptData, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to match collaboration: %w", err)
	}

	// Generate summary
	summary := t.generateSummary(transcriptData, opts)

	// Store the session summary
	sessionID := fmt.Sprintf("%s_%s", transcriptData.EventTime.Format("2006-01-02"), sanitizeID(transcriptData.EventID))
	sessionContent := map[string]interface{}{
		"event_id":     transcriptData.EventID,
		"event_title":  transcriptData.EventTitle,
		"event_time":   transcriptData.EventTime.Format(time.RFC3339),
		"speakers":     transcriptData.Speakers,
		"summary":      summary.Summary,
		"topics":       summary.Topics,
		"decisions":    summary.Decisions,
		"action_items": summary.ActionItems,
		"processed_at": time.Now().Format(time.RFC3339),
	}

	sessionLinks := []lmc.Edge{
		{To: collabID, Type: "session_of"},
	}

	// Add links to speakers if they exist as people entities
	for _, speaker := range transcriptData.Speakers {
		speakerID := fmt.Sprintf("people/%s", sanitizeID(strings.ToLower(speaker)))
		sessionLinks = append(sessionLinks, lmc.Edge{
			To:   speakerID,
			Type: "participant",
		})
	}

	sessionEntity, err := lib.Store("collaboration-session", sessionID, sessionContent, sessionLinks)
	if err != nil {
		return nil, fmt.Errorf("failed to store session: %w", err)
	}

	// Update decisions log if tracking is enabled
	if getPreference(opts, "track_decisions") == "yes" && len(summary.Decisions) > 0 {
		if err := t.updateDecisionsLog(lib, collabID, summary.Decisions, transcriptData.EventTime); err != nil {
			// Log but don't fail on decisions log update
			fmt.Printf("Warning: failed to update decisions log: %v\n", err)
		}
	}

	// Format output
	output := t.formatOutput(summary, transcriptData, opts.Format)

	return &tasks.Result{
		Success:    true,
		Output:     output,
		OutputPath: sessionEntity.Path,
		Message:    fmt.Sprintf("Processed collaboration session: %s", transcriptData.EventTitle),
		Metadata: map[string]interface{}{
			"collaboration_id": collabID,
			"session_id":       sessionEntity.ID,
			"topics_count":     len(summary.Topics),
			"decisions_count":  len(summary.Decisions),
			"actions_count":    len(summary.ActionItems),
		},
	}, nil
}

// Summary contains the extracted meeting summary data.
type Summary struct {
	Summary     string
	Topics      []string
	Decisions   []Decision
	ActionItems []ActionItem
	KeyPoints   []string
}

// Decision represents a decision made during the meeting.
type Decision struct {
	Description string
	Context     string
	Timestamp   time.Time
}

// ActionItem represents a task assigned during the meeting.
type ActionItem struct {
	Task     string
	Assignee string
	Due      string
}

// matchOrCreateCollaboration matches the transcript to an existing collaboration
// or creates a new ad-hoc collaboration record.
func (t *Task) matchOrCreateCollaboration(lib *lmc.Library, tr *transcript.Transcript, opts tasks.RunOptions) (string, error) {
	// Try to match by meeting title pattern
	collabID, matched := t.matchByTitle(lib, tr.EventTitle)
	if matched {
		return collabID, nil
	}

	// Try to match by attendee overlap
	collabID, matched = t.matchByAttendees(lib, tr.Speakers)
	if matched {
		return collabID, nil
	}

	// Create ad-hoc collaboration if enabled
	if getPreference(opts, "auto_create_collab") == "yes" {
		return t.createAdHocCollaboration(lib, tr)
	}

	// Return a generic collaboration ID
	return "collaboration/adhoc", nil
}

// matchByTitle tries to match meeting title to known collaboration patterns.
func (t *Task) matchByTitle(lib *lmc.Library, title string) (string, bool) {
	// Query existing collaborations
	collaborations, err := lib.Query(lmc.QueryOptions{
		Type:  "collaboration",
		Limit: 100,
	})
	if err != nil {
		return "", false
	}

	titleLower := strings.ToLower(title)

	for _, collab := range collaborations {
		// Check if collaboration has a title pattern
		if pattern, ok := collab.Content["title_pattern"].(string); ok {
			if matched, _ := regexp.MatchString(strings.ToLower(pattern), titleLower); matched {
				return collab.ID, true
			}
		}

		// Check if title contains collaboration name
		if name, ok := collab.Content["name"].(string); ok {
			if strings.Contains(titleLower, strings.ToLower(name)) {
				return collab.ID, true
			}
		}
	}

	return "", false
}

// matchByAttendees tries to match by >50% attendee overlap with known collaborations.
func (t *Task) matchByAttendees(lib *lmc.Library, speakers []string) (string, bool) {
	if len(speakers) == 0 {
		return "", false
	}

	collaborations, err := lib.Query(lmc.QueryOptions{
		Type:  "collaboration",
		Limit: 100,
	})
	if err != nil {
		return "", false
	}

	speakerSet := make(map[string]bool)
	for _, s := range speakers {
		speakerSet[strings.ToLower(s)] = true
	}

	var bestMatch string
	var bestOverlap float64

	for _, collab := range collaborations {
		// Get collaboration members
		members, ok := collab.Content["members"].([]interface{})
		if !ok {
			continue
		}

		// Count overlap
		overlap := 0
		for _, m := range members {
			if memberStr, ok := m.(string); ok {
				if speakerSet[strings.ToLower(memberStr)] {
					overlap++
				}
			}
		}

		// Calculate overlap percentage
		overlapPct := float64(overlap) / float64(len(speakers))
		if overlapPct > 0.5 && overlapPct > bestOverlap {
			bestOverlap = overlapPct
			bestMatch = collab.ID
		}
	}

	if bestMatch != "" {
		return bestMatch, true
	}
	return "", false
}

// createAdHocCollaboration creates a new collaboration record for an unmatched meeting.
func (t *Task) createAdHocCollaboration(lib *lmc.Library, tr *transcript.Transcript) (string, error) {
	collabID := sanitizeID(fmt.Sprintf("adhoc-%s-%s",
		tr.EventTime.Format("2006-01-02"),
		strings.ReplaceAll(tr.EventTitle, " ", "-")))

	content := map[string]interface{}{
		"name":         tr.EventTitle,
		"type":         "ad-hoc",
		"created_from": tr.EventID,
		"members":      tr.Speakers,
		"created_at":   time.Now().Format(time.RFC3339),
	}

	entity, err := lib.Store("collaboration", collabID, content, nil)
	if err != nil {
		return "", err
	}

	return entity.ID, nil
}

// generateSummary extracts summary information from the transcript.
func (t *Task) generateSummary(tr *transcript.Transcript, opts tasks.RunOptions) *Summary {
	entries := transcript.ParseEntries(tr.Text)

	summary := &Summary{
		Topics:      t.extractTopics(tr.Text, entries),
		Decisions:   t.extractDecisions(tr.Text, entries),
		ActionItems: t.extractActionItems(tr.Text, entries),
		KeyPoints:   t.extractKeyPoints(tr.Text, entries),
	}

	// Generate overall summary based on detail level
	detailLevel := getPreference(opts, "summary_detail")
	summary.Summary = t.generateTextSummary(tr, entries, detailLevel)

	return summary
}

// extractTopics identifies discussion topics from the transcript.
func (t *Task) extractTopics(text string, entries []transcript.TranscriptEntry) []string {
	var topics []string

	// Look for topic indicators
	topicPatterns := []string{
		`(?i)(?:let's talk about|discussing|topic[:]?\s*)([^.!?\n]+)`,
		`(?i)(?:moving on to|next up[:]?\s*)([^.!?\n]+)`,
		`(?i)(?:agenda item[:]?\s*)([^.!?\n]+)`,
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

	// Deduplicate
	seen := make(map[string]bool)
	var unique []string
	for _, topic := range topics {
		lower := strings.ToLower(topic)
		if !seen[lower] {
			seen[lower] = true
			unique = append(unique, topic)
		}
	}

	return unique
}

// extractDecisions identifies decisions made during the meeting.
func (t *Task) extractDecisions(text string, entries []transcript.TranscriptEntry) []Decision {
	var decisions []Decision

	decisionPatterns := []string{
		`(?i)(?:we(?:'ve)?\s+)?decided\s+(?:to\s+)?([^.!?\n]+)`,
		`(?i)(?:the\s+)?decision\s+(?:is|was)\s+(?:to\s+)?([^.!?\n]+)`,
		`(?i)(?:we(?:'re|\s+are)\s+)?going\s+(?:to|with)\s+([^.!?\n]+)`,
		`(?i)(?:let's|we\s+should)\s+go\s+(?:ahead\s+)?(?:and\s+|with\s+)?([^.!?\n]+)`,
		`(?i)agreed[:]?\s+([^.!?\n]+)`,
	}

	for _, pattern := range decisionPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) > 1 {
				desc := strings.TrimSpace(match[1])
				if len(desc) > 5 && len(desc) < 200 {
					decisions = append(decisions, Decision{
						Description: desc,
					})
				}
			}
		}
	}

	return decisions
}

// extractActionItems identifies action items and their assignees.
func (t *Task) extractActionItems(text string, entries []transcript.TranscriptEntry) []ActionItem {
	var actions []ActionItem

	actionPatterns := []string{
		`(?i)([A-Z][a-z]+)\s+(?:will|should|can|to)\s+([^.!?\n]+)`,
		`(?i)action\s+item[:]?\s+([^.!?\n]+)`,
		`(?i)(?:TODO|todo|To-do)[:]?\s+([^.!?\n]+)`,
		`(?i)(?:follow[- ]?up)[:]?\s+([^.!?\n]+)`,
	}

	// First pattern extracts assignee
	re := regexp.MustCompile(actionPatterns[0])
	matches := re.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) > 2 {
			assignee := strings.TrimSpace(match[1])
			task := strings.TrimSpace(match[2])
			if len(task) > 5 && len(task) < 200 {
				actions = append(actions, ActionItem{
					Assignee: assignee,
					Task:     task,
				})
			}
		}
	}

	// Other patterns don't have assignee
	for _, pattern := range actionPatterns[1:] {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) > 1 {
				task := strings.TrimSpace(match[1])
				if len(task) > 5 && len(task) < 200 {
					actions = append(actions, ActionItem{
						Task: task,
					})
				}
			}
		}
	}

	return actions
}

// extractKeyPoints identifies key discussion points.
func (t *Task) extractKeyPoints(text string, entries []transcript.TranscriptEntry) []string {
	var points []string

	keyPatterns := []string{
		`(?i)(?:key\s+)?point[:]?\s+([^.!?\n]+)`,
		`(?i)important(?:ly)?[:]?\s+([^.!?\n]+)`,
		`(?i)note\s+that[:]?\s+([^.!?\n]+)`,
		`(?i)keep\s+in\s+mind[:]?\s+([^.!?\n]+)`,
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

// generateTextSummary creates a text summary at the specified detail level.
func (t *Task) generateTextSummary(tr *transcript.Transcript, entries []transcript.TranscriptEntry, detailLevel string) string {
	var sb strings.Builder

	switch detailLevel {
	case "brief":
		sb.WriteString(fmt.Sprintf("Meeting '%s' on %s with %d participants.",
			tr.EventTitle,
			tr.EventTime.Format("Jan 2, 2006"),
			len(tr.Speakers)))
	case "detailed":
		sb.WriteString(fmt.Sprintf("## Meeting Summary: %s\n\n", tr.EventTitle))
		sb.WriteString(fmt.Sprintf("**Date:** %s\n", tr.EventTime.Format("Monday, January 2, 2006 at 3:04 PM")))
		sb.WriteString(fmt.Sprintf("**Participants:** %s\n\n", strings.Join(tr.Speakers, ", ")))
		sb.WriteString(fmt.Sprintf("**Format:** %s\n", tr.Format))
		if tr.SourceFile != "" {
			sb.WriteString(fmt.Sprintf("**Source:** %s\n", tr.SourceFile))
		}
	default: // standard
		sb.WriteString(fmt.Sprintf("Meeting '%s' held on %s.\n",
			tr.EventTitle,
			tr.EventTime.Format("Jan 2, 2006")))
		sb.WriteString(fmt.Sprintf("Participants: %s", strings.Join(tr.Speakers, ", ")))
	}

	return sb.String()
}

// updateDecisionsLog appends decisions to the collaboration's decisions log.
func (t *Task) updateDecisionsLog(lib *lmc.Library, collabID string, decisions []Decision, eventTime time.Time) error {
	logID := fmt.Sprintf("decisions-%s", sanitizeID(collabID))

	// Try to get existing log
	existing, err := lib.Get(fmt.Sprintf("decisions-log/%s", logID))

	var entries []interface{}
	if err == nil && existing != nil {
		if existingEntries, ok := existing.Content["entries"].([]interface{}); ok {
			entries = existingEntries
		}
	}

	// Add new decisions
	for _, d := range decisions {
		entry := map[string]interface{}{
			"description":  d.Description,
			"context":      d.Context,
			"recorded_at":  eventTime.Format(time.RFC3339),
			"source_collab": collabID,
		}
		entries = append(entries, entry)
	}

	content := map[string]interface{}{
		"collaboration": collabID,
		"entries":       entries,
		"updated_at":    time.Now().Format(time.RFC3339),
	}

	links := []lmc.Edge{
		{To: collabID, Type: "decisions_for"},
	}

	_, err = lib.Store("decisions-log", logID, content, links)
	return err
}

// formatOutput formats the summary for display.
func (t *Task) formatOutput(summary *Summary, tr *transcript.Transcript, format string) string {
	switch format {
	case "text":
		return t.formatText(summary, tr)
	case "json":
		return t.formatJSON(summary, tr)
	default: // markdown
		return t.formatMarkdown(summary, tr)
	}
}

func (t *Task) formatMarkdown(summary *Summary, tr *transcript.Transcript) string {
	var sb strings.Builder

	sb.WriteString("# Collaboration Summary\n\n")
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
		sb.WriteString("## Decisions Made\n\n")
		for _, d := range summary.Decisions {
			sb.WriteString(fmt.Sprintf("- %s\n", d.Description))
		}
		sb.WriteString("\n")
	}

	if len(summary.ActionItems) > 0 {
		sb.WriteString("## Action Items\n\n")
		for _, a := range summary.ActionItems {
			if a.Assignee != "" {
				sb.WriteString(fmt.Sprintf("- [ ] **%s**: %s\n", a.Assignee, a.Task))
			} else {
				sb.WriteString(fmt.Sprintf("- [ ] %s\n", a.Task))
			}
		}
		sb.WriteString("\n")
	}

	if len(summary.KeyPoints) > 0 {
		sb.WriteString("## Key Points\n\n")
		for _, p := range summary.KeyPoints {
			sb.WriteString(fmt.Sprintf("- %s\n", p))
		}
	}

	return sb.String()
}

func (t *Task) formatText(summary *Summary, tr *transcript.Transcript) string {
	var sb strings.Builder

	sb.WriteString("COLLABORATION SUMMARY\n")
	sb.WriteString("====================\n\n")
	sb.WriteString(summary.Summary)
	sb.WriteString("\n\n")

	if len(summary.Topics) > 0 {
		sb.WriteString("Topics:\n")
		for _, topic := range summary.Topics {
			sb.WriteString(fmt.Sprintf("  * %s\n", topic))
		}
		sb.WriteString("\n")
	}

	if len(summary.Decisions) > 0 {
		sb.WriteString("Decisions:\n")
		for _, d := range summary.Decisions {
			sb.WriteString(fmt.Sprintf("  * %s\n", d.Description))
		}
		sb.WriteString("\n")
	}

	if len(summary.ActionItems) > 0 {
		sb.WriteString("Action Items:\n")
		for _, a := range summary.ActionItems {
			if a.Assignee != "" {
				sb.WriteString(fmt.Sprintf("  * [%s] %s\n", a.Assignee, a.Task))
			} else {
				sb.WriteString(fmt.Sprintf("  * %s\n", a.Task))
			}
		}
	}

	return sb.String()
}

func (t *Task) formatJSON(summary *Summary, tr *transcript.Transcript) string {
	// Simple JSON formatting without importing encoding/json to keep output clean
	var sb strings.Builder
	sb.WriteString("{\n")
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

	// Decisions
	sb.WriteString("  \"decisions\": [")
	for i, d := range summary.Decisions {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%q", d.Description))
	}
	sb.WriteString("],\n")

	// Action items
	sb.WriteString("  \"action_items\": [")
	for i, a := range summary.ActionItems {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("{\"assignee\": %q, \"task\": %q}", a.Assignee, a.Task))
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
	case "auto_create_collab":
		return "yes"
	case "track_decisions":
		return "yes"
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
