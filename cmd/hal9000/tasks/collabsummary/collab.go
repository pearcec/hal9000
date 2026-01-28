// Package collabsummary provides a task for summarizing collaboration/team meeting transcripts.
// "I know I've made some very poor decisions recently, but I can give you
// my complete assurance that my work will be back to normal."
//
// This task processes meeting transcripts and updates Collaboration records with
// summaries, decisions, and action items organized by person.
package collabsummary

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pearcec/hal9000/cmd/hal9000/tasks"
	"github.com/pearcec/hal9000/discovery/bowman/transcript"
)

const (
	// Default library path for Collaboration records
	defaultLibraryPath = "~/Documents/Google Drive/Claude"
	collabsSubdir      = "collaborations"
)

// Task implements the collaboration summary task.
type Task struct{}

// init registers the task with the registry.
func init() {
	tasks.Register(&Task{})
}

// Name returns the task identifier.
func (t *Task) Name() string {
	return "collab"
}

// Description returns a human-readable description.
func (t *Task) Description() string {
	return "Summarize collaboration/team meeting transcripts and update Collaboration records"
}

// PreferencesKey returns the preferences file name.
func (t *Task) PreferencesKey() string {
	return "collab"
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
			Question: "Should I create new collaboration records for unmatched meetings?",
			Default:  "yes",
			Section:  "Collaboration Settings",
			Type:     tasks.QuestionConfirm,
		},
		{
			Key:      "track_decisions",
			Question: "Should I track decisions made in meetings?",
			Default:  "yes",
			Section:  "Summary Settings",
			Type:     tasks.QuestionConfirm,
		},
	}
}

// Run executes the task with given options.
func (t *Task) Run(ctx context.Context, opts tasks.RunOptions) (*tasks.Result, error) {
	// Validate we have an event ID
	if len(opts.Args) == 0 {
		return nil, fmt.Errorf("event ID required: hal9000 collab run <event-id>")
	}
	eventID := opts.Args[0]

	// Create transcript fetcher
	fetcher, err := transcript.NewFetcher(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize transcript fetcher: %w", err)
	}

	// Fetch transcript for the event
	tr, err := fetcher.FetchForEvent(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transcript: %w", err)
	}

	// Parse transcript entries
	entries := transcript.ParseEntries(tr.Text)

	// Match or create collaboration
	collab, isNew, err := matchOrCreateCollaboration(tr, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to match collaboration: %w", err)
	}

	// Generate summary
	summary := generateSummary(tr, entries, opts)

	// Update collaboration with new session
	updateCollaboration(collab, tr, summary)

	// Save collaboration
	collabPath, err := saveCollaboration(collab)
	if err != nil {
		return nil, fmt.Errorf("failed to save collaboration: %w", err)
	}

	action := "Updated"
	if isNew {
		action = "Created"
	}

	if opts.DryRun {
		return &tasks.Result{
			Success: true,
			Output:  formatSummary(summary),
			Message: fmt.Sprintf("Would %s collaboration '%s' at %s", strings.ToLower(action), collab.Name, collabPath),
		}, nil
	}

	return &tasks.Result{
		Success:    true,
		Output:     formatSummary(summary),
		OutputPath: collabPath,
		Message:    fmt.Sprintf("%s collaboration '%s'", action, collab.Name),
		Metadata: map[string]interface{}{
			"collaboration": collab.Name,
			"event_id":      eventID,
			"event_title":   tr.EventTitle,
			"collab_path":   collabPath,
			"is_new":        isNew,
			"participants":  len(tr.Speakers),
		},
	}, nil
}

// Collaboration represents a collaboration/team record in the library.
type Collaboration struct {
	Name           string           `json:"name"`
	Description    string           `json:"description,omitempty"`
	Participants   []Participant    `json:"participants"`
	TitlePatterns  []string         `json:"title_patterns,omitempty"`
	RecentSessions []Session        `json:"recent_sessions"`
	DecisionsLog   []Decision       `json:"decisions_log,omitempty"`
	OpenActions    []ActionItem     `json:"open_actions"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

// Participant represents a person involved in a collaboration.
type Participant struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	Role  string `json:"role,omitempty"`
}

// Session represents a recorded meeting session.
type Session struct {
	Date         time.Time    `json:"date"`
	Title        string       `json:"title"`
	Summary      string       `json:"summary"`
	Topics       []string     `json:"topics,omitempty"`
	Decisions    []Decision   `json:"decisions,omitempty"`
	ActionItems  []ActionItem `json:"action_items,omitempty"`
	Participants []string     `json:"participants,omitempty"`
	EventID      string       `json:"event_id,omitempty"`
}

// Decision represents a decision made during a meeting.
type Decision struct {
	Description string    `json:"description"`
	MadeBy      string    `json:"made_by,omitempty"`
	Date        time.Time `json:"date"`
	Context     string    `json:"context,omitempty"`
	Source      string    `json:"source,omitempty"` // meeting title/date
}

// ActionItem represents a tracked action item.
type ActionItem struct {
	Description string    `json:"description"`
	Owner       string    `json:"owner"`
	DueDate     string    `json:"due_date,omitempty"`
	Status      string    `json:"status"` // "open", "completed"
	CreatedAt   time.Time `json:"created_at"`
	Source      string    `json:"source,omitempty"` // meeting title/date
}

// MeetingSummary contains the generated summary for a meeting.
type MeetingSummary struct {
	Topics       []string     `json:"topics"`
	Decisions    []Decision   `json:"decisions"`
	ActionItems  []ActionItem `json:"action_items"`
	Summary      string       `json:"summary"`
	KeyPoints    []string     `json:"key_points,omitempty"`
	Participants []string     `json:"participants"`
}

// matchOrCreateCollaboration finds an existing collaboration or creates a new one.
func matchOrCreateCollaboration(tr *transcript.Transcript, opts tasks.RunOptions) (*Collaboration, bool, error) {
	// Try to find existing collaboration by title pattern
	collabs, err := loadAllCollaborations()
	if err != nil {
		// If we can't load collaborations, we'll create a new one
		collabs = []*Collaboration{}
	}

	// Match by title pattern
	for _, collab := range collabs {
		if matchesTitlePattern(tr.EventTitle, collab.TitlePatterns) {
			return collab, false, nil
		}
	}

	// Match by attendee overlap (>50%)
	for _, collab := range collabs {
		if hasSignificantOverlap(tr.Speakers, collab.Participants) {
			return collab, false, nil
		}
	}

	// Check if auto-create is enabled
	autoCreate := true
	if val, ok := opts.Overrides["auto_create_collab"]; ok {
		autoCreate = val == "yes"
	}

	if !autoCreate && len(collabs) > 0 {
		return nil, false, fmt.Errorf("no matching collaboration found and auto_create_collab is disabled")
	}

	// Create new ad-hoc collaboration
	collab := &Collaboration{
		Name:           deriveCollabName(tr.EventTitle),
		Description:    fmt.Sprintf("Auto-created from meeting: %s", tr.EventTitle),
		Participants:   speakersToParticipants(tr.Speakers),
		TitlePatterns:  []string{extractTitlePattern(tr.EventTitle)},
		RecentSessions: []Session{},
		DecisionsLog:   []Decision{},
		OpenActions:    []ActionItem{},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	return collab, true, nil
}

// matchesTitlePattern checks if a meeting title matches any pattern.
func matchesTitlePattern(title string, patterns []string) bool {
	titleLower := strings.ToLower(title)
	for _, pattern := range patterns {
		patternLower := strings.ToLower(pattern)
		if strings.Contains(titleLower, patternLower) {
			return true
		}
	}
	return false
}

// hasSignificantOverlap checks if >50% of speakers are collaboration participants.
func hasSignificantOverlap(speakers []string, participants []Participant) bool {
	if len(speakers) == 0 || len(participants) == 0 {
		return false
	}

	participantNames := make(map[string]bool)
	for _, p := range participants {
		participantNames[strings.ToLower(p.Name)] = true
	}

	matchCount := 0
	for _, speaker := range speakers {
		if participantNames[strings.ToLower(speaker)] {
			matchCount++
		}
	}

	// >50% overlap
	return float64(matchCount)/float64(len(speakers)) > 0.5
}

// deriveCollabName creates a collaboration name from meeting title.
func deriveCollabName(title string) string {
	// Remove common prefixes/suffixes
	name := title
	prefixes := []string{"Weekly ", "Daily ", "Monthly ", "Bi-weekly "}
	for _, prefix := range prefixes {
		name = strings.TrimPrefix(name, prefix)
	}

	suffixes := []string{" Meeting", " Sync", " Standup", " Check-in"}
	for _, suffix := range suffixes {
		name = strings.TrimSuffix(name, suffix)
	}

	if name == "" {
		name = "Ad-hoc Meeting"
	}

	return name
}

// extractTitlePattern extracts a reusable pattern from a meeting title.
func extractTitlePattern(title string) string {
	// Remove date-like patterns to create a generic pattern
	// For now, just return the title
	return title
}

// speakersToParticipants converts speaker names to participants.
func speakersToParticipants(speakers []string) []Participant {
	participants := make([]Participant, 0, len(speakers))
	for _, speaker := range speakers {
		participants = append(participants, Participant{
			Name: speaker,
		})
	}
	return participants
}

// loadAllCollaborations loads all collaboration records from the library.
func loadAllCollaborations() ([]*Collaboration, error) {
	collabDir := filepath.Join(expandPath(defaultLibraryPath), collabsSubdir)

	entries, err := os.ReadDir(collabDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Collaboration{}, nil
		}
		return nil, err
	}

	var collabs []*Collaboration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(collabDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var collab Collaboration
		if err := json.Unmarshal(data, &collab); err != nil {
			continue
		}

		collabs = append(collabs, &collab)
	}

	return collabs, nil
}

// saveCollaboration saves a collaboration to the library.
func saveCollaboration(collab *Collaboration) (string, error) {
	collab.UpdatedAt = time.Now()

	collabPath := getCollabPath(collab.Name)
	dir := filepath.Dir(collabPath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create collaborations directory: %w", err)
	}

	data, err := json.MarshalIndent(collab, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal collaboration: %w", err)
	}

	if err := os.WriteFile(collabPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write collaboration: %w", err)
	}

	return collabPath, nil
}

// getCollabPath returns the file path for a collaboration.
func getCollabPath(name string) string {
	basePath := expandPath(defaultLibraryPath)
	safeName := sanitizeName(name)
	return filepath.Join(basePath, collabsSubdir, safeName+".json")
}

// generateSummary generates a summary from the transcript.
// TODO: This should use an LLM for real summarization.
func generateSummary(tr *transcript.Transcript, entries []transcript.TranscriptEntry, opts tasks.RunOptions) *MeetingSummary {
	summary := &MeetingSummary{
		Topics:       []string{},
		Decisions:    []Decision{},
		ActionItems:  []ActionItem{},
		KeyPoints:    []string{},
		Participants: tr.Speakers,
	}

	// Extract detail level from preferences/overrides
	detailLevel := "standard"
	if level, ok := opts.Overrides["summary_detail"]; ok {
		detailLevel = level
	}

	// For now, generate a basic summary from transcript metadata
	// Real implementation would call an LLM
	switch detailLevel {
	case "brief":
		summary.Summary = fmt.Sprintf("Team meeting on %s: %s",
			tr.EventTime.Format("Jan 2, 2006"), tr.EventTitle)
	case "detailed":
		summary.Summary = fmt.Sprintf("Detailed team meeting on %s regarding %s. "+
			"Discussion involved %d participants with %d conversation turns.",
			tr.EventTime.Format("Jan 2, 2006 at 3:04 PM"),
			tr.EventTitle,
			len(tr.Speakers),
			len(entries))
	default: // standard
		summary.Summary = fmt.Sprintf("Team meeting on %s: %s with %d participants",
			tr.EventTime.Format("Jan 2, 2006"), tr.EventTitle, len(tr.Speakers))
	}

	// Extract basic topics from entries
	if len(entries) > 0 {
		seenTopics := make(map[string]bool)
		for i, entry := range entries {
			if i >= 5 { // Limit to first 5 entries for basic topic extraction
				break
			}
			// Simple extraction - real implementation would use NLP/LLM
			words := strings.Fields(entry.Text)
			if len(words) > 3 {
				topic := strings.Join(words[:3], " ") + "..."
				if !seenTopics[topic] {
					seenTopics[topic] = true
					summary.Topics = append(summary.Topics, topic)
				}
			}
		}
	}

	return summary
}

// updateCollaboration updates a collaboration with a new session.
func updateCollaboration(collab *Collaboration, tr *transcript.Transcript, summary *MeetingSummary) {
	session := Session{
		Date:         tr.EventTime,
		Title:        tr.EventTitle,
		Summary:      summary.Summary,
		Topics:       summary.Topics,
		Decisions:    summary.Decisions,
		ActionItems:  summary.ActionItems,
		Participants: summary.Participants,
		EventID:      tr.EventID,
	}

	// Prepend new session (most recent first)
	collab.RecentSessions = append([]Session{session}, collab.RecentSessions...)

	// Keep only last 30 sessions
	if len(collab.RecentSessions) > 30 {
		collab.RecentSessions = collab.RecentSessions[:30]
	}

	// Add decisions to decisions log
	for _, decision := range summary.Decisions {
		decision.Source = tr.EventTitle
		decision.Date = tr.EventTime
		collab.DecisionsLog = append(collab.DecisionsLog, decision)
	}

	// Keep decisions log to reasonable size
	if len(collab.DecisionsLog) > 100 {
		collab.DecisionsLog = collab.DecisionsLog[:100]
	}

	// Add new action items to open actions
	for _, item := range summary.ActionItems {
		item.Source = tr.EventTitle
		collab.OpenActions = append(collab.OpenActions, item)
	}

	// Update participants if we have new ones
	existingParticipants := make(map[string]bool)
	for _, p := range collab.Participants {
		existingParticipants[strings.ToLower(p.Name)] = true
	}

	for _, speaker := range tr.Speakers {
		if !existingParticipants[strings.ToLower(speaker)] {
			collab.Participants = append(collab.Participants, Participant{Name: speaker})
			existingParticipants[strings.ToLower(speaker)] = true
		}
	}
}

// formatSummary formats a summary for output.
func formatSummary(summary *MeetingSummary) string {
	var sb strings.Builder

	sb.WriteString("## Meeting Summary\n\n")
	sb.WriteString(summary.Summary)
	sb.WriteString("\n\n")

	if len(summary.Participants) > 0 {
		sb.WriteString("### Participants\n")
		sb.WriteString(strings.Join(summary.Participants, ", "))
		sb.WriteString("\n\n")
	}

	if len(summary.Topics) > 0 {
		sb.WriteString("### Topics Discussed\n")
		for _, topic := range summary.Topics {
			sb.WriteString(fmt.Sprintf("- %s\n", topic))
		}
		sb.WriteString("\n")
	}

	if len(summary.KeyPoints) > 0 {
		sb.WriteString("### Key Points\n")
		for _, point := range summary.KeyPoints {
			sb.WriteString(fmt.Sprintf("- %s\n", point))
		}
		sb.WriteString("\n")
	}

	if len(summary.Decisions) > 0 {
		sb.WriteString("### Decisions Made\n")
		for _, decision := range summary.Decisions {
			madeBy := ""
			if decision.MadeBy != "" {
				madeBy = fmt.Sprintf(" (by %s)", decision.MadeBy)
			}
			sb.WriteString(fmt.Sprintf("- %s%s\n", decision.Description, madeBy))
		}
		sb.WriteString("\n")
	}

	if len(summary.ActionItems) > 0 {
		sb.WriteString("### Action Items\n")
		for _, item := range summary.ActionItems {
			owner := item.Owner
			if owner == "" {
				owner = "TBD"
			}
			sb.WriteString(fmt.Sprintf("- [ ] %s (@%s)\n", item.Description, owner))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// expandPath expands ~ to home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// sanitizeName makes a name safe for use in filenames.
func sanitizeName(name string) string {
	// Replace spaces with underscores and remove problematic characters
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "_")

	var result strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			result.WriteRune(c)
		}
	}
	return result.String()
}
