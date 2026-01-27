// Package processor handles data transformations for HAL 9000.
// "This mission is too important for me to allow you to jeopardize it."
//
// Processor transforms data through medallion stages:
// - Raw → Bronze (cleaned, structured)
// - Bronze → Silver (enriched, linked)
package processor

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Stage represents a data processing stage.
type Stage string

const (
	StageRaw    Stage = "raw"
	StageBronze Stage = "bronze"
	StageSilver Stage = "silver"
)

// Document represents a processed document at any stage.
type Document struct {
	Meta    DocumentMeta           `json:"_meta"`
	Content map[string]interface{} `json:"content"`
	Links   []Link                 `json:"links,omitempty"` // Silver stage only
}

// DocumentMeta contains document metadata.
type DocumentMeta struct {
	Source      string    `json:"source"`
	EventID     string    `json:"event_id"`
	Stage       Stage     `json:"stage"`
	ProcessedAt time.Time `json:"processed_at"`
	PreviousID  string    `json:"previous_id,omitempty"` // Link to source document
}

// Link represents a relationship to another entity (Silver stage).
type Link struct {
	Type   string `json:"type"`   // e.g., "mentions", "scheduled_with", "relates_to"
	Target string `json:"target"` // Target entity ID or path
	Label  string `json:"label,omitempty"`
}

// ProcessConfig configures the processor.
type ProcessConfig struct {
	LibraryPath string // Base library path
}

// ToBronze transforms a raw document to bronze stage.
// Bronze: cleaned, structured, normalized.
func ToBronze(config ProcessConfig, source string, rawDoc map[string]interface{}) (*Document, error) {
	log.Printf("[processor][bronze] Processing %s document", source)

	// Extract metadata
	meta, ok := rawDoc["_meta"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing _meta in raw document")
	}

	eventID, _ := meta["event_id"].(string)
	sourceStr, _ := meta["source"].(string)

	// Build bronze document based on source type
	var content map[string]interface{}
	switch source {
	case "calendar", "google-calendar":
		content = bronzeCalendar(rawDoc)
	case "jira":
		content = bronzeJIRA(rawDoc)
	case "slack":
		content = bronzeSlack(rawDoc)
	default:
		// Generic bronze: just clean up the structure
		content = bronzeGeneric(rawDoc)
	}

	doc := &Document{
		Meta: DocumentMeta{
			Source:      sourceStr,
			EventID:     eventID,
			Stage:       StageBronze,
			ProcessedAt: time.Now(),
			PreviousID:  fmt.Sprintf("raw/%s/%s", source, eventID),
		},
		Content: content,
	}

	log.Printf("[processor][bronze] Created bronze document for %s", eventID)
	return doc, nil
}

// ToSilver transforms a bronze document to silver stage.
// Silver: enriched with extracted entities and links.
func ToSilver(config ProcessConfig, bronzeDoc *Document) (*Document, error) {
	log.Printf("[processor][silver] Processing %s document", bronzeDoc.Meta.EventID)

	// Start with bronze content
	content := make(map[string]interface{})
	for k, v := range bronzeDoc.Content {
		content[k] = v
	}

	// Extract links based on source type
	var links []Link
	switch bronzeDoc.Meta.Source {
	case "calendar", "google-calendar":
		links = extractCalendarLinks(bronzeDoc.Content)
	case "jira":
		links = extractJIRALinks(bronzeDoc.Content)
	case "slack":
		links = extractSlackLinks(bronzeDoc.Content)
	}

	// Extract mentions from text fields
	mentions := extractMentions(bronzeDoc.Content)
	for _, mention := range mentions {
		links = append(links, Link{
			Type:   "mentions",
			Target: mention,
		})
	}

	doc := &Document{
		Meta: DocumentMeta{
			Source:      bronzeDoc.Meta.Source,
			EventID:     bronzeDoc.Meta.EventID,
			Stage:       StageSilver,
			ProcessedAt: time.Now(),
			PreviousID:  fmt.Sprintf("bronze/%s/%s", bronzeDoc.Meta.Source, bronzeDoc.Meta.EventID),
		},
		Content: content,
		Links:   links,
	}

	log.Printf("[processor][silver] Created silver document with %d links", len(links))
	return doc, nil
}

// bronzeCalendar transforms raw calendar data to bronze.
func bronzeCalendar(raw map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"title":       raw["summary"],
		"description": cleanText(getString(raw, "description")),
		"location":    raw["location"],
		"start":       normalizeDateTime(raw["start"]),
		"end":         normalizeDateTime(raw["end"]),
		"attendees":   extractAttendees(raw["attendees"]),
		"organizer":   extractPerson(raw["organizer"]),
		"status":      raw["status"],
		"meeting_url": extractMeetingURL(raw),
	}
}

// bronzeJIRA transforms raw JIRA data to bronze.
func bronzeJIRA(raw map[string]interface{}) map[string]interface{} {
	fields, _ := raw["fields"].(map[string]interface{})
	if fields == nil {
		fields = raw
	}

	return map[string]interface{}{
		"key":         raw["key"],
		"title":       fields["summary"],
		"description": cleanText(getString(fields, "description")),
		"status":      extractNestedString(fields, "status", "name"),
		"priority":    extractNestedString(fields, "priority", "name"),
		"assignee":    extractNestedPerson(fields, "assignee"),
		"reporter":    extractNestedPerson(fields, "reporter"),
		"project":     extractNestedString(fields, "project", "key"),
		"issue_type":  extractNestedString(fields, "issuetype", "name"),
		"created":     fields["created"],
		"updated":     fields["updated"],
		"labels":      fields["labels"],
	}
}

// bronzeSlack transforms raw Slack data to bronze.
func bronzeSlack(raw map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"channel":   raw["channel"],
		"user":      raw["user"],
		"text":      cleanText(getString(raw, "text")),
		"timestamp": raw["ts"],
		"thread_id": raw["thread_ts"],
	}
}

// bronzeGeneric does minimal transformation for unknown sources.
func bronzeGeneric(raw map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range raw {
		if k != "_meta" {
			result[k] = v
		}
	}
	return result
}

// extractCalendarLinks extracts links from calendar events.
func extractCalendarLinks(content map[string]interface{}) []Link {
	var links []Link

	// Link to attendees
	if attendees, ok := content["attendees"].([]map[string]interface{}); ok {
		for _, a := range attendees {
			if email, ok := a["email"].(string); ok {
				links = append(links, Link{
					Type:   "scheduled_with",
					Target: fmt.Sprintf("people/%s", email),
					Label:  getString(a, "name"),
				})
			}
		}
	}

	return links
}

// extractJIRALinks extracts links from JIRA issues.
func extractJIRALinks(content map[string]interface{}) []Link {
	var links []Link

	// Link to assignee
	if assignee, ok := content["assignee"].(map[string]interface{}); ok {
		if email, ok := assignee["email"].(string); ok {
			links = append(links, Link{
				Type:   "assigned_to",
				Target: fmt.Sprintf("people/%s", email),
				Label:  getString(assignee, "name"),
			})
		}
	}

	// Link to project
	if project, ok := content["project"].(string); ok {
		links = append(links, Link{
			Type:   "belongs_to",
			Target: fmt.Sprintf("projects/%s", project),
		})
	}

	return links
}

// extractSlackLinks extracts links from Slack messages.
func extractSlackLinks(content map[string]interface{}) []Link {
	var links []Link

	// Link to channel
	if channel, ok := content["channel"].(string); ok {
		links = append(links, Link{
			Type:   "posted_in",
			Target: fmt.Sprintf("channels/%s", channel),
		})
	}

	// Link to user
	if user, ok := content["user"].(string); ok {
		links = append(links, Link{
			Type:   "authored_by",
			Target: fmt.Sprintf("users/%s", user),
		})
	}

	return links
}

// extractMentions finds @mentions and email-like patterns in text fields.
func extractMentions(content map[string]interface{}) []string {
	var mentions []string
	seen := make(map[string]bool)

	// Email regex
	emailRe := regexp.MustCompile(`[\w.+-]+@[\w.-]+\.\w+`)
	// @mention regex (Slack style)
	mentionRe := regexp.MustCompile(`<@([A-Z0-9]+)>`)

	var extractFromValue func(v interface{})
	extractFromValue = func(v interface{}) {
		switch val := v.(type) {
		case string:
			for _, email := range emailRe.FindAllString(val, -1) {
				if !seen[email] {
					mentions = append(mentions, email)
					seen[email] = true
				}
			}
			for _, match := range mentionRe.FindAllStringSubmatch(val, -1) {
				if len(match) > 1 && !seen[match[1]] {
					mentions = append(mentions, match[1])
					seen[match[1]] = true
				}
			}
		case map[string]interface{}:
			for _, v := range val {
				extractFromValue(v)
			}
		case []interface{}:
			for _, v := range val {
				extractFromValue(v)
			}
		}
	}

	extractFromValue(content)
	return mentions
}

// Helper functions

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func cleanText(s string) string {
	// Remove excessive whitespace
	s = strings.TrimSpace(s)
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	return s
}

func normalizeDateTime(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	// Already structured, return as-is
	return v
}

func extractAttendees(v interface{}) []map[string]interface{} {
	attendees, ok := v.([]interface{})
	if !ok {
		return nil
	}

	var result []map[string]interface{}
	for _, a := range attendees {
		if att, ok := a.(map[string]interface{}); ok {
			result = append(result, map[string]interface{}{
				"email": att["email"],
				"name":  att["displayName"],
				"status": func() string {
					if rs, ok := att["responseStatus"].(string); ok {
						return rs
					}
					return ""
				}(),
			})
		}
	}
	return result
}

func extractPerson(v interface{}) map[string]interface{} {
	if person, ok := v.(map[string]interface{}); ok {
		return map[string]interface{}{
			"email": person["email"],
			"name":  person["displayName"],
		}
	}
	return nil
}

func extractNestedString(m map[string]interface{}, key, subkey string) string {
	if nested, ok := m[key].(map[string]interface{}); ok {
		if v, ok := nested[subkey].(string); ok {
			return v
		}
	}
	return ""
}

func extractNestedPerson(m map[string]interface{}, key string) map[string]interface{} {
	if nested, ok := m[key].(map[string]interface{}); ok {
		return map[string]interface{}{
			"email": nested["emailAddress"],
			"name":  nested["displayName"],
		}
	}
	return nil
}

func extractMeetingURL(m map[string]interface{}) string {
	// Check hangout link
	if url, ok := m["hangoutLink"].(string); ok && url != "" {
		return url
	}
	// Check conference data
	if conf, ok := m["conferenceData"].(map[string]interface{}); ok {
		if eps, ok := conf["entryPoints"].([]interface{}); ok {
			for _, ep := range eps {
				if entry, ok := ep.(map[string]interface{}); ok {
					if uri, ok := entry["uri"].(string); ok {
						return uri
					}
				}
			}
		}
	}
	return ""
}

// SaveDocument saves a document to the library.
func SaveDocument(config ProcessConfig, doc *Document) (string, error) {
	libPath := expandPath(config.LibraryPath)
	stagePath := filepath.Join(libPath, string(doc.Meta.Stage), doc.Meta.Source)

	if err := os.MkdirAll(stagePath, 0755); err != nil {
		return "", err
	}

	filename := fmt.Sprintf("%s_%s.json",
		doc.Meta.ProcessedAt.Format("2006-01-02"),
		sanitizeFilename(doc.Meta.EventID))
	fullPath := filepath.Join(stagePath, filename)

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return "", err
	}

	log.Printf("[processor] Saved %s document: %s", doc.Meta.Stage, filename)
	return fullPath, nil
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func sanitizeFilename(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, c)
		} else {
			result = append(result, '_')
		}
	}
	return string(result)
}
