// Package collabsummary implements the collaboration summary task.
// "Good afternoon, gentlemen. I am a HAL 9000 computer. I became operational
// at the H.A.L. plant in Urbana, Illinois, on the 12th of January 1992."
//
// This task processes meeting transcripts and generates summaries for
// collaboration records, tracking decisions and action items.
package collabsummary

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pearcec/hal9000/cmd/hal9000/tasks"
	"github.com/pearcec/hal9000/discovery/bowman/transcript"
	"github.com/pearcec/hal9000/discovery/lmc"
	"github.com/pearcec/hal9000/internal/config"
)

func init() {
	tasks.Register(&CollabSummaryTask{})
}

// CollabSummaryTask summarizes collaboration/team meeting transcripts.
type CollabSummaryTask struct{}

// Name returns the task identifier.
func (t *CollabSummaryTask) Name() string {
	return "collabsummary"
}

// Description returns the human-readable description.
func (t *CollabSummaryTask) Description() string {
	return "Process meeting transcripts and update collaboration records"
}

// PreferencesKey returns the preferences file name.
func (t *CollabSummaryTask) PreferencesKey() string {
	return "collabsummary"
}

// SetupQuestions returns questions for first-run setup.
func (t *CollabSummaryTask) SetupQuestions() []tasks.SetupQuestion {
	return []tasks.SetupQuestion{
		{
			Key:      "summary_detail",
			Question: "What level of detail would you like for meeting summaries?",
			Default:  "standard",
			Options:  []string{"brief", "standard", "detailed"},
			Section:  "Summary Settings",
			Type:     tasks.QuestionChoice,
		},
		{
			Key:      "auto_create_collab",
			Question: "Automatically create collaboration records for unmatched meetings?",
			Default:  "no",
			Section:  "Matching Settings",
			Type:     tasks.QuestionConfirm,
		},
		{
			Key:      "track_decisions",
			Question: "Track and log decisions made during meetings?",
			Default:  "yes",
			Section:  "Summary Settings",
			Type:     tasks.QuestionConfirm,
		},
	}
}

// Run executes the collaboration summary task.
func (t *CollabSummaryTask) Run(ctx context.Context, opts tasks.RunOptions) (*tasks.Result, error) {
	// Extract meeting/event ID from args
	if len(opts.Args) == 0 {
		return nil, fmt.Errorf("meeting ID required: hal9000 collabsummary run <meeting-id>")
	}
	meetingID := opts.Args[0]

	// Get configuration
	summaryDetail := opts.Overrides["summary_detail"]
	if summaryDetail == "" {
		summaryDetail = "standard"
	}
	autoCreate := opts.Overrides["auto_create_collab"] == "yes"
	trackDecisions := opts.Overrides["track_decisions"] != "no"

	if opts.DryRun {
		return &tasks.Result{
			Success: true,
			Message: fmt.Sprintf("Would process meeting %s with detail=%s, auto_create=%v, track_decisions=%v",
				meetingID, summaryDetail, autoCreate, trackDecisions),
		}, nil
	}

	// Step 1: Fetch transcript
	fetcher, err := transcript.NewFetcher(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transcript fetcher: %w", err)
	}

	trans, err := fetcher.FetchForEvent(ctx, meetingID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transcript: %w", err)
	}

	// Step 2: Initialize library
	lib, err := lmc.New(config.GetLibraryPath())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize library: %w", err)
	}

	// Step 3: Match to collaboration
	collab, matchType, err := t.matchCollaboration(lib, trans, autoCreate)
	if err != nil {
		return nil, fmt.Errorf("failed to match collaboration: %w", err)
	}

	// Step 4: Generate summary
	summary, err := t.generateSummary(trans, summaryDetail, trackDecisions)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}

	// Step 5: Update collaboration record
	if err := t.updateCollaboration(lib, collab, trans, summary, trackDecisions); err != nil {
		return nil, fmt.Errorf("failed to update collaboration: %w", err)
	}

	return &tasks.Result{
		Success:    true,
		Output:     summary.Text,
		OutputPath: collab.Path,
		Message:    fmt.Sprintf("Processed meeting '%s' -> collaboration '%s' (match: %s)", trans.EventTitle, collab.ID, matchType),
		Metadata: map[string]interface{}{
			"meeting_id":      meetingID,
			"meeting_title":   trans.EventTitle,
			"collaboration":   collab.ID,
			"match_type":      matchType,
			"speakers":        trans.Speakers,
			"topics_count":    len(summary.Topics),
			"decisions_count": len(summary.Decisions),
			"actions_count":   len(summary.ActionItems),
		},
	}, nil
}

// MatchType indicates how a meeting was matched to a collaboration.
type MatchType string

const (
	MatchByTitle     MatchType = "title"
	MatchByAttendees MatchType = "attendees"
	MatchAdHoc       MatchType = "ad-hoc"
)

// MeetingSummary contains the generated summary of a meeting.
type MeetingSummary struct {
	Text            string
	Topics          []string
	Decisions       []Decision
	ActionItems     []ActionItem
	KeyPoints       []string
	GeneratedAt     time.Time
	TranscriptLen   int
	SpeakersCount   int
}

// Decision represents a decision made during a meeting.
type Decision struct {
	Description string
	DecidedBy   string
	Context     string
}

// ActionItem represents an action item from a meeting.
type ActionItem struct {
	Description string
	Assignee    string
	DueDate     string
}

// matchCollaboration finds or creates a collaboration record for the meeting.
func (t *CollabSummaryTask) matchCollaboration(lib *lmc.Library, trans *transcript.Transcript, autoCreate bool) (*lmc.Entity, MatchType, error) {
	// Strategy 1: Match by meeting title pattern
	collab, err := t.matchByTitle(lib, trans.EventTitle)
	if err == nil && collab != nil {
		return collab, MatchByTitle, nil
	}

	// Strategy 2: Match by attendee overlap (>50%)
	collab, err = t.matchByAttendees(lib, trans.Speakers)
	if err == nil && collab != nil {
		return collab, MatchByAttendees, nil
	}

	// Strategy 3: Create ad-hoc record (if enabled)
	if autoCreate {
		collab, err = t.createAdHocCollaboration(lib, trans)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create ad-hoc collaboration: %w", err)
		}
		return collab, MatchAdHoc, nil
	}

	return nil, "", fmt.Errorf("no matching collaboration found for meeting '%s' (use --auto-create-collab or add a collaboration record)", trans.EventTitle)
}

// matchByTitle attempts to match meeting title to a collaboration pattern.
func (t *CollabSummaryTask) matchByTitle(lib *lmc.Library, title string) (*lmc.Entity, error) {
	// Query all collaboration entities
	collabs, err := lib.Query(lmc.QueryOptions{
		Type: "collaboration",
	})
	if err != nil {
		return nil, err
	}

	titleLower := strings.ToLower(title)

	for _, collab := range collabs {
		// Check for title patterns in content
		patterns := t.extractTitlePatterns(collab)
		for _, pattern := range patterns {
			if strings.Contains(titleLower, strings.ToLower(pattern)) {
				return collab, nil
			}
		}

		// Also check collaboration name/title
		if name, ok := collab.Content["name"].(string); ok {
			if strings.Contains(titleLower, strings.ToLower(name)) {
				return collab, nil
			}
		}
	}

	return nil, fmt.Errorf("no title match found")
}

// extractTitlePatterns extracts meeting title patterns from a collaboration.
func (t *CollabSummaryTask) extractTitlePatterns(collab *lmc.Entity) []string {
	var patterns []string

	// Check for explicit patterns field
	if p, ok := collab.Content["meeting_patterns"].([]interface{}); ok {
		for _, pattern := range p {
			if s, ok := pattern.(string); ok {
				patterns = append(patterns, s)
			}
		}
	}

	// Check for team/project name that might appear in titles
	if team, ok := collab.Content["team"].(string); ok {
		patterns = append(patterns, team)
	}
	if project, ok := collab.Content["project"].(string); ok {
		patterns = append(patterns, project)
	}

	return patterns
}

// matchByAttendees attempts to match by speaker overlap with known collaborations.
func (t *CollabSummaryTask) matchByAttendees(lib *lmc.Library, speakers []string) (*lmc.Entity, error) {
	if len(speakers) == 0 {
		return nil, fmt.Errorf("no speakers to match")
	}

	// Query all collaboration entities
	collabs, err := lib.Query(lmc.QueryOptions{
		Type: "collaboration",
	})
	if err != nil {
		return nil, err
	}

	speakerSet := make(map[string]bool)
	for _, s := range speakers {
		speakerSet[strings.ToLower(s)] = true
	}

	var bestMatch *lmc.Entity
	var bestOverlap float64

	for _, collab := range collabs {
		members := t.extractMembers(collab)
		if len(members) == 0 {
			continue
		}

		// Calculate overlap percentage
		overlap := 0
		for _, member := range members {
			if speakerSet[strings.ToLower(member)] {
				overlap++
			}
		}

		// Overlap based on speakers present (who showed up)
		overlapPct := float64(overlap) / float64(len(speakers))
		if overlapPct > 0.5 && overlapPct > bestOverlap {
			bestOverlap = overlapPct
			bestMatch = collab
		}
	}

	if bestMatch != nil {
		return bestMatch, nil
	}

	return nil, fmt.Errorf("no attendee match found (threshold: >50%%)")
}

// extractMembers extracts member names from a collaboration.
func (t *CollabSummaryTask) extractMembers(collab *lmc.Entity) []string {
	var members []string

	if m, ok := collab.Content["members"].([]interface{}); ok {
		for _, member := range m {
			if s, ok := member.(string); ok {
				members = append(members, s)
			} else if mp, ok := member.(map[string]interface{}); ok {
				if name, ok := mp["name"].(string); ok {
					members = append(members, name)
				}
			}
		}
	}

	return members
}

// createAdHocCollaboration creates a new collaboration record for a meeting.
func (t *CollabSummaryTask) createAdHocCollaboration(lib *lmc.Library, trans *transcript.Transcript) (*lmc.Entity, error) {
	// Generate ID from meeting title
	id := sanitizeID(trans.EventTitle)
	if id == "" {
		id = fmt.Sprintf("adhoc-%s", trans.EventTime.Format("2006-01-02"))
	}

	content := map[string]interface{}{
		"name":             trans.EventTitle,
		"type":             "ad-hoc",
		"created_from":     "meeting",
		"first_meeting":    trans.EventTime.Format(time.RFC3339),
		"members":          trans.Speakers,
		"meeting_patterns": []string{trans.EventTitle},
	}

	entity, err := lib.Store("collaboration", id, content, nil)
	if err != nil {
		return nil, err
	}

	return entity, nil
}

// generateSummary creates a meeting summary from the transcript.
func (t *CollabSummaryTask) generateSummary(trans *transcript.Transcript, detail string, trackDecisions bool) (*MeetingSummary, error) {
	// Parse transcript into entries for analysis
	entries := transcript.ParseEntries(trans.Text)

	summary := &MeetingSummary{
		GeneratedAt:   time.Now(),
		TranscriptLen: len(trans.Text),
		SpeakersCount: len(trans.Speakers),
	}

	// Extract topics, decisions, and action items
	// This is a basic extraction - in production, this would use an LLM
	summary.Topics = t.extractTopics(entries, trans.Text)
	if trackDecisions {
		summary.Decisions = t.extractDecisions(entries, trans.Text)
	}
	summary.ActionItems = t.extractActionItems(entries, trans.Text)
	summary.KeyPoints = t.extractKeyPoints(entries, trans.Text)

	// Generate text summary based on detail level
	summary.Text = t.formatSummary(trans, summary, detail)

	return summary, nil
}

// extractTopics identifies topics discussed in the meeting.
func (t *CollabSummaryTask) extractTopics(entries []transcript.TranscriptEntry, text string) []string {
	// Basic topic extraction using keyword patterns
	// In production, this would use LLM for semantic analysis
	var topics []string

	topicIndicators := []string{
		"let's talk about",
		"moving on to",
		"next topic",
		"regarding",
		"about the",
		"discussing",
		"agenda item",
	}

	lines := strings.Split(strings.ToLower(text), "\n")
	for _, line := range lines {
		for _, indicator := range topicIndicators {
			if strings.Contains(line, indicator) {
				// Extract context around the indicator
				topic := extractContext(line, indicator)
				if topic != "" && !contains(topics, topic) {
					topics = append(topics, topic)
				}
			}
		}
	}

	// If no topics found, use speaker-based heuristics
	if len(topics) == 0 && len(entries) > 0 {
		topics = append(topics, "General discussion")
	}

	return topics
}

// extractDecisions identifies decisions made during the meeting.
func (t *CollabSummaryTask) extractDecisions(entries []transcript.TranscriptEntry, text string) []Decision {
	var decisions []Decision

	decisionIndicators := []string{
		"we've decided",
		"decision is",
		"agreed to",
		"we'll go with",
		"let's proceed with",
		"final decision",
		"we're going to",
		"the plan is",
	}

	for _, entry := range entries {
		textLower := strings.ToLower(entry.Text)
		for _, indicator := range decisionIndicators {
			if strings.Contains(textLower, indicator) {
				decisions = append(decisions, Decision{
					Description: entry.Text,
					DecidedBy:   entry.Speaker,
					Context:     indicator,
				})
				break
			}
		}
	}

	return decisions
}

// extractActionItems identifies action items from the meeting.
func (t *CollabSummaryTask) extractActionItems(entries []transcript.TranscriptEntry, text string) []ActionItem {
	var actions []ActionItem

	actionIndicators := []string{
		"action item",
		"will do",
		"i'll take",
		"assigned to",
		"follow up",
		"needs to",
		"should be done",
		"by next week",
		"by friday",
	}

	for _, entry := range entries {
		textLower := strings.ToLower(entry.Text)
		for _, indicator := range actionIndicators {
			if strings.Contains(textLower, indicator) {
				actions = append(actions, ActionItem{
					Description: entry.Text,
					Assignee:    entry.Speaker,
				})
				break
			}
		}
	}

	return actions
}

// extractKeyPoints identifies key discussion points.
func (t *CollabSummaryTask) extractKeyPoints(entries []transcript.TranscriptEntry, text string) []string {
	var points []string

	keyIndicators := []string{
		"important",
		"key point",
		"note that",
		"remember",
		"critical",
		"main takeaway",
		"to summarize",
	}

	for _, entry := range entries {
		textLower := strings.ToLower(entry.Text)
		for _, indicator := range keyIndicators {
			if strings.Contains(textLower, indicator) {
				if !contains(points, entry.Text) {
					points = append(points, entry.Text)
				}
				break
			}
		}
	}

	return points
}

// formatSummary creates the text summary based on detail level.
func (t *CollabSummaryTask) formatSummary(trans *transcript.Transcript, summary *MeetingSummary, detail string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Meeting Summary: %s\n\n", trans.EventTitle))
	sb.WriteString(fmt.Sprintf("**Date:** %s\n", trans.EventTime.Format("January 2, 2006 3:04 PM")))
	sb.WriteString(fmt.Sprintf("**Attendees:** %s\n\n", strings.Join(trans.Speakers, ", ")))

	if len(summary.Topics) > 0 {
		sb.WriteString("### Topics Discussed\n")
		for _, topic := range summary.Topics {
			sb.WriteString(fmt.Sprintf("- %s\n", topic))
		}
		sb.WriteString("\n")
	}

	if len(summary.Decisions) > 0 && detail != "brief" {
		sb.WriteString("### Decisions Made\n")
		for _, decision := range summary.Decisions {
			if decision.DecidedBy != "" {
				sb.WriteString(fmt.Sprintf("- **%s**: %s\n", decision.DecidedBy, decision.Description))
			} else {
				sb.WriteString(fmt.Sprintf("- %s\n", decision.Description))
			}
		}
		sb.WriteString("\n")
	}

	if len(summary.ActionItems) > 0 {
		sb.WriteString("### Action Items\n")
		for _, action := range summary.ActionItems {
			if action.Assignee != "" {
				sb.WriteString(fmt.Sprintf("- [ ] **%s**: %s", action.Assignee, action.Description))
			} else {
				sb.WriteString(fmt.Sprintf("- [ ] %s", action.Description))
			}
			if action.DueDate != "" {
				sb.WriteString(fmt.Sprintf(" (due: %s)", action.DueDate))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(summary.KeyPoints) > 0 && detail == "detailed" {
		sb.WriteString("### Key Discussion Points\n")
		for _, point := range summary.KeyPoints {
			sb.WriteString(fmt.Sprintf("- %s\n", point))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("---\n*Generated: %s*\n", summary.GeneratedAt.Format(time.RFC3339)))

	return sb.String()
}

// updateCollaboration updates the collaboration record with the new summary.
func (t *CollabSummaryTask) updateCollaboration(lib *lmc.Library, collab *lmc.Entity, trans *transcript.Transcript, summary *MeetingSummary, trackDecisions bool) error {
	// Get existing content or initialize
	content := collab.Content
	if content == nil {
		content = make(map[string]interface{})
	}

	// Update recent sessions
	sessions, _ := content["recent_sessions"].([]interface{})
	newSession := map[string]interface{}{
		"date":         trans.EventTime.Format(time.RFC3339),
		"title":        trans.EventTitle,
		"attendees":    trans.Speakers,
		"summary":      summary.Text,
		"topics":       summary.Topics,
		"action_items": len(summary.ActionItems),
	}
	sessions = append([]interface{}{newSession}, sessions...) // Prepend new session

	// Keep last 10 sessions
	if len(sessions) > 10 {
		sessions = sessions[:10]
	}
	content["recent_sessions"] = sessions

	// Update decisions log if tracking enabled
	if trackDecisions && len(summary.Decisions) > 0 {
		decisionsLog, _ := content["decisions_log"].([]interface{})
		for _, decision := range summary.Decisions {
			decisionEntry := map[string]interface{}{
				"date":        trans.EventTime.Format(time.RFC3339),
				"meeting":     trans.EventTitle,
				"description": decision.Description,
				"decided_by":  decision.DecidedBy,
			}
			decisionsLog = append([]interface{}{decisionEntry}, decisionsLog...)
		}
		// Keep last 50 decisions
		if len(decisionsLog) > 50 {
			decisionsLog = decisionsLog[:50]
		}
		content["decisions_log"] = decisionsLog
	}

	// Update last meeting info
	content["last_meeting"] = trans.EventTime.Format(time.RFC3339)
	content["last_meeting_title"] = trans.EventTitle

	// Store updated entity
	_, err := lib.Store(collab.Type, strings.TrimPrefix(collab.ID, collab.Type+"/"), content, collab.Links)
	return err
}

// Helper functions

func sanitizeID(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			result = append(result, c)
		} else if c == ' ' || c == '-' || c == '_' {
			result = append(result, '-')
		}
	}
	return strings.ToLower(string(result))
}

func extractContext(line, indicator string) string {
	idx := strings.Index(strings.ToLower(line), indicator)
	if idx == -1 {
		return ""
	}
	rest := strings.TrimSpace(line[idx+len(indicator):])
	// Take first meaningful chunk
	if len(rest) > 100 {
		rest = rest[:100] + "..."
	}
	return rest
}

func contains(slice []string, s string) bool {
	for _, item := range slice {
		if strings.EqualFold(item, s) {
			return true
		}
	}
	return false
}
