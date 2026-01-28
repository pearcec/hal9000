package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateDirIfNotExists(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "hal9000-init-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	var created []string

	// Test creating a new directory
	testPath := filepath.Join("test", "subdir")
	err = createDirIfNotExists(testPath, &created)
	if err != nil {
		t.Errorf("createDirIfNotExists failed: %v", err)
	}

	if len(created) != 1 {
		t.Errorf("Expected 1 created item, got %d", len(created))
	}

	// Verify directory exists
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Error("Directory was not created")
	}

	// Test idempotency - creating same directory again
	created = []string{}
	err = createDirIfNotExists(testPath, &created)
	if err != nil {
		t.Errorf("createDirIfNotExists failed on second call: %v", err)
	}

	if len(created) != 0 {
		t.Errorf("Expected 0 created items on idempotent call, got %d", len(created))
	}
}

func TestCreateConfigIfNotExists(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "hal9000-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	var created []string

	// Test creating a new config file
	configPath := filepath.Join(tmpDir, "config.yaml")
	err = createConfigIfNotExists(configPath, &created)
	if err != nil {
		t.Errorf("createConfigIfNotExists failed: %v", err)
	}

	if len(created) != 1 {
		t.Errorf("Expected 1 created item, got %d", len(created))
	}

	// Verify file exists and has content
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Errorf("Failed to read config file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Config file is empty")
	}

	// Test idempotency - creating same file again
	created = []string{}
	err = createConfigIfNotExists(configPath, &created)
	if err != nil {
		t.Errorf("createConfigIfNotExists failed on second call: %v", err)
	}

	if len(created) != 0 {
		t.Errorf("Expected 0 created items on idempotent call, got %d", len(created))
	}
}

func TestInitCreatesAllLibraryDirs(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "hal9000-init-full-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	expectedDirs := []string{
		"library/agenda",
		"library/preferences",
		"library/people-profiles",
		"library/collaborations",
		"library/url_library",
		"library/reminders",
		"library/hal-memory",
		"library/calendar",
		"library/schedules",
		"library/logs",
	}

	var created []string

	// Create all library directories
	for _, dir := range expectedDirs {
		err := createDirIfNotExists(dir, &created)
		if err != nil {
			t.Errorf("Failed to create %s: %v", dir, err)
		}
	}

	// Verify all directories exist
	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Directory %s was not created", dir)
		}
	}

	if len(created) != len(expectedDirs) {
		t.Errorf("Expected %d created dirs, got %d", len(expectedDirs), len(created))
	}
}
