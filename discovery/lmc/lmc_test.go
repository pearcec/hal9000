package lmc

import (
	"os"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lmc-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	lib, err := New(tmpDir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if lib.BasePath != tmpDir {
		t.Errorf("BasePath = %q, want %q", lib.BasePath, tmpDir)
	}
}

func TestStoreAndGet(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lmc-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	lib, err := New(tmpDir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	content := map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
	}

	links := []Edge{
		{Type: "works_at", To: "companies/acme"},
	}

	// Store
	entity, err := lib.Store("people", "john-doe", content, links)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	if entity.ID != "people/john-doe" {
		t.Errorf("ID = %q, want %q", entity.ID, "people/john-doe")
	}
	if entity.Type != "people" {
		t.Errorf("Type = %q, want %q", entity.Type, "people")
	}

	// Get
	retrieved, err := lib.Get("people/john-doe")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.ID != entity.ID {
		t.Errorf("Retrieved ID = %q, want %q", retrieved.ID, entity.ID)
	}
	if retrieved.Content["name"] != "John Doe" {
		t.Errorf("Retrieved name = %v, want %q", retrieved.Content["name"], "John Doe")
	}
}

func TestDelete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lmc-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	lib, err := New(tmpDir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Store
	_, err = lib.Store("test", "delete-me", map[string]interface{}{"key": "value"}, nil)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Verify exists
	_, err = lib.Get("test/delete-me")
	if err != nil {
		t.Fatal("Entity should exist before delete")
	}

	// Delete
	err = lib.Delete("test/delete-me")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify gone
	_, err = lib.Get("test/delete-me")
	if err == nil {
		t.Error("Entity should not exist after delete")
	}
}

func TestQuery(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lmc-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	lib, err := New(tmpDir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Store multiple entities
	lib.Store("people", "alice", map[string]interface{}{"name": "Alice", "role": "engineer"}, nil)
	lib.Store("people", "bob", map[string]interface{}{"name": "Bob", "role": "manager"}, nil)
	lib.Store("projects", "hal9000", map[string]interface{}{"name": "HAL 9000"}, nil)

	// Query by type
	results, err := lib.Query(QueryOptions{Type: "people"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Query returned %d results, want 2", len(results))
	}

	// Query with text search
	results, err = lib.Query(QueryOptions{Contains: "engineer"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Query returned %d results, want 1", len(results))
	}

	// Query with limit
	results, err = lib.Query(QueryOptions{Limit: 1})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Query returned %d results, want 1", len(results))
	}
}

func TestListTypes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lmc-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	lib, err := New(tmpDir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Store entities of different types
	lib.Store("people", "test1", map[string]interface{}{}, nil)
	lib.Store("projects", "test2", map[string]interface{}{}, nil)
	lib.Store("calendar", "test3", map[string]interface{}{}, nil)

	types, err := lib.ListTypes()
	if err != nil {
		t.Fatalf("ListTypes failed: %v", err)
	}

	if len(types) != 3 {
		t.Errorf("ListTypes returned %d types, want 3", len(types))
	}

	// Check all types present
	typeSet := make(map[string]bool)
	for _, typ := range types {
		typeSet[typ] = true
	}

	for _, expected := range []string{"people", "projects", "calendar"} {
		if !typeSet[expected] {
			t.Errorf("Expected type %q not found", expected)
		}
	}
}

func TestEdgeIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lmc-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	lib, err := New(tmpDir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Store entity with links
	links := []Edge{
		{Type: "works_with", To: "people/bob"},
		{Type: "manages", To: "projects/hal9000"},
	}
	lib.Store("people", "alice", map[string]interface{}{"name": "Alice"}, links)
	lib.Store("people", "bob", map[string]interface{}{"name": "Bob"}, nil)
	lib.Store("projects", "hal9000", map[string]interface{}{"name": "HAL 9000"}, nil)

	// Get linked entities (outgoing)
	related, err := lib.GetLinked("people/alice", "out")
	if err != nil {
		t.Fatalf("GetLinked failed: %v", err)
	}

	if len(related) != 2 {
		t.Errorf("GetLinked returned %d entities, want 2", len(related))
	}
}

func TestQuerySince(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lmc-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	lib, err := New(tmpDir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Store an entity
	lib.Store("test", "recent", map[string]interface{}{"key": "value"}, nil)

	// Query since yesterday - should find it
	yesterday := time.Now().Add(-24 * time.Hour)
	results, err := lib.Query(QueryOptions{Since: yesterday})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Query returned %d results, want 1", len(results))
	}

	// Query since tomorrow - should not find it
	tomorrow := time.Now().Add(24 * time.Hour)
	results, err = lib.Query(QueryOptions{Since: tomorrow})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Query returned %d results, want 0", len(results))
	}
}
