package tasks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreferencesExistIn(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	task := &mockTask{
		name:           "test-task",
		preferencesKey: "test-task",
	}

	// Should not exist initially
	if PreferencesExistIn(task, tmpDir) {
		t.Error("PreferencesExistIn should return false when file doesn't exist")
	}

	// Create preferences file
	prefsPath := filepath.Join(tmpDir, "test-task.md")
	if err := os.WriteFile(prefsPath, []byte("# Test Preferences\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Should exist now
	if !PreferencesExistIn(task, tmpDir) {
		t.Error("PreferencesExistIn should return true when file exists")
	}
}

func TestNeedsSetupIn(t *testing.T) {
	tmpDir := t.TempDir()

	// Task with setup questions and no preferences
	taskWithQuestions := &mockTask{
		name:           "needs-setup",
		preferencesKey: "needs-setup",
		setupQuestions: []SetupQuestion{
			{Key: "key1", Question: "Question?", Type: QuestionText},
		},
	}

	if !NeedsSetupIn(taskWithQuestions, tmpDir) {
		t.Error("NeedsSetupIn should return true when no preferences and has questions")
	}

	// Task with no setup questions
	taskNoQuestions := &mockTask{
		name:           "no-questions",
		preferencesKey: "no-questions",
	}

	if NeedsSetupIn(taskNoQuestions, tmpDir) {
		t.Error("NeedsSetupIn should return false when task has no questions")
	}

	// Create preferences file
	prefsPath := filepath.Join(tmpDir, "needs-setup.md")
	if err := os.WriteFile(prefsPath, []byte("# Prefs\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if NeedsSetupIn(taskWithQuestions, tmpDir) {
		t.Error("NeedsSetupIn should return false when preferences exist")
	}
}

func TestSavePreferencesTo(t *testing.T) {
	tmpDir := t.TempDir()

	task := &mockTask{
		name:           "save-test",
		preferencesKey: "save-test",
		setupQuestions: []SetupQuestion{
			{Key: "workday_start", Question: "Start time?", Section: "Schedule", Type: QuestionTime},
			{Key: "priority_count", Question: "Priority items?", Section: "Display", Type: QuestionText},
			{Key: "include_routine", Question: "Include routine?", Section: "Display", Type: QuestionConfirm},
		},
	}

	result := &SetupResult{
		Answers: map[string]string{
			"workday_start":   "8:30 AM",
			"priority_count":  "3",
			"include_routine": "yes",
		},
	}

	if err := SavePreferencesTo(task, result, tmpDir); err != nil {
		t.Fatalf("SavePreferencesTo failed: %v", err)
	}

	// Verify file was created
	prefsPath := filepath.Join(tmpDir, "save-test.md")
	content, err := os.ReadFile(prefsPath)
	if err != nil {
		t.Fatalf("failed to read preferences file: %v", err)
	}

	// Check content
	contentStr := string(content)
	if !strings.Contains(contentStr, "# Save-Test Preferences") {
		t.Error("preferences file should contain title")
	}
	if !strings.Contains(contentStr, "## Schedule") {
		t.Error("preferences file should contain Schedule section")
	}
	if !strings.Contains(contentStr, "## Display") {
		t.Error("preferences file should contain Display section")
	}
	if !strings.Contains(contentStr, "**workday_start**: 8:30 AM") {
		t.Error("preferences file should contain workday_start value")
	}
	if !strings.Contains(contentStr, "**priority_count**: 3") {
		t.Error("preferences file should contain priority_count value")
	}
}

func TestSavePreferencesToDefaultSection(t *testing.T) {
	tmpDir := t.TempDir()

	task := &mockTask{
		name:           "default-section",
		preferencesKey: "default-section",
		setupQuestions: []SetupQuestion{
			{Key: "some_key", Question: "Some question?", Type: QuestionText},
		},
	}

	result := &SetupResult{
		Answers: map[string]string{
			"some_key": "value",
		},
	}

	if err := SavePreferencesTo(task, result, tmpDir); err != nil {
		t.Fatalf("SavePreferencesTo failed: %v", err)
	}

	prefsPath := filepath.Join(tmpDir, "default-section.md")
	content, err := os.ReadFile(prefsPath)
	if err != nil {
		t.Fatalf("failed to read preferences file: %v", err)
	}

	// Questions without Section should go to "Settings"
	if !strings.Contains(string(content), "## Settings") {
		t.Error("preferences file should use default 'Settings' section")
	}
}

func TestSavePreferencesCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "nested", "path")

	task := &mockTask{
		name:           "nested-test",
		preferencesKey: "nested-test",
		setupQuestions: []SetupQuestion{
			{Key: "key", Question: "Q?", Type: QuestionText},
		},
	}

	result := &SetupResult{
		Answers: map[string]string{"key": "value"},
	}

	if err := SavePreferencesTo(task, result, nestedDir); err != nil {
		t.Fatalf("SavePreferencesTo should create nested directories: %v", err)
	}

	prefsPath := filepath.Join(nestedDir, "nested-test.md")
	if _, err := os.Stat(prefsPath); os.IsNotExist(err) {
		t.Error("preferences file should have been created")
	}
}
