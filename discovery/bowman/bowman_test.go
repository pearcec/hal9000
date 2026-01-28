package bowman

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStore(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "bowman-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := StoreConfig{
		LibraryPath: tmpDir,
		Category:    "test-category",
	}

	event := RawEvent{
		Source:    "test-source",
		EventID:   "test-123",
		FetchedAt: time.Date(2026, 1, 27, 12, 0, 0, 0, time.UTC),
		Stage:     "raw",
		Data: map[string]interface{}{
			"title": "Test Event",
			"value": 42,
		},
	}

	// Test Store
	path, err := Store(config, event)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("Expected file to exist at %s", path)
	}

	// Verify file content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read stored file: %v", err)
	}

	// Check metadata is present
	content := string(data)
	if !contains(content, `"source": "test-source"`) {
		t.Error("Expected source in stored data")
	}
	if !contains(content, `"event_id": "test-123"`) {
		t.Error("Expected event_id in stored data")
	}
	if !contains(content, `"stage": "raw"`) {
		t.Error("Expected stage in stored data")
	}
	if !contains(content, `"title": "Test Event"`) {
		t.Error("Expected title in stored data")
	}
}

func TestStoreCreatesDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bowman-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := StoreConfig{
		LibraryPath: tmpDir,
		Category:    "new-category",
	}

	event := RawEvent{
		Source:    "test",
		EventID:   "abc",
		FetchedAt: time.Now(),
		Stage:     "raw",
		Data:      map[string]interface{}{"key": "value"},
	}

	_, err = Store(config, event)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Verify category directory was created
	categoryPath := filepath.Join(tmpDir, "new-category")
	if _, err := os.Stat(categoryPath); os.IsNotExist(err) {
		t.Error("Expected category directory to be created")
	}
}

func TestDelete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bowman-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := StoreConfig{
		LibraryPath: tmpDir,
		Category:    "test-category",
	}

	event := RawEvent{
		Source:    "test",
		EventID:   "delete-me-123",
		FetchedAt: time.Now(),
		Stage:     "raw",
		Data:      map[string]interface{}{"key": "value"},
	}

	// Store first
	path, err := Store(config, event)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("File should exist before delete")
	}

	// Delete
	err = Delete(config, "delete-me-123")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("File should not exist after delete")
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with-dash", "with-dash"},
		{"with_underscore", "with_underscore"},
		{"with spaces", "with_spaces"},
		{"with/slashes", "with_slashes"},
		{"special@chars!", "special_chars_"},
		{"MixedCase123", "MixedCase123"},
	}

	for _, tt := range tests {
		result := sanitizeFilename(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input         string
		expected      string
		checkAbsolute bool // for relative paths, just check it becomes absolute
	}{
		{"~/test", filepath.Join(home, "test"), false},
		{"/absolute/path", "/absolute/path", false},
		{"relative/path", "", true}, // relative paths should become absolute
	}

	for _, tt := range tests {
		result := expandPath(tt.input)
		if tt.checkAbsolute {
			if !filepath.IsAbs(result) {
				t.Errorf("expandPath(%q) = %q, want absolute path", tt.input, result)
			}
		} else if result != tt.expected {
			t.Errorf("expandPath(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
