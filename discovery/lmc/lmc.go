// Package lmc implements HAL 9000's Logic Memory Center.
// "I know I've made some very poor decisions recently, but I can give you
// my complete assurance that my work will be back to normal."
//
// The LMC is a document-based knowledge graph where:
// - Folders = Entity types (people, agendas, calendar, jira, etc.)
// - Files = Nodes (individual documents)
// - References = Edges (extracted from document content)
//
// Named after HAL's Logic Memory Center - the modules Dave Bowman
// disconnects while HAL sings "Daisy Bell."
package lmc

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Library manages the document-based knowledge graph.
type Library struct {
	BasePath string
	index    *EdgeIndex
	mu       sync.RWMutex
}

// Entity represents a document/node in the library.
type Entity struct {
	ID       string                 `json:"id"`        // Unique identifier (type/filename)
	Type     string                 `json:"type"`      // Entity type (folder name)
	Path     string                 `json:"path"`      // Full file path
	Content  map[string]interface{} `json:"content"`   // Document content
	Links    []Edge                 `json:"links"`     // Outgoing edges
	Created  time.Time              `json:"created"`
	Modified time.Time              `json:"modified"`
}

// Edge represents a relationship between entities.
type Edge struct {
	From   string `json:"from"`   // Source entity ID
	To     string `json:"to"`     // Target entity ID
	Type   string `json:"type"`   // Relationship type
	Label  string `json:"label,omitempty"`
}

// EdgeIndex maintains an in-memory index of edges for fast lookup.
type EdgeIndex struct {
	outgoing map[string][]Edge // entityID -> outgoing edges
	incoming map[string][]Edge // entityID -> incoming edges
	byType   map[string][]Edge // edgeType -> edges
	mu       sync.RWMutex
}

// QueryOptions specifies search parameters.
type QueryOptions struct {
	Type      string    // Filter by entity type
	Since     time.Time // Modified since
	Before    time.Time // Modified before
	Contains  string    // Text search in content
	LinkedTo  string    // Has edge to this entity
	LinkedFrom string   // Has edge from this entity
	Limit     int       // Max results
}

// New creates a new Library instance.
func New(basePath string) (*Library, error) {
	path := expandPath(basePath)

	// Ensure base directory exists
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create library path: %v", err)
	}

	lib := &Library{
		BasePath: path,
		index:    newEdgeIndex(),
	}

	// Build initial index
	if err := lib.rebuildIndex(); err != nil {
		log.Printf("[lmc] Warning: failed to build index: %v", err)
	}

	log.Printf("[lmc] Initialized at %s", path)
	return lib, nil
}

func newEdgeIndex() *EdgeIndex {
	return &EdgeIndex{
		outgoing: make(map[string][]Edge),
		incoming: make(map[string][]Edge),
		byType:   make(map[string][]Edge),
	}
}

// Store saves an entity to the library.
func (l *Library) Store(entityType string, id string, content map[string]interface{}, links []Edge) (*Entity, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Ensure type directory exists
	typePath := filepath.Join(l.BasePath, entityType)
	if err := os.MkdirAll(typePath, 0755); err != nil {
		return nil, err
	}

	// Build entity
	entityID := fmt.Sprintf("%s/%s", entityType, id)
	filename := sanitizeFilename(id) + ".json"
	fullPath := filepath.Join(typePath, filename)

	now := time.Now()
	entity := &Entity{
		ID:       entityID,
		Type:     entityType,
		Path:     fullPath,
		Content:  content,
		Links:    links,
		Created:  now,
		Modified: now,
	}

	// Check if updating existing
	if info, err := os.Stat(fullPath); err == nil {
		entity.Created = info.ModTime() // Preserve original creation time
	}

	// Serialize
	doc := map[string]interface{}{
		"_meta": map[string]interface{}{
			"id":       entityID,
			"type":     entityType,
			"created":  entity.Created.Format(time.RFC3339),
			"modified": entity.Modified.Format(time.RFC3339),
		},
		"content": content,
		"links":   links,
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return nil, err
	}

	// Update index
	l.indexEntity(entity)

	log.Printf("[lmc] Stored entity: %s", entityID)
	return entity, nil
}

// Get retrieves an entity by ID.
func (l *Library) Get(entityID string) (*Entity, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	parts := strings.SplitN(entityID, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid entity ID: %s", entityID)
	}

	entityType, id := parts[0], parts[1]
	filename := sanitizeFilename(id) + ".json"
	fullPath := filepath.Join(l.BasePath, entityType, filename)

	return l.loadEntity(fullPath)
}

// Query searches for entities matching the options.
func (l *Library) Query(opts QueryOptions) ([]*Entity, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var results []*Entity
	searchPath := l.BasePath

	// Narrow search to specific type if given
	if opts.Type != "" {
		searchPath = filepath.Join(l.BasePath, opts.Type)
	}

	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		entity, err := l.loadEntity(path)
		if err != nil {
			return nil // Skip invalid files
		}

		// Apply filters
		if !opts.Since.IsZero() && entity.Modified.Before(opts.Since) {
			return nil
		}
		if !opts.Before.IsZero() && entity.Modified.After(opts.Before) {
			return nil
		}
		if opts.Contains != "" && !l.contentContains(entity, opts.Contains) {
			return nil
		}
		if opts.LinkedTo != "" && !l.hasLinkTo(entity.ID, opts.LinkedTo) {
			return nil
		}
		if opts.LinkedFrom != "" && !l.hasLinkFrom(opts.LinkedFrom, entity.ID) {
			return nil
		}

		results = append(results, entity)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort by modified time (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Modified.After(results[j].Modified)
	})

	// Apply limit
	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	return results, nil
}

// GetLinked returns entities linked to/from the given entity.
func (l *Library) GetLinked(entityID string, direction string) ([]*Entity, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var edges []Edge
	switch direction {
	case "out", "outgoing":
		edges = l.index.outgoing[entityID]
	case "in", "incoming":
		edges = l.index.incoming[entityID]
	case "both":
		edges = append(l.index.outgoing[entityID], l.index.incoming[entityID]...)
	default:
		return nil, fmt.Errorf("invalid direction: %s", direction)
	}

	var results []*Entity
	seen := make(map[string]bool)

	for _, edge := range edges {
		targetID := edge.To
		if direction == "in" || direction == "incoming" {
			targetID = edge.From
		}

		if seen[targetID] {
			continue
		}
		seen[targetID] = true

		entity, err := l.Get(targetID)
		if err != nil {
			continue // Skip missing entities
		}
		results = append(results, entity)
	}

	return results, nil
}

// Delete removes an entity from the library.
func (l *Library) Delete(entityID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Build path directly to avoid deadlock (Get() would try to acquire RLock)
	parts := strings.SplitN(entityID, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid entity ID: %s", entityID)
	}

	entityType, id := parts[0], parts[1]
	filename := sanitizeFilename(id) + ".json"
	fullPath := filepath.Join(l.BasePath, entityType, filename)

	entity, err := l.loadEntity(fullPath)
	if err != nil {
		return err
	}

	// Remove from index
	l.deindexEntity(entity)

	// Delete file
	if err := os.Remove(entity.Path); err != nil {
		return err
	}

	log.Printf("[lmc] Deleted entity: %s", entityID)
	return nil
}

// ListTypes returns all entity types in the library.
func (l *Library) ListTypes() ([]string, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var types []string
	entries, err := os.ReadDir(l.BasePath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			types = append(types, entry.Name())
		}
	}

	return types, nil
}

// Internal methods

func (l *Library) loadEntity(path string) (*Entity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	meta, _ := doc["_meta"].(map[string]interface{})
	content, _ := doc["content"].(map[string]interface{})

	entity := &Entity{
		ID:      getString(meta, "id"),
		Type:    getString(meta, "type"),
		Path:    path,
		Content: content,
	}

	// Parse times
	if created, ok := meta["created"].(string); ok {
		entity.Created, _ = time.Parse(time.RFC3339, created)
	}
	if modified, ok := meta["modified"].(string); ok {
		entity.Modified, _ = time.Parse(time.RFC3339, modified)
	}

	// Parse links
	if links, ok := doc["links"].([]interface{}); ok {
		for _, l := range links {
			if link, ok := l.(map[string]interface{}); ok {
				entity.Links = append(entity.Links, Edge{
					From:  entity.ID,
					To:    getString(link, "to"),
					Type:  getString(link, "type"),
					Label: getString(link, "label"),
				})
			}
		}
	}

	return entity, nil
}

func (l *Library) rebuildIndex() error {
	log.Printf("[lmc] Rebuilding edge index...")

	l.index = newEdgeIndex()

	return filepath.Walk(l.BasePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		entity, err := l.loadEntity(path)
		if err != nil {
			return nil
		}

		l.indexEntity(entity)
		return nil
	})
}

func (l *Library) indexEntity(entity *Entity) {
	l.index.mu.Lock()
	defer l.index.mu.Unlock()

	// Remove old edges for this entity
	delete(l.index.outgoing, entity.ID)

	// Add new edges
	for _, link := range entity.Links {
		edge := Edge{
			From:  entity.ID,
			To:    link.To,
			Type:  link.Type,
			Label: link.Label,
		}

		l.index.outgoing[entity.ID] = append(l.index.outgoing[entity.ID], edge)
		l.index.incoming[link.To] = append(l.index.incoming[link.To], edge)
		l.index.byType[link.Type] = append(l.index.byType[link.Type], edge)
	}
}

func (l *Library) deindexEntity(entity *Entity) {
	l.index.mu.Lock()
	defer l.index.mu.Unlock()

	delete(l.index.outgoing, entity.ID)

	// Remove from incoming edges (expensive but rare operation)
	for targetID, edges := range l.index.incoming {
		var filtered []Edge
		for _, e := range edges {
			if e.From != entity.ID {
				filtered = append(filtered, e)
			}
		}
		l.index.incoming[targetID] = filtered
	}
}

func (l *Library) contentContains(entity *Entity, search string) bool {
	data, _ := json.Marshal(entity.Content)
	return strings.Contains(strings.ToLower(string(data)), strings.ToLower(search))
}

func (l *Library) hasLinkTo(fromID, toID string) bool {
	for _, edge := range l.index.outgoing[fromID] {
		if edge.To == toID {
			return true
		}
	}
	return false
}

func (l *Library) hasLinkFrom(fromID, toID string) bool {
	for _, edge := range l.index.incoming[toID] {
		if edge.From == fromID {
			return true
		}
	}
	return false
}

// Helper functions

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
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
